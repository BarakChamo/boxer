package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

func gitCommit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "x"},
	} {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// A capsule turns "it failed here once" into something another person, or the same agent
// tomorrow, can run. Replay exits 0 when the failure reproduced, because the capsule is a
// question and not a test suite.
func TestCapsuleRecordsAndReplaysAFailure(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	gitCommit(t, dir)

	// Nothing has run yet, so there is nothing to capture, and the refusal says what to do.
	if code, out := call(t, nil, "capsule", "new"); code != 1 || !strings.Contains(out, "NO_RUN_RECORD") {
		t.Fatalf("empty: %d %s", code, out)
	}
	if code, _ := call(t, nil, "run", "-c", "echo boom >&2; exit 4"); code != 4 {
		t.Fatal("setup run should exit 4")
	}
	if code, out := call(t, nil, "capsule", "new"); code != 0 || !strings.Contains(out, "exit 4") {
		t.Fatalf("new: %d %s", code, out)
	}
	if code, out := call(t, nil, "capsule", "inspect", "capsule.toml"); code != 0 || !strings.Contains(out, "expect:    exit 4") {
		t.Fatalf("inspect: %d %s", code, out)
	}
	if code, out := call(t, nil, "capsule", "replay", "capsule.toml"); code != 0 || !strings.Contains(out, "reproduced") {
		t.Fatalf("replay: %d %s", code, out)
	}
	// Rewrite the capsule's command to one that passes: the failure no longer reproduces, and the
	// replay says so rather than reporting success.
	b, err := os.ReadFile(filepath.Join(dir, "capsule.toml"))
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(b), "echo boom >&2; exit 4", "true", 1)
	if err := os.WriteFile(filepath.Join(dir, "capsule.toml"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := call(t, nil, "capsule", "replay", "capsule.toml", "--allow-dirty"); code != 1 || !strings.Contains(out, "did NOT reproduce") {
		t.Fatalf("non-reproduction: %d %s", code, out)
	}
}

// A replay against a tree nobody knows the state of proves nothing, so a dirty worktree and a
// drifted HEAD are both refusals with a flag that overrides them.
func TestCapsuleReplayRefusesAnUnknownTree(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	gitCommit(t, dir)
	if code, _ := call(t, nil, "run", "-c", "exit 2"); code != 2 {
		t.Fatal("setup")
	}
	if code, _ := call(t, nil, "capsule", "new"); code != 0 {
		t.Fatal("new")
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := call(t, nil, "capsule", "replay", "capsule.toml")
	if code != 1 || !strings.Contains(out, "DIRTY_WORKTREE") || !strings.Contains(out, "--allow-dirty") {
		t.Fatalf("dirty: %d %s", code, out)
	}
	if code, out := call(t, nil, "capsule", "replay", "capsule.toml", "--allow-dirty"); code != 0 {
		t.Fatalf("--allow-dirty must proceed: %d %s", code, out)
	}
	// Move HEAD on: the capsule now describes a tree this one is not.
	gitCommit(t, dir)
	code, out = call(t, nil, "capsule", "replay", "capsule.toml")
	if code != 1 || !strings.Contains(out, "CAPSULE_DRIFT") {
		t.Fatalf("drift: %d %s", code, out)
	}
	if code, _ := call(t, nil, "capsule", "replay", "capsule.toml", "--allow-drift"); code != 0 {
		t.Fatalf("--allow-drift must proceed: %d", code)
	}
}
