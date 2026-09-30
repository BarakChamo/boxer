// Package box is boxer's core: resolve where we are, make sure the VM for that scope exists, run
// things in it, take it down. Every subcommand and every hook goes through here.
package box

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/BarakChamo/boxer/internal/sh"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Error is the agent-readable refusal from requirements §3.6.
type Error struct {
	Reason string
	Cause  string
	Fix    string
	Scope  scope.Scope
}

func (e *Error) Error() string {
	wt := e.Scope.Root
	if wt == "" {
		wt = "none"
	}
	fix := e.Fix
	if fix == "" {
		fix = "no agent-side fix; ask the operator"
	}
	return fmt.Sprintf("boxer: %s\n  scope:     %s (%s, %s)\n  worktree:  %s\n  cause:     %s\n  fix:       %s",
		e.Reason, e.Scope.Slug(), e.Scope.Key, e.Scope.Isolation, wt, e.Cause, fix)
}

// Env is one resolved invocation.
type Env struct {
	CWD     string
	Harness string
	Cfg     config.Config
	Git     scope.Git
	Scope   scope.Scope
	ID      scope.Identity // what the harness said about the caller, for hooks that re-invoke boxer
	VM      vm.Backend
	Stderr  io.Writer
	// Warnings collected during resolution, printed by doctor and by run when relevant.
	Warnings []string
	// secretsWarned stops HostSecrets naming the same missing secret on every exec.
	secretsWarned bool
	// FromPack names a saved pack (`boxer pack save`) this sandbox must be created from, instead
	// of the automatic environment/harness/image ladder. It is set by `boxer pack use` only.
	FromPack string
	// keepID, when set, is the uid:gid the next create maps the host user to — rootless podman's
	// answer to a guest user that cannot otherwise write the worktree. See matchUserUID.
	keepID string
	// seen is the last machine Exists looked at, so a second reader in the same command does not
	// pay another smolvm start-up. See seenMachine; anything that changes the machine clears it.
	seen *vm.Machine
	// built is what buildImage produced this command: the tag, or on smolvm the archive, which
	// only exists once the build has run and so cannot be predicted by Image beforehand.
	built string
}

// Resolve builds an Env for cwd. harness may be empty; id fields may be empty.
// ConfigError is a boxer.toml or devcontainer.json that could not be loaded. It is not "boxer is not
// in use here": the repository asked for a sandbox and said how, so callers that route commands
// must refuse them rather than let them run on the host.
type ConfigError struct{ Err error }

func (c *ConfigError) Error() string { return c.Err.Error() }
func (c *ConfigError) Unwrap() error { return c.Err }

func Resolve(cwd, harness string, id scope.Identity) (*Env, error) {
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	// git reports symlink-resolved paths; cwd must match or the guest workdir falls back to the root.
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = r
	}
	g, err := scope.Detect(cwd)
	if err != nil {
		return nil, err
	}
	repoRoot := ""
	if g.CommonDir != "" {
		repoRoot = filepath.Dir(g.CommonDir)
	}
	cfg, err := config.Load(g.Toplevel, repoRoot)
	if err != nil {
		return nil, &ConfigError{Err: err}
	}
	if harness != "" {
		cfg = cfg.ForHarness(harness)
	}
	// The first thing that knows the configuration turns the event stream on, so every later
	// emission in this process — hooks, MCP, run — uses the repository's own [telemetry] policy.
	obs.ConfigureFrom(cfg.Telemetry)
	backend, err := vm.Default(cfg.Backend)
	if err != nil {
		return nil, err
	}
	e := &Env{CWD: cwd, Harness: harness, Cfg: cfg, Git: g, ID: id, VM: backend, Stderr: os.Stderr}
	if g.Toplevel == "" {
		return e, &Error{Reason: "not inside a git repository", Cause: "NO_REPOSITORY", Fix: "cd into a git worktree, or `git init`"}
	}
	if !g.Linked {
		switch cfg.RequireWorktree {
		case "require":
			return e, &Error{Reason: "a linked git worktree is required here", Cause: "WORKTREE_REQUIRED", Scope: scope.Scope{Root: g.Toplevel},
				Fix: "git worktree add ../<name> -b <branch> && cd ../<name>"}
		case "warn":
			e.Warnings = append(e.Warnings, "running in the main checkout; a linked worktree isolates this agent's files")
		}
	}
	tag := ""
	if cfg.Integration == "inside" {
		tag = "inside"
	}
	isolation := cfg.Isolation
	if cfg.Worktree.Manage == "detect" && !g.Linked && isolation == "worktree" {
		// The worktree may still appear (the agent runs `git worktree add` mid-session); until
		// it does, the main checkout shares the repository sandbox rather than owning one.
		isolation = "repo"
		e.Warnings = append(e.Warnings, "worktree.manage = detect: no linked worktree yet, sharing the repository sandbox")
	}
	s, err := scope.ResolveTagged(isolation, cfg.OnMissingID, g, id, tag)
	if err != nil {
		return e, &Error{Reason: err.Error(), Cause: "SCOPE_UNRESOLVED", Scope: scope.Scope{Root: g.Toplevel},
			Fix: "set isolation = \"worktree\" in boxer.toml, or pass --session/--agent"}
	}
	if s.Degraded {
		e.Warnings = append(e.Warnings, "isolation degraded: "+s.Reason)
	}
	e.Scope = s
	// After e.Scope is set: PrepWarnings reads the worktree to find what it would get wrong.
	e.Warnings = append(e.Warnings, e.PrepWarnings()...)
	e.event(obs.Resolve, obs.OK, 0, map[string]any{"isolation": s.Isolation, "degraded": s.Degraded, "warnings": len(e.Warnings)})
	return e, nil
}

// event records one event for this Env; the scope and harness are always the same two fields.
func (e *Env) event(name, outcome string, d time.Duration, payload map[string]any) {
	if !obs.On() {
		return
	}
	obs.Emit(obs.Event{Name: name, Scope: e.Scope.Key, Harness: e.Harness, Outcome: outcome, Duration: d, Payload: payload})
}

// fail records the refusal and returns it, so every error contract boxer produces is also an event.
func (e *Env) fail(err *Error) *Error {
	e.event(obs.Err, obs.Failed, 0, map[string]any{"cause": err.Cause, "reason": err.Reason})
	return err
}

// Inside reports whether the harness itself runs in the guest (integration = "inside").
func (e *Env) Inside() bool { return e.Cfg.Integration == "inside" }

// MountAt is the guest path of the worktree: the host path in inside mode, so every path the
// harness sees is valid on both sides; the configured mount otherwise.
func (e *Env) MountAt() string {
	if e.Inside() {
		return e.Scope.Root
	}
	return e.Cfg.MountAt
}

// InsideHooks are supplied by package inside to avoid an import cycle: extra volumes, hosts, and
// the default image for inside mode.
var InsideHooks struct {
	Mounts     func() []string
	AllowHosts func() []string
	Image      string
	// InstallLine returns the harness's install command; it is part of the harness pack key so a
	// changed install line invalidates old packs.
	InstallLine func(harness string) string
}

// Image returns the guest image and the reason it was chosen (R-GUEST-1).
func (e *Env) Image() (string, string) {
	if e.built != "" {
		return e.built, "built from " + e.Cfg.Build
	}
	if e.Cfg.Build != "" {
		return e.buildTag(), "built from " + e.Cfg.Build
	}
	if e.Cfg.Smolfile != "" {
		return "", "smolfile " + e.Cfg.Smolfile
	}
	if e.Cfg.Image != "" {
		return e.Cfg.Image, "image from " + e.Cfg.Sources["image"]
	}
	if e.Inside() && InsideHooks.Image != "" {
		return InsideHooks.Image, "inside mode default (node for the harness)"
	}
	for _, d := range detectors {
		if _, err := os.Stat(filepath.Join(e.Scope.Root, d.file)); err == nil {
			return d.image, "detected " + d.file
		}
	}
	return "debian:bookworm-slim", "no lockfile found; boxer base"
}

// detectors map a lockfile to a guest image. Every entry is the *slim* variant, and that is a
// performance decision with a real trade-off behind it.
//
// A microVM boots by attaching the image's filesystem, so image size is start-up time, paid on
// every `boxer up` and every recreate. Measured on an M4, `boxer up` with the pack already built
// (bench/DEVSERVER.md has the method and the first-pack-build times):
//
//	node:24-bookworm        9.5s
//	node:24-bookworm-slim   2.5s
//	node:24-alpine          1.7s
//	alpine:3.21             0.6s
//
// The fat variants were costing 6 seconds of start-up for compilers and manpages almost no
// project uses. The trade is that a slim image has no build toolchain: a dependency that compiles
// from source (node-gyp, a Python package with no wheel) fails where it used to work. The fix is
// one line of `image_setup`, which is snapshotted into the pack and so is paid once rather than
// per start — see docs/troubleshooting.md.
//
// Alpine is faster still, and is not the default: it is musl rather than glibc, so a prebuilt
// binary that only ships a glibc build stops working, and that failure is much harder to read
// than a missing compiler. `image = "node:24-alpine"` is one line for anyone who wants it.
var detectors = []struct{ file, image string }{
	// Ruby and JVM first: a Rails or Spring app often carries a package.json for its assets, and
	// the language that runs the server is the one the image has to have.
	{"Gemfile.lock", "ruby:3-slim-bookworm"},
	{"pom.xml", "maven:3-eclipse-temurin-21"},
	{"build.gradle", "gradle:8-jdk21"},
	{"build.gradle.kts", "gradle:8-jdk21"},
	{"bun.lock", "oven/bun:1-slim"},
	{"bun.lockb", "oven/bun:1-slim"},
	{"pnpm-lock.yaml", "node:24-bookworm-slim"},
	{"package-lock.json", "node:24-bookworm-slim"},
	{"yarn.lock", "node:24-bookworm-slim"},
	{"uv.lock", "python:3.12-slim-bookworm"},
	{"requirements.txt", "python:3.12-slim-bookworm"},
	{"pyproject.toml", "python:3.12-slim-bookworm"},
	{"Cargo.lock", "rust:1-slim-bookworm"},
	// golang publishes no slim tag; the alpine one is a musl build and Go binaries are static, so
	// the usual musl objection does not apply unless the build needs cgo.
	{"go.sum", "golang:1-alpine"},
	{"go.mod", "golang:1-alpine"},
}

// IsLocalImage reports whether image names something on this machine rather than a registry: a
// `docker save` archive, an extracted rootfs directory, or stdin. A repository that builds its own
// image with its own tooling hands boxer the result this way, which is how E2B and Modal work too:
// the build happens outside the sandbox runtime.
func IsLocalImage(image string) bool {
	return image == "-" || strings.HasPrefix(image, "./") || strings.HasPrefix(image, "../") ||
		strings.HasPrefix(image, "/") || strings.HasPrefix(image, "~/")
}

