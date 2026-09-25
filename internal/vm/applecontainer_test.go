package vm_test

import (
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// Apple's `container` runs one lightweight VM per container. It skips by name when the CLI or its
// services are absent, like every other real-backend test here.
func apple(t *testing.T) vm.Apple {
	t.Helper()
	if _, err := exec.LookPath("container"); err != nil {
		t.Skip("apple container is not installed")
	}
	if err := exec.Command("container", "system", "status").Run(); err != nil {
		t.Skip("apple container is installed but its services are not running")
	}
	// Running services are not the same as a usable runtime: this one needs a guest kernel
	// installed separately, and without it every create fails. These tests reported that as a
	// failure rather than a skip, blaming boxer for an unconfigured host.
	//
	// Asking a property does not answer it — `system property list` shows the *recommended*
	// kernel whether or not it is installed. So the guard attempts the real thing and skips only
	// on a host-setup error. Any other failure is boxer's and must still fail.
	a := vm.NewApple()
	probe := "boxer-probe-ready"
	_ = a.Delete(probe)
	if err := a.Create(vm.CreateSpec{Name: probe, Image: testImage}); err != nil {
		_ = a.Delete(probe)
		if hostNotReady(err) {
			t.Skipf("apple container is not usable on this host: %v", err)
		}
		t.Fatalf("probe create failed for a reason that is not host setup: %v", err)
	}
	_ = a.Delete(probe)
	return a
}

// hostNotReady reports a failure that is the machine's fault rather than boxer's — a missing
// guest kernel, or an image that was never pulled. Kept narrow on purpose: everything it does not
// name is a real failure, because a guard that skips too eagerly turns a broken backend green.
func hostNotReady(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "kernel not configured") ||
		strings.Contains(s, "no such image") ||
		strings.Contains(s, "failed to resolve reference")
}

func TestAppleRoundTripsAWholeLifecycle(t *testing.T) {
	a := apple(t)
	name := "boxer-test-apple"
	_ = a.Delete(name)
	t.Cleanup(func() { _ = a.Delete(name) })

	spec := vm.CreateSpec{
		Name: name, Image: testImage, CPUs: 2, MemoryMiB: 1024,
		Labels: map[string]string{vm.LabelPrefix + "scope": "sb-test", vm.LabelPrefix + "root": "/tmp/x,y"},
	}
	if err := a.Create(spec); err != nil {
		t.Fatalf("create: %v", err)
	}

	m, ok, err := a.Status(name)
	if err != nil || !ok {
		t.Fatalf("status: %v ok=%v", err, ok)
	}
	// `status` is an object in this CLI's JSON, not a string. A struct that read it as a string
	// would leave State empty and report a running container as stopped, so boxer would start it
	// again — assert the state actually arrives.
	if m.State == "" {
		t.Error("state did not decode; `status` is an object, not a string")
	}
	if m.Running() {
		t.Error("a created container is not yet running")
	}
	if m.Labels[vm.LabelPrefix+"root"] != "/tmp/x,y" {
		t.Errorf("label did not survive: %q", m.Labels[vm.LabelPrefix+"root"])
	}
	if m.CPUs != 2 || m.MemoryMiB != 1024 {
		t.Errorf("limits not read back: cpus=%d mem=%d", m.CPUs, m.MemoryMiB)
	}

	if err := a.Start(name); err != nil {
		t.Fatalf("start: %v", err)
	}
	if m, _, _ := a.Status(name); !m.Running() {
		t.Fatalf("not running after Start, which waits for exactly that: %+v", m)
	}

	owned, err := vm.Owned(a)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, o := range owned {
		if o.Name == name {
			found = true
		}
	}
	if !found {
		t.Error("a labelled container is not reported as owned")
	}

	// The exit code, which decides whether a green run means anything.
	out, code, err := vm.Output(a, name, "/", "sh", "-c", "echo hello; exit 7")
	if err != nil || code != 7 || !strings.Contains(out, "hello") {
		t.Errorf("exit code and output must survive exec: out=%q code=%d err=%v", out, code, err)
	}

	if err := a.Stop(name); err != nil {
		t.Errorf("stop: %v", err)
	}
	if err := a.Delete(name); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := a.Delete(name); err != nil {
		t.Errorf("deleting twice must not error: %v", err)
	}
	if _, ok, err := a.Status(name); ok || err != nil {
		t.Errorf("a deleted container must read as absent: ok=%v err=%v", ok, err)
	}
}

// Version identifies the runtime in a doctor report.
func TestAppleVersionNamesTheRuntime(t *testing.T) {
	v, err := apple(t).Version()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(v), "container") {
		t.Errorf("version %q does not name the runtime", v)
	}
}

// Stdin must reach the guest. Without `-i` it is closed, and `boxer run -- wc -l` reads nothing
// from a pipe — which is how this was found, by the smoke suite rather than by reading the flags.
func TestAppleForwardsStdin(t *testing.T) {
	a := apple(t)
	name := "boxer-test-apple-stdin"
	_ = a.Delete(name)
	t.Cleanup(func() { _ = a.Delete(name) })
	if err := a.Create(vm.CreateSpec{Name: name, Image: testImage}); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(name); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	code, err := a.Exec(vm.ExecOpts{
		Name: name, Stdin: strings.NewReader("a\nb\n"), Stdout: &out, Stderr: &out,
	}, "wc", "-l")
	if err != nil || code != 0 || strings.TrimSpace(out.String()) != "2" {
		t.Errorf("stdin not forwarded: out=%q code=%d err=%v", out.String(), code, err)
	}
}

