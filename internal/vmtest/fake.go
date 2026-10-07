// Package vmtest provides a fake smolvm for tests in every package.
package vmtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
if [ -f "$fail" ]; then
  # smolvm writes a pack in place, so one that fails partway leaves a truncated file behind.
  if [ "$verb" = "pack create" ]; then
    prev=""; for a in "$@"; do [ "$prev" = "-o" ] && printf 'fake-pa' > "$a.smolmachine"; prev="$a"; done
  fi
  echo "Error: $(cat "$fail")" >&2; exit 1
fi
# A verb that fails a fixed number of times and then succeeds, for the retry paths. smolvm keeps
# its machine records in SQLite, so concurrent creates lose a race with "database is locked"; a
# fake that can only fail forever cannot tell a caller that retries from one that gives up.
lockf="$faildir/lock_$(echo "$verb" | tr ' /-' '___')"
if [ -f "$lockf" ]; then
  n=$(cat "$lockf")
  if [ "$n" -gt 0 ]; then
    echo $((n-1)) > "$lockf"
    echo "Error: database operation failed: reserve vm '$3': database is locked" >&2
    exit 1
  fi
fi

machine_json() {
  n=$(basename "$1"); s=$(sed -n 1p "$1"); img=$(sed -n 3p "$1"); labels=$(sed -n 5p "$1")
  # Labels are whatever was passed at create time, as smolvm does it: a fake that models a fixed
  # set silently drops every label added later, and the test that checks for one proves nothing.
  # No default label: a machine created without one has none, and boxer must not treat a machine
  # it did not create as its own. Synthesising a label here would hide exactly that bug.
  printf '{"name":"%s","state":"%s","image":"%s","labels":{%s},"created_at":1,"pid":%s,"cpus":2,"memory_mib":1024}' \
    "$n" "$s" "$img" "$labels" "$$"
}