// registryHosts returns the hosts a pull of image needs, since pulls happen in the guest.
func registryHosts(image string) []string {
	// A local image is not pulled, so it needs no registry host. smolvm takes a `docker save`
	// archive, a rootfs directory, or stdin as an image, and opening Docker Hub for one of those
	// widens the allowlist for a fetch that never happens.
	if IsLocalImage(image) {
		return nil
	}
	host := "docker.io"
	if i := strings.Index(image, "/"); i > 0 && strings.ContainsAny(image[:i], ".:") {
		host = image[:i]
	}
	switch host {
	case "docker.io", "index.docker.io":
		return []string{"registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com", "index.docker.io"}
	case "ghcr.io":
		return []string{"ghcr.io", "pkg-containers.githubusercontent.com"}
	case "mirror.gcr.io", "gcr.io":
		// Google's Docker Hub mirror serves blobs from GCS; it has no anonymous pull quota to hit.
		return []string{host, "storage.googleapis.com"}
	case "public.ecr.aws":
		return []string{host, "d2glxqk2uabbnd.cloudfront.net"}
	default:
		return []string{host}
	}
}

// Exists reports the VM's current state without changing it.
func (e *Env) Exists() (vm.Machine, bool, error) {
	m, ok, err := e.VM.Status(e.Scope.Key)
	if err == nil && ok {
		e.seen = &m
	}
	return m, ok, err
}

// seenMachine returns the machine this Env has already looked at, asking smolvm only if it has
// not. Every smolvm invocation costs about 19ms of CLI start-up before it does anything, and a
// warm `boxer run` used to spend three of them: one in Ensure to check the sandbox is there, one
// for the command, and a third *after the command returned* purely to read a label for the run
// record. The third asked for state the first had already fetched in the same process.
//
// The memo is cleared wherever boxer changes the machine, so it can only ever be as stale as the
// process is long — which is one command.
func (e *Env) seenMachine() (vm.Machine, bool) {
	if e.seen != nil {
		return *e.seen, true
	}
	m, ok, err := e.Exists()
	return m, ok && err == nil
}

