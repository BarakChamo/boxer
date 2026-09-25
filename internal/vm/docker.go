package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Docker drives an OCI daemon — `docker`, or anything that speaks its CLI. It is boxer's second
// backend and the one that makes the first one honest: everything smolvm-shaped that leaked into
// the rest of boxer had to become a capability before this file could exist.
//
// It is a weaker boundary than smolvm and says so. Containers share one kernel — the host's on
// Linux, the daemon's single Linux VM on macOS — so an escape reaches every other sandbox rather
// than one. boxer's whole claim is that the agent cannot get out, so `boxer doctor` prints which
// of the two you got rather than leaving it to be inferred from the backend's name.
//
// Bin is the binary: "docker", or "podman", which is argv-compatible for every verb here.
type Docker struct {
	Bin string
	Log interface{ Write([]byte) (int, error) }
}

// NewDocker returns a driver for the named binary, defaulting to docker.
func NewDocker(bin string) Docker {
	if bin == "" {
		bin = "docker"
	}
	return Docker{Bin: bin}
}

var (
	_ Backend  = Docker{}
	_ Declarer = Docker{}
)

func (d Docker) Name() string { return d.Bin }

// Declare states what no method set can. Allowlist is the consequential one: see Create.
func (d Docker) Declare() Caps {
	return Caps{
		Boundary:   "namespace",
		HostMounts: true,
		// The value of a secret never enters the host argument vector — it goes through a 0600
		// file passed as --env-file, which is the property `--secret-env` exists to provide.
		SecretEnv:       true,
		Allowlist:       false,
		ProvesOwnership: true, // docker carries and reports labels
	}
}

func (d Docker) Version() (string, error) {
	out, err := d.output("version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "", err
	}
	return d.Bin + " " + strings.TrimSpace(out), nil
}

