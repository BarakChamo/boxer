package box

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// A fork is a copy-on-write child of a running sandbox: smolvm branches the RAM and the disks, so
// the child starts with the parent's processes and page cache already warm rather than booting.
// It is for the work that wants many attempts at one state — a sweep, a bisect, several agents
// trying the same fix — and it is cheap in a way a second `boxer up` is not.
//
// Two smolvm constraints shape everything here:
//
//   - A source must have been started --branchable, which is a property of the current boot. A
//     restart is enough, but boxer has to remember, because smolvm reports no such field.
//   - A child inherits the source's volumes and cannot override them, and smolvm refuses to
//     branch a machine whose mount is `:staged` at all ("multiple descendants would synchronize
//     into the same host directory"). So every child shares the parent's live worktree, and
//     there is no per-child copy to offer.
//
// That bounds what a fork is for. Children are warm workers over one set of files: shards that
// write to separate paths, read-mostly checks, a warm start. Two children writing the same file
// race each other exactly as two processes on your machine would, and boxer cannot prevent it.

// forkChild matches the names boxer gives children. It is not how a child is *owned* — smolvm
// copies the source's labels onto a branch, so `ls` and `gc` find one by the ordinary rule — but
// it is how boxer tells a child from a sandbox when deciding what may be synced, what may be
// removed with `fork rm`, and what must never be recreated.
var forkChild = regexp.MustCompile(`^sb-[0-9a-f]{12}-f\d+$`)

// IsForkChild reports whether a machine name is one of boxer's fork children.
func IsForkChild(name string) bool { return forkChild.MatchString(name) }

// ParentOf returns the scope key a fork child was branched from.
func ParentOf(child string) string {
	if i := strings.LastIndex(child, "-f"); IsForkChild(child) && i > 0 {
		return child[:i]
	}
	return ""
}

// branchableMarker records that this scope's current boot is branchable. smolvm exposes no way to
// ask, and guessing wrong means a confusing failure from `machine branch` instead of a refusal
// that says what to do.
func branchableMarker(key string) string {
	return filepath.Join(filepath.Dir(LastUsedDir()), "branchable", key)
}

// Branchable reports whether PrepareFork has run for this scope since it was last stopped.
func Branchable(key string) bool {
	_, err := os.Stat(branchableMarker(key))
	return err == nil
}

func setBranchable(key string, on bool) {
	p := branchableMarker(key)
	if !on {
		_ = os.Remove(p)
		return
	}
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		_ = os.WriteFile(p, nil, 0o644)
	}
}

// stopVM stops the sandbox and forgets that it was branchable: the memfd and the control socket
// go with the process, so a marker that outlived a stop would promise a fork that cannot happen.
func (e *Env) stopVM() error {
	e.seen = nil
	setBranchable(e.Scope.Key, false)
	return e.VM.Stop(e.Scope.Key)
}

// deleteVM removes the sandbox, its children, and the marker saying it was branchable.
func (e *Env) deleteVM() error {
	// The ports, read before the machine is gone: a route to one of them is this sandbox's even
	// when this process cannot see the registry entry that recorded it.
	var ports map[string]string
	if e.Cfg.URLs.Enabled {
		if m, ok, err := e.Exists(); err == nil && ok {
			ports = e.hostPorts(m)
		}
	}
	e.seen = nil
	setBranchable(e.Scope.Key, false)
	err := e.VM.Delete(e.Scope.Key)
	if err == nil && len(ports) > 0 {
		removeRoutesTo(ports)
	}
	// After the delete, and regardless of whether it succeeded: Delete tolerates an already-gone
	// machine, so a failure here means something else, and keeping a mark for a machine that may
	// not exist only delays a prune.
	vm.ForgetOwned(e.VM, e.Scope.Key)
	if err == nil {
		ForgetScope(e.Scope.Key)
	}
	return err
}

func (e *Env) forkPrefix() string { return e.Scope.Key + "-f" }

// PrepareFork restarts this scope's sandbox as a branch source. That is all it does: the worktree
// stays the live mount every child will share, because smolvm cannot branch anything else.
func (e *Env) PrepareFork() error {
	brancher, ok := e.VM.(vm.Brancher)
	if !ok {
		return vm.Unsupported(e.VM, "fork a sandbox",
			"copy-on-write branching needs a hypervisor; set backend = \"smolvm\"")
	}
	if _, err := e.Ensure(true, false); err != nil {
		return err
	}
	if err := e.stopVM(); err != nil {
		return err
	}
	if err := brancher.StartBranchable(e.Scope.Key); err != nil {
		setBranchable(e.Scope.Key, false)
		return err
	}
	setBranchable(e.Scope.Key, true)
	e.event(obs.Provision, obs.OK, 0, map[string]any{"kind": "fork-prepare"})
	return nil
}

// Fork branches count children off this scope's prepared sandbox and returns their names.
func (e *Env) Fork(count int) ([]string, error) {
	brancher, ok := e.VM.(vm.Brancher)
	if !ok {
		return nil, vm.Unsupported(e.VM, "fork a sandbox",
			"copy-on-write branching needs a hypervisor; set backend = \"smolvm\"")
	}
	if !Branchable(e.Scope.Key) {
		return nil, &Error{
			Reason: "this sandbox was not started as a branch source, so it cannot be forked. Preparing restarts it, which ends whatever is running in it",
			Cause:  "NOT_FORKABLE", Scope: e.Scope, Fix: "boxer fork --prepare",
		}
	}
	start := time.Now()
	names, err := brancher.Branch(vm.BranchSpec{
		From: e.Scope.Key, NamePrefix: e.forkPrefix(), Count: count, SecretEnv: e.HostSecrets(),
	})
	outcome := obs.OK
	if err != nil {
		outcome = obs.Failed
	}
	e.event(obs.Provision, outcome, time.Since(start), map[string]any{"kind": "fork", "count": count})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// Forks lists this scope's children.
func (e *Env) Forks() ([]vm.Machine, error) {
	all, err := e.VM.List()
	if err != nil {
		return nil, err
	}
	var out []vm.Machine
	for _, m := range all {
		if ParentOf(m.Name) == e.Scope.Key {
			out = append(out, m)
		}
	}
	return out, nil
}

// ResolveName turns what a person typed — a scope key, a slug, or a fork child's either form —
// into the machine name. A slug is derived from the key, so the only way back is to look at what
// exists; an unknown name is returned unchanged so the caller's own "no such sandbox" is what the
// reader sees.
func ResolveName(client vm.Backend, name string) string {
	if _, ok, err := client.Status(name); err == nil && ok {
		return name
	}
	all, err := client.List()
	if err != nil {
		return name
	}
	for _, m := range all {
		if scope.Slug(m.Name) == name {
			return m.Name
		}
	}
	return name
}

// At returns this Env pointed at another sandbox by name — a fork child, or any sandbox a
// listing named. The configuration and the worktree stay this scope's, because a child is a
// branch of this worktree and has no other one; only the machine changes.
func (e *Env) At(name string) (*Env, error) {
	name = ResolveName(e.VM, name)
	if _, ok, err := e.VM.Status(name); err != nil {
		return nil, err
	} else if !ok {
		return nil, &Error{Reason: fmt.Sprintf("no sandbox named %q", name), Cause: "NO_SUCH_SCOPE",
			Scope: e.Scope, Fix: "boxer ls"}
	}
	out := *e
	out.seen = nil // a different machine: the memo describes this Env's, not that one's
	out.Scope.Key = name
	return &out, nil
}
