package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/cli"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/install"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// The commands a person — or an agent — uses to see and manage what boxer has on this machine.
// They are the CLI's own: everything here is built from the same public pieces every other
// command uses (vm.Backend, box.Env, install), and nothing in the core knows these commands exist.

// parseAnywhere parses flags wherever they appear among the arguments. The standard library stops
// at the first positional, so `boxer backends docker --probe` silently ignored --probe and
// `boxer rm swift-crab -y` would have asked anyway. It returns the positionals.
func parseAnywhere(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// backendNames are every backend, in the order they are shown.
var backendNames = vm.Names

func backendBin(b vm.Backend) string { return vm.Bin(b) }

// hostMachine is one sandbox and the backend it lives on.
type hostMachine struct {
	backend string
	client  vm.Backend
	m       vm.Machine
}

// listMachines returns boxer's sandboxes on the configured backend, or on every installed one.
// A backend that is installed but not answering is reported, not fatal: the others still list,
// and ok is false so the command does not exit 0 as if there were nothing.
func listMachines(all bool, stderr io.Writer) (_ []hostMachine, ok bool) {
	ok = true
	names := []string{hostBackend()}
	if all {
		names = backendNames
	}
	var out []hostMachine
	for _, n := range names {
		c := vm.Host(n)
		if all && !vm.Installed(c) {
			continue
		}
		ms, err := vm.Owned(c)
		if err != nil {
			fmt.Fprintf(stderr, "boxer: %s: %v\n", c.Name(), err)
			ok = false
			continue
		}
		for _, m := range ms {
			out = append(out, hostMachine{backend: c.Name(), client: c, m: m})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].m.Name < out[j].m.Name })
	return out, ok
}

// gitJSON is the state of the worktree a sandbox is attached to.
type gitJSON struct {
	Exists   bool    `json:"exists"`
	Branch   string  `json:"branch,omitempty"`
	Detached bool    `json:"detached,omitempty"`
	Dirty    bool    `json:"dirty"`
	Changed  int     `json:"changed"`
	Ahead    int     `json:"ahead"`
	Behind   int     `json:"behind"`
	PR       *prJSON `json:"pr,omitempty"`
}

type prJSON struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	URL    string `json:"url"`
}

// worktreeGit reads a worktree's branch, dirtiness and distance from upstream in one git call,
// and its pull request through `gh` when asked — that one is a network round trip, so it is opt-in.
func worktreeGit(root string, withPR bool) *gitJSON {
	g := &gitJSON{}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return g
	}
	g.Exists = true
	out, err := scope.GitCommand(root, "status", "--porcelain=v2", "--branch").Output()
	if err != nil {
		return g
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			g.Branch = strings.TrimPrefix(line, "# branch.head ")
			g.Detached = g.Branch == "(detached)"
		case strings.HasPrefix(line, "# branch.ab "):
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &g.Ahead, &g.Behind)
		case line != "" && !strings.HasPrefix(line, "#"):
			g.Changed++
		}
	}
	g.Dirty = g.Changed > 0
	if withPR && g.Branch != "" && !g.Detached {
		if _, err := exec.LookPath("gh"); err == nil {
			cmd := exec.Command("gh", "pr", "view", g.Branch, "--json", "number,state,url")
			cmd.Dir = root
			if b, err := cmd.Output(); err == nil {
				var pr prJSON
				if json.Unmarshal(b, &pr) == nil && pr.Number > 0 {
					g.PR = &pr
				}
			}
		}
	}
	return g
}

func (g *gitJSON) summary() string {
	switch {
	case !g.Exists:
		return "gone"
	case g.Dirty:
		return fmt.Sprintf("dirty(%d)", g.Changed)
	}
	return "clean"
}

func (g *gitJSON) branchLabel() string {
	b := g.Branch
	if g.Detached {
		b = "(detached)"
	}
	if g.Ahead > 0 || g.Behind > 0 {
		b += fmt.Sprintf(" ↑%d↓%d", g.Ahead, g.Behind)
	}
	if g.PR != nil {
		b += fmt.Sprintf(" #%d", g.PR.Number)
	}
	return b
}

// lsRow is `ls --json`: the machine, where it lives, and the worktree it serves.
type lsRow struct {
	machineJSON
	Backend string            `json:"backend"`
	Ports   map[string]string `json:"ports,omitempty"`
	Git     *gitJSON          `json:"git,omitempty"`
}

func lsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	resources := fs.Bool("resources", false, "measure what each sandbox is using: memory, CPU and disk")
	all := fs.Bool("all-backends", false, "list every installed backend, not only the configured one")
	fs.BoolVar(all, "A", false, "shorthand for --all-backends")
	pr := fs.Bool("pr", false, "look up each worktree's pull request with gh (a network call per worktree)")
	noGit := fs.Bool("no-git", false, "skip reading each worktree's git state")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	out := cli.New(stdout, *asJSON)
	rows := []lsRow{}
	hs, listed := listMachines(*all, stderr)
	code := map[bool]int{true: 0, false: 1}[listed]
	for _, h := range hs {
		row := lsRow{machineJSON: machineRow(h.m), Backend: h.backend, Ports: box.PortsOf(h.m)}
		if *resources {
			row.machineJSON = withResources(h.client, h.m, row.machineJSON)
		}
		if !*noGit {
			row.Git = worktreeGit(row.Worktree, *pr)
		}
		rows = append(rows, row)
	}
	if out.Mode == cli.JSON {
		emit(stdout, rows, true)
		return code
	}
	if len(rows) == 0 {
		if !listed {
			return code
		}
		out.Printf("no boxer sandboxes%s\n", map[bool]string{true: " on any backend", false: " on " + vm.Host(hostBackend()).Name()}[*all])
		if !*all {
			out.Hint("boxer ls -A", "look on every installed backend")
		}
		return 0
	}
	// Who a sandbox belongs to is shown when anyone said; a column of dashes is noise.
	attached := false
	for _, r := range rows {
		if attachment(r.machineJSON) != "" {
			attached = true
		}
	}
	header := []string{"NAME", "SCOPE", "STATE", "BACKEND", "BRANCH", "GIT"}
	if attached {
		header = append(header, "ATTACHED")
	}
	header = append(header, "SERVES")
	if *resources {
		header = append(header, "MEMORY", "DISK")
	}
	header = append(header, "WORKTREE")
	var table [][]string
	for _, r := range rows {
		gitState, branch := "-", "-"
		if r.Git != nil {
			gitState, branch = r.Git.summary(), r.Git.branchLabel()
		}
		serves := "-"
		if len(r.URLs) > 0 {
			serves = firstSorted(r.URLs)
		} else if len(r.Ports) > 0 {
			k := sortedMapKeys(r.Ports)[0]
			serves = ":" + k + "→" + r.Ports[k]
		}
		cells := []string{out.Bold(r.Name), out.Dim(r.Scope), out.State(r.State), r.Backend, branch, out.State(gitState)}
		if attached {
			cells = append(cells, firstNonBlank(attachment(r.machineJSON), "-"))
		}
		cells = append(cells, serves)
		if *resources {
			mem, disk := "-", "-"
			if r.Resources != nil {
				if r.Resources.RSSMiB > 0 {
					mem = fmt.Sprintf("%d MiB", r.Resources.RSSMiB)
				}
				if r.Resources.DiskBytes > 0 {
					disk = box.HumanBytes(r.Resources.DiskBytes)
				}
			}
			cells = append(cells, mem, disk)
		}
		table = append(table, append(cells, shortPath(r.Worktree)))
	}
	out.Table(header, table)
	if out.Mode == cli.Human {
		gone := 0
		for _, r := range rows {
			if r.Git != nil && !r.Git.Exists {
				gone++
			}
		}
		if gone > 0 {
			out.Hint("boxer rm --gone", plural(gone, "sandbox serves", "sandboxes serve")+" a worktree that no longer exists")
		}
		out.Hint("boxer rm -i", "choose sandboxes to remove")
	}
	return code
}

// plural is "1 sandbox" or "3 sandboxes".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func firstSorted(m map[string]string) string { return m[sortedMapKeys(m)[0]] }

// shortPath replaces the home directory with ~, which is what a person reads.
func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// selectMachines turns names (a slug or a scope key) and filters into the sandboxes they mean.
func selectMachines(ms []hostMachine, names []string, gone, stopped bool) ([]hostMachine, error) {
	var out []hostMachine
	matched := map[string]bool{}
	for _, h := range ms {
		pick := false
		for _, n := range names {
			if n == h.m.Name || n == scope.Slug(h.m.Name) {
				pick, matched[n] = true, true
			}
		}
		if gone {
			if _, err := os.Stat(h.m.Labels[vm.LabelPrefix+"root"]); err != nil {
				pick = true
			}
		}
		if stopped && !h.m.Running() {
			pick = true
		}
		if pick {
			out = append(out, h)
		}
	}
	for _, n := range names {
		if !matched[n] {
			return nil, fmt.Errorf("no boxer sandbox named %q (boxer ls -A lists them)", n)
		}
		// Two-word names can collide; one that matches several is refused, not applied to all.
		var keys []string
		for _, h := range out {
			if n != h.m.Name && n == scope.Slug(h.m.Name) {
				keys = append(keys, h.m.Name)
			}
		}
		if len(keys) > 1 {
			return nil, fmt.Errorf("%q names %d sandboxes (%s); name one by its key", n, len(keys), strings.Join(keys, ", "))
		}
	}
	return out, nil
}

