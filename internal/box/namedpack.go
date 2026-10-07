package box

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Named packs are the prepared guests someone meant to keep: a base image with the toolchain
// already installed, a state a fork should start from, a snapshot taken before a risky upgrade.
//
// They are a *filesystem* snapshot and nothing more. smolvm can write a real checkpoint —
// `.smolcheckpoint`, RAM and running processes included — but it refuses on any machine holding a
// host mount, and boxer always mounts the worktree, so resume-from-RAM is out of reach for a
// boxer sandbox by construction. A named pack carries what was installed, not what was running.
//
// They live in their own directory so that gc, which sweeps PackDir()/*.smolmachine by age and by
// count, never sees them: a pack someone named is a pack someone meant to keep.
func NamedPackDir() string { return filepath.Join(PackDir(), "named") }

// packName bounds what becomes a path segment.
var packName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// NamedPackPath is the file for a name, or an error when the name could not be one.
func NamedPackPath(name string) (string, error) {
	if !packName.MatchString(name) {
		return "", fmt.Errorf("pack name %q: want letters, digits, dot, dash or underscore, starting with a letter or digit, at most 64", name)
	}
	return filepath.Join(NamedPackDir(), name+".smolmachine"), nil
}

// NamedPack is one saved pack.
type NamedPack struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Bytes   int64     `json:"bytes"`
	SavedAt time.Time `json:"saved_at"`
}

// NamedPacks lists what has been saved, newest first.
func NamedPacks() []NamedPack {
	entries, err := os.ReadDir(NamedPackDir())
	if err != nil {
		return nil
	}
	var out []NamedPack
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".smolmachine") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		out = append(out, NamedPack{
			Name:    strings.TrimSuffix(ent.Name(), ".smolmachine"),
			Path:    filepath.Join(NamedPackDir(), ent.Name()),
			Bytes:   info.Size(),
			SavedAt: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt.After(out[j].SavedAt) })
	return out
}

// SavePack snapshots this scope's running sandbox into a named pack, replacing one of the same
// name. Unlike the automatic packs, every failure is returned: silently not saving something a
// person named is worse than failing in front of them.
func (e *Env) SavePack(name string) (string, error) {
	// First, before anything is validated or provisioned. A backend that cannot snapshot can
	// never satisfy this call, so telling the user to `boxer up` and only then refusing would
	// cost them a boot to learn something knowable up front.
	packer, ok := e.VM.(vm.Packer)
	if !ok {
		return "", vm.Unsupported(e.VM, "save a named environment",
			"set backend = \"smolvm\", which snapshots a machine to a file")
	}
	side, err := NamedPackPath(name)
	if err != nil {
		return "", err
	}
	// The snapshot stops the VM; another boxer must not start it, or run in it, meanwhile.
	unlock, err := lockScope(e.Scope.Key)
	if err != nil {
		return "", err
	}
	defer unlock()
	if _, ok, err := e.Exists(); err != nil {
		return "", err
	} else if !ok {
		return "", &Error{Reason: "there is no sandbox here to save", Cause: "NO_SANDBOX", Scope: e.Scope,
			Fix: "boxer up, then boxer pack save " + name}
	}
	if err := os.MkdirAll(NamedPackDir(), 0o755); err != nil {
		return "", err
	}
	if free, crowded := packingWouldCrowdTheDisk(e.minFree()); crowded {
		return "", fmt.Errorf("%s free, below min_free_gb (%.1f GB): `boxer gc --all` reclaims the automatic packs",
			humanBytes(free), e.Cfg.MinFreeGB)
	}
	stub := strings.TrimSuffix(side, ".smolmachine")
	start := time.Now()
	// smolvm packs only a stopped VM, so it is stopped and restarted around the snapshot.
	// PackFromVM moves the pack over the old one only once whole: a save that fails partway (a
	// full disk, Ctrl-C, a smolvm error) leaves the earlier snapshot of this name as it was.
	err = e.stopVM()
	if err == nil {
		_, err = packer.PackFromVM(e.Scope.Key, stub)
	}
	serr := e.VM.Start(e.Scope.Key)
	if serr != nil && err == nil {
		err = serr
	}
	if serr == nil {
		e.resumeServices("saving the pack")
	}
	outcome := obs.OK
	if err != nil {
		outcome = obs.Failed
	}
	e.event(obs.Pack, outcome, time.Since(start), map[string]any{"kind": "named", "name": name, "path": side})
	if err != nil {
		return "", err
	}
	return side, nil
}
