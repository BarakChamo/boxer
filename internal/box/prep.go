package box

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Prep runs commands on the *host*, in the worktree, before the sandbox exists.
//
// It is devcontainer's `initializeCommand`, which is the one lifecycle hook the specification
// puts on the host rather than in the container, and boxer's own `setup` is its in-guest twin.
// The reason to want it is that a dependency install on the host is roughly twice as fast as the
// same install in a guest — no virtualised filesystem, no cold package cache, every core.
//
// It is opt-in, and the reason is the whole story of this feature.
//
// A `node_modules` built on macOS contains `darwin-arm64` binaries that cannot execute in a Linux
// guest. `npm`, `bun` and `pnpm` can be told otherwise — `--os`, `--cpu`, `--libc` select the
// prebuilt artefact for another platform, and boxer derives the triple from the image so the
// caller does not have to. Verified: asking for linux/arm64/musl swaps `@next/swc-darwin-arm64`
// for `@next/swc-linux-arm64-musl`.
//
// What that cannot fix is a package that *compiles* at install time. node-gyp builds against the
// host's headers and toolchain and produces a host binary whatever the flags say, and the failure
// arrives much later, inside the guest, as `invalid ELF header`. So boxer does not enable this,
// warns when it sees a package that builds from source, and leaves `setup` — which runs in the
// guest and is always right — as the default answer.

// Prep runs the host-side preparation commands once per worktree.
//
// The marker is keyed to the commands and the target, exactly like `setup`'s, so changing either
// re-runs them and changing neither does not.
func (e *Env) Prep() error {
	if len(e.Cfg.Prep.Commands) == 0 {
		return nil
	}
	// A backend that copies the worktree instead of mounting it would never see what prep wrote.
	if !vm.CapsOf(e.VM).HostMounts {
		return vm.Unsupported(e.VM, "run host-side `prep`",
			"this backend does not mount the worktree, so prep's output would not reach the guest; move the commands to `setup`")
	}
	marker := prepMarkerPath(e.Scope.Root, e.worktreeKey(), e.Cfg)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	env := append(os.Environ(), e.prepEnv()...)
	start := time.Now()
	for _, line := range e.Cfg.Prep.Commands {
		fmt.Fprintf(e.Stderr, "boxer: prep (on the host): %s\n", line)
		cmd := exec.Command("sh", "-lc", line)
		cmd.Dir = e.Scope.Root
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = e.Stderr, e.Stderr
		if err := cmd.Run(); err != nil {
			e.event(obs.Provision, obs.Failed, time.Since(start), map[string]any{"kind": "prep"})
			return e.fail(&Error{
				Reason: fmt.Sprintf("prep failed on the host: %s: %v", line, err),
				Cause:  "PREP_FAILED", Scope: e.Scope,
				Fix: "fix the command in [prep], or move it to `setup` to run it in the guest",
			})
		}
	}
	e.event(obs.Provision, obs.OK, time.Since(start), map[string]any{"kind": "prep"})
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err == nil {
		_ = os.WriteFile(marker, []byte(strings.Join(e.Cfg.Prep.Commands, "\n")), 0o644)
	}
	return nil
}

// prepEnv exposes the guest's platform triple so a command can target it.
//
// Exposed as environment rather than spliced into the command line, because boxer must not
// rewrite what the user wrote: `npm ci $BOXER_TARGET_FLAGS` is the caller's decision to make, and
// a tool boxer has never heard of gets the same information without boxer inventing flags for it.
func (e *Env) prepEnv() []string {
	os_, cpu, libc := e.Target()
	return []string{
		"BOXER_TARGET_OS=" + os_,
		"BOXER_TARGET_CPU=" + cpu,
		"BOXER_TARGET_LIBC=" + libc,
		fmt.Sprintf("BOXER_TARGET_FLAGS=--os=%s --cpu=%s --libc=%s", os_, cpu, libc),
		"BOXER_WORKTREE=" + e.Scope.Root,
	}
}

