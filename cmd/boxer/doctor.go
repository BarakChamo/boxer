// `boxer doctor`: what boxer resolved, what it would do, and what is wrong with the setup.
//
// Everything a person needs when the answer is "it did not sandbox my command" lives here: the
// configuration as boxer read it, the scope it resolved, the hooks and shims it can see, the disk
// it is using, and the checks that catch a setup which looks right and does not hold — a login
// shell that demotes the shims being the one that costs the most to work out by hand.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/hook"
	"github.com/BarakChamo/boxer/internal/install"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/shim"
	"github.com/BarakChamo/boxer/internal/vm"
)

// doctorReport is everything `doctor` knows; the human form prints it in the order it was
// always printed, --json emits it whole.
type doctorReport struct {
	Version      string          `json:"version"`
	Inside       bool            `json:"inside"`
	Smolvm       string          `json:"smolvm"`
	SmolvmError  string          `json:"smolvm_error,omitempty"`
	Backend      *vm.Caps        `json:"backend,omitempty"`
	BoxerPath    string          `json:"boxer_path"`
	Git          *doctorGit      `json:"git,omitempty"`
	ConfigFiles  []string        `json:"config_files"`
	Settings     []doctorSetting `json:"settings"`
	Scope        *scopeJSON      `json:"scope,omitempty"`
	Image        string          `json:"image,omitempty"`
	ImageReason  string          `json:"image_reason,omitempty"`
	ImageWarning string          `json:"image_warning,omitempty"`
	Sandbox      *machineJSON    `json:"sandbox"`
	SandboxError string          `json:"sandbox_error,omitempty"`
	Shims        *doctorShims    `json:"shims,omitempty"`
	Drift        []string        `json:"drift,omitempty"`
	Signals      []hook.Signal   `json:"signals,omitempty"`
	Storage      *box.Footprint  `json:"storage,omitempty"`
	Denied       []string        `json:"denied_hosts,omitempty"`
	Environment  *doctorEnv      `json:"environment,omitempty"`
	Warnings     []string        `json:"warnings"`
	Error        string          `json:"error,omitempty"`
	resolved     bool            // Env exists (config could be loaded)
}

// scopeJSON is scope.Scope with the field names the JSON API promises.
type scopeJSON struct {
	Key string `json:"key"`
	// Name is the key said out loud: "swift-crab" for sb-7e1852e4a3c3. Derived from the key, so
	// nothing stores it and every boxer agrees.
	Name      string `json:"name"`
	Isolation string `json:"isolation"`
	Worktree  string `json:"worktree"`
	Degraded  bool   `json:"degraded"`
	Reason    string `json:"reason,omitempty"`
}

func scopeRow(s scope.Scope) *scopeJSON {
	return &scopeJSON{Key: s.Key, Name: s.Slug(), Isolation: s.Isolation, Worktree: s.Root, Degraded: s.Degraded, Reason: s.Reason}
}

// errorJSON is the agent-readable refusal (box.Error) as --json commands print it on stdout.
type errorJSON struct {
	Reason string     `json:"reason"`
	Cause  string     `json:"cause,omitempty"`
	Fix    string     `json:"fix,omitempty"`
	Scope  *scopeJSON `json:"scope,omitempty"`
}

func errorRow(err error) map[string]errorJSON {
	var be *box.Error
	if errors.As(err, &be) {
		return map[string]errorJSON{"error": {Reason: be.Reason, Cause: be.Cause, Fix: be.Fix, Scope: scopeRow(be.Scope)}}
	}
	return map[string]errorJSON{"error": {Reason: err.Error()}}
}

type doctorGit struct {
	Toplevel string `json:"toplevel"`
	Linked   bool   `json:"linked"`
}

type doctorSetting struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// doctorEnv answers "will this worktree have to run setup again": the environment's cache key,
// and whether a pack for it exists. Without it the only way to know is to watch a VM boot.
type doctorEnv struct {
	Key    string `json:"key"`
	Pack   string `json:"pack"`
	Cached bool   `json:"cached"`
}

