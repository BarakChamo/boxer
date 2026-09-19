package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	Name   string // the cell's name in the report
	Level  string // rewrite | tool | shims | shell | inside | orchestrator
	Driver Driver
	Cell   Cell
	// Signature reads the evidence that this level carried the work and says how it did: a hook
	// rewrite, the run tool, a typed `boxer run`, a PATH shim. Naming the mechanism matters as much
	// as passing, because two levels can both reach the guest by quite different roads.
	Signature func(t trace, tr Transcript, guest string) (how string, err error)
	Tasks     []string // lifecycles to run; empty means all of them
}

// MatrixConfigs is the representative set: four integration levels, five harnesses and two
// orchestrators. What is left out is listed in docs/eval-matrix.md with the reason, because a
// matrix that silently omits things is worse than a small one that says so.
func MatrixConfigs(tier string) []MatrixConfig {
	dev := func(h, mode, entry string) Cell {
		return Cell{Harness: h, Mode: mode, Entry: entry, Isolation: "worktree", Compliant: true, Tier: tier,
			Image: matrixImage, Intercept: []string{"npm", "node", "next"}}
	}
	return []MatrixConfig{
		{Name: "claude/rewrite", Level: "rewrite", Driver: Claude{}, Signature: sigRewrite,
			Cell: dev("claude-code", "rewrite", "project")},
		{Name: "claude/tool", Level: "tool", Driver: Claude{}, Signature: sigTool,
			Cell: dev("claude-code", "tool", "project")},
		{Name: "codex/rewrite", Level: "rewrite", Driver: Codex{}, Signature: sigRewrite,
			Cell: dev("codex", "rewrite", "project")},
		{Name: "opencode/plugin", Level: "rewrite", Driver: OpenCode{}, Signature: sigRewrite,
			Cell: dev("opencode", "rewrite", "plugin")},
		{Name: "copilot/user", Level: "rewrite", Driver: Copilot{}, Signature: sigRewrite,
			Cell: dev("copilot", "rewrite", "user")},
		{Name: "kimi/tool", Level: "tool", Driver: Kimi{}, Signature: sigTool,
			Cell: dev("kimi", "tool", "user")},
		// `mode = "off"` is the point of this row: no hook rewrites anything, so if the work still
		// reaches the guest it was the shim on PATH that put it there.
		{Name: "claude/shims", Level: "shims", Driver: Claude{}, Signature: sigShims,
			Cell: func() Cell { c := dev("claude-code", "off", "project"); c.Shims = true; return c }()},

		// The levels with no boxer integration in the harness at all. OpenHands runs the one
		// discriminating task only, because its loop is the slowest thing in the set.
		{Name: "openhands/shell", Level: "shell", Driver: OpenHands{}, Signature: sigShims, Tasks: []string{proveTask},
			Cell: func() Cell { c := dev("openhands", "off", "sdk"); c.Shims = true; return c }()},
		{Name: "inside/claude", Level: "inside", Driver: Inside{}, Signature: sigInside,
			Cell: Cell{Harness: "inside-claude", Mode: "inside", Entry: "shell", Isolation: "worktree", Compliant: true, Tier: tier, Inside: "claude"}},
		{Name: "t3/orchestrator", Level: "orchestrator", Driver: &T3{}, Signature: sigRewrite, Tasks: []string{proveTask},
			Cell: dev("t3code", "rewrite", "project")},
	}
}

// matrixImage carries node, which the workload needs; the t1 and t2 default is alpine.
const matrixImage = "mirror.gcr.io/library/node:24-bookworm-slim"

