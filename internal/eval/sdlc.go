package eval

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The SDLC tier asks the question the other tiers do not: can somebody actually *develop* in one of
// these sandboxes? Not run a command in it — develop. Change code, see the change in a browser,
// read the framework's own diagnostics, restart a server, and do it in a worktree of their own
// while other people do the same next door.
//
// Each lifecycle is a real git worktree with its own sandbox, its own port and its own agent. They
// run at the same time on purpose: a tool that keys a sandbox to a worktree has to survive several
// worktrees at once, and that is exactly where a design mistake shows up.
type SDLCTask struct {
	Name string
	// Prompt is what the agent is asked to do, in the words a person would use.
	Prompt string
	// Route is the page the change should be visible on, and Expect is the text that should be
	// there afterwards. The check is made from the host, through the forwarded port, with a real
	// browser — because "it compiles" is not the same as "it renders".
	Route  string
	Expect string
}

// SDLCTasks are ordinary jobs, not puzzles: the point is whether the sandbox gets in the way, so
// the work has to be the kind of work people really do.
func SDLCTasks() []SDLCTask {
	return []SDLCTask{
		{"add-a-page", "Add a new page at /about that renders an <h1> containing exactly the text BOXER-ABOUT-OK. Verify it renders.", "/about", "BOXER-ABOUT-OK"},
		{"api-route", "Add an API route at /api/health that returns JSON {\"status\":\"BOXER-HEALTH-OK\"}. Verify it responds.", "/api/health", "BOXER-HEALTH-OK"},
		{"client-state", "Add a page at /counter with a client component holding a count in useState, rendering the text BOXER-COUNTER-OK and a button. Verify it renders.", "/counter", "BOXER-COUNTER-OK"},
		{"fix-a-break", "The page at /broken has a deliberate error. Use the next-devtools MCP tools to find it, fix it, and make the page render the text BOXER-FIXED-OK.", "/broken", "BOXER-FIXED-OK"},
		{"layout-change", "Add a shared footer to the root layout containing the text BOXER-FOOTER-OK, so it appears on every page.", "/", "BOXER-FOOTER-OK"},
		{"dynamic-route", "Add a dynamic route /items/[id] that renders the text BOXER-ITEM-OK followed by the id. Verify /items/42 renders.", "/items/42", "BOXER-ITEM-OK"},
		{"server-data", "Add a page at /data that reads from an async server function and renders the text BOXER-DATA-OK.", "/data", "BOXER-DATA-OK"},
		{"metadata", "Give the /about page a title using the Metadata API, and make the page render the text BOXER-META-OK.", "/about", "BOXER-META-OK"},
		{"restart-server", "Restart the dev server in the sandbox, then add a page at /restarted rendering BOXER-RESTART-OK and verify it renders.", "/restarted", "BOXER-RESTART-OK"},
		{"css-module", "Add a page at /styled that uses a CSS module to colour a heading, rendering the text BOXER-STYLED-OK.", "/styled", "BOXER-STYLED-OK"},
		{"error-boundary", "Add an error.tsx boundary for the /boom route and make /boom render the text BOXER-BOUNDARY-OK instead of crashing.", "/boom", "BOXER-BOUNDARY-OK"},
		{"install-a-dep", "Install the `clsx` package in the sandbox and use it on a new page /clsx that renders the text BOXER-CLSX-OK.", "/clsx", "BOXER-CLSX-OK"},
	}
}

// SDLCResult is one lifecycle's outcome, with enough detail to tell a boxer problem from a model
// problem: what the agent did, what the browser saw, and what each phase cost in time.
type SDLCResult struct {
	Task       string
	Worktree   string
	Port       int
	Status     string // pass | fail
	Findings   []string
	Rendered   bool
	UsedMCP    bool
	UsedBrowse bool
	Provision  time.Duration
	Agent      time.Duration
	Total      time.Duration
	Spend      float64
	Raw        string
}