type doctorShims struct {
	OnPath  []string `json:"on_path"`
	Missing []string `json:"missing"`
}

func (r *doctorReport) exit() int {
	if r.Error != "" {
		return 1
	}
	return 0
}

// loginShellDemotesShims reports whether a login shell would resolve an intercepted program on the
// host instead of through boxer's shim.
//
// A shim only works while its directory is first on PATH, and a login shell rebuilds PATH: macOS
// runs path_helper from /etc/zprofile, which puts the system directories in front of whatever was
// there. A harness that runs its commands through `zsh -lc` or `bash -lc` — Codex does — then gets
// the host's own `uname`, `npm` or `node`, and the sandbox is quietly not in the path at all. That
// is worth saying out loud: enforcement the user has configured is not holding, and nothing else
// reports it.
// launchedPATH is the PATH boxer was started with, before it removed its own shim directories from
// it. Diagnostics need the original: what the harness's PATH looks like is the thing being checked.
var launchedPATH string

func loginShellDemotesShims(e *box.Env) string {
	if e == nil || e.Cfg.Enforcement == "hook" || len(e.Cfg.Intercepted()) == 0 {
		return ""
	}
	dir := ""
	for _, p := range filepath.SplitList(launchedPATH) {
		if _, err := os.Stat(filepath.Join(p, shim.Marker)); err == nil {
			dir = p
			break
		}
	}
	if dir == "" {
		return "" // no shims on this PATH; nothing to demote
	}
	prog := e.Cfg.Intercepted()[0]
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-lc", "command -v "+prog)
	cmd.Env = append(os.Environ(), "PATH="+launchedPATH) // ask about the harness's PATH, not boxer's
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if resolved := strings.TrimSpace(string(out)); resolved != "" && !strings.HasPrefix(resolved, dir) {
		return fmt.Sprintf("PATH shims do not hold in a login shell: `%s -lc` resolves %s to %s, not to %s. "+
			"A login shell rebuilds PATH (on macOS, path_helper), so a harness that runs commands through a login "+
			"shell reaches the host. Use hook or tool enforcement for those harnesses.", shell, prog, resolved, dir)
	}
	return ""
}

