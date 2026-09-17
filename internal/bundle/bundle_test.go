package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
)

func read(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEveryHarnessRenders(t *testing.T) {
	hs := Harnesses()
	if len(hs) != 8 {
		t.Fatalf("expected 8 harness bundles, got %v", hs)
	}
	cfg := config.Defaults()
	for _, h := range hs {
		dir := filepath.Join(t.TempDir(), h)
		files, err := Render(h, cfg, "test", dir)
		if err != nil {
			t.Fatalf("%s: %v", h, err)
		}
		if len(files) == 0 {
			t.Fatalf("%s: nothing rendered", h)
		}
		var all strings.Builder
		for _, f := range files {
			all.WriteString(read(t, f))
		}
		s := all.String()
		if !strings.Contains(s, "boxer hook "+h) && h != "opencode" {
			t.Errorf("%s: no hook invocation for its own name", h)
		}
		if !strings.Contains(s, "boxer") || !strings.Contains(s, cfg.MountAt) {
			t.Errorf("%s: instruction text missing", h)
		}
		if h != "claude-code" && strings.Contains(strings.ToLower(s), "claude") {
			t.Errorf("%s bundle mentions Claude; bundles must be harness-neutral", h)
		}
	}
}

func TestClaudeCarriesShimsOnlyWhenEnforcementSaysSo(t *testing.T) {
	cfg := config.Defaults()
	dir := t.TempDir()
	files, _ := Render("claude-code", cfg, "t", dir)
	if _, err := os.Stat(filepath.Join(dir, "bin", "npm")); err != nil {
		t.Fatal("default enforcement=both must render bin/ shims")
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agents", "boxed.md")), "disallowedTools: [Bash]") {
		t.Fatal("boxed agent must remove Bash")
	}
	_ = files
	cfg.Enforcement = "hook"
	dir = t.TempDir()
	Render("claude-code", cfg, "t", dir)
	if _, err := os.Stat(filepath.Join(dir, "bin")); err == nil {
		t.Fatal("enforcement=hook must not render shims")
	}
}

func TestGeminiToolModeExcludesShell(t *testing.T) {
	cfg := config.Defaults()
	dir := t.TempDir()
	Render("gemini-cli", cfg, "t", dir)
	if strings.Contains(read(t, filepath.Join(dir, "gemini-extension.json")), "excludeTools") {
		t.Fatal("rewrite mode must keep run_shell_command")
	}
	cfg.Mode = "tool"
	dir = t.TempDir()
	Render("gemini-cli", cfg, "t", dir)
	m := read(t, filepath.Join(dir, "gemini-extension.json"))
	if !strings.Contains(m, `"excludeTools": ["run_shell_command"]`) {
		t.Fatalf("tool mode must exclude the shell tool:\n%s", m)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "GEMINI.md")), "boxer_run") {
		t.Fatal("tool-mode instruction must name the tool")
	}
}

func TestUnknownHarness(t *testing.T) {
	if _, err := Render("emacs", config.Defaults(), "t", t.TempDir()); err == nil {
		t.Fatal("unknown harness must fail")
	}
}

func TestVersionIsStampedIntoManifestsAndSkill(t *testing.T) {
	cfg := config.Defaults()
	for h, files := range map[string][]string{
		"claude-code": {".claude-plugin/plugin.json", "skills/boxer/SKILL.md"},
		"codex":       {".codex-plugin/plugin.json"},
		"grok":        {".grok-plugin/plugin.json"},
		"gemini-cli":  {"gemini-extension.json", "GEMINI.md"},
		"opencode":    {"AGENTS.md"},
		"pi":          {"AGENTS.md"},
		"kimi":        {".agents/skills/boxer/SKILL.md"},
	} {
		dir := t.TempDir()
		if _, err := Render(h, cfg, "0.9.1-test", dir); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if !strings.Contains(read(t, filepath.Join(dir, f)), "0.9.1-test") {
				t.Errorf("%s/%s: version not stamped", h, f)
			}
		}
	}
}
