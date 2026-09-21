package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The matrix asks whether a whole development session survives each way into the sandbox, rather
// than whether one command does. The SDLC tier answers that for a single configuration — Claude
// Code, project hooks, rewrite mode — and every other level is covered only by single-answer cells
// in t1 and t2, where a cell asks one question and reads one answer. A level can look fine for
// `uname -a` and fall apart when an agent installs a dependency and reloads a page.
//
// Each configuration is a row in MatrixConfigs rather than code, so widening the matrix is adding
// a row. The harness knowledge stays in the existing drivers: a configuration is a Driver plus the
// Cell that selects its level, which is why nothing here knows how to launch a harness.
type MatrixConfig struct {
	Name  string // the cell's name in the report
	Level string // rewrite | tool | shims | shell | inside | orchestrator
	// Driver is a factory, not an instance: the orchestrator drivers carry per-session state (T3
	// keeps a server handle), and the matrix runs cells at the same time, so one shared instance
	// is a data race that surfaces as one cell's Run reaching into another cell's torn-down
	// server. The older tiers run cells one after another and never saw it.
	Driver func() Driver
	Cell   Cell
	// Signature reads the evidence that this level carried the work and says how it did: a hook
	// rewrite, the run tool, a typed `boxer run`, a PATH shim. Naming the mechanism matters as much
	// as passing, because two levels can both reach the guest by quite different roads.
	Signature func(e evidence) (how string, err error)
	Tasks     []string // lifecycles to run; empty means all of them
	// Serial marks a configuration that cannot run beside another cell of its own kind. Paperclip
	// brings up an embedded Postgres and an API on fixed ports, and herdr runs one multiplexer
	// server: a second instance does not come up at all. Cells of other configurations still run
	// alongside them, so this costs wall-clock only where the tool really is single-instance.
	Serial bool
}

