package bundle

import (
	"crypto/sha256"
	"encoding/hex"
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
	if len(hs) != 9 {
		t.Fatalf("expected 9 harness views, got %v", hs)
	}
	for _, h := range hs {
		dir := filepath.Join(t.TempDir(), h)
		files, err := Render(h, "test", dir)
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
		if !strings.Contains(s, "boxer brief") {
			t.Errorf("%s: nothing points the agent at the run-time brief", h)
		}
		// dsh is the exception: its only hook mechanism is the shipped compatibility bridge
		// @deepseek-ai/dsh-hooks-claude-code, so its README has to name it.
		if h != "claude-code" && h != "dsh" && strings.Contains(strings.ToLower(s), "claude") {
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
	pkg := t.TempDir()
	if _, err := Render(Package, "t", pkg); err != nil {
		t.Fatal(err)
	}
	for _, h := range Harnesses() {
		dir := t.TempDir()
		files, _ := Render(h, "t", dir)
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
	if _, err := Render(Package, "0.1.0", dir); err != nil {
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

// The bug this package was rewritten to kill: published content used to be rendered from the
// packager's own resolved configuration, so the release artifact carried one person's mode,
// intercept list and mount path into everybody else's repository. Two hostile configurations and
// a hostile environment must not move a single byte.
func TestPackageIsConfigIndependent(t *testing.T) {
	hostile := []struct {
		toml string
		env  map[string]string
	}{
		{"mode = \"tool\"\nenforcement = \"hook\"\nmount_at = \"/src\"\nintercept = [\"make\"]\npassthrough = [\"git\"]\n\n[harness.claude-code]\nmode = \"off\"\n",
			map[string]string{"BOXER_MODE": "off", "BOXER_MOUNT_AT": "/elsewhere", "BOXER_ENFORCEMENT": "shim", "BOXER_ISOLATION": "repo"}},
		{"mode = \"rewrite\"\nenforcement = \"both\"\nmount_at = \"/workspace\"\nintercept = [\"npm\", \"cargo\"]\n", nil},
	}
	var out []map[string]string
	for _, h := range hostile {
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", home)
		if err := os.MkdirAll(filepath.Join(home, "boxer"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "boxer", "boxer.toml"), []byte(h.toml), 0o644); err != nil {
			t.Fatal(err)
		}
		for k, v := range h.env {
			t.Setenv(k, v)
		}
		// Rendering goes through the same configuration load every command does; if any of it
		// reached the templates, this file set would differ.
		if _, err := config.Load("", ""); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		files, err := Render(Package, "1.0.0", dir)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, f := range files {
			rel, _ := filepath.Rel(dir, f)
			got[rel] = read(t, f)
		}
		out = append(out, got)
	}
	if len(out[0]) != len(out[1]) {
		t.Fatalf("different file sets: %d vs %d", len(out[0]), len(out[1]))
	}
	for rel, a := range out[0] {
		if b, ok := out[1][rel]; !ok || a != b {
			t.Errorf("%s differs between configurations", rel)
		}
	}
}

// The spec's executable layer: the scripts exist, are runnable, and call the installed binary
// rather than carrying policy of their own.
func TestSkillCarriesItsScripts(t *testing.T) {
	dir := t.TempDir()
	if _, err := Render(Package, "t", dir); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"run", "task", "status", "brief"} {
		p := filepath.Join(dir, "skills", "boxer", "scripts", n)
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("scripts/%s: %v", n, err)
		}
		if st.Mode()&0o111 == 0 {
			t.Errorf("scripts/%s is not executable", n)
		}
		if body := read(t, p); !strings.HasPrefix(body, "#!/bin/sh") || !strings.Contains(body, "exec boxer ") {
			t.Errorf("scripts/%s must be a POSIX sh wrapper around the binary:\n%s", n, body)
		}
	}
	if !strings.Contains(read(t, filepath.Join(dir, "skills", "boxer", "SKILL.md")), "allowed-tools: Bash(boxer:*)") {
		t.Error("SKILL.md must pre-approve the binary it tells the agent to run")
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "boxer", "references", "BRIEF.md")); err != nil {
		t.Error("the long-form brief belongs in references/, loaded on demand")
	}
	if _, err := os.Stat(filepath.Join(dir, "bin")); err == nil {
		t.Error("shims are generated by `boxer shim install`, not published in the package")
	}
}

func TestUnknownHarness(t *testing.T) {
	if _, err := Render("emacs", "t", t.TempDir()); err == nil {
		t.Fatal("unknown harness must fail")
	}
}

func TestVersionIsStampedIntoManifestsAndSkill(t *testing.T) {
	dir := t.TempDir()
	if _, err := Render(Package, "0.9.1-test", dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.json", ".claude-plugin/plugin.json", ".codex-plugin/plugin.json", ".grok-plugin/plugin.json", "gemini-extension.json", "skills/boxer/SKILL.md", "AGENTS.md"} {
		if !strings.Contains(read(t, filepath.Join(dir, f)), "0.9.1-test") {
			t.Errorf("%s: version not stamped", f)
		}
	}
}

// The skill is published three times — inside the plugin, at the installer path, and in the
// site's discovery index — and three copies of one document are only safe while something proves
// they are the same document.
func TestPublishedSkillMatchesThePluginsCopy(t *testing.T) {
	dir := t.TempDir()
	skills, wellKnown := filepath.Join(dir, "skills"), filepath.Join(dir, "well-known")
	if _, err := RenderSkills("1.2.3", skills, wellKnown); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, "pkg")
	if _, err := Render(Package, "1.2.3", pkg); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(pkg, "skills", "boxer", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(skills, "boxer", "SKILL.md"),
		filepath.Join(wellKnown, "agent-skills", "boxer", "SKILL.md"),
	} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s differs from the plugin's copy", p)
		}
	}
	// The index has to verify the exact bytes it points at, or a client cannot trust what it
	// fetched, and it has to carry the skill's own description rather than a second one.
	b, err := os.ReadFile(filepath.Join(wellKnown, "agent-skills", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index struct {
		SchemaVersion string `json:"schemaVersion"`
		Skills        []struct {
			Name, Description, URL, Digest, Version string
		}
	}
	if err := json.Unmarshal(b, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Skills) != 1 || index.Skills[0].Name != "boxer" || index.Skills[0].Version != "1.2.3" {
		t.Fatalf("index: %+v", index)
	}
	sum := sha256.Sum256(want)
	if index.Skills[0].Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("digest does not match the published bytes: %s", index.Skills[0].Digest)
	}
	if !strings.Contains(index.Skills[0].Description, "boxer sandbox") {
		t.Fatalf("the index must carry the skill's own description: %q", index.Skills[0].Description)
	}
	if !strings.HasPrefix(index.Skills[0].URL, SiteOrigin+"/.well-known/") {
		t.Fatalf("url: %q", index.Skills[0].URL)
	}
	// The scripts are the spec's executable layer and have to arrive runnable.
	st, err := os.Stat(filepath.Join(skills, "boxer", "scripts", "run"))
	if err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("scripts must be executable: %v %v", st, err)
	}
}

