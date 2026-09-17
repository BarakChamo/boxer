package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ecma compiles schema patterns with regexp2: the spec's name pattern uses a lookahead that
// Go's RE2 does not support.
func ecma(p string) (jsonschema.Regexp, error) {
	re, err := regexp2.Compile(p, regexp2.ECMAScript)
	return ecmaRegexp{re}, err
}

type ecmaRegexp struct{ *regexp2.Regexp }

func (r ecmaRegexp) MatchString(s string) bool { ok, _ := r.Regexp.MatchString(s); return ok }

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
		t.Fatalf("expected 8 harness views, got %v", hs)
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
		if !strings.Contains(s, "boxer hook "+h) {
			t.Errorf("%s: no hook invocation for its own name", h)
		}
		if !strings.Contains(s, "boxer") || !strings.Contains(s, cfg.MountAt) {
			t.Errorf("%s: instruction text missing", h)
		}
		if h != "claude-code" && strings.Contains(strings.ToLower(s), "claude") {
			t.Errorf("%s bundle mentions Claude; bundles must be harness-neutral", h)
		}
		for _, core := range []string{"plugin.json", "mcp.json", "skills/boxer/SKILL.md", Namespace(h) + "/README.md"} {
			if _, err := os.Stat(filepath.Join(dir, core)); err != nil {
				t.Errorf("%s: view lacks %s", h, core)
			}
		}
	}
}

// Every view is a subset of the package: same relative paths, same bytes.
func TestViewsAreSubsetsOfThePackage(t *testing.T) {
	cfg := config.Defaults()
	pkg := t.TempDir()
	if _, err := Render(Package, cfg, "t", pkg); err != nil {
		t.Fatal(err)
	}
	for _, h := range Harnesses() {
		dir := t.TempDir()
		files, _ := Render(h, cfg, "t", dir)
		for _, f := range files {
			rel, _ := filepath.Rel(dir, f)
			src := rel
			if a := views[h].Alias[rel]; a != "" {
				src = a
			}
			if read(t, f) != read(t, filepath.Join(pkg, src)) {
				t.Errorf("%s: %s differs from the package's %s", h, rel, src)
			}
		}
	}
}

// R-LVL-6a: the portable manifests conform to the Agent Plugins 1.0.0 schemas (vendored in spec/).
func TestManifestsConformToAgentPluginsSchemas(t *testing.T) {
	dir := t.TempDir()
	if _, err := Render(Package, config.Defaults(), "0.1.0", dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin", "mcp"} {
		c := jsonschema.NewCompiler()
		c.UseRegexpEngine(ecma)
		sch, err := c.Compile(filepath.Join("spec", f+".schema.json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal([]byte(read(t, filepath.Join(dir, f+".json"))), &doc); err != nil {
			t.Fatal(err)
		}
		if err := sch.Validate(doc); err != nil {
			t.Errorf("%s.json does not conform: %v", f, err)
		}
	}
	// Spec §4.2/§8.2: the portable core plus reverse-domain client directories, nothing else at
	// the top level except the native manifests each loader still reads.
	for _, h := range Harnesses() {
		if _, err := os.Stat(filepath.Join(dir, Namespace(h))); err != nil {
			t.Errorf("package lacks extension directory %s", Namespace(h))
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "hooks")); err == nil {
		t.Error("package must not carry a root hooks/ directory (Claude Code auto-loads it; Gemini's keys break it)")
	}
}

func TestClaudeCarriesShimsOnlyWhenEnforcementSaysSo(t *testing.T) {
	cfg := config.Defaults()
	dir := t.TempDir()
	Render("claude-code", cfg, "t", dir)
	if _, err := os.Stat(filepath.Join(dir, "bin", "npm")); err != nil {
		t.Fatal("default enforcement=both must render bin/ shims")
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agents", "boxed.md")), "disallowedTools: [Bash]") {
		t.Fatal("boxed agent must remove Bash")
	}
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
	if !strings.Contains(read(t, filepath.Join(dir, "hooks", "hooks.json")), "BeforeTool") {
		t.Fatal("gemini view must copy its hooks to the fixed hooks/hooks.json path")
	}
	cfg.Mode = "tool"
	dir = t.TempDir()
	Render("gemini-cli", cfg, "t", dir)
	m := read(t, filepath.Join(dir, "gemini-extension.json"))
	if !strings.Contains(m, `"excludeTools": ["run_shell_command"]`) {
		t.Fatalf("tool mode must exclude the shell tool:\n%s", m)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "AGENTS.md")), "boxer_run") {
		t.Fatal("tool-mode instruction must name the tool")
	}
}

func TestUnknownHarness(t *testing.T) {
	if _, err := Render("emacs", config.Defaults(), "t", t.TempDir()); err == nil {
		t.Fatal("unknown harness must fail")
	}
}

func TestVersionIsStampedIntoManifestsAndSkill(t *testing.T) {
	dir := t.TempDir()
	if _, err := Render(Package, config.Defaults(), "0.9.1-test", dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.json", ".claude-plugin/plugin.json", ".codex-plugin/plugin.json", ".grok-plugin/plugin.json", "gemini-extension.json", "skills/boxer/SKILL.md", "AGENTS.md"} {
		if !strings.Contains(read(t, filepath.Join(dir, f)), "0.9.1-test") {
			t.Errorf("%s: version not stamped", f)
		}
	}
}
