package box

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// R-GUEST-1: the detector table is the whole of image selection when nothing is configured, and
// each row is a promise about a repository somebody has.
func TestImageDetectorTable(t *testing.T) {
	vmtest.Install(t)
	for _, tc := range []struct{ file, image string }{
		{"bun.lock", "oven/bun:1-debian"},
		{"bun.lockb", "oven/bun:1-debian"},
		{"pnpm-lock.yaml", "node:24-bookworm"},
		{"package-lock.json", "node:24-bookworm"},
		{"yarn.lock", "node:24-bookworm"},
		{"uv.lock", "python:3.12-bookworm"},
		{"requirements.txt", "python:3.12-bookworm"},
		{"pyproject.toml", "python:3.12-bookworm"},
		{"Cargo.lock", "rust:1-bookworm"},
		{"go.sum", "golang:1-bookworm"},
		{"go.mod", "golang:1-bookworm"},
	} {
		dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
		if err := os.WriteFile(filepath.Join(dir, tc.file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		e, err := Resolve(dir, "", scope.Identity{})
		if err != nil {
			t.Fatal(err)
		}
		if img, why := e.Image(); img != tc.image || !strings.Contains(why, tc.file) {
			t.Errorf("%s: image %s (%s), want %s", tc.file, img, why, tc.image)
		}
	}
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	e, _ := Resolve(dir, "", scope.Identity{})
	if img, why := e.Image(); img != "debian:bookworm-slim" || !strings.Contains(why, "no lockfile") {
		t.Errorf("no lockfile: %s (%s)", img, why)
	}
	// Configuration wins over detection, and a smolfile wins over an image.
	dir = vmtest.Repo(t, vmtest.NoWorktreeCheck+"image = \"alpine:3\"\n")
	e, _ = Resolve(dir, "", scope.Identity{})
	if img, why := e.Image(); img != "alpine:3" || !strings.Contains(why, "image from") {
		t.Errorf("configured image: %s (%s)", img, why)
	}
	dir = vmtest.Repo(t, vmtest.NoWorktreeCheck+"image = \"alpine:3\"\nsmolfile = \"Smolfile\"\n")
	e, _ = Resolve(dir, "", scope.Identity{})
	if img, why := e.Image(); img != "" || !strings.Contains(why, "smolfile") {
		t.Errorf("smolfile wins: %q (%s)", img, why)
	}
}

// Pulls happen in the guest, so the allowlist has to admit the registry the image names — and a
// registry serves its blobs from somewhere else, which is the part that is easy to get wrong.
func TestRegistryHosts(t *testing.T) {
	for _, tc := range []struct {
		image string
		first string
		want  string
	}{
		{"node:24", "registry-1.docker.io", "auth.docker.io"},
		{"library/node:24", "registry-1.docker.io", "production.cloudflare.docker.com"},
		{"ghcr.io/owner/img", "ghcr.io", "pkg-containers.githubusercontent.com"},
		{"mirror.gcr.io/library/node:24", "mirror.gcr.io", "storage.googleapis.com"},
		{"gcr.io/p/img", "gcr.io", "storage.googleapis.com"},
		{"public.ecr.aws/x/img", "public.ecr.aws", "d2glxqk2uabbnd.cloudfront.net"},
		{"registry.example.com:5000/img", "registry.example.com:5000", "registry.example.com:5000"},
	} {
		hosts := registryHosts(tc.image)
		if hosts[0] != tc.first || !slices.Contains(hosts, tc.want) {
			t.Errorf("%s: %v, want %s first and %s present", tc.image, hosts, tc.first, tc.want)
		}
	}
}

// A dropped transport is not an absent marker: read as one, setup ran again on a VM that had
// already run it.
func TestSetupMarkerProbeDistinguishesTransportFailure(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, "setup = [\"echo installing\"]\n"+vmtest.NoWorktreeCheck)
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = &bytes.Buffer{}
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	vmtest.FailExecOnce(t, "test -f /var/lib/boxer/setup-done")
	err = e.setup()
	be, ok := err.(*Error)
	if !ok || be.Cause != "TRANSPORT_FAILED" {
		t.Fatalf("want TRANSPORT_FAILED, got %v", err)
	}
}

// PackHarness stops the VM, packs it and starts it again; a pack that fails says so and leaves the
// VM running, because a caching failure must never cost the caller its sandbox.
func TestPackHarnessReportsFailureAndRestarts(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"integration = \"inside\"\n")
	e, err := Resolve(dir, "claude", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	e.Stderr = &out
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	vmtest.FailVerb(t, "pack create", "no space left on device")
	e.PackHarness()
	if !strings.Contains(out.String(), "harness cache failed") {
		t.Fatalf("a failed pack must be reported: %q", out.String())
	}
	if m, ok, _ := e.Exists(); !ok || !m.Running() {
		t.Fatalf("the VM must be running again after a failed pack: %+v", m)
	}
	image, _ := e.Image()
	if _, err := os.Stat(PackPath(harnessKey(image, "claude"))); err == nil {
		t.Fatal("a failed pack must leave no pack behind")
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "machine stop") || !strings.Contains(string(b), "machine start") {
		t.Fatalf("stop, pack, start:\n%s", b)
	}
}

// StalePacks is what `boxer gc` prunes by; its three rules are idle_timeout, the boxer.pack label,
// and the pack's own mtime.
func TestStalePacks(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	used, free := PackPath("used"), PackPath("free")
	for _, p := range []string{used, free} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(free, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(used, old, old); err != nil {
		t.Fatal(err)
	}
	ms := []vm.Machine{{Labels: map[string]string{vm.LabelPrefix + "pack": used}}}
	got := StalePacks(ms, time.Minute)
	if len(got) != 1 || got[0] != free {
		t.Fatalf("only the unreferenced, idle pack is stale: %v", got)
	}
	if got := StalePacks(ms, 0); got != nil {
		t.Fatalf(`idle_timeout = "never" prunes nothing: %v`, got)
	}
	if got := StalePacks(nil, 2*time.Hour); got != nil {
		t.Fatalf("packs younger than the timeout survive: %v", got)
	}
}
