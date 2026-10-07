// Package vm wraps the smolvm CLI. It is the only place that knows smolvm's flags, and it keeps
// no state of its own: labels on the machine are the record.
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LabelPrefix marks machines boxer owns.
const LabelPrefix = "boxer."

// InsideEnv is set in every guest process boxer starts. When boxer itself, or a harness that an
// orchestrator launched inside the guest, sees it, there is nothing further to sandbox: hooks go
// silent and `boxer run` executes directly. This is what stops two layers of boxer (an
// orchestrator's and the local harness's) from fighting over the same command.
const InsideEnv = "BOXER_INSIDE"

// Inside reports whether this process is already running in a boxer guest.
func Inside() bool { return os.Getenv(InsideEnv) != "" }

// Error, the sentinels and the predicates live in errors.go.

// TransportFailure reports whether output carries smolvm's own failure to reach the guest, rather
// than something the guest said. An exec that never reached the guest exits non-zero exactly like
// a command that ran and failed, so the text is the only signal there is: a probe that cannot tell
// the two apart reads "the marker is missing" from "I could not look".
func TransportFailure(output string) bool {
	return strings.Contains(output, "connection closed") || strings.Contains(output, "agent response frame")
}

// smolvmFailure returns smolvm's own error line from an exec's stderr, or "" when the failure was
// the guest command's. Only smolvm's wording counts: a guest that prints "Error: tests failed" has
// failed, and that is its exit code to report. Only the last line: smolvm reports its own failure
// and exits, so nothing follows it, and a guest's "Error: ...: connection closed" in the middle of
// a test run is the guest's.
func smolvmFailure(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if strings.HasPrefix(line, "Error: vm not found") || strings.HasPrefix(line, "Error: agent operation failed") ||
		strings.HasPrefix(line, "Error: machine") && strings.Contains(line, "is not running") ||
		strings.HasPrefix(line, "Error: ") && TransportFailure(line) {
		return line
	}
	return ""
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	max int
	b   []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > t.max {
		t.b = t.b[len(t.b)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.b) }

func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

// Client shells out to smolvm. Bin is looked up on PATH when it has no slash.
type Client struct {
	Bin string
	// Log receives the smolvm invocations when non-nil (doctor --verbose, tests).
	Log io.Writer
}

// New returns a client for the smolvm on PATH, or the one named by BOXER_SMOLVM.
func New() Client {
	bin := os.Getenv("BOXER_SMOLVM")
	if bin == "" {
		bin = "smolvm"
	}
	return Client{Bin: bin}
}

// Machine is the subset of `smolvm machine ls --json` boxer reads.
type Machine struct {
	Name      string            `json:"name"`
	State     string            `json:"state"`
	Image     string            `json:"image"`
	Labels    map[string]string `json:"labels"`
	CreatedAt int64             `json:"created_at"`
	// What smolvm allocated and where its process is. A sandbox costs real memory and real disk,
	// and "what is this costing me" is the first question anyone watching them asks.
	PID       int `json:"pid"`
	CPUs      int `json:"cpus"`
	MemoryMiB int `json:"memory_mib"`
	// Backend names the backend the machine is on. Only listings that span backends set it.
	Backend string `json:"backend,omitempty"`
}

// DataDir is where smolvm keeps a machine's disks. It is the machine's real cost on disk, and it
// is not derivable from anything else boxer knows, so it is asked for rather than guessed. An
// error means "unknown", which callers report as unmeasured rather than as zero.
func (c Client) DataDir(name string) (string, error) {
	out, err := c.output("machine", "data-dir", "--name", name)
	return strings.TrimSpace(out), err
}

// Running reports whether the machine can accept exec.
func (m Machine) Running() bool { return m.State == "running" }

// Version returns the smolvm version string, or an error when the binary is missing.
func (c Client) Version() (string, error) {
	out, err := c.output("--version")
	return strings.TrimSpace(out), err
}

// List returns every machine, boxer-owned or not.
func (c Client) List() ([]Machine, error) {
	out, err := c.output("machine", "ls", "--json")
	if err != nil {
		return nil, err
	}
	var ms []Machine
	if err := json.Unmarshal([]byte(out), &ms); err != nil {
		return nil, fmt.Errorf("smolvm machine ls --json: %w", err)
	}
	return ms, nil
}

// EgressEvent is one outbound connection the machine's egress policy refused. Every event the
// command returns is a denial — `machine egress-events` reports nothing else — so there is no
// allowed flag to check.
//
// The field names are smolvm 1.16.1's, confirmed against a real guest: a blocked name lookup is
// {"operation":"resolve","dest":"registry.npmjs.org"} and a blocked connection is
// {"operation":"connect","dest":"1.1.1.1:80"}.
type EgressEvent struct {
	Timestamp string `json:"timestamp"`
	Operation string `json:"operation"` // resolve | connect
	Dest      string `json:"dest"`      // a hostname, or host:port for a connection
}

// Host is the destination without its port, which is what a person adds to an allowlist.
func (e EgressEvent) Host() string {
	if i := strings.LastIndex(e.Dest, ":"); i > 0 && !strings.Contains(e.Dest[i+1:], ":") {
		return e.Dest[:i]
	}
	return e.Dest
}

// Egress returns the machine's recent egress denials. An allowlist one host short is invisible
// from inside the guest — the program reports a DNS or connect failure and nothing names the
// cause — so this is the only place the real reason is written down.
func (c Client) Egress(name string, limit int) ([]EgressEvent, error) {
	out, err := c.output("machine", "egress-events", "-n", name, "--limit", strconv.Itoa(limit), "--json")
	if err != nil {
		return nil, err
	}
	var evs []EgressEvent
	if err := json.Unmarshal([]byte(out), &evs); err != nil {
		return nil, fmt.Errorf("smolvm machine egress-events --json: %w", err)
	}
	return evs, nil
}

// Status returns the machine and whether it exists.
func (c Client) Status(name string) (Machine, bool, error) {
	out, err := c.output("machine", "status", "-n", name, "--json")
	if err != nil {
		if IsNotFound(err) {
			return Machine{}, false, nil
		}
		return Machine{}, false, err
	}
	var m Machine
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		return Machine{}, false, fmt.Errorf("smolvm machine status --json: %w", err)
	}
	return m, true, nil
}

