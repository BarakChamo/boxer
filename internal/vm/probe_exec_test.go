package vm

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// boxer spends about 13ms per command on `machine status`, purely to learn whether the sandbox is
// there before running anything. Removing it is the last available win on the one-off path — it
// would take `boxer run -- true` from ~38ms to ~25ms — and the way to remove it would be to just
// try the command and treat "no such sandbox" as the signal to provision.
//
// This test exists to record why boxer does NOT do that, and to fail if that ever changes.
//
// smolvm reports a missing sandbox as a plain non-zero exit with the reason on stderr, which is
// byte-for-byte how it reports a guest command that exited 1. So the only way to tell them apart
// is to scan stderr for a phrase — and stderr belongs to the caller's command, which may print
// anything. Guess wrong and boxer provisions and runs the command a *second* time, which for
// anything with a side effect (a publish, a migration, a push) is far worse than 13ms is good.
//
// Since 1.4, Exec reads smolvm's own wording off stderr ("Error: vm not found", a dropped agent
// connection) and returns it as an error, so a code smolvm produced is never recorded as the
// command's. That reading is only ever used to report, never to retry: it must not become the
// existence check, because a guest that printed those words would then be provisioned and run
// twice. This test pins both halves against the real smolvm: the missing sandbox is an error
// classified as not found, and its exit code is still smolvm's non-zero one.
func TestAMissingSandboxIsIndistinguishableFromAFailedCommand(t *testing.T) {
	if _, err := exec.LookPath("smolvm"); err != nil {
		t.Skip("needs a real smolvm")
	}
	var out bytes.Buffer
	code, err := New().Exec(ExecOpts{Name: "definitely-not-here-boxer-probe", Stdout: &out, Stderr: &out}, "true")
	if err == nil || !IsNotFound(err) {
		t.Fatalf("a missing sandbox must come back as a not-found error, not as the command's code: %v", err)
	}
	if code == 0 {
		t.Fatalf("a missing sandbox reported success, which no caller can act on")
	}
	if !strings.Contains(out.String(), "not found") {
		t.Fatalf("the only signal boxer has is this text, and it changed: %q", out.String())
	}
}

// A guest that prints an error of its own is still the guest's failure.
func TestAGuestErrorIsNotSmolvms(t *testing.T) {
	for _, s := range []string{"Error: tests failed", "error: vm not found in inventory", "Error: machine learning broke",
		"redis: connection closed", "psql: error: server connection closed unexpectedly", "grpc: the client connection closed"} {
		if smolvmFailure(s) != "" {
			t.Errorf("%q is the guest's", s)
		}
	}
	for _, s := range []string{"Error: vm not found: sb-1", "Error: agent operation failed: x", "Error: machine 'sb-1' is not running", "Error: connection closed"} {
		if smolvmFailure("noise\n"+s+"\n") == "" {
			t.Errorf("%q is smolvm's", s)
		}
	}
}