// Ensure makes the scope's VM exist and run. It creates only when allowed; `run` passes
// config.Has(CreateOn, "run"), hooks pass their own event. A per-scope file lock serialises
// concurrent callers (a SessionStart hook and the first tool call arrive together), because two
// smolvm creates or starts for one machine race into "connection closed".
func (e *Env) Ensure(allowCreate, recreate bool) (created bool, err error) {
	start := time.Now()
	defer func() {
		outcome := obs.OK
		if err != nil {
			outcome = obs.Failed
		}
		image, why := e.Image()
		e.event(obs.Provision, outcome, time.Since(start), map[string]any{"created": created, "image": image, "image_reason": why})
	}()
	// Provisioning is the moment worth sweeping at: it is when boxer is about to spend storage,
	// and when a host that has drifted full is most likely to be about to fail.
	e.ReclaimDetached()
	unlock, err := lockScope(e.Scope.Key)
	if err != nil {
		return false, err
	}
	defer unlock()
	// Host-side prep runs alongside provisioning rather than before it. It touches only the
	// worktree, and provisioning touches only the backend, so the two have nothing to contend
	// over — and a pack boot is ~1.6s of otherwise dead time to install into. Joined before
	// `setup`, which is the first step that could observe what prep wrote.
	prep := make(chan error, 1)
	go func() { prep <- e.Prep() }()
	var prepErr error
	var prepOnce sync.Once
	joinPrep := func() error {
		prepOnce.Do(func() { prepErr = <-prep })
		return prepErr
	}
	// Every exit from here must join, or the process can end while prep is still writing to the
	// worktree — including the early returns below.
	defer func() {
		if perr := joinPrep(); perr != nil && err == nil {
			err = perr
		}
	}()

	m, ok, err := e.Exists()
	if err != nil {
		return false, err
	}
	if ok && recreate {
		if err := e.deleteVM(); err != nil {
			return false, err
		}
		ok = false
	}
	if !ok {
		if !allowCreate {
			return false, e.fail(&Error{Reason: "no sandbox exists for this scope", Cause: "NO_SANDBOX", Scope: e.Scope, Fix: "boxer up"})
		}
		if err := e.create(); err != nil {
			return false, err
		}
		// A backend that cannot carry boxer's label gets the mark on boxer's side instead, so
		// `ls` and `gc` can still tell this machine apart from one boxer did not create.
		vm.RecordOwned(e.VM, e.Scope.Key)
		created = true
	} else if m.Running() {
		// A running sandbox's routes are registered, but the proxy serving them may not be: it
		// does not survive a reboot or a crash. Checking is a pid-file read; starting it is paid
		// only when it is actually down.
		if e.Cfg.URLs.Enabled && len(e.Cfg.Network.Ports) > 0 && proxyDown() {
			if err := ensureProxy(e.Scope.Root); err != nil {
				fmt.Fprintf(e.Stderr, "boxer: warning: %v\n", err)
			}
		}
		return false, nil
	}
	if err := e.VM.Start(e.Scope.Key); err != nil {
		return created, e.fail(&Error{Reason: "sandbox failed to start: " + err.Error(), Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
	}
	if err := e.awaitMount(10); err != nil {
		// A container can keep the stale mount for its whole life rather than for a second, so
		// waiting longer is not the fix: a new container gets a new mount. Only for one this call
		// created — recreating a sandbox someone else is using would pull it out from under them.
		if !created {
			return created, err
		}
		fmt.Fprintf(e.Stderr, "boxer: the new sandbox's worktree mount is stale; recreating it once\n")
		if derr := e.deleteVM(); derr != nil {
			return created, err
		}
		if cerr := e.create(); cerr != nil {
			return created, cerr
		}
		vm.RecordOwned(e.VM, e.Scope.Key)
		if serr := e.VM.Start(e.Scope.Key); serr != nil {
			return created, e.fail(&Error{Reason: "sandbox failed to start: " + serr.Error(), Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
		}
		if err := e.awaitMount(25); err != nil {
			return created, err
		}
	}
	if err := e.matchUserUID(); err != nil {
		return created, err
	}
	// URLs are registered alongside setup rather than after it: they depend only on the host
	// ports, which are fixed once the machine exists, and three portless calls are otherwise a
	// few hundred milliseconds added to every start. Output is held until the join so it cannot
	// interleave with setup's.
	if e.Cfg.URLs.Enabled {
		if m, ok, err := e.Exists(); err == nil && ok {
			var buf bytes.Buffer
			done := make(chan struct{})
			go func() { e.publishURLs(m, &buf); close(done) }()
			var once sync.Once
			defer once.Do(func() { <-done; _, _ = e.Stderr.Write(buf.Bytes()) })
		}
	}
	// Two halves, in order. The image half changes the guest and is snapshotted; the worktree half
	// prepares the files the host mounted, and a snapshot can never stand in for it.
	ranImageSetup, err := e.imageSetup()
	if err != nil {
		return created, err
	}
	if ranImageSetup {
		e.PackEnv()
	}
	// Join before setup, not at return: setup prepares the same worktree prep was writing to, and
	// the two running at once is a race over one directory.
	if err := joinPrep(); err != nil {
		return created, err
	}
	if err := e.setup(); err != nil {
		return created, err
	}
	// Services come after the snapshot: a pack should carry what is installed, not a process that
	// was running when it was taken.
	if err := e.startServices(); err != nil {
		return created, err
	}
	return created, e.waitReady()
}

func (e *Env) create() error {
	e.seen = nil // the machine is about to change; a memoised one would describe the old one
	if e.Cfg.Build != "" {
		built, err := e.buildImage()
		if err != nil {
			return err
		}
		e.built = built
	}
	image, why := e.Image()
	if image == "" && e.Cfg.Smolfile == "" {
		return e.fail(&Error{Reason: "no guest image could be chosen (" + why + ")", Cause: "NO_IMAGE", Scope: e.Scope,
			Fix: "set image = \"debian:bookworm-slim\" in boxer.toml"})
	}
	mem, err := config.MemoryMiB(e.Cfg.Memory)
	if err != nil {
		return e.fail(&Error{Reason: err.Error(), Cause: "CONFIG_INVALID", Scope: e.Scope,
			Fix: "set memory = \"4G\" in boxer.toml"})
	}
	hosts := e.Cfg.Network.AllowedHosts()
	if e.Cfg.Network.Mode == "allowlist" && image != "" {
		hosts = append(hosts, registryHosts(image)...)
	}
	labels := map[string]string{
		vm.LabelPrefix + "scope":       e.Scope.Key,
		vm.LabelPrefix + "isolation":   e.Scope.Isolation,
		vm.LabelPrefix + "root":        e.Scope.Root,
		vm.LabelPrefix + "integration": e.Cfg.Integration,
	}
	// Who this sandbox belongs to. The identity is already in the scope key, but a hash cannot be
	// read back: without these labels a listing can say "sb-4f2a" and not "Claude Code, session
	// 4f2a", which is the difference between a list and something a person can act on.
	for k, v := range map[string]string{"harness": e.Harness, "session": e.ID.SessionID, "agent": e.ID.AgentID} {
		if v != "" {
			labels[vm.LabelPrefix+k] = v
		}
	}
	// Ports are per worktree, and boxer.toml is committed: every worktree of a repository would
	// otherwise ask for the same host port, and the second one to start would fail with a message
	// about a busy address rather than about worktrees. "auto:3000" asks for a free host port
	// instead, and the mapping is recorded on the machine so `status` can say which one it got.
	ports, chosen, err := allocatePorts(e.Cfg.Network.Ports)
	if err != nil {
		return e.fail(&Error{Reason: err.Error(), Cause: "NO_FREE_PORT", Scope: e.Scope,
			Fix: "free a port, or give `network.ports` fixed host ports"})
	}
	for guest, host := range chosen {
		labels[vm.LabelPrefix+"port."+guest] = host
	}
	volumes := []string{e.Scope.Root + ":" + e.MountAt()}
	for _, m := range e.Cfg.Mounts {
		volumes = append(volumes, expandMount(m))
	}
	// Appended here rather than folded into cfg.Mounts, deliberately: EnvKey hashes cfg.Mounts,
	// and a package cache changes nothing about what a pack contains. Putting them in the key
	// would rebuild every environment the first time a developer's ~/.npm appeared.
	volumes = append(volumes, e.CacheMounts()...)
	// Named volumes are not in EnvKey either: they hold data, not the environment.
	named, err := e.volumeMounts()
	if err != nil {
		return e.fail(&Error{Reason: "creating the sandbox's volumes: " + err.Error(), Cause: "CREATE_FAILED", Scope: e.Scope, Fix: "boxer doctor"})
	}
	volumes = append(volumes, named...)
	if e.Inside() {
		if InsideHooks.Mounts != nil {
			volumes = append(volumes, InsideHooks.Mounts()...)
		}
		if InsideHooks.AllowHosts != nil && e.Cfg.Network.Mode == "allowlist" {
			hosts = append(hosts, InsideHooks.AllowHosts()...)
		}
	}
	from := ""
	if e.FromPack != "" {
		// A named pack is an explicit instruction, so it wins over the ladder below and a missing
		// one is an error rather than a silent fall back to pulling the image.
		p, err := NamedPackPath(e.FromPack)
		if err != nil {
			return err
		}
		if !packReady(p) {
			return &Error{Reason: fmt.Sprintf("no saved pack named %q", e.FromPack), Cause: "NO_SUCH_PACK",
				Scope: e.Scope, Fix: "boxer pack ls"}
		}
		from = p
	} else if e.Cfg.Smolfile == "" && image != "" {
		// Most specific first: an environment (setup already run), then a harness install, then
		// the bare image. Each is a superset of the one after it.
		if from = e.envPack(image); from == "" {
			if from = e.harnessPack(image); from == "" {
				from = e.packed(image)
			}
		}
	}
	if from != "" {
		labels[vm.LabelPrefix+"pack"] = from // the exact reference gc needs before pruning a pack
		now := time.Now()
		_ = os.Chtimes(from, now, now) // a pack's mtime is its last use
	}
	// Hostnames and address ranges are separate rules to smolvm. Validate has already refused
	// anything that is neither, so an error here is a list boxer built itself.
	egress, err := config.SplitAllow(hosts)
	if err != nil {
		return e.fail(&Error{Reason: err.Error(), Cause: "CONFIG_INVALID", Scope: e.Scope, Fix: "fix network.allow_hosts in boxer.toml"})
	}
	spec := vm.CreateSpec{
		Name:       e.Scope.Key,
		Image:      image,
		From:       from,
		Smolfile:   e.Cfg.Smolfile,
		Volumes:    volumes,
		Labels:     labels,
		CPUs:       e.Cfg.CPUs,
		MemoryMiB:  mem,
		Network:    e.Cfg.Network.Mode,
		AllowHosts: e.resolvable(egress.Hosts),
		AllowCIDRs: egress.CIDRs,
		DNS:        e.Cfg.Network.DNS,
		Ports:      ports,
		KeepID:     e.keepID,
	}
	if err = e.VM.Create(spec); err != nil && from != "" && !vm.IsAlreadyExists(err) && !vm.IsLocked(err) {
		// A pack can be truncated: an interrupted `pack create` leaves a file smaller than its
		// own footer, and every later create from it fails with the same unhelpful I/O error
		// until someone deletes it by hand. Delete it and pull the image instead.
		//
		// Only for an error that is actually about the file. This used to fire on *any* create
		// failure, which made it destructive under the load boxer is designed for: several
		// worktrees provisioning together lose a race on smolvm's store lock, and boxer deleted
		// the environment pack all of them were about to boot from and pulled the image instead.
		// Four parallel provisions cost 27s each rather than 2s, and the pack had to be rebuilt
		// afterwards. vm.Create now waits out the lock, and a lock error never condemns a pack.
		fmt.Fprintf(e.Stderr, "boxer: cached image unusable, pulling directly: %v\n", err)
		_ = os.Remove(from)
		spec.From = ""
		delete(labels, vm.LabelPrefix+"pack")
		spec.Labels = labels
		err = e.VM.Create(spec)
	}
	if err != nil {
		if vm.IsAlreadyExists(err) {
			// Another boxer (a hook, a detached warm-up, an MCP server) is creating this scope
			// under a different lock directory (a harness that strips XDG_STATE_HOME from its
			// shell environment, for one). Wait for it rather than fail the command.
			return e.awaitCreated()
		}
		// A refusal is not a failure to be retried or diagnosed — it is this backend saying it
		// cannot do what the configuration asks, and the fix is in the configuration. Keeping it
		// as CREATE_FAILED would send an agent to `boxer doctor` for something doctor cannot fix.
		if errors.Is(err, vm.ErrUnsupported) {
			return e.fail(&Error{Reason: err.Error(), Cause: "UNSUPPORTED", Scope: e.Scope,
				Fix: "change the setting it names, or set a backend that supports it"})
		}
		return e.fail(&Error{Reason: "sandbox could not be created: " + err.Error(), Cause: "CREATE_FAILED", Scope: e.Scope,
			Fix: "boxer doctor"})
	}
	return nil
}

// resolvable drops allowlist hosts that do not resolve, and says which. smolvm resolves every
// --allow-host when the machine is created and refuses the whole create if one fails — so a single
// hostname that has been retired turned every sandbox that listed it into CREATE_FAILED. That
// happened: statsig.anthropic.com stopped resolving, it was on the inside placement's list, and
// every inside sandbox on the host failed to start with an error about a host nobody had asked
// for. A host that does not resolve cannot be reached through the allowlist anyway; dropping it
// loses nothing but the failure. Address ranges never reach it: they are CIDR rules, split off before.
// lookupHost is the resolver resolvable asks; tests replace it, because some networks answer
// every name and a test that depends on the local resolver skips there and proves nothing.
var lookupHost = net.DefaultResolver.LookupHost

func (e *Env) resolvable(hosts []string) []string {
	if e.Cfg.Network.Mode != "allowlist" || len(hosts) == 0 {
		return hosts
	}
	ok := make([]bool, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		name := h
		if net.ParseIP(name) != nil {
			ok[i] = true
			continue
		}
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			addrs, err := lookupHost(ctx, name)
			ok[i] = err == nil && len(addrs) > 0
		}(i, name)
	}
	wg.Wait()
	var out, dropped []string
	for i, h := range hosts {
		if ok[i] {
			out = append(out, h)
		} else {
			dropped = append(dropped, h)
		}
	}
	if len(dropped) > 0 {
		fmt.Fprintf(e.Stderr, "boxer: warning: allowlist host(s) do not resolve and were left out: %s\n", strings.Join(dropped, ", "))
	}
	return out
}

// awaitCreated polls for a machine another process is creating for this scope.
func (e *Env) awaitCreated() error {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok, err := e.Exists(); err == nil && ok {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return &Error{Reason: "another boxer is still creating this sandbox", Cause: "CREATE_FAILED", Scope: e.Scope,
		Fix: "boxer doctor"}
}

// PackDir is where the host keeps its .smolmachine packs: one per image, one per image and
// harness in inside mode. BOXER_PACKS overrides it (the eval shares one across isolated state dirs).
func PackDir() string {
	if dir := os.Getenv("BOXER_PACKS"); dir != "" {
		return dir
	}
	return filepath.Join(filepath.Dir(LastUsedDir()), "packs")
}

// PackPath is the .smolmachine for a cache key (the image, or image and harness).
func PackPath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(PackDir(), hex.EncodeToString(sum[:8])+".smolmachine")
}

// EnvKey names the guest an environment produces: the image plus everything that changes what is
// installed in it. Two worktrees of the same repository share a key, which is the point — the
// second starts from the first one's pack with `setup` already run.
//
// It is a hash of the inputs, not of the result, exactly like a Docker layer: change a setup line
// and the key changes, so the old pack is no longer used and `gc` reclaims it. Only `setup` shapes
// it today; the keys that 1.1 adds (start, env, mounts) join it as they land.
func EnvKey(image string, cfg config.Config) string {
	key := image + "\x00env"
	// Only what the pack actually contains: the image and the commands that change it. `setup`
	// prepares the worktree, which no pack carries, so it does not belong in the key.
	for _, c := range cfg.ImageSetup {
		key += "\x00image_setup:" + c
	}
	for _, k := range sortedKeys(cfg.Env) {
		key += "\x00env:" + k + "=" + cfg.Env[k]
	}
	for _, m := range sortedCopy(cfg.Mounts) {
		key += "\x00mount:" + m
	}
	// `start` and `ready` do not change what is installed, but they do change what a VM made from
	// the pack is expected to be running, and a pack that disagrees with them is confusing rather
	// than useful.
	for _, c := range cfg.Start {
		key += "\x00start:" + c
	}
	if cfg.Ready != "" {
		key += "\x00ready:" + cfg.Ready
	}
	return key
}

// sortedKeys and sortedCopy keep the key stable: a map has no order, and a mount list reordered by
// hand is the same environment.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// envPack returns the pack of image with this repository's setup already run, when one exists.
func (e *Env) envPack(image string) string {
	if len(e.Cfg.ImageSetup) == 0 {
		return ""
	}
	side := PackPath(EnvKey(image, e.Cfg))
	if !packReady(side) {
		return ""
	}
	return side
}

// DropEnvPack deletes this environment's cached pack, so the next provision runs setup again. It
// returns the path it removed, or "" when there was nothing cached.
func (e *Env) DropEnvPack() (string, error) {
	// A rebuild means "prepare this from scratch", which includes the worktree half, whether or not
	// there is a pack to drop. This was once removed only alongside a pack, so on docker, podman
	// and Apple container, or with no image_setup, `--rebuild` never ran `setup` again.
	_ = os.Remove(setupMarkerPath(e.Scope.Root, e.Scope.Key, e.Cfg))
	image, _ := e.Image()
	if image == "" || len(e.Cfg.ImageSetup) == 0 {
		return "", nil
	}
	side := PackPath(EnvKey(image, e.Cfg))
	if !packReady(side) {
		return "", nil
	}
	if err := os.Remove(side); err != nil {
		return "", err
	}
	_ = os.Remove(strings.TrimSuffix(side, ".smolmachine") + ".lock")
	return side, nil
}

// PackEnv snapshots the scope's VM, setup already run, into the pack envPack looks for. Without
// it every new worktree repeats `bun install` from scratch while a pack of the bare image sits
// beside it, which is the most expensive thing about boxer before 1.1.
//
// smolvm packs only a stopped VM, so the VM is stopped and restarted around it: a few seconds,
// once per host per environment. Failure is reported and never blocks the run.
func (e *Env) PackEnv() {
	image, _ := e.Image()
	if image == "" || e.Cfg.Smolfile != "" || len(e.Cfg.ImageSetup) == 0 || e.Inside() {
		return
	}
	// Caching a prepared filesystem is optional. A backend without it re-runs `image_setup` for
	// every worktree, which is slower and still correct — so this returns quietly rather than
	// refusing, the same as every other reason this function declines.
	packer, ok := e.VM.(vm.Packer)
	if !ok {
		return
	}
	side := PackPath(EnvKey(image, e.Cfg))
	stub := strings.TrimSuffix(side, ".smolmachine")
	if packReady(side) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(side), 0o755); err != nil {
		return
	}
	unlock, err := lockFile(stub + ".lock")
	if err != nil {
		return
	}
	defer unlock()
	if packReady(side) { // packed while we waited
		return
	}
	if free, crowded := packingWouldCrowdTheDisk(e.minFree()); crowded {
		fmt.Fprintf(e.Stderr, "boxer: not caching this environment: %s free, below min_free_gb (%.1f GB). The next worktree runs setup again; `boxer gc` reclaims space.\n",
			humanBytes(free), e.Cfg.MinFreeGB)
		e.event(obs.Pack, obs.Skipped, 0, map[string]any{"kind": "env", "image": image, "free_bytes": free})
		return
	}
	fmt.Fprintln(e.Stderr, "boxer: caching this environment so the next worktree skips setup (once per host)")
	start := time.Now()
	err = e.stopVM()
	if err == nil {
		_, err = packer.PackFromVM(e.Scope.Key, stub)
	}
	if serr := e.VM.Start(e.Scope.Key); serr != nil && err == nil {
		err = serr
	}
	outcome := obs.OK
	if err != nil {
		outcome = obs.Failed
		fmt.Fprintf(e.Stderr, "boxer: environment cache failed: %v\n", err)
	}
	e.event(obs.Pack, outcome, time.Since(start), map[string]any{"kind": "env", "image": image, "path": side})
}

func harnessKey(image, harness string) string {
	key := image + "\x00" + harness
	if InsideHooks.InstallLine != nil {
		key += "\x00" + InsideHooks.InstallLine(harness)
	}
	return key
}

// harnessPack returns the pack of image with this harness installed when one exists, so a new
// inside-mode VM skips the install (R-GUEST-4).
// PortsOf reads back the guest-to-host port mapping a machine was created with.
func PortsOf(m vm.Machine) map[string]string {
	var out map[string]string
	for k, v := range m.Labels {
		if guest, found := strings.CutPrefix(k, vm.LabelPrefix+"port."); found {
			if out == nil {
				out = map[string]string{}
			}
			out[guest] = v
		}
	}
	return out
}

// allocatePorts turns the configured list into what smolvm takes, resolving any "auto:<guest>"
// entry to a free host port. It returns the resolved list and the guest-to-host mapping, so the
// machine can carry it as labels and a person can find out where their dev server actually is.
func allocatePorts(spec []string) (resolved []string, chosen map[string]string, err error) {
	chosen = map[string]string{}
	for _, p := range spec {
		host, guest, found := strings.Cut(p, ":")
		if !found || host != "auto" {
			resolved = append(resolved, p)
			continue
		}
		free, err := freePort()
		if err != nil {
			return nil, nil, fmt.Errorf("no free host port for guest port %s: %w", guest, err)
		}
		chosen[guest] = free
		resolved = append(resolved, free+":"+guest)
	}
	return resolved, chosen, nil
}

// freePort asks the operating system for one, which is the only answer that is true at the moment
// it is given. A scan of a range would be a guess, and two boxers starting together would make the
// same guess.
func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close() //nolint:errcheck // the listener exists only to learn a free port number
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// packReady reports whether a pack file is present and whole. A zero-length or truncated pack is
// what an interrupted `pack create` leaves behind, and smolvm reports it as a checkpoint footer
// error at create time rather than as a missing file. A pack that is present but truncated past
// that gets caught at create time instead, where the create retries without it.
func packReady(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}

func (e *Env) harnessPack(image string) string {
	if !e.Inside() || e.Harness == "" {
		return ""
	}
	side := PackPath(harnessKey(image, e.Harness))
	if !packReady(side) {
		return ""
	}
	return side
}

// PackHarness snapshots the scope's VM, harness installed, into the pack harnessPack looks for.
// smolvm packs only a stopped VM, so the VM is stopped and restarted around the pack (a few
// seconds, once per image and harness per host). Failure is reported and never blocks the run.
func (e *Env) PackHarness() {
	image, _ := e.Image()
	if !e.Inside() || e.Harness == "" || image == "" || e.Cfg.Smolfile != "" {
		return
	}
	packer, ok := e.VM.(vm.Packer)
	if !ok {
		return
	}
	side := PackPath(harnessKey(image, e.Harness))
	stub := strings.TrimSuffix(side, ".smolmachine")
	if packReady(side) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(side), 0o755); err != nil {
		return
	}
	unlock, err := lockFile(stub + ".lock")
	if err != nil {
		return
	}
	defer unlock()
	if packReady(side) { // packed while we waited
		return
	}
	if free, crowded := packingWouldCrowdTheDisk(e.minFree()); crowded {
		fmt.Fprintf(e.Stderr, "boxer: not caching the %s install: %s free, below min_free_gb (%.1f GB). The next worktree pays the install again; `boxer gc` reclaims space.\n",
			e.Harness, humanBytes(free), e.Cfg.MinFreeGB)
		e.event(obs.Pack, obs.Skipped, 0, map[string]any{"kind": "harness", "image": image, "free_bytes": free})
		return
	}
	fmt.Fprintf(e.Stderr, "boxer: caching %s with %s installed (once per host)\n", image, e.Harness)
	start := time.Now()
	err = e.stopVM()
	if err == nil {
		_, err = packer.PackFromVM(e.Scope.Key, stub)
	}
	if serr := e.VM.Start(e.Scope.Key); serr != nil && err == nil {
		err = serr
	}
	outcome := obs.OK
	if err != nil {
		outcome = obs.Failed
		fmt.Fprintf(e.Stderr, "boxer: harness cache failed: %v\n", err)
	}
	e.event(obs.Pack, outcome, time.Since(start), map[string]any{"kind": "harness", "image": image, "path": side})
	e.resumeServices("caching")
}

// StalePacks lists packs no machine references (by its boxer.pack label), by two rules that catch
// different things. Age: unused for longer than idle, which is `idle_timeout`; idle <= 0 disables
// it, matching `idle_timeout = "never"`. Count: beyond keepLast, oldest first, which is what
// catches an environment that changes often — a pack per version, every one of them young.
func StalePacks(ms []vm.Machine, idle time.Duration, keepLast int) []string {
	used := map[string]bool{}
	for _, m := range ms {
		used[m.Labels[vm.LabelPrefix+"pack"]] = true
	}
	packs, _ := filepath.Glob(filepath.Join(PackDir(), "*.smolmachine"))
	type pack struct {
		path string
		age  time.Time
	}
	var unused []pack
	for _, p := range packs {
		if st, err := os.Stat(p); err == nil && !used[p] {
			unused = append(unused, pack{p, st.ModTime()})
		}
	}
	sort.Slice(unused, func(i, j int) bool { return unused[i].age.After(unused[j].age) }) // newest first
	stale := map[string]bool{}
	for i, p := range unused {
		if idle > 0 && time.Since(p.age) > idle {
			stale[p.path] = true
		}
		if keepLast > 0 && i >= keepLast {
			stale[p.path] = true
		}
	}
	var out []string
	for _, p := range unused {
		if stale[p.path] {
			out = append(out, p.path)
		}
	}
	return out
}

// packed returns the cached .smolmachine for image, packing it on first use under a per-image
// lock. smolvm pulls a registry image again for every machine, and that pull is the slow, flaky
// step (rate limits, stalled blobs); packing moves it to once per image per host. Any failure
// falls back to a direct pull so a pack problem never blocks a sandbox. `boxer gc` prunes packs.
func (e *Env) packed(image string) string {
	// First, before the lock, the disk check and the "caching…" line. A backend with no pack
	// store does none of this, and announcing a cache it will not write is its own small lie.
	packer, ok := e.VM.(vm.Packer)
	if !ok {
		return ""
	}
	// A local archive or rootfs is not pulled, so a pack of it saves nothing, and smolvm refuses
	// to pack one: every create printed that refusal and paid for the attempt.
	if IsLocalImage(image) {
		return ""
	}
	side := PackPath(image)
	stub := strings.TrimSuffix(side, ".smolmachine")
	if packReady(side) {
		return side
	}
	if err := os.MkdirAll(filepath.Dir(side), 0o755); err != nil {
		return ""
	}
	unlock, err := lockFile(stub + ".lock")
	if err != nil {
		return ""
	}
	defer unlock()
	if packReady(side) { // packed while we waited
		return side
	}
	if free, crowded := packingWouldCrowdTheDisk(e.minFree()); crowded {
		fmt.Fprintf(e.Stderr, "boxer: not caching %s: %s free, below min_free_gb (%.1f GB). The image is pulled instead; `boxer gc` reclaims space.\n",
			image, humanBytes(free), e.Cfg.MinFreeGB)
		e.event(obs.Pack, obs.Skipped, 0, map[string]any{"kind": "image", "image": image, "free_bytes": free})
		return ""
	}
	fmt.Fprintf(e.Stderr, "boxer: caching %s (once per host)\n", image)
	start := time.Now()
	if _, err := packer.Pack(image, stub); err != nil {
		e.event(obs.Pack, obs.Failed, time.Since(start), map[string]any{"kind": "image", "image": image})
		fmt.Fprintf(e.Stderr, "boxer: image cache failed, pulling directly: %v\n", err)
		return ""
	}
	e.event(obs.Pack, obs.OK, time.Since(start), map[string]any{"kind": "image", "image": image, "path": side})
	return side
}

// GuestEnv is the repository's own `env` table, as KEY=VALUE. It is configuration, not secrets:
// it travels into the environment pack, while EnvPassthrough and Secrets are read from the host at
// run time and never snapshotted.
func (e *Env) GuestEnv() []string {
	out := make([]string, 0, len(e.Cfg.Env)+2)
	for _, k := range sortedKeys(e.Cfg.Env) {
		out = append(out, k+"="+e.Cfg.Env[k])
	}
	if PollsForHostEdits(e.Cfg.Backend, runtime.GOOS) {
		for _, kv := range [][2]string{{"CHOKIDAR_USEPOLLING", "1"}, {"WATCHPACK_POLLING", "true"}} {
			if _, set := e.Cfg.Env[kv[0]]; !set {
				out = append(out, kv[0]+"="+kv[1])
			}
		}
	}
	return out
}

// PollsForHostEdits reports a backend whose shared worktree does not deliver host edits the way
// dev-server watchers read them, so boxer turns on their polling mode by default. Measured with a
// Next.js 16 dev server, five host edits each: on smolvm, webpack and Turbopack both saw none;
// on Apple container and podman's Mac VM nothing inotify-based saw any; OrbStack's docker saw all
// of them. With WATCHPACK_POLLING webpack saw all five everywhere. Turbopack has no polling that
// works on these backends, which doctor says for a Next.js project.
func PollsForHostEdits(backend, goos string) bool {
	switch backend {
	case "", "smolvm", "container":
		return true
	case "podman":
		return goos == "darwin"
	}
	return false
}

// DeniedHosts lists the distinct hosts the egress allowlist refused at or after since, newest
// first, or nothing when the allowlist is not in play. Passing the zero time takes the whole
// recorded history, which is what `doctor` wants; a failed command passes its own start, because
// a message that says "during this command" has to mean it — the machine remembers denials from
// every command before it.
//
// It is a diagnostic: every failure to read it is swallowed, because a command that already
// failed must not also fail to explain itself.
func DeniedHosts(client vm.Backend, name string, cfg config.Config, since time.Time) []string {
	if cfg.Network.Mode != "allowlist" {
		return nil
	}
	reporter, ok := client.(vm.EgressReporter)
	if !ok {
		return nil // this backend does not record denials; the caller falls back to a plain failure
	}
	evs, err := reporter.Egress(name, 50)
	if err != nil {
		return nil
	}
	var out []string
	for i := len(evs) - 1; i >= 0; i-- {
		host := evs[i].Host()
		if host == "" || slices.Contains(out, host) {
			continue
		}
		// smolvm stamps whole seconds, so an event in the same second as the start counts: the
		// alternative is losing the denial from a command that failed immediately.
		if !since.IsZero() {
			at, perr := time.Parse(time.RFC3339, evs[i].Timestamp)
			if perr != nil || at.Before(since.Truncate(time.Second)) {
				continue
			}
		}
		out = append(out, host)
	}
	return out
}

// HostSecrets returns the GUEST=HOSTVAR pairs for `secrets` and `env_passthrough`, naming only the
// variables the host actually sets. Both are read from the host at run time, and neither may go
// through `-e KEY=VALUE`: that puts the value in smolvm's argv, where `ps` shows it to every other
// process on the machine. `--secret-env` passes the name and lets smolvm read the value itself.
func (e *Env) HostSecrets() []string {
	names := append(append([]string{}, e.Cfg.Secrets...), e.Cfg.EnvPassthrough...)
	slices.Sort(names)
	// A secret the repository declared but the host does not set is the likeliest reason a test
	// that needs it fails, and dropping it silently made that hard to see. env_passthrough is
	// optional by nature (CI is unset on a laptop), so only `secrets` are named.
	if !e.secretsWarned && e.Stderr != nil {
		for _, k := range e.Cfg.Secrets {
			if _, ok := os.LookupEnv(k); !ok {
				fmt.Fprintf(e.Stderr, "boxer: warning: secret %s is not set on the host, so the sandbox will not have it\n", k)
			}
		}
		e.secretsWarned = true
	}
	out := []string{}
	for _, k := range slices.Compact(names) {
		if _, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+k)
		}
	}
	return out
}

