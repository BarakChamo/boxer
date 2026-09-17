package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// OpenCode drives `opencode run`. t1 adds an OpenAI-compatible provider pointed at the fake model
// to the repository's opencode.json and pre-allows bash so the headless run never waits on a
// prompt. stdin must be closed: `opencode run` otherwise reads it as more prompt.
type OpenCode struct{}

func (OpenCode) Name() string { return "opencode" }

func (OpenCode) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("opencode"); err != nil {
		return false, "opencode not installed"
	}
	if tier == "t2" {
		if _, why := gateway(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (OpenCode) Cells(tier string) []Cell {
	h := "opencode"
	mk := func(mode string, compliant bool) Cell {
		return Cell{Harness: h, Mode: mode, Entry: "project", Isolation: "worktree", Compliant: compliant, Tier: tier}
	}
	return []Cell{mk("rewrite", true), mk("tool", true), mk("tool", false), mk("off", true)}
}

func (OpenCode) Prepare(env *Env, c Cell) error {
	if out, err := env.boxer(env.Repo, "install", "opencode"); err != nil {
		return fmt.Errorf("boxer install opencode: %v\n%s", err, out)
	}
	path := filepath.Join(env.Repo, "opencode.json")
	cfg := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &cfg)
	}
	cfg["permission"] = map[string]any{"bash": "allow", "edit": "allow", "webfetch": "allow", "external_directory": "allow"}
	// One OpenAI-compatible provider named "eval": the fake model at t1, the gateway (or OpenAI) at t2.
	p := openAICompatible{BaseURL: env.LLMURL + "/v1", Model: "fake-model"}
	key := "fake"
	if env.Tier == "t2" {
		p, _ = gateway()
		key = os.Getenv(p.KeyVar)
	}
	cfg["provider"] = map[string]any{"eval": map[string]any{
		"npm": "@ai-sdk/openai-compatible", "name": "eval",
		"options": map[string]any{"baseURL": p.BaseURL, "apiKey": key},
		"models":  map[string]any{p.Model: map[string]any{"name": p.Model, "tool_call": true}},
	}}
	cfg["model"] = "eval/" + p.Model
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func (OpenCode) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	model := "eval/fake-model"
	if env.Tier == "t2" {
		p, _ := gateway()
		model = "eval/" + p.Model
	}
	cmd := exec.Command("opencode", "run", "--print-logs", "-m", model, prompt)
	cmd.Dir = env.Repo
	cmd.Env = env.BaseEnv()
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
		return Transcript{Raw: out.String()}, fmt.Errorf("opencode timed out")
	}
	waitForHook("boxer hook opencode")
	tr := Transcript{Raw: out.String(), Tools: env.LLMTools()}
	// The final assistant text is the last non-empty line that is not a "$ command" echo.
	for _, line := range strings.Split(stripANSI(out.String()), "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "$ ") || strings.HasPrefix(l, "timestamp=") || strings.HasPrefix(l, ">") || strings.HasPrefix(l, "|") {
			continue
		}
		if f := strings.Fields(l); len(f) > 0 {
			tr.Answer = f[0]
		}
	}
	return tr, nil
}

func (OpenCode) Cleanup(env *Env, c Cell) {}

// waitForHook waits for a lingering hook process. OpenCode delivers `session.created` to plugins
// as a fire-and-forget event, so when the only tool call is denied the turn can end and `opencode
// run` exit while `boxer hook opencode` is still provisioning the VM; the oracle then finds no VM.
// The hook process outlives its parent, so waiting for it is enough. The eval lock guarantees no
// other harness run owns a hook process at the same time.
func waitForHook(pattern string) {
	for i := 0; i < 120; i++ {
		if exec.Command("pgrep", "-f", pattern).Run() != nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// stripANSI removes terminal colour codes from harness output.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
				j++
			}
			i = j // skip the final letter
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
