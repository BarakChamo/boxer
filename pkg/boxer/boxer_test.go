package boxer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

func TestFacadeRoundTrip(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
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

// The host-wide functions look at the backend boxer.toml names, as the CLI does; they once always
// looked at smolvm, so a docker-configured caller listed nothing and stopped nothing.
func TestHostWideFunctionsUseConfiguredBackend(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("backend = \"docker\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if got := host().Name(); got != "docker" {
		t.Fatalf("host backend = %q, want docker", got)
	}
}
