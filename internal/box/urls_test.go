package box

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// fakePortless behaves like portless 0.15 where boxer depends on it: `get` prefixes the name with
// $FAKE_PREFIX (the branch) and `alias` does not, and an alias refuses a name it already has
// unless --force. The state directory is a file per name.
const fakePortless = `#!/bin/sh
st="$FAKE_PORTLESS_STATE"; mkdir -p "$st"
echo "$@" >> "$st/.calls"
case "$1" in
get)   if [ -n "$FAKE_PREFIX" ]; then echo "https://$FAKE_PREFIX.$2.localhost:1355"; else echo "https://$2.localhost:1355"; fi ;;
alias) if [ "$2" = --remove ]; then rm -f "$st/$3"; exit 0; fi
       if [ -f "$st/$2" ] && [ "$4" != --force ]; then echo "\"$2\" is already registered" >&2; exit 1; fi
       echo "$3" > "$st/$2" ;;
proxy) exit 0 ;;
esac
`

func installFakePortless(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "portless")
	if err := os.WriteFile(bin, []byte(fakePortless), 0o755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	t.Setenv("BOXER_PORTLESS", bin)
	t.Setenv("FAKE_PORTLESS_STATE", state)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // no ~/.portless/proxy.pid, so the proxy is "started"
	return state
}

func urlEnv(t *testing.T, key, root string, linked bool, ports ...string) *Env {
	// portless runs in the worktree, so it has to exist.
	root = filepath.Join(t.TempDir(), filepath.Base(root))
	_ = os.MkdirAll(root, 0o755)
	cfg := config.Defaults()
	cfg.URLs.Enabled = true
	cfg.Network.Ports = ports
	return &Env{
		Cfg:    cfg,
		Git:    scope.Git{Toplevel: root, CommonDir: "/src/myapp/.git", Linked: linked},
		Scope:  scope.Scope{Key: key, Root: root},
		Stderr: &bytes.Buffer{},
	}
}

func machineWith(key string, ports map[string]string) vm.Machine {
	labels := map[string]string{}
	for g, h := range ports {
		labels[vm.LabelPrefix+"port."+g] = h
	}
	return vm.Machine{Name: key, Labels: labels}
}

