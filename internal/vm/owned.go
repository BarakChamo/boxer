package vm

import (
	"os"
	"path/filepath"
	"strconv"
)

// Ownership, for backends that cannot carry a label.
//
// boxer proves it owns a machine by reading its own label off it, and deletes nothing it cannot
// prove. That rule is the reason `gc` is safe to run on a shared host, and it is not negotiable —
// a name is not proof. But `sbx` has no labels at all, on any object, and no `inspect` to read
// them from, so a backend can be worth having and still have nowhere to put the mark.
//
// For those, boxer writes the mark on its own side: one zero-byte file per machine it created.
//
// This is a fourth kind of host state after packs, locks and markers, and `architecture.md`
// already says what makes that acceptable — boxer keeps no state *it cannot lose*. Losing this
// directory does not corrupt anything and cannot cause a wrong deletion: it makes boxer **forget**
// a machine, which leaks it. Leaking is the safe direction. The dangerous direction is deleting
// something boxer did not create, and a registry that only ever *adds* names to the owned set can
// never do that — an unregistered machine is simply not boxer's, which is the correct answer for
// a machine boxer did not create and the merely inconvenient answer for one whose record was lost.
//
// A `boxer gc` that lists unowned machines for a person to delete closes the leak without ever
// guessing on their behalf.

// ownedDir is where the marks live, one directory per backend so two backends cannot collide on a
// name. It resolves the same way every other piece of boxer state does.
func ownedDir(backend string) string {
	return filepath.Join(StateHome(), "boxer", "owned", backend)
}

// StateHome is the XDG state directory boxer keeps its state under: $XDG_STATE_HOME when it is
// absolute (the specification says to ignore a relative one), else ~/.local/state. With no home
// directory it is a per-user directory under the system's temporary one: a path relative to the
// current directory put gc's state, and what gc deletes, wherever the command happened to run.
func StateHome() string {
	if d := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(d) {
		return d
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		return filepath.Join(home, ".local", "state")
	}
	return filepath.Join(os.TempDir(), "boxer-"+strconv.Itoa(os.Getuid()), "state")
}

// RecordOwned marks a machine as boxer's. Called after a successful create, and only for a backend
// whose Caps say it cannot prove ownership for itself.
//
// A failure to write is deliberately not fatal: the machine exists and works, and the cost of the
// missing mark is that boxer will not reclaim it automatically. Failing the user's command over a
// bookkeeping file would trade a leak for an outage.
func RecordOwned(b Backend, name string) {
	if CapsOf(b).ProvesOwnership {
		return
	}
	dir := ownedDir(b.Name())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_ = f.Close()
	}
}

// ForgetOwned removes the mark, after the machine is gone.
func ForgetOwned(b Backend, name string) {
	if CapsOf(b).ProvesOwnership {
		return
	}
	_ = os.Remove(filepath.Join(ownedDir(b.Name()), name))
}

// registered reports the names this backend's registry claims.
func registered(backend string) map[string]bool {
	entries, err := os.ReadDir(ownedDir(backend))
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			out[e.Name()] = true
		}
	}
	return out
}

// PruneOwned drops marks for machines the backend no longer has, so the registry does not grow
// forever when something is deleted outside boxer. Safe to call at gc time; it removes only
// bookkeeping, never a machine.
func PruneOwned(b Backend) {
	if CapsOf(b).ProvesOwnership {
		return
	}
	all, err := b.List()
	if err != nil {
		return // cannot tell what is gone; keeping a stale mark is harmless
	}
	live := make(map[string]bool, len(all))
	for _, m := range all {
		live[m.Name] = true
	}
	for name := range registered(b.Name()) {
		if !live[name] {
			_ = os.Remove(filepath.Join(ownedDir(b.Name()), name))
		}
	}
}
