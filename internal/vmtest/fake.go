// Package vmtest provides a fake smolvm for tests in every package.
package vmtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Script is a shell stand-in for smolvm. It records every invocation to $FAKE_LOG, keeps one
// machine's name and state in $FAKE_STATE, and runs `exec` commands locally with sh so tests
// observe real exit codes and output.
const Script = `#!/bin/sh
echo "$@" >> "$FAKE_LOG"
state_file="$FAKE_STATE"
case "$1 $2" in
  "machine ls")
    if [ -f "$state_file" ]; then
      name=$(sed -n 1p "$state_file"); state=$(sed -n 2p "$state_file"); root=$(sed -n 3p "$state_file")
      printf '[{"name":"%s","state":"%s","image":"alpine","labels":{"boxer.scope":"%s","boxer.root":"%s","boxer.isolation":"worktree"},"created_at":1,"pid":null}]\n' "$name" "$state" "$name" "$root"
    else
      echo "[]"
    fi ;;
  "machine status")
    if [ -f "$state_file" ]; then
      name=$(sed -n 1p "$state_file"); state=$(sed -n 2p "$state_file")
      printf '{"name":"%s","state":"%s","image":"alpine","labels":{"boxer.scope":"%s"},"created_at":1}\n' "$name" "$state" "$name"
    else
      echo "Error: config operation failed: machine status: machine not found" >&2; exit 1
    fi ;;
  "machine create")
    shift 2; name=""; root=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -n) name="$2"; shift;;
        --label) case "$2" in boxer.root=*) root="${2#boxer.root=}";; esac; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; shift;;
      esac; shift
    done
    printf '%s\nstopped\n%s\n' "$name" "$root" > "$state_file" ;;
  "pack create")
    shift 2; out=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -o) out="$2"; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; shift;;
      esac; shift
    done
    : > "$out"; : > "$out.smolmachine" ;;
  "machine start")
    name=$(sed -n 1p "$state_file"); root=$(sed -n 3p "$state_file"); printf '%s\nrunning\n%s\n' "$name" "$root" > "$state_file" ;;
  "machine stop")
    name=$(sed -n 1p "$state_file"); root=$(sed -n 3p "$state_file"); printf '%s\nstopped\n%s\n' "$name" "$root" > "$state_file" ;;
  "machine delete")
    rm -f "$state_file" "$state_file.setup" ;;
  "machine exec")
    shift 2; while [ $# -gt 0 ]; do case "$1" in --) shift; break;; -w|--name|-e) shift;; esac; shift; done
    if [ "$1" = "sh" ]; then
      case "$*" in
        *"test -f /var/lib/boxer/setup-done"*) [ -f "$FAKE_STATE.setup" ] && exit 0 || exit 1;;
        *"touch /var/lib/boxer/setup-done"*) touch "$FAKE_STATE.setup"; exit 0;;
      esac
    fi
    exec "$@" ;;
  "--version"*)
    echo "smolvm 0.0.0-fake" ;;
esac
`

// Install writes the fake, points BOXER_SMOLVM at it, and returns a client plus the log path.
func Install(t *testing.T) (vm.Client, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "smolvm")
	if err := os.WriteFile(bin, []byte(Script), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "log")
	t.Setenv("FAKE_LOG", log)
	t.Setenv("FAKE_STATE", filepath.Join(dir, "state"))
	t.Setenv("BOXER_SMOLVM", bin)
	return vm.Client{Bin: bin}, log
}