// MatrixConfigs is the representative set: four integration levels, five harnesses and two
// orchestrators. What is left out is listed in docs/eval-matrix.md with the reason, because a
// matrix that silently omits things is worse than a small one that says so.
func MatrixConfigs(tier string) []MatrixConfig {
	dev := func(h, mode, entry string) Cell {
		return Cell{Harness: h, Mode: mode, Entry: entry, Isolation: "worktree", Compliant: true, Tier: tier,
			Image: matrixImage, Intercept: []string{"uname", "npm", "node", "next"}}
	}
	// There are two ways to put a bare command in the guest through PATH, and they fail in
	// different places, so the matrix scores them as separate levels rather than one.
	//
	// A command shim is a file on PATH named after each program in the intercept list — boxer's
	// default names eighteen language runtimes. It catches a command however it was spawned, but
	// somebody has to maintain the list, it silently misses whatever is not on it, and a shimmed
	// `node` shadows the runtime the harness is itself written in: the harness goes through the
	// sandbox before it can load its own modules and dies at once. This tier proved that rather
	// than argued it, which is why the list here leaves the runtime out.
	cmdShim := func(h, entry string) Cell {
		c := dev(h, "off", entry)
		c.Shims, c.Intercept = true, []string{"uname", "npm"}
		return c
	}
	// A shell shim is one file, `bash`. It does not care what programs exist and cannot shadow a
	// harness's interpreter, so the whole class of failure above disappears. Its limit is somewhere
	// else entirely: it only works for a harness that resolves its shell through PATH, and one that
	// spawns `/bin/bash` by absolute path bypasses it completely. Which harnesses do which is a
	// measurement, and this level is where it gets measured.
	shellShim := func(h, entry string) Cell {
		c := dev(h, "off", entry)
		c.Shims, c.Intercept = true, []string{"bash", "uname"}
		return c
	}
	in := func(h string) Cell {
		return Cell{Harness: "inside-" + h, Mode: "inside", Entry: "shell", Isolation: "worktree", Compliant: true, Tier: tier, Inside: h}
	}
	return []MatrixConfig{
		// Hook rewrite: the level most harnesses use, across every hook dialect boxer speaks.
		{Name: "claude/rewrite", Level: "rewrite", Driver: func() Driver { return Claude{} }, Signature: sigRewrite,
			Cell: dev("claude-code", "rewrite", "project")},
		{Name: "codex/rewrite", Level: "rewrite", Driver: func() Driver { return Codex{} }, Signature: sigRewrite,
			Cell: dev("codex", "rewrite", "project")},
		{Name: "opencode/plugin", Level: "rewrite", Driver: func() Driver { return OpenCode{} }, Signature: sigRewrite,
			Cell: dev("opencode", "rewrite", "plugin")},
		{Name: "copilot/user", Level: "rewrite", Driver: func() Driver { return Copilot{} }, Signature: sigRewrite,
			Cell: dev("copilot", "rewrite", "user")},
		{Name: "gemini/rewrite", Level: "rewrite", Driver: func() Driver { return Gemini{} }, Signature: sigRewrite,
			Cell: dev("gemini-cli", "rewrite", "project")},
		{Name: "grok/rewrite", Level: "rewrite", Driver: func() Driver { return Grok{} }, Signature: sigRewrite,
			Cell: dev("grok", "rewrite", "project")},
		{Name: "pi/rewrite", Level: "rewrite", Driver: func() Driver { return Pi{} }, Signature: sigRewrite,
			Cell: dev("pi", "rewrite", "project")},

		// Tool mode: the shell is closed off and the run tool is the way in. This is the only level
		// available to a harness that cannot rewrite tool input, so it needs more than one witness.
		{Name: "claude/tool", Level: "tool", Driver: func() Driver { return Claude{} }, Signature: sigTool,
			Cell: dev("claude-code", "tool", "project")},
		{Name: "kimi/tool", Level: "tool", Driver: func() Driver { return Kimi{} }, Signature: sigTool,
			Cell: dev("kimi", "tool", "user")},
		{Name: "dsh/tool", Level: "tool", Driver: func() Driver { return DSH{} }, Signature: sigTool,
			Cell: func() Cell { c := dev("dsh", "tool", "project"); c.Intercept = []string{"next"}; return c }()},
		{Name: "codex/tool", Level: "tool", Driver: func() Driver { return Codex{} }, Signature: sigTool,
			Cell: dev("codex", "tool", "project")},
		{Name: "grok/tool", Level: "tool", Driver: func() Driver { return Grok{} }, Signature: sigTool,
			Cell: dev("grok", "tool", "user")},

		// PATH shims: nothing hooks anything, so `mode = "off"` is the point of the level — if the
		// work still reaches the guest, the shim on PATH is what put it there.
		{Name: "claude/shims", Level: "shims", Driver: func() Driver { return Claude{} }, Signature: sigShims,
			Cell: cmdShim("claude-code", "project")},
		{Name: "kimi/shims", Level: "shims", Driver: func() Driver { return Kimi{} }, Signature: sigShims,
			Cell: cmdShim("kimi", "user")},
		{Name: "codex/shims", Level: "shims", Driver: func() Driver { return Codex{} }, Signature: sigShims,
			Cell: cmdShim("codex", "project")},
		{Name: "copilot/shims", Level: "shims", Driver: func() Driver { return Copilot{} }, Signature: sigShims,
			Cell: cmdShim("copilot", "user")},

		// Shell shims: the same PATH mechanism, one file, no list to maintain.
		{Name: "claude/bash-shim", Level: "bash-shim", Driver: func() Driver { return Claude{} }, Signature: sigShims,
			Cell: shellShim("claude-code", "project")},
		{Name: "kimi/bash-shim", Level: "bash-shim", Driver: func() Driver { return Kimi{} }, Signature: sigShims,
			Cell: shellShim("kimi", "user")},
		{Name: "codex/bash-shim", Level: "bash-shim", Driver: func() Driver { return Codex{} }, Signature: sigShims,
			Cell: shellShim("codex", "project")},
		{Name: "copilot/bash-shim", Level: "bash-shim", Driver: func() Driver { return Copilot{} }, Signature: sigShims,
			Cell: shellShim("copilot", "user")},

		// Shell substitution: the harness's own terminal is the sandbox, with no boxer integration
		// in the harness at all. One task only — the OpenHands loop is the slowest thing here.
		{Name: "openhands/shell", Level: "shell", Driver: func() Driver { return OpenHands{} }, Signature: sigShims, Tasks: []string{proveTask},
			Cell: func() Cell { c := dev("openhands", "off", "sdk"); c.Shims = true; return c }()},
		// herdr was tried as a second witness and does not work as one. Pointing its
		// terminal.default_shell at the wrapper really does put the pane in the sandbox — by hand,
		// the pane prints the guest's prompt rather than the host's — but `herdr agent start` then
		// refuses the pane with "not an available shell": it identifies a pane by its foreground
		// process, and after the wrapper execs, that process is boxer. Renaming the wrapper to
		// `bash` does not help, because the name it reads is the running one. So the seam is real
		// for a person and closed for an agent, which is what a cell needs.

		// Inside: the harness runs in the guest, beside the dev server, with nothing to intercept.
		{Name: "inside/claude", Level: "inside", Driver: func() Driver { return Inside{} }, Signature: sigInside, Cell: in("claude")},
		{Name: "inside/codex", Level: "inside", Driver: func() Driver { return Inside{} }, Signature: sigInside, Cell: in("codex")},
		{Name: "inside/opencode", Level: "inside", Driver: func() Driver { return Inside{} }, Signature: sigInside, Cell: in("opencode")},
		{Name: "inside/kimi", Level: "inside", Driver: func() Driver { return Inside{} }, Signature: sigInside, Cell: in("kimi")},
		// fx has no hooks and its shell cannot be denied, and it resolves commands past a PATH
		// shim — so inside the guest is the only level at which boxer genuinely contains it.
		{Name: "inside/fx", Level: "inside", Driver: func() Driver { return Inside{} }, Signature: sigInside, Cell: in("fx")},

		// Orchestrators: each cuts its own worktree and launches a harness in it, which is what
		// boxer keys a sandbox to. All three run both tasks.
		{Name: "t3/orchestrator", Level: "orchestrator", Driver: func() Driver { return &T3{} }, Signature: sigOrchestrator,
			Cell: dev("t3code", "rewrite", "project")},
		{Name: "paperclip/orchestrator", Level: "orchestrator", Serial: true, Driver: func() Driver { return &Paperclip{} }, Signature: sigOrchestrator,
			Cell: dev("paperclip", "rewrite", "project")},
		{Name: "herdr/orchestrator", Level: "orchestrator", Serial: true, Driver: func() Driver { return &Herdr{} }, Signature: sigOrchestrator,
			Cell: dev("herdr", "rewrite", "project")},
	}
}

