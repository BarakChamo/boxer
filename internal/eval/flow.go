package eval

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Flow is one real development session rather than one command. Every other tier asks a question
// and reads an answer; this one scaffolds a project, serves it, reaches it from the host, talks to
// a tool running inside the sandbox, and restarts — which is the first time boxer's features have
// to work together rather than one at a time.
//
// The mechanics cell needs no model: what it proves is that the sandbox can host a development
// environment at all. An agent-driven cell belongs beside it and costs a live turn; this is its
// foundation and its oracle.
type Flow struct{}

func (Flow) Name() string { return "flow" }

// nextVersion is pinned, and that is not fussiness: `@latest` changes the scaffold under the test,
// and a moving scaffold cannot be an oracle. The unpinned canary is a separate cell that is
// allowed to fail loudly when upstream moves.
// 16 or newer: the dev server's /_next/mcp endpoint, which step 4 speaks to, exists only there.
const nextVersion = "16.3.5"

func (Flow) Available(tier string) (bool, string) {
	if tier != "flow" {
		return false, "the flow tier runs on demand and before a release, never in the default gate"
	}
	// The agent cell needs a live model; the mechanics cell does not. Reported per cell below.
	// It needs the npm registry in the guest, which needs the host to have a network.
	if _, err := net.LookupHost("registry.npmjs.org"); err != nil {
		return false, "no network: the flow tier installs a real project from the npm registry"
	}
	return true, ""
}

func (Flow) Cells(tier string) []Cell {
	mk := func(scenario string) Cell {
		return Cell{Harness: "flow", Mode: "rewrite", Entry: "project", Isolation: "worktree",
			Compliant: true, Tier: tier, Scenario: scenario}
	}
	// The mechanics cell proves the sandbox can host a development environment; the agent cell
	// proves an agent can work in one, reading the running app's own MCP tools from the guest.
	return []Cell{mk("flow"), mk("flow-agent")}
}

// Prepare writes a repository whose boxer.toml describes a development environment: an image with
// node, a setup that scaffolds the app, a start that serves it, and a ready probe. This is the
// configuration a real project would write, which is the point.
func (Flow) Prepare(env *Env, c Cell) error {
	port := 3111
	cfg := fmt.Sprintf(`require_worktree = "off"
image = "mirror.gcr.io/library/node:24-bookworm-slim"
setup = [
  "npx --yes create-next-app@%s app --yes --ts --app --no-eslint --no-tailwind --no-src-dir --no-import-alias --use-npm --skip-install",
  "cd app && npm install --no-audit --no-fund",
]
start = ["cd app && npx next dev -p %d -H 0.0.0.0"]
# curl is not in a slim image, so the probe uses the runtime that certainly is: single quotes in
# the JavaScript, escaped double quotes in the TOML.
ready = "node -e \"fetch('http://127.0.0.1:%d/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "180s"

[network]
mode = "allowlist"
allow_hosts = ["registry.npmjs.org"]
ports = ["%d:%d"]
`, nextVersion, port, port, port, port)
	if err := os.WriteFile(filepath.Join(env.Repo, "boxer.toml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	if out, err := env.boxer(env.Repo, "install", "claude-code"); err != nil {
		return fmt.Errorf("boxer install: %v\n%s", err, out)
	}
	return nil
}

// Run provisions the environment and exercises it: the scaffold, the server, a tool speaking MCP
// from inside the sandbox, and a restart. What it returns is a log of what happened; the oracle
// checks the observable results.
func (d Flow) Run(env *Env, c Cell, _ string) (Transcript, error) {
	var log strings.Builder
	agent := c.Scenario == "flow-agent"
	step := func(name string, f func() error) error {
		start := time.Now()
		err := f()
		fmt.Fprintf(&log, "%-24s %-8s %s\n", name, time.Since(start).Round(time.Millisecond), status(err))
		return err
	}

	// 1. The environment comes up: scaffold, install, serve, wait until it answers.
	if err := step("provision", func() error {
		out, err := env.boxer(env.Repo, "up")
		log.WriteString(out)
		return err
	}); err != nil {
		return Transcript{Raw: log.String()}, err
	}

	// 2. The scaffold is on the host, because the worktree is the same files.
	if err := step("scaffold on host", func() error {
		if _, err := os.Stat(filepath.Join(env.Repo, "app", "package.json")); err != nil {
			return fmt.Errorf("the scaffold must land in the host worktree: %w", err)
		}
		return nil
	}); err != nil {
		return Transcript{Raw: log.String()}, err
	}

	// 3. The host reaches the dev server through the forwarded port.
	if err := step("http from host", func() error { return httpOK("http://127.0.0.1:3111/", 30*time.Second) }); err != nil {
		return Transcript{Raw: log.String()}, err
	}

	// 4. A tool that must live beside the code runs in the sandbox, and the host speaks MCP to it
	// through `boxer run`. This is the capability the flow tier exists to claim.
	if err := step("mcp in the sandbox", func() error {
		tools, err := d.mcpTools(env)
		log.WriteString("mcp tools: " + strings.Join(tools, ", ") + "\n")
		return err
	}); err != nil {
		return Transcript{Raw: log.String()}, err
	}

	// 5. The agent cell stops here and hands over to a live model, which reads the app through
	// the MCP server running in the guest.
	if agent {
		if _, why := gatewayKey(); why != "" {
			return Transcript{Raw: log.String()}, SkipError{"the agent cell needs a live model: " + why}
		}
		tr, err := d.runAgent(env, c)
		tr.Raw = log.String() + "\n--- agent ---\n" + tr.Raw
		return tr, err
	}

	// 5. It survives a restart: dependencies from the environment pack, the server started again.
	if err := step("restart", func() error {
		if out, err := env.boxer(env.Repo, "down"); err != nil {
			return fmt.Errorf("%v\n%s", err, out)
		}
		out, err := env.boxer(env.Repo, "up")
		log.WriteString(out)
		if err != nil {
			return err
		}
		if strings.Contains(out, "create-next-app") {
			return fmt.Errorf("the environment pack must make the scaffold unnecessary on a restart")
		}
		return httpOK("http://127.0.0.1:3111/", 60*time.Second)
	}); err != nil {
		return Transcript{Raw: log.String()}, err
	}

	return Transcript{Raw: log.String(), Answer: "Linux", Tools: []string{"boxer_run"}}, nil
}

func (Flow) Cleanup(env *Env, c Cell) { env.ReapWork() }

// mcpTools speaks MCP to next-devtools-mcp running inside the sandbox: initialize, then
// tools/list. The server proxies the dev server's own /_next/mcp endpoint, so it has to run where
// the code is — and `boxer run` is a clean stdio pipe, which is what makes that possible.
func (d Flow) mcpTools(env *Env) ([]string, error) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"boxer-eval","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"
	out, err := env.boxerStdin(env.Repo, in, "run", "--", "npx", "--yes", "next-devtools-mcp@latest")
	if err != nil {
		return nil, fmt.Errorf("mcp through boxer run: %v\n%s", err, lastOf(out, 400))
	}
	var tools []string
	for _, name := range []string{"nextjs_index", "nextjs_call", "nextjs_docs"} {
		if strings.Contains(out, `"`+name+`"`) {
			tools = append(tools, name)
		}
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("the MCP server listed no tools:\n%s", lastOf(out, 400))
	}
	return tools, nil
}