// pick asks a person which sandboxes to act on. It is never reached for an agent.
func pick(out *cli.Out, in io.Reader, ms []hostMachine, verb string) []hostMachine {
	var rows [][]string
	for i, h := range ms {
		rows = append(rows, []string{strconv.Itoa(i + 1), h.m.Name, scope.Slug(h.m.Name), out.State(h.m.State), h.backend, shortPath(h.m.Labels[vm.LabelPrefix+"root"])})
	}
	out.Table([]string{"#", "SCOPE", "NAME", "STATE", "BACKEND", "WORKTREE"}, rows)
	out.Printf("%s which? (numbers, ranges like 1-3, 'all', or empty to cancel): ", verb)
	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if line == "all" {
		return ms
	}
	var chosen []hostMachine
	seen := map[int]bool{}
	for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' }) {
		lo, hi := part, part
		if a, b, ok := strings.Cut(part, "-"); ok {
			lo, hi = a, b
		}
		a, err1 := strconv.Atoi(lo)
		b, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil {
			continue
		}
		for i := a; i <= b; i++ {
			if i >= 1 && i <= len(ms) && !seen[i] {
				seen[i] = true
				chosen = append(chosen, ms[i-1])
			}
		}
	}
	return chosen
}

func confirm(out *cli.Out, in io.Reader, question string) bool {
	out.Printf("%s [y/N] ", question)
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
}

// stopRmCmd is `boxer stop` and `boxer rm`. With names it acts on those; with --all, --gone or
// --stopped on what they select; with -i it asks. An agent is never asked anything: it names what
// it means, or it is told how to.
func stopRmCmd(verb string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	all := fs.Bool("all", false, verb+" every boxer sandbox on the configured backend (with -A, on every backend)")
	gone := fs.Bool("gone", false, verb+" sandboxes whose worktree no longer exists")
	stopped := fs.Bool("stopped", false, verb+" sandboxes that are not running")
	interactive := fs.Bool("i", false, "choose interactively")
	yes := fs.Bool("y", false, "do not ask for confirmation")
	everywhere := fs.Bool("A", false, "look on every installed backend")
	asJSON := fs.Bool("json", false, "print what was done as JSON")
	var volumes *bool
	if verb == "rm" {
		volumes = fs.Bool("volumes", false, "also delete the sandboxes' named volumes, which rm otherwise keeps")
	}
	fs.SetOutput(stderr)
	names, err := parseAnywhere(fs, args)
	if err != nil {
		return 2
	}
	out := cli.New(stdout, *asJSON)
	// --all is every sandbox on the configured backend, as it is for `down --all`; every backend
	// is -A, said explicitly. It used to imply -A, and a unit test's `rm --all` reached the real
	// docker daemon and deleted another test's container — which is exactly what it would have done
	// to a person's sandboxes on a runtime they were not thinking about.
	ms, listed := listMachines(*everywhere, stderr)
	failed := map[bool]int{true: 0, false: 1}[listed]
	var targets []hostMachine
	switch {
	case *all:
		targets = ms
	case *interactive || (len(names) == 0 && !*gone && !*stopped):
		if out.Mode != cli.Human {
			fmt.Fprintf(stderr, "boxer: %s needs to know which sandboxes: name them, or pass --all, --gone or --stopped\n", verb)
			fmt.Fprintf(stderr, "boxer: fix: boxer ls --json, then boxer %s <name>\n", verb)
			return 2
		}
		if len(ms) == 0 {
			if listed {
				out.Printf("no boxer sandboxes\n")
			}
			return failed
		}
		targets = pick(out, stdin, ms, verb)
		*yes = true // choosing them was the confirmation
	default:
		var err error
		if targets, err = selectMachines(ms, names, *gone, *stopped); err != nil {
			fmt.Fprintf(stderr, "boxer: %v\n", err)
			return 1
		}
	}
	if len(targets) == 0 {
		if out.Mode == cli.JSON {
			emit(stdout, []downJSON{}, true)
		} else if listed {
			out.Printf("nothing to %s\n", verb)
		}
		return failed
	}
	// A filter is a question about what it will match, so a person is shown the answer first; a
	// sandbox named on the command line was the answer already.
	if verb == "rm" && (len(targets) > 1 || *all || *gone || *stopped) && !*yes {
		if out.Mode != cli.Human {
			// What a person is asked, an agent or a pipe is refused: it deleted every sandbox on
			// the host without a question, other worktrees' included.
			fmt.Fprintf(stderr, "boxer: rm would remove %s; a person is asked, and anything else must say so\n", plural(len(targets), "sandbox", "sandboxes"))
			fmt.Fprintln(stderr, "boxer: fix: add -y")
			return 2
		}
		if !confirm(out, stdin, "remove "+plural(len(targets), "sandbox", "sandboxes")+"?") {
			return 1
		}
	}
	code := failed
	rows := []downJSON{}
	for _, h := range targets {
		var err error
		row := downJSON{Scope: h.m.Name}
		if verb == "stop" {
			if err = h.client.Stop(h.m.Name); err == nil {
				box.Stopped(h.m.Name) // its next boot is not branchable
			}
		} else if err = h.client.Delete(h.m.Name); err == nil {
			vm.ForgetOwned(h.client, h.m.Name)
			box.ForgetScope(h.m.Name)
			row.Removed = true
			if *volumes {
				// The sandbox is gone either way; a row that hid it would say it was not.
				if verr := box.RemoveVolumes(h.m.Name); verr != nil {
					fmt.Fprintf(stderr, "boxer: rm %s: volumes: %v\n", h.m.Name, verr)
					row.Error = "volumes: " + verr.Error()
					code = 1
				}
			}
		}
		if err != nil {
			fmt.Fprintf(stderr, "boxer: %s %s: %v\n", verb, h.m.Name, err)
			row.Error = err.Error()
			rows = append(rows, row)
			code = 1
			continue
		}
		rows = append(rows, row)
		if out.Mode != cli.JSON {
			past := map[string]string{"stop": "stopped", "rm": "removed"}[verb]
			out.Printf("%s %s (%s)\n", out.Green(past), scope.Slug(h.m.Name), h.m.Name)
		}
	}
	if out.Mode == cli.JSON {
		emit(stdout, rows, true)
	}
	return code
}

