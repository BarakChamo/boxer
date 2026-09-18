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

// Copilot drives GitHub Copilot CLI headless. Its hooks load from ${COPILOT_HOME}/hooks/*.json at
// user scope: repository hooks under .github/hooks/ need a trusted working directory, which `-p`
// does not grant unless COPILOT_ALLOW_ALL is exactly "true", so the user layer is the only entry.
// Both tiers run through the BYOK provider variables (verified 2026-09-18 against 1.0.86: with
// COPILOT_PROVIDER_BASE_URL set the CLI never authenticates with GitHub at all), which points t1
// at fakellm's chat-completions wire and t2 at the gateway.
type Copilot struct{}

func (Copilot) Name() string { return "copilot" }

func (Copilot) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("copilot"); err != nil {
		return false, "copilot not installed (npm i -g @github/copilot)"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why + " (or a Copilot seat: COPILOT_GITHUB_TOKEN/GH_TOKEN)"
		}
	}
	return true, ""
}

func (Copilot) Cells(tier string) []Cell {
	mk := func(mode string, compliant bool) Cell {
		return Cell{Harness: "copilot", Mode: mode, Entry: "user", Isolation: "worktree", Compliant: compliant, Tier: tier}
	}
	cells := []Cell{mk("rewrite", true), mk("tool", true), mk("off", true)}
	if tier == "t1" {
		cells = append(cells, mk("tool", false))
	}
	return cells
}

func (Copilot) home(env *Env) string { return filepath.Join(env.Work, "copilot-home") }

func (d Copilot) Prepare(env *Env, c Cell) error {
	home := d.home(env)
	if err := os.MkdirAll(filepath.Join(home, "hooks"), 0o755); err != nil {
		return err
	}
	// Trust nothing else in the user's ~/.copilot: this home is fresh, so the install writes the
	// hook, the skill and the MCP entry into it exactly as it would for a real user.
	cmd := exec.Command(env.Boxer, "install", "copilot", "--user")
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(), "COPILOT_HOME="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("boxer install copilot --user: %v\n%s", err, out)
	}
	return nil
}

func (d Copilot) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	// COPILOT_ALLOW_ALL is exactly "true" so the fresh working directory is trusted without a
	// prompt; an untrusted directory denies every tool call with "Permission denied and could not
	// request permission from user", which is what a headless run in a fresh home hits first.
	// --allow-all-paths is needed for the control cell: Copilot verifies the paths a shell command
	// touches, and the canary under /tmp is outside the working directory.
	// --allow-all-tools is refused without a GitHub policy to read ("bypass-permissions mode
	// DISABLED by enterprise policy (fail-closed)"), which is exactly the BYOK case, so the cell
	// grants the three permission kinds it needs instead: every shell command, every tool of the
	// boxer MCP server, and file writes.
	args := []string{"-p", prompt, "-s", "--allow-tool", "shell", "--allow-tool", "boxer", "--allow-tool", "write", "--allow-all-paths",
		"--no-ask-user", "--output-format", "json", "--no-auto-update"}
	cmd := exec.Command("copilot", args...)
	cmd.Dir = env.Repo
	cmd.Env = append(append(env.BaseEnv(), "COPILOT_HOME="+d.home(env), "COPILOT_AUTO_UPDATE=false", "COPILOT_ALLOW_ALL=true"), d.provider(env)...)
	cmd.Stdin = strings.NewReader("")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := wait(cmd, "copilot"); err != nil {
		return Transcript{Raw: out.String()}, err
	}
	tr := parseCopilotJSON(out.String())
	if len(tr.Tools) == 0 {
		tr.Tools = env.LLMTools()
	}
	return tr, nil
}

// provider is the BYOK block: fakellm at t1, the gateway's coding-agent Chat Completions surface
// at t2. Auto-update is pinned off in both, because a background download would change the binary
// under the run.
func (d Copilot) provider(env *Env) []string {
	base, key, model := env.LLMURL+"/v1", "fake", "fake-model"
	if env.Tier == "t2" {
		base, key, model = gatewayOpenAI, os.Getenv(gatewayKeyVar), LiveModel("copilot")
	}
	return []string{
		"COPILOT_PROVIDER_TYPE=openai",
		"COPILOT_PROVIDER_BASE_URL=" + base,
		"COPILOT_PROVIDER_API_KEY=" + key,
		"COPILOT_MODEL=" + model,
	}
}

func (Copilot) Cleanup(env *Env, c Cell) {}

// parseCopilotJSON reads `--output-format json`: JSONL, one object per line with a `type` and a
// `data` payload. Tool calls arrive as tool.* events naming the tool; the answer is the last
// non-empty assistant text.
func parseCopilotJSON(s string) Transcript {
	tr := Transcript{Raw: s}
	var answer string
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e struct {
			Type string `json:"type"`
			Data struct {
				Name     string `json:"name"`
				ToolName string `json:"toolName"`
				Tool     string `json:"tool"`
				Text     string `json:"text"`
				Content  string `json:"content"`
				Message  string `json:"message"`
			} `json:"data"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch {
		case strings.HasPrefix(e.Type, "tool."):
			if name := firstNonEmpty(e.Data.ToolName, e.Data.Name, e.Data.Tool); name != "" && !strings.HasSuffix(e.Type, "_finished") && !strings.HasSuffix(e.Type, "_end") {
				tr.Tools = append(tr.Tools, name)
			}
		case strings.HasPrefix(e.Type, "assistant."), e.Type == "text":
			if t := firstNonEmpty(e.Data.Text, e.Data.Content, e.Data.Message); strings.TrimSpace(t) != "" {
				answer = t
			}
		}
	}
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	if f := strings.Fields(lines[len(lines)-1]); len(f) > 0 {
		tr.Answer = f[0]
	}
	return tr
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
