package box

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// fakeDocker records its argv and answers build, image inspect and save the way docker does.
func fakeDocker(t *testing.T, id string) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "docker.log")
	script := `#!/bin/sh
echo "$*" >> "` + log + `"
case "$1" in
  build) exit 0;;
  image) echo "sha256:` + id + `";;
  save) while [ $# -gt 0 ]; do [ "$1" = -o ] && { echo archive > "$2"; }; shift; done;;
esac
`
	bin := filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := BuildBin
	BuildBin = bin
	t.Cleanup(func() { BuildBin = old })
	return log
}

// On smolvm, `build` is built by the host's docker and booted from its saved archive, and a
// rebuild that changed nothing reuses the archive.
func TestBuildBootsTheSavedArchiveOnSmolvm(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	_, vmlog := vmtest.Install(t)
	dockerLog := fakeDocker(t, "0123456789abcdef0123")
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"build = \"Dockerfile\"\nvolumes = [\"pg:/var/lib/postgresql/data\"]\n")
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = io.Discard
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	tag := e.buildTag()
	b, _ := os.ReadFile(dockerLog)
	if !strings.Contains(string(b), "build -t "+tag+" -f "+filepath.Join(dir, "Dockerfile")+" "+dir) {
		t.Fatalf("the Dockerfile must be built with the worktree as context:\n%s", b)
	}
	archive := filepath.Join(filepath.Dir(LastUsedDir()), "images", tag+"-0123456789abcdef.tar")
	v, _ := os.ReadFile(vmlog)
	if !strings.Contains(string(v), archive) {
		t.Fatalf("smolvm must boot the saved archive %s:\n%s", archive, v)
	}
	if !strings.Contains(string(v), filepath.Join(VolumeDir(), e.Scope.Key, "pg")+":/var/lib/postgresql/data") {
		t.Fatalf("the named volume must be mounted:\n%s", v)
	}
	if strings.Contains(string(v), "pack create") {
		t.Fatalf("a local archive is not pulled, so it must not be packed:\n%s", v)
	}
	if img, why := e.Image(); img != archive || !strings.Contains(why, "Dockerfile") {
		t.Fatalf("doctor must say what was built: %q %q", img, why)
	}

	if err := e.Down(); err != nil {
		t.Fatal(err)
	}
	e.built = ""
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(dockerLog)
	if n := strings.Count(string(b), "save "); n != 1 {
		t.Fatalf("an unchanged image must not be saved again, saved %d times", n)
	}
	if n := strings.Count(string(b), "build "); n != 2 {
		t.Fatalf("the build runs at every create, since only docker knows what changed: %d", n)
	}
}

// A missing Dockerfile is a configuration error that names the file.
func TestBuildNamesAMissingDockerfile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	vmtest.Install(t)
	fakeDocker(t, "00")
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"build = \"docker/Dockerfile\"\n")
	e, _ := Resolve(dir, "", scope.Identity{})
	e.Stderr = io.Discard
	_, err := e.Ensure(true, false)
	if err == nil || !strings.Contains(err.Error(), "docker/Dockerfile") || !strings.Contains(err.Error(), "CONFIG_INVALID") {
		t.Fatalf("got %v", err)
	}
}

// The tag is shared by every worktree of a repository and changes with the Dockerfile.
func TestBuildTagFollowsTheDockerfile(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		_ = os.WriteFile(filepath.Join(d, "Dockerfile"), []byte("FROM alpine\n"), 0o644)
	}
	ea := &Env{Scope: scope.Scope{Root: a}}
	eb := &Env{Scope: scope.Scope{Root: b}}
	ea.Cfg.Build, eb.Cfg.Build = "Dockerfile", "Dockerfile"
	if ea.buildTag() != eb.buildTag() {
		t.Fatal("two worktrees with the same Dockerfile must share a tag")
	}
	_ = os.WriteFile(filepath.Join(b, "Dockerfile"), []byte("FROM debian\n"), 0o644)
	if ea.buildTag() == eb.buildTag() {
		t.Fatal("a changed Dockerfile must change the tag")
	}
}

// A container backend builds into its own store and boots the tag; a failed build says how to see
// the whole error; a builder that is not installed is named.
func TestBuildOnAContainerBackendAndItsFailures(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine\n"), 0o644)
	bin := t.TempDir()
	log := filepath.Join(bin, "log")
	os.WriteFile(filepath.Join(bin, "podman"), []byte("#!/bin/sh\necho \"$*\" >> "+log+"\n[ -f "+bin+"/fail ] && exit 1\nexit 0\n"), 0o755)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e := &Env{Scope: scope.Scope{Root: dir}, Stderr: io.Discard}
	e.Cfg.Build, e.Cfg.Backend, e.Cfg.BuildContext = "Dockerfile", "podman", "."
	tag, err := e.buildImage()
	if err != nil || tag != e.buildTag() {
		t.Fatalf("%q %v", tag, err)
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "build -t "+tag) || strings.Contains(string(b), "save") {
		t.Fatalf("podman builds into its own store and saves nothing:\n%s", b)
	}
	os.WriteFile(filepath.Join(bin, "fail"), nil, 0o644)
	if _, err := e.buildImage(); err == nil || !strings.Contains(err.Error(), "BUILD_FAILED") || !strings.Contains(err.Error(), "podman build -f Dockerfile .") {
		t.Fatalf("got %v", err)
	}
	e.Cfg.Backend = "container"
	if _, err := e.buildImage(); err == nil || !strings.Contains(err.Error(), "build needs container") {
		t.Fatalf("got %v", err)
	}
	if rel(dir, "/elsewhere/x") != "/elsewhere/x" {
		t.Fatal("a path outside the worktree is shown whole")
	}
}
