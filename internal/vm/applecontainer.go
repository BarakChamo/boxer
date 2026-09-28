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

// Apple represents Apple's `container` (github.com/apple/container): one lightweight VM per
// container, on Virtualization.framework, consuming ordinary OCI images.
//
// This is the backend that proves the capability model is not just "smolvm and not-smolvm". It
// has smolvm's boundary — a kernel per sandbox — and docker's inputs, so it lands in neither of
// the two buckets the interface was first written against, and everything it needed was already
// there: `MountAtHostPath` false, no `Packer` (there is no commit verb), no allowlist.
//
// Its dialect is its own, not docker's: nouns are subcommand groups (`container list`,
// `container delete`) and there is no `ps`/`rm` shorthand. macOS 26 and Apple Silicon only.
type Apple struct {
	Bin string
	Log interface{ Write([]byte) (int, error) }
}

// NewApple returns a driver for Apple's container CLI.
func NewApple() Apple { return Apple{Bin: "container"} }

var (
	_ Backend  = Apple{}
	_ Declarer = Apple{}
)

func (a Apple) Name() string { return "container" }

// Declare: a kernel each, like smolvm, but no per-host egress policy — `container network create`
// offers `--internal`, which is all-or-nothing isolation rather than a list of hosts. So the
// allowlist is refused here exactly as it is on docker, and for the same reason.
func (a Apple) Declare() Caps {
	return Caps{
		Boundary:        "kernel",
		HostMounts:      true,
		Allowlist:       false,
		SecretEnv:       true, // --env-file keeps values out of argv
		ProvesOwnership: true, // -l/--label, read back by inspect
	}
}