// backendJSON is one row of `boxer backends`.
type backendJSON struct {
	Name       string   `json:"name"`
	Configured bool     `json:"configured"`
	Installed  bool     `json:"installed"`
	Binary     string   `json:"binary,omitempty"`
	Version    string   `json:"version,omitempty"`
	Reachable  bool     `json:"reachable"`
	Sandboxes  int      `json:"sandboxes"`
	Caps       vm.Caps  `json:"capabilities"`
	Problems   []string `json:"problems,omitempty"`
	Probe      *probe   `json:"probe,omitempty"`
}

type probe struct {
	OK      bool    `json:"ok"`
	Seconds float64 `json:"seconds"`
	Error   string  `json:"error,omitempty"`
}

// backendsCmd reports every backend: installed, answering, how many sandboxes, what it can do,
// and the misconfigurations that have actually cost time on a real machine. --probe goes further
// and runs a real sandbox on each, which is the only check that catches a runtime that answers
// and cannot run anything.
func backendsCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("backends", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	doProbe := fs.Bool("probe", false, "create, use and delete a small sandbox on each installed backend")
	fs.SetOutput(stderr)
	names, err := parseAnywhere(fs, args)
	if err != nil {
		return 2
	}
	only := ""
	if len(names) > 0 {
		only = names[0]
		if _, err := vm.Default(only); err != nil {
			fmt.Fprintf(stderr, "boxer: %v\n", err)
			return 2
		}
	}
	out := cli.New(stdout, *asJSON)
	configured := vm.Host(hostBackend()).Name()
	var rows []backendJSON
	code := 0
	for _, n := range backendNames {
		if only != "" && n != only {
			continue
		}
		c := vm.Host(n)
		r := backendJSON{Name: c.Name(), Configured: c.Name() == configured, Caps: vm.CapsOf(c)}
		if p, err := exec.LookPath(backendBin(c)); err == nil {
			r.Installed, r.Binary = true, p
		}
		if r.Installed {
			if v, err := c.Version(); err == nil {
				r.Version = versionNumber(v)
			}
			if ms, err := vm.Owned(c); err == nil {
				r.Reachable, r.Sandboxes = true, len(ms)
			} else {
				r.Problems = append(r.Problems, "not answering: "+firstLine(err.Error()))
			}
			r.Problems = append(r.Problems, knownProblems(n)...)
			if *doProbe && r.Reachable {
				r.Probe = runProbe(n)
			}
		}
		if (r.Configured && (!r.Installed || !r.Reachable)) || (r.Probe != nil && !r.Probe.OK) {
			code = 1
		}
		rows = append(rows, r)
	}
	if out.Mode == cli.JSON {
		emit(stdout, rows, true)
		return code
	}
	var table [][]string
	for _, r := range rows {
		name := r.Name
		if r.Configured {
			name = out.Bold(name + " *")
		}
		status := "missing"
		switch {
		case r.Installed && r.Reachable:
			status = "ready"
		case r.Installed:
			status = "down"
		}
		probeCell := "-"
		if r.Probe != nil {
			probeCell = out.Green(fmt.Sprintf("ok %.1fs", r.Probe.Seconds))
			if !r.Probe.OK {
				probeCell = out.Red("failed")
			}
		}
		allow := "no"
		if r.Caps.Allowlist {
			allow = "yes"
		}
		table = append(table, []string{name, out.State(status), firstNonBlank(r.Version, "-"), r.Caps.Boundary, allow, strconv.Itoa(r.Sandboxes), probeCell})
	}
	out.Table([]string{"BACKEND", "STATUS", "VERSION", "BOUNDARY", "ALLOWLIST", "SANDBOXES", "PROBE"}, table)
	for _, r := range rows {
		for _, p := range r.Problems {
			out.Printf("%s %s: %s\n", out.Yellow("!"), r.Name, p)
		}
		if r.Probe != nil && !r.Probe.OK {
			out.Printf("%s %s: probe failed: %s\n", out.Red("✗"), r.Name, r.Probe.Error)
		}
	}
	if out.Mode == cli.Human && !*doProbe {
		out.Hint("boxer backends --probe", "run a real sandbox on each: the only check that proves one works")
	}
	return code
}

