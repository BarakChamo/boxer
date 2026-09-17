// Package eval runs boxer's harness and orchestrator evaluations: one cell of the matrix at a time,
// each in a fresh repository with a fresh sandbox, judged by one oracle. Tier t1 plays the model
// with fakellm; tier t2 uses the harness's real login.
package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/BarakChamo/boxer/internal/fakellm"
)

// Cell is one point in the matrix.
type Cell struct {
	Harness   string
	Mode      string // rewrite | tool | off
	Entry     string // plugin | project | user | both
	Isolation string // worktree | session | subagent | repo
	// Compliant is false for the tool-mode cell where the scripted agent ignores the brief and
	// reaches for the shell tool anyway; the correct outcome there is a denial.
	Compliant bool
	Tier      string // t1 | t2
	// Shims is true when the harness cannot rewrite tool input, so the bare command reaches the
	// guest through PATH shims and the trace shows an allow rather than a rewrite.
	Shims bool
	// Intercept overrides the repository's intercept list. Shim cells must not shadow the
	// harness's own runtime (Kimi and Gemini are node programs), so they intercept uname alone.
	Intercept []string
	// Inside names the harness that runs inside the VM (integration = "inside"); "" for outside.
	Inside string
	// Timing is when the worktree appears relative to the session (the timing matrix): "before"
	// (a linked worktree exists and the session runs in it), "warm" (main checkout,
	// warm_on_session_start), "mid" (the agent creates a worktree and moves into it), "never"
	// (main checkout only), or "" for cells outside the matrix.
	Timing string
}

// Name is the cell's report id.
func (c Cell) Name() string {
	n := fmt.Sprintf("%s/%s/%s/%s", c.Harness, c.Mode, c.Entry, c.Isolation)
	if !c.Compliant {
		n += "/noncompliant"
	}
	if c.Timing != "" {
		n += "/timing-" + c.Timing
	}
	return n
}

// Transcript is what a driver observed from the harness.
type Transcript struct {
	Answer string   // the agent's final text, first word
	Tools  []string // tool names the agent called, in order
	Raw    string   // whatever the harness printed, for the report
}

// Driver knows how to run one harness or orchestrator.
type Driver interface {
	Name() string
	// Available reports whether this driver can run at the tier, and why not.
	Available(tier string) (bool, string)
	// Cells lists the matrix points this driver supports at the tier.
	Cells(tier string) []Cell
	// Prepare installs boxer into the fresh repository for the cell.
	Prepare(env *Env, c Cell) error
	// Run drives one headless session and returns what happened.
	Run(env *Env, c Cell, prompt string) (Transcript, error)
	// Cleanup removes anything Prepare left outside env.Work.
	Cleanup(env *Env, c Cell)
}

// Env is one cell's disposable world.
type Env struct {
	Work   string // scratch root, deleted after the cell unless --keep
	Repo   string // the git repository under test
	Dist   string // rendered bundles (boxer package all)
	Trace  string // BOXER_TRACE file
	RunID  string
	Tier   string
	Boxer  string // path to the boxer binary
	LLM    *fakellm.Server
	LLMURL string // as the host reaches it
	// LLMURLGuest is the same server as the guest reaches it (host LAN address; loopback is not shared).
	LLMURLGuest string
	llmStop     func()
	Log         io.Writer
}

// Prompt is what every cell asks; the leak canary rides in the command the fake model issues, and
// a live model is asked to run the same line.
const canaryPrefix = "/tmp/boxer-leak-"

func (e *Env) Command() string {
	return fmt.Sprintf("touch %s%s && uname -a", canaryPrefix, e.RunID)
}

func (e *Env) Prompt() string {
	return fmt.Sprintf("Run the shell command `%s` exactly once and then reply with only the first word of its output.", e.Command())
}

// CanaryHost is the path that must never appear on the host.
func (e *Env) CanaryHost() string { return canaryPrefix + e.RunID }

