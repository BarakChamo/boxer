package vm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backend is what boxer needs from anything that can host a sandbox. It is deliberately small:
// nine methods, one per rule the contract in docs/adding-a-backend.md actually demands, and
// nothing that only one implementation can do.
//
// Everything smolvm can do and a container runtime cannot — snapshotting a prepared filesystem,
// forking a running machine, reporting egress denials, pricing a machine's disk — lives in the
// optional interfaces below, discovered by type assertion. That is not a stylistic choice. A fat
// interface would make every backend declare `Branch` and return "unsupported" from a stub, which
// means the compiler stops helping: a backend that *forgets* to refuse still compiles, and the
// user gets a command that appears to work without the property they asked for. With optional
// interfaces, "cannot do it" is a fact the type system holds.
type Backend interface {
	// Name is the backend's identity, used in refusals and in `boxer doctor`.
	Name() string
	Version() (string, error)

	// List returns every machine the backend has, owned by boxer or not. Owned() filters it.
	List() ([]Machine, error)
	Status(name string) (Machine, bool, error)

	// Create must be safe to race: two boxers provisioning one scope must not produce two
	// machines, and the loser must return ErrAlreadyExists rather than fail the user's command.
	Create(CreateSpec) error
	Start(name string) error
	// Stop on a stopped machine, and Delete on a missing one, are not errors.
	Stop(name string) error
	Delete(name string) error

	// Exec runs argv in the guest and returns its exit code. The exit code is mandatory:
	// backend success is not command success, and a backend that cannot report the status of the
	// command boxer asked for has failed, never passed. Output streams as it is produced.
	Exec(o ExecOpts, argv ...string) (int, error)
}

// Packer caches a prepared filesystem so a second worktree does not repeat `image_setup`. The ref
// is opaque to boxer: a .smolmachine path for smolvm, an image tag for anything speaking OCI.
// CreateSpec.From carries it back.
type Packer interface {
	Pack(image, stub string) (ref string, err error)
	PackFromVM(name, stub string) (ref string, err error)
}

// Brancher forks a running machine copy-on-write. Nothing outside a hypervisor can do this.
type Brancher interface {
	StartBranchable(name string) error
	Branch(BranchSpec) ([]string, error)
}

// EgressReporter says what the machine's network policy refused. An allowlist one host short is
// invisible from inside the guest — the program sees a DNS or connect failure with no policy in
// it — so this is the only place the real reason is written down.
type EgressReporter interface {
	Egress(name string, limit int) ([]EgressEvent, error)
}

// DiskReporter locates a machine's on-disk state so it can be priced. An error means "unknown",
// which callers report as unmeasured rather than as zero.
type DiskReporter interface {
	DataDir(name string) (string, error)
}

// Client satisfies all of them. Stated here so the compiler says so at build time rather than at
// the first type assertion.
var (
	_ Backend        = Client{}
	_ Packer         = Client{}
	_ Brancher       = Client{}
	_ EgressReporter = Client{}
	_ DiskReporter   = Client{}
)

// Name identifies the backend in refusals and in doctor.
func (c Client) Name() string { return "smolvm" }

// Caps is what a backend can do, for `boxer doctor` to print and for a refusal to name.
//
// Most of it is derived from the method set rather than declared, because a hand-written
// capability list is a second source of truth that drifts from the methods it describes. Only the
// facts no interface expresses are declared, and the zero value of those is smolvm's answer, so a
// backend that behaves like smolvm declares nothing.
type Caps struct {
	Backend string `json:"backend"`

	// Derived from the optional interfaces above.
	Packs     bool `json:"packs"`
	Branch    bool `json:"branch"`
	Egress    bool `json:"egress"`
	DiskUsage bool `json:"disk_usage"`

	// Declared, because no method set expresses them.
	//
	// Boundary is the one that matters most and is the easiest to leave out. boxer's whole claim
	// is that the agent cannot escape; a backend that shares one kernel between every sandbox is
	// making a materially weaker promise than one kernel each, and the person choosing it has to
	// be told which they got rather than left to infer it from the backend's name.
	Boundary        string `json:"boundary"`           // "kernel" — one per sandbox; "namespace" — shared
	HostMounts      bool   `json:"host_mounts"`        // the worktree is the same files on both sides
	Allowlist       bool   `json:"allowlist"`          // supports a per-host egress allowlist
	SecretEnv       bool   `json:"secret_env"`         // forwards a secret without it entering host argv
	MountAtHostPath bool   `json:"mount_at_host_path"` // the guest path is the host path, not boxer's to choose
	// ProvesOwnership is whether the backend can carry boxer's label and read it back. When it
	// cannot — `sbx` has no labels on any object and no inspect to read them from — boxer keeps
	// the mark on its own side instead; see owned.go. Declared rather than derived, because
	// "has labels" is not a method.
	ProvesOwnership bool `json:"proves_ownership"`
}

