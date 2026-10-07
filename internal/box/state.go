package box

import (
	"github.com/BarakChamo/boxer/internal/scope"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// boxer's host state is a handful of small files per scope. Each one is safe to lose, which is
// what lets boxer keep them without a database, but "safe to lose" had quietly become "never
// removed": a host that had run the benchmark suite for a few weeks held 1,078 last-used stamps,
// 2,016 lock files, 528 run records and 222 setup markers for sandboxes that no longer existed.
// None of it is large; all of it is exactly the crap boxer promises not to leave behind.
//
// Two kinds, removed at two different moments:
//
//   - machine state describes a sandbox — when it last ran, what it last ran, whether it can be
//     forked, which URLs point at it. It goes the moment the sandbox is deleted.
//   - worktree state (the setup and prep markers) describes the files on the host, which outlive
//     the sandbox: `boxer down` followed by `boxer up` must not re-run `npm ci` into a worktree
//     that already has node_modules. It now lives in the worktree's git directory and goes with
//     the worktree; what is swept here is the copies left in the old location, once they are old.

// stateRoot is the directory every piece of boxer's host state lives under.
func stateRoot() string { return filepath.Dir(LastUsedDir()) }

// ForgetScope removes the machine state of a sandbox that has just been deleted. Worktree state
// is kept: see the package comment above.
func ForgetScope(key string) {
	root := stateRoot()
	_ = os.Remove(filepath.Join(root, "last-used", key))
	_ = os.Remove(filepath.Join(root, "provisioned", key))
	_ = os.Remove(filepath.Join(root, "backend", key))
	_ = os.Remove(RunRecordPath(key))
	_ = os.Remove(branchableMarker(key))
	unpublishURLs(key)
	removeUnheldLock(filepath.Join(root, "locks", key))
}

// staleAfter is how long worktree state and orphaned machine state survive their sandbox.
// ponytail: fixed rather than tied to idle_timeout, because the sweep cannot see sandboxes on a
// backend other than the configured one, and a week is long enough that a second backend's live
// sandboxes have touched their last-used stamp in the meantime.
const staleAfter = 7 * 24 * time.Hour

// SweepState removes per-scope state whose sandbox is not in live and which has not been touched
// for staleAfter. It is what bounds the state directory when sandboxes are deleted by something
// other than boxer — `docker rm`, a reinstalled runtime, a wiped smolvm store — so ForgetScope
// never ran. It returns how many files it removed.
func SweepState(live map[string]bool, now time.Time) int {
	root := stateRoot()
	removed := 0
	stale := func(key string, info os.FileInfo) bool {
		if live[key] {
			return false
		}
		last := info.ModTime()
		if lu := LastUsed(key); lu.After(last) {
			last = lu
		}
		return now.Sub(last) > staleAfter
	}
	for _, dir := range []string{"last-used", "runs", "branchable", "setup", "prep", "locks", "provisioned", "backend"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, ent := range entries {
			key := scopeKeyOf(ent.Name())
			if key == "" {
				continue // not a per-scope file; leave anything boxer does not recognise alone
			}
			info, err := ent.Info()
			if err != nil || !stale(key, info) {
				continue
			}
			p := filepath.Join(root, dir, ent.Name())
			if dir == "locks" {
				if removeUnheldLock(p) {
					removed++
				}
				continue
			}
			if os.Remove(p) == nil {
				removed++
			}
		}
	}
	// A pack being written lives in a hidden directory until it is whole; one whose writer was
	// killed is left there. A day is far longer than any pack takes.
	dirs, _ := filepath.Glob(filepath.Join(PackDir(), ".packing-*"))
	named, _ := filepath.Glob(filepath.Join(NamedPackDir(), ".packing-*"))
	dirs = append(dirs, named...)
	// Image archives a build saved for smolvm, unused for a week: each is a whole image, and an
	// edited Dockerfile leaves the old one behind under another name. A sandbox already created
	// from one keeps running; smolvm unpacked it at create.
	archives, _ := filepath.Glob(filepath.Join(root, "images", "*.tar*"))
	for _, a := range archives {
		if info, err := os.Stat(a); err == nil && now.Sub(info.ModTime()) > staleAfter && os.Remove(a) == nil {
			removed++
		}
	}
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && now.Sub(info.ModTime()) > 24*time.Hour && os.RemoveAll(d) == nil {
			removed++
		}
	}
	return removed
}

// scopeKeyOf extracts "sb-<12 hex>" from a state file name ("sb-…", "sb-….json", "sb-…-a1b2c3d4").
func scopeKeyOf(name string) string {
	if !strings.HasPrefix(name, "sb-") || len(name) < 15 {
		return ""
	}
	key := name[:15]
	for _, c := range key[3:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return ""
		}
	}
	return key
}

// removeUnheldLock deletes a lock file only if nobody holds it. Unlinking a held lock would let
// the next boxer create a fresh file and lock that instead, and two processes would each believe
// they held the scope. Taking the lock first, without waiting, makes the removal safe: whoever
// comes next blocks on nothing and creates a new file, which is exactly what they would have done.
func removeUnheldLock(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // closing releases the flock
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return false
	}
	return os.Remove(path) == nil
}

// HostBackend is the backend a host-wide command (ls, gc, stop, rm) should look at from cwd: the
// one this checkout's configuration names, or "" for BOXER_BACKEND and then the default. A
// checkout that does not resolve still yields its configured backend when it has one.
func HostBackend(cwd string) string {
	e, err := Resolve(cwd, "", scope.Identity{})
	if e == nil || err != nil && e.Cfg.Backend == "" {
		return ""
	}
	return e.Cfg.Backend
}

// provisioned reports whether the scope's sandbox finished setup since it was last provisioned.
func provisioned(key string) bool {
	_, err := os.Stat(filepath.Join(stateRoot(), "provisioned", key))
	return err == nil
}

func setProvisioned(key string, done bool) {
	p := filepath.Join(stateRoot(), "provisioned", key)
	if !done {
		_ = os.Remove(p)
		return
	}
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		_ = os.WriteFile(p, nil, 0o644)
	}
}

// createdOn is the backend that created the scope's sandbox, or "" when boxer did not record one.
func createdOn(key string) string {
	b, _ := os.ReadFile(filepath.Join(stateRoot(), "backend", key))
	return strings.TrimSpace(string(b))
}

func setCreatedOn(key, backend string) {
	p := filepath.Join(stateRoot(), "backend", key)
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		_ = os.WriteFile(p, []byte(backend+"\n"), 0o644)
	}
}
