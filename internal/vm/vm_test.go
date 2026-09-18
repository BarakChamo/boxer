package vm_test

import (
	"errors"
	"os"
	"strings"
	"testing"

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
	out, code, err := c.Output("sb-x", "/workspace", "sh", "-c", "echo hi; exit 3")
	if err != nil || code != 3 || strings.TrimSpace(out) != "hi" {
		t.Fatalf("exec passthrough: %q %d %v", out, code, err)
	}
	owned, _ := c.Owned()
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
	for _, want := range []string{"-I alpine", "-v /tmp/wt:/workspace", "--label boxer.scope=sb-x", "--allow-host registry-1.docker.io", "--cpus 2", "--mem 1024", "exec --name sb-x -i -e BOXER_INSIDE=1 -w /workspace --", "delete -n sb-x --force --cascade"} {
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
	owned, err := c.Owned()
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
	if owned, _ := c.Owned(); len(owned) != 1 || owned[0].Name != "sb-b" {
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
