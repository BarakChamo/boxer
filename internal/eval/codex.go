package eval

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Codex drives `codex exec`. t1 uses a private CODEX_HOME whose config.toml points the Responses
// wire API at the fake model; hooks live either inline in that config.toml (entry=user) or in the
// repository's .codex/hooks.json (entry=project). Codex persists hook trust per hook, so a fresh
// home needs --dangerously-bypass-hook-trust for either layer; Codex's own sandbox is bypassed
// because boxer is the sandbox and a hypervisor cannot start inside seatbelt.
type Codex struct{}

func (Codex) Name() string { return "codex" }

func (Codex) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("codex"); err != nil {
		return false, "codex not installed"
	}
	if tier == "t2" {
		if out, err := exec.Command("codex", "login", "status").CombinedOutput(); err != nil || !strings.Contains(string(out), "Logged in") {
			return false, "codex not logged in"
		}
	}
	return true, ""
}

func (Codex) Cells(tier string) []Cell {
	h := "codex"
	cells := []Cell{
		{Harness: h, Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier},
		{Harness: h, Mode: "off", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier},
	}
	if tier == "t1" {
		cells = append(cells,
			Cell{Harness: h, Mode: "rewrite", Entry: "user", Isolation: "worktree", Compliant: true, Tier: tier},
			Cell{Harness: h, Mode: "rewrite", Entry: "both", Isolation: "worktree", Compliant: true, Tier: tier},
			Cell{Harness: h, Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: true, Tier: tier},
			Cell{Harness: h, Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: false, Tier: tier},
		)
	}
	return cells
}

func (Codex) home(env *Env) string { return filepath.Join(env.Work, "codex-home") }

func (d Codex) Prepare(env *Env, c Cell) error {
	home := d.home(env)
	if env.Tier == "t1" {
		if err := os.MkdirAll(home, 0o755); err != nil {
			return err
		}
		cfg := fmt.Sprintf(`model = "fake-model"
model_provider = "fake"
[model_providers.fake]
name = "fake"
base_url = "%s/v1"
wire_api = "responses"
requires_openai_auth = false
[mcp_servers.boxer]
command = "boxer"
args = ["mcp", "--harness", "codex"]
`, env.LLMURL)
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(cfg), 0o644); err != nil {
			return err
		}
	}
	if c.Entry == "user" || c.Entry == "both" {
		cmd := exec.Command(env.Boxer, "install", "codex", "--user")
		cmd.Dir = env.Repo
		cmd.Env = append(env.BaseEnv(), "CODEX_HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("boxer install codex --user: %v\n%s", err, out)
		}
	}
	if c.Entry == "project" || c.Entry == "both" {
		if out, err := env.boxer(env.Repo, "install", "codex"); err != nil {
			return fmt.Errorf("boxer install codex: %v\n%s", err, out)
		}
	}
	return nil
}

func (d Codex) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	last := filepath.Join(env.Work, "last.txt")
	args := []string{"exec", "--dangerously-bypass-approvals-and-sandbox", "--dangerously-bypass-hook-trust", "--skip-git-repo-check", "--json", "-o", last, prompt}
	cmd := exec.Command("codex", args...)
	cmd.Dir = env.Repo
	cmd.Env = env.BaseEnv()
	if env.Tier == "t1" {
		cmd.Env = append(cmd.Env, "CODEX_HOME="+d.home(env))
	}
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(4 * time.Minute):
		cmd.Process.Kill()
		return Transcript{Raw: out.String()}, fmt.Errorf("codex timed out")
	}
	tr := parseCodexJSON(out.String())
	if b, err := os.ReadFile(last); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			tr.Answer = f[0]
		}
	}
	if len(tr.Tools) == 0 {
		tr.Tools = env.LLMTools()
	}
	return tr, nil
}

func (Codex) Cleanup(env *Env, c Cell) {}

// parseCodexJSON reads `codex exec --json` events: command executions and MCP tool calls become
// tool names; the last agent message is the answer when -o did not capture one.
func parseCodexJSON(s string) Transcript {
	tr := Transcript{Raw: s}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e map[string]any
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		item, _ := e["item"].(map[string]any)
		if item == nil {
			continue
		}
		switch item["type"] {
		case "command_execution":
			if e["type"] == "item.completed" || e["type"] == "item.started" {
				tr.Tools = appendOnce(tr.Tools, "Bash:"+fmt.Sprint(item["id"]))
			}
		case "mcp_tool_call":
			tr.Tools = appendOnce(tr.Tools, fmt.Sprintf("%v__%v:%v", item["server"], item["tool"], item["id"]))
		case "agent_message":
			if t, ok := item["text"].(string); ok {
				if f := strings.Fields(t); len(f) > 0 {
					tr.Answer = f[0]
				}
			}
		}
	}
	for i, t := range tr.Tools { // drop the de-dup suffix
		if j := strings.LastIndex(t, ":"); j > 0 {
			tr.Tools[i] = t[:j]
		}
	}
	return tr
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
