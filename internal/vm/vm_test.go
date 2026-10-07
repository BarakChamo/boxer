package vm_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func TestLifecycleAgainstFake(t *testing.T) {
	c, log := vmtest.Install(t)
	if _, ok, _ := c.Status("sb-x"); ok {
		t.Fatal("should not exist yet")
	}
	err := c.Create(vm.CreateSpec{Name: "sb-x", Image: "alpine", Volumes: []string{"/tmp/wt:/workspace"}, Labels: map[string]string{"boxer.scope": "sb-x"}, CPUs: 2, MemoryMiB: 1024, Network: "allowlist", AllowHosts: []string{"registry-1.docker.io"}})
	if err != nil {
		t.Fatal(err)
	}
	m, ok, _ := c.Status("sb-x")
	if !ok || m.Running() {
		t.Fatalf("created but stopped expected: %+v", m)
	}
	if err := c.Start("sb-x"); err != nil {
		t.Fatal(err)
	}
	m, _, _ = c.Status("sb-x")
	if !m.Running() {
		t.Fatal("running expected")
	}
	out, code, err := vm.Output(c, "sb-x", "/workspace", "sh", "-c", "echo hi; exit 3")
	if err != nil || code != 3 || strings.TrimSpace(out) != "hi" {
		t.Fatalf("exec passthrough: %q %d %v", out, code, err)
	}
	owned, _ := vm.Owned(c)
	if len(owned) != 1 || owned[0].Labels["boxer.scope"] != "sb-x" {
		t.Fatalf("owned: %+v", owned)
	}
	if err := c.Delete("sb-x"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Status("sb-x"); ok {
		t.Fatal("deleted")
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	for _, want := range []string{"-I alpine", "-v /tmp/wt:/workspace", "--label boxer.scope=sb-x", "--allow-host registry-1.docker.io", "--cpus 2", "--mem 1024", "exec --name sb-x -i -e BOXER_INSIDE=1 -w /workspace --", "delete -n sb-x --force --cascade",
		// A workload that never exits, not the image's CMD: see Create.
		"-- sh -c while :; do sleep 3600; done"} {
		if !strings.Contains(s, want) {
			t.Errorf("smolvm never invoked with %q\nlog:\n%s", want, s)
		}
	}
	if strings.Contains(s, "--net") {
		t.Error("allowlist mode must not pass --net")
	}
}

func TestMissingBinaryMessage(t *testing.T) {
	c := vm.Client{Bin: "definitely-not-smolvm-xyz"}
	_, err := c.Version()
	if err == nil || !strings.Contains(err.Error(), "install") {
		t.Fatalf("want install hint, got %v", err)
	}
}

// Callers used to match substrings of a flattened message to tell smolvm's refusals apart. The
// typed error carries the verb, the exit code and what smolvm said, and the predicates are the
// only place those strings are known.
func TestTypedErrorAndPredicates(t *testing.T) {
	c, _ := vmtest.Install(t)
	vmtest.FailVerb(t, "machine create", "create machine: machine 'sb-x' already exists or is being created")
	err := c.Create(vm.CreateSpec{Name: "sb-x", Image: "alpine"})
	var ve *vm.Error
	if !errors.As(err, &ve) || ve.Verb != "machine" || ve.Code != 1 || !strings.Contains(ve.Stderr, "already exists") {
		t.Fatalf("typed error: %#v (%v)", ve, err)
	}
	if !vm.IsAlreadyExists(err) || vm.IsNotFound(err) || vm.IsNotRunning(err) {
		t.Fatalf("predicates disagree about %v", err)
	}
	vmtest.StopFailing(t, "machine create")

	// Stopping a machine that is not running is not an error, however smolvm phrases it.
	vmtest.FailVerb(t, "machine stop", "machine is not running")
	if err := c.Stop("sb-x"); err != nil {
		t.Fatalf("stopping a stopped machine: %v", err)
	}
	vmtest.FailVerb(t, "machine stop", "disk on fire")
	if err := c.Stop("sb-x"); err == nil {
		t.Fatal("a real stop failure must be reported")
	}
	vmtest.StopFailing(t, "machine stop")

	// A status that smolvm cannot answer is an error, not "the machine is absent".
	vmtest.FailVerb(t, "machine status", "daemon not reachable")
	if _, ok, err := c.Status("sb-x"); err == nil || ok {
		t.Fatalf("broken status: %v %v", ok, err)
	}
	vmtest.StopFailing(t, "machine status")

	// A transport failure looks exactly like a guest command that exited non-zero, so the text is
	// the only signal; the probes depend on this.
	if !vm.TransportFailure("Error: connection closed") || vm.TransportFailure("no such file") {
		t.Fatal("TransportFailure")
	}
}

// The fake models more than one machine, and its image and version are what the tests say.
func TestFakeModelsSeveralMachines(t *testing.T) {
	c, _ := vmtest.Install(t)
	vmtest.SetVersion(t, "smolvm 9.9.9")
	vmtest.SetImage(t, "ubuntu:24.04")
	for _, n := range []string{"sb-a", "sb-b"} {
		if err := c.Create(vm.CreateSpec{Name: n, Labels: map[string]string{"boxer.scope": n}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Start("sb-a"); err != nil {
		t.Fatal(err)
	}
	owned, err := vm.Owned(c)
	if err != nil || len(owned) != 2 {
		t.Fatalf("owned: %v %v", owned, err)
	}
	a, _, _ := c.Status("sb-a")
	b, _, _ := c.Status("sb-b")
	if !a.Running() || b.Running() || a.Image != "ubuntu:24.04" {
		t.Fatalf("machines are independent: %+v %+v", a, b)
	}
	if v, _ := c.Version(); v != "smolvm 9.9.9" {
		t.Fatalf("version: %s", v)
	}
	if err := c.Delete("sb-a"); err != nil {
		t.Fatal(err)
	}
	if owned, _ := vm.Owned(c); len(owned) != 1 || owned[0].Name != "sb-b" {
		t.Fatalf("after delete: %v", owned)
	}
}

// DataDir is where a machine's disks live, and it is the only way to measure what a sandbox costs
// on disk. A machine that does not exist has no directory, which is "unknown" to the caller rather
// than an empty path it might then walk.
func TestDataDir(t *testing.T) {
	vmtest.Install(t)
	c := vm.New()
	if err := c.Create(vm.CreateSpec{Name: "sb-data", Labels: map[string]string{"boxer.scope": "sb-data"}}); err != nil {
		t.Fatal(err)
	}
	dir, err := c.DataDir("sb-data")
	if err != nil || dir == "" {
		t.Fatalf("a live machine has a data directory: %q %v", dir, err)
	}
	if _, err := c.DataDir("sb-nosuchmachine"); err == nil {
		t.Fatal("a machine that does not exist has no data directory")
	}
}

// Branching is the one smolvm verb with real preconditions: the source must have been started
// --branchable, and the children's names are read back from the machine list rather than parsed
// out of stdout, whose format is not a contract.
func TestBranchRequiresABranchableSourceAndNamesItsChildren(t *testing.T) {
	c, log := vmtest.Install(t)
	if err := c.Create(vm.CreateSpec{Name: "sb-p", Image: "alpine", Labels: map[string]string{"boxer.scope": "sb-p"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start("sb-p"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Branch(vm.BranchSpec{From: "sb-p", NamePrefix: "sb-p-f", Count: 1}); err == nil {
		t.Fatal("an ordinary start cannot be branched from")
	}
	if err := c.StartBranchable("sb-p"); err != nil {
		t.Fatal(err)
	}
	names, err := c.Branch(vm.BranchSpec{From: "sb-p", NamePrefix: "sb-p-f", Count: 2, SecretEnv: []string{"TOK=TOK"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "sb-p-f1" || names[1] != "sb-p-f2" {
		t.Fatalf("children: %v", names)
	}
	for _, n := range names {
		if m, ok, _ := c.Status(n); !ok || !m.Running() {
			t.Fatalf("a branch child starts from its parent's running state: %+v", m)
		}
	}
	b, _ := os.ReadFile(log)
	// One named branch per child, never a --count/--name-prefix batch: a batch waits for the
	// source workload to run smolvm-branch-ready and gives up after ten minutes when — as in
	// every ordinary sandbox — nothing ever does.
	if !strings.Contains(string(b), "machine branch --from sb-p -n sb-p-f1 --secret-env TOK=TOK") {
		t.Fatalf("branch flags:\n%s", b)
	}
	if strings.Contains(string(b), "--count") || strings.Contains(string(b), "--name-prefix") {
		t.Fatalf("a batch branch would hang against a real source:\n%s", b)
	}
	// A stop takes branchability away, because the memfd and the control socket go with it.
	if err := c.Stop("sb-p"); err != nil {
		t.Fatal(err)
	}
	if err := c.Start("sb-p"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Branch(vm.BranchSpec{From: "sb-p", NamePrefix: "sb-p-g", Count: 1}); err == nil {
		t.Fatal("branchability must not survive a stop")
	}
}

// Egress denials are the only record of why an allowlisted guest could not reach a host.
func TestEgressDenials(t *testing.T) {
	c, _ := vmtest.Install(t)
	if err := c.Create(vm.CreateSpec{Name: "sb-u", Image: "alpine", Volumes: []string{"/wt:/workspace"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start("sb-u"); err != nil {
		t.Fatal(err)
	}
	// No denials recorded: an empty list, not an error.
	evs, err := c.Egress("sb-u", 10)
	if err != nil || len(evs) != 0 {
		t.Fatalf("egress: %v %v", evs, err)
	}
	// The real schema, from smolvm 1.16.1: every event is a denial, a blocked lookup names the
	// host and a blocked connection names host:port.
	vmtest.SetEgress(t, `[{"timestamp":"2026-09-21T11:14:14Z","operation":"resolve","dest":"registry.npmjs.org"},
	                      {"timestamp":"2026-09-21T11:14:23Z","operation":"connect","dest":"1.1.1.1:80"}]`)
	evs, err = c.Egress("sb-u", 10)
	if err != nil || len(evs) != 2 {
		t.Fatalf("egress: %+v %v", evs, err)
	}
	if evs[0].Host() != "registry.npmjs.org" || evs[1].Host() != "1.1.1.1" {
		t.Fatalf("a host is the destination without its port: %q %q", evs[0].Host(), evs[1].Host())
	}
}

// An exec carries a timeout and a secret by name; the value is never in the argv.
func TestExecPassesTimeoutAndSecretsByName(t *testing.T) {
	c, log := vmtest.Install(t)
	if err := c.Create(vm.CreateSpec{Name: "sb-e", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start("sb-e"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOK", "hunter2")
	var buf bytes.Buffer
	code, err := c.Exec(vm.ExecOpts{
		Name: "sb-e", SecretEnv: []string{"TOK=TOK"}, Timeout: 90 * time.Second,
		Stdin: strings.NewReader(""), Stdout: &buf, Stderr: &buf,
	}, "sh", "-c", "echo $TOK")
	out := buf.String()
	if err != nil || code != 0 || strings.TrimSpace(out) != "hunter2" {
		t.Fatalf("exec: %q %d %v", out, code, err)
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "--timeout 1m30s") {
		t.Fatalf("timeout flag:\n%s", b)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("a secret value must never reach the argv:\n%s", b)
	}
}

// The sentinels are the wire between a backend and the rest of boxer, so two things have to hold:
// the predicates answer through errors.Is rather than through a substring the caller knows, and
// smolvm's real wording maps onto the condition it actually means. Every string here is one
// smolvm 1.16.1 produced; if a future version rephrases one, this is what notices, and the fix is
// in classifySmolvm rather than scattered through internal/box.
func TestSmolvmWordingMapsOntoTheConditionItMeans(t *testing.T) {
	c, _ := vmtest.Install(t)
	for _, tc := range []struct {
		said string
		want error
	}{
		{"vm not found: sb-7e1852e4a3c3", vm.ErrNotFound},
		{"create machine: machine 'sb-x' already exists or is being created", vm.ErrAlreadyExists},
		{"database operation failed: reserve vm 'c2': database is locked", vm.ErrBusy},
		{"agent operation failed: connect: machine 'sb-x' is not running.", vm.ErrNotRunning},
	} {
		vmtest.FailVerb(t, "machine status", tc.said)
		_, _, err := c.Status("sb-x")
		// Status swallows a missing machine by design, so ask Create for that one instead.
		if tc.want == vm.ErrNotFound {
			vmtest.StopFailing(t, "machine status")
			vmtest.FailVerb(t, "machine create", tc.said)
			err = c.Create(vm.CreateSpec{Name: "sb-x", Image: "alpine"})
			vmtest.StopFailing(t, "machine create")
		} else {
			vmtest.StopFailing(t, "machine status")
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("%q\n  is   %v\n  want %v", tc.said, err, tc.want)
		}
	}
}

// A failure that is none of the named conditions must stay unclassified rather than being forced
// into the nearest one: "disk on fire" is not a missing machine, and a caller that retried it as
// one would loop.
func TestAnUnrecognisedFailureIsNotGuessedAt(t *testing.T) {
	c, _ := vmtest.Install(t)
	vmtest.FailVerb(t, "machine create", "disk on fire")
	err := c.Create(vm.CreateSpec{Name: "sb-x", Image: "alpine"})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, s := range []error{vm.ErrNotFound, vm.ErrAlreadyExists, vm.ErrBusy, vm.ErrNotRunning} {
		if errors.Is(err, s) {
			t.Errorf("classified %v as %v", err, s)
		}
	}
}

// A pack appears at its path only whole, and nothing else is left in the pack directory: other
// worktrees boot from any pack that exists there, and gc globs it.
func TestPackAppearsOnlyWhole(t *testing.T) {
	c, _ := vmtest.Install(t)
	dir := t.TempDir()
	stub := filepath.Join(dir, "env")
	got, err := c.Pack("alpine", stub)
	if err != nil || got != stub+".smolmachine" {
		t.Fatalf("pack: %q %v", got, err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 || ents[0].Name() != "env.smolmachine" {
		t.Fatalf("only the pack may be left: %v", ents)
	}
	vmtest.FailVerb(t, "pack create", "disk full")
	if _, err := c.PackFromVM("sb-x", filepath.Join(dir, "other")); err == nil {
		t.Fatal("a failed pack must say so")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("a failed pack must leave nothing: %v", ents)
	}
}

// State never lands relative to the current directory: gc deletes from it.
func TestStateHomeIsAlwaysAbsolute(t *testing.T) {
	for _, env := range [][2]string{{"relative/state", "/home/x"}, {"", ""}, {"relative", ""}} {
		t.Setenv("XDG_STATE_HOME", env[0])
		t.Setenv("HOME", env[1])
		if got := vm.StateHome(); !filepath.IsAbs(got) {
			t.Fatalf("XDG_STATE_HOME=%q HOME=%q: %q", env[0], env[1], got)
		}
	}
	t.Setenv("XDG_STATE_HOME", "/abs/state")
	if got := vm.StateHome(); got != "/abs/state" {
		t.Fatal(got)
	}
}

// A runtime CLI ended by a signal has not reported the command's status: that is a backend failure,
// never the command exiting -1.
func TestASignalledCLIIsABackendFailure(t *testing.T) {
	c, _ := vmtest.Install(t)
	vmtest.KillExecOnce(t)
	var out bytes.Buffer
	code, err := c.Exec(vm.ExecOpts{Name: "sb-x", Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}, "true")
	if err == nil || code < 0 {
		t.Fatalf("code %d err %v", code, err)
	}
}

// smolvm reports its own failure last. A guest line in the middle of the output that happens to
// read like one is the guest's, and its exit code is the command's.
func TestOnlySmolvmsLastLineIsItsOwn(t *testing.T) {
	if vm.SmolvmFailureForTest("Error: hyper: connection closed before message completed\n2 tests failed\n") != "" {
		t.Error("a guest's line was taken for smolvm's")
	}
	if vm.SmolvmFailureForTest("output\nError: agent operation failed: connection closed\n") == "" {
		t.Error("smolvm's own last line was missed")
	}
}