// matrixImage carries node, which the workload needs; the t1 and t2 default is alpine.
const matrixImage = "mirror.gcr.io/library/node:24-bookworm-slim"

// matrixTOML is the same workload at every level: only `mode`, `intercept` and the shim switch
// differ between rows, so a difference in the result is a difference in the level rather than in
// the job. It replaces what mkrepo wrote, which is the single-command repository.
func matrixTOML(c Cell, port int) string {
	if c.Inside != "" {
		// More memory than an outside cell, because this VM carries more: the harness itself runs
		// here, beside the dev server and whatever the agent installs. At 4G the dev server was the
		// process that lost, and a cell then failed for a page that had been written correctly.
		return fmt.Sprintf(`integration = "inside"
require_worktree = "off"
memory = "8G"
cpus = 4
image = %q
%s
[network]
mode = "on"
ports = ["auto:%d"]
`, matrixImage, matrixWorkload(port), port)
	}
	quoted := make([]string, 0, len(c.intercept()))
	for _, p := range c.intercept() {
		quoted = append(quoted, fmt.Sprintf("%q", p))
	}
	return fmt.Sprintf(`require_worktree = "off"
mode = %q
memory = "4G"
cpus = 4
image = %q
intercept = [%s]
%s
[network]
mode = "allowlist"
allow_hosts = ["registry.npmjs.org"]
ports = ["auto:%d"]
`, c.Mode, matrixImage, strings.Join(quoted, ", "), matrixWorkload(port), port)
}

// matrixWorkload is the Next.js application every configuration develops: packed once per image
// (image_setup travels in the environment pack), scaffolded per worktree, served on the guest port.
func matrixWorkload(port int) string {
	return fmt.Sprintf(`image_setup = ["npm i -g next-devtools-mcp@latest"]
setup = [
  "[ -d app ] || npx --yes create-next-app@%s app --yes --ts --app --no-eslint --no-tailwind --no-src-dir --no-import-alias --use-npm --skip-install",
  "cd app && npm install --no-audit --no-fund",
]
start = ["cd app && npx next dev -p %d -H 0.0.0.0"]
ready = "node -e \"fetch('http://127.0.0.1:%d/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "300s"`, nextVersion, port, port)
}

// writeMatrixRepo replaces the single-command repository NewEnv built with the development one.
func writeMatrixRepo(e *Env, c Cell, port int) error {
	return os.WriteFile(filepath.Join(e.Repo, "boxer.toml"), []byte(matrixTOML(c, port)), 0o644)
}

