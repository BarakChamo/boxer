// Package vmtest provides a fake smolvm for tests in every package.
package vmtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Script is a shell stand-in for smolvm. It records every invocation to $FAKE_LOG, keeps one file
// per machine under $FAKE_STATE.d, and runs `exec` commands locally with sh so tests observe real
// exit codes and output.
//
// Three things make it more than a stub, because the behaviour boxer needs tested lives in the
// differences between machines and in what happens when smolvm refuses: it models any number of
// machines (so `gc`, `down --all` and pack pruning have something to sweep), its image and version
// are settable (so the image detector and `doctor` can be observed), and any verb can be made to
// fail (so error paths are reachable without a real VM).
//
// Each machine file holds four lines: state, worktree root, image, pack reference.
const Script = `#!/bin/sh
echo "$@" >> "$FAKE_LOG"
dir="$FAKE_STATE.d"; mkdir -p "$dir"
image="${FAKE_IMAGE:-alpine}"
verb="$1 $2"
case "$1" in --version) verb="--version";; esac
faildir="$FAKE_STATE.fail"
fail="$faildir/$(echo "$verb" | tr ' /-' '___')"
if [ -f "$fail" ]; then echo "Error: $(cat "$fail")" >&2; exit 1; fi

machine_json() {
  n=$(basename "$1"); s=$(sed -n 1p "$1"); root=$(sed -n 2p "$1"); img=$(sed -n 3p "$1"); pack=$(sed -n 4p "$1")
  printf '{"name":"%s","state":"%s","image":"%s","labels":{"boxer.scope":"%s","boxer.root":"%s","boxer.isolation":"worktree","boxer.pack":"%s"},"created_at":1}' \
    "$n" "$s" "$img" "$n" "$root" "$pack"
}

case "$verb" in
  "machine ls")
    sep=""; printf '['
    for f in "$dir"/*; do
      [ -f "$f" ] || continue
      printf '%s' "$sep"; machine_json "$f"; sep=","
    done
    printf ']\n' ;;
  "machine status")
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    if [ -f "$dir/$name" ]; then machine_json "$dir/$name"; echo
    else echo "Error: config operation failed: machine status: machine not found" >&2; exit 1; fi ;;
  "machine create")
    shift 2; name=""; root=""; img="$image"; pack=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -n|--name) name="$2"; shift;;
        --label) case "$2" in boxer.root=*) root="${2#boxer.root=}";; boxer.pack=*) pack="${2#boxer.pack=}";; esac; shift;;
        --from) pack="$2"; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; [ -n "$2" ] && img="$2"; shift;;
      esac; shift
    done
    [ -f "$dir/$name" ] && { echo "Error: config operation failed: create machine: machine '$name' already exists or is being created" >&2; exit 1; }
    # smolvm reads the pack's footer, so a file that is not a whole pack fails here and nowhere
    # earlier: the same shape as a truncated pack on a real host.
    if [ -n "$pack" ] && [ -f "$pack" ] && [ "$(cat "$pack")" != "fake-pack" ]; then
      echo "Error: agent operation failed: read checkpoint footer: I/O error: sidecar file too small to contain footer" >&2; exit 1
    fi
    printf '%s\n%s\n%s\n%s\n' "stopped" "$root" "$img" "$pack" > "$dir/$name" ;;
  "pack create")
    shift 2; out=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -o) out="$2"; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; shift;;
      esac; shift
    done
    # A pack has a body: boxer treats an empty file as the truncation an interrupted
    # pack create leaves behind, and refuses to build a machine from it.
    printf 'fake-pack\n' > "$out"; printf 'fake-pack\n' > "$out.smolmachine" ;;
  "machine start"|"machine stop")
    state=running; [ "$2" = stop ] && state=stopped
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    [ -f "$dir/$name" ] || { echo "Error: machine not found" >&2; exit 1; }
    sed "1s/.*/$state/" "$dir/$name" > "$dir/$name.tmp" && mv "$dir/$name.tmp" "$dir/$name" ;;
  "machine delete")
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    rm -f "$dir/$name" "$dir/$name.setup" ;;
  "machine exec")
    name=""; shift 2
    while [ $# -gt 0 ]; do
      case "$1" in --) shift; break;; --name) name="$2"; shift;; -w|-e) shift;; esac; shift
    done
    if [ -f "$FAKE_STATE.flaky" ]; then
      # Drop the transport once for the first exec whose argv contains the marker's text.
      case "$*" in *"$(cat "$FAKE_STATE.flaky")"*) rm -f "$FAKE_STATE.flaky"; echo "Error: connection closed" >&2; exit 1;; esac
    fi
    if [ "$1" = "sh" ]; then
      case "$*" in
        *"test -f /var/lib/boxer/setup-done"*) [ -f "$dir/$name.setup" ] && exit 0 || exit 1;;
        *"touch /var/lib/boxer/setup-done"*) touch "$dir/$name.setup"; exit 0;;
        *"touch /var/lib/boxer/harness-"*) exit 0;;
      esac
    fi
    exec "$@" ;;
  "--version")
    echo "${FAKE_VERSION:-smolvm 0.0.0-fake}" ;;
esac
`

