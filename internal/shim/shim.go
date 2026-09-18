// Package shim writes PATH shims so a bare `npm` on the host becomes `boxer run` (§5.2, R-ENF).
package shim

import (
	"fmt"
	"os"
	"path/filepath"
)

// template removes the shim directory from PATH before handing off, so nothing boxer or smolvm
// run on the host (the smolvm launcher calls `uname`, for one) can resolve back into a shim.
const template = `#!/bin/sh
# boxer shim: %[1]s runs inside the sandbox for this worktree.
d=$(cd "$(dirname "$0")" && pwd)
new=; IFS=:; for p in $PATH; do [ "$p" = "$d" ] || new="${new:+$new:}$p"; done; unset IFS
PATH=$new
export PATH
exec boxer run -- %[1]s "$@"
`

// Install writes one shim per program into dir and returns the paths written.
func Install(dir string, programs []string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, p := range programs {
		if p == "*" || p == "boxer" || p == "smolvm" || p == "sh" || filepath.Base(p) != p {
			continue
		}
		path := filepath.Join(dir, p)
		if err := os.WriteFile(path, []byte(fmt.Sprintf(template, p)), 0o755); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// harnessTemplate also declares the agent kind for a multiplexer that classifies a pane by the
// program it started: the wrapper replaces the agent binary, so the kind is announced rather than
// inferred. herdr 0.9.1 classifies panes from the screen buffer instead and ignores this, so it
// costs one line and is right when a pane manager asks for it.
const harnessTemplate = `#!/bin/sh
# boxer harness shim: %[1]s runs inside the sandbox for this worktree (boxer shell).
HERDR_AGENT=%[1]s
export HERDR_AGENT
d=$(cd "$(dirname "$0")" && pwd)
new=; IFS=:; for p in $PATH; do [ "$p" = "$d" ] || new="${new:+$new:}$p"; done; unset IFS
PATH=$new
export PATH
exec boxer shell %[1]s -- "$@"
`

// InstallHarness writes shims named after harness binaries that exec `boxer shell <name>`, so an
// orchestrator that spawns the harness by name lands inside the guest.
func InstallHarness(dir string, names []string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, n := range names {
		path := filepath.Join(dir, n)
		if err := os.WriteFile(path, []byte(fmt.Sprintf(harnessTemplate, n)), 0o755); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// DefaultDir is where `boxer shim install` writes without an argument.
func DefaultDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "boxer", "shims")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "boxer", "shims")
}

// shellTemplate is a bash stand-in: an interactive shell in the guest for the current worktree.
// Any harness that lets you name its shell binary (OpenHands' terminal tool, for one) runs every
// command in the sandbox with no other integration. boxer run passes the PTY through.
const shellTemplate = `#!/bin/sh
# boxer shell wrapper: the sandbox's bash, for harnesses with a configurable shell path.
export PATH
exec boxer run -- bash "$@"
`

// InstallShell writes dir/boxer-bash and returns its path.
func InstallShell(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "boxer-bash")
	return path, os.WriteFile(path, []byte(shellTemplate), 0o755)
}
