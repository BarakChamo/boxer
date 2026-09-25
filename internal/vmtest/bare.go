package vmtest

import (
	"fmt"
	"io"
	"sync"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Bare is a backend that can do only what every backend must: the nine core methods and nothing
// else. No packs, no branching, no egress log, no disk accounting.
//
// It exists so the refusals are exercised. Every `if _, ok := b.(vm.Packer); !ok` branch in boxer
// is unreachable while smolvm is the only backend, and unreachable code is code that is wrong the
// first time something reaches it. Bare reaches all of them, in milliseconds, with no smolvm on
// the host — which is also the point: the refusal paths are the ones a *user* on a different
// backend hits first, so they deserve better coverage than the happy path, not worse.
//
// Deliberately not a full simulation. It records calls and returns plausible values; the fake
// smolvm binary in fake.go is what tests real CLI argv, and that job does not transfer to an
// in-process type.
type Bare struct {
	mu       sync.Mutex
	machines map[string]vm.Machine
	Calls    []string
	// ExecCode is returned by every Exec, so a test can drive the non-zero path.
	ExecCode int
	// ExecOut is written to the caller's stdout before returning.
	ExecOut string
}

// NewBare returns an empty bare backend.
func NewBare() *Bare { return &Bare{machines: map[string]vm.Machine{}} }

func (b *Bare) note(f string, a ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Calls = append(b.Calls, fmt.Sprintf(f, a...))
}

func (b *Bare) Name() string             { return "bare" }
func (b *Bare) Version() (string, error) { return "bare 0", nil }
func (b *Bare) Declare() vm.Caps         { return vm.Caps{Boundary: "namespace", HostMounts: true} }

func (b *Bare) List() ([]vm.Machine, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]vm.Machine, 0, len(b.machines))
	for _, m := range b.machines {
		out = append(out, m)
	}
	return out, nil
}

func (b *Bare) Status(name string) (vm.Machine, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.machines[name]
	return m, ok, nil
}

func (b *Bare) Create(s vm.CreateSpec) error {
	b.note("create %s", s.Name)
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.machines[s.Name]; ok {
		return &vm.Error{Verb: "create", Code: 1, Stderr: "already there", Kind: vm.ErrAlreadyExists}
	}
	b.machines[s.Name] = vm.Machine{Name: s.Name, State: "created", Image: s.Image, Labels: s.Labels}
	return nil
}

func (b *Bare) Start(name string) error { return b.setState(name, "running") }
func (b *Bare) Stop(name string) error  { return b.setState(name, "stopped") }

func (b *Bare) setState(name, state string) error {
	b.note("%s %s", state, name)
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.machines[name]
	if !ok {
		return &vm.Error{Verb: state, Code: 1, Stderr: "no such machine", Kind: vm.ErrNotFound}
	}
	m.State = state
	b.machines[name] = m
	return nil
}

// Delete tolerates an already-deleted machine, as the contract requires.
func (b *Bare) Delete(name string) error {
	b.note("delete %s", name)
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.machines, name)
	return nil
}

func (b *Bare) Exec(o vm.ExecOpts, argv ...string) (int, error) {
	b.note("exec %s %v", o.Name, argv)
	if o.Stdout != nil && b.ExecOut != "" {
		_, _ = io.WriteString(o.Stdout, b.ExecOut)
	}
	return b.ExecCode, nil
}