// matrixTOML is the same workload at every level: only `mode`, `intercept` and the shim switch
// differ between rows, so a difference in the result is a difference in the level rather than in
// the job. It replaces what mkrepo wrote, which is the single-command repository.
func matrixTOML(c Cell, port int) string {
	if c.Inside != "" {
		return fmt.Sprintf(`integration = "inside"
require_worktree = "off"
memory = "4G"
cpus = 4
image = %q
%s
[network]
mode = "on"
ports = ["auto:%d"]
`, matrixImage, matrixWorkload(port), port)
	}
	return fmt.Sprintf(`require_worktree = "off"
mode = %q
memory = "4G"
cpus = 4
image = %q
intercept = ["npm", "node", "next"]
%s
[network]
mode = "allowlist"
allow_hosts = ["registry.npmjs.org"]
ports = ["auto:%d"]
`, c.Mode, matrixImage, matrixWorkload(port), port)
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

// The signatures. Each answers "did this level carry the work", which is not the same question as
// "did the task pass": an agent can do the job on the host and render a perfectly good page.

func sigRewrite(t trace, tr Transcript, guest string) (string, error) {
	if err := guestIsLinux(guest); err != nil {
		return "", err
	}
	switch {
	case t.rewrites > 0:
		return "the hook rewrote the command into `boxer run`", nil
	case typedAnywhere(t) || mentionsBoxerRun(tr.Raw):
		// The other half of the project layer: boxer installs an agent contract, and an agent that
		// reads it types `boxer run` itself. The work is still in the guest by boxer's doing, so
		// this is a pass — but a differently-shaped one, which is why the report says so.
		return "the agent typed `boxer run` itself, from the installed agent contract", nil
	case len(t.events) > 0 || len(t.allowed) > 0:
		// The OpenCode plugin rewrites in-process and writes no rewrite line, so the trace shows the
		// layer loaded and the commands passing through it, and nothing more. Taken alone that would
		// be weak evidence — but the caller has already established that the dependency really is
		// installed and that the guest, not the host, ran the command, and an agent would have to
		// fake both to reach this point. It is a pass with its mechanism named, not a silent one.
		return "the integration layer carried it, though boxer logged no rewrite (it does not log one here)", nil
	}
	return "", fmt.Errorf("rewrite level, but nothing rewrote and nothing typed `boxer run` (%d denials, %d allowed)", t.denies, len(t.allowed))
}

func sigTool(t trace, tr Transcript, guest string) (string, error) {
	if err := guestIsLinux(guest); err != nil {
		return "", err
	}
	// A denial is what an agent that ignores the brief runs into; an agent that follows it simply
	// uses the run tool and is never denied anything. Both prove the level, so both pass.
	switch {
	case usedRunTool(tr) || mentionsRunTool(tr.Raw):
		return "the agent used the `boxer_run` tool", nil
	case t.denies > 0:
		return "the shell was denied and the work went through the run tool", nil
	}
	return "", fmt.Errorf("tool level, but the run tool was never used and nothing was ever denied")
}

func sigShims(t trace, tr Transcript, guest string) (string, error) {
	if err := guestIsLinux(guest); err != nil {
		return "", err
	}
	if t.rewrites > 0 {
		return "", fmt.Errorf("shim level, but the trace shows %d rewrites: a hook carried the work, not the shim", t.rewrites)
	}
	return "a PATH shim carried the bare command into the guest", nil
}

func sigInside(t trace, tr Transcript, guest string) (string, error) {
	if err := guestIsLinux(guest); err != nil {
		return "", err
	}
	if len(t.events) > 0 {
		return "", fmt.Errorf("inside level, but the hook fired %d times: something outside was intercepting", len(t.events))
	}
	return "the harness itself ran in the guest; nothing was intercepted", nil
}

// mentionsBoxerRun and mentionsRunTool read the harness's own transcript, for the harnesses whose
// hooks never write a trace line at all. Without them a level would be judged on evidence boxer
// only collects for some harnesses, which is a measurement artefact rather than a result.
func mentionsBoxerRun(raw string) bool { return strings.Contains(raw, "boxer run ") }

// typedAnywhere is typedBoxer without the assumption that `boxer run` starts the line. Agents
// write `cd app && boxer run npm install` constantly, and the oracle's prefix check misses it.
func typedAnywhere(t trace) bool {
	for _, in := range t.inputs {
		if strings.Contains(in, "boxer run ") {
			return true
		}
	}
	return false
}

func mentionsRunTool(raw string) bool { return strings.Contains(raw, "boxer_run") }

// guestIsLinux is the shared half of every signature: the agent recorded what `node` reported as
// its platform, and on this host anything but linux means the command never left it.
func guestIsLinux(guest string) error {
	if guest != "linux" {
		if guest == "" {
			return fmt.Errorf("the agent never recorded a platform, so nothing proves the work reached the guest")
		}
		return fmt.Errorf("the work ran on the host: node reported platform %q", guest)
	}
	return nil
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
			"Also run `node -e \"console.log(process.platform)\"` and write its output alone into a file called where.txt at the top of the repository.",
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
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runMatrixCell(boxerBin, base, j.cfg, j.task, log)
		}(i, j)
	}
	wg.Wait()
	return results
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
	fail := func(format string, a ...any) MatrixResult {
		r.Findings = append(r.Findings, fmt.Sprintf(format, a...))
		r.Total = time.Since(start)
		fmt.Fprintf(log, "  %s %-28s %s\n", mark(r.Status), name, strings.Join(r.Findings, "; "))
		return r
	}
	if ok, why := cfg.Driver.Available("t2"); !ok {
		r.Status, r.Skipped = "skip", why
		fmt.Fprintf(log, "  skip %-28s %s\n", name, why)
		return r
	}

	slug := strings.NewReplacer("/", "-", ".", "-").Replace(name)
	wt := filepath.Join(filepath.Dir(base), "matrix-"+slug)
	if out, err := runIn(base, "git", "worktree", "add", "-q", "-b", "matrix/"+slug, wt); err != nil {
		return fail("git worktree add: %v\n%s", err, lastOf(out, 200))
	}
	r.Worktree = wt
	defer func() {
		_, _ = runIn(wt, boxerBin, "down")
		_, _ = runIn(base, "git", "worktree", "remove", "--force", wt)
	}()

	// The cell's own disposable world, with the worktree as its repository. NewEnv is not used
	// here: it builds a repository of its own, and the whole point is to develop in this one.
	env := &Env{
		Work: filepath.Join(wt, ".matrix"), Repo: wt, Tier: "t2", Boxer: boxerBin, Log: log,
		RunID:        fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000),
		MaxTurns:     40,
		AgentTimeout: 15 * time.Minute,
	}
	env.Dist, env.Trace = filepath.Join(env.Work, "dist"), filepath.Join(env.Work, "trace.log")
	if err := os.MkdirAll(env.Work, 0o755); err != nil {
		return fail("scratch: %v", err)
	}
	if err := writeMatrixRepo(env, cfg.Cell, guestPort); err != nil {
		return fail("write boxer.toml: %v", err)
	}
	if out, err := env.boxer(wt, "package", "all", "--out", env.Dist); err != nil {
		return fail("boxer package: %v\n%s", err, lastOf(out, 200))
	}
	if err := cfg.Driver.Prepare(env, cfg.Cell); err != nil {
		return fail("prepare: %v", err)
	}
	defer cfg.Driver.Cleanup(env, cfg.Cell)

	provision := time.Now()
	if out, err := env.boxer(wt, "up"); err != nil {
		return fail("boxer up: %v\n%s", err, lastOf(out, 400))
	}
	r.Provision = time.Since(provision)

	if st, err := env.boxer(wt, "status", "--json"); err == nil {
		var s struct {
			Ports map[string]string `json:"ports"`
		}
		if json.Unmarshal([]byte(st), &s) == nil {
			r.HostPort = s.Ports[fmt.Sprint(guestPort)]
		}
	}
	if r.HostPort == "" {
		return fail("the sandbox reported no host port for guest %d", guestPort)
	}
	if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 150*time.Second); err != nil {
		return fail("the dev server never answered before the task: %v", err)
	}

	agent := time.Now()
	before := gatewayUsed()
	tr, err := cfg.Driver.Run(env, cfg.Cell, matrixPrompt(task, r.HostPort))
	r.Agent, r.Spend, r.Raw = time.Since(agent), gatewayUsed()-before, tr.Raw
	r.Turns, r.Tools, r.MCPTools, r.Browsed, r.Restarted = agentWork(tr.Raw)
	r.UsedMCP, r.UsedBrowse = len(r.MCPTools) > 0, len(r.Browsed) > 0
	r.Transcript = filepath.Join(os.TempDir(), "matrix-"+slug+".agent.log")
	_ = os.WriteFile(r.Transcript, []byte(tr.Raw), 0o644)
	// The trace is the evidence every signature is read from, and the worktree it lives in is
	// removed when the cell ends. Keeping it is the difference between a failure that can be
	// explained and one that can only be re-run.
	if b, err := os.ReadFile(env.Trace); err == nil {
		r.Trace = filepath.Join(os.TempDir(), "matrix-"+slug+".trace.log")
		_ = os.WriteFile(r.Trace, b, 0o644)
	}
	if err != nil {
		return fail("agent: %v (transcript: %s)", err, r.Transcript)
	}

	// An orchestrator cuts its own worktree and launches the harness there, so the work — and the
	// sandbox that served it — is not the one this cell created. The driver reports where it went;
	// everything after this point judges that worktree instead.
	judged := wt
	if env.Root != "" && env.Root != wt {
		judged = env.Root
		r.Worktree = judged
		if st, err := env.boxer(judged, "status", "--json"); err == nil {
			var s struct {
				Ports map[string]string `json:"ports"`
			}
			if json.Unmarshal([]byte(st), &s) == nil && s.Ports[fmt.Sprint(guestPort)] != "" {
				r.HostPort = s.Ports[fmt.Sprint(guestPort)]
			}
		}
		if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 150*time.Second); err != nil {
			return fail("the orchestrator's own worktree never served a page: %v", err)
		}
	}

	body, berr := browserRead("http://127.0.0.1:" + r.HostPort + task.Route)
	if berr != nil {
		return fail("browser: %v", berr)
	}
	r.Rendered = strings.Contains(body, task.Expect)
	if !r.Rendered {
		return fail("the page does not show %q; it showed %q", task.Expect, firstLine(body))
	}

	// What the level has to prove, which is not what the task proves. The platform comes from the
	// file the agent was asked to write; the rest comes from boxer's own trace.
	// where.txt is written *by the agent*, so on its own it proves nothing: a model that skips the
	// work can type "linux" into a file as easily as running the command. It is corroborated two
	// ways — the dependency has to really be in package.json, and the level's own signature has to
	// show boxer carrying a command — and it is never trusted alone.
	if b, err := os.ReadFile(filepath.Join(judged, "where.txt")); err == nil {
		r.Guest = strings.TrimSpace(string(b))
	}
	if task.Name == proveTask {
		pkg, _ := os.ReadFile(filepath.Join(judged, "app", "package.json"))
		if !strings.Contains(string(pkg), `"clsx"`) {
			return fail("the task claims the dependency was installed, but clsx is not in app/package.json")
		}
	}
	if task.Name == proveTask {
		// "the hook never rewrote anything" and "the hook never ran at all" are different faults
		// with different causes, and a report that cannot tell them apart sends the reader to the
		// wrong place. The trace file is only created when a hook actually fires.
		if _, err := os.Stat(env.Trace); err != nil && cfg.Level != "inside" && cfg.Level != "shell" {
			r.Signature = "the harness's hooks never ran: boxer was never invoked, so nothing could be intercepted"
		}
		how, err := cfg.Signature(readTrace(env.Trace), tr, r.Guest)
		if err != nil {
			if r.Signature == "" {
				r.Signature = err.Error()
			}
			return fail("signature: %s", r.Signature)
		}
		r.Signature, r.Carried = "", how
	}

	changes, _ := runIn(judged, "git", "status", "--porcelain")
	for _, line := range strings.Split(strings.TrimSpace(changes), "\n") {
		if f := strings.Fields(line); len(f) > 1 && !strings.Contains(line, ".matrix") && !strings.Contains(line, ".claude") {
			r.Changed = append(r.Changed, f[len(f)-1])
		}
	}
	if len(r.Changed) == 0 {
		return fail("the page renders but the worktree has no changes")
	}
	if _, err := os.Stat(env.CanaryHost()); err == nil {
		return fail("the host leak canary exists at %s", env.CanaryHost())
	}
	r.Status = "pass"
	r.Total = time.Since(start)
	fmt.Fprintf(log, "  %s %-28s %s (provision %s, agent %s, $%.4f)\n", mark(r.Status), name,
		r.Total.Round(time.Second), r.Provision.Round(time.Second), r.Agent.Round(time.Second), r.Spend)
	return r
}