// expandMount resolves a leading ~ in the host half of a "host:guest[:ro]" mount, because that is
// where a dependency cache usually lives and smolvm does no shell expansion.
func expandMount(m string) string {
	if !strings.HasPrefix(m, "~/") {
		return m
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return m
	}
	return filepath.Join(home, m[2:])
}

// startLog is where a started service's own output goes, so a readiness failure can show it.
const startLog = "/tmp/boxer-start.log"

// startServices runs the `start` list detached, once per running VM. This is how a database or a
// dev server runs: `setup` installs it, `start` launches it, `ready` waits for it. A marker in
// memory-backed /tmp is exactly right here, because it must not survive a restart: a restarted VM
// has no processes and has to start them again.
//
// smolvm has a native persistent workload (`machine create --init`) and it is the wrong tool for
// this: it launches at machine *start*, which is before `setup` has run, so the very common
// `start = ["npm run dev"]` against a worktree that `setup` populates dies immediately and is
// never relaunched. The eval's flow tier proved exactly that. `start` means "after setup", and
// only a post-setup launch can mean it.
const startMarker = "/tmp/boxer-started"

func (e *Env) startServices() error {
	if len(e.Cfg.Start) == 0 {
		return nil
	}
	out, code, err := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "test -f "+startMarker)
	if err != nil || vm.TransportFailure(out) {
		return e.fail(&Error{Reason: "could not read the start marker: " + firstNonEmpty(errText(err), strings.TrimSpace(out)), Cause: "TRANSPORT_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate"})
	}
	if code == 0 {
		return nil
	}
	for i, cmd := range e.Cfg.Start {
		fmt.Fprintf(e.Stderr, "boxer: start: %s\n", cmd)
		// Detached and disowned: the command that launches a server must not wait for it.
		line := "cd " + e.MountAt() + " && " + superviseLine(i, cmd, e.Cfg.Restart)
		var out strings.Builder
		// Services see the same environment as `setup` and every `boxer run`: a dev server that
		// reads DATABASE_URL or a secret from [env] or `secrets` needs it when it starts.
		code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.MountAt(), User: e.Cfg.User,
			Env: e.GuestEnv(), SecretEnv: e.HostSecrets(),
			Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}, "sh", "-c", line)
		if err != nil || code != 0 {
			return e.fail(&Error{Reason: fmt.Sprintf("start step failed (exit %d): %s", code, cmd), Cause: "START_FAILED", Scope: e.Scope,
				Fix: "check the `start` list in boxer.toml, then: boxer up"})
		}
	}
	_, _, _ = vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "touch "+startMarker)
	return nil
}

