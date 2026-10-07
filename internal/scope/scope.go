// Package scope turns "where am I and who is asking" into the name of one VM.
//
// The name is a hash rather than a path because it is also the smolvm machine name, which has to
// survive a worktree whose path contains anything a filesystem allows, and has to stay the same
// length whatever the path. It is not a secret: two people with the same worktree path get the
// same name, and nothing depends on them not.
package scope

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git describes the checkout the caller is in. Toplevel is empty outside a repository.
type Git struct {
	Toplevel  string
	CommonDir string
	Linked    bool
}

// RepoRoot is the main working tree that holds the repository's own boxer.toml: the parent of a
// `.git` common directory. A bare repository has no working tree, and the parent of `proj.git`
// is whatever directory it sits in, whose files are not the repository's; it returns "".
func (g Git) RepoRoot() string {
	if g.CommonDir == "" || filepath.Base(g.CommonDir) != ".git" {
		return ""
	}
	return filepath.Dir(g.CommonDir)
}

// Detect asks git about cwd. A non-repository is not an error; Toplevel is simply empty.
func Detect(cwd string) (Git, error) {
	if g, ok := detectFast(cwd); ok {
		return g, nil
	}
	return detectGit(cwd)
}

// detectGit is the authority the fast path is checked against, and the path taken whenever the
// fast path is not certain.
func detectGit(cwd string) (Git, error) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel", "--git-dir", "--git-common-dir").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return Git{}, nil
		}
		return Git{}, fmt.Errorf("git: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 {
		return Git{}, fmt.Errorf("git rev-parse: unexpected output %q", out)
	}
	// git resolves a relative --git-dir or --git-common-dir against the *current directory*, not
	// against the toplevel. Joining them to the toplevel instead put the common dir one level up
	// per directory of depth: from `internal/box` in this repository, `../../.git` became
	// `<repo>/../../.git`, which does not exist — so every command run from a subdirectory of a
	// main worktree reported Linked, and `isolation = "repo"` hashed a different name per depth.
	top := lines[0]
	gitDir := absIn(cwd, lines[1])
	common := absIn(cwd, lines[2])
	return Git{Toplevel: top, CommonDir: common, Linked: gitDir != common}, nil
}

func absIn(base, p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// Identity is what the harness told us about the caller. Either field may be empty.
type Identity struct {
	SessionID string
	AgentID   string
}

// Scope is the resolved VM identity.
type Scope struct {
	Key       string // "sb-" + 12 hex chars; also the VM name
	Isolation string // the isolation actually used, after degradation
	Root      string // the directory mounted into the guest
	Degraded  bool
	Reason    string
}

// Resolve applies the isolation policy from requirements §3.2 and its degradation rules.
func Resolve(isolation, onMissingID string, g Git, id Identity) (Scope, error) {
	return ResolveTagged(isolation, onMissingID, g, id, "")
}

// ResolveTagged is Resolve with a tag folded into the key, so two placements of the same worktree
// (harness outside, harness inside) own two VMs. They are genuinely different machines — one
// carries the harness and its credentials, the other does not — and sharing one would mean the
// first `boxer shell` silently changed what every outside command runs in.
func ResolveTagged(isolation, onMissingID string, g Git, id Identity, tag string) (Scope, error) {
	root := g.Toplevel
	if root == "" {
		return Scope{}, errors.New("not inside a git repository")
	}
	want := isolation
	material := ""
	var degraded bool
	var reason string
	// The degradation ladder: subagent → session → worktree. isolation is repository policy and is
	// read by every harness, but only some of them hand out the ids the narrower levels need. The
	// alternative to degrading is refusing to run on half the harnesses the repository supports,
	// so `degrade` is the default and `fail` is there for a repository that would rather stop than
	// sandbox at a coarser granularity than it asked for. Either way Scope.Reason says what
	// happened, because a sandbox that is quietly wider than configured is the kind of thing that
	// is only noticed when two agents collide in it.
	switch want {
	case "subagent":
		if id.AgentID == "" {
			if onMissingID == "fail" {
				return Scope{}, errors.New("isolation is subagent but the harness supplied no agent id")
			}
			degraded, reason = true, "no agent id; using session isolation"
			want = "session"
		} else {
			material = root + "\x00" + id.SessionID + "\x00" + id.AgentID
		}
	}
	switch want {
	case "session":
		if id.SessionID == "" {
			if onMissingID == "fail" {
				return Scope{}, errors.New("isolation is session but the harness supplied no session id")
			}
			degraded, reason = true, "no session id; using worktree isolation"
			want = "worktree"
		} else if material == "" {
			material = root + "\x00" + id.SessionID
		}
	}
	switch want {
	case "worktree":
		material = root
	case "repo":
		// The common directory, not the toplevel: every linked worktree shares it, which is
		// exactly the collapse `repo` asks for. The toplevel differs per worktree and would give
		// each one its own VM under the name of the setting that says not to.
		material = g.CommonDir
	case "session", "subagent":
	default:
		return Scope{}, fmt.Errorf("unknown isolation %q", isolation)
	}
	if tag != "" {
		material += "\x00" + tag
	}
	sum := sha256.Sum256([]byte(material))
	return Scope{
		Key:       "sb-" + hex.EncodeToString(sum[:])[:12],
		Isolation: want,
		Root:      root,
		Degraded:  degraded,
		Reason:    reason,
	}, nil
}
