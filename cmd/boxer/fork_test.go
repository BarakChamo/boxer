package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// A fork is a copy-on-write child of a running sandbox. Preparing one is explicit because the
// default preparation changes what the parent sees, and a child is reachable by name because it
// has no worktree of its own to resolve from.
func TestForkPreparesExplicitlyThenBranches(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	// Unprepared: the refusal has to say both what to run and what preparing costs.
	code, out := call(t, nil, "fork")
	if code != 1 || !strings.Contains(out, "NOT_FORKABLE") || !strings.Contains(out, "restarts it") {
		t.Fatalf("unprepared: %d %s", code, out)
	}
	var names []string
	if code, out := call(t, &names, "fork", "--prepare", "--count", "2", "--json"); code != 0 || len(names) != 2 {
		t.Fatalf("fork: %d %s %v", code, out, names)
	}
	if code, out := call(t, nil, "fork", "ls"); code != 0 || !strings.Contains(out, names[0]) {
		t.Fatalf("fork ls: %d %s", code, out)
	}
	// A child is addressed by name, and running in one is an ordinary run.
	if code, out := call(t, nil, "run", "--scope", names[0], "-c", "echo in-the-fork"); code != 0 || !strings.Contains(out, "in-the-fork") {
		t.Fatalf("run --scope: %d %s", code, out)
	}
	if code, out := call(t, nil, "fork", "rm", "--all"); code != 0 || !strings.Contains(out, names[1]) {
		t.Fatalf("fork rm: %d %s", code, out)
	}
	if code, out := call(t, nil, "fork", "ls"); code != 0 || strings.Contains(out, names[0]) {
		t.Fatalf("forks must be gone: %d %s", code, out)
	}
}

// Children share the parent's worktree, which is the whole boundary of the feature: smolvm
// refuses to branch a staged mount, so there is no private copy to hand a child.
func TestForkChildrenShareTheWorktree(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, _ := call(t, nil, "up"); code != 0 {
		t.Fatal("up")
	}
	var names []string
	if code, _ := call(t, &names, "fork", "--prepare", "--json"); code != 0 || len(names) != 1 {
		t.Fatalf("fork: %v", names)
	}
	// The fake runs exec locally, so what proves the shared mount is the workdir boxer asks for:
	// the child is addressed by name and given this worktree's guest path, not one of its own.
	if code, out := call(t, nil, "run", "--scope", names[0], "-c", "true"); code != 0 {
		t.Fatalf("a child runs: %d %s", code, out)
	}
	b, err := os.ReadFile(os.Getenv("FAKE_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "machine exec --name "+names[0]) || !strings.Contains(string(b), "-w /workspace") {
		t.Fatalf("the child runs in the parent's mount:\n%s", b)
	}
	// The brief keeps its ordinary promise, because a fork does not change what the sandbox sees.
	if code, out := call(t, nil, "brief"); code != 0 || !strings.Contains(out, "same files the sandbox sees") {
		t.Fatalf("brief: %d %s", code, out)
	}
}

// A machine boxer did not create is not boxer's to delete, however much it looks like one: a
// label is the proof of ownership, and a name is not. The one exception is a fork child, whose
// labels smolvm may not have copied, and which gc must reclaim once its parent is gone.
func TestGCTakesOrphanedForksAndLeavesForeignMachinesAlone(t *testing.T) {
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, _ := call(t, nil, "up"); code != 0 {
		t.Fatal("up")
	}
	var names []string
	if code, _ := call(t, &names, "fork", "--prepare", "--json"); code != 0 || len(names) != 1 {
		t.Fatalf("fork: %v", names)
	}
	// Something else's machine, with no boxer label at all.
	if err := client.Create(vm.CreateSpec{Name: "someone-elses-vm", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, nil, "down"); code != 0 {
		t.Fatal("down")
	}
	if code, out := call(t, nil, "gc", "--all"); code != 0 || !strings.Contains(out, "which is gone") {
		t.Fatalf("gc must reclaim an orphaned fork: %d %s", code, out)
	}
	all, err := client.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.Name == names[0] {
			t.Fatalf("the orphaned fork survived: %v", all)
		}
	}
	if _, ok, _ := client.Status("someone-elses-vm"); !ok {
		t.Fatal("gc deleted a machine boxer does not own")
	}
}

// --count says how many children this scope should have, so asking again for the same number is
// a no-op rather than an error — and the JSON stays an array, because a caller that indexes it
// would crash on a null.
func TestForkingTwiceForTheSameCountMakesNothingNew(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, _ := call(t, nil, "up"); code != 0 {
		t.Fatal("up")
	}
	var first []string
	if code, _ := call(t, &first, "fork", "--prepare", "--count", "2", "--json"); code != 0 || len(first) != 2 {
		t.Fatalf("fork: %v", first)
	}
	var again []string
	code, out := call(t, &again, "fork", "--count", "2", "--json")
	if code != 0 || len(again) != 0 {
		t.Fatalf("a repeated fork makes nothing new: %d %v %s", code, again, out)
	}
	var kids []map[string]any
	if code, _ := call(t, &kids, "fork", "ls", "--json"); code != 0 || len(kids) != 2 {
		t.Fatalf("there are still two: %v", kids)
	}
	if code, out := call(t, nil, "fork", "--count", "2"); code != 0 || !strings.Contains(out, "already has that many forks") {
		t.Fatalf("the prose form says so rather than crashing: %d %s", code, out)
	}
}

// `fork rm` takes children only. A sandbox is not a fork, and removing one through this command
// would delete somebody's worktree sandbox on the strength of a typo.
func TestForkRmRefusesAnythingThatIsNotAChild(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, _ := call(t, nil, "up"); code != 0 {
		t.Fatal("up")
	}
	if code, out := call(t, nil, "fork", "rm", "sb-000000000000"); code != 2 || !strings.Contains(out, "not a fork child") {
		t.Fatalf("a sandbox is not a fork: %d %s", code, out)
	}
	if code, out := call(t, nil, "fork", "rm"); code != 2 || !strings.Contains(out, "usage") {
		t.Fatalf("no name and no --all: %d %s", code, out)
	}
	if code, out := call(t, nil, "fork", "rm", "--all"); code != 0 || !strings.Contains(out, "no forks to remove") {
		t.Fatalf("--all with no children says so: %d %s", code, out)
	}
	// An unknown subcommand is a usage error, not a fork.
	if code, out := call(t, nil, "fork", "wat"); code == 0 || !strings.Contains(out, "fork") {
		t.Fatalf("unknown subcommand: %d %s", code, out)
	}
}

// A fork child shares its parent's worktree, which is already set up: running in the child must not
// run setup into the parent's files again.
func TestForkChildDoesNotRepeatSetup(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"setup = [\"echo x >> setup.runs\"]\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var names []string
	if code, out := call(t, &names, "fork", "--prepare", "--count", "1", "--json"); code != 0 || len(names) != 1 {
		t.Fatalf("fork: %d %s", code, out)
	}
	if code, out := call(t, nil, "stop", names[0]); code != 0 {
		t.Fatal(out)
	}
	if code, out := call(t, nil, "run", "--scope", names[0], "--", "true"); code != 0 {
		t.Fatal(out)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "setup.runs")); strings.Count(string(b), "x") != 1 {
		t.Fatalf("setup ran %d times", strings.Count(string(b), "x"))
	}
}
