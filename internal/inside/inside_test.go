package inside

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// R-INT-4: a dropped exec transport during the harness install restarts the VM and retries once.
func TestInstallRetriesAfterDroppedTransport(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("require_worktree = \"off\"\n"), 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e, err := box.Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = os.Stderr
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	vmtest.FailExecOnce(t, "touch /var/lib/boxer/harness-x")
	if err := install(e, "x", Harness{Bin: "true", Install: "true"}); err != nil {
		t.Fatalf("install must succeed after one dropped exec: %v", err)
	}
	b, _ := os.ReadFile(log)
	if strings.Count(string(b), "harness-x") != 3 || !strings.Contains(string(b), "machine stop") {
		t.Fatalf("want marker check, failed install, stop+start, retried install:\n%s", b)
	}
}

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
	if hosts[0] != "registry.npmjs.org" || !slices.Contains(hosts, "deb.debian.org") || !slices.Contains(hosts, "api.anthropic.com") || !slices.Contains(hosts, "api.openai.com") {
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
	env, err := guestEnv(Harnesses["codex"], []string{"EXTRA=1"})
	if err != nil {
		t.Fatal(err)
	}
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
	// An empty HOME would reach the guest and break every mounted config path, so it is refused.
	t.Setenv("HOME", "")
	if _, err := guestEnv(Harnesses["codex"], nil); err == nil {
		t.Fatal("an unresolvable home directory must be an error, not HOME=")
	}
}

func TestLoginHintOnlyWithoutCredentials(t *testing.T) {
	for _, k := range Harnesses["claude"].Creds {
		t.Setenv(k, "")
	}
	if loginHint(Harnesses["claude"], nil) == "" {
		t.Fatal("claude without any token must print the setup-token hint")
	}
	if loginHint(Harnesses["claude"], []string{"ANTHROPIC_API_KEY=k"}) != "" {
		t.Fatal("a key passed with -e silences the hint")
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "x")
	if loginHint(Harnesses["claude"], nil) != "" {
		t.Fatal("a token silences the hint")
	}
	if loginHint(Harnesses["codex"], nil) != "" {
		t.Fatal("codex's auth.json travels with the mount; no hint")
	}
}

// The harness marker probe must not read a dropped transport as "not installed": that reinstalls
// a harness that is already there, which is npm in the guest for minutes.
func TestHarnessMarkerProbeDistinguishesTransportFailure(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	e, err := box.Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = os.Stderr
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	vmtest.FailExecOnce(t, "test -f /var/lib/boxer/harness-x")
	err = install(e, "x", Harness{Bin: "true", Install: "true"})
	be, ok := err.(*box.Error)
	if !ok || be.Cause != "TRANSPORT_FAILED" {
		t.Fatalf("want TRANSPORT_FAILED, got %v", err)
	}
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "npm") {
		t.Fatalf("no install may run when the probe could not answer:\n%s", b)
	}
}

// LastLines is what an install or an agent failure carries instead of a whole log.
func TestLastLines(t *testing.T) {
	raw := "npm warn one\n\n  npm error two  \nnpm error three\n"
	if got := LastLines(raw, 2); got != "npm error two; npm error three" {
		t.Fatalf("got %q", got)
	}
	if got := LastLines(raw, 9); got != "npm warn one; npm error two; npm error three" {
		t.Fatalf("fewer lines than asked for: %q", got)
	}
	if got := LastLines("\n \n", 3); got != "" {
		t.Fatalf("nothing but blank lines: %q", got)
	}
}

// Inside mode is the level where boxer has nothing to hook, because the harness itself is in the
// guest. Run is the whole of it, and these are the four things a user meets: an unknown harness,
// a harness with no ACP server, the shell path, and the ACP path.
func TestRunLaunchesTheHarnessInTheGuest(t *testing.T) {
	_, log := vmtest.Install(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"integration = \"inside\"\n")
	e, err := box.Resolve(dir, "claude", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = io.Discard

	if code, err := Run(e, "nosuchharness", nil, false, Options{}); code != 2 || err == nil || !strings.Contains(err.Error(), "known:") {
		t.Fatalf("an unknown harness must list the known ones: %d %v", code, err)
	}
	// pi has no ACP server, and the refusal names the command that does work.
	if code, err := Run(e, "pi", nil, true, Options{}); code != 2 || err == nil || !strings.Contains(err.Error(), "boxer shell pi") {
		t.Fatalf("a harness without ACP must point at shell: %d %v", code, err)
	}

	// The shell path: the VM is provisioned, the harness installed once, and the binary run in the
	// guest at the mounted worktree.
	if _, err := Run(e, "claude", []string{"--version"}, false, Options{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatalf("shell: %v", err)
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	if !strings.Contains(s, "machine create") || !strings.Contains(s, "harness-claude") {
		t.Fatalf("inside mode provisions and installs:\n%s", s)
	}
	if !strings.Contains(s, "claude --version") {
		t.Fatalf("the harness binary must run in the guest:\n%s", s)
	}

	// The ACP path runs a different entry point in the same guest, and installs nothing twice.
	before := strings.Count(s, "harness-claude")
	if _, err := Run(e, "claude", nil, true, Options{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatalf("acp: %v", err)
	}
	b, _ = os.ReadFile(log)
	if !strings.Contains(string(b), "claude-agent-acp") {
		t.Fatalf("the ACP server is a different entry point:\n%s", b)
	}
	// One install line per VM. Counting "npm i -g" would count twice, because the line retries
	// itself; the environment prefix appears once per invocation.
	if installs := strings.Count(string(b), "NPM_CONFIG_FETCH_TIMEOUT"); installs != 1 {
		t.Fatalf("the harness installs once per VM, not per launch (%d):\n%s", installs, b)
	}
	if now := strings.Count(string(b), "harness-claude"); now <= before {
		t.Fatalf("the second launch still checks the marker: %d then %d", before, now)
	}
}
