// Package box is boxer's core: resolve where we are, make sure the VM for that scope exists, run
// things in it, take it down. Every subcommand and every hook goes through here.
package box

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	return fmt.Sprintf("boxer: %s\n  scope:     %s (%s)\n  worktree:  %s\n  cause:     %s\n  fix:       %s",
		e.Reason, e.Scope.Key, e.Scope.Isolation, wt, e.Cause, fix)
}

// Env is one resolved invocation.
type Env struct {
	CWD     string
	Harness string
	Cfg     config.Config
	Git     scope.Git
	Scope   scope.Scope
	ID      scope.Identity // what the harness said about the caller, for hooks that re-invoke boxer
	VM      vm.Client
	Stderr  io.Writer
	// Warnings collected during resolution, printed by doctor and by run when relevant.
	Warnings []string
}

// Resolve builds an Env for cwd. harness may be empty; id fields may be empty.
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
		return nil, err
	}
	if harness != "" {
		cfg = cfg.ForHarness(harness)
	}
	// The first thing that knows the configuration turns the event stream on, so every later
	// emission in this process — hooks, MCP, run — uses the repository's own [telemetry] policy.
	obs.ConfigureFrom(cfg.Telemetry)
	e := &Env{CWD: cwd, Harness: harness, Cfg: cfg, Git: g, ID: id, VM: vm.New(), Stderr: os.Stderr}
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

