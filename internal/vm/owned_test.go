package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// unlabelled is a backend with no labels at all — `sbx`'s shape. It exists so the registry has
// something to be necessary for.
type unlabelled struct{ *vmtest.Bare }

func (u unlabelled) Name() string { return "unlabelled" }
func (u unlabelled) Declare() vm.Caps {
	return vm.Caps{Boundary: "kernel", HostMounts: true, ProvesOwnership: false}
}

// The rule the registry exists to preserve: boxer owns what boxer made, and a name is not proof.
// The failure that would matter is the registry making boxer claim something it did not create,
// so that is what this asserts first.
func TestABackendWithoutLabelsOwnsOnlyWhatBoxerRecorded(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b := unlabelled{vmtest.NewBare()}

	// Two machines exist. boxer made one of them.
	for _, n := range []string{"sb-mine", "somebody-elses"} {
		if err := b.Create(vm.CreateSpec{Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	vm.RecordOwned(b, "sb-mine")

	owned, err := vm.Owned(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 1 || owned[0].Name != "sb-mine" {
		t.Fatalf("owned %v, want only sb-mine — a machine boxer did not create is not boxer's", owned)
	}
}

// Losing the registry must make boxer forget a machine, never delete a stranger's. Leaking is the
// safe direction and is the entire argument for doing this on the host side at all.
func TestLosingTheRegistryLeaksRatherThanMisclaims(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	b := unlabelled{vmtest.NewBare()}
	if err := b.Create(vm.CreateSpec{Name: "sb-mine"}); err != nil {
		t.Fatal(err)
	}
	vm.RecordOwned(b, "sb-mine")

	// The registry is thrown away, as a wiped state dir or a new machine would do.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	owned, err := vm.Owned(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 0 {
		t.Errorf("without its record boxer still claimed %v; it must forget, not guess", owned)
	}
}

// Forgetting removes the claim, so a deleted machine's name cannot be inherited by a later one.
func TestForgettingDropsTheClaim(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b := unlabelled{vmtest.NewBare()}
	_ = b.Create(vm.CreateSpec{Name: "sb-mine"})
	vm.RecordOwned(b, "sb-mine")
	vm.ForgetOwned(b, "sb-mine")
	if owned, _ := vm.Owned(b); len(owned) != 0 {
		t.Errorf("still claimed after ForgetOwned: %v", owned)
	}
}

// A machine deleted outside boxer leaves a mark behind; pruning drops it. Bookkeeping only — the
// prune must never remove a machine.
func TestPruningDropsMarksForMachinesThatAreGone(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b := unlabelled{vmtest.NewBare()}
	_ = b.Create(vm.CreateSpec{Name: "sb-live"})
	vm.RecordOwned(b, "sb-live")
	vm.RecordOwned(b, "sb-vanished") // never existed, or removed behind boxer's back

	vm.PruneOwned(b)

	owned, _ := vm.Owned(b)
	if len(owned) != 1 || owned[0].Name != "sb-live" {
		t.Fatalf("after pruning, owned %v, want only sb-live", owned)
	}
	if _, ok, _ := b.Status("sb-live"); !ok {
		t.Error("pruning removed a machine; it may only remove bookkeeping")
	}
}

// A backend that carries labels must not touch the registry at all. Writing a mark there would
// create a second source of truth that can disagree with the label, and the label is the one the
// contract calls proof.
func TestALabelledBackendNeverWritesToTheRegistry(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	for _, b := range []vm.Backend{vm.New(), vm.NewDocker("docker")} {
		if !vm.CapsOf(b).ProvesOwnership {
			t.Errorf("%s carries labels but does not claim to prove ownership", b.Name())
			continue
		}
		vm.RecordOwned(b, "sb-x")
		if _, err := os.Stat(filepath.Join(state, "boxer", "owned", b.Name())); !os.IsNotExist(err) {
			t.Errorf("%s wrote a registry entry it does not need (%v)", b.Name(), err)
		}
	}
}
