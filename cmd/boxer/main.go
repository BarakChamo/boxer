// Command boxer puts agent shell commands in a smolvm microVM keyed to the git worktree.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/bundle"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/hook"
	"github.com/BarakChamo/boxer/internal/inside"
	"github.com/BarakChamo/boxer/internal/install"
	"github.com/BarakChamo/boxer/internal/mcp"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/shim"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Version is set by the release build; "dev" otherwise.
var Version = "dev"

const usage = `boxer — run agent commands in a microVM per worktree

  boxer up [--recreate|--detach]   create and start the sandbox for this scope (--detach: in the background)
  boxer run -c '<shell>'           run a shell line in the sandbox
  boxer run -- <prog> [args]       run a program in the sandbox
  boxer down [--all|--scope NAME]  delete this scope's sandbox, every one, or one by name
  boxer status                     this scope's sandbox; exit 0 running, 3 stopped, 4 absent
  boxer ls                         list boxer sandboxes
  boxer gc [--dry-run]             delete sandboxes whose worktree is gone, idle sandboxes and packs
  boxer doctor                     explain the resolved configuration and state
  boxer brief [--json]             the agent brief for this checkout: mount, mode, intercept, tasks
  boxer tasks [--json]             the command lines this repository declares in [tasks]
  boxer run --task <name>          run one of them in the sandbox
  boxer shim install [dir]         write PATH shims for the intercept list
  boxer hook <harness>             harness hook entry point (reads JSON on stdin)
  boxer mcp                        MCP server exposing boxer_run and boxer_status
  boxer package plugin|<harness>|all  render the Agent Plugins package (dist/boxer), one client's view, or both
  boxer install <harness>|all      write project-level hooks/tool/instruction into this repo
                                   (the layer orchestrators like T3 Code and Paperclip also load)
  boxer install git                post-checkout hook: boxer up --detach in every new worktree (not in all)
  boxer install <harness> --user   write ~/.claude/settings.json or ~/.codex/config.toml hooks
                                   (the files Paperclip seeds its managed harness homes from)
  boxer shell <harness> [-e K=V] [-- args]   run the harness itself inside the sandbox (integration = inside)
                                   harnesses: claude, codex, gemini, kimi, opencode, pi, grok
                                   bundles/installs: claude-code, codex, gemini-cli, opencode, grok, pi, kimi, dsh
  boxer acp <harness> [-e K=V]               run the harness's ACP server inside the sandbox, stdio piped
  boxer shim install --harness a,b [dir]     PATH shims named after harness binaries → boxer shell
  boxer shim install --shell [dir]           boxer-bash: a shell that runs in the sandbox (OpenHands shell_path)
  boxer version

Identity flags accepted by up/run/down/status/doctor: --harness NAME --session ID --agent ID
--json on ls, status, down, gc, doctor prints one JSON object or array (see docs/api.md)
Harnesses: claude-code codex gemini-cli grok kimi dsh opencode pi
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "boxer", Version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "hook":
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "usage: boxer hook <harness>")
			return 2
		}
		return hook.Run(rest[0], stdin, stdout, stderr, box.Resolve)
	case "mcp":
		fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
		harness := fs.String("harness", "", "harness name for [harness.<name>] overrides")
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		s := &mcp.Server{Harness: *harness, Resolve: box.Resolve, Version: Version}
		if err := s.Serve(stdin, stdout); err != nil {
			fmt.Fprintln(stderr, "boxer mcp:", err)
			return 1
		}
		return 0
	case "shim":
		return shimCmd(rest, stdout, stderr)
	case "package":
		return packageCmd(rest, stdout, stderr)
	case "install":
		return installCmd(rest, stdout, stderr)
	case "ls":
		return lsCmd(rest, stdout, stderr)
	case "gc":
		return gcCmd(rest, stdout, stderr)
	case "up", "run", "down", "status", "doctor", "brief", "tasks":
		return scoped(cmd, rest, stdin, stdout, stderr)
	case "shell", "acp":
		return insideCmd(cmd, rest, stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "boxer: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

// identity parses the flags shared by scope-bound commands.
func identity(name string, args []string) (*flag.FlagSet, *string, *scope.Identity) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	harness := fs.String("harness", os.Getenv("BOXER_HARNESS"), "harness name")
	id := &scope.Identity{}
	fs.StringVar(&id.SessionID, "session", os.Getenv("BOXER_SESSION_ID"), "session id")
	fs.StringVar(&id.AgentID, "agent", os.Getenv("BOXER_AGENT_ID"), "agent id")
	return fs, harness, id
}

func scoped(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs, harness, id := identity(cmd, args)
	recreate := fs.Bool("recreate", false, "delete and recreate the sandbox (up)")
	detach := fs.Bool("detach", false, "start the sandbox in a background boxer and return at once (up)")
	all := fs.Bool("all", false, "every boxer sandbox (down)")
	scopeName := fs.String("scope", "", "sandbox name from `boxer ls` (down): act on it without resolving a worktree")
	shellLine := fs.String("c", "", "shell command line to run with sh -c (run)")
	task := fs.String("task", "", "name of a [tasks] entry in boxer.toml to run (run)")
	asJSON := fs.Bool("json", false, "print JSON (down, status, doctor)")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// A sandbox can be taken down by name, from anywhere: a dashboard built on `ls --json` has
	// machine names, not worktrees, and the worktree may be gone.
	if cmd == "down" && *scopeName != "" {
		if err := vm.New().Delete(*scopeName); err != nil {
			emit(stdout, errorRow(err), *asJSON)
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !emit(stdout, []downJSON{{Scope: *scopeName, Removed: true}}, *asJSON) {
			fmt.Fprintf(stdout, "boxer: %s removed\n", *scopeName)
		}
		return 0
	}
	e, err := box.Resolve("", *harness, *id)
	if cmd == "doctor" {
		r := collectDoctor(e, err)
		if emit(stdout, r, *asJSON) {
			return r.exit()
		}
		return printDoctor(r, stdout)
	}
	if err != nil {
		emit(stdout, errorRow(err), *asJSON)
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch cmd {
	case "brief":
		return briefCmd(e, *asJSON, stdout)
	case "tasks":
		emitTasks(e, *asJSON, stdout)
		return 0
	case "status":
		return statusCmd(e, *asJSON, stdout, stderr)
	case "up":
		for _, w := range e.Warnings {
			fmt.Fprintln(stderr, "boxer: warning:", w)
		}
		if *detach {
			if err := e.UpDetached(); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			fmt.Fprintf(stdout, "boxer: %s starting in the background\n", e.Scope.Key)
			return 0
		}
		created, err := e.Ensure(true, *recreate)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		img, why := e.Image()
		state := "running"
		if created {
			state = "created and running"
		}
		fmt.Fprintf(stdout, "boxer: %s %s (%s) image %s [%s] mounted %s -> %s\n", e.Scope.Key, state, e.Scope.Isolation, img, why, e.Scope.Root, e.MountAt())
		return 0
	case "down":
		if *all {
			return downAll(e.VM, *asJSON, stdout, stderr)
		}
		_, existed, _ := e.Exists()
		if err := e.Down(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if emit(stdout, downJSON{Scope: e.Scope.Key, Removed: existed}, *asJSON) {
			return 0
		}
		if !existed {
			fmt.Fprintf(stdout, "boxer: %s had no sandbox\n", e.Scope.Key)
			return 0
		}
		fmt.Fprintf(stdout, "boxer: %s removed\n", e.Scope.Key)
		return 0
	case "run":
		argv := fs.Args()
		if *task != "" {
			line, err := taskLine(e, *task)
			if err != nil {
				emit(stdout, errorRow(err), *asJSON)
				fmt.Fprintln(stderr, err)
				return 1
			}
			if len(argv) > 0 || *shellLine != "" {
				fmt.Fprintln(stderr, "boxer run: --task names the whole command; do not add -c or arguments")
				return 2
			}
			argv = []string{"sh", "-c", line}
		}
		if *shellLine != "" {
			if len(argv) > 0 {
				fmt.Fprintln(stderr, "boxer run: use either -c '<shell>' or -- <prog> [args], not both")
				return 2
			}
			argv = []string{"sh", "-c", *shellLine}
		}
		if len(argv) == 0 {
			fmt.Fprintln(stderr, "usage: boxer run -c '<shell>' | boxer run -- <prog> [args]")
			return 2
		}
		if vm.Inside() {
			return hostRun(argv, stdin, stdout, stderr) // already in the guest; run directly
		}
		tty := isTerminal(os.Stdin) && isTerminal(os.Stdout)
		code, err := e.Run(argv, box.RunOpts{Stdin: stdin, Stdout: stdout, Stderr: stderr, TTY: tty})
		if err != nil {
			if code == -1 { // passthrough policy: run on the host
				fmt.Fprintln(stderr, err)
				return hostRun(argv, stdin, stdout, stderr)
			}
			fmt.Fprintln(stderr, err)
			return code
		}
		return code
	}
	return 2
}

func hostRun(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 127
	}
	return 0
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// emit writes v as one indented JSON document when asJSON is set and reports whether it did;
// callers print the human form otherwise. Every --json command goes through here.
func emit(w io.Writer, v any, asJSON bool) bool {
	if !asJSON {
		return false
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(w, "null")
	}
	return true
}

// machineJSON is one boxer sandbox as `ls`, `gc`, `status` and `doctor` report it.
type machineJSON struct {
	Scope       string  `json:"scope"`
	State       string  `json:"state"`
	Isolation   string  `json:"isolation"`
	Worktree    string  `json:"worktree"`
	Integration string  `json:"integration"`
	Image       string  `json:"image"`
	CreatedAt   int64   `json:"created_at"`
	LastUsed    *string `json:"last_used"`
}

func machineRow(m vm.Machine) machineJSON {
	return machineJSON{
		Scope: m.Name, State: m.State, Image: m.Image, CreatedAt: m.CreatedAt,
		Isolation: m.Labels["boxer.isolation"], Worktree: m.Labels["boxer.root"], Integration: m.Labels["boxer.integration"],
		LastUsed: lastUsedJSON(m.Name),
	}
}

func lastUsedJSON(name string) *string {
	t := box.LastUsed(name)
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

type downJSON struct {
	Scope   string `json:"scope"`
	Removed bool   `json:"removed"`
}

type statusJSON struct {
	Scope       string            `json:"scope"`
	Isolation   string            `json:"isolation"`
	Worktree    string            `json:"worktree"`
	MountAt     string            `json:"mount_at"`
	Exists      bool              `json:"exists"`
	State       string            `json:"state"`
	Image       string            `json:"image"`
	ImageReason string            `json:"image_reason"`
	Labels      map[string]string `json:"labels"`
	LastUsed    *string           `json:"last_used"`
}

// Exit codes of `boxer status`.
const (
	exitStopped = 3
	exitAbsent  = 4
)

func statusCmd(e *box.Env, asJSON bool, stdout, stderr io.Writer) int {
	m, ok, err := e.Exists()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	img, why := e.Image()
	s := statusJSON{
		Scope: e.Scope.Key, Isolation: e.Scope.Isolation, Worktree: e.Scope.Root, MountAt: e.MountAt(),
		Exists: ok, State: "absent", Image: img, ImageReason: why, Labels: map[string]string{}, LastUsed: lastUsedJSON(e.Scope.Key),
	}
	code := exitAbsent
	if ok {
		s.State, s.Image, s.ImageReason, s.Labels = m.State, m.Image, "machine", m.Labels
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		code = exitStopped
		if m.Running() {
			code = 0
		}
	}
	if emit(stdout, s, asJSON) {
		return code
	}
	if !ok {
		fmt.Fprintf(stdout, "boxer: %s absent (image would be %s: %s)\n", s.Scope, img, why)
	} else {
		fmt.Fprintf(stdout, "boxer: %s %s, image %s, mounted %s -> %s\n", s.Scope, s.State, s.Image, s.Worktree, s.MountAt)
	}
	return code
}

// doctorReport is everything `doctor` knows; the human form prints it in the order it was
// always printed, --json emits it whole.
type doctorReport struct {
	Version       string          `json:"version"`
	Inside        bool            `json:"inside"`
	Smolvm        string          `json:"smolvm"`
	SmolvmError   string          `json:"smolvm_error,omitempty"`
	BoxerPath     string          `json:"boxer_path"`
	Git           *doctorGit      `json:"git,omitempty"`
	ConfigFiles   []string        `json:"config_files"`
	Settings      []doctorSetting `json:"settings"`
	Scope         *scopeJSON      `json:"scope,omitempty"`
	Image         string          `json:"image,omitempty"`
	ImageReason   string          `json:"image_reason,omitempty"`
	ImageWarning  string          `json:"image_warning,omitempty"`
	Sandbox       *machineJSON    `json:"sandbox"`
	SandboxError  string          `json:"sandbox_error,omitempty"`
	Shims         *doctorShims    `json:"shims,omitempty"`
	Drift         []string        `json:"drift,omitempty"`
	Signals       []hook.Signal   `json:"signals,omitempty"`
	Warnings      []string        `json:"warnings"`
	Error         string          `json:"error,omitempty"`
	resolved      bool            // Env exists (config could be loaded)
	configPrinted bool
}

// scopeJSON is scope.Scope with the field names the JSON API promises.
type scopeJSON struct {
	Key       string `json:"key"`
	Isolation string `json:"isolation"`
	Worktree  string `json:"worktree"`
	Degraded  bool   `json:"degraded"`
	Reason    string `json:"reason,omitempty"`
}

func scopeRow(s scope.Scope) *scopeJSON {
	return &scopeJSON{Key: s.Key, Isolation: s.Isolation, Worktree: s.Root, Degraded: s.Degraded, Reason: s.Reason}
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

func collectDoctor(e *box.Env, resolveErr error) *doctorReport {
	r := &doctorReport{Version: Version, Inside: vm.Inside(), ConfigFiles: []string{}, Settings: []doctorSetting{}, Warnings: []string{}}
	client := vm.New()
	if e != nil {
		client = e.VM
	}
	if v, err := client.Version(); err != nil {
		r.SmolvmError = err.Error()
	} else {
		r.Smolvm = v
	}
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
		{"intercept", strings.Join(e.Cfg.Intercept, ",")}, {"passthrough", strings.Join(e.Cfg.Passthrough, ",")},
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
	if e.Cfg.Network.Mode == "off" {
		r.ImageWarning = "network.mode = off — the image can only be used if smolvm already has it cached"
	}
	if m, ok, err := e.Exists(); err != nil {
		r.SandboxError = err.Error()
	} else if ok {
		row := machineRow(m)
		r.Sandbox = &row
	}
	if e.Cfg.Enforcement == "shim" || e.Cfg.Enforcement == "both" {
		sh := &doctorShims{OnPath: []string{}, Missing: []string{}}
		for _, p := range e.Cfg.Intercept {
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
	r.Signals = hook.Signals(e.Cfg.Isolation, install.GitInstalled(e.Scope.Root))
	// Installed content is published verbatim and never edited afterwards, so the drift check is
	// a comparison against this binary's own copy.
	r.Drift = install.Drift(e.Scope.Root, Version)
	for _, p := range r.Drift {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%s differs from the copy in boxer %s (boxer install <harness> rewrites it)", p, Version))
	}
	r.Warnings = append(r.Warnings, e.Cfg.Warnings...)
	return r
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func printDoctor(r *doctorReport, w io.Writer) int {
	fmt.Fprintln(w, "boxer", r.Version)
	if r.Inside {
		fmt.Fprintln(w, "inside:    this process is already in a boxer guest; hooks are silent and `boxer run` executes directly")
	}
	if r.SmolvmError != "" {
		fmt.Fprintln(w, "smolvm:    MISSING —", r.SmolvmError)
	} else {
		fmt.Fprintln(w, "smolvm:   ", r.Smolvm)
	}
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
	fmt.Fprintf(w, "scope:     %s (%s) root %s\n", r.Scope.Key, r.Scope.Isolation, r.Scope.Worktree)
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

func isShim(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "boxer shim")
}

func lsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ms, err := vm.New().Owned()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
	rows := make([]machineJSON, 0, len(ms))
	for _, m := range ms {
		rows = append(rows, machineRow(m))
	}
	if emit(stdout, rows, *asJSON) {
		return 0
	}
	fmt.Fprintf(stdout, "%-16s %-9s %-10s %s\n", "SCOPE", "STATE", "ISOLATION", "WORKTREE")
	for _, m := range rows {
		fmt.Fprintf(stdout, "%-16s %-9s %-10s %s\n", m.Scope, m.State, m.Isolation, m.Worktree)
	}
	return 0
}

// gcJSON is one `gc` decision: Deleted is false under --dry-run or when Error is set.
type gcJSON struct {
	machineJSON
	Pack    string `json:"pack,omitempty"` // set for pack rows; machine fields are then empty
	Reason  string `json:"reason"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

func gcCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "print what would be deleted")
	asJSON := fs.Bool("json", false, "print a JSON array of decisions")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := vm.New()
	ms, err := client.Owned()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// idle_timeout comes from the config visible here; gc is a host-wide sweep, so the user or
	// current repository layer decides. "never" disables the idle rule.
	wt, repo := cwdRoot()
	cfg, err := config.Load(wt, repo)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var idle time.Duration
	if cfg.IdleTimeout != "" && cfg.IdleTimeout != "never" {
		if idle, err = time.ParseDuration(cfg.IdleTimeout); err != nil {
			fmt.Fprintf(stderr, "idle_timeout = %q: %v\n", cfg.IdleTimeout, err)
			return 1
		}
	}
	code := 0
	rows := []gcJSON{}
	for _, m := range ms {
		root := m.Labels["boxer.root"]
		reason := ""
		if _, err := os.Stat(root); err != nil {
			reason = fmt.Sprintf("worktree %s is gone", root)
		} else if last := box.LastUsed(m.Name); idle > 0 && !last.IsZero() && time.Since(last) > idle {
			reason = fmt.Sprintf("idle since %s (idle_timeout %s)", last.Format(time.RFC3339), cfg.IdleTimeout)
		}
		if reason == "" {
			continue
		}
		row := gcJSON{machineJSON: machineRow(m), Reason: reason}
		if *dry {
			rows = append(rows, row)
			if !*asJSON {
				fmt.Fprintf(stdout, "would delete %s (%s)\n", m.Name, reason)
			}
			continue
		}
		if err := client.Delete(m.Name); err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			fmt.Fprintln(stderr, err)
			code = 1
			continue
		}
		row.Deleted = true
		rows = append(rows, row)
		if !*asJSON {
			fmt.Fprintf(stdout, "deleted %s (%s)\n", m.Name, reason)
		}
	}
	for _, p := range box.StalePacks(ms, idle) {
		row := gcJSON{Pack: p, Reason: fmt.Sprintf("pack unused for %s", cfg.IdleTimeout)}
		if *dry {
			rows = append(rows, row)
			if !*asJSON {
				fmt.Fprintf(stdout, "would delete pack %s (%s)\n", p, row.Reason)
			}
			continue
		}
		if err := os.Remove(p); err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			fmt.Fprintln(stderr, err)
			code = 1
			continue
		}
		os.Remove(strings.TrimSuffix(p, ".smolmachine") + ".lock")
		row.Deleted = true
		rows = append(rows, row)
		if !*asJSON {
			fmt.Fprintf(stdout, "deleted pack %s (%s)\n", p, row.Reason)
		}
	}
	emit(stdout, rows, *asJSON)
	return code
}

