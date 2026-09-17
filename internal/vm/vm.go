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
	"strconv"
	"strings"
	"syscall"
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
	Name       string            `json:"name"`
	State      string            `json:"state"`
	Image      string            `json:"image"`
	Labels     map[string]string `json:"labels"`
	CreatedAt  int64             `json:"created_at"`
	PID        *int              `json:"pid"`
	Branchable bool              `json:"branchable"`
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

// Owned returns only machines carrying a boxer label.
func (c Client) Owned() ([]Machine, error) {
	all, err := c.List()
	if err != nil {
		return nil, err
	}
	var out []Machine
	for _, m := range all {
		if _, ok := m.Labels[LabelPrefix+"scope"]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// Status returns the machine and whether it exists.
func (c Client) Status(name string) (Machine, bool, error) {
	out, err := c.output("machine", "status", "-n", name, "--json")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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
	Ports      []string
}

// Create defines the machine. It does not start it.
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
	}
	_, err := c.output(args...)
	return err
}

// Pack pulls image once into a .smolmachine at stub+".smolmachine" and returns that path. Machines
// created --from it boot from pre-extracted layers, so the pull happens once per image instead of
// once per VM. The executable stub smolvm also writes is removed; only the sidecar is kept.
func (c Client) Pack(image, stub string) (string, error) {
	if _, err := c.output("pack", "create", "-I", image, "-o", stub, "--no-sign"); err != nil {
		return "", err
	}
	os.Remove(stub)
	return stub + ".smolmachine", nil
}

// PackFromVM snapshots a stopped machine's root filesystem into stub+".smolmachine": whatever
// was installed in it boots pre-installed in machines created --from the pack.
func (c Client) PackFromVM(name, stub string) (string, error) {
	if _, err := c.output("pack", "create", "--from-vm", name, "-o", stub, "--no-sign"); err != nil {
		return "", err
	}
	os.Remove(stub)
	return stub + ".smolmachine", nil
}

// Start boots a defined machine.
func (c Client) Start(name string, branchable bool) error {
	args := []string{"machine", "start", "-n", name}
	if branchable {
		args = append(args, "--branchable")
	}
	_, err := c.output(args...)
	return err
}

// Stop halts a machine; stopping a stopped machine is not an error.
func (c Client) Stop(name string) error {
	_, err := c.output("machine", "stop", "-n", name)
	if err != nil && strings.Contains(err.Error(), "not running") {
		return nil
	}
	return err
}

// Delete removes a machine and any children branched from it.
func (c Client) Delete(name string) error {
	_, err := c.output("machine", "delete", "-n", name, "--force", "--cascade")
	return err
}

// Branch forks a running branchable machine.
func (c Client) Branch(from, name string) error {
	_, err := c.output("machine", "branch", "--from", from, "-n", name)
	return err
}

// ExecOpts controls one command inside the guest.
type ExecOpts struct {
	Name    string
	Workdir string
	Env     []string // KEY=VALUE
	TTY     bool
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

// Exec runs argv in the guest and returns its exit status. stdin is always kept open so piped
// input works (R-CMD-5); a TTY is requested only when asked (R-CMD-7).
func (c Client) Exec(o ExecOpts, argv ...string) (int, error) {
	// InsideEnv lets a boxer (or a harness) running inside the guest know not to sandbox again.
	args := []string{"machine", "exec", "--name", o.Name, "-i", "-e", InsideEnv + "=1"}
	if o.TTY {
		args = append(args, "-t")
	}
	if o.Workdir != "" {
		args = append(args, "-w", o.Workdir)
	}
	for _, e := range o.Env {
		args = append(args, "-e", e)
	}
	args = append(args, "--")
	args = append(args, argv...)
	c.log(args)
	cmd := exec.Command(c.Bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = o.Stdin, o.Stdout, o.Stderr
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
			if errors.As(err, &ee) {
				return ee.ExitCode(), nil
			}
			if err != nil {
				return 1, err
			}
			return 0, nil
		}
	}
}

// Output runs argv in the guest and returns its combined output and exit status.
func (c Client) Output(name, workdir string, argv ...string) (string, int, error) {
	var buf bytes.Buffer
	code, err := c.Exec(ExecOpts{Name: name, Workdir: workdir, Stdin: strings.NewReader(""), Stdout: &buf, Stderr: &buf}, argv...)
	return buf.String(), code, err
}

func (c Client) output(args ...string) (string, error) {
	c.log(args)
	cmd := exec.Command(c.Bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			return "", c.wrap(err)
		}
		return "", fmt.Errorf("smolvm %s: %s", args[0], msg)
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
