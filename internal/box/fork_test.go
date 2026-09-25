package box

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func forkEnv(t *testing.T) (*Env, string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"image = \"alpine\"\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = io.Discard
	return e, log
}

// Preparing restarts the sandbox as a branch source and changes nothing else: children share the
// live worktree, because smolvm refuses to branch a machine whose mount is staged.
func TestPrepareForkMakesABranchSource(t *testing.T) {
	e, log := forkEnv(t)
	if _, err := e.Fork(1); err == nil {
		t.Fatal("an unprepared sandbox must refuse to fork")
	} else if be, ok := err.(*Error); !ok || be.Cause != "NOT_FORKABLE" || !strings.Contains(be.Fix, "--prepare") {
		t.Fatalf("the refusal must say what to run: %v", err)
	}
	if err := e.PrepareFork(); err != nil {
		t.Fatal(err)
	}
	if !Branchable(e.Scope.Key) {
		t.Fatal("the source must be recorded as branchable")
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "machine start -n "+e.Scope.Key+" --branchable") {
		t.Fatalf("the source must be started branchable:\n%s", b)
	}
	if strings.Contains(string(b), ":staged") {
		t.Fatalf("a staged mount cannot be branched at all, so boxer must never ask for one:\n%s", b)
	}
	names, err := e.Fork(2)
	if err != nil || len(names) != 2 {
		t.Fatalf("fork: %v %v", names, err)
	}
	kids, err := e.Forks()
	if err != nil || len(kids) != 2 {
		t.Fatalf("forks: %v %v", kids, err)
	}
	for _, k := range kids {
		if ParentOf(k.Name) != e.Scope.Key || !IsForkChild(k.Name) {
			t.Fatalf("a child must name its parent: %s", k.Name)
		}
	}
	// A child is reachable by name, and by the name said out loud.
	if _, err := e.At(names[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := e.At(scope.Slug(names[0])); err != nil {
		t.Fatalf("a slug must address a child too: %v", err)
	}
	if _, err := e.At("sb-000000000000"); err == nil {
		t.Fatal("an unknown sandbox must be refused")
	}
	// Stopping the sandbox ends the branch point with the process that held it.
	if err := e.Down(); err != nil {
		t.Fatal(err)
	}
	if Branchable(e.Scope.Key) {
		t.Fatal("branchability must not outlive the sandbox")
	}
}

// A named pack is an artifact, so its failures are reported rather than swallowed, and its name
// is checked before it becomes a path.
func TestNamedPacks(t *testing.T) {
	e, _ := forkEnv(t)
	if _, err := e.SavePack("base"); err == nil {
		t.Fatal("there is nothing to save before the sandbox exists")
	}
	if _, err := NamedPackPath("../escape"); err == nil {
		t.Fatal("a name that is not a path segment must be refused")
	}
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	path, err := e.SavePack("base")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != NamedPackDir() {
		t.Fatalf("a named pack lives apart from the cache gc sweeps: %s", path)
	}
	packs := NamedPacks()
	if len(packs) != 1 || packs[0].Name != "base" || packs[0].Bytes == 0 {
		t.Fatalf("packs: %+v", packs)
	}
	// Using a pack that is gone is an error, never a silent pull.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	e.FromPack = "base"
	if _, err := e.Ensure(true, true); err == nil {
		t.Fatal("a missing named pack must be refused")
	} else if be, ok := err.(*Error); !ok || be.Cause != "NO_SUCH_PACK" {
		t.Fatalf("cause: %v", err)
	}
}

// The run record explains the last command well enough to replay it, and stores a shell line
// rather than a joined argv: `sh -c "exit 4"` joined is a different command.
func TestRunRecordIsReplayable(t *testing.T) {
	e, _ := forkEnv(t)
	// Separate writers, because that is what the CLI passes and it is the case where the record
	// keeps the two tails apart.
	var out, errOut bytes.Buffer
	if _, err := e.Run([]string{"sh", "-c", "echo out; echo err >&2; exit 4"},
		RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errOut, Task: "test"}); err != nil {
		t.Fatal(err)
	}
	rec, ok := ReadRunRecord(e.Scope.Key)
	if !ok {
		t.Fatal("a run must leave a record")
	}
	if rec.Command != "echo out; echo err >&2; exit 4" || rec.Exit != 4 || rec.Task != "test" {
		t.Fatalf("record: %+v", rec)
	}
	if !strings.Contains(rec.Stdout, "out") || !strings.Contains(rec.Stderr, "err") {
		t.Fatalf("tails: %q %q", rec.Stdout, rec.Stderr)
	}
	// One writer for both streams stays one writer: os/exec gives them a single pipe in that
	// case, and splitting it would make a caller's plain buffer concurrent behind its back.
	var both bytes.Buffer
	if _, err := e.Run([]string{"sh", "-c", "echo one; echo two >&2"},
		RunOpts{Stdin: strings.NewReader(""), Stdout: &both, Stderr: &both}); err != nil {
		t.Fatal(err)
	}
	rec, _ = ReadRunRecord(e.Scope.Key)
	if !strings.Contains(rec.Stdout, "one") || !strings.Contains(rec.Stdout, "two") || rec.Stderr != "" {
		t.Fatalf("a combined stream is recorded once: %q / %q", rec.Stdout, rec.Stderr)
	}
	if CommandLine([]string{"npm", "test", "--", "--watch all"}) != "npm test -- '--watch all'" {
		t.Fatalf("argv form: %q", CommandLine([]string{"npm", "test", "--", "--watch all"}))
	}
	AmendRunTests(e.Scope.Key, &TestSummary{Tests: 3, Failures: 1, Failed: []string{"a/b"}})
	rec, _ = ReadRunRecord(e.Scope.Key)
	if rec.Tests == nil || rec.Tests.Failures != 1 || rec.Command == "" {
		t.Fatalf("amending must keep the rest of the record: %+v", rec)
	}
}

// The tail keeps the end of a long stream without holding the whole thing.
func TestTailIsBounded(t *testing.T) {
	ta := &tail{n: 8}
	for i := 0; i < 100; i++ {
		if _, err := ta.Write([]byte("0123456789")); err != nil {
			t.Fatal(err)
		}
	}
	if got := ta.String(); got != "23456789" || len(got) != 8 {
		t.Fatalf("tail: %q", got)
	}
}

// A backend that cannot branch must say so by name, and must say so *before* doing any work.
// The failure mode this guards against is the one the contract calls silent degradation: a fork
// that reports success and hands back the parent, or that restarts the sandbox — ending whatever
// was running in it — and only then discovers it cannot branch.
func TestForkRefusesByNameOnABackendThatCannotBranch(t *testing.T) {
	e, _ := forkEnv(t)
	bare := vmtest.NewBare()
	e.VM = bare

	for _, tc := range []struct {
		what string
		run  func() error
	}{
		{"fork", func() error { _, err := e.Fork(1); return err }},
		{"prepare", e.PrepareFork},
	} {
		err := tc.run()
		if !errors.Is(err, vm.ErrUnsupported) {
			t.Errorf("%s: want a refusal, got %v", tc.what, err)
		}
		if !strings.Contains(err.Error(), "bare") || !strings.Contains(err.Error(), "smolvm") {
			t.Errorf("%s: the refusal must name the backend and the one that works: %v", tc.what, err)
		}
	}
	if len(bare.Calls) != 0 {
		t.Errorf("refused, but still touched the backend: %v", bare.Calls)
	}
}

// Saving a *named* pack is an explicit request, so it refuses. That is deliberately different
// from the automatic environment cache, which declines in silence because nobody asked for it.
func TestANamedPackRefusesWhileTheAutomaticCacheJustDeclines(t *testing.T) {
	e, _ := forkEnv(t)
	e.VM = vmtest.NewBare()

	if _, err := e.SavePack("base"); !errors.Is(err, vm.ErrUnsupported) {
		t.Errorf("an explicit `pack save` must refuse: %v", err)
	}
	// PackEnv returns nothing and must simply do nothing rather than panic or refuse.
	e.PackEnv()
}