var reVersion = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// versionNumber is the version a person compares, out of whatever a runtime prints around it.
func versionNumber(s string) string {
	if v := reVersion.FindString(s); v != "" {
		return v
	}
	return firstLine(s)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func firstNonBlank(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// knownProblems are configurations that looked fine and were not, each found on a real machine.
func knownProblems(backend string) []string {
	var p []string
	switch backend {
	case "podman":
		// The machine boxer talks to is the connection's; `podman machine` commands look only under
		// the configured provider. A libkrun machine could not bind-mount any host path, which is
		// every boxer sandbox's worktree.
		for _, provider := range []string{"libkrun", "applehv"} {
			cmd := exec.Command("podman", "machine", "list", "--format", "{{.Name}} {{.Running}}")
			cmd.Env = append(os.Environ(), "CONTAINERS_MACHINE_PROVIDER="+provider)
			b, err := cmd.Output()
			if err != nil || !strings.Contains(string(b), "true") {
				continue
			}
			if provider == "libkrun" {
				p = append(p, "the running podman machine uses the libkrun provider, which cannot bind-mount host paths; "+
					"recreate it with CONTAINERS_MACHINE_PROVIDER=applehv")
			}
		}
	case "container":
		if b, err := exec.Command("container", "system", "status").CombinedOutput(); err != nil || !strings.Contains(string(b), "running") {
			p = append(p, "the container system service is not running; fix: container system start")
		}
	}
	return p
}

// runProbe creates a throwaway repository, brings a sandbox up on the backend, proves a command
// runs and a write reaches the host, and removes everything.
func runProbe(backend string) *probe {
	start := time.Now()
	fail := func(err error) *probe {
		return &probe{OK: false, Seconds: time.Since(start).Seconds(), Error: firstLine(err.Error())}
	}
	dir, err := os.MkdirTemp("", "boxer-probe-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a temporary
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return fail(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"-c", "user.email=probe@boxer", "-c", "user.name=boxer", "commit", "-q", "--allow-empty", "-m", "probe"}} {
		if b, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
			return fail(fmt.Errorf("git: %v: %s", err, b))
		}
	}
	// boxer's own default network where the backend has it, so the probe tests what people run;
	// "off" where it does not, because the allowlist is refused there.
	mode := "off"
	if vm.CapsOf(vm.Host(backend)).Allowlist {
		mode = "allowlist"
	}
	// The probe's own file decides: BOXER_BACKEND, BOXER_IMAGE and the rest override a file, and
	// with one set every row probed that backend and reported it under its own name.
	for _, name := range config.SettingEnv() {
		if v, ok := os.LookupEnv(name); ok {
			_ = os.Unsetenv(name)
			defer os.Setenv(name, v) //nolint:errcheck // restoring what was there
		}
	}
	// The probe's own throwaway repository, with a network it sets itself: boxer's.
	if _, ok := os.LookupEnv("BOXER_TRUST"); !ok {
		_ = os.Setenv("BOXER_TRUST", "1")
		defer os.Unsetenv("BOXER_TRUST") //nolint:errcheck
	}
	toml := fmt.Sprintf("backend = %q\nimage = \"mirror.gcr.io/library/alpine:3.20\"\nrequire_worktree = \"off\"\nmemory = \"512M\"\ncpus = 1\nnetwork = { mode = %q }\n", backend, mode)
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte(toml), 0o644); err != nil {
		return fail(err)
	}
	e, err := box.Resolve(dir, "", scope.Identity{})
	if err != nil {
		return fail(err)
	}
	// Kept rather than discarded: a probe that fails has to say why, and the reason is usually in
	// what boxer printed on the way — an image that could not be cached, a pull that fell back.
	var said strings.Builder
	e.Stderr = &said
	defer func() { _ = e.Down() }()
	if _, err := e.Ensure(true, false); err != nil {
		if tail := strings.TrimSpace(said.String()); tail != "" {
			return fail(fmt.Errorf("%v (after: %s)", err, lastLines(tail, 2)))
		}
		return fail(err)
	}
	code, err := e.Run([]string{"sh", "-c", "echo ok > probe.txt"}, box.RunOpts{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil || code != 0 {
		return fail(firstErr(err, fmt.Errorf("the guest command exited %d", code)))
	}
	if b, err := os.ReadFile(filepath.Join(dir, "probe.txt")); err != nil || strings.TrimSpace(string(b)) != "ok" {
		return fail(errors.New("the guest wrote to the worktree and the host did not see it"))
	}
	return &probe{OK: true, Seconds: time.Since(start).Seconds()}
}

// lastLines is the final n lines of s, joined with " / ".
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " / ")
}

func firstErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
}

// integrationJSON is one row of `boxer integrations`.
type integrationJSON struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // harness | orchestrator | tool
	CLI     string   `json:"cli,omitempty"`
	Project string   `json:"project"` // installed | partial | no | n/a
	User    string   `json:"user"`
	Skill   string   `json:"skill"`
	Stale   []string `json:"stale,omitempty"`
	Fix     string   `json:"fix,omitempty"`
}

// harnessCLIs names each harness's program, for "is it installed here".
var harnessCLIs = map[string]string{
	"claude-code": "claude", "codex": "codex", "gemini-cli": "gemini", "grok": "grok", "kimi": "kimi",
	"dsh": "dsh", "opencode": "opencode", "pi": "pi", "copilot": "copilot",
}

// userHarnesses are the harnesses `boxer install <h> --user` supports.
var userHarnesses = map[string]bool{"claude-code": true, "codex": true, "copilot": true}

// integrationsCmd reports, for every harness boxer integrates with, whether its program is here
// and whether boxer's hooks, skill and MCP entry are installed for it — in this repository and for
// this user — and whether what is installed is out of date. `add` installs one.
func integrationsCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "add" {
		return integrationsAdd(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("integrations", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	out := cli.New(stdout, *asJSON)
	wt, repo := cwdRoot()
	root := firstNonBlank(repo, wt)
	cfg := config.Defaults()
	if root != "" {
		if c, err := config.Load(wt, repo); err == nil {
			cfg = c
		}
	}
	var rows []integrationJSON
	names := make([]string, 0, len(harnessCLIs))
	for h := range harnessCLIs {
		names = append(names, h)
	}
	sort.Strings(names)
	for _, h := range names {
		r := integrationJSON{Name: h, Kind: "harness", Project: "n/a", User: "n/a", Skill: "no"}
		if p, err := exec.LookPath(harnessCLIs[h]); err == nil {
			r.CLI = p
		}
		if root != "" {
			r.Project = installState(func(tmp string) (install.Result, error) { return install.Install(h, cfg, version(), tmp) }, root)
		}
		if userHarnesses[h] {
			home, _ := os.UserHomeDir()
			r.User = installState(func(tmp string) (install.Result, error) {
				return withHome(tmp, func() (install.Result, error) { return install.User(h, cfg, version()) })
			}, home)
		}
		r.Skill = skillState(root)
		if r.CLI != "" && r.Project != "installed" && r.User != "installed" {
			r.Fix = "boxer integrations add " + h
		}
		rows = append(rows, r)
	}
	if root != "" {
		stale := install.Drift(root, version())
		for i := range rows {
			if rows[i].Project == "installed" || rows[i].Project == "partial" {
				rows[i].Stale = stale
			}
		}
		rows = append(rows, orchestratorStates(root)...)
	}
	rows = append(rows, toolStates()...)
	if out.Mode == cli.JSON {
		emit(stdout, rows, true)
		return 0
	}
	var table [][]string
	for _, r := range rows {
		cliCell := "-"
		if r.CLI != "" {
			cliCell = out.Green("yes")
		} else if r.Kind != "orchestrator" {
			cliCell = out.Dim("no")
		}
		project := out.State(map[string]string{"installed": "yes", "partial": "partial", "no": "-", "n/a": "-"}[r.Project])
		if len(r.Stale) > 0 {
			project = out.Yellow("stale")
		}
		table = append(table, []string{r.Name, r.Kind, cliCell, project, out.State(map[string]string{"installed": "yes", "partial": "partial", "no": "-", "n/a": "-"}[r.User]), out.State(map[string]string{"yes": "yes", "no": "-"}[r.Skill])})
	}
	out.Table([]string{"INTEGRATION", "KIND", "ON PATH", "THIS REPO", "USER", "SKILL"}, table)
	if root == "" && out.Mode != cli.JSON {
		out.Printf("%s\n", out.Dim("not in a git repository: project installs not checked"))
	}
	var unwired []string
	for _, r := range rows {
		if r.Fix != "" {
			unwired = append(unwired, r.Name)
		}
	}
	for _, r := range rows {
		if len(r.Stale) > 0 {
			out.Hint("boxer install "+r.Name, "installed files are from another boxer release: "+strings.Join(r.Stale, ", "))
			break
		}
	}
	if len(unwired) > 0 {
		out.Hint("boxer integrations add <name>", "on PATH and not wired to boxer: "+strings.Join(unwired, ", "))
	}
	return 0
}

// installState dry-runs an install into a temporary root to learn which files it writes, then
// asks whether each of those is present, with boxer in it, under the real root. Asking the
// installer rather than keeping a second list is what stops this check and the installer from
// disagreeing about what "installed" means.
func installState(render func(tmp string) (install.Result, error), root string) string {
	tmp, err := os.MkdirTemp("", "boxer-integrations-")
	if err != nil {
		return "no"
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a temporary
	res, err := render(tmp)
	if err != nil || len(res.Written) == 0 {
		return "n/a"
	}
	have := 0
	for _, w := range res.Written {
		rel, err := filepath.Rel(tmp, w)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil && (strings.Contains(string(b), "boxer") || strings.Contains(rel, "boxer")) {
			have++
		}
	}
	switch {
	case have == 0:
		return "no"
	case have < len(res.Written):
		return "partial"
	}
	return "installed"
}

// withHome runs f with HOME pointed at dir, for the user-level installer's dry run.
func withHome(dir string, f func() (install.Result, error)) (install.Result, error) {
	old, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", dir)
	defer func() {
		if had {
			_ = os.Setenv("HOME", old)
		} else {
			_ = os.Unsetenv("HOME")
		}
	}()
	return f()
}

// skillState reports whether the boxer skill is where an agent would find it: in this repository,
// or installed for the user by `npx skills` or a harness's own skills directory.
func skillState(root string) string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{
		filepath.Join(root, ".agents", "skills", "boxer", "SKILL.md"),
		filepath.Join(root, ".claude", "skills", "boxer", "SKILL.md"),
		filepath.Join(home, ".agents", "skills", "boxer", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "boxer", "SKILL.md"),
	} {
		if root == "" && !strings.HasPrefix(p, home) {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return "yes"
		}
	}
	return "no"
}

// orchestratorStates reports the integrations that are a file in the repository rather than a
// harness: Conductor's settings, and the git hook that warms a sandbox for every new worktree.
func orchestratorStates(root string) []integrationJSON {
	has := func(p string) string {
		if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), "boxer") {
			return "installed"
		}
		return "no"
	}
	gitHook := filepath.Join(root, ".git", "hooks", "post-checkout")
	rows := []integrationJSON{
		{Name: "conductor", Kind: "orchestrator", Project: has(filepath.Join(root, ".conductor", "settings.toml")), User: "n/a", Skill: "no"},
		{Name: "git-hook", Kind: "orchestrator", Project: has(gitHook), User: "n/a", Skill: "no"},
	}
	for i := range rows {
		if rows[i].Project == "no" {
			rows[i].Fix = ""
		}
	}
	return rows
}

