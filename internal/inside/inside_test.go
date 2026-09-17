package inside

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTableIsConsistent(t *testing.T) {
	for _, n := range Names() {
		h, ok := Harnesses[n]
		if !ok || h.Bin == "" || h.Install == "" || h.ConfigDir == "" {
			t.Fatalf("%s: incomplete row %+v", n, h)
		}
		if h.ACP != nil && h.ACP[0] == "" {
			t.Fatalf("%s: empty ACP argv", n)
		}
	}
	hosts := AllowHosts()
	if hosts[0] != "registry.npmjs.org" || !contains(hosts, "deb.debian.org") || !contains(hosts, "api.anthropic.com") || !contains(hosts, "api.openai.com") {
		t.Fatalf("allow hosts: %v", hosts)
	}
}

func TestMountsOnlyExistingConfigDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, v := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "GEMINI_CLI_HOME", "KIMI_CODE_HOME", "GROK_HOME"} {
		t.Setenv(v, "")
	}
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	m := Mounts()
	want := []string{custom + ":" + custom, filepath.Join(home, ".codex") + ":" + filepath.Join(home, ".codex")}
	if strings.Join(m, ",") != strings.Join(want, ",") {
		t.Fatalf("mounts %v, want %v", m, want)
	}
}

func TestGuestEnv(t *testing.T) {
	t.Setenv("HOME", "/Users/x")
	t.Setenv("CODEX_HOME", "/Users/x/codex-home")
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("OPENAI_BASE_URL", "")
	env := guestEnv(Harnesses["codex"], []string{"EXTRA=1"})
	joined := strings.Join(env, "\n")
	for _, want := range []string{"HOME=/Users/x", "CODEX_HOME=/Users/x/codex-home", "OPENAI_API_KEY=k", "OPENAI_BASE_URL=", "EXTRA=1", `CODEX_CONFIG={"sandbox_mode":"danger-full-access"}`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "ANTHROPIC") {
		t.Fatal("other harnesses' variables must not leak")
	}
	if a := Harnesses["codex"].Args; len(a) != 2 || a[0] != "-c" {
		t.Fatalf("codex shell args must turn its nested sandbox off: %v", a)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestLoginHintOnlyWithoutCredentials(t *testing.T) {
	for _, k := range Harnesses["claude"].Creds {
		t.Setenv(k, "")
	}
	if loginHint(Harnesses["claude"]) == "" {
		t.Fatal("claude without any token must print the setup-token hint")
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "x")
	if loginHint(Harnesses["claude"]) != "" {
		t.Fatal("a token silences the hint")
	}
	if loginHint(Harnesses["codex"]) != "" {
		t.Fatal("codex's auth.json travels with the mount; no hint")
	}
}
