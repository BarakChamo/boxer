package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// A Ruby or JVM project's tools are not in the default intercept list, and cannot be added to it
// within 1.x, so doctor names them and the line that fixes it.
func TestDoctorSuggestsInterceptAlso(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Gemfile"), nil, 0o644)
	w := interceptSuggestion(dir, []string{"npm", "rake"})
	if !strings.Contains(w, `intercept_also = ["ruby", "bundle", "rails", "rspec"]`) {
		t.Fatalf("got %q", w)
	}
	if w := interceptSuggestion(dir, []string{"ruby", "bundle", "rake", "rails", "rspec"}); w != "" {
		t.Fatalf("nothing to suggest once they are intercepted: %q", w)
	}
	if w := interceptSuggestion(dir, []string{"*"}); w != "" {
		t.Fatalf("intercept = [\"*\"] already takes everything: %q", w)
	}
	if w := interceptSuggestion(t.TempDir(), nil); w != "" {
		t.Fatalf("a project with no marker file gets no suggestion: %q", w)
	}
}

// install then uninstall through the CLI leaves the repository as it was, and says what it did.
func TestUninstallCommand(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	before := snapshot(t, dir)
	if code, out := call(t, nil, "install", "all"); code != 0 {
		t.Fatalf("install all: %d %s", code, out)
	}
	if code, out := call(t, nil, "install", "git"); code != 0 {
		t.Fatalf("install git: %d %s", code, out)
	}
	code, out := call(t, nil, "uninstall", "all")
	if code != 0 || !strings.Contains(out, "removed boxer from .claude/settings.json") || !strings.Contains(out, "note:") {
		t.Fatalf("uninstall all: %d %s", code, out)
	}
	if code, out := call(t, nil, "uninstall", "git"); code != 0 || !strings.Contains(out, "removed boxer from") {
		t.Fatalf("uninstall git: %d %s", code, out)
	}
	// opencode.json is kept with only $schema, since boxer cannot tell whether it made it.
	if b, err := os.ReadFile(filepath.Join(dir, "opencode.json")); err == nil {
		if !strings.Contains(string(b), "$schema") || strings.Contains(string(b), "boxer") {
			t.Fatalf("opencode.json must be left with only its $schema: %s", b)
		}
		os.Remove(filepath.Join(dir, "opencode.json"))
	}
	if after := snapshot(t, dir); after != before {
		t.Fatalf("the repository changed:\nbefore %s\nafter  %s", before, after)
	}
	if code, out := call(t, nil, "uninstall", "claude-code"); code != 0 || !strings.Contains(out, "nothing to remove") {
		t.Fatalf("a second uninstall: %d %s", code, out)
	}
	if code, out := call(t, nil, "uninstall", "conductor"); code != 0 || !strings.Contains(out, "nothing to remove") {
		t.Fatalf("uninstall conductor: %d %s", code, out)
	}
	if code, _ := call(t, nil, "uninstall"); code != 2 {
		t.Fatal("bare uninstall is a usage error")
	}
	if code, out := call(t, nil, "uninstall", "nope"); code != 1 || !strings.Contains(out, "nope") {
		t.Fatalf("unknown target: %d %s", code, out)
	}
	t.Chdir(t.TempDir())
	if code, out := call(t, nil, "uninstall", "codex"); code != 1 || !strings.Contains(out, "git repository") {
		t.Fatalf("outside a repository: %d %s", code, out)
	}
}

// snapshot lists every file outside .git with its size, as one comparable string.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			st, _ := d.Info()
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintf(&b, "%s:%d ", rel, st.Size())
		}
		return nil
	})
	return b.String()
}

