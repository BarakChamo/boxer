package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Herdr drives herdr headlessly over its documented socket API: a private server
// (HERDR_SOCKET_PATH and HERDR_HOME keep it clear of the user's own session), `workspace create`
// in the repository, `pane split`, `agent start --kind claude --pane <id>`, `agent prompt --wait`
// and `agent read --source recent-unwrapped`.
//
// herdr runs the harness in a pane with the shell's own environment, so the project layer applies
// unchanged and nothing herdr-specific is needed on boxer's side. Verified against herdr 0.9.1:
// pane classification reads the pane's screen buffer, not the process tree or an environment
// variable, so a pane started through boxer's harness shim is still classified as its agent kind.
type Herdr struct{ srv *server }

func (*Herdr) Name() string { return "herdr" }

func (*Herdr) Available(tier string) (bool, string) {
	if !installed("herdr") {
		return false, "herdr is not installed (brew install herdr)"
	}
	if !installed("claude") {
		return false, "claude is not installed; herdr starts a harness in a pane"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (*Herdr) Cells(tier string) []Cell {
	return []Cell{{Harness: "herdr", Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier}}
}

func (d *Herdr) sock(env *Env) string { return filepath.Join(env.Work, "herdr.sock") }

// env is what the herdr server runs with, and therefore what every pane it spawns inherits: the
// eval's PATH and trace file plus the Claude Code cell's private config dir and model endpoint.
func (d *Herdr) env(env *Env) []string {
	return append(append(env.BaseEnv(), Claude{}.modelEnv(env)...),
		"HERDR_SOCKET_PATH="+d.sock(env),
		"HERDR_HOME="+filepath.Join(env.Work, "herdr-home"),
		"HERDR_CONFIG_PATH="+filepath.Join(env.Work, "herdr-config.toml"),
		// A pane whose shell another tool has renamed (kiro-cli rewrites argv0 to
		// "bash (kiro-cli-term)") is refused by `agent start` as "not an available shell", so the
		// cell pins a plain non-login /bin/sh. terminal.default_shell is also boxer's level-S seam
		// here: pointing it at boxer-bash puts every pane's shell in the sandbox.
		"SHELL=/bin/sh",
		"HERDR_ENV=1")
}

// cli runs one herdr CLI command against the private server and decodes its JSON reply.
func (d *Herdr) cli(env *Env, args ...string) (map[string]any, string, error) {
	cmd := exec.Command("herdr", args...)
	cmd.Dir = env.Repo
	cmd.Env = d.env(env)
	out, err := cmd.CombinedOutput()
	raw := string(out)
	if err != nil {
		return nil, raw, fmt.Errorf("herdr %s: %v\n%s", strings.Join(args, " "), err, raw)
	}
	var reply struct {
		Result map[string]any `json:"result"`
	}
	for _, line := range strings.Split(raw, "\n") {
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &reply) == nil && reply.Result != nil {
			return reply.Result, raw, nil
		}
	}
	return nil, raw, nil
}

func (d *Herdr) Prepare(env *Env, c Cell) error {
	// The harness herdr starts is stock Claude Code reading the project layer, with the eval's
	// private config dir and fake or gateway model: exactly the Claude driver's project cell.
	if err := (Claude{}).Prepare(env, Cell{Harness: "claude-code", Mode: c.Mode, Entry: "project", Isolation: c.Isolation, Tier: c.Tier}); err != nil {
		return err
	}
	// Headless drivers pass -p and never see Claude Code's first-run dialogs; a pane runs it
	// interactively, where the workspace-trust question blocks startup. Accepting it in the
	// private config dir is what a human does once.
	if err := trustProject(filepath.Join(env.Work, "claude-home", ".claude.json"), env.Repo); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(env.Work, "herdr-home"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(env.Work, "herdr-config.toml"),
		[]byte("[terminal]\ndefault_shell = \"/bin/sh\"\nshell_mode = \"non_login\"\n"), 0o644); err != nil {
		return err
	}
	srv, err := startServer(env.Repo, d.env(env), "herdr", "server")
	if err != nil {
		return err
	}
	d.srv = srv
	// The server is ready when the socket answers; `herdr status` is the cheapest probe.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := d.cli(env, "workspace", "list"); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("herdr server did not come up on %s\n%s", d.sock(env), srv.output())
}

func (d *Herdr) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	tr := Transcript{}
	ws, raw, err := d.cli(env, "workspace", "create", "--cwd", env.Repo, "--label", "boxer-eval", "--no-focus")
	tr.Raw += raw
	if err != nil {
		return tr, err
	}
	wsID, _ := dig(ws, "workspace", "workspace_id").(string)
	// Split rather than reuse the workspace's first pane: that one is still starting its shell
	// ("agent target pane … is not an available shell"), and a split returns a pane id directly.
	list, raw, err := d.cli(env, "pane", "list", "--workspace", wsID)
	tr.Raw += raw
	if err != nil {
		return tr, err
	}
	first := firstPaneID(list)
	if first == "" {
		return tr, fmt.Errorf("herdr: no pane in workspace %s\n%s", wsID, raw)
	}
	split, raw, err := d.cli(env, "pane", "split", first, "--direction", "down", "--cwd", env.Repo, "--no-focus")
	tr.Raw += raw
	if err != nil {
		return tr, err
	}
	paneID, _ := dig(split, "pane", "pane_id").(string)
	if paneID == "" {
		paneID = firstPaneID(split)
	}
	if paneID == "" {
		return tr, fmt.Errorf("herdr: pane split returned no pane id\n%s", raw)
	}
	// `agent start` returns agent_not_ready when the harness is still blocked at startup; the name
	// stays usable, so the cell waits for idle rather than failing there (herdr's own contract).
	_, raw, err = d.cli(env, "agent", "start", "boxeval", "--kind", "claude", "--pane", paneID, "--timeout", "120000")
	tr.Raw += raw
	if err != nil && !strings.Contains(raw, "agent_not_ready") {
		return tr, err
	}
	if _, raw, err = d.cli(env, "agent", "wait", "boxeval", "--until", "idle", "--timeout", "120000"); err != nil {
		tr.Raw += raw
		return tr, err
	}
	tr.Raw += raw
	_, raw, err = d.cli(env, "agent", "prompt", "boxeval", prompt, "--wait", "--until", "idle", "--timeout", "240000")
	tr.Raw += raw
	if err != nil {
		return tr, err
	}
	_, raw, err = d.cli(env, "agent", "read", "boxeval", "--source", "recent-unwrapped", "--lines", "200")
	tr.Raw += raw
	if err != nil {
		return tr, err
	}
	tr.Answer = lastKernel(tr.Raw)
	return tr, nil
}

func (d *Herdr) Cleanup(env *Env, c Cell) {
	if d.srv != nil {
		d.cli(env, "server", "stop")
		d.srv.stop()
		d.srv = nil
	}
}

// trustProject marks root as trusted in a Claude Code config file.
func trustProject(path, root string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	projects, _ := m["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	projects[root] = map[string]any{"hasTrustDialogAccepted": true, "hasCompletedProjectOnboarding": true}
	m["projects"] = projects
	out, _ := json.Marshal(m)
	return os.WriteFile(path, out, 0o600)
}

// dig walks nested maps.
func dig(m map[string]any, path ...string) any {
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// firstPaneID returns the first pane id in a `pane list` reply.
func firstPaneID(m map[string]any) string {
	panes, _ := m["panes"].([]any)
	for _, p := range panes {
		if pm, ok := p.(map[string]any); ok {
			if id, ok := pm["pane_id"].(string); ok {
				return id
			}
		}
	}
	return ""
}