// Either half can be rendered alone: the Makefile writes both, but the site build wants only the
// discovery index and a skill manager wants only the installer path.
func TestSkillsRenderEitherHalfAlone(t *testing.T) {
	dir := t.TempDir()
	files, err := RenderSkills("1.2.3", filepath.Join(dir, "skills"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.Contains(f, "well-known") {
			t.Fatalf("no index was asked for: %v", files)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "boxer", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderSkills("1.2.3", "", filepath.Join(dir, "wk")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wk", "agent-skills", "index.json")); err != nil {
		t.Fatal(err)
	}
	// Rendering twice is how CI checks for drift, so it must overwrite rather than accumulate.
	if _, err := RenderSkills("1.2.3", filepath.Join(dir, "skills"), filepath.Join(dir, "wk")); err != nil {
		t.Fatal(err)
	}
	if n := len(mustReadDir(t, filepath.Join(dir, "skills", "boxer"))); n != 3 {
		t.Fatalf("SKILL.md, references and scripts, not more: %d", n)
	}
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	e, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// The index takes its description from the skill's own frontmatter, so the two cannot disagree.
// A skill with none yields an empty string rather than an invented one.
func TestSkillDescriptionComesFromTheDocument(t *testing.T) {
	if got := skillDescription([]byte("---\nname: boxer\ndescription:  run things \n---\n")); got != "run things" {
		t.Fatalf("%q", got)
	}
	if got := skillDescription([]byte("# no frontmatter\n")); got != "" {
		t.Fatalf("%q", got)
	}
}

// Writing is the one step that touches a filesystem boxer does not own, so it reports rather
// than panics when the destination refuses.
func TestWriteAllReportsAnUnwritableDestination(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeAll(file, map[string][]byte{"a/b.txt": []byte("x")}); err == nil {
		t.Fatal("writing under a regular file must fail")
	}
	if _, err := RenderSkills("1.2.3", file, ""); err == nil {
		t.Fatal("RenderSkills must report it too")
	}
}

// The session-start hook waits for the sandbox: image pull, pack build, setup and ready. At 120s a
// cold first boot outlasted it, the agent killed the hook, and the session started without boxer's
// brief. Every provisioning event now allows ten minutes.
func TestProvisioningHooksAllowASlowFirstBoot(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("templates", "plugin", "*", "hooks", "hooks.json"))
	if len(files) < 4 {
		t.Fatalf("found only %d hooks files", len(files))
	}
	provisioning := map[string]bool{"SessionStart": true, "SubagentStart": true, "UserPromptSubmit": true}
	for _, f := range files {
		var doc struct {
			Hooks map[string][]struct {
				Hooks []struct {
					Timeout float64 `json:"timeout"`
				} `json:"hooks"`
			} `json:"hooks"`
		}
		b, _ := os.ReadFile(f)
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for event, groups := range doc.Hooks {
			if !provisioning[event] {
				continue
			}
			for _, g := range groups {
				for _, h := range g.Hooks {
					secs := h.Timeout
					if strings.Contains(f, "gemini") {
						secs /= 1000 // Gemini CLI counts milliseconds
					}
					if secs < 600 {
						t.Errorf("%s: %s allows %gs; a cold first boot needs up to 600s", f, event, secs)
					}
				}
			}
		}
	}
}