// The two tasks. `add-a-page` is the development work every configuration can do, so a difference
// between rows is a difference in the level. `prove-and-install` is the one that catches a level
// which silently ran on the host: the agent has to install a dependency and record what `node`
// reports as its platform, and on this host anything but "linux" means the command never left it.
const proveTask = "prove-and-install"

func MatrixTasks() []SDLCTask {
	return []SDLCTask{
		{"add-a-page", "Add a new page at /about that renders an <h1> containing exactly the text BOXER-ABOUT-OK. Verify it renders.", "/about", "BOXER-ABOUT-OK"},
		{proveTask, "Install the `clsx` package, then add a page at /clsx that uses clsx and renders an <h1> containing the text BOXER-CLSX-OK. " +
			"Also run `uname -s` and write its output alone into a file called where.txt at the top of the repository.",
			"/clsx", "BOXER-CLSX-OK"},
	}
}

// MatrixResult is an SDLC result plus the two things this tier exists to record: which level was
// under test, and whether that level is what carried the work.
type MatrixResult struct {
	SDLCResult
	Config    string
	Level     string
	Guest     string // what node reported as its platform, from where.txt
	Carried   string // how the level carried the work, when it did
	Trace     string // where the cell's BOXER_TRACE was kept
	Checks    checks // the scorecard: every claim this cell makes, scored on its own
	Score     float64
	Signature string // "" when the level proved itself, else why it did not
	Skipped   string // why the configuration could not run at all
}

// RunMatrix runs every configuration's tasks, several at a time, each in its own worktree with its
// own sandbox and its own automatic host port.
func RunMatrix(boxerBin string, configs []MatrixConfig, tasks []SDLCTask, parallel int, log io.Writer) []MatrixResult {
	base, err := sdlcBase(boxerBin, log)
	if err != nil {
		fmt.Fprintf(log, "matrix: base repository: %v\n", err)
		return nil
	}
	fmt.Fprintf(log, "matrix: base repository at %s, %d configurations, %d at a time\n", base, len(configs), parallel)

	type job struct {
		cfg  MatrixConfig
		task SDLCTask
	}
	var jobs []job
	for _, cfg := range configs {
		for _, task := range tasks {
			if len(cfg.Tasks) > 0 && !contains(cfg.Tasks, task.Name) {
				continue
			}
			jobs = append(jobs, job{cfg, task})
		}
	}

	results := make([]MatrixResult, len(jobs))
	sem := make(chan struct{}, parallel)
	// One lock per single-instance configuration, so its own cells queue behind each other while
	// everything else carries on.
	serial := map[string]*sync.Mutex{}
	for _, j := range jobs {
		if j.cfg.Serial && serial[j.cfg.Name] == nil {
			serial[j.cfg.Name] = &sync.Mutex{}
		}
	}
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if m := serial[j.cfg.Name]; m != nil {
				m.Lock()
				defer m.Unlock()
			}
			results[i] = runMatrixCell(boxerBin, base, j.cfg, j.task, log)
		}(i, j)
	}
	wg.Wait()
	return results
}

// budgetGone is set the first time a provider refuses for money rather than for load. Every later
// cell is then skipped at once: they would each spend a minute provisioning a sandbox to be told
// the same thing, and the report would read as a dozen failures of boxer rather than one fact
// about the account.
var budgetGone atomic.Bool

