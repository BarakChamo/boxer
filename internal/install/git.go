package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	gitBlockStart = "# >>> boxer (managed by `boxer install git`; do not edit inside)"
	gitBlockEnd   = "# <<< boxer"
	// gitHookBody warms the sandbox for a worktree the moment git creates it (R-GIT-1): flag 1
	// means the checkout changed the working tree, which is what `git worktree add` fires in the
	// new worktree. Detached, so git returns at once; a hook without boxer on PATH is silent.
	gitHookBody = `[ "$3" = "1" ] && command -v boxer >/dev/null 2>&1 && boxer up --detach >/dev/null 2>&1`
)

// Git writes boxer's post-checkout hook into the repository's hooks directory, honouring
// core.hooksPath, merging into an existing hook file once. Opt-in only: it touches the
// developer's git configuration, so `boxer install all` leaves it out.
func Git(repoRoot string) (Result, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return Result{}, err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	path := filepath.Join(dir, "post-checkout")
	r := &Result{}
	block := gitBlockStart + "\n" + gitHookBody + "\n" + gitBlockEnd + "\n"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		block = "#!/bin/sh\n" + block
	}
	if err := r.replaceBlock(path, gitBlockStart, gitBlockEnd, block); err != nil {
		return *r, err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return *r, err
	}
	r.Notes = append(r.Notes, "post-checkout runs `boxer up --detach` in every worktree `git worktree add` creates; boxer must be on PATH where git runs.")
	return *r, nil
}

// GitInstalled reports whether the repository's post-checkout hook carries boxer's block.
func GitInstalled(repoRoot string) bool {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return false
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	b, err := os.ReadFile(filepath.Join(dir, "post-checkout"))
	return err == nil && strings.Contains(string(b), gitBlockStart)
}