// serviceDir holds one pid file per `start` line: the supervisor's pid, which is also its process
// group when setsid exists, so `boxer restart` can stop a service and everything it spawned.
const serviceDir = "/tmp/boxer-svc"

// superviseLine launches one `start` command detached, under a supervisor that restarts it when
// it crashes (restart = "on-failure", the default) or that only records it ("never").
//
// A service that exits 0 is taken to have finished and is not restarted. One that fails is
// restarted after 1s, then 2s, doubling to 30s; a run that lasted 10s or more resets the delay,
// and five quick failures in a row stop the loop, so a service that cannot start does not spin
// forever. Every restart is a line in the start log.
func superviseLine(i int, cmd, restart string) string {
	script := superviseScript(fmt.Sprintf("%s/%d.pid", serviceDir, i), cmd, restart)
	// setsid, where the image has it, makes the supervisor a process group of its own, so
	// StopServices can stop the service and everything it spawned with one signal. The whole list
	// is backgrounded, as a single `&`, so the exec that launches it returns at once.
	launch := `command -v setsid >/dev/null 2>&1 && exec setsid sh -c "$0"; exec sh -c "$0"`
	return "mkdir -p " + serviceDir + " && nohup sh -c " + sh.Quote(launch) + " " + sh.Quote(script) + " >>" + startLog + " 2>&1 &"
}

// superviseScript is the supervisor itself, a POSIX sh loop, so it needs nothing in the image.
func superviseScript(pid, cmd, restart string) string {
	run := shellWords(guestShell(cmd))
	label := sh.Quote(cmd)
	var script string
	if restart == "never" {
		script = "echo $$ > " + pid + "; exec " + run
	} else {
		script = "echo $$ > " + pid + "\n" +
			"fast=0; delay=1\n" +
			"while :; do\n" +
			"  began=$(date +%s)\n" +
			"  " + run + "\n" +
			"  code=$?\n" +
			"  [ \"$code\" = 0 ] && exit 0\n" +
			"  if [ $(( $(date +%s) - began )) -ge 10 ]; then fast=0; delay=1; else fast=$((fast + 1)); fi\n" +
			"  if [ \"$fast\" -ge 5 ]; then echo \"boxer: start: gave up after 5 quick crashes (exit $code): \"" + label + "; exit \"$code\"; fi\n" +
			"  echo \"boxer: start: exited $code; restarting in ${delay}s: \"" + label + "\n" +
			"  sleep \"$delay\"; delay=$((delay * 2)); [ \"$delay\" -gt 30 ] && delay=30\n" +
			"done"
	}
	return script
}

// StopServices stops every `start` service and its children, and clears the marker that says
// they are running, so the next startServices launches them again.
func (e *Env) StopServices() error {
	// Signal each supervisor's process group, then report any that is still alive. "stuck" is not
	// an error in the script: the caller decides what to do about it.
	stop := "for f in " + serviceDir + "/*.pid; do [ -f \"$f\" ] || continue; p=$(cat \"$f\"); " +
		"kill -TERM -\"$p\" 2>/dev/null || { pkill -TERM -P \"$p\" 2>/dev/null; kill -TERM \"$p\" 2>/dev/null; }; " +
		"sleep 0.2; kill -0 \"$p\" 2>/dev/null && echo stuck; rm -f \"$f\"; done; rm -f " + startMarker
	out, code, err := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", stop)
	if err != nil || code != 0 {
		return fmt.Errorf("stopping services: %s", firstNonEmpty(errText(err), strings.TrimSpace(out)))
	}
	if !strings.Contains(out, "stuck") {
		return nil
	}
	// A process boxer started could not be signalled from another exec. Docker on Ubuntu does
	// this: AppArmor stacks docker-default with unconfined for every `docker exec`, and
	// docker-default only accepts signals from plain docker-default, so even root is refused.
	// Stopping the sandbox ends every process in it and keeps its files, so it is the same
	// restart by other means.
	if err := e.VM.Stop(e.Scope.Key); err != nil {
		return fmt.Errorf("stopping services: a service would not stop, and neither would the sandbox: %w", err)
	}
	if err := e.VM.Start(e.Scope.Key); err != nil {
		return fmt.Errorf("stopping services: restarting the sandbox: %w", err)
	}
	e.seen = nil
	_, _, err = vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "rm -rf "+serviceDir+" "+startMarker)
	return err
}

