package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Pi drives pi in print mode with the boxer extension loaded via -e. pi has no config-dir
// variable, so t1 runs it under a private HOME holding models.json; smolvm's own data lives under
// the real HOME, so that directory is linked into the private one.
type Pi struct{}

func (Pi) Name() string { return "pi" }

func (Pi) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("pi"); err != nil {
		return false, "pi not installed"
	}
	if tier == "t2" {
		if _, why := gateway("pi"); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (Pi) Cells(tier string) []Cell {
	h := "pi"
	mk := func(mode string, compliant bool) Cell {
		return Cell{Harness: h, Mode: mode, Entry: "project", Isolation: "worktree", Compliant: compliant, Tier: tier}
	}
	return []Cell{mk("rewrite", true), mk("tool", true), mk("tool", false), mk("off", true)}
}

func (Pi) home(env *Env) string { return filepath.Join(env.Work, "pi-home") }

func (d Pi) Prepare(env *Env, c Cell) error {
	if out, err := env.boxer(env.Repo, "install", "pi"); err != nil {
		return fmt.Errorf("boxer install pi: %v\n%s", err, out)
	}
	home := d.home(env)
	if err := os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0o755); err != nil {
		return err
	}
	// One provider named "eval": the fake model over Anthropic Messages at t1; the gateway (or
	// OpenAI) over chat completions at t2.
	models := fmt.Sprintf(`{"providers":{"eval":{"baseUrl":%q,"api":"anthropic-messages","apiKey":"fake","models":[{"id":"fake-model","name":"fake","contextWindow":200000,"maxTokens":8192,"input":["text"],"reasoning":false}]}}}`, env.LLMURL)
	if env.Tier == "t2" {
		p, _ := gateway("pi")
		models = fmt.Sprintf(`{"providers":{"eval":{"baseUrl":%q,"api":"openai-completions","apiKey":%q,"models":[{"id":%q,"name":%q,"contextWindow":200000,"maxTokens":8192,"input":["text"],"reasoning":false}]}}}`, p.BaseURL, os.Getenv(p.KeyVar), p.Model, p.Model)
	}
	if err := os.WriteFile(filepath.Join(home, ".pi", "agent", "models.json"), []byte(models+"\n"), 0o644); err != nil {
		return err
	}
	// boxer and its hooks run inside pi's private HOME, but smolvm's state must stay in the real
	// one, or it fights the real machine over the same disks. A wrapper restores HOME for smolvm.
	real, _ := os.UserHomeDir()
	smolvm, err := exec.LookPath("smolvm")
	if err != nil {
		return fmt.Errorf("smolvm not on PATH: %v", err)
	}
	wrapper := filepath.Join(env.Work, "smolvm-realhome")
	script := fmt.Sprintf("#!/bin/sh\nHOME=%q exec %q \"$@\"\n", real, smolvm)
	return os.WriteFile(wrapper, []byte(script), 0o755)
}

func (d Pi) smolvmWrapper(env *Env) string { return filepath.Join(env.Work, "smolvm-realhome") }

func (d Pi) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	model := "fake-model"
	if env.Tier == "t2" {
		p, _ := gateway("pi")
		model = p.Model
	}
	args := []string{"--provider", "eval", "--model", model, "-p", "--no-session", "-e", ".pi/extensions/boxer.ts", prompt}
	cmd := exec.Command("pi", args...)
	cmd.Dir = env.Repo
	// pi has no config-dir variable, so both tiers run under the private HOME.
	cmd.Env = append(env.BaseEnv(), "PI_SKIP_VERSION_CHECK=1", "PI_TELEMETRY=0", "HOME="+d.home(env), "BOXER_SMOLVM="+d.smolvmWrapper(env))
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := wait(cmd, "pi"); err != nil {
		return Transcript{Raw: out.String()}, err
	}
	tr := Transcript{Raw: out.String(), Tools: env.LLMTools()}
	tr.Answer = lastAnswer(out.String(), "boxer", "scope:", "worktree:", "cause:", "fix:")
	return tr, nil
}

func (Pi) Cleanup(env *Env, c Cell) {}
