// Command boxer-eval runs the harness and orchestrator evaluation matrix.
//
//	boxer-eval --tier t1                      # deterministic: real harness, fake model
//	boxer-eval --tier t2 --harness claude-code # live: the harness's own login
//	boxer-eval --tier t1 --cell claude-code/tool --keep
//	boxer-eval --tier adherence --models zai/glm-5.3-flash,anthropic/claude-haiku-4.5 --jsonl docs/adherence.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/BarakChamo/boxer/internal/eval"
)

func main() {
	parallel := flag.Int("parallel", 4, "sdlc: how many lifecycles run at once")
	limit := flag.Int("limit", 0, "sdlc: run only the first n lifecycles")
	tier := flag.String("tier", "t1", "t1 (fake model), t2 (live), adherence (live; brief, recovery, multistep per harness), or flow (one real development session; slow, network-heavy, on demand)")
	models := flag.String("models", "", "adherence: comma-separated gateway model ids to run every cell on; default BOXER_EVAL_MODEL")
	jsonl := flag.String("jsonl", "", "adherence: append each result here and render the report from the whole file, so cells can run one at a time")
	harness := flag.String("harness", "", "comma-separated driver names; default all")
	cell := flag.String("cell", "", "substring filter on cell names")
	out := flag.String("out", "", "write the Markdown report here")
	keep := flag.Bool("keep", false, "keep every cell's scratch directory (failures are always kept)")
	list := flag.Bool("list", false, "list cells and exit")
	lockRun := flag.Bool("lock-run", false, "take the host smolvm lock, then run the command after -- (used by evals/smoke.sh)")
	flag.Parse()
	loadDotEnv()

	if *lockRun {
		unlock, err := eval.HostLock(os.Stderr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "lock:", err)
			os.Exit(2)
		}
		cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "BOXER_EVAL_LOCKED=1")
		err = cmd.Run()
		unlock()
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		} else if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	if *tier == "t2" && os.Getenv("BOXER_EVAL_LOCKED") == "" {
		fmt.Fprintln(os.Stderr, "tier t2: credentials come from evals/.env when present; missing ones are reported as skips")
	}
	boxerBin, err := exec.LookPath("boxer")
	if err != nil {
		if p, e := filepath.Abs("bin/boxer"); e == nil {
			if _, e := os.Stat(p); e == nil {
				boxerBin = p
				err = nil
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "boxer binary not found on PATH or at bin/boxer")
		os.Exit(2)
	}

	drivers := eval.Drivers()
	// The SDLC tier is its own runner: its lifecycles run at the same time on purpose, which the
	// cell loop below deliberately does not do.
	if *tier == "sdlc" {
		tasks := eval.SDLCTasks()
		if *cell != "" {
			var picked []eval.SDLCTask
			for _, t := range tasks {
				if strings.Contains(t.Name, *cell) {
					picked = append(picked, t)
				}
			}
			tasks = picked
		}
		if *limit > 0 && *limit < len(tasks) {
			tasks = tasks[:*limit]
		}
		rs := eval.RunSDLC(boxerBin, tasks, *parallel, os.Stderr)
		report := eval.SDLCReport(rs, *parallel)
		if *out != "" {
			_ = os.WriteFile(*out, []byte(report), 0o644)
		}
		fmt.Println(report)
		for _, r := range rs {
			if r.Status != "pass" {
				os.Exit(1)
			}
		}
		return
	}

	adherence := *tier == "adherence"
	if adherence {
		drivers = eval.AdherenceDrivers(drivers)
	}
	if *harness != "" {
		want := map[string]bool{}
		for _, h := range strings.Split(*harness, ",") {
			want[strings.TrimSpace(h)] = true
		}
		var picked []eval.Driver
		for _, d := range drivers {
			if want[d.Name()] {
				picked = append(picked, d)
			}
		}
		drivers = picked
	}
	only := func(c eval.Cell) bool { return *cell == "" || strings.Contains(c.Name(), *cell) }
	if *list {
		for _, d := range drivers {
			for _, c := range d.Cells(*tier) {
				if only(c) {
					fmt.Println(c.Name())
				}
			}
		}
		return
	}
	// An interrupt writes the report for the cells that finished, then exits; the cell in flight
	// may leave a VM behind, so the message says how to reclaim it.
	var mu sync.Mutex
	var partial []eval.Result
	render := func(rs []eval.Result) string {
		if adherence && *jsonl != "" {
			return eval.AdherenceReport(readJSONL(*jsonl)) // every result is already appended there
		}
		if adherence {
			return eval.AdherenceReport(rs)
		}
		return eval.Report(rs, *tier)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		mu.Lock()
		report := render(partial) + "\n_interrupted; the cell in flight is not listed. Run `boxer down --all` to reclaim its VM._\n"
		mu.Unlock()
		if *out != "" {
			_ = os.WriteFile(*out, []byte(report), 0o644)
		}
		fmt.Println(report)
		os.Exit(130)
	}()
	var results []eval.Result
	if adherence {
		// Adherence cells are t2 cells with another prompt: the drivers see tier t2 and the .env
		// model override is set per model, so every driver's model plumbing is reused as is.
		for _, m := range eval.Models(*models) {
			_ = os.Setenv("BOXER_EVAL_MODEL", m)
			fmt.Fprintf(os.Stderr, "model %s\n", m)
			// The per-cell callback is what collects the results, because only it can stamp the
			// model; Run's return value is the same rows without that field.
			eval.Run(drivers, "t2", boxerBin, only, *keep, os.Stderr, func(r eval.Result) {
				r.Model = m
				mu.Lock()
				partial = append(partial, r)
				mu.Unlock()
				appendJSONL(*jsonl, r)
			})
		}
		results = partial
	} else {
		results = eval.Run(drivers, *tier, boxerBin, only, *keep, os.Stderr, func(r eval.Result) {
			mu.Lock()
			partial = append(partial, r)
			mu.Unlock()
		})
	}
	report := render(results)
	if *out != "" {
		_ = os.WriteFile(*out, []byte(report), 0o644)
	}
	fmt.Println(report)
	for _, r := range results {
		if r.Status == "fail" {
			os.Exit(1)
		}
	}
}

// appendJSONL records one result (without its transcript) for a report assembled across runs.
func appendJSONL(path string, r eval.Result) {
	if path == "" {
		return
	}
	r.Raw = ""
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jsonl:", err)
		return
	}
	defer f.Close() //nolint:errcheck // cleanup of a temporary; nothing can act on the failure
	b, _ := json.Marshal(r)
	_, _ = f.Write(append(b, '\n'))
}

// readJSONL loads earlier results; a missing file is an empty history.
func readJSONL(path string) []eval.Result {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck // read-only handle
	var out []eval.Result
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r eval.Result
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// loadDotEnv reads KEY=value lines from evals/.env or .env (both gitignored; looked up from the working directory
// and from the binary's repository root) into the environment without overriding what is already
// set. Values are never printed.
func loadDotEnv() {
	candidates := []string{filepath.Join("evals", ".env"), ".env"}
	if exe, err := os.Executable(); err == nil {
		root := filepath.Join(filepath.Dir(exe), "..")
		candidates = append(candidates, filepath.Join(root, "evals", ".env"), filepath.Join(root, ".env"))
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok || os.Getenv(k) != "" {
				continue
			}
			_ = os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
		return
	}
}
