package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The fast path is only allowed to exist while it agrees with git exactly, because its answer is
// hashed into the machine name: disagreeing by one symlink renames every sandbox. So the test is
// differential rather than expectation-based — it asks both and compares.
func TestTheFastPathAgreesWithGitOrDeclinesToAnswer(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(real, "main")
	run(t, real, "init", "-q", main)
	run(t, main, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q",
		"--allow-empty", "-m", "init")
	sub := filepath.Join(main, "deep", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(real, "wt")
	run(t, main, "worktree", "add", "-q", linked, "-b", "side")

	link := filepath.Join(real, "link-to-main")
	if err := os.Symlink(main, link); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(real, "nothing")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{main, sub, linked, filepath.Join(linked, "."), link, outside} {
		want, err := detectGit(dir)
		if err != nil {
			t.Fatalf("git in %s: %v", dir, err)
		}
		got, ok := detectFast(dir)
		if !ok {
			continue // declining is always allowed; being wrong is not
		}
		if got != want {
			t.Errorf("%s:\n fast %+v\n git  %+v", dir, got, want)
		}
	}
}

// The layouts the fast path is not allowed to guess at. Each one changes discovery in a way it
// does not model, so the only correct answer is to hand back to git.
func TestTheFastPathDeclinesWhatItDoesNotModel(t *testing.T) {
	t.Setenv("GIT_DIR", "/nowhere")
	if _, ok := detectFast(t.TempDir()); ok {
		t.Error("answered with GIT_DIR set")
	}
	t.Setenv("GIT_DIR", "")
	os.Unsetenv("GIT_DIR")

	// A directory called .git that is not one.
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := detectFast(d); ok {
		t.Error("read an empty .git directory as a repository")
	}

	// A .git file that does not point anywhere usable.
	e := t.TempDir()
	if err := os.WriteFile(filepath.Join(e, ".git"), []byte("gitdir: /nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := detectFast(e); ok {
		t.Error("followed a dangling gitdir pointer")
	}

	// A .git that is a symlink, which git follows and this does not model.
	f := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(f, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, ok := detectFast(f); ok {
		t.Error("answered for a symlinked .git")
	}

	// A .git file that is not a gitdir pointer at all.
	g := t.TempDir()
	if err := os.WriteFile(filepath.Join(g, ".git"), []byte("not a pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := detectFast(g); ok {
		t.Error("read a .git file that says nothing")
	}

	// A gitdir pointer whose target exists but has no commondir.
	h := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(h, ".git"), []byte("gitdir: "+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := detectFast(h); ok {
		t.Error("answered for a linked worktree with no commondir")
	}

	// A main worktree whose git dir names a common dir elsewhere.
	i := t.TempDir()
	dot := filepath.Join(i, ".git")
	if err := os.MkdirAll(filepath.Join(dot, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"HEAD", "commondir"} {
		if err := os.WriteFile(filepath.Join(dot, f), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := detectFast(i); ok {
		t.Error("answered for a git dir with its own commondir")
	}

	// Outside any repository the walk reaches the root and says so, rather than declining.
	if g, ok := detectFast(t.TempDir()); !ok || g.Toplevel != "" {
		t.Errorf("outside a repository: %+v ok=%v, want an empty answer", g, ok)
	}
}
