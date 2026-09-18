package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DSH drives the DeepSeek Harness. `dsh --profile headless "<task>"` is its one-shot mode: the
// final answer on stdout, provider reasoning on stderr, exit 0 when the turn completed. DSH reads
// no project-level plugin config, so boxer's MCP row and its hook bridge ride in the patch layer
// `boxer install dsh` writes, and a second patch points the provider at the model under test.
// Hooks exist only through @deepseek-ai/dsh-hooks-claude-code, which ignores updatedInput, so DSH
// is tool-mode only (verified 2026-09-18 against 0.1.5-rc.2).
type DSH struct{}

func (DSH) Name() string { return "dsh" }

func (DSH) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("dsh"); err != nil {
		return false, "dsh not installed"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (DSH) Cells(tier string) []Cell {
	h := "dsh"
	// dsh is a node program; a shim cell must not shadow its own runtime, so intercept uname alone.
	only := []string{"uname"}
	return []Cell{
		{Harness: h, Mode: "tool", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier, Intercept: only},
		{Harness: h, Mode: "tool", Entry: "project", Isolation: "worktree", Compliant: false, Tier: tier, Intercept: only},
		{Harness: h, Mode: "off", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier, Intercept: only},
	}
}

func (DSH) home(env *Env) string  { return filepath.Join(env.Work, "dsh-home") }
func (DSH) patch(env *Env) string { return filepath.Join(env.Work, "dsh-llm.patch.yml") }

func (d DSH) Prepare(env *Env, c Cell) error {
	if err := os.MkdirAll(d.home(env), 0o755); err != nil {
		return err
	}
	if out, err := env.boxer(env.Repo, "install", "dsh"); err != nil {
		return fmt.Errorf("boxer install dsh: %v\n%s", err, out)
	}
	base, model := env.LLMURL, "fake-model"
	if env.Tier == "t2" {
		base, model = gatewayOpenAI, LiveModel("dsh")
	}
	if err := os.WriteFile(d.patch(env), []byte(dshLLMPatch(base, model)), 0o644); err != nil {
		return err
	}
	if c.Shims {
		if out, err := env.boxer(env.Repo, "shim", "install", filepath.Join(env.Work, "shims")); err != nil {
			return fmt.Errorf("boxer shim install: %v\n%s", err, out)
		}
	}
	return nil
}

func (d DSH) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	cmd := exec.Command("dsh", "--profile", "headless",
		"--patch", filepath.Join(env.Repo, ".dsh", "cordis.patch.yml"), "--patch", d.patch(env), prompt)
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(),
		"DSH_HOME="+d.home(env),
		"DSH_TELEMETRY_MODE=DISABLED",
		// Headless has nobody to approve a tool call, and boxer's own `boxer run` starts a VM
		// outside the workspace the default sandbox confines the shell to.
		"DSH_PERMISSION_MODE=danger-full-access",
		"BOXER_EVAL_FAKE_KEY=fake",
	)
	if c.Shims {
		cmd.Env = prependPath(cmd.Env, filepath.Join(env.Work, "shims"))
	}
	var answer, all bytes.Buffer
	cmd.Stdout = &answer
	cmd.Stderr = &all
	if err := wait(cmd, "dsh"); err != nil {
		return Transcript{Raw: all.String() + answer.String()}, err
	}
	tr := Transcript{Raw: all.String() + "\n--- stdout ---\n" + answer.String(), Tools: env.LLMTools()}
	// Headless prints the final answer, and only that, on stdout.
	for _, line := range strings.Split(answer.String(), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			tr.Answer = f[0]
		}
	}
	return tr, nil
}

func (DSH) Cleanup(env *Env, c Cell) {}

// dshLLMPatch is a profile patch that gives dsh one hand-declared OpenAI-compatible route and
// makes fresh agents start on it. A hand-declared route needs api, baseURL and a models list.
func dshLLMPatch(baseURL, model string) string {
	key := "BOXER_EVAL_FAKE_KEY"
	if strings.HasPrefix(baseURL, "https://") {
		key = gatewayKeyVar
	}
	return fmt.Sprintf(`- id: llm-pi-ai
  config:
    providers:
      eval:
        api: openai-completions
        baseURL: %s
        apiKeyEnv: %s
        models:
          - id: %s
            contextWindow: 131072
- id: agent-default-model
  config:
    provider: eval
    model: %s
`, baseURL, key, model, model)
}