// The cases that decide whether several worktrees actually work: each gets its own name, the
// name registered is the one reported, and nobody steals a name from anyone.
func TestURLNamesAreDistinctPerWorktree(t *testing.T) {
	state := installFakePortless(t)
	var w bytes.Buffer

	// The main checkout: portless gives no prefix, and none is needed.
	main := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000")
	got := main.publishURLs(machineWith(main.Scope.Key, map[string]string{"3000": "51001"}), &w)
	if got["3000"] != "https://myapp.localhost:1355" {
		t.Fatalf("main checkout: %v\n%s", got, w.String())
	}

	// A linked worktree on a branch: portless's own prefix, registered in full. Registering the
	// bare name `get` was asked for was the 404 this code exists to avoid.
	t.Setenv("FAKE_PREFIX", "fix-ui")
	branch := urlEnv(t, "sb-bbbbbbbbbbbb", "/src/wt-fix-ui", true, "auto:3000")
	got = branch.publishURLs(machineWith(branch.Scope.Key, map[string]string{"3000": "51002"}), &w)
	if got["3000"] != "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("branch worktree: %v\n%s", got, w.String())
	}
	if b, err := os.ReadFile(filepath.Join(state, "fix-ui.myapp")); err != nil || strings.TrimSpace(string(b)) != "51002" {
		t.Fatalf("the prefixed name must be the one aliased, pointing at this sandbox's port: %q %v", b, err)
	}

	// A detached worktree: portless gives no prefix, which would collide with the main checkout.
	t.Setenv("FAKE_PREFIX", "")
	det := urlEnv(t, "sb-cccccccccccc", "/src/wt-detached", true, "auto:3000")
	got = det.publishURLs(machineWith(det.Scope.Key, map[string]string{"3000": "51003"}), &w)
	want := "https://" + det.Scope.Slug() + ".myapp.localhost:1355"
	if got["3000"] != want {
		t.Fatalf("detached worktree: got %v, want %s", got, want)
	}

	// Two branches whose last segment matches get the same portless prefix; the second must not
	// take the first one's name.
	t.Setenv("FAKE_PREFIX", "fix-ui")
	twin := urlEnv(t, "sb-dddddddddddd", "/src/wt-other-fix-ui", true, "auto:3000")
	got = twin.publishURLs(machineWith(twin.Scope.Key, map[string]string{"3000": "51004"}), &w)
	if got["3000"] == "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("a second sandbox stole a name another one holds")
	}
	if b, _ := os.ReadFile(filepath.Join(state, "fix-ui.myapp")); strings.TrimSpace(string(b)) != "51002" {
		t.Fatalf("the first sandbox's route was re-pointed: %q", b)
	}

	// A name registered outside boxer is never taken either.
	t.Setenv("FAKE_PREFIX", "mine")
	if err := os.WriteFile(filepath.Join(state, "mine.myapp"), []byte("9999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := urlEnv(t, "sb-eeeeeeeeeeee", "/src/wt-mine", true, "auto:3000")
	got = other.publishURLs(machineWith(other.Scope.Key, map[string]string{"3000": "51005"}), &w)
	if got["3000"] != "https://"+other.Scope.Slug()+".myapp.localhost:1355" {
		t.Fatalf("a person's own route must be left alone: %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(state, "mine.myapp")); strings.TrimSpace(string(b)) != "9999" {
		t.Fatalf("boxer overwrote a route it did not create: %q", b)
	}

	// Restarting a sandbox re-points its own name rather than falling back to another.
	t.Setenv("FAKE_PREFIX", "fix-ui")
	got = branch.publishURLs(machineWith(branch.Scope.Key, map[string]string{"3000": "51012"}), &w)
	if got["3000"] != "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("a restart must keep the same URL: %v", got)
	}

	// status reads this without running portless.
	if URLsOf(branch.Scope.Key)["3000"] != "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("URLsOf: %v", URLsOf(branch.Scope.Key))
	}

	// Deleting a sandbox removes its routes and only its routes.
	ForgetScope(branch.Scope.Key)
	if _, err := os.Stat(filepath.Join(state, "fix-ui.myapp")); !os.IsNotExist(err) {
		t.Fatalf("the route outlived its sandbox")
	}
	if len(URLsOf(branch.Scope.Key)) != 0 {
		t.Fatalf("the registry outlived its sandbox")
	}
	if _, err := os.Stat(filepath.Join(state, "myapp")); err != nil {
		t.Fatalf("another sandbox's route went with it: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "mine.myapp")); err != nil {
		t.Fatalf("a person's route went with it: %v", err)
	}
}

// Several ports are named by label, or by number when unlabelled; a fixed port counts too.
func TestURLNamesForSeveralPorts(t *testing.T) {
	installFakePortless(t)
	e := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000", "8080:8080")
	e.Cfg.URLs.Names = map[string]string{"3000": "Web App"}
	var w bytes.Buffer
	got := e.publishURLs(machineWith(e.Scope.Key, map[string]string{"3000": "51001"}), &w)
	if got["3000"] != "https://web-app.myapp.localhost:1355" || got["8080"] != "https://8080.myapp.localhost:1355" {
		t.Fatalf("%v\n%s", got, w.String())
	}
}

// Off is off: no portless call, no registry, nothing printed.
func TestURLsOffRunsNothing(t *testing.T) {
	state := installFakePortless(t)
	e := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000")
	e.Cfg.URLs.Enabled = false
	var w bytes.Buffer
	if got := e.publishURLs(machineWith(e.Scope.Key, map[string]string{"3000": "51001"}), &w); got != nil || w.Len() != 0 {
		t.Fatalf("%v %q", got, w.String())
	}
	if _, err := os.Stat(filepath.Join(state, ".calls")); !os.IsNotExist(err) {
		t.Fatalf("portless was run with urls off")
	}
}

// Missing portless is a warning with a fix, never a failure.
func TestURLsWithoutPortlessWarns(t *testing.T) {
	t.Setenv("BOXER_PORTLESS", filepath.Join(t.TempDir(), "no-such-portless"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000")
	var w bytes.Buffer
	if got := e.publishURLs(machineWith(e.Scope.Key, map[string]string{"3000": "51001"}), &w); got != nil {
		t.Fatalf("%v", got)
	}
	if !strings.Contains(w.String(), "fix: npm install -g portless") {
		t.Fatalf("%q", w.String())
	}
}

// gc's sweep for routes whose sandbox vanished outside boxer: a dead port and no live sandbox is
// removed; a port something still listens on is kept, because it may be another backend's.
func TestPruneURLsKeepsRoutesWithAListener(t *testing.T) {
	state := installFakePortless(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, alive, _ := net.SplitHostPort(l.Addr().String())
	var w bytes.Buffer
	a := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000")
	a.publishURLs(machineWith(a.Scope.Key, map[string]string{"3000": alive}), &w)
	t.Setenv("FAKE_PREFIX", "gone")
	b := urlEnv(t, "sb-bbbbbbbbbbbb", "/src/wt-gone", true, "auto:3000")
	b.publishURLs(machineWith(b.Scope.Key, map[string]string{"3000": "1"}), &w) // nothing on port 1
	if n := PruneURLs(map[string]bool{}); n != 1 {
		t.Fatalf("pruned %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(state, "gone.myapp")); !os.IsNotExist(err) {
		t.Fatalf("a dead route survived")
	}
	if _, err := os.Stat(filepath.Join(state, "myapp")); err != nil {
		t.Fatalf("a route with a listener was removed: %v", err)
	}
}

// The state sweep removes what belongs to no live sandbox once it is old, keeps what is fresh or
// live, and never removes a lock someone holds.
func TestSweepStateBoundsTheStateDirectory(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := stateRoot()
	old := time.Now().Add(-8 * 24 * time.Hour)
	write := func(dir, name string, when time.Time) string {
		p := filepath.Join(root, dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
		_ = os.Chtimes(p, when, when)
		return p
	}
	gone := []string{
		write("last-used", "sb-111111111111", old),
		write("runs", "sb-111111111111.json", old),
		write("setup", "sb-111111111111-abcd1234", old),
		write("locks", "sb-111111111111", old),
	}
	kept := []string{
		write("setup", "sb-222222222222-abcd1234", old), // live
		write("setup", "sb-333333333333-abcd1234", time.Now()),
		write("runs", "notes.txt", old), // not boxer's shape: never touched
	}
	// Recently used, even though its marker is old: the stamp is the activity that counts.
	write("last-used", "sb-555555555555", time.Now())
	kept = append(kept, write("setup", "sb-555555555555-abcd1234", old))
	held := write("locks", "sb-444444444444", old)
	unlock, err := lockFile(held)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	SweepState(map[string]bool{"sb-222222222222": true}, time.Now())
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("stale state survived: %s", p)
		}
	}
	for _, p := range append(kept, held) {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("state that must be kept was removed: %s", p)
		}
	}
}

// Deleting a sandbox takes its machine state with it, and leaves the worktree's setup marker:
// `down` then `up` must not re-run setup into a worktree that is already set up.
func TestForgetScopeKeepsWorktreeState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := stateRoot()
	for _, p := range []string{"last-used/sb-111111111111", "runs/sb-111111111111.json", "locks/sb-111111111111",
		"branchable/sb-111111111111", "setup/sb-111111111111-abcd1234"} {
		_ = os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		_ = os.WriteFile(filepath.Join(root, p), nil, 0o644)
	}
	ForgetScope("sb-111111111111")
	for _, p := range []string{"last-used/sb-111111111111", "runs/sb-111111111111.json", "locks/sb-111111111111", "branchable/sb-111111111111"} {
		if _, err := os.Stat(filepath.Join(root, p)); !os.IsNotExist(err) {
			t.Errorf("machine state survived its sandbox: %s", p)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "setup/sb-111111111111-abcd1234")); err != nil {
		t.Errorf("the worktree's setup marker must survive the sandbox: %v", err)
	}
}

// The brief is what every harness shows the agent first, so it has to say where a server is —
// and must not call a container a microVM.
func TestBriefSaysWhereServersAre(t *testing.T) {
	cfg := config.Defaults()
	if b := Instructions(cfg); strings.Contains(b, "urls") || strings.Contains(b, "ports") {
		t.Fatalf("a sandbox that forwards nothing needs no sentence about ports: %s", b)
	}
	cfg.Network.Ports = []string{"auto:3000"}
	if b := Instructions(cfg); !strings.Contains(b, "`ports`") || strings.Contains(b, "`urls`") {
		t.Fatalf("ports without names: %s", b)
	}
	cfg.URLs.Enabled = true
	if b := Instructions(cfg); !strings.Contains(b, "`urls`") || !strings.Contains(b, "not localhost") {
		t.Fatalf("named ports: %s", b)
	}
	if b := Instructions(cfg); !strings.Contains(b, "a microVM per") {
		t.Fatalf("smolvm is a microVM: %s", b)
	}
	cfg.Backend = "docker"
	if b := Instructions(cfg); !strings.Contains(b, "a container per") || strings.Contains(b, "microVM") {
		t.Fatalf("docker is a container, and saying otherwise claims a boundary it lacks: %s", b)
	}
}

// A harness that strips XDG_STATE_HOME from the MCP server it launches gave boxer_status a
// different state directory from the one `boxer up` wrote the registry to, and status reported
// the port instead of the URL. The agent used the port: it did what it was told. portless's own
// route table is under $HOME, which harnesses keep, so a port the registry does not name is
// found there by host port.
func TestURLsAreFoundWithoutTheRegistry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // an empty registry: some other process's state dir
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".portless")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "routes.json"), []byte(`[{"hostname":"fix-ui.myapp.localhost","port":51002,"pid":0},{"hostname":"other.localhost","port":9,"pid":0}]`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "proxy.port"), []byte("1355\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "proxy.tls"), []byte("1"), 0o644)
	e := urlEnv(t, "sb-bbbbbbbbbbbb", "/src/wt", true, "auto:3000")
	m := machineWith(e.Scope.Key, map[string]string{"3000": "51002"})
	if got := e.URLs(m); got["3000"] != "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("URLs: %v", got)
	}
	if got := MachineURLs(m); got["3000"] != "https://fix-ui.myapp.localhost:1355" {
		t.Fatalf("MachineURLs: %v", got)
	}
	// A port with no route has no URL, rather than someone else's.
	if got := e.URLs(machineWith(e.Scope.Key, map[string]string{"3000": "51099"})); len(got) != 0 {
		t.Fatalf("a port with no route got a URL: %v", got)
	}
}