func collectDoctor(e *box.Env, resolveErr error) *doctorReport {
	r := &doctorReport{Version: version(), Inside: vm.Inside(), ConfigFiles: []string{}, Settings: []doctorSetting{}, Warnings: []string{}}
	var client vm.Backend = vm.New()
	if e != nil {
		client = e.VM
	}
	if v, err := client.Version(); err != nil {
		r.SmolvmError = err.Error()
	} else {
		r.Smolvm = v
	}
	caps := vm.CapsOf(client)
	r.Backend = &caps
	r.BoxerPath, _ = exec.LookPath("boxer")
	if resolveErr != nil {
		r.Error = resolveErr.Error()
	}
	if e == nil {
		return r
	}
	r.resolved = true
	if e.Git.Toplevel != "" {
		r.Git = &doctorGit{Toplevel: e.Git.Toplevel, Linked: e.Git.Linked}
	}
	r.ConfigFiles = append(r.ConfigFiles, e.Cfg.Files...)
	for _, k := range []struct{ key, val string }{
		{"isolation", e.Cfg.Isolation}, {"mode", e.Cfg.Mode}, {"enforcement", e.Cfg.Enforcement},
		{"require_worktree", e.Cfg.RequireWorktree}, {"on_sandbox_unavailable", e.Cfg.OnSandboxUnavailable},
		{"create_on", strings.Join(e.Cfg.CreateOn, ",")}, {"destroy_on", strings.Join(e.Cfg.DestroyOn, ",")},
		{"warm_on_session_start", fmt.Sprint(e.Cfg.WarmOnSessionStart)}, {"worktree.manage", e.Cfg.Worktree.Manage},
		{"intercept", strings.Join(e.Cfg.Intercepted(), ",")}, {"passthrough", strings.Join(e.Cfg.Passthrough, ",")},
		{"network.mode", e.Cfg.Network.Mode}, {"mount_at", e.Cfg.MountAt}, {"cpus", fmt.Sprint(e.Cfg.CPUs)}, {"memory", e.Cfg.Memory},
	} {
		src := e.Cfg.Sources[strings.SplitN(k.key, ".", 2)[0]]
		if src == "" {
			src = "default"
		}
		r.Settings = append(r.Settings, doctorSetting{k.key, k.val, src})
	}
	if resolveErr != nil {
		return r
	}
	r.Scope = scopeRow(e.Scope)
	r.Image, r.ImageReason = e.Image()
	// The cache is of the image half: `setup` prepares the worktree and is never packed.
	if len(e.Cfg.ImageSetup) > 0 && r.Image != "" {
		key := box.EnvKey(r.Image, e.Cfg)
		pack := box.PackPath(key)
		_, err := os.Stat(pack)
		r.Environment = &doctorEnv{Key: filepath.Base(pack), Pack: pack, Cached: err == nil}
	}
	if e.Cfg.Network.Mode == "off" {
		r.ImageWarning = "network.mode = off — the image can only be used if smolvm already has it cached"
	}
	if m, ok, err := e.Exists(); err != nil {
		r.SandboxError = err.Error()
	} else if ok {
		row := machineRow(m)
		r.Sandbox = &row
		// A denied host is the most common reason a sandboxed build fails in a way the guest
		// cannot explain. Reading it must never fail doctor: a diagnostic that dies takes the
		// whole report with it.
		// No since: doctor is asked what is wrong here, not what went wrong in one command.
		r.Denied = box.DeniedHosts(client, e.Scope.Key, e.Cfg, time.Time{})
	}
	if e.Cfg.Enforcement == "shim" || e.Cfg.Enforcement == "both" {
		sh := &doctorShims{OnPath: []string{}, Missing: []string{}}
		for _, p := range e.Cfg.Intercepted() {
			if p == "*" {
				continue
			}
			path, err := exec.LookPath(p)
			if err == nil && isShim(path) {
				sh.OnPath = append(sh.OnPath, p)
			} else if err == nil {
				sh.Missing = append(sh.Missing, p)
			}
		}
		r.Shims = sh
	}
	r.Warnings = append(r.Warnings, e.Warnings...)
	if w := interceptSuggestion(e.Scope.Root, e.Cfg.Intercepted()); w != "" {
		r.Warnings = append(r.Warnings, w)
	}
	if w := loginShellDemotesShims(e); w != "" {
		r.Warnings = append(r.Warnings, w)
	}
	r.Signals = hook.Signals(e.Cfg.Isolation, install.GitInstalled(e.Scope.Root))
	// Installed content is published verbatim and never edited afterwards, so the drift check is
	// a comparison against this binary's own copy.
	r.Drift = install.Drift(e.Scope.Root, version())
	for _, p := range r.Drift {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%s differs from the copy in boxer %s (boxer install <harness> rewrites it)", p, version()))
	}
	// What boxer costs this host, because "why is my disk full" is the question a sandbox tool
	// has to be able to answer about itself.
	owned, _ := vm.Owned(client)
	st := box.Usage(len(owned))
	r.Storage = &st
	if min := int64(e.Cfg.MinFreeGB * 1024 * 1024 * 1024); min > 0 && st.FreeBytes > 0 && st.FreeBytes < min {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%s free, below min_free_gb (%.1f GB): boxer will pull images rather than cache them; `boxer gc --all` reclaims %s",
			box.HumanBytes(st.FreeBytes), e.Cfg.MinFreeGB, box.HumanBytes(st.PackBytes)))
	}
	r.Warnings = append(r.Warnings, e.Cfg.Warnings...)
	return r
}

