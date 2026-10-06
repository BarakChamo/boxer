package install

import (
	"fmt"
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
	//
	// The block keeps the status the hook had when it reached it, and adds none of its own. It
	// used to be a bare `[ ... ] && ...` chain, whose false test became the hook's status, so
	// `git checkout -- file` (flag 0) exited 1 and broke every script with `set -e`.
	gitHookBody = `boxer_rc=$?
if [ "$3" = "1" ] && command -v boxer >/dev/null 2>&1; then boxer up --detach >/dev/null 2>&1 || true; fi
(exit $boxer_rc)`
)

// Git writes boxer's post-checkout hook into the repository's hooks directory, honouring
// core.hooksPath, merging into an existing hook file once. Opt-in only: it touches the
// developer's git configuration, so `boxer install all` leaves it out.
// hookPath is the post-checkout hook git runs for repoRoot, honouring core.hooksPath.
func hookPath(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	return filepath.Join(dir, "post-checkout"), nil
}

func Git(repoRoot string) (Result, error) {
	path, err := hookPath(repoRoot)
	if err != nil {
		return Result{}, err
	}
	r := &Result{}
	block := gitBlockStart + "\n" + gitHookBody + "\n" + gitBlockEnd + "\n"
	if b, err := os.ReadFile(path); os.IsNotExist(err) {
		block = "#!/bin/sh\n" + block
	} else if err == nil {
		// boxer's block is shell. Appended to a Python or Node hook it is a syntax error on every
		// checkout, so a hook in another language is left alone and the line handed over instead.
		if interp := shebang(string(b)); interp != "" && !shellInterp[interp] {
			return *r, fmt.Errorf("%s is a %s script, not a shell one; add this to it yourself: boxer up --detach (when the checkout flag, $3, is 1)", path, interp)
		}
		if strings.Contains(stripBlock(string(b), gitBlockStart, gitBlockEnd), "\nexit") {
			r.Notes = append(r.Notes, path+" calls exit before its end, so boxer's block, appended last, may never run.")
		}
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

var shellInterp = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "ash": true}

// shebang returns the interpreter a script names, through env: "python3" for
// `#!/usr/bin/env python3`, "" for a file without one.
func shebang(s string) string {
	if !strings.HasPrefix(s, "#!") {
		return ""
	}
	line, _, _ := strings.Cut(s[2:], "\n")
	f := strings.Fields(line)
	if len(f) == 0 {
		return ""
	}
	name := filepath.Base(f[0])
	if name == "env" && len(f) > 1 {
		name = filepath.Base(f[len(f)-1])
	}
	return name
}

// stripBlock is s without the marked block, so a check on the user's content ignores boxer's.
func stripBlock(s, start, end string) string {
	i := strings.Index(s, start)
	j := strings.Index(s, end)
	if i < 0 || j < i {
		return s
	}
	return s[:i] + s[j+len(end):]
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
