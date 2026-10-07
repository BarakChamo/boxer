package vm_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// These run against a real daemon, and skip by name when there is not one.
//
// Deliberately not a fake binary. The plan called for a shell script impersonating `docker`, on
// the reasoning that a Go fake proves nothing about argv construction — which is right, but a
// fake *shell* proves only that boxer's argv matches what the fake was written to expect, and the
// fake is written from the same reading of the docs that produced the argv. The two agree by
// construction and are wrong together. A real daemon is the only thing that knows whether
// `--entrypoint` goes before the image and `-c` after it, whether `inspect` returns an array,
// and whether an exit code survives `docker exec`.
func daemon(t *testing.T, bin string) vm.Docker {
	t.Helper()
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s is not installed", bin)
	}
	if err := exec.Command(bin, "info").Run(); err != nil {
		t.Skipf("%s is installed but its daemon is not running", bin)
	}
	return vm.NewDocker(bin)
}

const testImage = "mirror.gcr.io/library/alpine:3.21"

func TestDockerRoundTripsAWholeLifecycleAgainstARealDaemon(t *testing.T) {
	d := daemon(t, "docker")
	name := "boxer-test-lifecycle"
	_ = d.Delete(name)
	t.Cleanup(func() { _ = d.Delete(name) })

	// Create carries labels, because ownership is proved by label and nothing else.
	spec := vm.CreateSpec{
		Name: name, Image: testImage, CPUs: 1, MemoryMiB: 256,
		Labels: map[string]string{vm.LabelPrefix + "scope": "sb-test", vm.LabelPrefix + "root": "/tmp/x,y"},
	}
	if err := d.Create(spec); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Racing a create must be recognisable as a race, not reported as a hard failure.
	if err := d.Create(spec); !errors.Is(err, vm.ErrAlreadyExists) {
		t.Errorf("a second create must say ErrAlreadyExists, said %v", err)
	}

	m, ok, err := d.Status(name)
	if err != nil || !ok {
		t.Fatalf("status: %v ok=%v", err, ok)
	}
	// A label value containing a comma is exactly what `ps --format` would mangle, which is why
	// List goes through inspect. Assert the awkward one survives.
	if m.Labels[vm.LabelPrefix+"root"] != "/tmp/x,y" {
		t.Errorf("label with a comma did not survive: %q", m.Labels[vm.LabelPrefix+"root"])
	}
	if m.MemoryMiB != 256 || m.CPUs != 1 {
		t.Errorf("limits not read back: cpus=%d mem=%d", m.CPUs, m.MemoryMiB)
	}
	if m.Running() {
		t.Error("a created container is not yet running")
	}

	if err := d.Start(name); err != nil {
		t.Fatalf("start: %v", err)
	}
	if m, _, _ := d.Status(name); !m.Running() {
		t.Fatalf("not running after start: %+v", m)
	}

	// Ownership: Owned must find it by label, among every other container on the host.
	owned, err := vm.Owned(d)
	if err != nil {
		t.Fatalf("owned: %v", err)
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

	// The rule that decides whether a green run means anything: the guest's exit code, not the
	// backend's. A backend that loses this turns every failure into a pass.
	out, code, err := vm.Output(d, name, "/", "sh", "-c", "echo hello; exit 7")
	if err != nil || code != 7 || !strings.Contains(out, "hello") {
		t.Errorf("exit code and output must both survive exec: out=%q code=%d err=%v", out, code, err)
	}
	if _, code, _ := vm.Output(d, name, "/", "true"); code != 0 {
		t.Errorf("a successful command must report 0, reported %d", code)
	}

	// Stopping twice, and deleting twice, are both fine.
	if err := d.Stop(name); err != nil {
		t.Errorf("stop: %v", err)
	}
	if err := d.Stop(name); err != nil {
		t.Errorf("stopping an already-stopped container must not error: %v", err)
	}
	if err := d.Delete(name); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := d.Delete(name); err != nil {
		t.Errorf("deleting an already-deleted container must not error: %v", err)
	}
	if _, ok, err := d.Status(name); ok || err != nil {
		t.Errorf("a deleted container must read as absent: ok=%v err=%v", ok, err)
	}
}

// The refusal that matters most. boxer's default network mode is `allowlist`, and an OCI daemon
// has no per-host egress policy — so it must refuse rather than start a container with the whole
// internet reachable, which would read as enforced and not be.
func TestDockerRefusesTheAllowlistRatherThanRunningWideOpen(t *testing.T) {
	d := vm.NewDocker("docker") // no daemon needed: the refusal precedes any call
	err := d.Create(vm.CreateSpec{Name: "boxer-test-never", Image: testImage, Network: "allowlist"})
	if !errors.Is(err, vm.ErrUnsupported) {
		t.Fatalf("want a refusal, got %v", err)
	}
	for _, want := range []string{"allowlist", "smolvm"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must mention %q: %v", want, err)
		}
	}
	// And it must not have created anything on the way to refusing.
	if _, ok, _ := d.Status("boxer-test-never"); ok {
		_ = d.Delete("boxer-test-never")
		t.Error("refused, but created the container anyway")
	}
}

