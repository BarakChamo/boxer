package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// The real-runtime tests above skip wherever Apple's `container` is not installed, which is every
// CI runner, so the driver's argv, JSON decoding and exit-code handling would otherwise go
// unexercised there. This fake answers the verbs the driver calls, in the shapes a real 1.4.1
// prints, and logs every call. It proves boxer builds the right command lines and reads the
// answers correctly; the real-runtime tests and `make smoke` prove the runtime agrees.
const fakeContainer = `#!/bin/sh
echo "$*" >> "$FAKE_LOG"
inspect='[{"status":{"state":"running"},"configuration":{"id":"sb-a","labels":{"boxer.scope":"sb-a"},"image":{"reference":"alpine:3.20"},"resources":{"cpus":2,"memoryInBytes":4294967296}}}]'
case "$1" in
--version) echo "container CLI version 1.4.1" ;;
list) echo "$inspect" ;;
inspect)
  if [ "$2" = sb-missing ]; then echo "Error: container not found: sb-missing" >&2; exit 1; fi
  echo "$inspect" ;;
start|delete) ;;
stop)
  if [ "$2" = sb-stopped ]; then echo "Error: container is not running" >&2; exit 1; fi ;;
exec)
  last=""; for a in "$@"; do last=$a; done
  case "$last" in
  exit7) exit 7 ;;
  hang) sleep 30 ;;
  refused) echo "Error: get failed: container sb-gone not found" >&2; exit 1 ;;
  *) echo "ran $last" ;;
  esac ;;
*) echo "unexpected $1" >&2; exit 64 ;;
esac
`

func fakeApple(t *testing.T) (vm.Apple, func() string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "container")
	if err := os.WriteFile(bin, []byte(fakeContainer), 0o755); err != nil {
		t.Fatal(err)
	}
	logf := filepath.Join(dir, "calls.log")
	t.Setenv("FAKE_LOG", logf)
	calls := func() string { b, _ := os.ReadFile(logf); return string(b) }
	return vm.Apple{Bin: bin}, calls
}

func TestAppleDriverAgainstAFakeCLI(t *testing.T) {
	a, calls := fakeApple(t)

	if v, err := a.Version(); err != nil || v != "container CLI version 1.4.1" {
		t.Fatalf("version: %q %v", v, err)
	}
	ms, err := a.List()
	if err != nil || len(ms) != 1 {
		t.Fatalf("list: %v %v", ms, err)
	}
	if m := ms[0]; m.Name != "sb-a" || !m.Running() || m.CPUs != 2 || m.MemoryMiB != 4096 || m.Labels["boxer.scope"] != "sb-a" {
		t.Fatalf("list decoded wrong: %+v", m)
	}
	if _, ok, err := a.Status("sb-a"); !ok || err != nil {
		t.Fatalf("status of a present sandbox: %v %v", ok, err)
	}
	// Absent is an answer, not an error: boxer creates on it.
	if _, ok, err := a.Status("sb-missing"); ok || err != nil {
		t.Fatalf("status of an absent sandbox: %v %v", ok, err)
	}
	if err := a.Start("sb-a"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Stopping a stopped sandbox is not an error.
	if err := a.Stop("sb-stopped"); err != nil {
		t.Fatalf("stop of a stopped sandbox: %v", err)
	}
	if err := a.Delete("sb-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, want := range []string{"list --all --format json", "inspect sb-missing", "start sb-a", "stop sb-stopped", "delete --force sb-a"} {
		if !strings.Contains(calls(), want+"\n") {
			t.Errorf("no call %q in:\n%s", want, calls())
		}
	}
}

func TestAppleExecPassesOptionsAndExitCodes(t *testing.T) {
	a, calls := fakeApple(t)

	var out bytes.Buffer
	code, err := a.Exec(vm.ExecOpts{Name: "sb-a", User: "node", Workdir: "/workspace/app",
		Env: []string{"CI=1"}, SecretEnv: []string{"TOKEN=TOKEN"}, Stdout: &out, Stderr: &out}, "sh", "-c", "hello")
	if code != 0 || err != nil || !strings.Contains(out.String(), "ran hello") {
		t.Fatalf("exec: %d %v %q", code, err, out.String())
	}
	line := calls()
	for _, want := range []string{"exec -i -e BOXER_INSIDE=1 -u node -w /workspace/app -e CI=1 --env-file ", " sb-a sh -c hello"} {
		if !strings.Contains(line, want) {
			t.Errorf("exec argv lacks %q: %s", want, line)
		}
	}
	if strings.Contains(line, "TOKEN=") {
		t.Errorf("a secret reached argv: %s", line)
	}

	// The guest's exit code comes back as the command's, not as a backend failure.
	if code, err := a.Exec(vm.ExecOpts{Name: "sb-a", Stdout: &out, Stderr: &out}, "exit7"); code != 7 || err != nil {
		t.Fatalf("exit code: %d %v", code, err)
	}
	// The runtime's own refusal is an error, so boxer does not record it as the command's exit.
	if _, err := a.Exec(vm.ExecOpts{Name: "sb-a", Stdout: &out, Stderr: &out}, "refused"); !vm.IsNotFound(err) {
		t.Fatalf("runtime refusal: %v", err)
	}
	// A timeout stops the command rather than holding the caller.
	start := time.Now()
	if code, _ := a.Exec(vm.ExecOpts{Name: "sb-a", Timeout: 200 * time.Millisecond, Stdout: &out, Stderr: &out}, "hang"); code == 0 {
		t.Fatal("a timed-out exec reported success")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("timeout not enforced: took %s", d)
	}
}