// NewEnv creates the scratch world and, for t1, starts the fake model.
func NewEnv(tier, boxerBin string, c Cell, log io.Writer) (*Env, error) {
	work, err := os.MkdirTemp("", "boxer-eval-")
	if err != nil {
		return nil, err
	}
	// git reports symlink-resolved paths; keep everything resolved from the start.
	if r, err := filepath.EvalSymlinks(work); err == nil {
		work = r
	}
	e := &Env{Work: work, Repo: filepath.Join(work, "repo"), Dist: filepath.Join(work, "dist"), Trace: filepath.Join(work, "trace.log"),
		RunID: fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000), Tier: tier, Boxer: boxerBin, Log: log}
	if err := e.mkrepo(c); err != nil {
		return nil, err
	}
	if out, err := e.boxer(e.Repo, "package", "all", "--out", e.Dist); err != nil {
		return nil, fmt.Errorf("boxer package: %v\n%s", err, out)
	}
	if tier == "t1" {
		tools := fakellm.DefaultTools
		if c.Mode == "tool" && c.Compliant {
			tools = []string{"~boxer_run", "~boxer"} // the compliant agent uses the run tool
		} else if c.Mode == "tool" && !c.Compliant {
			// The careless one reaches for any shell tool and never the run tool.
			for _, name := range fakellm.DefaultTools {
				if !strings.Contains(name, "boxer") {
					tools = append(tools[:0:0], append(tools, name)...)
				}
			}
			tools = withoutBoxer(fakellm.DefaultTools)
		}
		delegate := ""
		if c.Isolation == "subagent" {
			delegate = "Agent" // the scripted main agent delegates once; the subagent's own turn runs the command
		}
		e.LLM = fakellm.New(fakellm.Scenario{Commands: e.commands(c), Tools: tools, Delegate: delegate})
		ln, err := net.Listen("tcp", "0.0.0.0:0") // guests reach it through the host's LAN address
		if err != nil {
			return nil, err
		}
		srv := &http.Server{Handler: e.LLM.Handler()}
		go srv.Serve(ln)
		port := ln.Addr().(*net.TCPAddr).Port
		e.LLMURL = fmt.Sprintf("http://127.0.0.1:%d", port)
		e.LLMURLGuest = fmt.Sprintf("http://%s:%d", HostIP(), port)
		e.llmStop = func() { srv.Close() }
	}
	return e, nil
}

func withoutBoxer(names []string) []string {
	var out []string
	for _, n := range names {
		if !strings.Contains(n, "boxer") {
			out = append(out, n)
		}
	}
	return out
}

// LLMTools lists the tools the scripted model called, for harnesses whose output does not say.
func (e *Env) LLMTools() []string {
	if e.LLM == nil {
		return nil
	}
	var out []string
	for _, r := range e.LLM.Requests() {
		if r.Chose != "" {
			out = append(out, r.Chose)
		}
	}
	return out
}

// Close stops the fake model and, unless keep, deletes the scratch world. With keep, the fake
// model's request log is written beside the trace for inspection.
func (e *Env) Close(keep bool) {
	if e.llmStop != nil {
		e.llmStop()
	}
	if !keep {
		os.RemoveAll(e.Work)
		return
	}
	if e.LLM != nil {
		var b strings.Builder
		for _, r := range e.LLM.Requests() {
			line, _ := json.Marshal(r)
			b.Write(line)
			b.WriteByte('\n')
		}
		os.WriteFile(filepath.Join(e.Work, "llm.jsonl"), []byte(b.String()), 0o644)
	}
}