// RunSDLC prepares one base repository, cuts a worktree per task, and runs `parallel` of them at
// once. The base repository is scaffolded once and committed, so every worktree starts from the
// same code and only its own changes differ — which is what a team actually looks like.
func RunSDLC(boxerBin string, tasks []SDLCTask, parallel int, log io.Writer) []SDLCResult {
	base, err := sdlcBase(boxerBin, log)
	if err != nil {
		fmt.Fprintf(log, "sdlc: could not prepare the base repository: %v\n", err)
		return nil
	}
	fmt.Fprintf(log, "sdlc: base repository at %s, %d lifecycles, %d at a time\n", base, len(tasks), parallel)

	results := make([]SDLCResult, len(tasks))
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task SDLCTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runLifecycle(boxerBin, base, task, 3200+i, log)
		}(i, task)
	}
	wg.Wait()
	return results
}

// sdlcBase scaffolds the application once, in a sandbox, and commits it. Every lifecycle's worktree
// then starts from the same commit: node_modules is gitignored, so each one installs its own, which
// is exactly the per-worktree half of the setup split.
func sdlcBase(boxerBin string, log io.Writer) (string, error) {
	dir, err := os.MkdirTemp("", "boxer-sdlc-base-")
	if err != nil {
		return "", err
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return "", err
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte(sdlcConfig(3199)), 0o644); err != nil {
		return "", err
	}
	fmt.Fprintln(log, "sdlc: scaffolding the base application in a sandbox (once)")
	if out, err := runIn(dir, boxerBin, "up"); err != nil {
		return "", fmt.Errorf("boxer up: %v\n%s", err, out)
	}
	// A deliberately broken page for the fix-a-break task, and a route that crashes for the
	// boundary task: both are ordinary mistakes rather than contrivances.
	brokenDir := filepath.Join(dir, "app", "app", "broken")
	_ = os.MkdirAll(brokenDir, 0o755)
	_ = os.WriteFile(filepath.Join(brokenDir, "page.tsx"),
		[]byte("import { NotARealThing } from './nowhere'\n\nexport default function Broken() {\n  return <h1>{NotARealThing}</h1>\n}\n"), 0o644)
	boomDir := filepath.Join(dir, "app", "app", "boom")
	_ = os.MkdirAll(boomDir, 0o755)
	_ = os.WriteFile(filepath.Join(boomDir, "page.tsx"),
		[]byte("export default function Boom() {\n  throw new Error('deliberate')\n}\n"), 0o644)
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "scaffold"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %v: %v\n%s", args, err, out)
		}
	}
	_, _ = runIn(dir, boxerBin, "down")
	return dir, nil
}

// sdlcConfig is the boxer.toml a project like this would really have: an image with node, the MCP
// server installed into the image once, dependencies installed per worktree, a dev server started
// on this worktree's own port, and a readiness probe.
func sdlcConfig(port int) string {
	return fmt.Sprintf(`require_worktree = "off"
image = "mirror.gcr.io/library/node:24-bookworm-slim"

image_setup = ["npm i -g next-devtools-mcp@latest"]

setup = [
  "[ -d app ] || npx --yes create-next-app@%s app --yes --ts --app --no-eslint --no-tailwind --no-src-dir --no-import-alias --use-npm --skip-install",
  "cd app && npm install --no-audit --no-fund",
]
start = ["cd app && npx next dev -p %d -H 0.0.0.0"]
ready = "node -e \"fetch('http://127.0.0.1:%d/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "240s"

[network]
mode = "allowlist"
allow_hosts = ["registry.npmjs.org"]
ports = ["%d:%d"]
`, nextVersion, port, port, port, port)
}

