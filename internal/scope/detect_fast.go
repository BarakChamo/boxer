package scope

import (
	"os"
	"path/filepath"
	"strings"
)

// detectFast answers the same question as `git rev-parse --show-toplevel --git-dir
// --git-common-dir` by reading the filesystem, and reports false when it is not certain it agrees.
//
// It exists because that one git call was the largest single cost in a warm `boxer run`: spawning
// git is about 6ms on a small repository and 9-18ms on a large one, against 15ms for the guest
// command itself. Every boxer command pays it, including the ones that do nothing else.
//
// The rule for anything here is that a wrong answer is far worse than a slow one: the toplevel is
// hashed into the machine name, so disagreeing with git by one symlink would strand every existing
// sandbox under a new name. So this handles only the two layouts that cover ordinary use — a
// `.git` directory, and a `.git` file pointing at a linked worktree — and hands back to git for
// everything else: any GIT_* override, a bare repository, a `.git` that does not look like one.
func detectFast(cwd string) (Git, bool) {
	// Any of these changes discovery in ways this does not model. Cheap to check, and a wrong
	// answer under one of them would be silent.
	for _, k := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_OBJECT_DIRECTORY", "GIT_INDEX_FILE",
	} {
		if _, ok := os.LookupEnv(k); ok {
			return Git{}, false
		}
	}
	// git reports physical paths, so resolve before walking or the hash changes under a symlink.
	dir, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return Git{}, false
	}
	for {
		dot := filepath.Join(dir, ".git")
		st, err := os.Lstat(dot)
		switch {
		case err == nil && st.IsDir():
			if !looksLikeGitDir(dot) {
				return Git{}, false
			}
			// A main worktree's git dir is its own common dir, unless someone has pointed it
			// elsewhere — which is exactly the case this does not model.
			if _, err := os.Stat(filepath.Join(dot, "commondir")); err == nil {
				return Git{}, false
			}
			return Git{Toplevel: dir, CommonDir: absIn(dir, dot), Linked: false}, true
		case err == nil && st.Mode().IsRegular():
			g, ok := linkedWorktree(dir, dot)
			return g, ok
		case err == nil:
			return Git{}, false // a symlink or something stranger; let git decide
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Git{}, true // not in a repository at all; git says the same with an empty Toplevel
		}
		dir = parent
	}
}

// linkedWorktree reads the `gitdir:` pointer a linked worktree keeps in place of a .git directory.
func linkedWorktree(top, dotfile string) (Git, bool) {
	b, err := os.ReadFile(dotfile)
	if err != nil {
		return Git{}, false
	}
	line := strings.TrimSpace(string(b))
	rest, ok := strings.CutPrefix(line, "gitdir:")
	if !ok || strings.Contains(line, "\n") {
		return Git{}, false
	}
	gitDir := absIn(top, strings.TrimSpace(rest))
	// The linked git dir names its common dir in a file, relative to itself.
	c, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return Git{}, false
	}
	common := absIn(gitDir, strings.TrimSpace(string(c)))
	if !looksLikeGitDir(common) {
		return Git{}, false
	}
	return Git{Toplevel: top, CommonDir: common, Linked: gitDir != common}, true
}

// looksLikeGitDir is a sanity check, not a validation: it stops a directory that merely happens to
// be called .git from being read as a repository.
func looksLikeGitDir(p string) bool {
	if _, err := os.Stat(filepath.Join(p, "HEAD")); err != nil {
		return false
	}
	st, err := os.Stat(filepath.Join(p, "objects"))
	return err == nil && st.IsDir()
}