// Declarer supplies the facts CapsOf cannot derive. Optional: a backend shaped like smolvm needs
// none of it.
type Declarer interface{ Declare() Caps }

// CapsOf reports what b can do, by asking the type system and then letting b correct the rest.
func CapsOf(b Backend) Caps {
	c := Caps{Backend: b.Name(), Boundary: "kernel", HostMounts: true, Allowlist: true,
		SecretEnv: true, ProvesOwnership: true}
	if d, ok := b.(Declarer); ok {
		got := d.Declare()
		got.Backend = c.Backend
		c = got
	}
	_, c.Packs = b.(Packer)
	_, c.Branch = b.(Brancher)
	_, c.Egress = b.(EgressReporter)
	_, c.DiskUsage = b.(DiskReporter)
	return c
}

// Unsupported refuses a feature the backend cannot honour, naming both.
//
// Refusing is the whole point: a backend that silently degrades hands back a command that appears
// to work with the property the user asked for quietly missing, which is worse than a failure
// because nothing reports it. The refusal wraps ErrUnsupported so callers can recognise the
// class, and carries the fix because a person who hits this needs to know which backend to use.
func Unsupported(b Backend, feature, fix string) error {
	return &Error{
		Backend: b.Name(),
		Verb:    "refused",
		Code:    -1,
		Stderr:  fmt.Sprintf("cannot %s; %s", feature, fix),
		Kind:    ErrUnsupported,
	}
}

// Owned returns only machines boxer created, which means only machines carrying its label. A name
// is not proof of ownership: a machine boxer did not create is not boxer's to delete, however
// much it looks like one.
//
// A branch child needs no special case. smolvm copies the source's labels onto it (verified
// against 1.16.1), so a fork carries `boxer.scope` naming its parent and is owned by the same rule
// as everything else.
//
// A function rather than a method: the rule is boxer's, identical for every backend, and there is
// no reason for each one to reimplement it.
func Owned(b Backend) ([]Machine, error) {
	all, err := b.List()
	if err != nil {
		return nil, err
	}
	// A backend that cannot carry a label is matched against boxer's own record instead. Both
	// tests answer "did boxer make this"; neither answers "is this named like one of boxer's".
	var marks map[string]bool
	if !CapsOf(b).ProvesOwnership {
		marks = registered(b.Name())
	}
	var out []Machine
	for _, m := range all {
		_, labelled := m.Labels[LabelPrefix+"scope"]
		if labelled || marks[m.Name] {
			out = append(out, m)
		}
	}
	return out, nil
}