// Inside, the harness is in the guest: no `boxer` to run and no route to the host's proxy. The
// brief must not send it to either, and the environment must carry the host-side addresses.
func TestInsideBriefAndServerEnv(t *testing.T) {
	cfg := config.Defaults()
	cfg.Integration = "inside"
	cfg.Network.Ports = []string{"auto:3000"}
	cfg.URLs.Enabled = true
	b := Instructions(cfg)
	if strings.Contains(b, "boxer status") || !strings.Contains(b, "127.0.0.1:<its port>") || !strings.Contains(b, "$BOXER_URLS") {
		t.Fatalf("inside brief: %s", b)
	}
	installFakePortless(t)
	e := urlEnv(t, "sb-aaaaaaaaaaaa", "/src/myapp", false, "auto:3000", "8080:8080")
	m := machineWith(e.Scope.Key, map[string]string{"3000": "51001"})
	var w bytes.Buffer
	e.publishURLs(m, &w)
	env := strings.Join(e.ServerEnv(m), "\n")
	for _, want := range []string{"BOXER_PORTS=3000=51001 8080=8080", "BOXER_URLS=3000=https://3000.myapp.localhost:1355 8080=https://8080.myapp.localhost:1355"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %q in %q", want, env)
		}
	}
}

// An orchestrator that reuses a task name removes the worktree and adds a new one at the same path.
// The setup marker used to be keyed by that path in boxer's state, so the new worktree inherited
// "set up" with no node_modules in it. It lives in the worktree's git directory now, and goes
// with it — for a main checkout and for a linked worktree alike.
func TestSetupMarkerDiesWithTheWorktree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	base := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	main := filepath.Join(base, "main")
	_ = os.MkdirAll(main, 0o755)
	git(main, "init", "-q")
	git(main, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "i")
	cfg := config.Defaults()
	cfg.Setup = []string{"npm ci"}
	if m := setupMarkerPath(main, "sb-aaaaaaaaaaaa", cfg); !strings.HasPrefix(m, filepath.Join(main, ".git", "boxer")) {
		t.Fatalf("main checkout marker: %s", m)
	}
	wt := filepath.Join(base, "wt")
	git(main, "worktree", "add", "-q", "-b", "task", wt)
	m := setupMarkerPath(wt, "sb-bbbbbbbbbbbb", cfg)
	_ = os.MkdirAll(filepath.Dir(m), 0o755)
	if err := os.WriteFile(m, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git(main, "worktree", "remove", "--force", wt)
	git(main, "worktree", "add", "-q", "-b", "task2", wt) // the same path, a new worktree
	if _, err := os.Stat(setupMarkerPath(wt, "sb-bbbbbbbbbbbb", cfg)); !os.IsNotExist(err) {
		t.Fatalf("a recreated worktree inherited the old one's setup marker")
	}
}

// One retired hostname on the allowlist made smolvm refuse to create any sandbox that listed it.
// A host that does not resolve is left out, by name; wildcards and addresses are not looked up.
func TestUnresolvableAllowlistHostsAreDropped(t *testing.T) {
	var w bytes.Buffer
	e := &Env{Cfg: config.Defaults(), Stderr: &w}
	got := e.resolvable([]string{"localhost", "no-such-host.invalid", "*.npmjs.org", "10.0.0.1", "localhost:8080"})
	if strings.Join(got, " ") != "localhost *.npmjs.org 10.0.0.1 localhost:8080" {
		t.Fatalf("kept %v", got)
	}
	if !strings.Contains(w.String(), "no-such-host.invalid") {
		t.Fatalf("the dropped host must be named: %q", w.String())
	}
	e.Cfg.Network.Mode = "on"
	if got := e.resolvable([]string{"no-such-host.invalid"}); len(got) != 1 {
		t.Fatalf("outside allowlist mode nothing is filtered: %v", got)
	}
}
