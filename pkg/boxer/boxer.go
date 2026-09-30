// Package boxer is the importable surface of boxer: open the sandbox for a working directory,
// make sure it exists, run commands in it, take it down, and list what is running on the host.
// Everything else in the module lives under internal/ and is not importable.
//
// Covered by the 1.0 stability contract in docs/release.md: identifiers are added, never renamed
// or removed.
package boxer

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Machine is one sandbox, on any backend, as boxer sees it; Labels carry the boxer.* keys.
type Machine = vm.Machine

// Scope is the resolved sandbox identity: Key is the machine name, Root the mounted worktree.
type Scope = scope.Scope

// Error is boxer's agent-readable refusal: Reason, Cause (a stable code), Fix, and Scope.
type Error = box.Error

// RunOpts controls Run: the three streams and whether to request a TTY.
type RunOpts = box.RunOpts

// Options selects the harness configuration layer and the identity used by session and
// subagent isolation. All fields may be empty.
type Options struct {
	Harness   string
	SessionID string
	AgentID   string
}

// Box is the sandbox resolved for one working directory.
type Box struct {
	env *box.Env
}

// Open resolves the sandbox for cwd (the process working directory when empty). The error is a
// *Error when boxer refused (no repository, worktree required, scope unresolved).
func Open(cwd string, o Options) (*Box, error) {
	e, err := box.Resolve(cwd, o.Harness, scope.Identity{SessionID: o.SessionID, AgentID: o.AgentID})
	if err != nil {
		return nil, err
	}
	return &Box{env: e}, nil
}

// Scope returns the resolved scope.
func (b *Box) Scope() Scope { return b.env.Scope }

// Warnings are the non-fatal findings from resolution (main checkout, degraded isolation).
func (b *Box) Warnings() []string { return b.env.Warnings }

// Ensure makes the machine exist and run. It creates only when allowCreate is set; recreate
// deletes an existing machine first. It reports whether a machine was created.
func (b *Box) Ensure(allowCreate, recreate bool) (created bool, err error) {
	return b.env.Ensure(allowCreate, recreate)
}

// Run executes argv in the guest, provisioning first when the configuration allows, and returns
// the guest exit code. A code of -1 with an error means the passthrough policy applies and the
// caller may run argv on the host.
func (b *Box) Run(argv []string, o RunOpts) (int, error) { return b.env.Run(argv, o) }

// Down deletes the machine. Absence is not an error.
func (b *Box) Down() error { return b.env.Down() }

// Status returns the machine and whether it exists.
func (b *Box) Status() (Machine, bool, error) { return b.env.Exists() }

// List returns every boxer-owned machine on the backend the CLI would use from the process's
// working directory: BOXER_BACKEND, else the one boxer.toml names, else smolvm.
func List() ([]Machine, error) { return vm.Owned(host()) }

// ListAll returns every boxer-owned machine on every backend installed on this host, with
// Machine.Backend saying which. A backend that is installed and not answering is skipped, and its
// error returned alongside whatever the others listed.
func ListAll() ([]Machine, error) {
	var out []Machine
	var errs []error
	for _, n := range vm.Names {
		c := vm.Host(n)
		if !vm.Installed(c) {
			continue
		}
		ms, err := vm.Owned(c)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n, err))
			continue
		}
		for _, m := range ms {
			m.Backend = n
			out = append(out, m)
		}
	}
	return out, errors.Join(errs...)
}

// Ports returns a machine's forwarded ports, guest port to host port, from its labels.
func Ports(m Machine) map[string]string { return box.PortsOf(m) }

// URLs returns a URL for each forwarded port of a machine, keyed by guest port: its portless name
// when [urls] is on and the route is registered, otherwise http://127.0.0.1:<host port>.
func URLs(m Machine) map[string]string { return box.ReachableURLs(m) }

// Stop halts a machine by name; stopping a stopped machine is not an error.
func Stop(name string) error { return host().Stop(name) }

// Delete removes a machine by name, and what boxer kept about it, as `boxer rm` does: its run
// record, last-used stamp, lock and URLs. Its named volumes are kept.
func Delete(name string) error {
	c := host()
	if err := c.Delete(name); err != nil {
		return err
	}
	vm.ForgetOwned(c, name)
	box.ForgetScope(name)
	return nil
}

func host() vm.Backend {
	cwd, _ := os.Getwd()
	return vm.Host(box.HostBackend(cwd))
}

// LastUsed returns when the scope last ran a command, or the zero time when unknown.
func LastUsed(name string) time.Time { return box.LastUsed(name) }