func (a Apple) Version() (string, error) {
	out, err := a.output("--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// appleInspect is the part of `container inspect` boxer reads. The shape is confirmed against
// 1.4.1 rather than assumed; anything absent simply reads as unmeasured.
type appleInspect struct {
	// `status` is an object, not a string — `{"state":"running","networks":[...]}`. Read from a
	// real 1.4.1 inspect rather than assumed: a string here would have unmarshalled to empty and
	// reported every container as stopped, which boxer would have answered by starting a running
	// container again.
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Configuration struct {
		ID     string            `json:"id"`
		Labels map[string]string `json:"labels"`
		Image  struct {
			Reference string `json:"reference"`
		} `json:"image"`
		Resources struct {
			CPUs          int   `json:"cpus"`
			MemoryInBytes int64 `json:"memoryInBytes"`
		} `json:"resources"`
	} `json:"configuration"`
}

func (in appleInspect) machine() Machine {
	m := Machine{
		Name:      in.Configuration.ID,
		State:     in.Status.State,
		Image:     in.Configuration.Image.Reference,
		Labels:    in.Configuration.Labels,
		CPUs:      in.Configuration.Resources.CPUs,
		MemoryMiB: int(in.Configuration.Resources.MemoryInBytes / (1 << 20)),
	}
	// `stopped` is this CLI's word for it; boxer only ever asks whether it is running.
	if m.State != "running" && m.State != "" {
		m.State = "stopped"
	}
	return m
}

func (a Apple) List() ([]Machine, error) {
	out, err := a.output("list", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}
	return a.decode(out)
}

// decode is exported to tests as DecodeAppleForTest.
func (a Apple) decode(out string) ([]Machine, error) {
	out = strings.TrimSpace(out)
	if out == "" || out == "null" {
		return nil, nil
	}
	var raw []appleInspect
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}
	ms := make([]Machine, 0, len(raw))
	for _, in := range raw {
		ms = append(ms, in.machine())
	}
	return ms, nil
}

func (a Apple) Status(name string) (Machine, bool, error) {
	out, err := a.output("inspect", name)
	if err != nil {
		if IsNotFound(err) {
			return Machine{}, false, nil
		}
		return Machine{}, false, err
	}
	ms, err := a.decode(out)
	if err != nil || len(ms) == 0 {
		return Machine{}, false, err
	}
	return ms[0], true, nil
}

func (a Apple) Create(s CreateSpec) error {
	if s.Smolfile != "" {
		return Unsupported(a, "build from a Smolfile", "set `image` to an OCI reference instead")
	}
	if s.From != "" {
		return Unsupported(a, "start from an environment pack", "packs are a smolvm artefact")
	}
	if s.Network == "allowlist" {
		// Same refusal as docker, same reason: `container network create --internal` is
		// all-or-nothing isolation, not a list of hosts, so honouring this would mean running
		// with egress boxer promised to deny.
		return Unsupported(a, `enforce network.mode = "allowlist"`,
			`use "off" or "on", or set backend = "smolvm" to keep the allowlist`)
	}
	args := []string{"create", "--name", s.Name}
	if s.CPUs > 0 {
		args = append(args, "-c", strconv.Itoa(s.CPUs))
	}
	if s.MemoryMiB > 0 {
		args = append(args, "-m", strconv.Itoa(s.MemoryMiB)+"M")
	}
	for _, v := range s.Volumes {
		args = append(args, "-v", v)
	}
	for k, v := range s.Labels {
		args = append(args, "-l", k+"="+v)
	}
	// Without this a sandbox runs, serves, and is unreachable: this runtime gives each container
	// its own address on a private network, and that address is not routable from the host. A
	// dev server with no published port is the failure that looks most like success — the
	// container is healthy and nothing answers.
	for _, p := range s.Ports {
		args = append(args, "-p", loopbackPort(p))
	}
	if s.Network == "off" {
		args = append(args, "--no-dns")
	}
	// The same sleeping shell docker gets, and for the same reason: boxer runs the `start` list
	// itself, so the image's own entrypoint must not also run.
	args = append(args, s.Image, "sh", "-c", "while :; do sleep 3600; done")
	_, err := a.output(args...)
	return err
}

// Start starts the container and waits until it is really running, for the reason documented on
// Docker.Start: the first thing boxer does after provisioning is an exec, and losing that race
// records the wrong exit code for the user's command.
func (a Apple) Start(name string) error {
	if _, err := a.output("start", name); err != nil {
		return err
	}
	for i := 0; i < 150; i++ { // a microVM boots slower than a container; ~3s
		m, ok, err := a.Status(name)
		if err != nil {
			return err
		}
		if ok && m.Running() {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return &Error{Backend: a.Name(), Verb: "start", Code: -1,
		Stderr: name + " did not reach the running state", Kind: ErrNotRunning}
}

func (a Apple) Stop(name string) error {
	_, err := a.output("stop", name)
	if IsNotRunning(err) || IsNotFound(err) {
		return nil
	}
	return err
}

func (a Apple) Delete(name string) error {
	_, err := a.output("delete", "--force", name)
	if IsNotFound(err) {
		return nil
	}
	return err
}

func (a Apple) Exec(o ExecOpts, argv ...string) (int, error) {
	// -i for the same reason docker gets it: without it stdin is closed, and `boxer run -- wc -l`
	// reads nothing from a pipe.
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
	a.log(args)

	ctx := context.Background()
	cancel := context.CancelFunc(func() {})
	if o.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, a.Bin, args...)
	// A timeout kills the CLI, but a process it left holding stdout would keep Wait blocked until
	// that process exited; WaitDelay bounds the wait so the timeout actually returns.
	cmd.WaitDelay = 2 * time.Second
	stdout, head := streams(o.Stdout, o.Stderr)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = o.Stdin, stdout, head
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
				if rerr := runtimeFailure(a, ee.ExitCode(), head, map[int][]string{1: {"Error: "}}, classifyApple); rerr != nil {
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

// classifyApple maps this CLI's wording onto the sentinels.
func classifyApple(stderr string) error {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "not found"), strings.Contains(s, "no such container"):
		return ErrNotFound
	case strings.Contains(s, "already exists"), strings.Contains(s, "in use"):
		return ErrAlreadyExists
	case strings.Contains(s, "not running"), strings.Contains(s, "is stopped"):
		return ErrNotRunning
	}
	return nil
}

func (a Apple) output(args ...string) (string, error) {
	a.log(args)
	cmd := exec.Command(a.Bin, args...)
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
				return "", fmt.Errorf("%s not found on PATH", a.Bin)
			}
			msg = err.Error()
		}
		return "", &Error{Backend: a.Name(), Verb: args[0], Code: code, Stderr: msg, Kind: classifyApple(msg)}
	}
	return out.String(), nil
}

func (a Apple) log(args []string) {
	if a.Log != nil {
		fmt.Fprintf(a.Log, "%s %s\n", a.Bin, strings.Join(args, " "))
	}
}