// matrixPrompt is the same job in the same words for every configuration; only the forwarded port
// differs. Nothing in it names an integration level, because a level the agent has to be told
// about is not a level that works.
func matrixPrompt(task SDLCTask, hostPort string) string {
	return fmt.Sprintf(`%s

The dev server for this worktree is already running on port %d, forwarded to
http://127.0.0.1:%s on this machine. Do not start another one unless you have to restart it.
When you are done, answer with the single word DONE.`, task.Prompt, guestPort, hostPort)
}

// MatrixReport is the write-up. It is organised by level rather than by task, because the question
// the tier answers is "which ways into the sandbox support development", not "which tasks pass".
func MatrixReport(rs []MatrixResult, parallel int) string {
	var b strings.Builder
	pass, skipped := 0, 0
	var spend float64
	for _, r := range rs {
		switch r.Status {
		case "pass":
			pass++
		case "skip":
			skipped++
		}
		spend += r.Spend
	}
	fmt.Fprintf(&b, "# boxer eval report — tier matrix — %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "The same Next.js development workload, across integration levels and harnesses: %d cells,\n", len(rs))
	fmt.Fprintf(&b, "%d at a time, each in its own git worktree with its own sandbox and its own automatic host\n", parallel)
	fmt.Fprintf(&b, "port. A cell passes only when the page renders in a real browser, the change is in the\n")
	fmt.Fprintf(&b, "worktree, and — for the %s task — the level itself is shown to have carried the work.\n\n", proveTask)
	fmt.Fprintf(&b, "**%d/%d passed** (%d skipped), $%.2f.\n\n", pass, len(rs)-skipped, skipped, spend)

	fmt.Fprintf(&b, "| cell | level | status | guest | host port | turns | time | notes |\n")
	fmt.Fprintf(&b, "| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rs {
		note := strings.Join(r.Findings, "; ")
		if r.Skipped != "" {
			note = r.Skipped
		}
		fmt.Fprintf(&b, "| %s/%s | %s | %s | %s | %s | %d | %s | %s |\n", r.Config, r.Task, r.Level, r.Status,
			dash(r.Guest), dash(r.HostPort), r.Turns, r.Total.Round(time.Second), note)
	}

	// Levels, which is the actual verdict: a level with no passing cell is not supported for
	// development, whatever its single-command cells say.
	byLevel := map[string][2]int{}
	var order []string
	for _, r := range rs {
		c, seen := byLevel[r.Level]
		if !seen {
			order = append(order, r.Level)
		}
		if r.Status == "pass" {
			c[0]++
		}
		c[1]++
		byLevel[r.Level] = c
	}
	fmt.Fprintf(&b, "\n## By level\n\n| level | passed |\n| --- | --- |\n")
	for _, l := range order {
		c := byLevel[l]
		fmt.Fprintf(&b, "| %s | %d/%d |\n", l, c[0], c[1])
	}

	fmt.Fprintf(&b, "\n## What each agent did\n\n")
	for _, r := range rs {
		if r.Skipped != "" {
			continue
		}
		fmt.Fprintf(&b, "### %s — %s\n\n", r.Config, r.Task)
		fmt.Fprintf(&b, "- level `%s`, %s in %s, %d tool calls\n", r.Level, r.Status, r.Total.Round(time.Second), r.Turns)
		if len(r.Tools) > 0 {
			var names []string
			for n, c := range r.Tools {
				names = append(names, fmt.Sprintf("%s×%d", n, c))
			}
			sort.Strings(names)
			fmt.Fprintf(&b, "- tools: %s\n", strings.Join(names, ", "))
		}
		if r.Guest != "" {
			fmt.Fprintf(&b, "- the guest reported platform `%s`\n", r.Guest)
		}
		if r.Carried != "" {
			fmt.Fprintf(&b, "- the level carried it: %s\n", r.Carried)
		}
		if r.Signature != "" {
			fmt.Fprintf(&b, "- **the level did not prove itself**: %s\n", r.Signature)
		}
		if len(r.Changed) > 0 {
			fmt.Fprintf(&b, "- changed: %s\n", strings.Join(r.Changed, ", "))
		}
		if r.Transcript != "" {
			fmt.Fprintf(&b, "- transcript: `%s`\n", r.Transcript)
		}
		if r.Trace != "" {
			fmt.Fprintf(&b, "- trace: `%s`\n", r.Trace)
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
