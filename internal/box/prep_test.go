package box

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func prepEnvFor(t *testing.T, image string, cmds []string, files map[string]string) *Env {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	cfg.Image = image
	cfg.Prep.Commands = cmds
	return &Env{Cfg: cfg, Scope: scope.Scope{Root: root, Key: "sb-test"}}
}

// The libc half of the triple is the one that silently breaks things: a musl guest loading a
// glibc binary fails at load time, far from the install that chose it. The image name is the only
// signal available and every Alpine tag carries it.
func TestTheTargetTripleComesFromTheImage(t *testing.T) {
	for _, tc := range []struct{ image, wantLibc string }{
		{"node:24-alpine", "musl"},
		{"mirror.gcr.io/library/node:24-alpine", "musl"},
		{"node:24-bookworm-slim", "glibc"},
		{"debian:bookworm-slim", "glibc"},
	} {
		e := prepEnvFor(t, tc.image, []string{"true"}, nil)
		os_, cpu, libc := e.Target()
		if os_ != "linux" || libc != tc.wantLibc || cpu == "" {
			t.Errorf("%s -> %s/%s/%s, want linux/*/%s", tc.image, os_, cpu, libc, tc.wantLibc)
		}
	}

	// An explicit target overrides the guess, because someone cross-building knows better.
	e := prepEnvFor(t, "node:24-alpine", []string{"true"}, nil)
	e.Cfg.Prep.Target = "linux/amd64/glibc"
	if os_, cpu, libc := e.Target(); os_ != "linux" || cpu != "amd64" || libc != "glibc" {
		t.Errorf("explicit target ignored: %s/%s/%s", os_, cpu, libc)
	}
}

// The triple reaches the command as environment, never spliced into its argv — boxer must not
// rewrite what the user wrote.
func TestTheTripleIsOfferedAsEnvironmentNotInjectedIntoTheCommand(t *testing.T) {
	e := prepEnvFor(t, "node:24-alpine", []string{"npm ci"}, nil)
	env := strings.Join(e.prepEnv(), "\n")
	for _, want := range []string{"BOXER_TARGET_OS=linux", "BOXER_TARGET_LIBC=musl", "--os=linux"} {
		if !strings.Contains(env, want) {
			t.Errorf("prep environment is missing %q: %v", want, env)
		}
	}
	if e.Cfg.Prep.Commands[0] != "npm ci" {
		t.Errorf("boxer rewrote the command: %q", e.Cfg.Prep.Commands[0])
	}
}

// The tripwire. Without this warning the feature ships a sandbox that fails at run time with
// `invalid ELF header`, naming neither prep nor the package that caused it.
func TestPrepWarnsAboutPackagesThatCompileOnTheHost(t *testing.T) {
	lock := `{"packages":{"node_modules/sharp":{},"node_modules/left-pad":{}}}`
	e := prepEnvFor(t, "node:24-alpine", []string{"npm ci"}, map[string]string{"package-lock.json": lock})
	w := e.PrepWarnings()
	if len(w) != 1 || !strings.Contains(w[0], "sharp") {
		t.Fatalf("want a warning naming sharp, got %v", w)
	}
	if strings.Contains(w[0], "left-pad") {
		t.Errorf("warned about a pure-JavaScript package: %v", w)
	}

	// No prep configured: nothing to warn about, however the tree looks.
	quiet := prepEnvFor(t, "node:24-alpine", nil, map[string]string{"package-lock.json": lock})
	if w := quiet.PrepWarnings(); len(w) != 0 {
		t.Errorf("warned with no prep configured: %v", w)
	}

	// Opting out of targeting means installing in the guest, which is always correct.
	none := prepEnvFor(t, "node:24-alpine", []string{"npm ci"}, map[string]string{"package-lock.json": lock})
	none.Cfg.Prep.Target = "none"
	if w := none.PrepWarnings(); len(w) != 0 {
		t.Errorf("warned despite target = none: %v", w)
	}
}

// The marker is keyed to what prep says, so changing a command re-runs it and changing nothing
// does not. Same shape as `setup`'s, for the same reason: the work lands in the worktree.
func TestThePrepMarkerFollowsTheCommands(t *testing.T) {
	a := config.Defaults()
	a.Prep.Commands = []string{"npm ci"}
	b := a
	b.Prep.Commands = []string{"npm ci --omit=dev"}
	if prepMarkerPath("/r", "sb-x", a) == prepMarkerPath("/r", "sb-x", b) {
		t.Error("changing a prep command must change the marker")
	}
	first, second := prepMarkerPath("/r", "sb-x", a), prepMarkerPath("/r", "sb-x", a)
	if first != second {
		t.Errorf("the marker is not stable for one configuration: %q then %q", first, second)
	}
	if prepMarkerPath("/r", "sb-x", a) == prepMarkerPath("/r", "sb-y", a) {
		t.Error("two scopes must not share a marker")
	}
	// Building the key must not scribble on the caller's slice.
	_ = prepMarkerPath("/r", "sb-x", a)
	if len(a.Prep.Commands) != 1 || a.Prep.Commands[0] != "npm ci" {
		t.Errorf("the config was mutated: %v", a.Prep.Commands)
	}
}

// Prep runs on the host, in the worktree, once. The three things worth proving are that it ran
// where it said it would, that the triple reached it, and that it does not run again.
func TestPrepRunsInTheWorktreeOnceAndSeesTheTriple(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e := prepEnvFor(t, "node:24-alpine", []string{
		"pwd > where.txt", "printf %s \"$BOXER_TARGET_LIBC\" > libc.txt", "echo x >> runs.txt",
	}, nil)
	e.VM = vmtest.NewBare()
	e.Stderr = io.Discard

	if err := e.Prep(); err != nil {
		t.Fatalf("prep: %v", err)
	}
	where, err := os.ReadFile(filepath.Join(e.Scope.Root, "where.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Both sides resolved: on macOS a temp dir is /var/... which is a symlink to /private/var,
	// and the shell's pwd reports the resolved one.
	ran, _ := filepath.EvalSymlinks(strings.TrimSpace(string(where)))
	want, _ := filepath.EvalSymlinks(e.Scope.Root)
	if ran != want {
		t.Errorf("prep ran in %q, want the worktree %q", ran, want)
	}
	if b, _ := os.ReadFile(filepath.Join(e.Scope.Root, "libc.txt")); string(b) != "musl" {
		t.Errorf("the target triple did not reach the command: %q", b)
	}

	// Once per worktree: a second call is a no-op while the commands are unchanged.
	if err := e.Prep(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(e.Scope.Root, "runs.txt")); strings.Count(string(b), "x") != 1 {
		t.Errorf("prep ran again: %q", b)
	}
}

// A command that fails must fail the provision, naming prep — not be swallowed into a sandbox
// that comes up missing its dependencies.
func TestPrepFailureIsReportedRatherThanSwallowed(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e := prepEnvFor(t, "node:24-alpine", []string{"exit 3"}, nil)
	e.VM = vmtest.NewBare()
	e.Stderr = io.Discard
	err := e.Prep()
	be, ok := err.(*Error)
	if !ok || be.Cause != "PREP_FAILED" || !strings.Contains(be.Fix, "setup") {
		t.Fatalf("want a PREP_FAILED naming the alternative, got %v", err)
	}
}
