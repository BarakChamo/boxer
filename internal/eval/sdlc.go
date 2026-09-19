package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	HostPort   string // what the guest port was actually forwarded to
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

	// What the agent actually did, read from its own transcript rather than from its account of
	// itself. This is the difference between "the task passed" and knowing whether the sandbox
	// helped or was worked around.
	Turns      int
	Tools      map[string]int // tool name to number of calls
	MCPTools   []string       // the sandboxed server's tools it reached for, in first-use order
	Browsed    []string       // the pages it looked at
	Restarted  bool           // did it restart the dev server
	Changed    []string       // files the worktree ended up with, per git
	Transcript string         // where the full session was kept
}

// agentWork reads a Claude session transcript and reports what the agent did. Counting tool calls
// is the only honest way to tell whether the MCP server in the guest was used or merely available.
func agentWork(raw string) (turns int, tools map[string]int, mcpTools, browsed []string, restarted bool) {
	tools = map[string]int{}
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, `"type":"assistant"`) {
			continue
		}
		var e struct {
			Message struct {
				Content []struct {
					Type  string          `json:"type"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		for _, c := range e.Message.Content {
			if c.Type != "tool_use" {
				continue
			}
			turns++
			tools[c.Name]++
			if name, ok := strings.CutPrefix(c.Name, "mcp__next-devtools__"); ok && !seen[name] {
				seen[name] = true
				mcpTools = append(mcpTools, name)
			}
			in := string(c.Input)
			if c.Name == "Bash" {
				if strings.Contains(in, "agent-browser") {
					browsed = append(browsed, browsedURL(in))
				}
				if strings.Contains(in, "next dev") || strings.Contains(in, "npm run dev") {
					restarted = true
				}
			}
		}
	}
	return turns, tools, mcpTools, browsed, restarted
}

// browsedURL pulls the address out of an agent-browser command, for the report.
func browsedURL(in string) string {
	i := strings.Index(in, "http")
	if i < 0 {
		return "(a page)"
	}
	rest := in[i:]
	if j := strings.IndexAny(rest, `"' \`); j > 0 {
		rest = rest[:j]
	}
	return rest
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
	// A dev server inside a sandbox is reached from the host through a forwarded port, so the
	// browser's origin is not the one the server expects and Next.js refuses its own dev chunks.
	// A real project hits this once and writes this line; the fixture writes it too, so the tier
	// measures development rather than that single trap. It is in troubleshooting.md.
	_ = os.WriteFile(filepath.Join(dir, "app", "next.config.ts"),
		[]byte("import type { NextConfig } from 'next'\n\nconst nextConfig: NextConfig = {\n  allowedDevOrigins: ['127.0.0.1', 'localhost'],\n}\n\nexport default nextConfig\n"), 0o644)

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
// guestPort is the same in every worktree on purpose: each sandbox is its own machine, so the
// inside of one cannot collide with the inside of another. Only the host side needs arranging,
// which is what `auto` does.
const guestPort = 3000

func sdlcConfig(port int) string {
	_ = port
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
# An automatic host port: boxer.toml is committed, so a fixed one would collide the moment a
# second worktree started. This is the line a real project would write.
ports = ["auto:%d"]
`, nextVersion, guestPort, guestPort, guestPort)
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

	// Where the guest's port actually landed on the host. With `auto` this is the only way to know,
	// and everything the host does — the browser, the checks — has to use it.
	if st, err := runIn(wt, boxerBin, "status", "--json"); err == nil {
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

	// The page must be there before the agent starts, or the task is testing the scaffold rather
	// than the change.
	if err := httpOK("http://127.0.0.1:"+r.HostPort+"/", 120*time.Second); err != nil {
		return fail("the dev server never answered before the task: %v", err)
	}

	agent := time.Now()
	out, spend, err := runSDLCAgent(boxerBin, wt, task, r.HostPort)
	r.Agent, r.Spend, r.Raw = time.Since(agent), spend, out
	r.Turns, r.Tools, r.MCPTools, r.Browsed, r.Restarted = agentWork(out)
	r.UsedMCP = len(r.MCPTools) > 0
	r.UsedBrowse = len(r.Browsed) > 0
	// Every session is kept, not only the failures: a report about how agents work in a sandbox
	// needs the sessions that went well just as much.
	r.Transcript = filepath.Join(os.TempDir(), "sdlc-"+task.Name+".agent.log")
	_ = os.WriteFile(r.Transcript, []byte(out), 0o644)
	if err != nil {
		return fail("agent: %v (transcript: %s)", err, r.Transcript)
	}

	// What a person would check: open the page and read it. Through the forwarded port, with a real
	// browser, from the host — nothing about this test knows it is talking to a microVM.
	body, berr := browserRead("http://127.0.0.1:" + r.HostPort + task.Route)
	if berr != nil {
		return fail("browser: %v", berr)
	}
	r.Rendered = strings.Contains(body, task.Expect)
	if !r.Rendered {
		return fail("the page does not show %q; it showed %q", task.Expect, firstLine(body))
	}
	// And the change has to be in the worktree, on the host, where the developer would commit it.
	changes, _ := runIn(wt, "git", "status", "--porcelain")
	for _, line := range strings.Split(strings.TrimSpace(changes), "\n") {
		if f := strings.Fields(line); len(f) > 1 && !strings.Contains(line, ".claude") && !strings.Contains(line, ".mcp-sdlc") {
			r.Changed = append(r.Changed, f[len(f)-1])
		}
	}
	if len(r.Changed) == 0 {
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
func runSDLCAgent(boxerBin, wt string, task SDLCTask, hostPort string) (string, float64, error) {
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

The dev server for this worktree is already running inside the sandbox on port %d *inside the
sandbox*, forwarded to http://127.0.0.1:%s on this machine.
Use the next-devtools MCP tools to inspect it (they run inside the sandbox, so they see port %d),
and `+"`agent-browser read http://127.0.0.1:%s<route>`"+` to see what a browser sees.
Do not start another dev server unless you have to restart this one.
When you are done, answer with the single word DONE.`, task.Prompt, guestPort, hostPort, guestPort, hostPort)

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
	if err := waitFor(cmd, "agent-browser", 2*time.Minute); err == nil {
		return out.String(), nil
	}
	// A page can be correct and not be 200: an error boundary renders its message *with* a 500,
	// which is the whole point of one. `read <url>` refuses those, but navigating and then reading
	// renders it — which is also what a person does. Fetching the HTML instead would not do: the
	// boundary is client-rendered, so the server's body is an empty shell.
	if err := waitFor(exec.Command("agent-browser", "open", url), "agent-browser", 2*time.Minute); err != nil {
		return out.String(), fmt.Errorf("browser open: %v", err)
	}
	var second bytes.Buffer
	read := exec.Command("agent-browser", "read")
	read.Stdout, read.Stderr = &second, &second
	if err := waitFor(read, "agent-browser", 2*time.Minute); err != nil {
		return second.String(), fmt.Errorf("browser read after open: %v: %s", err, lastOf(second.String(), 200))
	}
	return second.String(), nil
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
	fmt.Fprintln(&b, "| Lifecycle | Status | Port | Provision | Agent | Turns | MCP calls | Browsed | Spend |")
	fmt.Fprintln(&b, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, r := range rs {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d | %d | %d | $%.4f |\n", r.Task, r.Status,
			hostPort(r), r.Provision.Round(time.Second), r.Agent.Round(time.Second),
			r.Turns, mcpCalls(r), len(r.Browsed), r.Spend)
	}

	// Ports are the thing several worktrees at once are most likely to fight over, so the report
	// says plainly whether any two got the same one.
	fmt.Fprintf(&b, "\n## Ports\n\nEvery lifecycle asked for guest port %d with `auto`, and each was given its own host port.\n\n", 3000)
	used := map[string][]string{}
	for _, r := range rs {
		if r.HostPort != "" {
			used[r.HostPort] = append(used[r.HostPort], r.Task)
		}
	}
	clash := false
	for port, tasks := range used {
		if len(tasks) > 1 {
			clash = true
			fmt.Fprintf(&b, "- **collision**: %s all took host port %s\n", strings.Join(tasks, ", "), port)
		}
	}
	if !clash {
		fmt.Fprintf(&b, "No collisions: %d distinct host ports for %d lifecycles.\n", len(used), len(rs))
	}

	fmt.Fprintln(&b, "\n## What each agent did")
	fmt.Fprintln(&b, "\nRead from each session's own transcript, not from what the agent said about itself.")
	for _, r := range rs {
		fmt.Fprintf(&b, "\n### %s — %s in %s\n\n", r.Task, r.Status, r.Total.Round(time.Second))
		if len(r.Findings) > 0 {
			fmt.Fprintf(&b, "Failed: %s\n\n", strings.Join(r.Findings, "; "))
		}
		fmt.Fprintf(&b, "- **tools**: %s\n", toolSummary(r.Tools))
		if len(r.MCPTools) > 0 {
			fmt.Fprintf(&b, "- **MCP server in the guest**: %s\n", strings.Join(r.MCPTools, ", "))
		} else {
			fmt.Fprintln(&b, "- **MCP server in the guest**: not used")
		}
		if len(r.Browsed) > 0 {
			fmt.Fprintf(&b, "- **browser**: %d page reads, e.g. %s\n", len(r.Browsed), r.Browsed[0])
		} else {
			fmt.Fprintln(&b, "- **browser**: not used")
		}
		if r.Restarted {
			fmt.Fprintln(&b, "- **restarted the dev server** in the sandbox")
		}
		if len(r.Changed) > 0 {
			fmt.Fprintf(&b, "- **left in the worktree**: %s\n", strings.Join(r.Changed, ", "))
		}
		fmt.Fprintf(&b, "- **host port**: %s · **transcript**: `%s`\n", hostPort(r), r.Transcript)
	}
	return b.String()
}

func hostPort(r SDLCResult) string {
	if r.HostPort == "" {
		return "-"
	}
	return r.HostPort
}

func mcpCalls(r SDLCResult) int {
	n := 0
	for name, count := range r.Tools {
		if strings.HasPrefix(name, "mcp__") {
			n += count
		}
	}
	return n
}

// toolSummary prints the tools a session used, most-used first, because the shape of the work is
// in the proportions: an agent that ran twenty shell commands and no MCP call was working around
// something.
func toolSummary(tools map[string]int) string {
	type kv struct {
		name  string
		count int
	}
	var all []kv
	for k, v := range tools {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].count != all[j].count {
			return all[i].count > all[j].count
		}
		return all[i].name < all[j].name
	})
	var parts []string
	for _, t := range all {
		parts = append(parts, fmt.Sprintf("%s×%d", strings.TrimPrefix(t.name, "mcp__next-devtools__"), t.count))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}
