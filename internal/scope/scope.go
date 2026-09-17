// Package scope turns "where am I and who is asking" into the name of one VM.
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

// Detect asks git about cwd. A non-repository is not an error; Toplevel is simply empty.
func Detect(cwd string) (Git, error) {
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
	top := lines[0]
	gitDir := absIn(top, lines[1])
	common := absIn(top, lines[2])
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
// (harness outside, harness inside) own two VMs.
func ResolveTagged(isolation, onMissingID string, g Git, id Identity, tag string) (Scope, error) {
	root := g.Toplevel
	if root == "" {
		return Scope{}, errors.New("not inside a git repository")
	}
	want := isolation
	material := ""
	var degraded bool
	var reason string
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