func printDoctor(r *doctorReport, w io.Writer) int {
	fmt.Fprintln(w, "boxer", r.Version)
	if r.Inside {
		fmt.Fprintln(w, "inside:    this process is already in a boxer guest; hooks are silent and `boxer run` executes directly")
	}
	if r.SmolvmError != "" {
		fmt.Fprintln(w, "runtime:   MISSING —", r.SmolvmError)
	} else {
		fmt.Fprintln(w, "runtime:  ", r.Smolvm)
	}
	printCaps(w, r.Backend)
	if r.BoxerPath == "" {
		fmt.Fprintln(w, "boxer:     NOT on PATH — hooks and shims call `boxer` by name")
	} else {
		fmt.Fprintln(w, "boxer:    ", r.BoxerPath)
	}
	if !r.resolved {
		fmt.Fprintln(w, r.Error)
		return 1
	}
	if r.Git != nil {
		kind := "main checkout"
		if r.Git.Linked {
			kind = "linked worktree"
		}
		fmt.Fprintf(w, "git:       %s (%s)\n", r.Git.Toplevel, kind)
	}
	fmt.Fprintf(w, "config:    %s\n", strings.Join(append([]string{"defaults"}, r.ConfigFiles...), " < "))
	for _, k := range r.Settings {
		fmt.Fprintf(w, "  %-24s = %-40s (%s)\n", k.Key, k.Value, k.Source)
	}
	if r.Error != "" {
		fmt.Fprintln(w, r.Error)
		return 1
	}
	fmt.Fprintf(w, "scope:     %s (%s) — %s, root %s\n", r.Scope.Name, r.Scope.Key, r.Scope.Isolation, r.Scope.Worktree)
	fmt.Fprintf(w, "image:     %s (%s)\n", r.Image, r.ImageReason)
	if r.ImageWarning != "" {
		fmt.Fprintln(w, "warning:  ", r.ImageWarning)
	}
	switch {
	case r.SandboxError != "":
		fmt.Fprintln(w, "sandbox:   error:", r.SandboxError)
	case r.Sandbox == nil:
		fmt.Fprintln(w, "sandbox:   absent (boxer up, or the first sandboxed command, creates it)")
	default:
		fmt.Fprintf(w, "sandbox:   %s, image %s\n", r.Sandbox.State, r.Sandbox.Image)
	}
	if r.Shims != nil {
		fmt.Fprintf(w, "shims:     %d on PATH", len(r.Shims.OnPath))
		if len(r.Shims.Missing) > 0 {
			fmt.Fprintf(w, "; NOT shimmed: %s (boxer shim install; prepend %s to PATH)", strings.Join(r.Shims.Missing, ","), shim.DefaultDir())
		}
		fmt.Fprintln(w)
	}
	if len(r.Signals) > 0 {
		fmt.Fprintf(w, "signals:   %-12s %-8s %-8s %-6s %-9s %-8s %-5s %-9s %s\n", "harness", "session", "rewrite", "block", "subagent", "sess_end", "mcp", "git_hook", "isolation")
		for _, s := range r.Signals {
			fmt.Fprintf(w, "           %-12s %-8s %-8s %-6s %-9s %-8s %-5s %-9s %s\n", s.Harness, yn(s.SessionStart), yn(s.Rewrite), yn(s.BlockOnly), yn(s.SubagentStart), yn(s.SessionEnd), yn(s.MCP), yn(s.GitHook), s.EffectiveIsolation)
		}
		if !r.Signals[0].GitHook {
			fmt.Fprintln(w, "           git_hook: none; `boxer install git` warms new worktrees as git creates them")
		}
	}
	if env := r.Environment; env != nil {
		state := "not cached yet; the first sandbox here runs setup and caches the result"
		if env.Cached {
			state = "cached; a new worktree starts from it and skips setup"
		}
		fmt.Fprintf(w, "environment: %s — %s\n", env.Key, state)
	}
	if len(r.Denied) > 0 {
		fmt.Fprintf(w, "denied:    %s — refused by the egress allowlist\n", strings.Join(r.Denied, ", "))
		fmt.Fprintf(w, "           fix: add what the build needs to network.allow_hosts in boxer.toml\n")
	}
	if st := r.Storage; st != nil {
		fmt.Fprintf(w, "storage:   %d sandbox(es), %d pack(s) %s cached, %s free (boxer gc --all reclaims the packs)\n",
			st.Machines, st.PackCount, box.HumanBytes(st.PackBytes), box.HumanBytes(st.FreeBytes))
	}
	for _, wn := range r.Warnings {
		fmt.Fprintln(w, "warning:  ", wn)
	}
	return 0
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}