// A published port must actually reach the host. This runtime gives each container its own
// address on a private network that the host cannot route to, so a sandbox with no published port
// runs perfectly and answers nothing — the failure that looks most like success. boxer omitted the
// flag entirely at first, and three dev servers came up healthy and unreachable.
func TestApplePublishesPortsToTheHost(t *testing.T) {
	a := apple(t)
	name := "boxer-test-apple-port"
	_ = a.Delete(name)
	t.Cleanup(func() { _ = a.Delete(name) })

	const port = "39511"
	if err := a.Create(vm.CreateSpec{
		Name: name, Image: testImage, Ports: []string{port + ":" + port},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(name); err != nil {
		t.Fatal(err)
	}
	// A one-line server in the guest, then ask for it from the host.
	go func() {
		_, _, _ = vm.Output(a, name, "/", "sh", "-c",
			"while :; do printf 'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nhi' | nc -l -p "+port+" || break; done")
	}()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:" + port + "/")
		if err == nil {
			_ = resp.Body.Close()
			return // reachable: the port was published
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Error("a published port never became reachable from the host")
}

// A kernel per sandbox, like smolvm — but no per-host egress policy, so the allowlist is refused
// exactly as it is on docker. This is the backend that shows the capability model is not simply
// "smolvm and not-smolvm": it shares a boundary with one and a refusal with the other.
func TestAppleHasSmolvmsBoundaryAndDockersRefusal(t *testing.T) {
	c := vm.CapsOf(vm.NewApple())
	if c.Boundary != "kernel" {
		t.Errorf("one VM per container is a kernel boundary; reads %q", c.Boundary)
	}
	if c.Allowlist {
		t.Error("claims an egress allowlist it cannot enforce")
	}
	if !c.ProvesOwnership || !c.HostMounts {
		t.Errorf("understates what it can do: %+v", c)
	}
	if c.Packs || c.Branch || c.Egress || c.DiskUsage {
		t.Errorf("claims an optional interface it does not implement: %+v", c)
	}

	err := vm.NewApple().Create(vm.CreateSpec{Name: "boxer-test-never", Image: testImage, Network: "allowlist"})
	if !errors.Is(err, vm.ErrUnsupported) {
		t.Fatalf("want a refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "smolvm") {
		t.Errorf("the refusal must name the backend that can: %v", err)
	}
}

// The wording this CLI uses, mapped onto the conditions boxer branches on.
func TestAppleWordingMapsOntoTheConditionItMeans(t *testing.T) {
	for _, tc := range []struct {
		said string
		want error
	}{
		{"Error: container not found: sb-x", vm.ErrNotFound},
		{"Error: container with id sb-x already exists", vm.ErrAlreadyExists},
		{"Error: container sb-x is not running", vm.ErrNotRunning},
		{"disk on fire", nil},
	} {
		if got := vm.ClassifyAppleForTest(tc.said); !errors.Is(got, tc.want) {
			t.Errorf("%q\n  is   %v\n  want %v", tc.said, got, tc.want)
		}
	}
}

// A real `container inspect` payload, trimmed to the fields boxer reads. Captured from 1.4.1
// rather than written from the documentation, because the documentation does not say that
// `status` is an object — and reading it as a string left State empty, which reports a running
// container as stopped. boxer answers that by starting a container that is already running.
//
// Needs no runtime, so it keeps the decoding covered on a machine where Apple's `container` is
// not installed: the parsing is where the mistakes were, and it should not go untested just
// because an optional backend is absent.
const appleInspectSample = `[
  {
    "status": { "state": "running", "networks": [] },
    "configuration": {
      "id": "sb-459b60414640",
      "labels": { "boxer.scope": "sb-459b60414640", "boxer.root": "/tmp/x,y" },
      "image": { "reference": "mirror.gcr.io/library/node:24-alpine" },
      "resources": { "cpus": 2, "memoryInBytes": 4294967296 }
    }
  },
  {
    "status": { "state": "stopped", "networks": [] },
    "configuration": {
      "id": "sb-other",
      "labels": {},
      "image": { "reference": "alpine:3.21" },
      "resources": { "cpus": 1, "memoryInBytes": 1073741824 }
    }
  }
]`

func TestAppleInspectDecodesWithoutTheRuntime(t *testing.T) {
	ms, err := vm.DecodeAppleForTest(appleInspectSample)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("decoded %d machines, want 2", len(ms))
	}
	first := ms[0]
	if !first.Running() {
		t.Errorf("state did not decode; `status` is an object, not a string: %+v", first)
	}
	if first.Name != "sb-459b60414640" {
		t.Errorf("name: %q", first.Name)
	}
	if first.Labels["boxer.root"] != "/tmp/x,y" {
		t.Errorf("a label containing a comma did not survive: %q", first.Labels["boxer.root"])
	}
	if first.CPUs != 2 || first.MemoryMiB != 4096 {
		t.Errorf("resources: cpus=%d mem=%d, want 2 and 4096", first.CPUs, first.MemoryMiB)
	}
	if ms[1].Running() {
		t.Error("a stopped container reported as running")
	}

	// An empty or null list is normal — no containers yet — and must not be an error.
	for _, empty := range []string{"", "null", "[]"} {
		if got, err := vm.DecodeAppleForTest(empty); err != nil || len(got) != 0 {
			t.Errorf("decode(%q) = %v, %v; want empty and no error", empty, got, err)
		}
	}
}
