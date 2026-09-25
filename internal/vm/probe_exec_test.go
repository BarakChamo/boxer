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
// If a future smolvm distinguishes the two — a reserved exit code, or a structured error — this
// test fails, and the optimisation becomes available. That is the intent: it is a tripwire on an
// upstream contract, not a check on boxer.
func TestAMissingSandboxIsIndistinguishableFromAFailedCommand(t *testing.T) {
	if _, err := exec.LookPath("smolvm"); err != nil {
		t.Skip("needs a real smolvm")
	}
	var out bytes.Buffer
	code, err := New().Exec(ExecOpts{Name: "definitely-not-here-boxer-probe", Stdout: &out, Stderr: &out}, "true")
	if err != nil {
		t.Fatalf("smolvm now returns an error for a missing sandbox (%v) — boxer can skip its "+
			"existence check; see the comment above", err)
	}
	if code == 0 {
		t.Fatalf("a missing sandbox reported success, which no caller can act on")
	}
	if !strings.Contains(out.String(), "not found") {
		t.Fatalf("the only signal boxer has is this text, and it changed: %q", out.String())
	}
}