// FailExecOnce makes the next exec whose argv contains text fail with smolvm's "connection closed".
func FailExecOnce(t *testing.T, text string) {
	t.Helper()
	if err := os.WriteFile(os.Getenv("FAKE_STATE")+".flaky", []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// FailVerb makes every later invocation of one smolvm verb ("machine stop", "pack create",
// "--version") fail with message, until StopFailing. Error paths are most of what a VM layer has
// to get right and none of them are reachable from a fake that always succeeds.
func FailVerb(t *testing.T, verb, message string) {
	t.Helper()
	dir := os.Getenv("FAKE_STATE") + ".fail"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, verbFile(verb)), []byte(message), 0o644); err != nil {
		t.Fatal(err)
	}
}

// StopFailing undoes FailVerb.
func StopFailing(t *testing.T, verb string) {
	t.Helper()
	if err := os.Remove(filepath.Join(os.Getenv("FAKE_STATE")+".fail", verbFile(verb))); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// verbFile matches the shell's `tr ' /-' '___'`.
func verbFile(verb string) string {
	out := []byte(verb)
	for i, c := range out {
		if c == ' ' || c == '/' || c == '-' {
			out[i] = '_'
		}
	}
	return string(out)
}

// SetImage changes the image the fake reports for machines created without an explicit one.
func SetImage(t *testing.T, image string) { t.Helper(); t.Setenv("FAKE_IMAGE", image) }

// SetVersion changes what `smolvm --version` prints.
func SetVersion(t *testing.T, v string) { t.Helper(); t.Setenv("FAKE_VERSION", v) }

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

// Repo makes a git repository in a temporary directory, writes boxer.toml, and isolates the
// process from the developer's own configuration and state: XDG_CONFIG_HOME so no user boxer.toml
// leaks in, XDG_STATE_HOME so locks, last-used stamps and image packs stay inside the test. It
// returns the symlink-resolved path, because macOS temporary directories are symlinks and git
// reports the resolved form.
//
// The TOML is written exactly as given: a fixture that quietly adds configuration would change
// what a test means. Most callers want NoWorktreeCheck, because a temporary directory is a main
// checkout and would otherwise warn.
//
// Every test that needs a repository uses this; when the isolation has to change, it changes here.
// NoWorktreeCheck is the configuration most tests want: a temporary directory is a main checkout,
// not a linked worktree, and boxer warns about that by default.
const NoWorktreeCheck = "require_worktree = \"off\"\n"

func Repo(t *testing.T, toml string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// RepoIn is Repo for a subject that reads the process working directory rather than taking a path.
func RepoIn(t *testing.T, toml string) string {
	t.Helper()
	dir := Repo(t, toml)
	t.Chdir(dir)
	return dir
}