// Capabilities must describe this backend rather than smolvm's defaults.
func TestDockerDeclaresTheWeakerBoundaryItActuallyHas(t *testing.T) {
	c := vm.CapsOf(vm.NewDocker("docker"))
	if c.Boundary != "namespace" {
		t.Errorf("a container shares a kernel; boundary reads %q", c.Boundary)
	}
	if c.Allowlist {
		t.Error("claims an egress allowlist it cannot enforce")
	}
	if c.Packs || c.Branch || c.Egress || c.DiskUsage {
		t.Errorf("claims an optional interface it does not implement: %+v", c)
	}
	if !c.HostMounts || !c.SecretEnv {
		t.Errorf("understates what it can do: %+v", c)
	}
}

// Podman is the same driver with a different binary, so the only thing worth asserting separately
// is that it really is argv-compatible for the whole lifecycle.
func TestPodmanIsTheSameDriverWithADifferentBinary(t *testing.T) {
	d := daemon(t, "podman")
	name := "boxer-test-podman"
	_ = d.Delete(name)
	t.Cleanup(func() { _ = d.Delete(name) })

	if err := d.Create(vm.CreateSpec{
		Name: name, Image: testImage, MemoryMiB: 256,
		Labels: map[string]string{vm.LabelPrefix + "scope": "sb-test"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := d.Start(name); err != nil {
		t.Fatalf("start: %v", err)
	}
	out, code, err := vm.Output(d, name, "/", "sh", "-c", "echo hi; exit 3")
	if err != nil || code != 3 || !strings.Contains(out, "hi") {
		t.Errorf("podman exec: out=%q code=%d err=%v", out, code, err)
	}
	if m, ok, err := d.Status(name); err != nil || !ok || m.Labels[vm.LabelPrefix+"scope"] != "sb-test" {
		t.Errorf("podman status/labels: %+v ok=%v err=%v", m, ok, err)
	}
}

// A secret's value must never reach the argument vector. That is the whole reason this path
// exists, so it is asserted directly: the file carries the value, the value is named only by
// environment variable elsewhere, and nothing but this user can read the file.
func TestASecretGoesByNameRatherThanInTheArgumentVector(t *testing.T) {
	t.Setenv("BOXER_TEST_TOKEN", "hunter2\nline two")
	t.Setenv("BOXER_TEST_HOSTLIKE", "tcp://x")
	args, env, path, err := vm.SecretFlagsForTest([]string{"TOK=BOXER_TEST_TOKEN", "ABSENT=BOXER_TEST_UNSET", "DOCKER_X=BOXER_TEST_HOSTLIKE"})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	if strings.Contains(strings.Join(args, " "), "hunter2") || !slices.Contains(args, "TOK") {
		t.Fatalf("the value must stay out of argv, the name in it: %v", args)
	}
	// Several lines arrive whole: an --env-file cut them at the first newline.
	if !slices.Contains(env, "TOK=hunter2\nline two") {
		t.Errorf("value not forwarded whole: %q", env)
	}
	// A name the host does not set is omitted, not forwarded as empty: the two mean different
	// things to the program that reads them.
	if strings.Contains(strings.Join(append(args, env...), " "), "ABSENT") {
		t.Errorf("an unset secret was forwarded anyway: %v %v", args, env)
	}
	// A name the CLI would read as its own setting goes through a file only this user can read.
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("a file holding a secret: %v %v", err, st)
	}
	if b, _ := os.ReadFile(path); string(b) != "DOCKER_X=tcp://x\n" || slices.Contains(env, "DOCKER_X=tcp://x") {
		t.Errorf("file %q env %v", b, env)
	}
	t.Setenv("BOXER_TEST_HOSTLIKE", "a\nb")
	if _, _, _, err := vm.SecretFlagsForTest([]string{"DOCKER_X=BOXER_TEST_HOSTLIKE"}); err == nil || strings.Contains(err.Error(), "a\nb") {
		t.Errorf("refused by name only: %v", err)
	}
}

// A secret of several lines reaches the guest whole on a real daemon.
func TestAMultiLineSecretReachesTheGuest(t *testing.T) {
	d := daemon(t, "docker")
	name := "boxer-test-secret"
	_ = d.Delete(name)
	t.Cleanup(func() { _ = d.Delete(name) })
	if err := d.Create(vm.CreateSpec{Name: name, Image: testImage, CPUs: 1, MemoryMiB: 256}); err != nil {
		t.Fatal(err)
	}
	if err := d.Start(name); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOXER_TEST_PEM", "-----BEGIN KEY-----\nabc def\n-----END KEY-----")
	var out bytes.Buffer
	code, err := d.Exec(vm.ExecOpts{Name: name, SecretEnv: []string{"PEM=BOXER_TEST_PEM"}, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}, "sh", "-c", `printf %s "$PEM"`)
	if err != nil || code != 0 || out.String() != "-----BEGIN KEY-----\nabc def\n-----END KEY-----" {
		t.Fatalf("%d %v %q", code, err, out.String())
	}
}

// Version names the backend, so a doctor report says which runtime it found.
func TestDockerVersionNamesTheBinary(t *testing.T) {
	d := daemon(t, "docker")
	v, err := d.Version()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v, "docker ") {
		t.Errorf("version %q does not name the binary", v)
	}
}

// The daemon's wording, mapped onto the conditions boxer branches on. These strings are docker's,
// not smolvm's, which is the entire reason the sentinels exist.
func TestDockerWordingMapsOntoTheConditionItMeans(t *testing.T) {
	for _, tc := range []struct {
		said string
		want error
	}{
		{"Error response from daemon: No such container: sb-x", vm.ErrNotFound},
		{"Error: No such object: sb-x", vm.ErrNotFound},
		{`Conflict. The container name "/sb-x" is already in use`, vm.ErrAlreadyExists},
		{"Error response from daemon: Container sb-x is not running", vm.ErrNotRunning},
		{"disk on fire", nil},
		// A pull before the create reports cached layers; the failure after it is not a race.
		{"Unable to find image 'x' locally\n9a1b: Already exists\nError response from daemon: invalid mount config", nil},
		{"9a1b: Already exists\nError: creating container storage: the container name \"sb-x\" is already in use", vm.ErrAlreadyExists},
	} {
		if got := vm.ClassifyDockerForTest(tc.said); !errors.Is(got, tc.want) {
			t.Errorf("%q\n  is   %v\n  want %v", tc.said, got, tc.want)
		}
	}
}

// docker, podman and Apple's container publish on every interface unless told otherwise, which put
// a sandbox's dev server on the local network. boxer binds loopback, as smolvm does.
func TestPublishedPortsBindLoopback(t *testing.T) {
	for in, want := range map[string]string{
		"51043:3000":          "127.0.0.1:51043:3000",
		"3000:3000":           "127.0.0.1:3000:3000",
		"3000":                "127.0.0.1:3000:3000", // a bare port is not all interfaces
		"3000/udp":            "127.0.0.1:3000:3000/udp",
		"8080:3000/tcp":       "127.0.0.1:8080:3000/tcp",
		"0.0.0.0:3000:3000":   "0.0.0.0:3000:3000", // an explicit address the user chose
		"127.0.0.1:3000:3000": "127.0.0.1:3000:3000",
	} {
		if got := vm.LoopbackPortForTest(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

// A timed-out command ends with everything it started. busybox's timeout (Alpine) killed only the
// process it ran, and a test runner's workers kept running in the container with nothing to stop
// them.
func TestATimeoutEndsTheWholeCommandOnBusybox(t *testing.T) {
	d := daemon(t, "docker")
	name := "boxer-test-timeout"
	_ = d.Delete(name)
	t.Cleanup(func() { _ = d.Delete(name) })
	if err := d.Create(vm.CreateSpec{Name: name, Image: testImage, CPUs: 1, MemoryMiB: 256}); err != nil {
		t.Fatal(err)
	}
	if err := d.Start(name); err != nil {
		t.Fatal(err)
	}
	// A child whose process name has spaces, as npm gives itself; its own children must go too.
	var out bytes.Buffer
	if code, _ := d.Exec(vm.ExecOpts{Name: name, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}, "sh", "-c", `printf '#!/bin/sh\nsleep 304\n' > "/tmp/npm run lint" && chmod +x "/tmp/npm run lint"`); code != 0 {
		t.Fatal(out.String())
	}
	out.Reset()
	start := time.Now()
	code, err := d.Exec(vm.ExecOpts{Name: name, Timeout: 2 * time.Second, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out},
		"sh", "-c", "sleep 300 & sh -c 'sleep 302 & sleep 303' & '/tmp/npm run lint' & sleep 301")
	if err != nil || code != 137 || time.Since(start) > 10*time.Second {
		t.Fatalf("a timed-out command exits 137 at its deadline: code %d err %v after %s: %s", code, err, time.Since(start), out.String())
	}
	out.Reset()
	if code, _ := d.Exec(vm.ExecOpts{Name: name, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}, "sh", "-c", "sleep 2; ps -o args | grep '[s]leep 30\\|[n]pm run'"); code != 1 || out.Len() != 0 {
		t.Fatalf("nothing it started may be left running: %s", out.String())
	}
	out.Reset()
	if code, _ := d.Exec(vm.ExecOpts{Name: name, Timeout: 30 * time.Second, Stdin: strings.NewReader("hi\n"), Stdout: &out, Stderr: &out}, "sh", "-c", "cat; grep -q '^SigIgn:.*0000$' /proc/self/status || echo ignores-interrupt; exit 7"); code != 7 || out.String() != "hi\n" {
		t.Fatalf("under a deadline, stdin and the exit code pass through: %d %q", code, out.String())
	}
}
