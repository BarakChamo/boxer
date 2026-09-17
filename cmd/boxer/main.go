// Command boxer puts agent shell commands in a smolvm microVM keyed to the git worktree.
package main

import (
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

  boxer up [--recreate]            create and start the sandbox for this scope
  boxer run -c '<shell>'           run a shell line in the sandbox
  boxer run -- <prog> [args]       run a program in the sandbox
  boxer down [--all]               delete this scope's sandbox (or every boxer sandbox)
  boxer ls                         list boxer sandboxes
  boxer gc [--dry-run]             delete sandboxes whose worktree is gone
  boxer doctor                     explain the resolved configuration and state
  boxer shim install [dir]         write PATH shims for the intercept list
  boxer hook <harness>             harness hook entry point (reads JSON on stdin)
  boxer mcp                        MCP server exposing boxer_run and boxer_status
  boxer package <harness>|all      render the plugin bundle for a harness
  boxer install <harness>|all      write project-level hooks/tool/instruction into this repo
                                   (the layer orchestrators like T3 Code and Paperclip also load)
  boxer install <harness> --user   write ~/.claude/settings.json or ~/.codex/config.toml hooks
                                   (the files Paperclip seeds its managed harness homes from)
  boxer shell <harness> [-e K=V] [-- args]   run the harness itself inside the sandbox (integration = inside)
                                   harnesses: claude, codex, gemini, kimi, opencode, pi, grok
                                   bundles/installs: claude-code, codex, gemini-cli, opencode, grok, pi, kimi, dsh
  boxer acp <harness> [-e K=V]               run the harness's ACP server inside the sandbox, stdio piped
  boxer shim install --harness a,b [dir]     PATH shims named after harness binaries → boxer shell
  boxer version

Identity flags accepted by up/run/down/doctor: --harness NAME --session ID --agent ID
Harnesses: claude-code codex gemini-cli grok kimi dsh opencode
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
		return lsCmd(stdout, stderr)
	case "gc":
		return gcCmd(rest, stdout, stderr)
	case "up", "run", "down", "doctor":
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
	all := fs.Bool("all", false, "every boxer sandbox (down)")
	shellLine := fs.String("c", "", "shell command line to run with sh -c (run)")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	e, err := box.Resolve("", *harness, *id)
	if cmd == "doctor" {
		return doctor(e, err, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch cmd {
	case "up":
		for _, w := range e.Warnings {
			fmt.Fprintln(stderr, "boxer: warning:", w)
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
			return downAll(e.VM, stdout, stderr)
		}
		_, existed, _ := e.Exists()
		if err := e.Down(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !existed {
			fmt.Fprintf(stdout, "boxer: %s had no sandbox\n", e.Scope.Key)
			return 0
		}
		fmt.Fprintf(stdout, "boxer: %s removed\n", e.Scope.Key)
		return 0
	case "run":
		argv := fs.Args()
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

func doctor(e *box.Env, resolveErr error, w io.Writer) int {
	fmt.Fprintln(w, "boxer", Version)
	if vm.Inside() {
		fmt.Fprintln(w, "inside:    this process is already in a boxer guest; hooks are silent and `boxer run` executes directly")
	}
	client := vm.New()
	if e != nil {
		client = e.VM
	}
	if v, err := client.Version(); err != nil {
		fmt.Fprintln(w, "smolvm:    MISSING —", err)
	} else {
		fmt.Fprintln(w, "smolvm:   ", v)
	}
	if p, err := exec.LookPath("boxer"); err != nil {
		fmt.Fprintln(w, "boxer:     NOT on PATH — hooks and shims call `boxer` by name")
	} else {
		fmt.Fprintln(w, "boxer:    ", p)
	}
	if e == nil {
		fmt.Fprintln(w, resolveErr)
		return 1
	}
	if e.Git.Toplevel != "" {
		kind := "main checkout"
		if e.Git.Linked {
			kind = "linked worktree"
		}
		fmt.Fprintf(w, "git:       %s (%s)\n", e.Git.Toplevel, kind)
	}
	fmt.Fprintf(w, "config:    %s\n", strings.Join(append([]string{"defaults"}, e.Cfg.Files...), " < "))
	for _, k := range []struct{ key, val string }{
		{"isolation", e.Cfg.Isolation}, {"mode", e.Cfg.Mode}, {"enforcement", e.Cfg.Enforcement},
		{"require_worktree", e.Cfg.RequireWorktree}, {"on_sandbox_unavailable", e.Cfg.OnSandboxUnavailable},
		{"create_on", strings.Join(e.Cfg.CreateOn, ",")}, {"destroy_on", strings.Join(e.Cfg.DestroyOn, ",")},
		{"intercept", strings.Join(e.Cfg.Intercept, ",")}, {"passthrough", strings.Join(e.Cfg.Passthrough, ",")},
		{"network.mode", e.Cfg.Network.Mode}, {"mount_at", e.Cfg.MountAt}, {"cpus", fmt.Sprint(e.Cfg.CPUs)}, {"memory", e.Cfg.Memory},
	} {
		src := e.Cfg.Sources[strings.SplitN(k.key, ".", 2)[0]]
		if src == "" {
			src = "default"
		}
		fmt.Fprintf(w, "  %-24s = %-40s (%s)\n", k.key, k.val, src)
	}
	if resolveErr != nil {
		fmt.Fprintln(w, resolveErr)
		return 1
	}
	fmt.Fprintf(w, "scope:     %s (%s) root %s\n", e.Scope.Key, e.Scope.Isolation, e.Scope.Root)
	img, why := e.Image()
	fmt.Fprintf(w, "image:     %s (%s)\n", img, why)
	if e.Cfg.Network.Mode == "off" {
		fmt.Fprintln(w, "warning:   network.mode = off — the image can only be used if smolvm already has it cached")
	}
	if m, ok, err := e.Exists(); err != nil {
		fmt.Fprintln(w, "sandbox:   error:", err)
	} else if !ok {
		fmt.Fprintln(w, "sandbox:   absent (boxer up, or the first sandboxed command, creates it)")
	} else {
		fmt.Fprintf(w, "sandbox:   %s, image %s\n", m.State, m.Image)
	}
	if e.Cfg.Enforcement == "shim" || e.Cfg.Enforcement == "both" {
		var shimmed, bare []string
		for _, p := range e.Cfg.Intercept {
			if p == "*" {
				continue
			}
			path, err := exec.LookPath(p)
			if err == nil && isShim(path) {
				shimmed = append(shimmed, p)
			} else if err == nil {
				bare = append(bare, p)
			}
		}
		fmt.Fprintf(w, "shims:     %d on PATH", len(shimmed))
		if len(bare) > 0 {
			fmt.Fprintf(w, "; NOT shimmed: %s (boxer shim install; prepend %s to PATH)", strings.Join(bare, ","), shim.DefaultDir())
		}
		fmt.Fprintln(w)
	}
	for _, wn := range e.Warnings {
		fmt.Fprintln(w, "warning:  ", wn)
	}
	return 0
}

func isShim(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "boxer shim")
}

func lsCmd(stdout, stderr io.Writer) int {
	ms, err := vm.New().Owned()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
	fmt.Fprintf(stdout, "%-16s %-9s %-10s %s\n", "SCOPE", "STATE", "ISOLATION", "WORKTREE")
	for _, m := range ms {
		fmt.Fprintf(stdout, "%-16s %-9s %-10s %s\n", m.Name, m.State, m.Labels["boxer.isolation"], m.Labels["boxer.root"])
	}
	return 0
}

func gcCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "print what would be deleted")
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
		if *dry {
			fmt.Fprintf(stdout, "would delete %s (%s)\n", m.Name, reason)
			continue
		}
		if err := client.Delete(m.Name); err != nil {
			fmt.Fprintln(stderr, err)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "deleted %s (%s)\n", m.Name, reason)
	}
	return code
}

func downAll(client vm.Client, stdout, stderr io.Writer) int {
	ms, err := client.Owned()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, m := range ms {
		if err := client.Delete(m.Name); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "boxer: %s removed\n", m.Name)
	}
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
		fmt.Fprintln(stderr, "usage: boxer shim install [--harness a,b] [dir]")
		return 2
	}
	fs := flag.NewFlagSet("shim", flag.ContinueOnError)
	harnesses := fs.String("harness", "", "comma-separated harness names: write shims named after their binaries that exec `boxer shell`")
	fs.SetOutput(stderr)
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	dir := shim.DefaultDir()
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
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
		fmt.Fprintln(stderr, "usage: boxer package <harness>|all [--out dir]")
		return 2
	}
	target := pos[0]
	cfg, err := config.Load(cwdRoot())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	names := []string{target}
	if target == "all" {
		names = bundle.Harnesses()
	}
	for _, h := range names {
		cfgH := cfg.ForHarness(h)
		dir := filepath.Join(*out, h)
		files, err := bundle.Render(h, cfgH, Version, dir)
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
