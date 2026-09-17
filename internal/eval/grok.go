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

// Grok drives Grok Build headless. Hooks load from three places without a prompt: $GROK_HOME/hooks
// (user), the repository's .grok/hooks with folder trust disabled (project), and $GROK_HOME/plugins
// (plugin, auto-trusted). t1 uses a private GROK_HOME whose config.toml defines a BYOK model
// pointed at the fake model over the chat completions wire; no sign-in is needed for a BYOK model.
type Grok struct{}

func (Grok) Name() string { return "grok" }

func (Grok) Available(tier string) (bool, string) {
	if _, err := exec.LookPath("grok"); err != nil {
		return false, "grok not installed"
	}
	if tier == "t2" && os.Getenv("XAI_API_KEY") == "" {
		return false, "XAI_API_KEY is not set (grok is not signed in on this machine)"
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
		cells = append(cells, mk("rewrite", "plugin", true), mk("rewrite", "both", true))
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
	if env.Tier == "t1" {
		cfg += fmt.Sprintf("[models]\ndefault = \"fake\"\n[model.fake]\nmodel = \"fake-model\"\nname = \"fake\"\nbase_url = \"%s/v1\"\napi_backend = \"chat_completions\"\nenv_key = \"FAKE_LLM_KEY\"\ncontext_window = 128000\n", env.LLMURL)
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	dist := filepath.Join(env.Dist, "grok")
	switch c.Entry {
	case "user":
		b, err := os.ReadFile(filepath.Join(dist, "hooks", "hooks.json"))
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
	if c.Entry == "plugin" {
		return d.runACP(env, c, prompt)
	}
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

// runACP drives `grok agent --no-leader --plugin-dir <bundle> stdio` with the eval's ACP client:
// the documented headless way to load a plugin, since a leader-backed session (`grok -p`) discovers
// plugins from the real home only.
func (d Grok) runACP(env *Env, c Cell, prompt string) (Transcript, error) {
	args := []string{"agent", "--no-leader", "--plugin-dir", filepath.Join(env.Dist, "grok"), "--always-approve", "stdio"}
	if env.Tier == "t1" {
		args = append([]string{"-m", "fake"}, args...)
	}
	cmd := exec.Command("grok", args...)
	cmd.Dir = env.Repo
	cmd.Env = append(env.BaseEnv(), "GROK_HOME="+d.home(env), "GROK_FOLDER_TRUST=0", "FAKE_LLM_KEY=fake")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	defer cmd.Process.Kill()
	cl := &acpClient{w: stdin, r: bufio.NewReaderSize(stdout, 1<<20), repo: env.Repo}
	done := make(chan error, 1)
	go func() { done <- cl.session(prompt) }()
	select {
	case err := <-done:
		tr := Transcript{Raw: cl.log.String() + "\n--- stderr ---\n" + stderr.String(), Tools: cl.tools}
		if len(tr.Tools) == 0 {
			tr.Tools = env.LLMTools()
		}
		lines := strings.Split(strings.TrimSpace(cl.text.String()), "\n")
		if f := strings.Fields(lines[len(lines)-1]); len(f) > 0 {
			tr.Answer = f[0]
		}
		return tr, err
	case <-time.After(4 * time.Minute):
		return Transcript{Raw: cl.log.String() + stderr.String()}, fmt.Errorf("grok agent timed out")
	}
}

// parseGrokStream reads `--output-format streaming-json`: one event per line, `text` carrying the
// answer in `data` and `tool_call` naming the tool in `toolName`.
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
			tr.Tools = append(tr.Tools, e.ToolName)
		}
	}
	lines := strings.Split(strings.TrimSpace(text.String()), "\n")
	if f := strings.Fields(lines[len(lines)-1]); len(f) > 0 {
		tr.Answer = f[0]
	}
	return tr
}
