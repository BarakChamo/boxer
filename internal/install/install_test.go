package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BurntSushi/toml"
)

func readJSONT(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return m
}

func TestClaudeProjectInstallMergesAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["Read"]},"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"fmt"}]}]}}`), 0o644)
	os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o644)

	cfg := config.Defaults()
	for i := 0; i < 2; i++ {
		if _, err := Install("claude-code", cfg, "t", root); err != nil {
			t.Fatal(err)
		}
	}
	s := readJSONT(t, filepath.Join(root, ".claude", "settings.json"))
	if s["permissions"] == nil {
		t.Fatal("existing keys must survive")
	}
	pre := s["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 2 {
		t.Fatalf("PreToolUse should have the existing group plus one boxer group, got %d", len(pre))
	}
	if _, ok := s["hooks"].(map[string]any)["SessionStart"]; !ok {
		t.Fatal("SessionStart hook missing")
	}
	m := readJSONT(t, filepath.Join(root, ".mcp.json"))
	servers := m["mcpServers"].(map[string]any)
	if servers["other"] == nil || servers["boxer"] == nil {
		t.Fatalf("mcp merge: %v", servers)
	}
	for _, p := range []string{".claude/skills/boxer/SKILL.md", ".claude/agents/boxed.md"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Fatalf("%s not written", p)
		}
	}
}

func TestGeminiToolModeExcludesShell(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "GEMINI.md"), []byte("# Project\n"), 0o644)
	cfg := config.Defaults()
	cfg.Mode = "tool"
	for i := 0; i < 2; i++ {
		if _, err := Install("gemini-cli", cfg, "t", root); err != nil {
			t.Fatal(err)
		}
	}
	s := readJSONT(t, filepath.Join(root, ".gemini", "settings.json"))
	ex := s["tools"].(map[string]any)["exclude"].([]any)
	if len(ex) != 1 || ex[0] != "run_shell_command" {
		t.Fatalf("exclude: %v", ex)
	}
	if s["mcpServers"].(map[string]any)["boxer"] == nil {
		t.Fatal("mcp missing")
	}
	g, _ := os.ReadFile(filepath.Join(root, "GEMINI.md"))
	if strings.Count(string(g), "# boxer sandbox") != 1 || !strings.HasPrefix(string(g), "# Project") {
		t.Fatalf("GEMINI.md append once:\n%s", g)
	}
}

func TestOpenCodeAndCodex(t *testing.T) {
	root := t.TempDir()
	if _, err := Install("opencode", config.Defaults(), "t", root); err != nil {
		t.Fatal(err)
	}
	oc := readJSONT(t, filepath.Join(root, "opencode.json"))
	if oc["mcp"].(map[string]any)["boxer"] == nil {
		t.Fatal("opencode mcp missing")
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "plugins", "boxer.ts")); err != nil {
		t.Fatal(err)
	}
	r, err := Install("codex", config.Defaults(), "t", root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".codex", "hooks.json")); err != nil {
		t.Fatal(err)
	}
	if len(r.Notes) == 0 || !strings.Contains(strings.Join(r.Notes, "\n"), "trust") {
		t.Fatal("codex install must explain hook trust")
	}
}

func TestRefusesNonObjectJSON(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("[1,2]"), 0o644)
	if _, err := Install("claude-code", config.Defaults(), "t", root); err == nil {
		t.Fatal("must not clobber a non-object settings file")
	}
}

func TestKimiAndDSH(t *testing.T) {
	root := t.TempDir()
	r, err := Install("kimi", config.Defaults(), "t", root)
	if err != nil {
		t.Fatal(err)
	}
	m := readJSONT(t, filepath.Join(root, ".kimi-code", "mcp.json"))
	if m["mcpServers"].(map[string]any)["boxer"] == nil {
		t.Fatal("kimi mcp missing")
	}
	if !strings.Contains(strings.Join(r.Notes, "\n"), "[[hooks]]") {
		t.Fatal("kimi install must hand over the TOML hooks snippet")
	}
	if _, err := Install("dsh", config.Defaults(), "t", root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dsh", "hooks.json")); err != nil {
		t.Fatal(err)
	}
}

func TestUserLevelInstall(t *testing.T) {
	claude := t.TempDir()
	codex := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	t.Setenv("CODEX_HOME", codex)
	os.WriteFile(filepath.Join(codex, "config.toml"), []byte("model = \"gpt-5\"\n[features]\nhooks = true\n"), 0o644)
	cfg := config.Defaults()
	for i := 0; i < 2; i++ {
		if _, err := User("claude-code", cfg, "t"); err != nil {
			t.Fatal(err)
		}
		if _, err := User("codex", cfg, "t"); err != nil {
			t.Fatal(err)
		}
	}
	s := readJSONT(t, filepath.Join(claude, "settings.json"))
	if len(s["hooks"].(map[string]any)["PreToolUse"].([]any)) != 1 {
		t.Fatal("claude user hooks must be added once")
	}
	b, _ := os.ReadFile(filepath.Join(codex, "config.toml"))
	var parsed map[string]any
	if _, err := toml.Decode(string(b), &parsed); err != nil {
		t.Fatalf("config.toml must stay valid TOML: %v\n%s", err, b)
	}
	if parsed["model"] != "gpt-5" {
		t.Fatal("existing config must survive")
	}
	pre := parsed["hooks"].(map[string]any)["PreToolUse"].([]map[string]any)
	if len(pre) != 1 || pre[0]["matcher"] != "Bash" {
		t.Fatalf("inline hooks: %v", parsed["hooks"])
	}
	if strings.Count(string(b), blockStart) != 1 {
		t.Fatal("block must be replaced, not appended twice")
	}
	if _, err := User("gemini-cli", cfg, "t"); err == nil {
		t.Fatal("no user layer for gemini")
	}
}

func TestUnknownHarness(t *testing.T) {
	if _, err := Install("emacs", config.Defaults(), "t", t.TempDir()); err == nil {
		t.Fatal("unknown harness must error with guidance")
	}
}