func status(err error) string {
	if err != nil {
		return "FAIL: " + err.Error()
	}
	return "ok"
}

func lastOf(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// httpOK polls until the page answers, because a dev server compiles its first page on request.
func httpOK(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			last = fmt.Errorf("%s returned %d", url, resp.StatusCode)
		} else {
			last = err
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("%s never answered: %w", url, last)
}

// agentFlow is the same session with an agent in it. The mechanics cell proves the sandbox can
// host a development environment; this one proves an agent can work in it — reading the running
// app's own MCP tools, which live in the guest, through an ordinary `.mcp.json` entry.
//
// It costs one live turn, so it skips without a gateway key rather than failing.
func (d Flow) runAgent(env *Env, c Cell) (Transcript, error) {
	claude := Claude{}
	if err := claude.Prepare(env, Cell{Harness: "claude-code", Entry: "project", Tier: env.Tier}); err != nil {
		return Transcript{}, err
	}
	// The dev server's own tools, running beside the code — which is inside the sandbox.
	mcp := `{"mcpServers":{"next-devtools":{"command":"boxer","args":["run","--","npx","-y","next-devtools-mcp@latest"]}}}`
	path := filepath.Join(env.Repo, ".mcp-flow.json")
	if err := os.WriteFile(path, []byte(mcp), 0o644); err != nil {
		return Transcript{}, err
	}
	prompt := "Using the next-devtools MCP server, list this app's routes. Answer with the routes only."
	cmd := exec.Command("claude", "-p", prompt, "--permission-mode", "bypassPermissions",
		"--mcp-config", path, "--output-format", "stream-json", "--verbose", "--max-turns", "8")
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(), claude.modelEnv(env)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := wait(cmd, "claude"); err != nil {
		return Transcript{Raw: out.String()}, err
	}
	tr := parseClaudeStream(out.String())
	if q := quotaError(out.String()); tr.Answer == "" && q != "" {
		return tr, SkipError{q}
	}
	return tr, nil
}