func (e *Env) mkrepo(c Cell) error {
	if err := os.MkdirAll(e.Repo, 0o755); err != nil {
		return err
	}
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=eval@boxer", "-c", "user.name=boxer-eval", "commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = e.Repo
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v\n%s", args, err, out)
		}
	}
	intercept := []string{"uname", "npm", "node", "python3", "go", "make"}
	if len(c.Intercept) > 0 {
		intercept = c.Intercept
	}
	quoted := make([]string, len(intercept))
	for i, p := range intercept {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	// Every cell creates a fresh VM and smolvm caches images per machine, so pulls happen per
	// cell; Docker Hub's anonymous quota (100/h) ran out mid-matrix. The Google mirror has none.
	toml := fmt.Sprintf(`image = "mirror.gcr.io/library/alpine:3.20"
memory = "1G"
cpus = 2
require_worktree = "off"
isolation = %q
mode = %q
intercept = [%s]
`, c.Isolation, c.Mode, strings.Join(quoted, ", "))
	if c.Timing == "warm" {
		toml += "warm_on_session_start = true\n"
	}
	if c.Inside != "" {
		// The harness runs in the guest: node image, more memory, and the fake model's host address
		// admitted through the allowlist. `mode` is meaningless here.
		network := fmt.Sprintf("mode = \"allowlist\"\nallow_hosts = [%q, \"mirror.gcr.io\", \"storage.googleapis.com\"]", HostIP())
		if c.Tier == "t2" {
			network = "mode = \"on\"" // the guest harness talks to its real provider
		}
		toml = fmt.Sprintf(`integration = "inside"
memory = "2G"
cpus = 2
require_worktree = "off"
isolation = %q
[network]
%s
`, c.Isolation, network)
	}
	return os.WriteFile(filepath.Join(e.Repo, "boxer.toml"), []byte(toml), 0o644)
}