// RestartServices stops the `start` services and launches them again, then waits for `ready`,
// without recreating the sandbox: whatever the services keep in the guest, such as a database, stays.
func (e *Env) RestartServices() error {
	m, ok, err := e.Exists()
	if err != nil {
		return err
	}
	if !ok || !m.Running() {
		return e.fail(&Error{Reason: "this worktree's sandbox is not running", Cause: "NO_SANDBOX", Scope: e.Scope, Fix: "boxer up"})
	}
	if err := e.StopServices(); err != nil {
		return e.fail(&Error{Reason: err.Error(), Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
	}
	if err := e.startServices(); err != nil {
		return err
	}
	return e.waitReady()
}

// startTail is the service's own last words, which are the whole diagnosis when a sandbox does
// not come up.
func (e *Env) startTail() string {
	out, _, err := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "tail -n 5 "+startLog)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// waitReady polls the `ready` command until it exits zero. A server accepts connections when it
// accepts them; a fixed sleep is either too short, which fails, or too long, which everyone pays.
func (e *Env) waitReady() error {
	if e.Cfg.Ready == "" {
		return nil
	}
	timeout, err := time.ParseDuration(e.Cfg.ReadyTimeout)
	if err != nil || timeout <= 0 {
		timeout = time.Minute
	}
	deadline := time.Now().Add(timeout)
	// Poll tight and then back off. A fixed 250ms interval spends, on average, half of it waiting
	// after the service is already up — real money against a bring-up measured in single-digit
	// seconds, and most of the total for something that starts quickly. The cap is the old
	// interval, so a service that takes a minute is polled no harder than before.
	wait := 25 * time.Millisecond
	for {
		var probe bytes.Buffer
		if code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.MountAt(), User: e.Cfg.User,
			Env: e.GuestEnv(), SecretEnv: e.HostSecrets(),
			Stdin: strings.NewReader(""), Stdout: &probe, Stderr: &probe}, guestShell(e.Cfg.Ready)...); err == nil && code == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			// Show the service's own last words rather than telling the reader where to find
			// them: by the time this fails, whatever the server printed is the whole diagnosis.
			reason := fmt.Sprintf("the sandbox never became ready: %q did not succeed within %s", e.Cfg.Ready, timeout)
			if out := e.startTail(); out != "" {
				reason += "\n  service:   " + strings.ReplaceAll(out, "\n", "\n             ")
			}
			return e.fail(&Error{Reason: reason, Cause: "NOT_READY", Scope: e.Scope,
				Fix: "check the `start` list and the `ready` command in boxer.toml"})
		}
		time.Sleep(wait)
		if wait < 250*time.Millisecond {
			wait = wait * 3 / 2
		}
	}
}

// imageSetupMarker lives in the guest, so it travels in the pack: that is exactly what lets a new
// VM skip work that is already in the image.
const imageSetupMarker = "/var/lib/boxer/image-setup-done"

// setup prepares the worktree the host mounted — installing dependencies into it, usually. Its
// marker is on the host, keyed to the scope and to what the setup list says, because the work
// lands in the worktree rather than in the guest: a pack cannot carry it, and a VM recreated for
// the same worktree should not repeat it.
func (e *Env) setup() error {
	if len(e.Cfg.Setup) == 0 {
		return nil
	}
	marker := setupMarkerPath(e.Scope.Root, e.Scope.Key, e.Cfg)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	for _, cmd := range e.Cfg.Setup {
		fmt.Fprintf(e.Stderr, "boxer: setup: %s\n", cmd)
		stepStart := time.Now()
		code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.MountAt(), Env: e.GuestEnv(),
			SecretEnv: e.HostSecrets(), User: e.Cfg.User,
			Stdin: strings.NewReader(""), Stdout: e.Stderr, Stderr: e.Stderr}, guestShell(cmd)...)
		if err != nil || code != 0 {
			// The VM stays: the guest is fine and the next attempt should not pay for a new one.
			// An image-setup failure is the opposite — a half-built guest is worth throwing away.
			return e.fail(e.setupError("setup", "`setup`", code, cmd, stepStart))
		}
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return err
	}
	return os.WriteFile(marker, nil, 0o644)
}

// awaitMount waits until the worktree mount answers in a sandbox that has just started. On docker
// (measured on OrbStack) a container created moments after the same path was removed and added
// again — what an orchestrator does when it reuses a task name — can see a stale mount for up to a
// second: `chdir to cwd ("/workspace") … no such file or directory`, and the first `setup` step
// failed for a reason that had nothing to do with it. One exec when the mount is fine; a few
// retries when it is not.
// mountProbe proves the worktree mount is the live one: its .git is there, and a file written
// through it can be removed through it.
const mountProbe = "test -e .git && p=.boxer-mount-probe-$$ && : > \"$p\" && rm -f \"$p\""

func (e *Env) awaitMount(tries int) error {
	var last error
	passed := 0
	for i := 0; i < tries; i++ {
		// Not just "the directory exists": a stale mount can answer for the path and still be the
		// directory that was removed, and it can flap — readable on one exec, gone on the next, so
		// a probe that saw .git was followed by a setup step that could not create a file. So the
		// probe does what setup will do, a write, and must pass twice running. The file lives for
		// milliseconds, before any setup step or service has started.
		_, code, err := vm.Output(e.VM, e.Scope.Key, e.MountAt(), "sh", "-c", mountProbe)
		if err == nil && code == 0 {
			if passed++; passed >= 2 {
				return nil
			}
			continue
		}
		passed = 0
		last = err
		if last == nil {
			last = fmt.Errorf("exit %d", code)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return e.fail(&Error{Reason: fmt.Sprintf("the worktree mount at %s never became usable: %v", e.MountAt(), last),
		Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
}

// matchUserUID makes sure the configured guest user can write the worktree, giving it the
// mount owner's uid when it cannot. Without it `user = "node"` on smolvm could read the worktree
// and write nothing to it: the mount is 501:755 and node is 1000, so setup's `npm install` failed
// on the first file. devcontainer calls this updateRemoteUserUID and defaults it on, for exactly
// this reason. It runs as root, once per VM — the marker lives in the guest and travels in a pack.
func (e *Env) matchUserUID() error {
	u := e.Cfg.User
	if u == "" || u == "root" || u == "0" || strings.ContainsAny(u, ":") {
		return nil
	}
	if _, err := strconv.Atoi(u); err == nil {
		return nil // a numeric user is already a uid; it is the caller's to choose
	}
	// Asked of the mount, not of the backend: whether the user can write it depends on how this
	// runtime maps ownership on this host, which a capability flag guessed wrong. smolvm shows the
	// worktree owned by the host uid; OrbStack maps it to whoever asks; rootless podman on Linux
	// maps the host user to the container's root, so it looks root-owned inside.
	marker := "/var/lib/boxer/uid-" + u
	probe := fmt.Sprintf(`test -f %[2]s && exit 0
su -s /bin/sh %[1]s -c 'p=.boxer-user-probe-$$; : > "$p" && rm -f "$p"' 2>/dev/null && exit 0
stat -c '%%u %%g' . 2>/dev/null || echo unknown`, u, marker)
	out, code, err := vm.Output(e.VM, e.Scope.Key, e.MountAt(), "sh", "-c", probe)
	if err != nil {
		return e.fail(&Error{Reason: "could not check whether " + u + " can write the worktree: " + err.Error(), Cause: "SETUP_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate"})
	}
	owner, group, _ := strings.Cut(strings.TrimSpace(out), " ")
	if code == 0 && owner == "" {
		return nil // the user can already write it, or this sandbox was set up before
	}
	if owner == "0" && e.VM.Name() == "podman" && e.keepID == "" {
		// Rootless podman maps the host user to the container's root. podman's own answer is
		// keep-id, which maps it to a chosen user instead — but the uid has to be known when the
		// container is created, and it lives in the image. So read it, and create the sandbox
		// again with the mapping. Once: a second failure is refused below.
		// ponytail: every new sandbox with a `user` pays this extra create (about a second on
		// podman); remember the uid per image and user if that ever matters.
		ids, _, ierr := vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", "id -u "+sh.Quote(u)+" && id -g "+sh.Quote(u))
		f := strings.Fields(ids)
		if ierr == nil && len(f) == 2 {
			e.keepID = f[0] + ":" + f[1]
			fmt.Fprintf(e.Stderr, "boxer: recreating the sandbox so %s (uid %s) owns the worktree, as rootless podman needs\n", u, f[0])
			if err := e.deleteVM(); err != nil {
				return err
			}
			if err := e.create(); err != nil {
				return err
			}
			vm.RecordOwned(e.VM, e.Scope.Key)
			if err := e.VM.Start(e.Scope.Key); err != nil {
				return e.fail(&Error{Reason: "sandbox failed to start: " + err.Error(), Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
			}
			if err := e.awaitMount(25); err != nil {
				return err
			}
			return e.matchUserUID()
		}
	}
	if owner == "0" {
		return e.fail(&Error{
			Reason: fmt.Sprintf("guest user %q cannot write the worktree: this runtime maps you to the container's root, so the "+
				"worktree is root's inside it (rootless podman does this)", u),
			Cause: "UNSUPPORTED", Scope: e.Scope,
			Fix: "remove `user` (commands then run as the container's root, which is you on the host), or run podman rootful"})
	}
	uid, cerr := strconv.Atoi(owner)
	if cerr != nil {
		return e.fail(&Error{Reason: fmt.Sprintf("guest user %q cannot write the worktree, and its owner could not be read: %q", u, owner),
			Cause: "SETUP_FAILED", Scope: e.Scope, Fix: "check that `user` names a user the image has, or remove `user`"})
	}
	// The group too: on Linux smolvm's file server runs as the host user and cannot give a new
	// file a group it is not in, so a user whose primary group is the image's gets EPERM on
	// every create even with the right uid. Kept as it is when the owner's group is unknown.
	gid := "$oldg"
	if g, gerr := strconv.Atoi(group); gerr == nil {
		gid = strconv.Itoa(g)
	}
	// busybox has no usermod, so the passwd line is edited directly; the home directory follows.
	script := fmt.Sprintf(`set -e
old=$(id -u %[1]s); oldg=$(id -g %[1]s)
if [ "$old:$oldg" != "%[2]d:%[4]s" ]; then
  sed -i "s/^%[1]s:\([^:]*\):$old:$oldg:/%[1]s:\1:%[2]d:%[4]s:/" /etc/passwd
  home=$(awk -F: '$1=="%[1]s"{print $6}' /etc/passwd)
  [ -n "$home" ] && [ -d "$home" ] && chown -R %[2]d:%[4]s "$home"
fi
mkdir -p /var/lib/boxer && touch %[3]s`, u, uid, marker, gid)
	out, code, err = vm.Output(e.VM, e.Scope.Key, "", "sh", "-c", script)
	if err != nil || code != 0 {
		return e.fail(&Error{Reason: fmt.Sprintf("could not give guest user %q the worktree owner's uid %d: %s", u, uid, firstNonEmpty(errText(err), strings.TrimSpace(out))),
			Cause: "SETUP_FAILED", Scope: e.Scope, Fix: "check that `user` names a user the image has, or set user = \"root\""})
	}
	return nil
}

// guestShell is the argv that runs a configured command line — setup, image_setup, start, ready —
// in the guest: a login shell, so profile scripts a toolchain relies on are read, with the image's
// own PATH put back in front.
//
// A plain `sh -lc` lost it. Alpine's /etc/profile assigns PATH outright, so every directory the
// image added with ENV PATH vanished: `go run .` in `start` on golang:alpine failed with "go: not
// found", while `boxer run -- go version` in the same sandbox worked, because `run` does not use a
// login shell. Node images keep node in /usr/local/bin, which the profile does set, so nothing
// built on them ever noticed.
func guestShell(cmd string) []string {
	return []string{"sh", "-c", `exec sh -lc 'PATH="$1:$PATH"; export PATH; eval "$2"' boxer "$PATH" "$0"`, cmd}
}

// shellWords quotes argv for a shell line.
func shellWords(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = sh.Quote(a)
	}
	return strings.Join(q, " ")
}

// quoteList renders hosts as a TOML array body: "a", "b".
func quoteList(hosts []string) string {
	q := make([]string, len(hosts))
	for i, h := range hosts {
		q[i] = strconv.Quote(h)
	}
	return strings.Join(q, ", ")
}

// setupError explains a failed setup step, and in particular explains exit 137, which is the
// guest's out-of-memory killer rather than anything the command did wrong. A person reading
// "exit 137" learns nothing; a person reading "the guest ran out of memory" knows to raise
// `memory` or run fewer sandboxes at once, which is the actual fix. `npm install` in several
// sandboxes at the same time is the way most people will meet it.
func (e *Env) setupError(what, key string, code int, cmd string, since time.Time) *Error {
	reason := fmt.Sprintf("%s step failed (exit %d): %s", what, code, cmd)
	fix := "fix the " + key + " list in boxer.toml, then: boxer up"
	// An install that failed under the allowlist almost always failed because of it, and the
	// command cannot say so — npm reports ENOTFOUND for a host it was never allowed to resolve.
	// `boxer run` already names the refused host; setup did not, and sent people to rewrite a
	// setup list that was fine. Checked before the out-of-memory case, which it is not.
	if denied := DeniedHosts(e.VM, e.Scope.Key, e.Cfg, since); len(denied) > 0 && code != 137 {
		return &Error{Reason: fmt.Sprintf("%s step failed (exit %d) because the egress allowlist refused %s: %s", what, code, strings.Join(denied, ", "), cmd),
			Cause: "SETUP_FAILED", Scope: e.Scope,
			Fix: fmt.Sprintf("add %s to network.allow_hosts in boxer.toml, then: boxer up", quoteList(denied))}
	}
	if code == 137 {
		reason = fmt.Sprintf("%s step was killed, out of memory (exit 137): %s", what, cmd)
		fix = fmt.Sprintf("raise `memory` in boxer.toml (this sandbox has %s), or run fewer sandboxes at once", firstNonEmpty(e.Cfg.Memory, "the default"))
	}
	return &Error{Reason: reason, Cause: "SETUP_FAILED", Scope: e.Scope, Fix: fix}
}

// setupMarkerPath names the record that this worktree has been prepared for this setup list.
// Changing the list changes the path, so the new commands run.
//
// It lives in the worktree's own git directory, not in boxer's state. It used to be in the state
// directory, keyed by the worktree's path — and a worktree removed and added again at the same
// path, which is what an orchestrator does with a task name it reuses, inherited a marker saying
// "set up" and no node_modules. The git directory goes with the worktree, so the record does too.
func setupMarkerPath(root, scopeKey string, cfg config.Config) string {
	h := sha256.Sum256([]byte(strings.Join(cfg.Setup, "\x00")))
	return filepath.Join(worktreeMarkerDir(root), "setup-"+scopeKey+"-"+hex.EncodeToString(h[:4]))
}

// worktreeMarkerDir is boxer's directory inside a worktree's git directory: <root>/.git/boxer for
// a main checkout, <common>/.git/worktrees/<name>/boxer for a linked one. When there is no git
// directory to find, it falls back to boxer's state, which is where these markers used to live.
func worktreeMarkerDir(root string) string {
	gd := filepath.Join(root, ".git")
	st, err := os.Stat(gd)
	switch {
	case err == nil && st.IsDir():
		return filepath.Join(gd, "boxer")
	case err == nil:
		// A linked worktree's .git is a file: "gitdir: <path>", relative to the worktree.
		if b, err := os.ReadFile(gd); err == nil {
			if dir, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:"); ok {
				dir = strings.TrimSpace(dir)
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(root, dir)
				}
				return filepath.Join(dir, "boxer")
			}
		}
	}
	return filepath.Join(stateRoot(), "worktree-markers", strings.ReplaceAll(strings.Trim(root, "/"), "/", "_"))
}

// setup runs the configured commands once per VM (R-GUEST-2). Failure deletes the VM.
// imageSetup runs the commands that change the guest image, once per VM, recorded by a marker
// inside the guest. It reports whether it ran them, so the caller can snapshot the result: a VM
// created from an environment pack carries the marker and runs nothing, which is the pack's point.
func (e *Env) imageSetup() (ran bool, err error) {
	if len(e.Cfg.ImageSetup) == 0 {
		return false, nil
	}
	// A transport failure is not a missing marker. Treating it as one re-ran every setup step on
	// a VM that had already run them, which for a repository whose setup installs dependencies is
	// minutes, not milliseconds.
	out, code, err := e.rootOutput("sh", "-c", "test -f "+imageSetupMarker)
	if err != nil || vm.TransportFailure(out) {
		return false, e.fail(&Error{Reason: "could not read the setup marker: " + firstNonEmpty(errText(err), strings.TrimSpace(out)), Cause: "TRANSPORT_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate"})
	}
	if code == 0 {
		return false, nil
	}
	for _, cmd := range e.Cfg.ImageSetup {
		fmt.Fprintf(e.Stderr, "boxer: image setup: %s\n", cmd)
		stepStart := time.Now()
		// Setup sees the same environment as every later command: a build that needs a registry
		// token or a proxy setting needs it while installing, not only when running.
		// As root, whatever the image's USER: image_setup installs system packages, and an image that
		// ends in `USER node` would otherwise fail on the first apt-get.
		code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.MountAt(), User: "0", Env: e.GuestEnv(),
			SecretEnv: e.HostSecrets(),
			Stdin:     strings.NewReader(""), Stdout: e.Stderr, Stderr: e.Stderr}, guestShell(cmd)...)
		if err != nil || code != 0 {
			_ = e.deleteVM()
			return false, e.fail(e.setupError("image setup", "`image_setup`", code, cmd, stepStart))
		}
	}
	_, code, err = e.rootOutput("sh", "-c", "mkdir -p /var/lib/boxer && touch "+imageSetupMarker)
	if err == nil && code != 0 {
		err = fmt.Errorf("writing the setup marker exited %d", code)
	}
	return err == nil, err
}

// rootOutput runs argv in the guest as root and returns its combined output.
func (e *Env) rootOutput(argv ...string) (string, int, error) {
	var buf bytes.Buffer
	code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, User: "0", Stdin: strings.NewReader(""), Stdout: &buf, Stderr: &buf}, argv...)
	return buf.String(), code, err
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// GuestWorkdir maps the host cwd into the mount.
func (e *Env) GuestWorkdir() string {
	rel, err := filepath.Rel(e.Scope.Root, e.CWD)
	if err != nil || strings.HasPrefix(rel, "..") {
		return e.MountAt()
	}
	return filepath.ToSlash(filepath.Join(e.MountAt(), rel))
}