// CreateSpec is what boxer needs to say about a new machine.
type CreateSpec struct {
	Name       string
	Image      string
	From       string // a packed .smolmachine of Image; wins over Image when set
	Smolfile   string
	Volumes    []string // host:guest
	Labels     map[string]string
	CPUs       int
	MemoryMiB  int
	Network    string // off | allowlist | on
	AllowHosts []string
	// AllowCIDRs are address blocks the guest may reach on every port, under allowlist.
	AllowCIDRs []string
	Ports      []string
	// DNS is the resolver the guest should use. Empty means boxer picks the host's (see dns.go);
	// "off" leaves smolvm's public-resolver default alone.
	DNS string
	// KeepID is "uid:gid": map the host user to that user in the guest. Only rootless podman needs
	// it and only podman honours it (--userns=keep-id); every other backend ignores it.
	KeepID string
}

// Create defines the machine. It does not start it.
// idleWorkload is the machine's persistent workload: it does nothing, forever.
const idleWorkload = "while :; do sleep 3600; done"

func (c Client) Create(s CreateSpec) error {
	args := []string{"machine", "create", "-n", s.Name}
	switch {
	case s.Smolfile != "":
		args = append(args, "--smolfile", s.Smolfile)
	case s.From != "":
		args = append(args, "--from", s.From)
	default:
		args = append(args, "-I", s.Image)
	}
	if s.CPUs > 0 {
		args = append(args, "--cpus", strconv.Itoa(s.CPUs))
	}
	if s.MemoryMiB > 0 {
		args = append(args, "--mem", strconv.Itoa(s.MemoryMiB))
	}
	for _, v := range s.Volumes {
		args = append(args, "-v", v)
	}
	for k, v := range s.Labels {
		args = append(args, "--label", k+"="+v)
	}
	for _, p := range s.Ports {
		args = append(args, "-p", p)
	}
	switch s.Network {
	case "on":
		args = append(args, "--net")
	case "allowlist":
		for _, h := range s.AllowHosts {
			args = append(args, "--allow-host", h)
		}
		for _, c := range s.AllowCIDRs {
			args = append(args, "--allow-cidr", c)
		}
	}
	// Point a networked guest at the host's caching resolver rather than smolvm's public default:
	// ~415ms per lookup becomes ~2ms, and 0ms once the host has it cached. See dns.go.
	if s.Network == "on" || s.Network == "allowlist" {
		if d := s.DNS; d != "off" {
			if d == "" {
				d = HostResolver()
			}
			if d != "" {
				args = append(args, "--dns", d)
			}
		}
	}
	// Wait out smolvm's store lock rather than failing. A create that lost the race did not
	// happen at all, so asking again is safe — there is no half-created machine to clean up. The
	// budget is generous because a real create takes about half a second and several arriving
	// together is the ordinary case for one-sandbox-per-worktree.
	// Without a workload smolvm launches the image's own ENTRYPOINT/CMD as the first container,
	// and execs join it until it exits: `node` or `python3` is a REPL that exits on its closed
	// stdin, a moment after boot, and takes with it everything run in it — the `start` services
	// and /tmp. On Linux that moment is after boxer has started them. A workload that never exits
	// gives every exec one container from the first second, which is what a sandbox is. A
	// Smolfile says for itself what runs.
	if s.Smolfile == "" {
		args = append(args, "--", "sh", "-c", idleWorkload)
	}
	_, err := c.retryWhileLocked(func() (string, error) { return c.output(args...) })
	return err
}