// Target is the platform the guest runs, as npm and bun spell it.
//
// The libc half is the one that bites: an Alpine image links musl, and a package that ships only
// a glibc build will install happily and fail to load. The image name is the only signal boxer
// has, and it is a good one — every Alpine tag says so.
func (e *Env) Target() (os_, cpu, libc string) {
	if t := e.Cfg.Prep.Target; t != "" && t != "auto" && t != "none" {
		parts := strings.Split(t, "/")
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		return parts[0], parts[1], parts[2]
	}
	cpu = runtime.GOARCH // the guest matches the host: a microVM and a container both do
	image, _ := e.Image()
	libc = "glibc"
	if strings.Contains(image, "alpine") || strings.Contains(image, "musl") {
		libc = "musl"
	}
	return "linux", cpu, libc
}

// prepMarkerPath is keyed to the scope and to what prep says, the same shape `setup` uses: the
// work lands in the worktree, which no image can carry, so the record of having done it lives in
// the worktree's git directory and dies with the worktree rather than with the sandbox.
func prepMarkerPath(root, scopeKey string, cfg config.Config) string {
	// A fresh slice: appending to cfg.Prep.Commands would write into the caller's backing array.
	key := append(append([]string{}, cfg.Prep.Commands...), cfg.Prep.Target)
	sum := sha256.Sum256([]byte(strings.Join(key, "\x00")))
	return filepath.Join(worktreeMarkerDir(root), "prep-"+scopeKey+"-"+hex.EncodeToString(sum[:])[:4])
}

// buildsFromSource names dependencies that compile at install time.
//
// This is the tripwire for the whole feature. A package with an install script builds against the
// host's headers and toolchain, so it produces a host binary no matter what `--os`/`--cpu` say,
// and the failure surfaces much later and much further away: the guest loads the `.node` file and
// reports `invalid ELF header`, naming neither prep nor the package. Warning here, by name, at
// the moment the person configures it, is the difference between a feature and a trap.
//
// Deliberately a small hard-coded list rather than a lockfile parse. npm's lockfile records
// `hasInstallScript` only sometimes, and being wrong in the reassuring direction is worse than
// being incomplete — so this names the ones that are both common and certain, and the doc says it
// is not exhaustive.
var buildsFromSource = []string{
	"better-sqlite3", "node-pty", "canvas", "sharp", "bcrypt", "grpc",
	"node-sass", "sqlite3", "zeromq", "usb", "serialport", "re2",
}

// PrepWarnings reports dependencies that host-side prep will get wrong for this worktree.
func (e *Env) PrepWarnings() []string {
	if len(e.Cfg.Prep.Commands) == 0 || e.Cfg.Prep.Target == "none" {
		return nil
	}
	lock, err := os.ReadFile(filepath.Join(e.Scope.Root, "package-lock.json"))
	if err != nil {
		return nil
	}
	var parsed struct {
		Packages map[string]struct {
			OptionalDependencies map[string]string `json:"optionalDependencies"`
		} `json:"packages"`
	}
	_ = json.Unmarshal(lock, &parsed) // an unreadable lock falls back to the name check alone
	var found []string
	for _, name := range buildsFromSource {
		if !strings.Contains(string(lock), `"node_modules/`+name+`"`) {
			continue
		}
		// A package that ships its binaries as per-platform optional dependencies — sharp since
		// 0.33, through @img/sharp-* — gets the right ones from --os/--cpu/--libc, and compiles
		// only as a fallback that never runs. Warning about it was a false alarm, found by
		// running it: sharp installed by prep on a Mac renders an image in an Alpine guest.
		if len(parsed.Packages["node_modules/"+name].OptionalDependencies) > 0 {
			continue
		}
		found = append(found, name)
	}
	if len(found) == 0 {
		return nil
	}
	os_, _, libc := e.Target()
	verb, them := "builds", "it"
	if len(found) > 1 {
		verb, them = "build", "them"
	}
	return []string{fmt.Sprintf(
		"`prep` runs on the host, but %s %s from source and will be compiled for this machine rather than for %s/%s. "+
			"Move %s to `setup`, or set prep.target = \"none\" and install in the guest.",
		strings.Join(found, ", "), verb, os_, libc, them)}
}