case "$verb" in
  "machine ls")
    sep=""; printf '['
    for f in "$dir"/*; do
      [ -f "$f" ] || continue
      # Sidecars (.setup, .started, .harness-*, .branchable) record guest state for a machine that
      # is already listed. A machine name never contains a dot, so this is what tells them apart:
      # without the guard, ls and gc would see machines that do not exist.
      case "${f##*/}" in *.*) continue;; esac
      printf '%s' "$sep"; machine_json "$f"; sep=","
    done
    printf ']\n' ;;
  "machine status")
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    if [ -f "$dir/$name" ]; then machine_json "$dir/$name"; echo
    else echo "Error: config operation failed: machine status: machine not found" >&2; exit 1; fi ;;
  "machine create")
    shift 2; name=""; root=""; img="$image"; pack=""; labels=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -n|--name) name="$2"; shift;;
        --label)
          case "$2" in boxer.root=*) root="${2#boxer.root=}";; boxer.pack=*) pack="${2#boxer.pack=}";; esac
          k=${2%%=*}; v=${2#*=}
          labels="$labels${labels:+,}\"$k\":\"$v\""
          shift;;
        --from) pack="$2"; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; [ -n "$2" ] && img="$2"; shift;;
      esac; shift
    done
    [ -f "$dir/$name" ] && { echo "Error: config operation failed: create machine: machine '$name' already exists or is being created" >&2; exit 1; }
    # smolvm reads the pack's footer, so a file that is not a whole pack fails here and nowhere
    # earlier: the same shape as a truncated pack on a real host.
    if [ -n "$pack" ] && [ -f "$pack" ] && [ "$(head -n 1 "$pack")" != "fake-pack" ]; then
      echo "Error: agent operation failed: read checkpoint footer: I/O error: sidecar file too small to contain footer" >&2; exit 1
    fi
    printf '%s\n%s\n%s\n%s\n%s\n' "stopped" "$root" "$img" "$pack" "$labels" > "$dir/$name"
    # Restore the guest state the pack carries, so a VM created from it skips what it already has.
    if [ -n "$pack" ] && [ -f "$pack" ]; then
      id=$(sed -n 2p "$pack")
      for m in "$dir/pack-$id".marker-*; do
        [ -n "$id" ] && [ -f "$m" ] || continue
        cp "$m" "$dir/$name.$(basename "$m" | sed 's/^.*\.marker-//')"
      done
    fi ;;
  "pack create")
    shift 2; out=""; fromvm=""
    while [ $# -gt 0 ]; do
      case "$1" in
        -o) out="$2"; shift;;
        --from-vm) fromvm="$2"; shift;;
        -I) [ "$2" = "fail-image" ] && { echo "Error: image pull failed" >&2; exit 1; }; shift;;
      esac; shift
    done
    # A pack has a body: boxer treats an empty file as the truncation an interrupted
    # pack create leaves behind, and refuses to build a machine from it.
    # The second line names the guest state this pack carries, kept in the fake's own state: the
    # pack is written somewhere temporary and moved, so nothing may live beside it.
    id=$(printf '%s' "$out" | cksum | cut -d' ' -f1)
    printf 'fake-pack\n' > "$out"; printf 'fake-pack\n%s\n' "$id" > "$out.smolmachine"
    # A pack of a machine carries that machine's guest state. The markers are what decide whether
    # setup and the harness install run again, so a pack that loses them is not a pack: every VM
    # made from it would repeat the work the pack exists to skip.
    rm -f "$dir/pack-$id".marker-*
    if [ -n "$fromvm" ]; then
      for m in "$dir/$fromvm".setup "$dir/$fromvm".harness-*; do
        [ -f "$m" ] || continue
        cp "$m" "$dir/pack-$id.marker-$(basename "$m" | sed "s/^$fromvm\.//")"
      done
    fi ;;
  "machine start"|"machine stop")
    state=running; [ "$2" = stop ] && state=stopped
    # A stop empties the guest's tmpfs, so the services have to be started again.
    name=""; branchable=""; shift 2
    while [ $# -gt 0 ]; do
      case "$1" in -n|--name) name="$2"; shift;; --branchable) branchable=1;; esac; shift
    done
    [ -f "$dir/$name" ] || { echo "Error: machine not found" >&2; exit 1; }
    # Branchability is a property of the current boot: a stop takes it away, exactly as the memfd
    # and the control socket go away with the process.
    [ "$state" = stopped ] && rm -f "$dir/$name.started" "$dir/$name.branchable"
    [ -n "$branchable" ] && touch "$dir/$name.branchable"
    sed "1s/.*/$state/" "$dir/$name" > "$dir/$name.tmp" && mv "$dir/$name.tmp" "$dir/$name" ;;
  "machine delete")
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    # --cascade takes the children branched from it, as the real one does.
    rm -f "$dir/$name" "$dir/$name".* ;;
  "machine branch")
    shift 2; from=""; child=""; batch=""
    while [ $# -gt 0 ]; do
      case "$1" in
        --from) from="$2"; shift;;
        -n|--name) child="$2"; shift;;
        # A prefix or a count makes it a batch branch, which the real smolvm answers only after
        # the source workload runs smolvm-branch-ready. boxer never asks for one; the fake
        # refuses it so that a change which starts asking fails here rather than hanging for ten
        # minutes against a real VM.
        --name-prefix|--count) batch=1; shift;;
        -e|--secret-env|-p|--ready-timeout) shift;;
      esac; shift
    done
    [ -f "$dir/$from" ] || { echo "Error: machine not found" >&2; exit 1; }
    [ -f "$dir/$from.branchable" ] || { echo "Error: machine is not branchable" >&2; exit 1; }
    [ -n "$batch" ] && { echo "Error: wait for forkpoint: the workload did not reach a branchpoint" >&2; exit 1; }
    [ -n "$child" ] || { echo "Error: no child name" >&2; exit 1; }
    # A branch copies the source's disk and RAM, so the child starts running with the guest state
    # the parent had: the markers travel or the child would repeat setup.
    sed "1s/.*/running/" "$dir/$from" > "$dir/$child"
    for m in "$dir/$from".setup "$dir/$from".harness-* "$dir/$from".started; do
      [ -f "$m" ] || continue
      cp "$m" "$dir/$child.$(basename "$m" | sed "s/^$from\.//")"
    done
    echo "$child" ;;
  "machine sync")
    name=""; shift 2
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    [ -f "$dir/$name" ] || { echo "Error: machine not found" >&2; exit 1; }
    touch "$dir/$name.synced" ;;
  "machine update")
    name=""; shift 2
    add=""; while [ $# -gt 0 ]; do
      case "$1" in -n|--name) name="$2"; shift;; -v) add="$2"; shift;; --remove-volume) shift;; esac; shift
    done
    [ -f "$dir/$name" ] || { echo "Error: machine not found" >&2; exit 1; }
    # The real one refuses on a running machine, and boxer has to stop first because of it.
    [ "$(sed -n 1p "$dir/$name")" = running ] && { echo "Error: machine is running" >&2; exit 1; }
    [ -n "$add" ] && echo "$add" > "$dir/$name.volume"
    : ;;
  "machine exec")
    # KillExecOnce: the CLI itself dies by a signal, as when it is interrupted or killed.
    [ -f "$FAKE_STATE.killexec" ] && { rm -f "$FAKE_STATE.killexec"; kill -KILL $$; }
    name=""; secrets=""; shift 2
    while [ $# -gt 0 ]; do
      case "$1" in
        --) shift; break;;
        --name) name="$2"; shift;;
        # A secret is passed by name, never by value: the test that matters reads $FAKE_LOG and
        # asserts the value is absent from it, so the fake has to resolve the name the way smolvm
        # does rather than merely tolerate the flag.
        --secret-env) secrets="$secrets $2"; shift;;
        -w|-e|--timeout|--secret-file|-u) shift;;
      esac
      shift
    done
    for s in $secrets; do
      g=${s%%=*}; h=${s#*=}
      eval "v=\$$h"; eval "export $g=\"\$v\""
    done
    if [ -f "$FAKE_STATE.flaky" ]; then
      # Drop the transport once for the first exec whose argv contains the marker's text.
      case "$*" in *"$(cat "$FAKE_STATE.flaky")"*) rm -f "$FAKE_STATE.flaky"; echo "Error: connection closed" >&2; exit 1;; esac
    fi
    if [ "$1" = "sh" ]; then
      case "$*" in
        # Guest marker files decide whether setup and the harness install run again, so the fake
        # models them per machine: without that, "once per VM" cannot be tested at all.
        *"test -f /var/lib/boxer/image-setup-done"*) [ -f "$dir/$name.setup" ] && exit 0 || exit 1;;
        # The start marker lives in the guest's memory-backed /tmp: it must not survive a restart,
        # and it must never touch the host, which is where an unmodelled marker would land.
        *'cat /tmp/boxer-started'*) [ -f "$dir/$name.started" ] && exit 0 || exit 1;;
        *"> /tmp/boxer-started"*) touch "$dir/$name.started"; exit 0;;
        # Stopping services kills guest processes by pid file; on the host that would be a kill
        # of whatever those pids are here. Modelled: the marker goes, nothing is signalled.
        *"/tmp/boxer-svc/*.pid"*) rm -f "$dir/$name.started"; [ -f "$FAKE_STATE.stuck" ] && echo stuck; exit 0;;
        *"rm -rf /tmp/boxer-svc"*) rm -f "$dir/$name.started"; exit 0;;
        *"touch /var/lib/boxer/image-setup-done"*) touch "$dir/$name.setup"; exit 0;;
        *"test -f /var/lib/boxer/harness-"*)
          h=${*##*harness-}; [ -f "$dir/$name.harness-${h%% *}" ] && exit 0 || exit 1;;
        *"touch /var/lib/boxer/harness-"*)
          h=${*##*harness-}; touch "$dir/$name.harness-${h%% *}"; exit 0;;
      esac
    fi
    # The mount-readiness probe. The fake ignores -w and runs in boxer's own directory, so it
    # answers for the guest instead: its mount is always the live worktree.
    case "$*" in *"boxer-mount-probe"*) exit 0;; esac
    # The user-write probe, likewise, and it must never reach the host: past it is an edit to
    # /etc/passwd, and the fake runs guest commands on the host.
    # FAKE_USER_OWNER makes the probe report that the user cannot write, and that the worktree is
    # owned by this uid, so each branch of boxer's answer can be tested.
    case "$*" in *"boxer-user-probe"*) [ -n "$FAKE_USER_OWNER" ] && { echo "$FAKE_USER_OWNER"; exit 1; }; exit 0;; esac
    case "$*" in *"/etc/passwd"*) exit 0;; esac
    exec "$@" ;;
  "machine egress-events")
    # Egress denials are the only record of why an allowlisted guest could not reach a host: the
    # guest itself sees a DNS or connect error and nothing names the cause.
    if [ -f "$FAKE_STATE.egress" ]; then cat "$FAKE_STATE.egress"; else echo "[]"; fi ;;
  "machine data-dir")
    # Real smolvm prints where a machine's disks live; the fake points at its own state, which is
    # small but real, so a resource measurement has something honest to walk.
    shift 2; name=""
    while [ $# -gt 0 ]; do case "$1" in -n|--name) name="$2"; shift;; esac; shift; done
    [ -f "$dir/$name" ] || { echo "Error: machine not found" >&2; exit 1; }
    printf '%s\n' "$dir" ;;
  "--version")
    echo "${FAKE_VERSION:-smolvm 0.0.0-fake}" ;;