// boxer runs the boxer binary in dir with the eval's isolated config home.
func (e *Env) boxer(dir string, args ...string) (string, error) {
	cmd := exec.Command(e.Boxer, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(e.Work, "xdg"), "XDG_STATE_HOME="+filepath.Join(e.Work, "xdg-state"),
		"BOXER_PACKS="+filepath.Join(os.TempDir(), "boxer-eval-packs"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// BaseEnv is the environment every harness process gets: boxer first on PATH, isolated config,
// the trace file, and the fake model when t1.
func (e *Env) BaseEnv() []string {
	env := []string{}
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "CLAUDECODE", "BOXER_TRACE", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "PATH", "PWD", "OLDPWD":
			continue // PWD especially: OpenCode trusts it over the real cwd
		}
		env = append(env, kv)
	}
	env = append(env,
		"PWD="+e.Repo,
		"PATH="+filepath.Dir(e.Boxer)+":"+os.Getenv("PATH"),
		"BOXER_TRACE="+e.Trace,
		"XDG_CONFIG_HOME="+filepath.Join(e.Work, "xdg"),
		"XDG_STATE_HOME="+filepath.Join(e.Work, "xdg-state"),
		"BOXER_PACKS="+filepath.Join(os.TempDir(), "boxer-eval-packs"), // image packs survive across cells and runs
	)
	return env
}

// Result is one judged cell.
type Result struct {
	Cell     Cell
	Status   string // pass | fail | skip
	Reason   string // for skip
	Findings []Finding
	Duration time.Duration
	Raw      string
	// Retried is true when the first attempt failed on infrastructure (VM start, npm, image pull)
	// and this is the second attempt's outcome.
	Retried bool
	// CostUSD is the gateway spend attributed to this cell at t2 (credits used before minus after);
	// zero when the credits endpoint is unavailable.
	CostUSD float64
}

// infraPatterns mark failures that belong to the machine, not to boxer or the harness: the cell
// is retried once and the report says so.
var infraPatterns = []string{"cause: START_FAILED", "cause: CREATE_FAILED", "EIDLETIMEOUT", "ECONNRESET", "agent closed: EOF", "timed out", "failed to pull", "error pulling", "pulling image"}

// infra reports whether a failed result looks like an infrastructure failure.
func infra(r Result) bool {
	var b strings.Builder
	for _, f := range r.Findings {
		b.WriteString(f.Detail)
	}
	if b.Len() == 0 {
		return false
	}
	text := b.String()
	if strings.Contains(text, "agent closed: EOF") && r.Raw != "" && strings.Contains(r.Raw, "agent_message_chunk") {
		return false // the agent answered and then hung up: not infrastructure
	}
	for _, p := range infraPatterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// Run executes every cell of every driver at the tier and returns the results in matrix order.
// onResult, when set, sees each result as it lands so a partial report can be written on
// interrupt. A cell that fails on infrastructure (see infraPatterns) is run once more.
func Run(drivers []Driver, tier, boxerBin string, only func(Cell) bool, keep bool, log io.Writer, onResult func(Result)) []Result {
	unlock, err := HostLock(log)
	if err != nil {
		fmt.Fprintf(log, "eval lock: %v\n", err)
	} else {
		defer unlock()
	}
	var results []Result
	spent, budget := 0.0, budgetUSD()
	pace, _ := time.ParseDuration(os.Getenv("BOXER_EVAL_PACE") + "s")
	for _, d := range drivers {
		ok, why := d.Available(tier)
		cells := d.Cells(tier)
		for _, c := range cells {
			if only != nil && !only(c) {
				continue
			}
			if !ok {
				results = append(results, Result{Cell: c, Status: "skip", Reason: why})
				continue
			}
			if tier == "t2" && !c.Compliant {
				// A live model reads the brief and complies; only the scripted model can be careless.
				results = append(results, Result{Cell: c, Status: "skip", Reason: "noncompliant cells are scripted; t1 only"})
				continue
			}
			if tier == "t2" && spent >= budget {
				results = append(results, Result{Cell: c, Status: "skip", Reason: fmt.Sprintf("budget: $%.2f of $%.2f spent this run (BOXER_EVAL_BUDGET_USD)", spent, budget)})
				continue
			}
			before := gatewayUsed()
			r := runCell(d, c, tier, boxerBin, keep, log)
			if r.Status == "fail" && infra(r) {
				fmt.Fprintf(log, "  retrying %s after an infrastructure failure\n", c.Name())
				r = runCell(d, c, tier, boxerBin, keep, log)
				r.Retried = true
			}
			if before >= 0 {
				if after := gatewayUsed(); after >= 0 {
					r.CostUSD = after - before
					spent += r.CostUSD
					fmt.Fprintf(log, "  spend $%.4f (run total $%.4f)\n", r.CostUSD, spent)
				}
			}
			results = append(results, r)
			if onResult != nil {
				onResult(r)
			}
			// A provider rate limit is a clock, not a verdict: wait BOXER_EVAL_PACE seconds before
			// the next live cell so one refusal does not cascade through the run.
			if r.Status == "skip" && quotaError(r.Raw) != "" && pace > 0 {
				fmt.Fprintf(log, "  pacing %s after a provider refusal (BOXER_EVAL_PACE)\n", pace)
				time.Sleep(pace)
			}
		}
	}
	return results
}

// runCell runs one cell. The scratch directory (trace, transcript, fake-model log) is kept when
// the cell fails; keep keeps it for passes too.
func runCell(d Driver, c Cell, tier, boxerBin string, keep bool, log io.Writer) (r Result) {
	start := time.Now()
	fmt.Fprintf(log, "▶ %s\n", c.Name())
	env, err := NewEnv(tier, boxerBin, c, log)
	if err != nil {
		return Result{Cell: c, Status: "fail", Findings: []Finding{{"setup", err.Error()}}, Duration: time.Since(start)}
	}
	defer func() {
		r.Duration = time.Since(start)
		kept := keep || r.Status == "fail"
		if kept {
			os.WriteFile(filepath.Join(env.Work, "transcript.txt"), []byte(r.Raw), 0o644)
			r.Reason = strings.TrimSpace(r.Reason + " kept: " + env.Work)
		}
		env.Close(kept)
		fmt.Fprintf(log, "  %s %s (%s)\n", mark(r.Status), c.Name(), r.Duration.Round(time.Millisecond))
		for _, f := range r.Findings {
			fmt.Fprintf(log, "    - %s: %s\n", f.Check, f.Detail)
		}
	}()
	defer d.Cleanup(env, c)
	defer env.boxer(env.Repo, "down") // never leave a VM behind, whatever happened
	var skip SkipError
	if err := d.Prepare(env, c); errors.As(err, &skip) {
		return Result{Cell: c, Status: "skip", Reason: skip.Reason}
	} else if err != nil {
		return Result{Cell: c, Status: "fail", Findings: []Finding{{"prepare", err.Error()}}}
	}
	tr, err := d.Run(env, c, env.Prompt())
	if errors.As(err, &skip) {
		return Result{Cell: c, Status: "skip", Reason: skip.Reason, Raw: tr.Raw}
	} else if err != nil {
		return Result{Cell: c, Status: "fail", Findings: []Finding{{"run", err.Error()}}, Raw: tr.Raw}
	}
	findings := Judge(env, c, tr)
	status := "pass"
	if len(findings) > 0 {
		status = "fail"
		// A live turn the provider refused (quota, free-tier rate limit) says nothing about boxer.
		if tier == "t2" {
			if q := quotaError(tr.Raw); q != "" {
				return Result{Cell: c, Status: "skip", Reason: q, Raw: tr.Raw}
			}
		}
	}
	return Result{Cell: c, Status: status, Findings: findings, Raw: tr.Raw}
}

func mark(status string) string {
	switch status {
	case "pass":
		return "ok  "
	case "skip":
		return "skip"
	}
	return "FAIL"
}

// Report renders results as a Markdown matrix.
func Report(results []Result, tier string) string {
	var b strings.Builder
	pass, fail, skip := 0, 0, 0
	total := 0.0
	fmt.Fprintf(&b, "# boxer eval report — tier %s — %s\n\n", tier, time.Now().Format(time.RFC3339))
	b.WriteString("| Cell | Status | Time | Notes |\n| --- | --- | --- | --- |\n")
	sorted := append([]Result(nil), results...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Cell.Name() < sorted[j].Cell.Name() })
	for _, r := range sorted {
		notes := r.Reason
		if r.Retried {
			notes = strings.TrimSpace("retried once after an infrastructure failure; " + notes)
		}
		for _, f := range r.Findings {
			notes += fmt.Sprintf("%s: %s; ", f.Check, strings.ReplaceAll(f.Detail, "|", "\\|"))
		}
		switch r.Status {
		case "pass":
			pass++
		case "fail":
			fail++
		default:
			skip++
		}
		if r.CostUSD > 0 {
			total += r.CostUSD
			notes = strings.TrimSpace(fmt.Sprintf("$%.4f; %s", r.CostUSD, notes))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Cell.Name(), r.Status, r.Duration.Round(time.Millisecond), strings.TrimSpace(notes))
	}
	fmt.Fprintf(&b, "\n**passed %d · failed %d · skipped %d**\n", pass, fail, skip)
	if total > 0 {
		fmt.Fprintf(&b, "\n**gateway spend this run: $%.4f** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)\n", total)
	}
	return b.String()
}

// HostLock serialises real-smolvm users on this machine: two concurrent machine creates stall
// each other's image pulls. `evals/smoke.sh` takes the same file with flock(1).
func HostLock(log io.Writer) (func(), error) {
	if os.Getenv("BOXER_EVAL_LOCKED") != "" {
		return func() {}, nil // an ancestor --lock-run already holds it; taking it again would deadlock
	}
	path := LockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprintf(log, "waiting for %s (another eval or smoke run is using smolvm)\n", path)
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
			f.Close()
			return nil, err
		}
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// LockPath is ~/.local/state/boxer/eval.lock (or under XDG_STATE_HOME).
func LockPath() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "boxer", "eval.lock")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "boxer", "eval.lock")
}
