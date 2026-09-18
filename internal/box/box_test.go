package box

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func TestRunProvisionsLazilyAndPropagatesExit(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, "setup = [\"echo installing\"]\n"+vmtest.NoWorktreeCheck)
	os.WriteFile(filepath.Join(dir, "bun.lock"), nil, 0o644)
	sub := filepath.Join(dir, "apps", "api")
	os.MkdirAll(sub, 0o755)

	e, err := Resolve(sub, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = &bytes.Buffer{}
	if img, why := e.Image(); img != "oven/bun:1-debian" || !strings.Contains(why, "bun.lock") {
		t.Fatalf("image detection: %s %s", img, why)
	}
	if e.GuestWorkdir() != "/workspace/apps/api" {
		t.Fatalf("workdir: %s", e.GuestWorkdir())
	}
	var out bytes.Buffer
	code, err := e.Run([]string{"sh", "-c", "echo ran; exit 7"}, RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out})
	if err != nil || code != 7 || !strings.Contains(out.String(), "ran") {
		t.Fatalf("run: %d %v %q", code, err, out.String())
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	if !strings.Contains(s, "machine create -n "+e.Scope.Key) || !strings.Contains(s, "pack create -I oven/bun:1-debian") || !strings.Contains(s, "--from ") || !strings.Contains(s, "--allow-host registry-1.docker.io") {
		t.Fatalf("create flags:\n%s", s)
	}
	if strings.Count(s, "echo installing") != 1 {
		t.Fatalf("setup should run once:\n%s", s)
	}
	// Second run: VM running, setup marker present, no create.
	code, err = e.Run([]string{"true"}, RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out})
	if err != nil || code != 0 {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(log)
	if strings.Count(string(b), "machine create") != 1 || strings.Count(string(b), "echo installing") != 1 {
		t.Fatalf("second run must reuse:\n%s", b)
	}
	if err := e.Down(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := e.Exists(); ok {
		t.Fatal("down should delete")
	}
}

func TestRunRefusesWhenCreateNotAllowed(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "create_on = [\"session_start\"]\n"+vmtest.NoWorktreeCheck)
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code, err := e.Run([]string{"true"}, RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out})
	be, ok := err.(*Error)
	if !ok || code != 1 || be.Cause != "NO_SANDBOX" || !strings.Contains(be.Error(), "fix:       boxer up") {
		t.Fatalf("want NO_SANDBOX error, got %d %v", code, err)
	}
	if !strings.HasPrefix(be.Error(), "boxer: ") || !strings.Contains(be.Error(), "scope:     sb-") {
		t.Fatalf("error contract: %q", be.Error())
	}
}

func TestSetupFailureDeletesVM(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "setup = [\"exit 5\"]\n"+vmtest.NoWorktreeCheck)
	e, _ := Resolve(dir, "", scope.Identity{})
	e.Stderr = &bytes.Buffer{}
	_, err := e.Ensure(true, false)
	be, ok := err.(*Error)
	if !ok || be.Cause != "SETUP_FAILED" {
		t.Fatalf("want SETUP_FAILED, got %v", err)
	}
	if _, exists, _ := e.Exists(); exists {
		t.Fatal("failed setup must delete the VM")
	}
}

func TestRequireWorktree(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"require\"\n")
	_, err := Resolve(dir, "", scope.Identity{})
	be, ok := err.(*Error)
	if !ok || be.Cause != "WORKTREE_REQUIRED" {
		t.Fatalf("want WORKTREE_REQUIRED, got %v", err)
	}
	dir = vmtest.Repo(t, "") // the default is "warn", so a main checkout resolves with a warning
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil || len(e.Warnings) != 1 {
		t.Fatalf("warn default: %v %v", err, e.Warnings)
	}
}

func TestHarnessOverrideAndInstructions(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n[harness.gemini-cli]\nmode = \"tool\"\n")
	e, _ := Resolve(dir, "gemini-cli", scope.Identity{})
	if e.Cfg.Mode != "tool" || !strings.Contains(e.Instructions(), "boxer_run tool") {
		t.Fatalf("override: %s\n%s", e.Cfg.Mode, e.Instructions())
	}
	e, _ = Resolve(dir, "claude-code", scope.Identity{})
	if e.Cfg.Mode != "rewrite" || !strings.Contains(e.Instructions(), "transparently") {
		t.Fatal("other harness keeps default")
	}
}

func TestGuestWorkdirThroughSymlinkedCwd(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n")
	os.MkdirAll(filepath.Join(dir, "apps", "web"), 0o755)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip(err)
	}
	e, err := Resolve(filepath.Join(link, "apps", "web"), "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	if e.GuestWorkdir() != "/workspace/apps/web" {
		t.Fatalf("workdir through symlink: %s", e.GuestWorkdir())
	}
}