// retryWhileLocked calls f until it stops reporting smolvm's store as locked, backing off between
// attempts. Only for operations that are safe to repeat when they failed to take effect.
func (c Client) retryWhileLocked(f func() (string, error)) (string, error) {
	const attempts = 40 // with the backoff below, a little over 20 seconds
	wait := 25 * time.Millisecond
	var out string
	var err error
	for i := 0; i < attempts; i++ {
		if out, err = f(); !IsLocked(err) {
			return out, err
		}
		time.Sleep(wait)
		if wait < 750*time.Millisecond {
			wait *= 2
		}
	}
	return out, err
}

// Pack pulls image once into a .smolmachine at stub+".smolmachine" and returns that path. Machines
// created --from it boot from pre-extracted layers, so the pull happens once per image instead of
// once per VM. The executable stub smolvm also writes is removed; only the sidecar is kept.
func (c Client) Pack(image, stub string) (string, error) {
	return packAside(stub, func(tmp string) error {
		_, err := c.output("pack", "create", "-I", image, "-o", tmp, "--no-sign")
		return err
	})
}

// packAside has smolvm write a pack in a hidden directory beside stub and moves it into place only
// once whole. smolvm writes in place, and other worktrees boot from any pack that exists: one
// still being written was booted half-made, failed as corrupt, and was deleted under its writer.
func packAside(stub string, pack func(tmp string) error) (string, error) {
	dir, err := os.MkdirTemp(filepath.Dir(stub), ".packing-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // best effort; gc sweeps what a killed pack leaves
	tmp := filepath.Join(dir, filepath.Base(stub))
	if err := pack(tmp); err != nil {
		return "", err
	}
	out := stub + ".smolmachine"
	return out, os.Rename(tmp+".smolmachine", out)
}

// PackFromVM snapshots a stopped machine's root filesystem into stub+".smolmachine": whatever
// was installed in it boots pre-installed in machines created --from the pack.
func (c Client) PackFromVM(name, stub string) (string, error) {
	return packAside(stub, func(tmp string) error {
		_, err := c.output("pack", "create", "--from-vm", name, "-o", tmp, "--no-sign")
		return err
	})
}

// Start boots a defined machine.
func (c Client) Start(name string) error {
	_, err := c.output("machine", "start", "-n", name)
	return err
}

// StartBranchable boots a machine as a branch source: its RAM is backed by a memfd, so it can be
// copied on write, and it exposes the control socket `machine branch` needs. An ordinary start
// cannot be branched from, and smolvm reports no way to ask whether the current boot was
// branchable, so the caller has to remember.
func (c Client) StartBranchable(name string) error {
	_, err := c.output("machine", "start", "-n", name, "--branchable")
	return err
}

// BranchSpec is one fork of a running branchable machine. A child inherits the source's volumes
// and cannot override them, which is why the worktree mount has to be decided on the parent.
type BranchSpec struct {
	From       string
	NamePrefix string
	Count      int
	Env        []string // KEY=VALUE, per child
	SecretEnv  []string // GUEST=HOSTVAR
	Ports      []string
}

// Branch forks the source and returns the children's names.
//
// Each child is taken as its own single `--name` branch, never as a `--count`/`--name-prefix`
// batch, because a batch waits for the source's workload to declare a branch point by running
// `smolvm-branch-ready` and gives up after ten minutes when it never does. An ordinary
// development sandbox has no such workload, so a batch would hang every time; a single named
// branch checkpoints the source wherever it happens to be, which is what boxer wants.
func (c Client) Branch(s BranchSpec) ([]string, error) {
	if s.Count <= 0 {
		s.Count = 1
	}
	var out []string
	for i := 1; i <= s.Count; i++ {
		name := fmt.Sprintf("%s%d", s.NamePrefix, i)
		if _, ok, err := c.Status(name); err == nil && ok {
			continue // a child of that number is already there
		}
		args := []string{"machine", "branch", "--from", s.From, "-n", name}
		for _, e := range s.Env {
			args = append(args, "-e", e)
		}
		for _, e := range s.SecretEnv {
			args = append(args, "--secret-env", e)
		}
		for _, p := range s.Ports {
			args = append(args, "-p", p)
		}
		if _, err := c.output(args...); err != nil {
			// Whatever was made before the failure is real and has to be reported, or the caller
			// cannot reclaim it.
			return out, err
		}
		out = append(out, name)
	}
	return out, nil
}

// Stop halts a machine; stopping a stopped machine is not an error.
func (c Client) Stop(name string) error {
	_, err := c.output("machine", "stop", "-n", name)
	if err != nil && IsNotRunning(err) {
		return nil
	}
	return err
}

// Delete removes a machine and any children branched from it.
func (c Client) Delete(name string) error {
	_, err := c.output("machine", "delete", "-n", name, "--force", "--cascade")
	// Absence is not an error (the Backend contract): a fork child deleted by its parent's
	// cascade is gone by the time a bulk delete reaches it, and that is success.
	if IsNotFound(err) {
		return nil
	}
	return err
}

// ExecOpts controls one command inside the guest.
type ExecOpts struct {
	Name    string
	Workdir string
	Env     []string // KEY=VALUE
	// SecretEnv is GUEST=HOSTVAR: smolvm reads the value out of its own environment, so a
	// forwarded token never appears in the argv that `ps` shows on the host.
	SecretEnv []string
	// Timeout bounds the guest command. Zero leaves it unbounded, which is what a shell does.
	Timeout time.Duration
	TTY     bool
	// User runs the command as this guest user (a name or uid[:gid]); empty is the image's own.
	// Every backend spells it `-u`, which is why it is one field rather than a capability.
	User   string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Exec runs argv in the guest and returns its exit status. stdin is always kept open so piped
// input works (R-CMD-5); a TTY is requested only when asked (R-CMD-7).
func (c Client) Exec(o ExecOpts, argv ...string) (int, error) {
	// InsideEnv lets a boxer (or a harness) running inside the guest know not to sandbox again.
	args := []string{"machine", "exec", "--name", o.Name, "-i", "-e", InsideEnv + "=1"}
	if o.TTY {
		args = append(args, "-t")
	}
	if o.User != "" {
		args = append(args, "-u", o.User)
	}
	if o.Workdir != "" {
		args = append(args, "-w", o.Workdir)
	}
	for _, e := range o.Env {
		args = append(args, "-e", e)
	}
	for _, s := range o.SecretEnv {
		args = append(args, "--secret-env", s)
	}
	if o.Timeout > 0 {
		args = append(args, "--timeout", o.Timeout.String())
	}
	args = append(args, "--")
	args = append(args, argv...)
	c.log(args)
	cmd := c.command(args...)
	// smolvm's own failures exit non-zero exactly as a guest command can, and print to the same
	// stderr. The tail is kept so the two can be told apart: a code that came from smolvm is not
	// the command's, and recording it as one fabricated a run that never happened.
	tail := &tailBuffer{max: 4096}
	cmd.Stdin, cmd.Stdout = o.Stdin, o.Stdout
	cmd.Stderr = io.MultiWriter(writerOrDiscard(o.Stderr), tail)
	if o.Stdout != nil && o.Stdout == o.Stderr {
		// One combined stream stays one stream: os/exec shares a single pipe only when the two
		// writers are the same value, and that is what keeps their output in order.
		cmd.Stdout = cmd.Stderr
	}
	if err := cmd.Start(); err != nil {
		return 127, c.wrap(err)
	}
	// Forward termination signals to smolvm so the guest process ends with us (R-CMD-6).
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case s := <-sigs:
			_ = cmd.Process.Signal(s)
		case err := <-done:
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() < 0 {
				return 1, killedCLI(cmd.Path, ee)
			}
			if errors.As(err, &ee) {
				if own := smolvmFailure(tail.String()); own != "" {
					return ee.ExitCode(), &Error{Backend: "smolvm", Verb: "machine", Code: ee.ExitCode(), Stderr: own, Kind: classifySmolvm(own)}
				}
				return ee.ExitCode(), nil
			}
			if err != nil {
				return 1, err
			}
			return 0, nil
		}
	}
}

func (c Client) output(args ...string) (string, error) {
	c.log(args)
	cmd := c.command(args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		if msg == "" {
			if code == -1 {
				return "", c.wrap(err) // smolvm never ran: a missing binary, not a refusal
			}
			msg = err.Error()
		}
		return "", &Error{Verb: args[0], Code: code, Stderr: msg, Kind: classifySmolvm(msg)}
	}
	return out.String(), nil
}

func (c Client) wrap(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("smolvm not found on PATH; install: curl -sSL https://smolmachines.com/install.sh | bash")
	}
	return err
}

func (c Client) log(args []string) {
	if c.Log != nil {
		fmt.Fprintf(c.Log, "smolvm %s\n", strings.Join(args, " "))
	}
}