func runIn(dir, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runLifecycle is one developer's afternoon, compressed: cut a worktree, bring up its sandbox,
// hand an agent the job with the tools it would really have, then check the result the way a person
// would — by looking at the page in a browser.
func runLifecycle(boxerBin, base string, task SDLCTask, port int, log io.Writer) SDLCResult {
	start := time.Now()
	r := SDLCResult{Task: task.Name, Port: port, Status: "fail"}
	fail := func(format string, a ...any) SDLCResult {
		r.Findings = append(r.Findings, fmt.Sprintf(format, a...))
		r.Total = time.Since(start)
		fmt.Fprintf(log, "  %s %-16s %s\n", mark(r.Status), task.Name, strings.Join(r.Findings, "; "))
		return r
	}

	// A linked worktree, which is what an orchestrator makes and what boxer keys a sandbox to.
	wt := filepath.Join(filepath.Dir(base), "sdlc-"+task.Name)
	if out, err := runIn(base, "git", "worktree", "add", "-q", "-b", "task/"+task.Name, wt); err != nil {
		return fail("git worktree add: %v\n%s", err, out)
	}
	r.Worktree = wt
	defer func() {
		_, _ = runIn(wt, boxerBin, "down")
		_, _ = runIn(base, "git", "worktree", "remove", "--force", wt)
	}()

	// Its own port, so several worktrees can serve at once.
	if err := os.WriteFile(filepath.Join(wt, "boxer.toml"), []byte(sdlcConfig(port)), 0o644); err != nil {
		return fail("write boxer.toml: %v", err)
	}

	provision := time.Now()
	if out, err := runIn(wt, boxerBin, "up"); err != nil {
		return fail("boxer up: %v\n%s", err, lastOf(out, 400))
	}
	r.Provision = time.Since(provision)

	// The page must be there before the agent starts, or the task is testing the scaffold rather
	// than the change.
	if err := httpOK(fmt.Sprintf("http://127.0.0.1:%d/", port), 120*time.Second); err != nil {
		return fail("the dev server never answered before the task: %v", err)
	}

	agent := time.Now()
	out, spend, err := runSDLCAgent(boxerBin, wt, task, port)
	r.Agent, r.Spend, r.Raw = time.Since(agent), spend, out
	r.UsedMCP = strings.Contains(out, "mcp__next-devtools__")
	r.UsedBrowse = strings.Contains(out, "agent-browser")
	if err != nil {
		if path := filepath.Join(wt, "..", "sdlc-"+task.Name+".agent.log"); os.WriteFile(path, []byte(out), 0o644) == nil {
			return fail("agent: %v (transcript: %s)", err, path)
		}
		return fail("agent: %v", err)
	}

	// What a person would check: open the page and read it. Through the forwarded port, with a real
	// browser, from the host — nothing about this test knows it is talking to a microVM.
	body, berr := browserRead(fmt.Sprintf("http://127.0.0.1:%d%s", port, task.Route))
	if berr != nil {
		return fail("browser: %v", berr)
	}
	r.Rendered = strings.Contains(body, task.Expect)
	if !r.Rendered {
		return fail("the page does not show %q; it showed %q", task.Expect, firstLine(body))
	}
	// And the change has to be in the worktree, on the host, where the developer would commit it.
	if out, _ := runIn(wt, "git", "status", "--porcelain"); strings.TrimSpace(out) == "" {
		return fail("the page renders but the worktree has no changes")
	}
	r.Status = "pass"
	r.Total = time.Since(start)
	fmt.Fprintf(log, "  %s %-16s %s (provision %s, agent %s, $%.4f)\n", mark(r.Status), task.Name,
		r.Total.Round(time.Second), r.Provision.Round(time.Second), r.Agent.Round(time.Second), r.Spend)
	return r
}

// runSDLCAgent gives the agent what a developer would have: boxer's own layer (installed into the
// worktree), the dev server's MCP tools running inside the sandbox, and a browser on the host.
func runSDLCAgent(boxerBin, wt string, task SDLCTask, port int) (string, float64, error) {
	if out, err := runIn(wt, boxerBin, "install", "claude-code"); err != nil {
		return out, 0, fmt.Errorf("boxer install: %v", err)
	}
	mcp := `{"mcpServers":{"next-devtools":{"command":"` + boxerBin + `","args":["run","--","next-devtools-mcp"]}}}`
	cfgPath := filepath.Join(wt, ".mcp-sdlc.json")
	if err := os.WriteFile(cfgPath, []byte(mcp), 0o644); err != nil {
		return "", 0, err
	}
	key, why := gatewayKey()
	if why != "" {
		return "", 0, fmt.Errorf("no live model: %s", why)
	}
	// A fresh config directory asks about the key and about onboarding, and a headless session
	// answers neither: it exits in a couple of seconds with no useful message. Pre-approve both,
	// exactly as the other live drivers do.
	home := filepath.Join(wt, ".claude-home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", 0, err
	}
	approved := fmt.Sprintf(`{"customApiKeyResponses":{"approved":[%q],"rejected":[]},"hasCompletedOnboarding":true,"theme":"dark","numStartups":3}`, key[len(key)-20:])
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(approved), 0o600); err != nil {
		return "", 0, err
	}
	prompt := fmt.Sprintf(`%s

The dev server for this worktree is already running inside the sandbox on port %d.
Use the next-devtools MCP tools to inspect it, and `+"`agent-browser read http://127.0.0.1:%d<route>`"+` to see
what a browser sees. Do not start another dev server unless you have to restart this one.
When you are done, answer with the single word DONE.`, task.Prompt, port, port)

	before := gatewayUsed()
	cmd := exec.Command("claude", "-p", prompt, "--permission-mode", "bypassPermissions",
		"--mcp-config", cfgPath, "--strict-mcp-config",
		"--output-format", "stream-json", "--verbose", "--max-turns", "40")
	cmd.Dir = wt
	cmd.Env = append(os.Environ(),
		"CLAUDE_CONFIG_DIR="+filepath.Join(wt, ".claude-home"),
		"ANTHROPIC_BASE_URL="+gatewayClaude, "ANTHROPIC_API_KEY="+key,
		"ANTHROPIC_MODEL="+LiveModel("claude"),
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.Stdin = strings.NewReader("")
	err := waitFor(cmd, "claude", 12*time.Minute)
	return out.String(), gatewayUsed() - before, err
}

// browserRead reads a page the way a person would: a real browser, on the host, through the
// forwarded port. `read` returns the rendered text rather than the HTML source, so a page that
// compiles but renders nothing fails here, which is the point.
func browserRead(url string) (string, error) {
	cmd := exec.Command("agent-browser", "read", url)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := waitFor(cmd, "agent-browser", 2*time.Minute); err != nil {
		return out.String(), fmt.Errorf("%v: %s", err, lastOf(out.String(), 200))
	}
	return out.String(), nil
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// SDLCReport is the write-up: what passed, what each phase cost, and — the reason the tier exists —
// which tools the agents actually reached for when left to work normally.
func SDLCReport(rs []SDLCResult, parallel int) string {
	var b strings.Builder
	pass, mcp, browse := 0, 0, 0
	var spend float64
	var slowest time.Duration
	for _, r := range rs {
		if r.Status == "pass" {
			pass++
		}
		if r.UsedMCP {
			mcp++
		}
		if r.UsedBrowse {
			browse++
		}
		spend += r.Spend
		if r.Total > slowest {
			slowest = r.Total
		}
	}
	fmt.Fprintf(&b, "# boxer eval report — tier sdlc — %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "%d development lifecycles, %d at a time, each in its own git worktree with its own\n", len(rs), parallel)
	fmt.Fprintf(&b, "sandbox and its own port. Every check is made from the host: the page is read with a real\n")
	fmt.Fprintf(&b, "browser through the forwarded port, and the change has to be in the worktree afterwards.\n\n")
	fmt.Fprintf(&b, "**passed %d of %d · MCP used in %d · browser used in %d · $%.4f · slowest %s**\n\n",
		pass, len(rs), mcp, browse, spend, slowest.Round(time.Second))
	fmt.Fprintln(&b, "| Lifecycle | Status | Provision | Agent | Total | MCP | Browser | Spend | Notes |")
	fmt.Fprintln(&b, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, r := range rs {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | $%.4f | %s |\n", r.Task, r.Status,
			r.Provision.Round(time.Second), r.Agent.Round(time.Second), r.Total.Round(time.Second),
			yesNo(r.UsedMCP), yesNo(r.UsedBrowse), r.Spend, strings.Join(r.Findings, "; "))
	}
	return b.String()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}