// toolStates are the host programs boxer's optional features need.
func toolStates() []integrationJSON {
	var out []integrationJSON
	for _, t := range []struct{ name, why string }{
		{"portless", "stable URLs for [urls]"},
		{"gh", "pull requests in boxer ls --pr"},
		{"npx", "boxer integrations add skill|plugin"},
	} {
		r := integrationJSON{Name: t.name, Kind: "tool", Project: "n/a", User: "n/a", Skill: "no"}
		if p, err := exec.LookPath(t.name); err == nil {
			r.CLI = p
		}
		out = append(out, r)
	}
	return out
}

// integrationsAdd installs one integration. A harness goes through boxer's own installer; the
// skill and the plugin go through the ecosystem tools people already use for everything else,
// `npx skills` and `npx plugins`, so boxer is installed and updated the same way as the rest.
func integrationsAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("integrations add", flag.ContinueOnError)
	user := fs.Bool("user", false, "install for this user rather than this repository")
	agent := fs.String("agent", "", "skill: the agents to install it for (npx skills -a); plugin: the target tool")
	source := fs.String("source", "BarakChamo/boxer", "where the skill and plugin are published")
	fs.SetOutput(stderr)
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: boxer integrations add <harness>|skill|plugin [--user] [--agent NAME]")
		return 2
	}
	// Flags before or after the name, as everywhere else: `add --user claude-code` read --user as
	// the integration.
	names, err := parseAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(names) != 1 {
		fmt.Fprintln(stderr, "usage: boxer integrations add <harness>|skill|plugin [--user] [--agent NAME]")
		return 2
	}
	what := names[0]
	var cmd *exec.Cmd
	switch what {
	case "skill":
		a := []string{"--yes", "skills", "add", *source, "--skill", "boxer", "-y"}
		if *agent != "" {
			a = append(a, "-a", *agent)
		}
		if *user {
			a = append(a, "-g")
		}
		cmd = exec.Command("npx", a...)
	case "plugin":
		a := []string{"--yes", "plugins", "add", *source, "-y"}
		if *agent != "" {
			a = append(a, "-t", *agent)
		}
		if !*user {
			a = append(a, "-s", "project")
		}
		cmd = exec.Command("npx", a...)
	default:
		if _, ok := harnessCLIs[what]; !ok && what != "all" && what != "git" && what != "conductor" {
			fmt.Fprintf(stderr, "boxer: unknown integration %q; harnesses: %s, or skill, plugin, git, conductor\n", what, strings.Join(sortedKeys(harnessCLIs), ", "))
			return 2
		}
		a := []string{"install", what}
		if *user {
			a = append(a, "--user")
		}
		return run(a, os.Stdin, stdout, stderr)
	}
	if _, err := exec.LookPath("npx"); err != nil {
		fmt.Fprintln(stderr, "boxer: npx is not on PATH; fix: install Node.js, or copy skills/boxer into .agents/skills yourself")
		return 1
	}
	fmt.Fprintf(stderr, "boxer: %s\n", strings.Join(cmd.Args, " "))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, "boxer:", err)
		return 1
	}
	return 0
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// urlCmd prints where this worktree's server is reachable — its name when [urls] gave it one, its
// forwarded host port otherwise — and `open` opens it. With a port, that one; without, the first.
func urlCmd(open bool, e *box.Env, args []string, stdout, stderr io.Writer) int {
	m, ok, err := e.Exists()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !ok {
		fmt.Fprintln(stderr, "boxer: no sandbox for this worktree\nboxer: fix: boxer up")
		return 4
	}
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "boxer url takes a guest port, not %q\n", args[0])
		return 2
	}
	if !m.Running() { // its address would answer nothing
		fmt.Fprintln(stderr, "boxer: this worktree's sandbox is stopped\nboxer: fix: boxer up")
		return 3
	}
	urls := e.URLs(m)
	for guest, host := range box.PortsOf(m) {
		if _, named := urls[guest]; !named {
			if urls == nil {
				urls = map[string]string{}
			}
			urls[guest] = "http://127.0.0.1:" + host
		}
	}
	if len(urls) == 0 {
		fmt.Fprintln(stderr, "boxer: this sandbox forwards no ports\nboxer: fix: add network.ports = [\"auto:3000\"] to boxer.toml, then boxer up --recreate")
		return 1
	}
	guest := sortedMapKeys(urls)[0]
	if len(args) > 0 {
		guest = args[0]
	}
	u, found := urls[guest]
	if !found {
		fmt.Fprintf(stderr, "boxer: guest port %s is not forwarded; forwarded: %s\n", guest, strings.Join(sortedMapKeys(urls), ", "))
		return 1
	}
	fmt.Fprintln(stdout, u)
	if open && cli.Detect(stdout, os.Getenv) == cli.Human {
		openURL(u)
	}
	return 0
}