func TestConcurrentEnsureCreatesOnce(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, err := Resolve(dir, "", scope.Identity{})
			if err != nil {
				t.Error(err)
				return
			}
			e.Stderr = &bytes.Buffer{}
			if _, err := e.Ensure(true, false); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	b, _ := os.ReadFile(log)
	if n := strings.Count(string(b), "machine create"); n != 1 {
		t.Fatalf("concurrent Ensure must create once, created %d times:\n%s", n, b)
	}
}

func TestOutsideRepo(t *testing.T) {
	vmtest.Install(t)
	_, err := Resolve(t.TempDir(), "", scope.Identity{})
	if be, ok := err.(*Error); !ok || be.Cause != "NO_REPOSITORY" {
		t.Fatalf("got %v", err)
	}
}

func TestCreatePacksImageOncePerHost(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n")
	for i := 0; i < 2; i++ {
		e, err := Resolve(dir, "", scope.Identity{})
		if err != nil {
			t.Fatal(err)
		}
		e.Stderr = &bytes.Buffer{}
		if _, err := e.Ensure(true, true); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	if strings.Count(s, "pack create") != 1 || strings.Count(s, "machine create") != 2 || strings.Count(s, "--from ") != 2 {
		t.Fatalf("want one pack, two creates from it, log:\n%s", s)
	}
}

func TestHarnessPackSkipsInstallOnNextVM(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\nintegration = \"inside\"\n")
	e, err := Resolve(dir, "claude", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = &bytes.Buffer{}
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	e.PackHarness()
	image, _ := e.Image()
	side := PackPath(harnessKey(image, "claude"))
	if _, err := os.Stat(side); err != nil {
		t.Fatalf("harness pack not written: %v", err)
	}
	e.PackHarness() // already packed: no second stop/pack
	if _, err := e.Ensure(true, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	if strings.Count(s, "pack create --from-vm") != 1 || !strings.Contains(s, "--from "+side) || !strings.Contains(s, "--label boxer.pack="+side) {
		t.Fatalf("want one from-vm pack, the recreate from it with a boxer.pack label, log:\n%s", s)
	}
	stale := StalePacks([]vm.Machine{{Labels: map[string]string{"boxer.pack": side}}}, time.Nanosecond)
	if len(stale) != 1 || stale[0] == side {
		t.Fatalf("referenced pack must survive, the image pack is stale: %v", stale)
	}
	if got := StalePacks(nil, 0); got != nil {
		t.Fatalf("idle_timeout never must prune nothing: %v", got)
	}
	if got := StalePacks(nil, time.Hour); got != nil {
		t.Fatalf("fresh packs are not stale: %v", got)
	}
}

func TestWorktreeDetectSharesRepoSandbox(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n[worktree]\nmanage = \"detect\"\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil || e.Scope.Isolation != "repo" || len(e.Warnings) != 1 {
		t.Fatalf("detect in main checkout: %v %+v %v", err, e.Scope, e.Warnings)
	}
	wt := filepath.Join(filepath.Dir(dir), "wt")
	cmd := exec.Command("git", "-C", dir, "worktree", "add", "-q", wt, "-b", "wt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	e, err = Resolve(wt, "", scope.Identity{})
	if err != nil || e.Scope.Isolation != "worktree" || len(e.Warnings) != 0 {
		t.Fatalf("detect in linked worktree: %v %+v %v", err, e.Scope, e.Warnings)
	}
}

func TestUpDetachedSpawnsWithoutWaiting(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n")
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "boxer")
	os.WriteFile(script, []byte("#!/bin/sh\nsleep 0.3\necho \"$@\" > "+marker+"\n"), 0o755)
	Executable = func() (string, error) { return script, nil }
	t.Cleanup(func() { Executable = os.Executable })
	e, err := Resolve(dir, "claude-code", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := e.UpDetached(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("UpDetached waited for the child")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil {
			if !strings.Contains(string(b), "up --harness claude-code") {
				t.Fatalf("child args: %q", b)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("detached child never ran")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Two boxer processes with different lock directories (a harness that strips XDG_STATE_HOME from
// its shell) both try to create the scope; smolvm rejects the second, which must wait, not fail.
func TestCreateWaitsForAConcurrentCreator(t *testing.T) {
	_, _ = vmtest.Install(t)
	dir := vmtest.Repo(t, "require_worktree = \"off\"\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = &bytes.Buffer{}
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	// The fake now holds this machine; a second create for the same name is what a racing
	// process sees. create() must return nil once Exists reports the machine.
	if err := e.create(); err != nil {
		t.Fatalf("second create should wait for the existing machine, got %v", err)
	}
}
