// Command boxer puts agent shell commands in a smolvm microVM keyed to the git worktree.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/bundle"
	"github.com/BarakChamo/boxer/internal/cli"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/hook"
	"github.com/BarakChamo/boxer/internal/inside"
	"github.com/BarakChamo/boxer/internal/install"
	"github.com/BarakChamo/boxer/internal/junit"
	"github.com/BarakChamo/boxer/internal/mcp"
	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/shim"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Version is stamped by the release build with -ldflags -X. It MUST stay a plain string constant
// initializer: the linker refuses to rewrite a variable initialized by anything else, silently,
// so computing a fallback here would unstamp every release. version() resolves the fallback at
// the point of use instead.
var Version = "dev"

// version is the version to report: the release stamp when there is one, otherwise the module
// version the toolchain recorded for a `go install ...@v1.2.3`, and only then "dev".
func version() string { return versionOrBuildInfo(Version) }

func versionOrBuildInfo(stamped string) string {
	if stamped != "dev" && stamped != "" {
		return stamped
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const usage = `boxer — run agent commands in a sandbox per worktree

See what is here
  boxer ls [-A] [--pr] [--resources]   sandboxes, their backend, branch, git state and URLs (-A: every backend)
  boxer backends [NAME] [--probe]      which backends are installed and answering; --probe runs a real sandbox
  boxer integrations [add NAME]        harnesses, orchestrators and tools, and whether boxer is wired into each
                                       add: <harness>, skill (npx skills), plugin (npx plugins), git, conductor
  boxer url [PORT] | boxer open [PORT] where this worktree's server is, by name when [urls] gives it one

Manage sandboxes
  boxer stop|rm [NAME...] [--all|--gone|--stopped] [-i] [-y]   stop or remove; -i to choose

Run and configure
  boxer up [--recreate|--detach]   create and start the sandbox for this scope (--detach: in the background)
  boxer run -c '<shell>'           run a shell line in the sandbox
  boxer run -- <prog> [args]       run a program in the sandbox
  boxer down [--all|--scope NAME]  delete this scope's sandbox, every one, or one by name
  boxer status                     this scope's sandbox; exit 0 running, 3 stopped, 4 absent
  boxer gc [--all] [--dry-run]     delete sandboxes whose worktree is gone, idle sandboxes and packs
  boxer doctor                     explain the resolved configuration and state
  boxer brief [--json]             the agent brief for this checkout: mount, mode, intercept, tasks
  boxer tasks [--json]             the command lines this repository declares in [tasks]
  boxer run --task <name>          run one of them in the sandbox
  boxer run --junit a.xml[,b.xml]  summarise the JUnit reports the command wrote
  boxer fork [--prepare] [--count N]  branch this sandbox into copy-on-write children that
                                   share this worktree, warm, without booting
  boxer fork ls|rm <name>|--all    list them, reclaim them
  boxer pack save|ls|use|rm <name>  keep a prepared sandbox as a named artifact and start from it
  boxer capsule new [-o PATH]      write a replayable manifest of the last run in this scope
  boxer capsule inspect|replay <path>  read one, or run it again and say whether it reproduced
  boxer logs [--scope NAME] [-n N] [--json]  read the event log ([telemetry] sink = "file")
  boxer watch [--json]             stream sandbox state changes as they happen
  boxer shim install [dir]         write PATH shims for the intercept list
  boxer hook <harness>             harness hook entry point (reads JSON on stdin)
  boxer mcp                        MCP server exposing boxer_run and boxer_status
  boxer package plugin|skills|<harness>|all  render the Agent Plugins package (dist/boxer), the
                                   published skill and its .well-known index, one client's view, or all
  boxer install <harness>|all      write project-level hooks/tool/instruction into this repo
                                   (the layer orchestrators like T3 Code and Paperclip also load)
  boxer install git                post-checkout hook: boxer up --detach in every new worktree (not in all)
  boxer install conductor          .conductor/settings.toml: harness shims + a setup script that warms the sandbox
  boxer install copilot --user     ~/.copilot hooks, skill and MCP entry (Copilot has no project layer)
  boxer install <harness> --user   write ~/.claude/settings.json or ~/.codex/config.toml hooks
                                   (the files Paperclip seeds its managed harness homes from)
  boxer shell <harness> [-e K=V] [-- args]   run the harness itself inside the sandbox (integration = inside)
                                   harnesses: claude, codex, copilot, fx, gemini, grok, kimi, opencode, pi
                                   bundles/installs: claude-code, codex, gemini-cli, opencode, grok, pi, kimi, dsh
  boxer acp <harness> [-e K=V]               run the harness's ACP server inside the sandbox, stdio piped
  boxer shim install --harness a,b [dir]     PATH shims named after harness binaries → boxer shell
  boxer shim install --shell [dir]           boxer-bash: a shell that runs in the sandbox (OpenHands shell_path)
  boxer version

Identity flags accepted by up/run/down/status/doctor: --harness NAME --session ID --agent ID
  boxer completion bash|zsh|fish      shell completion

--json prints one JSON document on ls, status, down, gc, doctor, brief, tasks, logs, backends,
integrations, stop, rm, pack ls, fork and capsule inspect; watch --json prints one per line.
Output adapts to who is reading: colour and prompts at a terminal, plain text for an agent or a pipe.
BOXER_OUTPUT=json|text|human overrides it; NO_COLOR turns colour off; BOXER_AGENT=1 marks an agent.
Install targets: claude-code codex copilot gemini-cli grok kimi dsh opencode pi
`

func main() {
	// Never resolve a program through boxer's own shims: a shim runs `boxer run`, so a boxer that
	// resolved one would call into itself, and the inner call waits for the sandbox the outer one
	// is already holding. This has to happen before anything is spawned, which means here.
	launchedPATH = os.Getenv("PATH")
	if p := shim.SanitizePath(launchedPATH); p != launchedPATH {
		_ = os.Setenv("PATH", p)
	}
	// Only the command sweeps: the library never re-executes its host program.
	box.ReclaimAllowed = true
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	// BOXER_TRACE turns the file sink on before any configuration is resolved; box.Resolve
	// reconfigures from the repository's [telemetry] table as soon as it has one.
	obs.Configure(obs.Config{})
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "boxer", version())
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
		s := &mcp.Server{Harness: *harness, Resolve: box.Resolve, Version: version()}
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
	case "stop", "rm":
		return stopRmCmd(args[0], args[1:], stdin, stdout, stderr)
	case "backends":
		return backendsCmd(args[1:], stdout, stderr)
	case "integrations", "harnesses":
		return integrationsCmd(args[1:], stdin, stdout, stderr)
	case "completion":
		return completionCmd(args[1:], stdout, stderr)
	case "url", "open":
		e, err := box.Resolve("", "", scope.Identity{})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return urlCmd(args[0] == "open", e, args[1:], stdout, stderr)
	case "ls":
		return lsCmd(rest, stdout, stderr)
	case "gc":
		return gcCmd(rest, stdout, stderr)
	case "fork":
		return forkCmd(rest, stdout, stderr)
	case "pack":
		return packCmd(rest, stdout, stderr)
	case "capsule":
		return capsuleCmd(rest, stdin, stdout, stderr)
	case "logs":
		return logsCmd(rest, stdout, stderr)
	case "watch":
		return watchCmd(rest, stdout, stderr)
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
	rebuild := fs.Bool("rebuild", false, "recreate the sandbox and rebuild its environment, ignoring the cached pack (up)")
	detach := fs.Bool("detach", false, "start the sandbox in a background boxer and return at once (up)")
	all := fs.Bool("all", false, "every boxer sandbox (down)")
	scopeName := fs.String("scope", "", "sandbox name from `boxer ls` (down): act on it without resolving a worktree")
	shellLine := fs.String("c", "", "shell command line to run with sh -c (run)")
	// A harness that replaces its terminal's shell with boxer-bash drives it over pipes, not a
	// terminal, so the automatic detection below says "no TTY" and bash starts non-interactive: it
	// prints no prompt, and a harness that delimits command output by the prompt (OpenHands does)
	// sees nothing and waits forever. Such a caller asks for a terminal explicitly.
	forceTTY := fs.Bool("tty", false, "allocate a terminal in the guest even when stdin is not one (run)")
	taskName := fs.String("task", "", "name of a [tasks] entry in boxer.toml to run (run)")
	junitPaths := fs.String("junit", "", "comma-separated JUnit XML paths, relative to the worktree, to summarise after the command (run)")
	failOnTests := fs.Bool("fail-on-test-failures", false, "exit 1 when the command passed but a test in the JUnit report did not (run)")
	asJSON := fs.Bool("json", false, "print JSON (down, status, doctor)")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// A sandbox can be taken down by name, from anywhere: a dashboard built on `ls --json` has
	// machine names, not worktrees, and the worktree may be gone.
	if cmd == "down" && *scopeName != "" {
		hostVM := vm.Host(hostBackend())
		*scopeName = box.ResolveName(hostVM, *scopeName)
		if err := hostVM.Delete(*scopeName); err != nil {
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
	// A fork child has no worktree of its own to resolve from — it is a branch of this one — so
	// `run` and `status` take it by name, the way `down` already did.
	if *scopeName != "" && (cmd == "run" || cmd == "status") {
		if err != nil {
			emit(stdout, errorRow(err), *asJSON)
			fmt.Fprintln(stderr, err)
			return 1
		}
		if e, err = e.At(*scopeName); err != nil {
			emit(stdout, errorRow(err), *asJSON)
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
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
			fmt.Fprintf(stdout, "boxer: %s starting in the background\n", e.Scope.Names())
			return 0
		}
		if *rebuild {
			// The pack is a cache of `setup`, and a cache you cannot drop is a liability: a build
			// that depends on something outside the setup list (a base image tag that moved, a
			// dependency resolved at install time) needs a way to start again.
			if n, err := e.DropEnvPack(); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			} else if n != "" {
				fmt.Fprintf(stdout, "boxer: dropped the cached environment %s\n", filepath.Base(n))
			}
		}
		created, err := e.Ensure(true, *recreate || *rebuild)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		img, why := e.Image()
		state := "running"
		if created {
			state = "created and running"
		}
		fmt.Fprintf(stdout, "boxer: %s %s (%s) image %s [%s] mounted %s -> %s\n", e.Scope.Names(), state, e.Scope.Isolation, img, why, e.Scope.Root, e.MountAt())
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
			fmt.Fprintf(stdout, "boxer: %s had no sandbox\n", e.Scope.Names())
			return 0
		}
		fmt.Fprintf(stdout, "boxer: %s removed\n", e.Scope.Names())
		return 0
	case "run":
		argv := fs.Args()
		var chosen config.Task
		if *taskName != "" {
			t, err := task(e, *taskName)
			if err != nil {
				emit(stdout, errorRow(err), *asJSON)
				fmt.Fprintln(stderr, err)
				return 1
			}
			if len(argv) > 0 || *shellLine != "" {
				fmt.Fprintln(stderr, "boxer run: --task names the whole command; do not add -c or arguments")
				return 2
			}
			chosen = t
			argv = []string{"sh", "-c", t.Cmd}
			if len(t.Env) > 0 { // env(1) sets the task's variables in the guest without quoting them into the line
				pre := []string{"env"}
				for _, k := range slices.Sorted(maps.Keys(t.Env)) {
					pre = append(pre, k+"="+t.Env[k])
				}
				argv = append(pre, argv...)
			}
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
		tty := *forceTTY || (cli.IsTerminal(os.Stdin) && cli.IsTerminal(os.Stdout))
		if *forceTTY {
			// A caller that asked for a terminal explicitly is driving this shell, and it may write
			// to it before the guest exists. Capture that input rather than let the guest terminal
			// discard it when it opens.
			stdin = readEarly(stdin)
		}
		timeout := time.Duration(0)
		if chosen.Timeout != "" {
			timeout, _ = time.ParseDuration(chosen.Timeout) // validated at load
		}
		started := time.Now()
		code, err := e.Run(argv, box.RunOpts{Stdin: stdin, Stdout: stdout, Stderr: stderr, TTY: tty, Timeout: timeout, Task: *taskName})
		if err != nil {
			if code == -1 { // passthrough policy: run on the host
				fmt.Fprintln(stderr, err)
				return hostRun(argv, stdin, stdout, stderr)
			}
			fmt.Fprintln(stderr, err)
			return code
		}
		return withTestResults(e, chosen, *junitPaths, *failOnTests, started, code, stderr)
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

// emit writes v as one indented JSON document when asJSON is set and reports whether it did;
// callers print the human form otherwise. Every --json command goes through here.
func emit(w io.Writer, v any, asJSON bool) bool {
	// BOXER_OUTPUT=json makes every command that has a JSON form use it, for a caller that cannot
	// add --json to each invocation — an agent's shell, a wrapper script.
	if !asJSON && !strings.EqualFold(os.Getenv("BOXER_OUTPUT"), "json") {
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
	Scope string `json:"scope"`
	// Name is the same sandbox said out loud — "swift-crab" for sb-7e1852e4a3c3. It is derived
	// from the key, so it costs nothing to carry and nothing has to agree on it.
	Name        string  `json:"name"`
	State       string  `json:"state"`
	Isolation   string  `json:"isolation"`
	Worktree    string  `json:"worktree"`
	Integration string  `json:"integration"`
	Image       string  `json:"image"`
	CreatedAt   int64   `json:"created_at"`
	LastUsed    *string `json:"last_used"`
	// Who the sandbox belongs to, when the harness said: a listing that can name the session is
	// the difference between a list of hashes and something a person can act on.
	Harness string `json:"harness,omitempty"`
	Session string `json:"session,omitempty"`
	Agent   string `json:"agent,omitempty"`
	// What it costs, when asked for: allocation, real use, and disk. Measuring means running `ps`
	// and walking a directory per machine, so a plain listing does not pay for it.
	Resources *box.Resources `json:"resources,omitempty"`
	// URLs are the stable names its ports are reachable at, when [urls] is on.
	URLs map[string]string `json:"urls,omitempty"`
}

// withResources measures one machine. Failure to measure is reported in the row rather than as an
// error, because a listing that dies over a resource number is worse than one that is honest
// about what it could not read.
// attachment names who a sandbox belongs to, for the human listing: the harness and the short form
// of the session, or the isolation when nothing said.
func attachment(m machineJSON) string {
	if m.Harness == "" {
		return m.Isolation
	}
	if m.Session == "" {
		return m.Harness
	}
	id := m.Session
	if len(id) > 6 {
		id = id[:6]
	}
	return m.Harness + "/" + id
}

// dataDir locates a machine's disk, or reports that it cannot be located. A backend with no
// notion of a per-machine directory is indistinguishable here from one whose lookup failed, and
// both mean the same thing to every caller: the size is unmeasured, which is not the same as zero.
// hostBackend is the backend this repository selects, for commands that sweep the whole host and
// so have no Env of their own. Outside a repository it returns "", and vm.Host falls back to the
// environment — which is the case `boxer gc` most often runs in.
func hostBackend() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return box.HostBackend(cwd)
}

func dataDir(client vm.Backend, name string) (string, error) {
	r, ok := client.(vm.DiskReporter)
	if !ok {
		return "", vm.Unsupported(client, "measure a machine's disk", "sizes are reported as unmeasured")
	}
	return r.DataDir(name)
}

func withResources(client vm.Backend, m vm.Machine, row machineJSON) machineJSON {
	dir, err := dataDir(client, m.Name)
	if err != nil {
		dir = ""
	}
	r := box.MachineResources(m, dir)
	if err != nil {
		r.MeasuredAll = false
	}
	row.Resources = &r
	return row
}

func machineRow(m vm.Machine) machineJSON {
	return machineJSON{
		Scope: m.Name, Name: scope.Slug(m.Name), State: m.State, Image: m.Image, CreatedAt: m.CreatedAt,
		Isolation: m.Labels["boxer.isolation"], Worktree: m.Labels["boxer.root"], Integration: m.Labels["boxer.integration"],
		Harness: m.Labels["boxer.harness"], Session: m.Labels["boxer.session"], Agent: m.Labels["boxer.agent"],
		LastUsed: lastUsedJSON(m.Name), URLs: box.MachineURLs(m),
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
	Scope string `json:"scope"`
	// Name is the scope key said out loud, the same one `ls` prints.
	Name        string            `json:"name"`
	Isolation   string            `json:"isolation"`
	Worktree    string            `json:"worktree"`
	MountAt     string            `json:"mount_at"`
	Exists      bool              `json:"exists"`
	State       string            `json:"state"`
	Image       string            `json:"image"`
	ImageReason string            `json:"image_reason"`
	Labels      map[string]string `json:"labels"`
	LastUsed    *string           `json:"last_used"`
	// Events is the tail of this scope's event log when telemetry writes one; a dashboard reads
	// status and wants the last few things that happened without a second command.
	Events []obs.Event `json:"events,omitempty"`
	// Resources is what this sandbox costs, measured when it exists. One sandbox is cheap to
	// measure, so unlike `ls` this does not need asking for.
	Resources *box.Resources `json:"resources,omitempty"`
	// Ports maps a guest port to the host port it was given, which is the only way to find an
	// automatically allocated one.
	Ports map[string]string `json:"ports,omitempty"`
	// URLs maps a guest port to its stable name, when [urls] is enabled: use this, not the host
	// port, which differs in every worktree.
	URLs map[string]string `json:"urls,omitempty"`
	// LastRun is what the last command in this scope did, when one has been recorded. It is what
	// `boxer capsule new` would capture.
	LastRun *box.RunRecord `json:"last_run,omitempty"`
}

// Exit codes of `boxer status`.
const (
	exitStopped = 3
	exitAbsent  = 4
)

// sortedMapKeys keeps printed output stable, because a map is not.
func sortedMapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func statusCmd(e *box.Env, asJSON bool, stdout, stderr io.Writer) int {
	m, ok, err := e.Exists()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	img, why := e.Image()
	s := statusJSON{
		Scope: e.Scope.Key, Name: e.Scope.Slug(), Isolation: e.Scope.Isolation, Worktree: e.Scope.Root, MountAt: e.MountAt(),
		Exists: ok, State: "absent", Image: img, ImageReason: why, Labels: map[string]string{}, LastUsed: lastUsedJSON(e.Scope.Key),
	}
	s.Events, _ = obs.Read(obs.Current().Path, e.Scope.Key, 5)
	if rec, found := box.ReadRunRecord(e.Scope.Key); found {
		s.LastRun = &rec
	}
	code := exitAbsent
	if ok {
		s.State, s.Image, s.ImageReason, s.Labels = m.State, m.Image, "machine", m.Labels
		// One sandbox, so measuring is cheap enough to do unasked: this is the command someone
		// runs when they want to know about *this* worktree, and its cost is part of that.
		dir, derr := dataDir(vm.Host(hostBackend()), m.Name)
		if derr != nil {
			dir = ""
		}
		r := box.MachineResources(m, dir)
		if derr != nil {
			r.MeasuredAll = false
		}
		s.Resources = &r
		s.Ports = box.PortsOf(m)
		s.URLs = e.URLs(m)
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
		fmt.Fprintf(stdout, "boxer: %s (%s) absent (image would be %s: %s)\n", s.Name, s.Scope, img, why)
	} else {
		fmt.Fprintf(stdout, "boxer: %s (%s) %s, image %s, mounted %s -> %s\n", s.Name, s.Scope, s.State, s.Image, s.Worktree, s.MountAt)
		// Where the forwarded ports actually landed: with `auto` this is the only way to know.
		for _, guest := range sortedMapKeys(s.Ports) {
			fmt.Fprintf(stdout, "  port:      guest %s -> http://127.0.0.1:%s\n", guest, s.Ports[guest])
		}
		for _, guest := range sortedMapKeys(s.URLs) {
			fmt.Fprintf(stdout, "  url:       guest %s -> %s\n", guest, s.URLs[guest])
		}
	}
	return code
}

func isShim(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "boxer shim")
}

// gcJSON is one `gc` decision: Deleted is false under --dry-run or when Error is set.
type gcJSON struct {
	machineJSON
	Pack    string `json:"pack,omitempty"` // set for pack rows; machine fields are then empty
	Bytes   int64  `json:"bytes,omitempty"`
	Reason  string `json:"reason"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

func gcCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "print what would be deleted")
	all := fs.Bool("all", false, "ignore idle_timeout: reclaim every sandbox whose worktree is gone, every stopped sandbox, and every pack")
	asJSON := fs.Bool("json", false, "print a JSON array of decisions")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := vm.Host(hostBackend())
	// Drop marks for machines that are gone before deciding what to reclaim, so a backend that
	// keeps its ownership record on the host does not accumulate one entry per deleted sandbox.
	vm.PruneOwned(client)
	ms, err := vm.Owned(client)
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
	// For sandboxes, --all turns the idle rule off rather than making everything idle: a running
	// sandbox is in use, and only a stopped one is taken below. Packs are aged separately.
	var idle time.Duration
	if !*all && cfg.IdleTimeout != "" && cfg.IdleTimeout != "never" {
		if idle, err = time.ParseDuration(cfg.IdleTimeout); err != nil {
			fmt.Fprintf(stderr, "idle_timeout = %q: %v\n", cfg.IdleTimeout, err)
			return 1
		}
	}
	code := 0
	rows := []gcJSON{}
	live := map[string]bool{}
	for _, m := range ms {
		live[m.Name] = true
	}
	for _, m := range ms {
		root := m.Labels["boxer.root"]
		reason := ""
		if parent := box.ParentOf(m.Name); parent != "" && !live[parent] {
			// A fork child outlives its parent only by accident: nothing addresses it, and it
			// costs what a sandbox costs.
			reason = "fork of " + parent + ", which is gone"
		} else if _, err := os.Stat(root); err != nil {
			reason = fmt.Sprintf("worktree %s is gone", root)
		} else if last := box.LastUsed(m.Name); idle > 0 && !last.IsZero() && time.Since(last) > idle {
			reason = fmt.Sprintf("idle since %s (idle_timeout %s)", last.Format(time.RFC3339), cfg.IdleTimeout)
		} else if *all && m.State != "running" {
			reason = "not running (--all)"
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
		// Everything boxer kept about the sandbox goes with it: its run record, its last-used
		// stamp, its lock, and any URL that pointed at it.
		box.ForgetScope(m.Name)
		row.Deleted = true
		rows = append(rows, row)
		if !*asJSON {
			fmt.Fprintf(stdout, "deleted %s (%s)\n", m.Name, reason)
		}
	}
	// State for sandboxes that went away without boxer deleting them — `docker rm`, a wiped
	// store — which no row above could see.
	if !*dry {
		for _, r := range rows {
			if r.Deleted {
				delete(live, r.Scope)
			}
		}
		box.PruneURLs(live)
		box.SweepState(live, time.Now())
	}
	reason := fmt.Sprintf("pack unused for %s", cfg.IdleTimeout)
	if *all {
		reason = "pack unreferenced (--all)"
	}
	keepLast, packAge := cfg.PacksKeepLast, idle
	if *all {
		keepLast = 1 // --all means the cache goes, not that it is trimmed
		packAge = time.Nanosecond
	}
	for _, p := range box.StalePacks(ms, packAge, keepLast) {
		size := int64(0)
		if st, err := os.Stat(p); err == nil {
			size = st.Size()
		}
		row := gcJSON{Pack: p, Bytes: size, Reason: reason}
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
		_ = os.Remove(strings.TrimSuffix(p, ".smolmachine") + ".lock")
		row.Deleted = true
		rows = append(rows, row)
		if !*asJSON {
			fmt.Fprintf(stdout, "deleted pack %s (%s)\n", p, row.Reason)
		}
	}
	machines, packs, freed := 0, 0, int64(0)
	for _, r := range rows {
		if r.Pack != "" {
			packs++
			freed += r.Bytes
		} else {
			machines++
		}
	}
	if !*asJSON && (machines > 0 || packs > 0) {
		verb := "reclaimed"
		if *dry {
			verb = "would reclaim"
		}
		fmt.Fprintf(stdout, "%s %d sandbox(es) and %d pack(s), %s of cache\n", verb, machines, packs, box.HumanBytes(freed))
	}
	obs.Emit(obs.Event{Name: obs.GC, Outcome: obs.OK, Payload: map[string]any{"machines": machines, "packs": packs, "dry_run": *dry}})
	emit(stdout, rows, *asJSON)
	return code
}

// logsCmd reads the file sink back. There is no other reader: the stream is JSON on disk, and a
// command that pretty-prints it is the difference between telemetry and a file nobody opens.
func logsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	scopeName := fs.String("scope", "", "only events for this sandbox (`boxer status` prints the key)")
	n := fs.Int("n", 0, "print only the last n events")
	asJSON := fs.Bool("json", false, "print a JSON array of events")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := config.Load(cwdRoot())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	obs.ConfigureFrom(cfg.Telemetry)
	path := obs.Current().Path
	if path == "" {
		path = obs.DefaultPath()
	}
	events, err := obs.Read(path, *scopeName, *n)
	if err != nil {
		fmt.Fprintf(stderr, "boxer: no event log at %s\n  fix:       set [telemetry] enabled = true in boxer.toml, or BOXER_TRACE=<path>\n", path)
		return 1
	}
	if events == nil {
		events = []obs.Event{}
	}
	if emit(stdout, events, *asJSON) {
		return 0
	}
	for _, e := range events {
		fmt.Fprintln(stdout, e.Line())
	}
	return 0
}

func downAll(client vm.Backend, asJSON bool, stdout, stderr io.Writer) int {
	ms, err := vm.Owned(client)
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
		vm.ForgetOwned(client, m.Name)
		box.ForgetScope(m.Name)
		rows = append(rows, downJSON{Scope: m.Name, Removed: true})
		if !asJSON {
			fmt.Fprintf(stdout, "boxer: %s removed\n", m.Name)
		}
	}
	// "Everything" includes routes whose sandbox went away by another path: a `down` run with a
	// different state directory removes the route but cannot see this directory's record of it.
	box.PruneURLs(map[string]bool{})
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
	_ = os.Setenv("BOXER_INTEGRATION", "inside") // this command is the inside placement by definition
	e, err := box.Resolve("", name, scope.Identity{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tty := kind == "shell" && cli.IsTerminal(os.Stdin) && cli.IsTerminal(os.Stdout)
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
		fmt.Fprintln(stderr, "usage: boxer package plugin|skills|<harness>|all [--out dir]")
		return 2
	}
	target := pos[0]
	// The skill is published twice more: at the non-hidden installer path a skill manager reads,
	// and in the site's .well-known index a domain-discovery client reads. Both come from the
	// same template as the plugin, so there is one document and two projections of it.
	if target == "skills" || target == "all" {
		files, err := bundle.RenderSkills(Version, "skills", filepath.Join("site", "public", ".well-known"))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "skills: %d files in skills/ and site/public/.well-known/\n", len(files))
		if target == "skills" {
			return 0
		}
	}
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
	if pos[0] == "conductor" {
		r, err := install.Conductor(wt, shim.DefaultDir())
		if err != nil {
			fmt.Fprintln(stderr, "boxer install conductor:", err)
			return 1
		}
		fmt.Fprintf(stdout, "conductor:\n  wrote %s\n", r.Written[0])
		for _, n := range r.Notes {
			fmt.Fprintf(stdout, "  note: %s\n", n)
		}
		return 0
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
			names = []string{"claude-code", "codex", "copilot"}
		}
	}
	code := 0
	for _, h := range names {
		var r install.Result
		if *user {
			r, err = install.User(h, cfg.ForHarness(h), version())
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
	// TaskDetails carries what a table-form task declares beside its command line. Tasks stays a
	// name→command map so a reader written against an older boxer keeps working.
	TaskDetails map[string]taskJSON `json:"task_details,omitempty"`
	// Ports is the forwarding the configuration asks for; URLs says whether they get stable names.
	// Where they actually landed is `status`, because that depends on the running sandbox.
	Ports []string `json:"ports"`
	URLs  bool     `json:"urls"`
}

// briefCmd is the run-time half of static published content: the skill and the hooks say to run
// this, so no rendered file has to carry anyone's configuration.
func briefCmd(e *box.Env, asJSON bool, stdout io.Writer) int {
	if emit(stdout, briefJSON{
		Brief: e.Instructions(), Scope: scopeRow(e.Scope), Isolation: e.Cfg.Isolation, MountAt: e.MountAt(),
		Mode: e.Cfg.Mode, Enforcement: e.Cfg.Enforcement, Intercept: e.Cfg.Intercept,
		Passthrough: e.Cfg.Passthrough, Tasks: tasksOrEmpty(e.Cfg.Tasks), TaskDetails: taskDetails(e.Cfg),
		Ports: append([]string{}, e.Cfg.Network.Ports...), URLs: e.Cfg.URLs.Enabled,
	}, asJSON) {
		return 0
	}
	// No task footer: the brief itself now lists each task with what it is for, and printing the
	// names twice made the useful line the one nobody read.
	fmt.Fprintln(stdout, e.Instructions())
	return 0
}

type taskJSON struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Description string   `json:"description,omitempty"`
	JUnit       []string `json:"junit,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
}

func taskRow(name string, t config.Task) taskJSON {
	return taskJSON{Name: name, Command: t.Cmd, Description: t.Description, JUnit: t.JUnit, Timeout: t.Timeout}
}

// taskDetails is the table-form half of a task, keyed by name, and nil when no task declares any:
// a repository that only writes `test = "make test"` sees no new JSON at all.
func taskDetails(cfg config.Config) map[string]taskJSON {
	out := map[string]taskJSON{}
	for _, n := range cfg.TaskNames() {
		t := cfg.Tasks[n]
		if t.Description == "" && len(t.JUnit) == 0 && t.Timeout == "" && len(t.Env) == 0 {
			continue
		}
		out[n] = taskRow(n, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func emitTasks(e *box.Env, asJSON bool, stdout io.Writer) {
	rows := []taskJSON{}
	for _, n := range e.Cfg.TaskNames() {
		rows = append(rows, taskRow(n, e.Cfg.Tasks[n]))
	}
	if emit(stdout, rows, asJSON) {
		return
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "boxer: this repository declares no tasks; add a [tasks] table to boxer.toml")
		return
	}
	for _, t := range rows {
		what := t.Command
		if t.Description != "" {
			what = t.Description
		}
		fmt.Fprintf(stdout, "%-16s %s\n", t.Name, what)
	}
}

// task resolves a task name, refusing an unknown one in the shape an agent can act on.
func task(e *box.Env, name string) (config.Task, error) {
	if t, ok := e.Cfg.Tasks[name]; ok {
		return t, nil
	}
	known := "none declared; add a [tasks] table to boxer.toml"
	if names := e.Cfg.TaskNames(); len(names) > 0 {
		known = "boxer run --task " + strings.Join(names, " | ")
	}
	return config.Task{}, &box.Error{
		Reason: fmt.Sprintf("no task named %q in boxer.toml", name),
		Cause:  "NO_SUCH_TASK", Scope: e.Scope, Fix: known,
	}
}

func tasksOrEmpty(m map[string]config.Task) map[string]string {
	out := map[string]string{}
	for n, t := range m {
		out[n] = t.Cmd
	}
	return out
}

// watchCmd streams what is happening, so an interface tails one process instead of polling. Two
// sources, one line-delimited stream: state changes it notices by comparing successive listings,
// and events from the file sink when telemetry is on. Line-delimited JSON rather than an array,
// because a stream has no end and a consumer must be able to read it as it arrives.
func watchCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "one JSON document per line")
	interval := fs.Duration("interval", time.Second, "how often to look for state changes")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	client := vm.Host(hostBackend())
	seen := map[string]string{}
	first := true
	enc := json.NewEncoder(stdout)
	// The event log, when telemetry writes one: a state change says a VM started, an event says
	// what it was asked to do. An interface wants both in one stream.
	cfg, _ := config.Load(cwdRoot())
	obs.ConfigureFrom(cfg.Telemetry)
	events := obs.Current().Path
	if events == "" {
		events = obs.DefaultPath()
	}
	sentEvents := 0
	if seenNow, err := obs.Read(events, "", 0); err == nil {
		sentEvents = len(seenNow) // only what happens from now on
	}
	for {
		ms, err := vm.Owned(client)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		now := map[string]string{}
		for _, m := range ms {
			now[m.Name] = m.State
			row := machineRow(m)
			switch was, had := seen[m.Name]; {
			case !had && first:
				emitWatch(enc, stdout, *asJSON, "present", row)
			case !had:
				emitWatch(enc, stdout, *asJSON, "created", row)
			case was != m.State:
				emitWatch(enc, stdout, *asJSON, m.State, row)
			}
		}
		for name := range seen {
			if _, still := now[name]; !still {
				emitWatch(enc, stdout, *asJSON, "gone", machineJSON{Scope: name, State: "gone"})
			}
		}
		if all, err := obs.Read(events, "", 0); err == nil && len(all) > sentEvents {
			for _, e := range all[sentEvents:] {
				emitEvent(enc, stdout, *asJSON, e)
			}
			sentEvents = len(all)
		}
		seen, first = now, false
		time.Sleep(*interval)
	}
}

// watchEvent is one line of the stream: what happened, to which sandbox, when.
type watchEvent struct {
	Time    string      `json:"time"`
	Change  string      `json:"change"`
	Sandbox machineJSON `json:"sandbox"`
}

// emitEvent puts one event from the log into the same stream as the state changes, so a consumer
// reads one thing rather than correlating two.
func emitEvent(enc *json.Encoder, w io.Writer, asJSON bool, e obs.Event) {
	if asJSON {
		_ = enc.Encode(struct {
			Time   string    `json:"time"`
			Change string    `json:"change"`
			Event  obs.Event `json:"event"`
		}{Time: e.Time.UTC().Format(time.RFC3339), Change: "event", Event: e})
		return
	}
	outcome := e.Outcome
	if outcome == "" {
		outcome = "-"
	}
	fmt.Fprintf(w, "%s  %-9s %-16s %-14s %s\n", e.Time.Format("15:04:05"), e.Name, e.Scope, e.Harness, outcome)
}

func emitWatch(enc *json.Encoder, w io.Writer, asJSON bool, change string, row machineJSON) {
	if asJSON {
		_ = enc.Encode(watchEvent{Time: time.Now().UTC().Format(time.RFC3339), Change: change, Sandbox: row})
		return
	}
	who := attachment(row)
	fmt.Fprintf(w, "%s  %-9s %-16s %-14s %s\n", time.Now().Format("15:04:05"), change, row.Scope, who, row.Worktree)
}

// junitPatterns is where a run should look for reports: the flag, then the task's own list, then
// the repository's [results] table. The first that names anything wins, because a caller that
// passed --junit means that one.
func junitPatterns(cfg config.Config, t config.Task, flagValue string) []string {
	if flagValue != "" {
		return strings.Split(flagValue, ",")
	}
	if len(t.JUnit) > 0 {
		return t.JUnit
	}
	return cfg.Results.Auto
}

// withTestResults summarises the run's JUnit reports, attaches them to the run record, and decides
// the exit code. Parsing happens on the host: the worktree is mounted, so a report the guest wrote
// under it is already a file here.
func withTestResults(e *box.Env, t config.Task, flagValue string, failFlag bool, started time.Time, code int, stderr io.Writer) int {
	pats := junitPatterns(e.Cfg, t, flagValue)
	if len(pats) == 0 {
		return code
	}
	s, err := junit.Collect(e.Scope.Root, pats, started)
	if err != nil {
		fmt.Fprintf(stderr, "boxer: %v\n", err)
		return code
	}
	if len(s.Files) == 0 {
		return code
	}
	fmt.Fprintf(stderr, "boxer: %s\n", s.Line())
	for i, name := range s.Failed {
		if i == 10 {
			fmt.Fprintf(stderr, "boxer:   … %d more\n", len(s.Failed)-10)
			break
		}
		fmt.Fprintf(stderr, "boxer:   FAIL %s\n", name)
	}
	box.AmendRunTests(e.Scope.Key, &box.TestSummary{
		Tests: s.Tests, Failures: s.Failures, Errors: s.Errors, Skipped: s.Skipped, Failed: s.Failed,
	})
	// Never replace a non-zero code with a different one: the command's own exit status is the
	// more specific answer, and a caller keying off it would be misled.
	if code == 0 && s.Bad() && (failFlag || e.Cfg.Results.FailOnTestFailures) {
		return 1
	}
	return code
}
