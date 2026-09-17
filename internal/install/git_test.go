package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	return dir
}

func TestGitHookMergesIdempotentlyAndFiresOnWorktreeAdd(t *testing.T) {
	dir := gitRepo(t)
	hooks := filepath.Join(dir, "myhooks")
	os.MkdirAll(hooks, 0o755)
	exec.Command("git", "-C", dir, "config", "core.hooksPath", hooks).Run()
	existing := "#!/bin/sh\necho theirs >> " + filepath.Join(dir, "fired") + "\n"
	os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(existing), 0o755)
	if GitInstalled(dir) {
		t.Fatal("not installed yet")
	}
	for i := 0; i < 2; i++ {
		if _, err := Git(dir); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(hooks, "post-checkout"))
	s := string(b)
	if !strings.HasPrefix(s, existing) || strings.Count(s, gitBlockStart) != 1 || !strings.Contains(s, "boxer up --detach") {
		t.Fatalf("merge: %q", s)
	}
	if !GitInstalled(dir) {
		t.Fatal("GitInstalled after install")
	}
	// A fake boxer on PATH records that the hook ran it in the new worktree, with flag 1.
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "boxer"), []byte("#!/bin/sh\necho \"$PWD $*\" >> "+filepath.Join(dir, "fired")+"\n"), 0o755)
	wt := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "-C", dir, "worktree", "add", "-q", wt, "-b", "wt")
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	fired, _ := os.ReadFile(filepath.Join(dir, "fired"))
	wtReal, _ := filepath.EvalSymlinks(wt)
	if !strings.Contains(string(fired), "theirs") || !strings.Contains(string(fired), wtReal+" up --detach") {
		t.Fatalf("hook did not run boxer up in the new worktree: %q", fired)
	}
}

func TestGitHookCreatesFileWhenAbsent(t *testing.T) {
	dir := gitRepo(t)
	if _, err := Git(dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, ".git", "hooks", "post-checkout")
	b, err := os.ReadFile(p)
	if err != nil || !strings.HasPrefix(string(b), "#!/bin/sh\n"+gitBlockStart) {
		t.Fatalf("%v %q", err, b)
	}
	if st, _ := os.Stat(p); st.Mode()&0o111 == 0 {
		t.Fatal("hook must be executable")
	}
}