var detectors = []struct{ file, image string }{
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
	return e.VM.Status(e.Scope.Key)
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
	m, ok, err := e.Exists()
	if err != nil {
		return false, err
	}
	if ok && recreate {
		if err := e.VM.Delete(e.Scope.Key); err != nil {
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
		created = true
	} else if m.Running() {
		return false, nil
	}
	if err := e.VM.Start(e.Scope.Key); err != nil {
		return created, e.fail(&Error{Reason: "sandbox failed to start: " + err.Error(), Cause: "START_FAILED", Scope: e.Scope, Fix: "boxer up --recreate"})
	}
	ranSetup, err := e.setup()
	if err != nil {
		return created, err
	}
	if ranSetup {
		e.PackEnv()
	}
	// Services come after the snapshot: a pack should carry what is installed, not a process that
	// was running when it was taken.
	if err := e.startServices(); err != nil {
		return created, err
	}
	return created, e.waitReady()
}

func (e *Env) create() error {
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
	hosts := append([]string{}, e.Cfg.Network.AllowHosts...)
	if e.Cfg.Network.Mode == "allowlist" && image != "" {
		hosts = append(hosts, registryHosts(image)...)
	}
	labels := map[string]string{
		vm.LabelPrefix + "scope":       e.Scope.Key,
		vm.LabelPrefix + "isolation":   e.Scope.Isolation,
		vm.LabelPrefix + "root":        e.Scope.Root,
		vm.LabelPrefix + "integration": e.Cfg.Integration,
	}
	volumes := []string{e.Scope.Root + ":" + e.MountAt()}
	for _, m := range e.Cfg.Mounts {
		volumes = append(volumes, expandMount(m))
	}
	if e.Inside() {
		if InsideHooks.Mounts != nil {
			volumes = append(volumes, InsideHooks.Mounts()...)
		}
		if InsideHooks.AllowHosts != nil && e.Cfg.Network.Mode == "allowlist" {
			hosts = append(hosts, InsideHooks.AllowHosts()...)
		}
	}
	from := ""
	if e.Cfg.Smolfile == "" && image != "" {
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
		AllowHosts: hosts,
		Ports:      e.Cfg.Network.Ports,
	}
	if err = e.VM.Create(spec); err != nil && from != "" && !vm.IsAlreadyExists(err) {
		// A pack can be truncated: an interrupted `pack create` leaves a file smaller than its
		// own footer, and every later create from it fails with the same unhelpful I/O error
		// until someone deletes it by hand. Delete it and pull the image instead.
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
		return e.fail(&Error{Reason: "sandbox could not be created: " + err.Error(), Cause: "CREATE_FAILED", Scope: e.Scope,
			Fix: "boxer doctor"})
	}
	return nil
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
	for _, c := range cfg.Setup {
		key += "\x00setup:" + c
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
	if len(e.Cfg.Setup) == 0 {
		return ""
	}
	side := PackPath(EnvKey(image, e.Cfg))
	if !packReady(side) {
		return ""
	}
	return side
}

// PackEnv snapshots the scope's VM, setup already run, into the pack envPack looks for. Without
// it every new worktree repeats `bun install` from scratch while a pack of the bare image sits
// beside it, which is the most expensive thing about boxer before 1.1.
//
// smolvm packs only a stopped VM, so the VM is stopped and restarted around it: a few seconds,
// once per host per environment. Failure is reported and never blocks the run.
func (e *Env) PackEnv() {
	image, _ := e.Image()
	if image == "" || e.Cfg.Smolfile != "" || len(e.Cfg.Setup) == 0 || e.Inside() {
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
	err = e.VM.Stop(e.Scope.Key)
	if err == nil {
		_, err = e.VM.PackFromVM(e.Scope.Key, stub)
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
	err = e.VM.Stop(e.Scope.Key)
	if err == nil {
		_, err = e.VM.PackFromVM(e.Scope.Key, stub)
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
}

// StalePacks lists packs no machine references (by its boxer.pack label) whose last use is
// older than idle; idle <= 0 disables pruning, matching idle_timeout = "never".
func StalePacks(ms []vm.Machine, idle time.Duration) []string {
	if idle <= 0 {
		return nil
	}
	used := map[string]bool{}
	for _, m := range ms {
		used[m.Labels[vm.LabelPrefix+"pack"]] = true
	}
	packs, _ := filepath.Glob(filepath.Join(PackDir(), "*.smolmachine"))
	var out []string
	for _, p := range packs {
		if st, err := os.Stat(p); err == nil && !used[p] && time.Since(st.ModTime()) > idle {
			out = append(out, p)
		}
	}
	return out
}

// packed returns the cached .smolmachine for image, packing it on first use under a per-image
// lock. smolvm pulls a registry image again for every machine, and that pull is the slow, flaky
// step (rate limits, stalled blobs); packing moves it to once per image per host. Any failure
// falls back to a direct pull so a pack problem never blocks a sandbox. `boxer gc` prunes packs.
func (e *Env) packed(image string) string {
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
	if _, err := e.VM.Pack(image, stub); err != nil {
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
	out := make([]string, 0, len(e.Cfg.Env))
	for _, k := range sortedKeys(e.Cfg.Env) {
		out = append(out, k+"="+e.Cfg.Env[k])
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

// startServices runs the `start` list detached, once per running VM. This is how a database or a
// dev server runs: `setup` installs it, `start` launches it, `ready` waits for it. A marker in
// memory-backed /tmp is exactly right here, because it must not survive a restart: a restarted VM
// has no processes and has to start them again.
const startMarker = "/tmp/boxer-started"

func (e *Env) startServices() error {
	if len(e.Cfg.Start) == 0 {
		return nil
	}
	out, code, err := e.VM.Output(e.Scope.Key, "", "sh", "-c", "test -f "+startMarker)
	if err != nil || vm.TransportFailure(out) {
		return e.fail(&Error{Reason: "could not read the start marker: " + firstNonEmpty(errText(err), strings.TrimSpace(out)), Cause: "TRANSPORT_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate"})
	}
	if code == 0 {
		return nil
	}
	for _, cmd := range e.Cfg.Start {
		fmt.Fprintf(e.Stderr, "boxer: start: %s\n", cmd)
		// Detached and disowned: the command that launches a server must not wait for it.
		line := "cd " + e.MountAt() + " && nohup sh -lc " + shellQuote(cmd) + " >/tmp/boxer-start.log 2>&1 &"
		if _, code, err := e.VM.Output(e.Scope.Key, e.MountAt(), "sh", "-c", line); err != nil || code != 0 {
			return e.fail(&Error{Reason: fmt.Sprintf("start step failed (exit %d): %s", code, cmd), Cause: "START_FAILED", Scope: e.Scope,
				Fix: "check the `start` list in boxer.toml, then: boxer up"})
		}
	}
	_, _, _ = e.VM.Output(e.Scope.Key, "", "sh", "-c", "touch "+startMarker)
	return nil
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
	for {
		if _, code, err := e.VM.Output(e.Scope.Key, e.MountAt(), "sh", "-lc", e.Cfg.Ready); err == nil && code == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return e.fail(&Error{Reason: fmt.Sprintf("the sandbox never became ready: %q did not succeed within %s", e.Cfg.Ready, timeout),
				Cause: "NOT_READY", Scope: e.Scope, Fix: "check the `start` list and `ready` command, and /tmp/boxer-start.log in the guest"})
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// shellQuote wraps s for `sh -c`, because a start command is written by a person and will contain
// quotes sooner or later.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

const setupMarker = "/var/lib/boxer/setup-done"

// setup runs the configured commands once per VM (R-GUEST-2). Failure deletes the VM.
// setup runs the configured commands once per VM, recorded by a marker file inside the guest. It
// reports whether it ran them, so the caller can snapshot the result: a VM created from an
// environment pack carries the marker and runs nothing, which is the whole point of the pack.
func (e *Env) setup() (ran bool, err error) {
	if len(e.Cfg.Setup) == 0 {
		return false, nil
	}
	// A transport failure is not a missing marker. Treating it as one re-ran every setup step on
	// a VM that had already run them, which for a repository whose setup installs dependencies is
	// minutes, not milliseconds.
	out, code, err := e.VM.Output(e.Scope.Key, "", "sh", "-c", "test -f "+setupMarker)
	if err != nil || vm.TransportFailure(out) {
		return false, e.fail(&Error{Reason: "could not read the setup marker: " + firstNonEmpty(errText(err), strings.TrimSpace(out)), Cause: "TRANSPORT_FAILED", Scope: e.Scope,
			Fix: "boxer up --recreate"})
	}
	if code == 0 {
		return false, nil
	}
	for _, cmd := range e.Cfg.Setup {
		fmt.Fprintf(e.Stderr, "boxer: setup: %s\n", cmd)
		// Setup sees the same environment as every later command: a build that needs a registry
		// token or a proxy setting needs it while installing, not only when running.
		code, err := e.VM.Exec(vm.ExecOpts{Name: e.Scope.Key, Workdir: e.MountAt(), Env: e.GuestEnv(),
			Stdin: strings.NewReader(""), Stdout: e.Stderr, Stderr: e.Stderr}, "sh", "-lc", cmd)
		if err != nil || code != 0 {
			_ = e.VM.Delete(e.Scope.Key)
			return false, e.fail(&Error{Reason: fmt.Sprintf("setup step failed (exit %d): %s", code, cmd), Cause: "SETUP_FAILED", Scope: e.Scope,
				Fix: "fix the `setup` list in boxer.toml, then: boxer up"})
		}
	}
	_, code, err = e.VM.Output(e.Scope.Key, "", "sh", "-c", "mkdir -p /var/lib/boxer && touch "+setupMarker)
	if err == nil && code != 0 {
		err = fmt.Errorf("writing the setup marker exited %d", code)
	}
	return err == nil, err
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
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	TTY    bool
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
	env := e.GuestEnv()
	for _, k := range e.Cfg.EnvPassthrough {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	start := time.Now()
	code, err := e.VM.Exec(vm.ExecOpts{
		Name: e.Scope.Key, Workdir: e.GuestWorkdir(), Env: env, TTY: o.TTY,
		Stdin: o.Stdin, Stdout: o.Stdout, Stderr: o.Stderr,
	}, argv...)
	outcome := obs.OK
	if err != nil || code != 0 {
		outcome = obs.Failed
	}
	e.event(obs.Run, outcome, time.Since(start), map[string]any{"exit": code, "command": strings.Join(argv, " ")})
	return code, err
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
	if err := e.VM.Stop(e.Scope.Key); err != nil {
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
	return e.VM.Delete(e.Scope.Key)
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
	b.WriteString("This repository runs commands inside a boxer sandbox: a microVM per ")
	b.WriteString(cfg.Isolation)
	b.WriteString(" with the worktree mounted at ")
	b.WriteString(cfg.MountAt)
	b.WriteString(". Files you edit on the host are the same files the sandbox sees.\n")
	switch cfg.Mode {
	case "rewrite":
		b.WriteString("Shell commands that start with ")
		b.WriteString(strings.Join(cfg.Intercept, ", "))
		b.WriteString(" are transparently executed in the sandbox; write them normally. ")
	case "tool":
		b.WriteString("Do not run ")
		b.WriteString(strings.Join(cfg.Intercept, ", "))
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
		b.WriteString("This repository declares tasks; prefer them over composing a command line: ")
		for i, n := range names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("`boxer run --task " + n + "`")
		}
		b.WriteString(".\n")
	}
	b.WriteString("Any line beginning `boxer:` on stderr is an instruction, not a transient error: its `fix:` line is the exact command to run next.")
	return b.String()
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