// `boxer restart` through the CLI, and gc --dry-run's view of idle stops and orphaned volumes.
func TestRestartCommandAndGCDryRun(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"start = [\"echo serving\"]\nidle_timeout = \"1m\"\n")
	if code, out := call(t, nil, "restart"); code != 1 || !strings.Contains(out, "NO_SANDBOX") {
		t.Fatalf("restart with no sandbox: %d %s", code, out)
	}
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if code, out := call(t, nil, "restart"); code != 0 || !strings.Contains(out, "restarted 1 service") {
		t.Fatalf("restart: %d %s", code, out)
	}
	var ls []map[string]any
	call(t, &ls, "ls", "--json")
	key := ls[0]["scope"].(string)
	ageStamp(t, key)
	orphan := filepath.Join(box.VolumeDir(), "sb-orphan")
	os.MkdirAll(orphan, 0o755)
	os.WriteFile(filepath.Join(orphan, ".worktree"), []byte(filepath.Join(t.TempDir(), "gone")), 0o644)
	code, out := call(t, nil, "gc", "--dry-run")
	if code != 0 || !strings.Contains(out, "would stop "+key) || !strings.Contains(out, "would delete the volumes of sb-orphan") {
		t.Fatalf("gc --dry-run: %d %s", code, out)
	}
	if strings.Contains(out, "reclaim 1 sandbox") {
		t.Fatalf("a stop is not a reclaimed sandbox: %s", out)
	}
	if code, out := call(t, nil, "gc"); code != 0 || !strings.Contains(out, "stopped "+key) || !strings.Contains(out, "deleted the volumes of sb-orphan") {
		t.Fatalf("gc: %d %s", code, out)
	}
}

func TestTurbopackWarning(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"16"}}`), 0o644)
	if w := turbopackWarning(dir, "smolvm", "darwin"); !strings.Contains(w, "next dev --webpack") {
		t.Fatalf("smolvm: %q", w)
	}
	if w := turbopackWarning(dir, "docker", "darwin"); w != "" {
		t.Fatalf("docker passes host edits through: %q", w)
	}
	if w := turbopackWarning(dir, "podman", "linux"); w != "" {
		t.Fatalf("podman on Linux shares the host kernel's inotify: %q", w)
	}
	if w := turbopackWarning(t.TempDir(), "smolvm", "darwin"); w != "" {
		t.Fatalf("not a Next.js project: %q", w)
	}
}

// A shim under mode = "off", enforcement = "audit" or "hook" runs the program on the host: the
// repository asked for nothing to be sandboxed by a shim.
func TestShimsHonourModeOffAndAudit(t *testing.T) {
	for _, tc := range []struct{ toml, wantErr string }{
		{"mode = \"off\"\n", ""},
		{"enforcement = \"hook\"\n", ""},
		{"enforcement = \"audit\"\n", "boxer: audit: would run in the sandbox: echo from-host"},
	} {
		_, log := vmtest.Install(t)
		vmtest.RepoIn(t, vmtest.NoWorktreeCheck+tc.toml)
		t.Setenv("BOXER_SHIM", "1")
		code, out := call(t, nil, "run", "--", "echo", "from-host")
		if code != 0 || !strings.Contains(out, "from-host") || !strings.Contains(out, tc.wantErr) {
			t.Fatalf("%s: %d %s", tc.toml, code, out)
		}
		if b, _ := os.ReadFile(log); strings.Contains(string(b), "machine create") {
			t.Fatalf("%s: a shim created a sandbox:\n%s", tc.toml, b)
		}
	}
}

// A harness override that relies on shims keeps them sandboxing, whatever the top level says.
func TestShimsKeepSandboxingForAHarnessThatReliesOnThem(t *testing.T) {
	_, log := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"enforcement = \"hook\"\n[harness.kimi]\nenforcement = \"shim\"\n")
	t.Setenv("BOXER_SHIM", "1")
	if code, out := call(t, nil, "run", "--", "echo", "x"); code != 0 {
		t.Fatalf("%d %s", code, out)
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "machine exec") {
		t.Fatalf("the shim must still sandbox:\n%s", b)
	}
}