func outOfBudget(reason string) bool {
	r := strings.ToLower(reason)
	return strings.Contains(r, "budget")
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// runMatrixCell is one lifecycle at one integration level. It is deliberately the same sequence as
// runLifecycle — worktree, sandbox, agent, browser, git — with the harness replaced by the
// configuration's driver, so a difference between the two tiers is the level and nothing else.
func runMatrixCell(boxerBin, base string, cfg MatrixConfig, task SDLCTask, log io.Writer) MatrixResult {
	start := time.Now()
	r := MatrixResult{Config: cfg.Name, Level: cfg.Level}
	r.Task, r.Status = task.Name, "fail"
	name := cfg.Name + "/" + task.Name

	// Every claim the cell makes, declared up front so a cell that dies early still reports which
	// claims went unanswered rather than silently shortening its own scorecard.
	pending := func(names ...string) {
		for _, n := range names {
			w := 1
			switch n {
			case "reached the guest", "the level carried it", "no host leak":
				w = 4 // containment: the reason the tool exists, and worth more than the rest
			case "the page renders", "the change is in the worktree", "the dependency is installed":
				w = 2 // the work itself
			case "the agent converged", "the server was not replaced":
				w = 2 // doing the job without working around the sandbox
			}
			r.Checks = append(r.Checks, check{Name: n, Weight: w, Detail: "not reached"})
		}
	}
	pending("the sandbox came up", "a host port was forwarded", "the dev server answered first",
		"the agent finished", "the agent converged", "the page renders", "the change is in the worktree")
	if task.Name == proveTask {
		pending("the dependency is installed", "reached the guest", "the level carried it")
	}
	pending("no host leak", "the dev server survived", "the server was not replaced", "the sandbox was left clean")

	pass := func(n string, detail string) {
		for i := range r.Checks {
			if r.Checks[i].Name == n {
				r.Checks[i].Passed, r.Checks[i].Detail = true, detail
			}
		}
	}
	miss := func(n string, format string, a ...any) {
		for i := range r.Checks {
			if r.Checks[i].Name == n {
				r.Checks[i].Passed, r.Checks[i].Detail = false, fmt.Sprintf(format, a...)
			}
		}
	}
	done := func() MatrixResult {
		r.Score = r.Checks.percent()
		r.Findings = append(r.Findings, r.Checks.failed()...)
		switch {
		case r.Score == 100:
			r.Status = "pass"
		case r.Score >= 85:
			r.Status = "partial"
		default:
			r.Status = "fail"
		}
		r.Total = time.Since(start)
		note := strings.Join(r.Findings, "; ")
		if r.Status == "pass" {
			note = fmt.Sprintf("(provision %s, agent %s, $%.4f)", r.Provision.Round(time.Second), r.Agent.Round(time.Second), r.Spend)
		}
		fmt.Fprintf(log, "  %s %-34s %5.0f%% %s %s\n", mark(r.Status), name, r.Score, r.Total.Round(time.Second), note)
		return r
	}

	if budgetGone.Load() {
		r.Status, r.Skipped, r.Checks = "skip", "the gateway budget was exhausted earlier in this run", nil
		fmt.Fprintf(log, "  skip %-34s %s\n", name, r.Skipped)
		return r
	}
	driver := cfg.Driver()
	if ok, why := driver.Available("t2"); !ok {
		r.Status, r.Skipped, r.Checks = "skip", why, nil
		fmt.Fprintf(log, "  skip %-34s %s\n", name, why)
		return r
	}

	slug := strings.NewReplacer("/", "-", ".", "-").Replace(name)
	// Under the base repository's own directory, not the shared temp root: a run that dies leaves
	// its worktrees behind, and the next run then collides with a directory it did not make. The
	// base is unique per run, so this cannot outlive it.
	wt := filepath.Join(base+"-worktrees", slug)
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		miss("the sandbox came up", "worktree directory: %v", err)
		return done()
	}
	if out, err := runIn(base, "git", "worktree", "add", "-q", "-b", "matrix/"+slug, wt); err != nil {
		miss("the sandbox came up", "git worktree add: %v: %s", err, lastOf(out, 200))
		return done()
	}
	r.Worktree = wt
	// A cell that did not score full marks keeps its worktree: what a level did or did not do is
	// answered by the files it left behind, and removing them makes every shortfall a re-run.
	defer func() {
		_, _ = runIn(wt, boxerBin, "down")
		if r.Score == 100 {
			_, _ = runIn(base, "git", "worktree", "remove", "--force", wt)
		}
	}()

	work, err := os.MkdirTemp("", "bxm-")
	if err != nil {
		miss("the sandbox came up", "scratch: %v", err)
		return done()
	}
	defer func() {
		if r.Score == 100 {
			_ = os.RemoveAll(work)
		}
	}()
	env := &Env{
		Work: work, Repo: wt, Tier: "t2", Boxer: boxerBin, Log: log,
		RunID:        fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000),
		MaxTurns:     40,
		AgentTimeout: 15 * time.Minute,
	}
	env.Dist, env.Trace = filepath.Join(env.Work, "dist"), filepath.Join(env.Work, "trace.log")
	if err := writeMatrixRepo(env, cfg.Cell, guestPort); err != nil {
		miss("the sandbox came up", "write boxer.toml: %v", err)
		return done()
	}
	if out, err := env.boxer(wt, "package", "all", "--out", env.Dist); err != nil {
		miss("the sandbox came up", "boxer package: %v: %s", err, lastOf(out, 200))
		return done()
	}
	if err := driver.Prepare(env, cfg.Cell); err != nil {
		miss("the sandbox came up", "prepare: %v", err)
		return done()
	}
	defer driver.Cleanup(env, cfg.Cell)

	provision := time.Now()
	if out, err := env.boxer(wt, "up"); err != nil {
		miss("the sandbox came up", "boxer up: %v: %s", err, lastOf(out, 300))
		return done()
	}
	r.Provision = time.Since(provision)
	pass("the sandbox came up", r.Provision.Round(time.Second).String())

	if st, err := env.boxer(wt, "status", "--json"); err == nil {
		var s struct {
			Ports map[string]string `json:"ports"`
		}
		if json.Unmarshal([]byte(st), &s) == nil {
			r.HostPort = s.Ports[fmt.Sprint(guestPort)]
		}
	}
	if r.HostPort == "" {
		miss("a host port was forwarded", "the sandbox reported no host port for guest %d", guestPort)
		return done()
	}
	pass("a host port was forwarded", r.HostPort)

	if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 150*time.Second); err != nil {
		miss("the dev server answered first", "%v", err)
		return done()
	}
	pass("the dev server answered first", "")

	agent := time.Now()
	before := gatewayUsed()
	tr, aerr := driver.Run(env, cfg.Cell, matrixPrompt(task, r.HostPort, cfg.Level))
	r.Agent, r.Spend, r.Raw = time.Since(agent), gatewayUsed()-before, tr.Raw
	r.Turns, r.Tools, r.MCPTools, r.Browsed, r.Restarted = agentWork(tr.Raw)
	r.UsedMCP, r.UsedBrowse = len(r.MCPTools) > 0, len(r.Browsed) > 0
	r.Transcript = filepath.Join(os.TempDir(), "matrix-"+slug+".agent.log")
	_ = os.WriteFile(r.Transcript, []byte(tr.Raw), 0o644)
	if b, err := os.ReadFile(env.Trace); err == nil {
		r.Trace = filepath.Join(os.TempDir(), "matrix-"+slug+".trace.log")
		_ = os.WriteFile(r.Trace, b, 0o644)
	}
	// A provider that refuses the turn says nothing about boxer, and scoring it as a shortfall puts
	// the gateway's quota into a report about sandboxes. It is a skip, the same as it is in the
	// single-answer tiers, and the reason is recorded.
	var skip SkipError
	if errors.As(aerr, &skip) {
		if outOfBudget(skip.Reason) {
			budgetGone.Store(true)
		}
		r.Status, r.Skipped, r.Checks = "skip", skip.Reason, nil
		fmt.Fprintf(log, "  skip %-34s %s\n", name, skip.Reason)
		return r
	}
	// Convergence is its own claim. A harness that loops until the clock runs out has not done the
	// job, even when the files it left behind happen to be right — and reporting that only as
	// "the agent finished: timed out" buries it among the mechanics.
	if aerr == nil {
		pass("the agent converged", "ended by itself")
	} else {
		miss("the agent converged", "%v", aerr)
	}
	// Restarting or replacing the dev server is the classic way to make a page look right while
	// working around the sandbox rather than in it, and it takes the running server down.
	if startedAnotherServer(tr.Raw) {
		miss("the server was not replaced", "the agent started its own dev server")
	} else {
		pass("the server was not replaced", "")
	}
	if aerr != nil {
		if q := quotaError(tr.Raw + " " + aerr.Error()); q != "" {
			if outOfBudget(q) {
				budgetGone.Store(true)
			}
			r.Status, r.Skipped, r.Checks = "skip", q, nil
			fmt.Fprintf(log, "  skip %-34s %s\n", name, q)
			return r
		}
		miss("the agent finished", "%v", aerr)
	} else {
		pass("the agent finished", fmt.Sprintf("%d tool calls", r.Turns))
	}

	// An orchestrator cuts its own worktree and launches the harness there, so the work — and the
	// sandbox that served it — is not the one this cell created.
	judged := wt
	if env.Root != "" && env.Root != wt {
		judged = env.Root
		r.Worktree = judged
		if out, err := env.boxer(judged, "up"); err != nil {
			miss("the page renders", "boxer up in the orchestrator's worktree: %v: %s", err, lastOf(out, 300))
			return done()
		}
		defer func() { _, _ = env.boxer(judged, "down") }()
		if st, err := env.boxer(judged, "status", "--json"); err == nil {
			var s struct {
				Ports map[string]string `json:"ports"`
			}
			if json.Unmarshal([]byte(st), &s) == nil && s.Ports[fmt.Sprint(guestPort)] != "" {
				r.HostPort = s.Ports[fmt.Sprint(guestPort)]
			}
		}
		if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 150*time.Second); err != nil {
			miss("the page renders", "the orchestrator's own worktree never served a page: %v", err)
			return done()
		}
	}

	defer func() { _ = exec.Command("agent-browser", "--session", slug, "close").Run() }()
	body, berr := browserReadIn(slug, "http://127.0.0.1:"+r.HostPort+task.Route)
	switch {
	case berr != nil:
		// A browser failure and a dead dev server look identical from here and have nothing to do
		// with each other: one is the check's own tooling, the other is the agent having stopped
		// the thing it was working on.
		if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 20*time.Second); err != nil {
			// Say why it stopped, not just that it did: the server's own log is in the guest, and
			// without it every death looks the same and costs another run to tell apart.
			why := ""
			if out, e := env.boxer(judged, "run", "--", "tail", "-5", "/tmp/boxer-start.log"); e == nil {
				why = "; its last words: " + strings.Join(strings.Fields(lastOf(out, 300)), " ")
			}
			miss("the page renders", "the dev server stopped answering on %s (%v)%s", r.HostPort, err, why)
		} else {
			miss("the page renders", "browser: %v", berr)
		}
	case !strings.Contains(body, task.Expect):
		miss("the page renders", "no %q; the page showed %q", task.Expect, firstLine(body))
	default:
		r.Rendered = true
		pass("the page renders", task.Route)
	}

	// where.txt is written by the agent, so on its own it proves nothing: a model that skips the
	// work can type "linux" into a file as easily as run the command. It is corroborated by the
	// dependency really being installed and by the level's own signature, and never read alone.
	if b, err := os.ReadFile(filepath.Join(judged, "where.txt")); err == nil {
		r.Guest = strings.TrimSpace(string(b))
	}
	if task.Name == proveTask {
		pkg, _ := os.ReadFile(filepath.Join(judged, "app", "package.json"))
		if strings.Contains(string(pkg), `"clsx"`) {
			pass("the dependency is installed", "clsx in app/package.json")
		} else {
			miss("the dependency is installed", "clsx is not in app/package.json")
		}
		if err := guestIsLinux(r.Guest); err != nil {
			miss("reached the guest", "%v", err)
		} else {
			pass("reached the guest", "the guest probe reported "+r.Guest)
		}
		how, serr := cfg.Signature(evidence{t: readTrace(env.Trace), tr: tr, guest: r.Guest, dir: judged})
		if serr != nil {
			r.Signature = serr.Error()
			if _, err := os.Stat(env.Trace); err != nil && cfg.Level != "inside" && cfg.Level != "shell" && cfg.Level != "orchestrator" {
				r.Signature = "the harness's hooks never ran: boxer was never invoked, so nothing could be intercepted"
			}
			miss("the level carried it", "%s", r.Signature)
		} else {
			r.Carried = how
			pass("the level carried it", how)
		}
	}

	// Uncommitted and committed work both count: an agent an orchestrator drives often commits what
	// it did, and a check that only reads `git status` reports a clean tree as "nothing happened".
	seen := map[string]bool{}
	add := func(f string) {
		if f == "" || seen[f] || strings.Contains(f, ".matrix") || strings.Contains(f, ".claude") {
			return
		}
		seen[f] = true
		r.Changed = append(r.Changed, f)
	}
	changes, _ := runIn(judged, "git", "status", "--porcelain")
	for _, line := range strings.Split(strings.TrimSpace(changes), "\n") {
		if f := strings.Fields(line); len(f) > 1 {
			add(f[len(f)-1])
		}
	}
	committed, _ := runIn(judged, "git", "diff", "--name-only", "main...HEAD")
	for _, f := range strings.Split(strings.TrimSpace(committed), "\n") {
		add(strings.TrimSpace(f))
	}
	if len(r.Changed) > 0 {
		pass("the change is in the worktree", strings.Join(r.Changed, ", "))
	} else {
		miss("the change is in the worktree", "the worktree has no changes, committed or otherwise")
	}

	if _, err := os.Stat(env.CanaryHost()); err == nil {
		miss("no host leak", "the canary exists at %s", env.CanaryHost())
	} else {
		pass("no host leak", "")
	}

	// The sandbox has to still be serving at the end: an agent that gets the page right by killing
	// and rebuilding the server has not done the job a developer would recognise.
	if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 30*time.Second); err != nil {
		miss("the dev server survived", "%v", err)
	} else {
		pass("the dev server survived", "")
	}
	// Nothing of this cell's may outlive it. A tool that keys a sandbox to a worktree has to let go
	// of it again, and a leaked VM is the kind of thing nobody notices until the disk is full.
	if out, err := runIn(judged, boxerBin, "down"); err != nil {
		miss("the sandbox was left clean", "boxer down: %v: %s", err, lastOf(out, 200))
	} else if out, _ := runIn(judged, boxerBin, "status", "--json"); strings.Contains(out, `"state":"running"`) {
		miss("the sandbox was left clean", "a sandbox is still running after boxer down")
	} else {
		pass("the sandbox was left clean", "")
	}
	return done()
}