esac
`

// KillExecOnce makes the next `machine exec` end by SIGKILL before it reports anything.
func KillExecOnce(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(os.Getenv("FAKE_STATE")+".killexec", nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

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
// LockVerb makes the next n calls to verb fail the way smolvm's store does under concurrent use,
// after which the verb succeeds. Nothing may treat that error as a statement about the request:
// boxer once read it as a corrupt environment pack and deleted the pack.
func LockVerb(t *testing.T, verb string, n int) {
	t.Helper()
	dir := os.Getenv("FAKE_STATE") + ".fail"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "lock_"+verbFile(verb))
	if err := os.WriteFile(p, []byte(strconv.Itoa(n)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(p) })
}

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
	// A test repository is the operator's own, so its host-affecting keys are trusted; a test of
	// the trust gate itself sets BOXER_TRUST back to "".
	t.Setenv("BOXER_TRUST", "1")
	// A fake machine must never write to the developer's own state. Without this, a test that
	// installed the fake but made no repository (a backend probe, say) cached `fake-pack` into
	// the real pack directory, where a real create would later find it. A test that already
	// pointed these at its own temporary directory keeps it.
	for _, name := range []string{"XDG_STATE_HOME", "BOXER_PACKS"} {
		if v := os.Getenv(name); v == "" || !strings.HasPrefix(v, os.TempDir()) {
			t.Setenv(name, t.TempDir())
		}
	}
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

// SetEgress makes the fake report these egress events, as smolvm's `machine egress-events --json`
// would. The argument is the JSON array verbatim.
func SetEgress(t *testing.T, jsonArray string) {
	t.Helper()
	if err := os.WriteFile(os.Getenv("FAKE_STATE")+".egress", []byte(jsonArray), 0o644); err != nil {
		t.Fatal(err)
	}
}
