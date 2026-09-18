package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Kimi drives Kimi Code CLI. Its hooks are user-level TOML and block-only (updatedInput is not
// honoured, and its Bash tool does not see a prepended PATH, so shims cannot help either: both
// verified 2026-09-17). The hook denies in tool mode and boxer_run is the way in. t1 uses a private KIMI_CODE_HOME with an Anthropic-type provider pointed at the fake
// model; the run tool is a user-level mcp.json because project MCP needs folder trust.
type Kimi struct{}

func (Kimi) Name() string { return "kimi" }

func (Kimi) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("kimi"); err != nil {
		return false, "kimi not installed"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (Kimi) Cells(tier string) []Cell {
	h := "kimi"
	// Kimi cannot rewrite tool input and its Bash tool ignores a prepended PATH, so neither the hook
	// nor shims can put a bare command in the guest (verified 2026-09-17). Kimi is tool-mode only.
	only := []string{"uname"}
	return []Cell{
		{Harness: h, Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: true, Tier: tier, Intercept: only},
		{Harness: h, Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: false, Tier: tier, Intercept: only},
		{Harness: h, Mode: "off", Entry: "user", Isolation: "worktree", Compliant: true, Tier: tier, Intercept: only},
	}
}

func (Kimi) home(env *Env) string { return filepath.Join(env.Work, "kimi-home") }

func (d Kimi) Prepare(env *Env, c Cell) error {
	home := d.home(env)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	cfg := kimiConfig("anthropic", env.LLMURL, "fake", "fake-model")
	if env.Tier == "t2" {
		key, _ := gatewayKey()
		cfg = kimiConfig("anthropic", gatewayRoot, key, LiveModel("kimi"))
	}
	// boxer install kimi prints the [[hooks]] snippet; append it to the private config.
	out, err := env.boxer(env.Repo, "install", "kimi")
	if err != nil {
		return fmt.Errorf("boxer install kimi: %v\n%s", err, out)
	}
	if i := strings.Index(out, "[[hooks]]"); i >= 0 {
		snippet := out[i:]
		if j := strings.Index(snippet, "\n  note:"); j >= 0 {
			snippet = snippet[:j]
		}
		var lines []string
		for _, l := range strings.Split(snippet, "\n") {
			lines = append(lines, strings.TrimLeft(l, " "))
		}
		cfg += "\n" + strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	// Run tool at the user level; the project-level one is skipped in untrusted folders.
	if b, err := os.ReadFile(filepath.Join(env.Repo, ".kimi-code", "mcp.json")); err == nil {
		os.WriteFile(filepath.Join(home, "mcp.json"), b, 0o644)
	}
	if c.Shims {
		if out, err := env.boxer(env.Repo, "shim", "install", filepath.Join(env.Work, "shims")); err != nil {
			return fmt.Errorf("boxer shim install: %v\n%s", err, out)
		}
	}
	return nil
}

func (d Kimi) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	cmd := exec.Command("kimi", "-p", prompt)
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(), "KIMI_CODE_HOME="+d.home(env))
	if c.Shims {
		cmd.Env = prependPath(cmd.Env, filepath.Join(env.Work, "shims"))
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := wait(cmd, "kimi"); err != nil {
		return Transcript{Raw: out.String()}, err
	}
	tr := Transcript{Raw: out.String(), Tools: env.LLMTools()}
	// The final assistant line is printed as "• <text>".
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "• ") {
			if f := strings.Fields(strings.TrimPrefix(line, "• ")); len(f) > 0 {
				tr.Answer = f[0]
			}
		}
	}
	return tr, nil
}

func (Kimi) Cleanup(env *Env, c Cell) {}

// kimiConfig is a private KIMI_CODE_HOME config with one provider and one model named "eval".
func kimiConfig(typ, baseURL, key, model string) string {
	return fmt.Sprintf(`default_model = "eval"
default_permission_mode = "auto"
telemetry = false
[providers.eval]
type = %q
api_key = %q
base_url = %q
[models.eval]
provider = "eval"
model = %q
max_context_size = 131072
`, typ, key, baseURL, model) // 131072: Kimi derives max_tokens from this, and gateway flash models cap output there
}

// prependPath puts dir first on PATH inside an environment slice.
func prependPath(env []string, dir string) []string {
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + dir + ":" + strings.TrimPrefix(kv, "PATH=")
			return env
		}
	}
	return append(env, "PATH="+dir)
}
