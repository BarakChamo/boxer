package eval

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Gemini drives Gemini CLI headless. t1 uses a private GEMINI_CLI_HOME whose settings select
// API-key auth, installs the rendered extension there, points the API at the fake model through
// GOOGLE_GEMINI_BASE_URL, and grants workspace trust. Gemini is a node program, so shims are never
// used for it.
type Gemini struct{}

func (Gemini) Name() string { return "gemini-cli" }

func (Gemini) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("gemini"); err != nil {
		return false, "gemini not installed"
	}
	if tier == "t2" {
		if why := needOne("GEMINI_API_KEY", "GOOGLE_API_KEY"); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (Gemini) Cells(tier string) []Cell {
	h := "gemini-cli"
	mk := func(mode, entry string, compliant bool) Cell {
		return Cell{Harness: h, Mode: mode, Entry: entry, Isolation: "worktree", Compliant: compliant, Tier: tier}
	}
	return []Cell{
		mk("rewrite", "plugin", true),
		mk("rewrite", "project", true),
		mk("tool", "plugin", true),
		mk("tool", "plugin", false),
		mk("off", "plugin", true),
	}
}

func (Gemini) home(env *Env) string { return filepath.Join(env.Work, "gemini-home") }

func (d Gemini) Prepare(env *Env, c Cell) error {
	// A private home at both tiers: API-key auth selected, no update nags; t2 supplies a real key.
	home := d.home(env)
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		return err
	}
	settings := `{"security":{"auth":{"selectedType":"gemini-api-key"}},"general":{"disableAutoUpdate":true,"disableUpdateNag":true},"privacy":{"usageStatisticsEnabled":false}}` + "\n"
	if err := os.WriteFile(filepath.Join(home, ".gemini", "settings.json"), []byte(settings), 0o644); err != nil {
		return err
	}
	switch c.Entry {
	case "plugin":
		cmd := exec.Command("gemini", "extensions", "install", "--consent", filepath.Join(env.Dist, "gemini-cli"))
		cmd.Dir = env.Repo
		cmd.Env = d.envFor(env)
		cmd.Stdin = strings.NewReader(strings.Repeat("y\n", 8))
		if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "already installed") {
			return fmt.Errorf("gemini extensions install: %v\n%s", err, out)
		}
	case "project":
		if out, err := env.boxer(env.Repo, "install", "gemini-cli"); err != nil {
			return fmt.Errorf("boxer install gemini-cli: %v\n%s", err, out)
		}
	}
	return nil
}

func (d Gemini) envFor(env *Env) []string {
	e := append(env.BaseEnv(), "GEMINI_CLI_TRUST_WORKSPACE=true", "GEMINI_CLI_HOME="+d.home(env))
	if env.Tier == "t1" {
		e = append(e, "GEMINI_API_KEY=fake", "GOOGLE_GEMINI_BASE_URL="+env.LLMURL)
	} else if k, ok := anySet("GEMINI_API_KEY", "GOOGLE_API_KEY"); ok {
		e = append(e, "GEMINI_API_KEY="+os.Getenv(k))
	}
	return e
}

func (d Gemini) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	cmd := exec.Command("gemini", "-p", prompt, "--yolo")
	cmd.Dir = env.Repo
	cmd.Env = d.envFor(env)
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
		return Transcript{Raw: out.String()}, fmt.Errorf("gemini timed out")
	}
	tr := Transcript{Raw: out.String(), Tools: env.LLMTools()}
	for _, line := range strings.Split(stripANSI(out.String()), "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "<") || strings.HasPrefix(l, "YOLO") || strings.HasPrefix(l, "[") || strings.HasPrefix(l, "Warning") || strings.HasPrefix(l, "Approval") {
			continue
		}
		if f := strings.Fields(l); len(f) > 0 {
			tr.Answer = f[0]
		}
	}
	return tr, nil
}

func (Gemini) Cleanup(env *Env, c Cell) {}
