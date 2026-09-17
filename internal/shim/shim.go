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

const harnessTemplate = `#!/bin/sh
# boxer harness shim: %[1]s runs inside the sandbox for this worktree (boxer shell).
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