// urlAllCmd prints, and with open opens, every running sandbox's servers on this backend: one line
// per server, the sandbox's name and then its address. Several agents in several worktrees each
// run a dev server, and finding them one worktree at a time was the chore this removes.
func urlAllCmd(open bool, stdout, stderr io.Writer) int {
	ms, err := vm.Owned(vm.Host(hostBackend()))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Name < ms[j].Name })
	human := cli.Detect(stdout, os.Getenv) == cli.Human
	n := 0
	for _, m := range ms {
		if !m.Running() {
			continue
		}
		urls := box.ReachableURLs(m)
		for _, guest := range sortedMapKeys(urls) {
			fmt.Fprintf(stdout, "%s\t%s\n", scope.Slug(m.Name), urls[guest])
			if open && human {
				openURL(urls[guest])
			}
			n++
		}
	}
	if n == 0 {
		fmt.Fprintln(stderr, "boxer: no running sandbox forwards a port")
		return 1
	}
	return 0
}

func openURL(u string) {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	_ = exec.Command(opener, u).Start()
}

// completionCmd prints a shell completion script. The command list is static: it is the set of
// subcommands, which changes with a release, not with a repository.
func completionCmd(args []string, stdout, stderr io.Writer) int {
	cmds := "up run down stop rm status restart ls gc doctor brief tasks backends integrations url open fork pack capsule logs watch shim hook mcp package install uninstall shell acp completion version"
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: boxer completion bash|zsh|fish")
		return 2
	}
	switch args[0] {
	case "bash":
		fmt.Fprintf(stdout, "complete -W %q boxer\n", cmds)
	case "zsh":
		fmt.Fprintf(stdout, "#compdef boxer\n_arguments '1:command:(%s)' '*::arg:_files'\n", cmds)
	case "fish":
		fmt.Fprintf(stdout, "complete -c boxer -f -n __fish_use_subcommand -a %q\n", cmds)
	default:
		fmt.Fprintln(stderr, "usage: boxer completion bash|zsh|fish")
		return 2
	}
	return 0
}