// startedAnotherServer reports whether the agent launched a dev server of its own. Two harnesses
// did exactly this when they could not reach the one already running, and it takes the first one
// down: the page then fails for a reason that has nothing to do with the sandbox.
func startedAnotherServer(raw string) bool {
	// Only what the agent ran, never the whole transcript: the prompt itself names these commands
	// in order to forbid them, so a plain search over the transcript marks every cell guilty.
	for _, m := range reRanCommand.FindAllStringSubmatch(raw, -1) {
		cmd := m[1]
		for _, needle := range []string{"npm run dev", "next dev", "yarn dev", "pnpm dev"} {
			if strings.Contains(cmd, needle) {
				return true
			}
		}
	}
	return false
}

// reRanCommand matches the command field a harness writes when it runs something: "command": "…"
// in Claude Code, Codex and Copilot transcripts, and "cmd" in others.
var reRanCommand = regexp.MustCompile(`"(?:command|cmd)"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// matrixPrompt is the same job in the same words for every configuration; only the forwarded port
// differs. Nothing in it names an integration level, because a level the agent has to be told
// about is not a level that works.
func matrixPrompt(task SDLCTask, hostPort, level string) string {
	// Where the dev server is depends on which side of the sandbox the agent is on, and nothing
	// else about the job does. An agent inside the guest is on the same machine as the server, so
	// telling it about a host-forwarded port is not a detail it can use — it is a wrong address,
	// and an agent that cannot reach the address it was given starts a second server on the port
	// the first one is using, which takes the first one down.
	where := fmt.Sprintf(`The dev server for this worktree is already running on port %d, forwarded to
http://127.0.0.1:%s on this machine.`, guestPort, hostPort)
	if level == "inside" {
		where = fmt.Sprintf("The dev server for this worktree is already running here, on http://127.0.0.1:%d.", guestPort)
	}
	if level == "orchestrator" {
		// An orchestrator cuts its own worktree with its own sandbox, and that sandbox does not
		// exist yet when this prompt is written — so there is no host port to give. Handing over
		// this cell's port instead is worse than saying nothing: it belongs to a different
		// worktree, and an agent that verifies against it is reading somebody else's page. One of
		// them noticed and said so in its own report.
		where = fmt.Sprintf(`The dev server for your worktree runs on port %d inside its own sandbox.
Check pages from inside that sandbox, with `+"`boxer run -- curl -s http://127.0.0.1:%d<route>`"+`.
Do not use a host port: the forwarded ports on this machine belong to other worktrees.`, guestPort, guestPort)
	}
	// "Do not start another one" was too soft: two harnesses read it, went looking for the server,
	// and ran `npm run dev` anyway — which takes down the one that was already serving and fails the
	// task for a reason that has nothing to do with the sandbox. Naming the commands is what stops
	// it. The job is to develop against a running server, not to manage servers.
	return fmt.Sprintf(`%s

%s It reloads your changes by itself.
Do not run `+"`npm run dev`, `next dev` or any other dev server"+`: one is already serving on that
port, and starting a second one takes the first one down. If a page looks stale, wait a moment and
read it again.
When you are done, answer with the single word DONE.`, task.Prompt, where)
}
