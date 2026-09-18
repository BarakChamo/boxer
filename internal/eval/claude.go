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
)

// Claude drives Claude Code headless. t1 points it at the fake model through ANTHROPIC_BASE_URL
// with a pre-approved dummy key in a private CLAUDE_CONFIG_DIR; t2 uses the user's login.
type Claude struct{}

func (Claude) Name() string { return "claude-code" }

func (Claude) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("claude"); err != nil {
		return false, "claude not installed"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (Claude) Cells(tier string) []Cell {
	h := "claude-code"
	mk := func(mode, entry, iso string, compliant bool) Cell {
		return Cell{Harness: h, Mode: mode, Entry: entry, Isolation: iso, Compliant: compliant, Tier: tier}
	}
	cells := []Cell{
		mk("rewrite", "plugin", "worktree", true),
		mk("rewrite", "project", "worktree", true),
		mk("rewrite", "both", "worktree", true),
		mk("tool", "plugin", "worktree", true),
		mk("tool", "plugin", "worktree", false),
		mk("tool", "project", "worktree", true),
		mk("rewrite", "plugin", "repo", true),
		mk("off", "plugin", "worktree", true),
	}
	if tier == "t1" {
		cells = append(cells, mk("rewrite", "user", "worktree", true),
			mk("rewrite", "plugin", "session", true),
			mk("rewrite", "plugin", "subagent", true))
		for _, timing := range []string{"before", "warm", "mid", "never"} {
			c := mk("rewrite", "plugin", "worktree", true)
			c.Timing = timing
			cells = append(cells, c)
		}
	}
	return cells
}

const fakeKey = "sk-ant-fake-0123456789abcdefghij"

func (Claude) home(env *Env) string { return filepath.Join(env.Work, "claude-home") }

func (d Claude) Prepare(env *Env, c Cell) error {
	// A fresh config dir with the key pre-approved, so no login and no prompt; the user's own
	// Keychain login is never touched. t1 approves the dummy key, t2 the gateway key.
	home := d.home(env)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	key := d.apiKey(env)
	cfg := map[string]any{
		"customApiKeyResponses":  map[string]any{"approved": []string{key[len(key)-20:]}, "rejected": []string{}},
		"hasCompletedOnboarding": true, "theme": "dark", "numStartups": 3,
	}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), b, 0o600); err != nil {
		return err
	}
	if c.Timing == "before" {
		if err := env.AddWorktree(); err != nil {
			return err
		}
	}
	switch c.Entry {
	case "project", "both":
		if out, err := env.boxer(env.Repo, "install", "claude-code"); err != nil {
			return fmt.Errorf("boxer install: %v\n%s", err, out)
		}
		// Project MCP servers need approval; grant it for the eval repository only.
		local := filepath.Join(env.Repo, ".claude", "settings.local.json")
		os.WriteFile(local, []byte(`{"enableAllProjectMcpServers": true}`+"\n"), 0o644)
	case "user":
		cmd := exec.Command(env.Boxer, "install", "claude-code", "--user")
		cmd.Dir = env.Repo
		cmd.Env = append(env.BaseEnv(), "CLAUDE_CONFIG_DIR="+d.home(env))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("boxer install --user: %v\n%s", err, out)
		}
	}
	return nil
}

func (d Claude) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	args := []string{"-p", prompt, "--permission-mode", "bypassPermissions", "--output-format", "stream-json", "--verbose", "--max-turns", "6"}
	if c.Entry == "plugin" || c.Entry == "both" {
		args = append(args, "--plugin-dir", filepath.Join(env.Dist, "claude-code"))
	}
	cmd := exec.Command("claude", args...)
	cmd.Dir = env.Repo
	if c.Timing == "before" {
		cmd.Dir = env.Worktree() // the session opens in the worktree an orchestrator made
	}
	cmd.Env = append(env.BaseEnv(), d.modelEnv(env)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := wait(cmd, "claude"); err != nil {
		return Transcript{Raw: out.String()}, err
	}
	tr := parseClaudeStream(out.String())
	if env.Tier == "t2" && tr.Answer == "" {
		if q := quotaError(out.String()); q != "" {
			return tr, SkipError{q}
		}
	}
	return tr, nil
}

func (Claude) Cleanup(env *Env, c Cell) { env.ReapWork() }

// parseClaudeStream extracts the final answer and tool names from stream-json output.
func parseClaudeStream(s string) Transcript {
	tr := Transcript{Raw: s}
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e map[string]any
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch e["type"] {
		case "assistant":
			msg, _ := e["message"].(map[string]any)
			content, _ := msg["content"].([]any)
			for _, c := range content {
				if m, ok := c.(map[string]any); ok && m["type"] == "tool_use" {
					tr.Tools = append(tr.Tools, fmt.Sprint(m["name"]))
				}
			}
		case "result":
			if r, ok := e["result"].(string); ok {
				if f := strings.Fields(r); len(f) > 0 {
					tr.Answer = f[0]
				}
			}
		}
	}
	return tr
}

// apiKey is what the eval's Claude Code authenticates with: the dummy key at t1, the gateway key
// at t2 (the gateway accepts its key as x-api-key, so ANTHROPIC_API_KEY works and the approval
// list keeps the run non-interactive).
func (d Claude) apiKey(env *Env) string {
	if env.Tier == "t2" {
		k, _ := gatewayKey()
		return k
	}
	return fakeKey
}

// modelEnv points the eval's Claude Code at the fake model (t1) or the gateway (t2), always in
// its private config dir so the user's own Claude Code and login are untouched.
func (d Claude) modelEnv(env *Env) []string {
	e := []string{"CLAUDE_CONFIG_DIR=" + d.home(env), "ANTHROPIC_API_KEY=" + d.apiKey(env)}
	if env.Tier == "t2" {
		e = append(e, "ANTHROPIC_BASE_URL="+gatewayClaude, "ANTHROPIC_MODEL="+LiveModel("claude"), "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
		if !strings.HasPrefix(LiveModel("claude"), "anthropic/") {
			// Anthropic-only beta headers and tool-schema fields; other providers reject them (Vercel docs).
			e = append(e, "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1")
		}
		return e
	}
	return append(e, "ANTHROPIC_BASE_URL="+env.LLMURL)
}
