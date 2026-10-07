package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDetect(t *testing.T) {
	main := t.TempDir()
	run(t, main, "init", "-q", "-b", "main")
	run(t, main, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")
	linked := filepath.Join(t.TempDir(), "wt")
	run(t, main, "worktree", "add", "-q", linked, "-b", "feature")

	gm, err := Detect(main)
	if err != nil || gm.Linked {
		t.Fatalf("main: %+v %v", gm, err)
	}
	gl, err := Detect(linked)
	if err != nil || !gl.Linked {
		t.Fatalf("linked: %+v %v", gl, err)
	}
	if gm.CommonDir != gl.CommonDir {
		t.Fatalf("common dir differs: %q vs %q", gm.CommonDir, gl.CommonDir)
	}
	if gn, err := Detect(t.TempDir()); err != nil || gn.Toplevel != "" {
		t.Fatalf("non-repo: %+v %v", gn, err)
	}
}

func TestResolve(t *testing.T) {
	g := Git{Toplevel: "/w/a", CommonDir: "/w/main/.git", Linked: true}
	g2 := Git{Toplevel: "/w/b", CommonDir: "/w/main/.git", Linked: true}
	id := Identity{SessionID: "s1", AgentID: "a1"}

	repoA, _ := Resolve("repo", "degrade", g, id)
	repoB, _ := Resolve("repo", "degrade", g2, id)
	if repoA.Key != repoB.Key {
		t.Fatal("repo isolation must share across worktrees")
	}
	wtA, _ := Resolve("worktree", "degrade", g, id)
	wtB, _ := Resolve("worktree", "degrade", g2, id)
	if wtA.Key == wtB.Key || wtA.Key == repoA.Key {
		t.Fatal("worktree isolation must differ per worktree")
	}
	s1, _ := Resolve("session", "degrade", g, id)
	s2, _ := Resolve("session", "degrade", g, Identity{SessionID: "s2"})
	if s1.Key == s2.Key {
		t.Fatal("session isolation must differ per session")
	}
	sub, _ := Resolve("subagent", "degrade", g, id)
	if sub.Key == s1.Key || sub.Isolation != "subagent" {
		t.Fatal("subagent must differ from its session")
	}

	d, err := Resolve("subagent", "degrade", g, Identity{SessionID: "s1"})
	if err != nil || !d.Degraded || d.Isolation != "session" || d.Key != s1.Key {
		t.Fatalf("degrade subagent→session: %+v %v", d, err)
	}
	d, err = Resolve("session", "degrade", g, Identity{})
	if err != nil || !d.Degraded || d.Isolation != "worktree" || d.Key != wtA.Key {
		t.Fatalf("degrade session→worktree: %+v %v", d, err)
	}
	if _, err := Resolve("session", "fail", g, Identity{}); err == nil {
		t.Fatal("on_missing_id=fail must error")
	}
	if _, err := Resolve("worktree", "degrade", Git{}, id); err == nil {
		t.Fatal("outside a repo must error")
	}
	if len(wtA.Key) != 15 || wtA.Key[:3] != "sb-" {
		t.Fatalf("key shape: %q", wtA.Key)
	}
}

// GitCommand runs git with the repository's own hooks, fsmonitor, pager and external diff turned
// off, so a config the guest can write cannot run a command when boxer reads the worktree's state.
func TestGitCommandRunsNothingTheConfigNames(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := GitCommand(dir, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")
	ran := filepath.Join(t.TempDir(), "ran")
	run("config", "core.fsmonitor", "touch "+ran)
	run("config", "core.pager", "touch "+ran)
	if out, err := GitCommand(dir, "status", "--porcelain").CombinedOutput(); err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("GitCommand ran a command the config named")
	}
	// It still answers: HEAD resolves.
	if out, err := GitCommand(dir, "rev-parse", "HEAD").CombinedOutput(); err != nil || len(out) < 7 {
		t.Fatalf("rev-parse: %v %s", err, out)
	}
}