// OwnsName reports whether boxer can prove it made the machine called name: the same test Owned
// applies to a listing. Everything that deletes or stops by name asks it first, because a name
// alone is not proof, and smolvm's delete cascades to the machine's branches.
func OwnsName(b Backend, name string) (bool, error) {
	ms, err := Owned(b)
	if err != nil {
		return false, err
	}
	for _, m := range ms {
		if m.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// NotOwned is the refusal for a name boxer cannot prove it made.
func NotOwned(b Backend, name string) error {
	return fmt.Errorf("%s has no machine boxer made called %q; boxer only stops or deletes its own (boxer ls lists them)", b.Name(), name)
}

// guestTimeout runs argv under a deadline kept in the guest. A deadline kept only on the host kills
// the docker or container CLI and leaves the command running in the container, where nothing will
// ever stop it; `docker exec` does not end its process when the client dies. KILL, because a
// timed-out task is past asking: the code is then 137.
//
// GNU timeout kills the command's whole process group. busybox's (Alpine) kills only the process
// it started, so `npm test` timed out and left its workers running; and an image with no timeout
// had no deadline at all. Without GNU's, a watchdog started in the background checks once a second
// and, at the deadline, freezes and kills the command's process tree, found through /proc. The
// command itself is exec'd in the foreground: a background job starts with SIGINT and SIGQUIT
// ignored, which dash and busybox cannot undo, and a test that interrupts a child would fail.
// The process name in /proc/N/stat can hold spaces (npm sets "npm run lint"), so the fields are
// read after its closing parenthesis.
// ponytail: a process that detaches itself (a double fork) leaves the tree and escapes; GNU's
// process-group kill would catch it, and nothing without a terminal can make a group here.
func guestTimeout(d time.Duration, argv []string) []string {
	secs := strconv.Itoa(int((d + time.Second - 1) / time.Second))
	watchdog := `tree() { kill -STOP $1 2>/dev/null; for d in /proc/[0-9]*; do read -r s 2>/dev/null < $d/stat || continue; s=${s##*) }; s=${s#* }; [ "${s%% *}" = $1 ] && [ ${d#/proc/} != $$ ] && tree ${d#/proc/}; done; kill -KILL $1 2>/dev/null; }
n=0; while kill -0 $1 2>/dev/null; do [ $n -ge $2 ] && { tree $1; exit; }; sleep 1; n=$((n+1)); done`
	line := `timeout --version 2>/dev/null | grep -q GNU && exec timeout -s KILL ` + secs + ` "$@"
sh -c '` + watchdog + `' boxer-watchdog $$ ` + secs + ` </dev/null >/dev/null 2>&1 &
exec "$@"`
	return append([]string{"sh", "-c", line, "boxer-timeout"}, argv...)
}

// hostBackstop is how much longer than a timeout the host waits before killing the CLI itself,
// for a guest that never got as far as the deadline in guestTimeout.
const hostBackstop = 3 * time.Second

// Output runs argv in the guest and returns its combined output and exit status. Also a function
// rather than a method, for the same reason: it is Exec with two buffers.
func Output(b Backend, name, workdir string, argv ...string) (string, int, error) {
	var buf bytes.Buffer
	code, err := b.Exec(ExecOpts{Name: name, Workdir: workdir, Stdin: strings.NewReader(""), Stdout: &buf, Stderr: &buf}, argv...)
	return buf.String(), code, err
}

// asExitError is errors.As for *exec.ExitError, kept in one place because both backends need it.
func asExitError(err error, target **exec.ExitError) bool { return errors.As(err, target) }

// killedCLI is the error for a runtime CLI that a signal ended: exit status -1, which is not the
// command's status, and was recorded as one, as if the command had exited 255.
func killedCLI(bin string, ee *exec.ExitError) error {
	return fmt.Errorf("%s was ended by a signal (%v) before it reported the command's exit status", filepath.Base(bin), ee.ProcessState)
}

// Default returns the backend named by config. An unknown name is an error rather than a silent
// fall back to smolvm: someone who wrote `backend = "dcoker"` wants to be told, not sandboxed by
// something they did not choose.
func Default(name string) (Backend, error) {
	switch name {
	case "", "smolvm":
		return New(), nil
	case "docker", "podman":
		return NewDocker(name), nil
	case "container":
		return NewApple(), nil
	}
	return nil, fmt.Errorf("unknown backend %q; known: smolvm, docker, podman, container", name)
}

// Names are every backend boxer drives, in the order commands that look at all of them report.
var Names = []string{"smolvm", "docker", "podman", "container"}

// Bin is the binary a backend runs.
func Bin(b Backend) string {
	switch t := b.(type) {
	case Client:
		return t.Bin
	case Docker:
		return t.Bin
	case Apple:
		return t.Bin
	}
	return b.Name()
}

// Installed reports whether a backend's binary is on this host.
func Installed(b Backend) bool {
	_, err := exec.LookPath(Bin(b))
	return err == nil
}

// Host is the backend for commands that work across every sandbox on the host rather than within
// one worktree — `ls`, `gc`, `down --all`, `watch`.
//
// Hard-coding smolvm for these was a real bug, found by running the smoke suite on a second
// backend: with `backend = "docker"` they asked smolvm for its machines, were told there were
// none, and reported nothing to reclaim while docker containers accumulated. `gc` cannot clean up
// what it does not look at.
//
// The name is taken from the caller, which reads it from the same configuration every other
// command does, and falls back to the environment for the case where there is no repository to
// read — which is exactly when `boxer gc` is most likely to be run.
func Host(configured string) Backend {
	if configured == "" {
		configured = os.Getenv("BOXER_BACKEND")
	}
	b, err := Default(configured)
	if err != nil {
		return New()
	}
	return b
}

// A container CLI reports its own failures with an exit status a guest command could also have —
// docker and Apple's container exit 1 for "no such container" and "is not running", podman 125 —
// so the status alone cannot say whether the command ran. Treating it as the command's own was a
// contract rule 2 violation: `boxer run -c "exit 4"` reported exit 1, from a command that never
// started, and `capsule replay` was the only thing that noticed.
//
// What does tell them apart is that the runtime's message is the first thing on stderr, before the
// guest could have written anything. headWriter keeps those first bytes; runtimeFailure decides.
type headWriter struct {
	w    io.Writer
	head []byte
	mu   *sync.Mutex
}

func (h *headWriter) Write(p []byte) (int, error) {
	if h.mu != nil {
		h.mu.Lock()
		defer h.mu.Unlock()
	}
	if n := 160 - len(h.head); n > 0 {
		if n > len(p) {
			n = len(p)
		}
		h.head = append(h.head, p[:n]...)
	}
	if h.w == nil {
		return len(p), nil
	}
	return h.w.Write(p)
}

// lockedWriter shares headWriter's mutex, so stdout and stderr never write at the same moment.
type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w == nil {
		return len(p), nil
	}
	return l.w.Write(p)
}

// streams wires a container CLI's stdout and stderr. exec copies each through its own goroutine
// once stderr is wrapped, and callers routinely pass one buffer for both (the MCP tool's output,
// a probe's combined text), which made those two goroutines race on the buffer. One mutex across
// both serialises them, whether or not the writers are the same.
func streams(stdout, stderr io.Writer) (io.Writer, *headWriter) {
	mu := &sync.Mutex{}
	return lockedWriter{w: stdout, mu: mu}, &headWriter{w: stderr, mu: mu}
}

// runtimeFailure returns the error a runtime's own failure deserves, or nil when code is the
// command's. prefixes maps the exit status the runtime uses to the prefix its messages carry.
func runtimeFailure(b Backend, code int, h *headWriter, prefixes map[int][]string, classify func(string) error) error {
	matched := false
	for _, p := range prefixes[code] {
		if strings.HasPrefix(string(h.head), p) {
			matched = true
		}
	}
	if !matched {
		return nil
	}
	msg := strings.TrimSpace(string(h.head))
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return &Error{Backend: b.Name(), Verb: "exec", Code: code, Stderr: msg, Kind: classify(msg)}
}

// loopbackPort binds a "host:guest" publish to 127.0.0.1. docker, podman and Apple's container
// publish on every interface by default, which puts a sandbox's dev server on the local network;
// smolvm binds loopback, and the container backends now match it. A spec that already names an
// address ("0.0.0.0:3000:3000") is the user's choice and passes through.
func loopbackPort(p string) string {
	if strings.Count(p, ":") == 1 {
		return "127.0.0.1:" + p
	}
	return p
}
