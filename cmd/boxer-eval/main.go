// Command boxer-eval runs the harness and orchestrator evaluation matrix.
//
//	boxer-eval --tier t1                      # deterministic: real harness, fake model
//	boxer-eval --tier t2 --harness claude-code # live: the harness's own login
//	boxer-eval --tier t1 --cell claude-code/tool --keep
package main

import (
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
	tier := flag.String("tier", "t1", "t1 (fake model) or t2 (live)")
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
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		mu.Lock()
		report := eval.Report(partial, *tier) + "\n_interrupted; the cell in flight is not listed. Run `boxer down --all` to reclaim its VM._\n"
		mu.Unlock()
		if *out != "" {
			os.WriteFile(*out, []byte(report), 0o644)
		}
		fmt.Println(report)
		os.Exit(130)
	}()
	results := eval.Run(drivers, *tier, boxerBin, only, *keep, os.Stderr, func(r eval.Result) {
		mu.Lock()
		partial = append(partial, r)
		mu.Unlock()
	})
	report := eval.Report(results, *tier)
	if *out != "" {
		os.WriteFile(*out, []byte(report), 0o644)
	}
	fmt.Println(report)
	for _, r := range results {
		if r.Status == "fail" {
			os.Exit(1)
		}
	}
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
			os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
		return
	}
}