// printCaps says what this backend can do, at setup rather than three commands later.
//
// The boundary line is the one that earns its place. boxer's claim is that the agent cannot
// escape, and "one kernel per sandbox" and "one kernel shared by all of them" are materially
// different promises. Someone who picked a backend for its speed should be told which of the two
// they bought, not left to infer it from the backend's name.
func printCaps(w io.Writer, c *vm.Caps) {
	if c == nil {
		return
	}
	boundary := map[string]string{
		"kernel":    "a kernel per sandbox",
		"namespace": "one shared kernel",
	}[c.Boundary]
	if boundary == "" {
		boundary = c.Boundary
	}
	fmt.Fprintf(w, "backend:   %s — %s\n", c.Backend, boundary)
	for _, row := range []struct {
		name string
		on   bool
		off  string
	}{
		{"worktree mount", c.HostMounts, "the guest gets a copy, not the same files"},
		{"egress allowlist", c.Allowlist, "network.mode = \"allowlist\" is refused; use \"off\" or \"on\""},
		{"secrets by name", c.SecretEnv, "secret values reach the backend's argument vector"},
		{"environment cache", c.Packs, "image_setup runs again for every worktree"},
		{"fork", c.Branch, "needs a backend that can branch a running machine"},
		{"egress log", c.Egress, "denials are not recorded, so a short allowlist is silent"},
		{"disk usage", c.DiskUsage, "sizes report as unmeasured"},
	} {
		if row.on {
			fmt.Fprintf(w, "  %-18s yes\n", row.name)
		} else {
			fmt.Fprintf(w, "  %-18s no    — %s\n", row.name, row.off)
		}
	}
}

// languageTools are the programs a project in a language the default intercept list does not
// cover runs to build and test, keyed by the file that marks such a project. The default list
// cannot grow within 1.x, so doctor names what is missing instead.
var languageTools = []struct {
	file  string
	tools []string
}{
	{"Gemfile", []string{"ruby", "bundle", "rake", "rails", "rspec"}},
	{"pom.xml", []string{"mvn", "mvnw", "java"}},
	{"build.gradle", []string{"gradle", "gradlew", "java"}},
	{"build.gradle.kts", []string{"gradle", "gradlew", "java"}},
}

// interceptSuggestion says which of this project's own tools would run on the host, and the one
// line that sends them to the sandbox.
func interceptSuggestion(root string, intercepted []string) string {
	if slices.Contains(intercepted, "*") {
		return ""
	}
	var missing []string
	for _, l := range languageTools {
		if _, err := os.Stat(filepath.Join(root, l.file)); err != nil {
			continue
		}
		for _, t := range l.tools {
			if !slices.Contains(intercepted, t) && !slices.Contains(missing, t) {
				missing = append(missing, t)
			}
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("%s run on the host: they are not in intercept. To send them to the sandbox, add intercept_also = [%s] to boxer.toml",
		strings.Join(missing, ", "), quoteJoin(missing))
}

func quoteJoin(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = strconv.Quote(x)
	}
	return strings.Join(q, ", ")
}