// RunOpts controls Run.
type RunOpts struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	TTY     bool
	Timeout time.Duration
	// Task names the [tasks] entry this run came from, for the run record. Empty for a bare
	// command line.
	Task string
}

// Run executes argv in the guest, provisioning first when policy allows. It returns the exit
// code to propagate. Errors are *Error and already agent-readable.
func (e *Env) Run(argv []string, o RunOpts) (int, error) {
	if _, err := e.Ensure(config.Has(e.Cfg.CreateOn, "run"), false); err != nil {
		if e.Cfg.OnSandboxUnavailable == "passthrough" {
			fmt.Fprintf(o.Stderr, "boxer: sandbox unavailable, running on host (on_sandbox_unavailable = passthrough)\n")
			return -1, err
		}
		return 1, err
	}
	touchLastUsed(e.Scope.Key)
	start := time.Now()
	// The run record wants the tree's git state, and `git status --porcelain` costs 10-25ms on a
	// real repository. Asking for it after the command has finished adds that to every single
	// run, for a cache nothing blocks on — so it runs alongside the command instead. Taking the
	// state from the *start* of the run is also the more correct answer: a capsule replays what
	// the tree was when the command ran, not what the command left behind.
	git := make(chan gitInfo, 1)
	go func() {
		head, dirty := gitState(e.Scope.Root)
		git <- gitInfo{head, dirty}
	}()
	// The tails are tee'd, not buffered: io.MultiWriter passes each chunk straight through, so the
	// caller still sees output as the guest produces it.
	//
	// A caller that passes one writer for both streams must keep getting one: os/exec gives the
	// two streams a single pipe when they are the same writer, and hands them separate goroutines
	// when they are not. Wrapping each in its own MultiWriter would quietly turn a caller's
	// single buffer into a concurrent one.
	so, se := &tail{n: 8 << 10}, &tail{n: 8 << 10}
	stdout, stderr := io.Writer(io.MultiWriter(o.Stdout, so)), io.Writer(nil)
	if o.Stderr == o.Stdout {
		se = so
		stderr = stdout
	} else {
		stderr = io.MultiWriter(o.Stderr, se)
	}
	code, err := e.VM.Exec(vm.ExecOpts{
		Name: e.Scope.Key, Workdir: e.GuestWorkdir(), Env: e.GuestEnv(), SecretEnv: e.HostSecrets(),
		Timeout: o.Timeout, TTY: o.TTY, User: e.Cfg.User,
		Stdin: o.Stdin, Stdout: stdout, Stderr: stderr,
	}, argv...)
	outcome := obs.OK
	if err != nil || code != 0 {
		outcome = obs.Failed
		// A command that failed under an allowlist usually failed because of the allowlist, and
		// the guest cannot say so: it saw a DNS or connect error with no policy in it.
		if denied := DeniedHosts(e.VM, e.Scope.Key, e.Cfg, start); len(denied) > 0 {
			fmt.Fprintf(o.Stderr, "boxer: the egress allowlist refused %s during this command\n", strings.Join(denied, ", "))
			fmt.Fprintf(o.Stderr, "boxer: fix: add it to network.allow_hosts in boxer.toml, or set network.mode = \"on\"\n")
		}
	}
	e.event(obs.Run, outcome, time.Since(start), map[string]any{"exit": code, "command": strings.Join(argv, " ")})
	// Only record a command that actually ran. When Exec returns an error the backend refused —
	// the container was gone, the daemon was unreachable — and no command produced a status, so
	// writing one would put a number in the record that nothing ever exited with. A stale record
	// is recoverable; a fabricated one is what `capsule replay` would then fail to reproduce.
	if err == nil {
		e.recordRun(o, argv, code, time.Since(start), so, se, <-git)
	} else {
		<-git
	}
	return code, err
}

