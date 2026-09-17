package boxer

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

func repo(t *testing.T) string {
	t.Helper()
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
	r, _ := filepath.EvalSymlinks(dir)
	return r
}

func TestFacadeRoundTrip(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t)
	b, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if s := b.Scope(); !strings.HasPrefix(s.Key, "sb-") || s.Root != dir {
		t.Fatalf("scope: %+v", s)
	}
	if _, ok, _ := b.Status(); ok {
		t.Fatal("machine should not exist yet")
	}
	var out bytes.Buffer
	code, err := b.Run([]string{"sh", "-c", "echo hi; exit 3"}, RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out})
	if err != nil || code != 3 || !strings.Contains(out.String(), "hi") {
		t.Fatalf("run: %d %v %q", code, err, out.String())
	}
	if LastUsed(b.Scope().Key).IsZero() {
		t.Fatal("last-used not recorded")
	}
	ms, err := List()
	if err != nil || len(ms) != 1 || ms[0].Name != b.Scope().Key {
		t.Fatalf("list: %v %v", ms, err)
	}
	if err := Stop(ms[0].Name); err != nil {
		t.Fatal(err)
	}
	if m, ok, _ := b.Status(); !ok || m.Running() {
		t.Fatalf("after stop: %+v %v", m, ok)
	}
	if err := b.Down(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := b.Status(); ok {
		t.Fatal("machine should be gone")
	}
}

func TestOpenOutsideRepositoryIsError(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := Open(t.TempDir(), Options{})
	var be *Error
	if !errors.As(err, &be) || be.Cause != "NO_REPOSITORY" {
		t.Fatalf("want *Error NO_REPOSITORY, got %v", err)
	}
}