// dockerInspect is the part of `docker inspect` boxer reads.
type dockerInspect struct {
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status string `json:"Status"`
		Pid    int    `json:"Pid"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		NanoCpus int64 `json:"NanoCpus"`
		Memory   int64 `json:"Memory"`
	} `json:"HostConfig"`
}

func (in dockerInspect) machine() Machine {
	m := Machine{
		Name:      strings.TrimPrefix(in.Name, "/"),
		State:     in.State.Status,
		Image:     in.Config.Image,
		Labels:    in.Config.Labels,
		PID:       in.State.Pid,
		MemoryMiB: int(in.HostConfig.Memory / (1 << 20)),
	}
	// A container with no CPU limit reports 0, which is "every core", not "no cores". Reporting
	// the zero through would have `boxer ls` print `cpus: 0` for a sandbox using the whole
	// machine, so it stays unset and reads as unmeasured — which is what it is.
	if in.HostConfig.NanoCpus > 0 {
		m.CPUs = int(in.HostConfig.NanoCpus / 1e9)
	}
	// `exited` and `created` both mean "not running"; boxer only ever asks which.
	if m.State == "exited" {
		m.State = "stopped"
	}
	if t, err := time.Parse(time.RFC3339Nano, in.Created); err == nil {
		m.CreatedAt = t.Unix()
	}
	return m
}

// List returns every container, boxer's or not. Ownership is decided by label, in Owned.
//
// Two calls rather than one: `ps --format` flattens labels into a comma-joined string, and a
// boxer label carries a filesystem path, which may contain a comma. Parsing that back is a bug
// waiting for a worktree with an awkward name, so the names come from `ps` and the real values
// from `inspect`, which emits them as JSON.
func (d Docker) List() ([]Machine, error) {
	out, err := d.output("ps", "-a", "--format", "{{.Names}}")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	if len(names) == 0 {
		return nil, nil
	}
	return d.inspect(names...)
}

func (d Docker) inspect(names ...string) ([]Machine, error) {
	out, err := d.output(append([]string{"inspect"}, names...)...)
	if err != nil {
		return nil, err
	}
	var raw []dockerInspect
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("%s inspect: %w", d.Bin, err)
	}
	ms := make([]Machine, 0, len(raw))
	for _, in := range raw {
		ms = append(ms, in.machine())
	}
	return ms, nil
}

func (d Docker) Status(name string) (Machine, bool, error) {
	ms, err := d.inspect(name)
	if err != nil {
		if IsNotFound(err) {
			return Machine{}, false, nil
		}
		return Machine{}, false, err
	}
	if len(ms) == 0 {
		return Machine{}, false, nil
	}
	return ms[0], true, nil
}

// Create makes the container. It does not start it; Start does, exactly as smolvm splits them.
//
// The container needs a process that outlives the create so `exec` has something to enter, and it
// must not be the image's own entrypoint: boxer runs the `start` commands itself, so an image
// whose CMD is a server would start it twice. A sleeping shell is the smallest thing that keeps
// the namespace alive and does nothing else.
func (d Docker) Create(s CreateSpec) error {
	if s.Smolfile != "" {
		return Unsupported(d, "build from a Smolfile", "set `image` to an OCI reference instead")
	}
	if s.From != "" {
		return Unsupported(d, "start from an environment pack", "packs are a smolvm artefact")
	}
	args := []string{"create", "--name", s.Name}
	if s.CPUs > 0 {
		args = append(args, "--cpus", strconv.Itoa(s.CPUs))
	}
	if s.MemoryMiB > 0 {
		args = append(args, "-m", strconv.Itoa(s.MemoryMiB)+"m")
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
	case "off":
		args = append(args, "--network", "none")
	case "allowlist":
		// Refused rather than approximated. An OCI daemon has "no network" and "the whole
		// internet" and nothing between, so honouring this would mean running with egress boxer
		// promised to deny — a sandbox that reads as enforced and is not. That is the exact
		// failure the contract's third rule exists to prevent, and it is worse than not starting.
		return Unsupported(d, `enforce network.mode = "allowlist"`,
			`use "off" or "on", or set backend = "smolvm" to keep the allowlist`)
	}
	args = append(args, "--entrypoint", "sh", s.Image, "-c", "while :; do sleep 3600; done")
	_, err := d.output(args...)
	return err
}

// Start starts the container and waits until it really is running.
//
// `docker start` returns once the daemon has accepted the start, not once the container's process
// is up, so an `exec` issued immediately after can be refused with "is not running". boxer's very
// first act after provisioning is an exec, which made this a race it lost regularly: the command
// never ran, `Exec` returned an error rather than an exit code, and the run was recorded as exit 1
// — a command that exits 4 reported as 1. It was invisible from the outside because every attempt
// to observe it added the delay that made it pass.
//
// The wait is bounded and short. Polling `Status` is the same question `exec` is about to ask, so
// there is no cleverness here, only asking first.
func (d Docker) Start(name string) error {
	if _, err := d.output("start", name); err != nil {
		return err
	}
	for i := 0; i < 100; i++ { // ~2s, far longer than the window has ever been observed to be
		m, ok, err := d.Status(name)
		if err != nil {
			return err
		}
		if ok && m.Running() {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Report it rather than pressing on: an exec against a container that never came up fails
	// in a way that reads like the command failing, which is the confusion this exists to end.
	return &Error{Backend: d.Bin, Verb: "start", Code: -1,
		Stderr: name + " did not reach the running state", Kind: ErrNotRunning}
}

// Stop is not an error when the container is already stopped.
func (d Docker) Stop(name string) error {
	_, err := d.output("stop", name)
	if IsNotRunning(err) {
		return nil
	}
	return err
}

// Delete tolerates an already-deleted container, as the contract requires.
func (d Docker) Delete(name string) error {
	_, err := d.output("rm", "-f", name)
	if IsNotFound(err) {
		return nil
	}
	return err
}

func (d Docker) Exec(o ExecOpts, argv ...string) (int, error) {
	args := []string{"exec", "-i", "-e", InsideEnv + "=1"}
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
	// A secret's value must not reach the argument vector, where `ps` would show it to every
	// process on the host. There is no `--secret-env` here, so the values go through a file only
	// this process can read, which is deleted as soon as the command returns.
	if len(o.SecretEnv) > 0 {
		f, err := secretFile(o.SecretEnv)
		if err != nil {
			return 127, err
		}
		defer func() { _ = os.Remove(f) }()
		args = append(args, "--env-file", f)
	}
	args = append(args, o.Name)
	args = append(args, argv...)
	d.log(args)

	ctx := context.Background()
	cancel := context.CancelFunc(func() {})
	if o.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, d.Bin, args...)
	head := &headWriter{w: o.Stderr}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = o.Stdin, o.Stdout, head
	if err := cmd.Start(); err != nil {
		return 127, err
	}
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
			if asExitError(err, &ee) {
				if rerr := runtimeFailure(d, ee.ExitCode(), head, map[int][]string{
					1:   {"Error response from daemon:"},
					125: {"Error: "},
					// The runtime could not start the process at all — a missing working directory,
					// a mount not yet visible. 126 and 127 are also a shell's "cannot execute" and
					// "not found", which is exactly why the prefix, not the status, decides.
					126: {"OCI runtime exec failed", "Error: "},
					127: {"OCI runtime exec failed", "Error: "},
				}, classifyDocker); rerr != nil {
					return ee.ExitCode(), rerr
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

// secretFile writes GUEST=<value-from-host> pairs to a file only this user can read.
func secretFile(pairs []string) (string, error) {
	f, err := os.CreateTemp("", "boxer-secret-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if err := f.Chmod(0o600); err != nil {
		return f.Name(), err
	}
	var b strings.Builder
	for _, p := range pairs {
		guest, host, ok := strings.Cut(p, "=")
		if !ok {
			guest, host = p, p
		}
		if v, set := os.LookupEnv(host); set {
			fmt.Fprintf(&b, "%s=%s\n", guest, v)
		}
	}
	_, err = f.WriteString(b.String())
	return f.Name(), err
}

// classifyDocker maps the daemon's wording onto the sentinels. Docker's own, not smolvm's — which
// is the entire reason the sentinels exist.
func classifyDocker(stderr string) error {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "no such container"), strings.Contains(s, "no such object"):
		return ErrNotFound
	case strings.Contains(s, "is already in use"), strings.Contains(s, "already exists"):
		return ErrAlreadyExists
	case strings.Contains(s, "is not running"), strings.Contains(s, "not running"):
		return ErrNotRunning
	}
	return nil
}

func (d Docker) output(args ...string) (string, error) {
	d.log(args)
	cmd := exec.Command(d.Bin, args...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		code := -1
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			code = ee.ExitCode()
		}
		if msg == "" {
			if code == -1 {
				return "", fmt.Errorf("%s not found on PATH", d.Bin)
			}
			msg = err.Error()
		}
		return "", &Error{Backend: d.Bin, Verb: args[0], Code: code, Stderr: msg, Kind: classifyDocker(msg)}
	}
	return out.String(), nil
}

func (d Docker) log(args []string) {
	if d.Log != nil {
		fmt.Fprintf(d.Log, "%s %s\n", d.Bin, strings.Join(args, " "))
	}
}
