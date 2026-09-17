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

// Grok drives Grok Build headless. Hooks load from two places without a prompt: $GROK_HOME/hooks
// (user) and the repository's .grok/hooks with folder trust disabled (project). A plugin's hooks
// do not run in a headless session (verified 2026-09-17 on 1.0.34, both through `grok -p` with the
// plugin under $GROK_HOME/plugins and through `grok agent --plugin-dir` over ACP: the plugin's MCP
// server came up, its hooks never fired), so the plugin cell is the "both" cell, which proves the
// plugin's MCP server and the project hooks coexist. t1 uses a private GROK_HOME whose config.toml
// defines a BYOK model pointed at the fake model over the chat completions wire; a BYOK model
// needs no sign-in.
type Grok struct{}

func (Grok) Name() string { return "grok" }

func (Grok) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("grok"); err != nil {
		return false, "grok not installed"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (Grok) Cells(tier string) []Cell {
	h := "grok"
	mk := func(mode, entry string, compliant bool) Cell {
		return Cell{Harness: h, Mode: mode, Entry: entry, Isolation: "worktree", Compliant: compliant, Tier: tier}
	}
	cells := []Cell{
		mk("rewrite", "user", true),
		mk("rewrite", "project", true),
		mk("tool", "user", false),
		mk("off", "user", true),
	}
	if tier == "t1" {
		cells = append(cells, mk("rewrite", "both", true))
	} else {
		// Grok reaches MCP tools only through its search_tool/use_tool dispatcher, which the scripted
		// model cannot drive; a live model can, so the compliant tool cell is t2 only.
		cells = append(cells, mk("tool", "user", true))
	}
	return cells
}

func (Grok) home(env *Env) string { return filepath.Join(env.Work, "grok-home") }

func (d Grok) Prepare(env *Env, c Cell) error {
	home := d.home(env)
	for _, sub := range []string{"hooks", "plugins"} {
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			return err
		}
	}
	cfg := "[features]\ntelemetry = false\n[mcp_servers.boxer]\ncommand = \"boxer\"\nargs = [\"mcp\", \"--harness\", \"grok\"]\n"
	cfg += grokModel(env)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	dist := filepath.Join(env.Dist, "grok")
	switch c.Entry {
	case "user":
		b, err := os.ReadFile(filepath.Join(dist, "ai.x.grok", "hooks", "hooks.json")) // the Grok view of the Agent Plugins package
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(home, "hooks", "boxer.json"), b, 0o644); err != nil {
			return err
		}
	case "plugin", "both":
		if err := exec.Command("cp", "-R", dist, filepath.Join(home, "plugins", "boxer")).Run(); err != nil {
			return fmt.Errorf("copy plugin: %v", err)
		}
	}
	if c.Entry == "project" || c.Entry == "both" {
		if out, err := env.boxer(env.Repo, "install", "grok"); err != nil {
			return fmt.Errorf("boxer install grok: %v\n%s", err, out)
		}
	}
	return nil
}

func (d Grok) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	// A private leader socket: the default one is the user's own leader under ~/.grok, which
	// discovers plugins from the real home rather than from GROK_HOME.
	args := []string{"-p", prompt, "--permission-mode", "bypassPermissions", "--output-format", "streaming-json", "--max-turns", "6", "--no-subagents", "--leader-socket", filepath.Join(d.home(env), "leader-eval.sock")}
	if env.Tier == "t1" {
		args = append(args, "-m", "fake")
	}
	cmd := exec.Command("grok", args...)
	cmd.Dir = env.Repo
	// Folder trust gates project hooks and the project .mcp.json; disabling it is the headless
	// equivalent of --trust, which only the interactive session accepts.
	cmd.Env = append(env.BaseEnv(), "GROK_HOME="+d.home(env), "GROK_FOLDER_TRUST=0", "FAKE_LLM_KEY=fake")
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
		return Transcript{Raw: out.String()}, fmt.Errorf("grok timed out")
	}
	tr := parseGrokStream(out.String())
	if len(tr.Tools) == 0 {
		tr.Tools = env.LLMTools()
	}
	return tr, nil
}

func (Grok) Cleanup(env *Env, c Cell) {}

// parseGrokStream reads `--output-format streaming-json`: one event per line, `text` carrying the
// answer in `data` and `tool_call` naming the tool in `toolName`. MCP tools go through Grok's
// `use_tool` dispatcher, so the inner `rawInput.tool_name` is recorded as `use_tool:<name>`.
func parseGrokStream(s string) Transcript {
	tr := Transcript{Raw: s}
	var text strings.Builder
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var e struct {
			Type     string `json:"type"`
			Data     string `json:"data"`
			ToolName string `json:"toolName"`
			Title    string `json:"title"`
			RawInput struct {
				ToolName string `json:"tool_name"`
			} `json:"rawInput"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch e.Type {
		case "text":
			text.WriteString(e.Data)
		case "tool_call":
			if e.ToolName == "" {
				e.ToolName = e.Title
			}
			if e.ToolName == "use_tool" && e.RawInput.ToolName != "" {
				e.ToolName += ":" + e.RawInput.ToolName
			}
			tr.Tools = append(tr.Tools, e.ToolName)
		}
	}
	lines := strings.Split(strings.TrimSpace(text.String()), "\n")
	if f := strings.Fields(lines[len(lines)-1]); len(f) > 0 {
		tr.Answer = f[0]
	}
	return tr
}

// grokModel is the BYOK model block: the fake model at t1, the gateway at t2. Both run `grok -p`
// with no sign-in.
func grokModel(env *Env) string {
	if env.Tier == "t2" {
		return fmt.Sprintf("[models]\ndefault = \"live\"\n[model.live]\nmodel = %q\nname = \"live\"\nbase_url = %q\napi_backend = \"chat_completions\"\nenv_key = %q\ncontext_window = 128000\n", LiveModel("grok"), gatewayOpenAI, gatewayKeyVar)
	}
	return fmt.Sprintf("[models]\ndefault = \"fake\"\n[model.fake]\nmodel = \"fake-model\"\nname = \"fake\"\nbase_url = \"%s/v1\"\napi_backend = \"chat_completions\"\nenv_key = \"FAKE_LLM_KEY\"\ncontext_window = 128000\n", env.LLMURL)
}