// recordRun writes the last-run record. The command line is stored verbatim whatever
// telemetry.record_commands says: that setting guards the event *sink*, which can be a file
// someone ships off the host, while this is a 0600 file beside the worktree it describes.
func (e *Env) recordRun(o RunOpts, argv []string, code int, took time.Duration, so, se *tail, g gitInfo) {
	image, _ := e.Image()
	head, dirty := g.head, g.dirty
	dir, err := filepath.Rel(e.Scope.Root, e.CWD)
	if err != nil {
		dir = "."
	}
	WriteRunRecord(RunRecord{
		Time: time.Now(), Scope: e.Scope.Key, Task: o.Task, Command: CommandLine(argv),
		Dir: filepath.ToSlash(dir), Exit: code, Duration: float64(took) / float64(time.Millisecond),
		Image: image, Pack: e.packLabel(), GitHead: head, Dirty: dirty, Config: e.Cfg.Files,
		Stdout: so.String(), Stderr: stderrTail(so, se),
	})
}

// stderrTail avoids repeating the whole combined stream twice when the caller gave one writer
// for both: the record then has one tail, under stdout, which is where a reader looks first.
func stderrTail(so, se *tail) string {
	if so == se {
		return ""
	}
	return se.String()
}

// packLabel is the pack the running sandbox was created from, as smolvm recorded it.
func (e *Env) packLabel() string {
	m, ok := e.seenMachine()
	if !ok {
		return ""
	}
	return m.Labels[vm.LabelPrefix+"pack"]
}

// lockScope takes an exclusive flock on a per-scope file under the state directory.
func lockScope(key string) (func(), error) {
	dir := filepath.Join(filepath.Dir(LastUsedDir()), "locks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return lockFile(filepath.Join(dir, key))
}

// lockFile takes an exclusive flock on path, creating it.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// LastUsedDir holds one zero-byte file per scope whose mtime is the last `run`. smolvm exposes
// no last-used time, so this is the only host state boxer keeps; losing it only delays gc.
func LastUsedDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "boxer", "last-used")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "boxer", "last-used")
}

func touchLastUsed(key string) {
	dir := LastUsedDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	p := filepath.Join(dir, key)
	now := time.Now()
	if err := os.Chtimes(p, now, now); err != nil {
		_ = os.WriteFile(p, nil, 0o644)
	}
}

// LastUsed returns when the scope last ran a command, or zero when unknown.
func LastUsed(key string) time.Time {
	st, err := os.Stat(filepath.Join(LastUsedDir(), key))
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

// Restart stops and starts the scope's VM: the recovery when the exec transport drops mid-command.
func (e *Env) Restart() error {
	if err := e.stopVM(); err != nil {
		return err
	}
	_, err := e.Ensure(false, false)
	return err
}

// Executable is the boxer binary UpDetached spawns; tests point it at a script.
var Executable = os.Executable

// UpDetached starts `boxer up` for this Env in its own session and returns without waiting, so a
// hook (harness SessionStart, git post-checkout) can warm the scope without blocking its caller.
// Ensure's per-scope lock makes the detached create and a concurrent first `run` produce one VM.
// ReclaimAllowed lets the boxer command enable the automatic sweep. It is off by default so that
// a program importing pkg/boxer never re-executes itself, and so tests do not spawn themselves.
var ReclaimAllowed = false

// ReclaimDetached starts a background `boxer gc` when one is due and the configuration allows it.
// It is how boxer's storage stays bounded without a daemon: an ordinary command starts the sweep,
// never waits for it, and never fails because of it. A sandbox costs about half a gigabyte, so
// the alternative is a host that fills up while every individual command looks harmless.
func (e *Env) ReclaimDetached() {
	// The sweep re-executes this binary. That is right for the boxer command and wrong for
	// anything embedding pkg/boxer, which would run its own program with an argument it never
	// declared, so the launcher opts in and BOXER_NO_RECLAIM opts out.
	if !e.Cfg.AutoReclaim || e.Inside() || !ReclaimAllowed || os.Getenv("BOXER_NO_RECLAIM") == "1" {
		return
	}
	every, err := time.ParseDuration(e.Cfg.ReclaimEvery)
	if err != nil || !ReclaimDue(every) {
		return
	}
	exe, err := Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "gc")
	cmd.Dir = e.CWD
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}

func (e *Env) UpDetached() error {
	exe, err := Executable()
	if err != nil {
		return err
	}
	args := []string{"up"}
	if e.Harness != "" {
		args = append(args, "--harness", e.Harness)
	}
	cmd := exec.Command(exe, append(args, e.IdentityArgs()...)...)
	cmd.Dir = e.CWD
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// IdentityArgs are the --session and --agent flags another boxer invocation needs to resolve
// this Env's scope; empty unless the isolation uses those ids.
func (e *Env) IdentityArgs() []string {
	var args []string
	if e.Cfg.Isolation == "session" || e.Cfg.Isolation == "subagent" {
		if e.ID.SessionID != "" {
			args = append(args, "--session", e.ID.SessionID)
		}
		if e.ID.AgentID != "" {
			args = append(args, "--agent", e.ID.AgentID)
		}
	}
	return args
}

// Down stops and deletes the scope's VM. Absence is not an error.
func (e *Env) Down() error {
	_, ok, err := e.Exists()
	if err != nil || !ok {
		return err
	}
	return e.deleteVM()
}

// Instructions is the text every harness shows the agent before its first action (R-PKG-2).
func (e *Env) Instructions() string {
	return Instructions(e.Cfg)
}

// Instructions renders the agent brief for a configuration.
func Instructions(cfg config.Config) string { return InstructionsFor(cfg, "") }

// InstructionsFor renders the brief naming the run tool the way the harness shows it; runTool ""
// means plain boxer_run. Grok, for one, exposes MCP tools only through a dispatcher, and a brief
// that names an invisible tool made every model try the shell first (eval-adherence.md).
func InstructionsFor(cfg config.Config, runTool string) string {
	if runTool == "" {
		runTool = "the boxer_run tool"
	}
	var b strings.Builder
	// What the sandbox is depends on the backend, and saying "microVM" of a container would tell
	// the agent a stronger boundary than it has.
	kind := "a microVM"
	if cfg.Backend == "docker" || cfg.Backend == "podman" {
		kind = "a container"
	}
	b.WriteString("This repository runs commands inside a boxer sandbox: " + kind + " per ")
	b.WriteString(cfg.Isolation)
	b.WriteString(" with the worktree mounted at ")
	b.WriteString(cfg.MountAt)
	b.WriteString(". Files you edit on the host are the same files the sandbox sees.\n")
	switch cfg.Mode {
	case "rewrite":
		b.WriteString("Shell commands that start with ")
		b.WriteString(strings.Join(cfg.Intercepted(), ", "))
		b.WriteString(" are transparently executed in the sandbox; write them normally. ")
	case "tool":
		b.WriteString("Do not run ")
		b.WriteString(strings.Join(cfg.Intercepted(), ", "))
		b.WriteString(" through the shell tool. Use ")
		b.WriteString(runTool)
		b.WriteString(", or the shell form `boxer run -c '<command>'`. ")
	case "off":
		b.WriteString("Sandboxing is currently off; commands run on the host. ")
	}
	b.WriteString(strings.Join(cfg.Passthrough, ", "))
	b.WriteString(" always run on the host.\n")
	// Named tasks are the deterministic path, so the brief has to name them: an agent that never
	// reads the skill still learns them here, and a task is the one command whose spelling the
	// repository guarantees.
	if names := cfg.TaskNames(); len(names) > 0 {
		b.WriteString("This repository declares tasks; prefer them over composing a command line:\n")
		// A described task is listed on its own line with what it is for, because choosing the
		// right task is the decision the agent actually has to make; an undescribed one falls back
		// to its command line, which is the only description it has.
		for _, n := range names {
			t := cfg.Tasks[n]
			what := t.Cmd
			if t.Description != "" {
				what = t.Description
			}
			b.WriteString("  `boxer run --task " + n + "` — " + what + "\n")
		}
	}
	b.WriteString(DevServerNote(cfg))
	b.WriteString("Any line beginning `boxer:` on stderr is an instruction, not a transient error: its `fix:` line is the exact command to run next.")
	return b.String()
}

// DevServerNote is the brief's sentence about reaching a server the sandbox runs, or "" when it
// forwards nothing. Without it an agent assumes localhost:<guest port>, which is right in no
// worktree once `auto:` gives each one its own host port, and right in only one with fixed ports.
func DevServerNote(cfg config.Config) string {
	if len(cfg.Network.Ports) == 0 {
		return ""
	}
	// Inside, the agent is in the guest: there is no `boxer` to ask, and the names are served by a
	// proxy on the host's loopback, which the guest cannot reach. What is true there is simpler.
	if cfg.Integration == "inside" {
		note := "A server you start here is at http://127.0.0.1:<its port> from inside the sandbox. "
		if cfg.URLs.Enabled {
			note += "A person on the host reaches it at a stable URL, listed in $BOXER_URLS (guest port=URL); give them that one. "
		} else {
			note += "A person on the host reaches it at the host port listed in $BOXER_PORTS (guest port=host port). "
		}
		return note + "\n"
	}
	if cfg.URLs.Enabled {
		return "Servers the sandbox runs are reachable at stable URLs, one set per worktree: `boxer url` prints this " +
			"worktree's (`boxer status --json` lists them all under `urls`). Use it, not localhost:<port>, in a browser, a test or a curl.\n"
	}
	return "Forwarded guest ports land on a different host port in each worktree: `boxer url` prints this worktree's " +
		"address (`boxer status --json` has them all under `ports`) — do not assume localhost:<guest port>.\n"
}

// resumeServices brings back the services `start` launched, after a snapshot stopped and started
// the machine. Stopping empties the guest's /tmp, where the start marker lives, so the services
// are gone and nothing else would notice. Without this the first `boxer shell` on a host — which
// caches the harness install — silently killed the dev server, and so did `boxer pack save`. The
// matrix saw it as inside cells losing their server on the first round only, which read for three
// runs as a flake that URLs caused. Failures are reported, never fatal: the snapshot itself worked.
func (e *Env) resumeServices(after string) {
	if err := e.startServices(); err != nil {
		fmt.Fprintf(e.Stderr, "boxer: restarting services after %s: %v\n", after, err)
		return
	}
	if err := e.waitReady(); err != nil {
		fmt.Fprintf(e.Stderr, "boxer: services did not come back after %s: %v\n", after, err)
	}
}

// minFree is the configured margin in bytes; 0 disables the check.
func (e *Env) minFree() int64 {
	if e.Cfg.MinFreeGB <= 0 {
		return 0
	}
	return int64(e.Cfg.MinFreeGB * 1024 * 1024 * 1024)
}

// HumanBytes is humanBytes for callers outside this package.
func HumanBytes(n int64) string { return humanBytes(n) }

// humanBytes prints a size the way a person reads one. Storage messages are read by someone who
// is already annoyed; "3.1 GB" is kinder than 3328599654.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}
