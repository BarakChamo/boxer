package box

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildBin is the host builder smolvm uses for `build`; tests point it at a script.
var BuildBin = "docker"

// buildPaths resolves `build` and `build_context` against the worktree. The context defaults to
// the directory holding the Dockerfile, which is what `docker build -f` users expect for a file at
// the root and what devcontainer.json means by a Dockerfile beside it.
func (e *Env) buildPaths() (dockerfile, context string) {
	dockerfile = e.Cfg.Build
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(e.Scope.Root, dockerfile)
	}
	context = e.Cfg.BuildContext
	switch {
	case context == "":
		context = filepath.Dir(dockerfile)
	case !filepath.IsAbs(context):
		context = filepath.Join(e.Scope.Root, context)
	}
	return dockerfile, context
}

// buildTag names the image a Dockerfile builds. It hashes the file and where the context sits
// relative to the worktree, not the worktree's own path, so every worktree of a repository shares
// one tag and one set of layers.
func (e *Env) buildTag() string {
	dockerfile, context := e.buildPaths()
	b, _ := os.ReadFile(dockerfile)
	rel, _ := filepath.Rel(e.Scope.Root, context)
	sum := sha256.Sum256(append(b, "\x00"+rel...))
	return "boxer-" + hex.EncodeToString(sum[:6])
}

// buildImage builds `build` and returns what the backend boots: the tag on docker, podman and
// Apple container, which build into their own image store, and a `docker save` archive on
// smolvm, which boots archives but builds nothing. The build runs every time a sandbox is
// created, because only the builder knows whether a file the Dockerfile copies has changed, and
// its layer cache makes an unchanged build take a second.
func (e *Env) buildImage() (string, error) {
	dockerfile, context := e.buildPaths()
	if _, err := os.Stat(dockerfile); err != nil {
		return "", e.fail(&Error{Reason: "build names a Dockerfile that is not there: " + dockerfile, Cause: "CONFIG_INVALID",
			Scope: e.Scope, Fix: "fix `build` in boxer.toml; it is relative to the worktree"})
	}
	// The tag is per scope: two worktrees building at once each tag their own result, where one
	// shared tag could be re-pointed by the other between this build and its use. Layers are still
	// shared through the builder's cache, and the smolvm archive is named by content, so nothing
	// is built or stored twice.
	base := e.buildTag()
	tag := e.scopeTag()
	bin := BuildBin
	switch e.Cfg.Backend {
	case "docker", "podman", "container":
		bin = e.Cfg.Backend
	}
	if _, err := exec.LookPath(bin); err != nil {
		fix := "install " + bin
		if e.Cfg.Backend == "smolvm" || e.Cfg.Backend == "" {
			fix = "install docker to build the image, or build it yourself and set image = \"./image.tar\" to a `docker save` archive"
		}
		return "", e.fail(&Error{Reason: "build needs " + bin + " on the host, and it is not on PATH", Cause: "BUILD_FAILED", Scope: e.Scope, Fix: fix})
	}
	fmt.Fprintf(e.Stderr, "boxer: building %s from %s\n", tag, rel(e.Scope.Root, dockerfile))
	if err := e.hostRun(bin, "build", "-t", tag, "-f", dockerfile, context); err != nil {
		return "", e.fail(&Error{Reason: "image build failed: " + err.Error(), Cause: "BUILD_FAILED", Scope: e.Scope,
			Fix: "run `" + bin + " build -f " + rel(e.Scope.Root, dockerfile) + " " + rel(e.Scope.Root, context) + "` to see the whole error"})
	}
	if bin != BuildBin || (e.Cfg.Backend != "smolvm" && e.Cfg.Backend != "") {
		return tag, nil
	}
	// smolvm: an archive per image ID, so a rebuild that changed nothing reuses the file and one
	// that changed something gets a new path, and with it a new pack key.
	out, err := exec.Command(bin, "image", "inspect", "-f", "{{.Id}}", tag).Output()
	if err != nil {
		return "", e.fail(&Error{Reason: "reading the built image: " + errText(err), Cause: "BUILD_FAILED", Scope: e.Scope, Fix: "boxer doctor"})
	}
	id := strings.TrimPrefix(strings.TrimSpace(string(out)), "sha256:")
	if len(id) > 16 {
		id = id[:16]
	}
	dir := filepath.Join(filepath.Dir(LastUsedDir()), "images")
	archive := filepath.Join(dir, base+"-"+id+".tar")
	if _, err := os.Stat(archive); err == nil {
		now := time.Now()
		_ = os.Chtimes(archive, now, now) // its mtime is its last use, which the sweep reads
		return archive, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp := fmt.Sprintf("%s.%d.part", archive, os.Getpid()) // two creates saving at once each get their own
	if err := e.hostRun(bin, "save", "-o", tmp, tag); err != nil {
		_ = os.Remove(tmp)
		return "", e.fail(&Error{Reason: "saving the built image: " + err.Error(), Cause: "BUILD_FAILED", Scope: e.Scope, Fix: "boxer doctor"})
	}
	if err := os.Rename(tmp, archive); err != nil {
		return "", err
	}
	pruneArchives(dir, base, 2)
	return archive, nil
}

// pruneArchives keeps the newest keep archives of one tag. Each is the whole image, often
// hundreds of megabytes, and every edit to the Dockerfile's inputs writes another.
// ponytail: per tag by mtime; a sandbox still booted from an older archive keeps running, since
// smolvm unpacks it at create.
func pruneArchives(dir, tag string, keep int) {
	paths, _ := filepath.Glob(filepath.Join(dir, tag+"-*.tar"))
	sort.Slice(paths, func(i, j int) bool { return mtime(paths[i]).After(mtime(paths[j])) })
	for i, p := range paths {
		if i >= keep {
			_ = os.Remove(p)
		}
	}
}

func mtime(p string) time.Time {
	if st, err := os.Stat(p); err == nil {
		return st.ModTime()
	}
	return time.Time{}
}

func (e *Env) hostRun(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = e.Stderr, e.Stderr
	return cmd.Run()
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

// scopeTag is the tag this worktree's build is given: the shared base, then the scope.
func (e *Env) scopeTag() string {
	if k := strings.TrimPrefix(e.Scope.Key, "sb-"); k != "" {
		return e.buildTag() + "-" + k
	}
	return e.buildTag()
}

// forgetBuildTag removes this scope's image tag when its sandbox goes. Tags are per worktree, so
// without this every worktree that ever built left one behind; the layers stay in the builder's
// cache for the next build, and smolvm's archives are pruned on their own.
func (e *Env) forgetBuildTag() {
	if e.Cfg.Build == "" {
		return
	}
	bin := BuildBin
	switch e.Cfg.Backend {
	case "docker", "podman", "container":
		bin = e.Cfg.Backend
	}
	tag := e.scopeTag()
	verb := []string{"image", "rm", tag}
	if bin == "container" {
		verb = []string{"image", "delete", tag}
	}
	_ = exec.Command(bin, verb...).Run() // best effort: a tag already gone is fine
}
