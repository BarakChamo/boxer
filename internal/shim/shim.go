// Package shim writes PATH shims so a bare `npm` on the host becomes `boxer run` (§5.2, R-ENF).
package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// template removes the shim directory from PATH before handing off, so nothing boxer or smolvm
// run on the host (the smolvm launcher calls `uname`, for one) can resolve back into a shim.
const template = `#!/bin/sh
# boxer shim: %[1]s runs inside the sandbox for this worktree.
d=$(cd "$(dirname "$0")" && pwd)
set -f; new=; IFS=:; for p in $PATH; do [ "${p%%/}" = "$d" ] || new="${new:+$new:}$p"; done; unset IFS; set +f
PATH=$new
export PATH
BOXER_SHIM=1 exec boxer run -- %[1]s "$@"
`

// Marker names a directory as boxer's own shims. boxer drops any PATH entry containing it before
// spawning anything, because a shim it resolves itself is a shim that calls back into boxer: the
// smolvm launcher runs `uname`, and with a `uname` shim on PATH that became `boxer run -- uname`
// inside a boxer that was already working on the same sandbox, which deadlocked until it was
// killed. The shim script strips its own directory for the program it hands off to, which does not
// help any other process boxer starts.
const Marker = ".boxer-shims"

// SanitizePath removes every boxer shim directory from a PATH value.
func SanitizePath(path string) string {
	var keep []string
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		if isShimDir(dir) {
			continue
		}
		keep = append(keep, dir)
	}
	return strings.Join(keep, string(filepath.ListSeparator))
}

// Install writes one shim per program into dir and returns the paths written.
//
// A few names are never shimmed however the intercept list is written. `boxer` and `smolvm` are
// how a shim does its work, so shimming either is a loop. `sh` and `bash` are worse than a loop:
// they are the interpreter behind every `#!/usr/bin/env bash` script on the host, including
// smolvm's own launcher, so a shim on them puts an unrelated tool inside a sandbox that has no
// idea what to do with it and hangs. That is not hypothetical — one such invocation was found
// still running two days later. An interactive guest shell is `boxer-bash` (InstallShell) for
// exactly this reason; the wildcard `*` is a policy for the hook, not a list of programs.
func Install(dir string, programs []string) ([]string, error) {
	if err := mark(dir, Marker); err != nil {
		return nil, err
	}
	var written []string
	for _, p := range programs {
		if p == "*" || p == "boxer" || p == "smolvm" || p == "sh" || p == "bash" || filepath.Base(p) != p {
			continue
		}
		path := filepath.Join(dir, p)
		if err := os.WriteFile(path, []byte(fmt.Sprintf(template, p)), 0o755); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	// A program taken out of the list keeps its old shim otherwise, and taking `node` out is what
	// the docs advise when a node-based harness breaks. Only a file that is exactly boxer's shim
	// for its own name goes; anything else in the directory is someone's.
	keep := map[string]bool{}
	for _, w := range written {
		keep[filepath.Base(w)] = true
	}
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			name := e.Name()
			if keep[name] || name == Marker || e.IsDir() {
				continue
			}
			if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil && string(b) == fmt.Sprintf(template, name) {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
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
set -f; new=; IFS=:; for p in $PATH; do [ "${p%%/}" = "$d" ] || new="${new:+$new:}$p"; done; unset IFS; set +f
PATH=$new
export PATH
exec boxer shell %[1]s -- "$@"
`

// InstallHarness writes shims named after harness binaries that exec `boxer shell <name>`, so an
// orchestrator that spawns the harness by name lands inside the guest.
func InstallHarness(dir string, names []string) ([]string, error) {
	if err := mark(dir, HarnessMarker); err != nil {
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
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "boxer", "shims")
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		return filepath.Join(home, ".local", "share", "boxer", "shims")
	}
	return filepath.Join(os.TempDir(), "boxer-"+strconv.Itoa(os.Getuid()), "shims")
}

// shellTemplate is a bash stand-in: an interactive shell in the guest for the current worktree.
// --tty is what makes it a shell rather than a pipe: a harness that substitutes its terminal's
// shell connects over pipes, and without a terminal bash never prints a prompt — so a harness that
// finds the end of a command's output by looking for the prompt waits for one that never comes.
// --noediting turns readline off, which is what keeps that stream readable: with it on, a terminal
// emits bracketed-paste markers and echoes each command back, and a harness parsing the output
// reads its own input as the command's result and loops.
//
// PROMPT_COMMAND is carried across because that is how a harness driving a terminal installs its
// prompt: OpenHands sets PROMPT_COMMAND to export a PS1 carrying a metadata block, and reads the
// command's exit code back out of it. boxer does not forward host environment into the guest, so
// without this the guest shell keeps the image's default prompt and the harness sees no metadata
// at all and waits for output that never ends.
//
// PS1 and PS2 are deliberately absent: a non-interactive shell strips both from the environment,
// so a wrapper script cannot observe them however it is written. PROMPT_COMMAND survives and
// reassigns PS1 at every prompt, which is the mechanism the harness relies on anyway. Each
// variable is expanded only when set, so an unset one does not arrive as an empty one.
//
// --norc is the other half of carrying PS1: an interactive bash reads the image's rc files, and a
// stock Debian bashrc assigns PS1 unconditionally, so the harness's prompt is overwritten a moment
// after it arrives. Without rc files the environment's prompt is the one that survives.
// Any harness that lets you name its shell binary (OpenHands' terminal tool, for one) runs every
// command in the sandbox with no other integration. boxer run passes the PTY through.
const shellTemplate = `#!/bin/sh
# boxer shell wrapper: the sandbox's bash, for harnesses with a configurable shell path.
export PATH
exec boxer run --tty -- env \
  ${PROMPT_COMMAND+PROMPT_COMMAND="$PROMPT_COMMAND"} ${TERM+TERM="$TERM"} \
  ${GIT_PAGER+GIT_PAGER="$GIT_PAGER"} ${PAGER+PAGER="$PAGER"} \
  bash --noediting --norc "$@"
`

// InstallShell writes dir/boxer-bash and returns its path.
func InstallShell(dir string) (string, error) {
	if err := mark(dir, HarnessMarker); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "boxer-bash")
	return path, os.WriteFile(path, []byte(shellTemplate), 0o755)
}

// HarnessMarker names a directory of harness shims or boxer-bash. It is a marker of its own: the
// hook reads Marker as "the program shims are on PATH" before letting a block-only harness defer
// to them, and a directory of harness shims holds none of those.
const HarnessMarker = ".boxer-harness-shims"

// mark creates dir and names it as boxer's: SanitizePath drops a directory with either marker from
// boxer's own PATH, and harness shims and boxer-bash had none.
func mark(dir, marker string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, marker), []byte("boxer shims; boxer removes this directory from its own PATH\n"), 0o644)
}

func isShimDir(dir string) bool {
	for _, m := range []string{Marker, HarnessMarker} {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}