func downAll(client vm.Client, asJSON bool, stdout, stderr io.Writer) int {
	ms, err := client.Owned()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rows := []downJSON{}
	for _, m := range ms {
		if err := client.Delete(m.Name); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		rows = append(rows, downJSON{Scope: m.Name, Removed: true})
		if !asJSON {
			fmt.Fprintf(stdout, "boxer: %s removed\n", m.Name)
		}
	}
	emit(stdout, rows, asJSON)
	return 0
}

// envList collects repeatable -e KEY=VALUE flags.
type envList []string

func (e *envList) String() string     { return strings.Join(*e, ",") }
func (e *envList) Set(v string) error { *e = append(*e, v); return nil }

// insideCmd runs `boxer shell` and `boxer acp`: the harness inside the worktree's VM.
func insideCmd(kind string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(kind, flag.ContinueOnError)
	var extra envList
	fs.Var(&extra, "e", "extra KEY=VALUE for the guest process (repeatable)")
	fs.SetOutput(stderr)
	if len(args) == 0 {
		fmt.Fprintf(stderr, "usage: boxer %s <harness> [-e K=V]... [-- args]\nharnesses: %s\n", kind, strings.Join(inside.Names(), ", "))
		return 2
	}
	name := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	os.Setenv("BOXER_INTEGRATION", "inside") // this command is the inside placement by definition
	e, err := box.Resolve("", name, scope.Identity{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tty := kind == "shell" && isTerminal(os.Stdin) && isTerminal(os.Stdout)
	code, err := inside.Run(e, name, fs.Args(), kind == "acp", inside.Options{TTY: tty, Env: extra, Stdin: stdin, Stdout: stdout, Stderr: stderr})
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
	return code
}

func shimCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "install" {
		fmt.Fprintln(stderr, "usage: boxer shim install [--harness a,b | --shell] [dir]")
		return 2
	}
	fs := flag.NewFlagSet("shim", flag.ContinueOnError)
	harnesses := fs.String("harness", "", "comma-separated harness names: write shims named after their binaries that exec `boxer shell`")
	shell := fs.Bool("shell", false, "write boxer-bash: a bash whose every command runs in the sandbox, for harnesses with a configurable shell path")
	fs.SetOutput(stderr)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	dir := shim.DefaultDir()
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if *shell {
		path, err := shim.InstallShell(dir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "boxer: wrote %s\nPoint the harness's shell at it (OpenHands: TerminalTool shell_path).\n", path)
		return 0
	}
	if *harnesses != "" {
		var bins []string
		for _, h := range strings.Split(*harnesses, ",") {
			def, ok := inside.Harnesses[strings.TrimSpace(h)]
			if !ok {
				fmt.Fprintf(stderr, "unknown harness %q; known: %s\n", h, strings.Join(inside.Names(), ", "))
				return 2
			}
			bins = append(bins, def.Bin)
		}
		paths, err := shim.InstallHarness(dir, bins)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "boxer: wrote %d harness shims to %s\nPrepend it to PATH where the orchestrator starts:\n  export PATH=\"%s:$PATH\"\n", len(paths), dir, dir)
		return 0
	}
	cfg, err := config.Load(cwdRoot())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	paths, err := shim.Install(dir, cfg.Intercept)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "boxer: wrote %d shims to %s\nPrepend it to PATH:\n  export PATH=\"%s:$PATH\"\n", len(paths), dir, dir)
	return 0
}

func packageCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	out := fs.String("out", "dist", "output directory")
	// Accept flags before or after the positional: `boxer package all --out dist`.
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			if !strings.Contains(args[i], "=") && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, args[i])
	}
	if err := fs.Parse(flags); err != nil || len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: boxer package plugin|<harness>|all [--out dir]")
		return 2
	}
	target := pos[0]
	names := []string{target}
	if target == "all" {
		names = append([]string{bundle.Package}, bundle.Harnesses()...)
	}
	for _, h := range names {
		dir := filepath.Join(*out, "boxer") // the whole package, named after the plugin
		if h != bundle.Package {
			dir = filepath.Join(*out, h)
		}
		files, err := bundle.Render(h, Version, dir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: %d files in %s\n", h, len(files), dir)
	}
	return 0
}

func installCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	user := fs.Bool("user", false, "write the user-level layer (~/.claude/settings.json, ~/.codex/config.toml) instead of the project layer")
	fs.SetOutput(stderr)
	var flags, pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	if err := fs.Parse(flags); err != nil || len(pos) == 0 {
		fmt.Fprintln(stderr, "usage: boxer install <harness>|all [--user]   (project layer by default; --user for the files orchestrators seed from)")
		return 2
	}
	wt, repo := cwdRoot()
	if wt == "" && !*user {
		fmt.Fprintln(stderr, "boxer install: run inside the git repository to configure")
		return 1
	}
	cfg, err := config.Load(wt, repo)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if pos[0] == "git" {
		r, err := install.Git(repo)
		if err != nil {
			fmt.Fprintln(stderr, "boxer install git:", err)
			return 1
		}
		fmt.Fprintf(stdout, "git:\n  wrote %s\n", r.Written[0])
		for _, n := range r.Notes {
			fmt.Fprintf(stdout, "  note: %s\n", n)
		}
		return 0
	}
	names := pos
	if pos[0] == "all" {
		names = []string{"claude-code", "codex", "gemini-cli", "opencode", "grok", "kimi", "dsh", "pi"}
		if *user {
			names = []string{"claude-code", "codex"}
		}
	}
	code := 0
	for _, h := range names {
		var r install.Result
		if *user {
			r, err = install.User(h, cfg.ForHarness(h), Version)
		} else {
			r, err = install.Install(h, cfg.ForHarness(h), Version, wt)
		}
		if err != nil {
			fmt.Fprintln(stderr, "boxer install:", err)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "%s:\n", h)
		for _, w := range r.Written {
			shown := w
			if rel, err := filepath.Rel(wt, w); err == nil && !strings.HasPrefix(rel, "..") && wt != "" {
				shown = rel
			}
			fmt.Fprintf(stdout, "  wrote %s\n", shown)
		}
		for _, n := range r.Notes {
			fmt.Fprintf(stdout, "  note: %s\n", strings.ReplaceAll(n, "\n", "\n        "))
		}
	}
	return code
}

// cwdRoot returns (worktreeRoot, repoRoot) for config loading outside a resolved Env.
func cwdRoot() (string, string) {
	cwd, _ := os.Getwd()
	g, err := scope.Detect(cwd)
	if err != nil || g.Toplevel == "" {
		return "", ""
	}
	return g.Toplevel, filepath.Dir(g.CommonDir)
}

// briefJSON is `boxer brief --json`: the facts the brief states, so a harness or a dashboard can
// present them instead of parsing prose.
type briefJSON struct {
	Brief       string            `json:"brief"`
	Scope       *scopeJSON        `json:"scope"`
	Isolation   string            `json:"isolation"`
	MountAt     string            `json:"mount_at"`
	Mode        string            `json:"mode"`
	Enforcement string            `json:"enforcement"`
	Intercept   []string          `json:"intercept"`
	Passthrough []string          `json:"passthrough"`
	Tasks       map[string]string `json:"tasks"`
}

// briefCmd is the run-time half of static published content: the skill and the hooks say to run
// this, so no rendered file has to carry anyone's configuration.
func briefCmd(e *box.Env, asJSON bool, stdout io.Writer) int {
	if emit(stdout, briefJSON{
		Brief: e.Instructions(), Scope: scopeRow(e.Scope), Isolation: e.Cfg.Isolation, MountAt: e.MountAt(),
		Mode: e.Cfg.Mode, Enforcement: e.Cfg.Enforcement, Intercept: e.Cfg.Intercept,
		Passthrough: e.Cfg.Passthrough, Tasks: tasksOrEmpty(e.Cfg.Tasks),
	}, asJSON) {
		return 0
	}
	fmt.Fprintln(stdout, e.Instructions())
	if names := e.Cfg.TaskNames(); len(names) > 0 {
		fmt.Fprintf(stdout, "\nTasks (boxer run --task <name>): %s\n", strings.Join(names, ", "))
	}
	return 0
}

type taskJSON struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

func emitTasks(e *box.Env, asJSON bool, stdout io.Writer) {
	rows := []taskJSON{}
	for _, n := range e.Cfg.TaskNames() {
		rows = append(rows, taskJSON{n, e.Cfg.Tasks[n]})
	}
	if emit(stdout, rows, asJSON) {
		return
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "boxer: this repository declares no tasks; add a [tasks] table to boxer.toml")
		return
	}
	for _, t := range rows {
		fmt.Fprintf(stdout, "%-16s %s\n", t.Name, t.Command)
	}
}

// taskLine resolves a task name, refusing an unknown one in the shape an agent can act on.
func taskLine(e *box.Env, name string) (string, error) {
	if line, ok := e.Cfg.Tasks[name]; ok {
		return line, nil
	}
	known := "none declared; add a [tasks] table to boxer.toml"
	if names := e.Cfg.TaskNames(); len(names) > 0 {
		known = "boxer run --task " + strings.Join(names, " | ")
	}
	return "", &box.Error{
		Reason: fmt.Sprintf("no task named %q in boxer.toml", name),
		Cause:  "NO_SUCH_TASK", Scope: e.Scope, Fix: known,
	}
}

func tasksOrEmpty(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
