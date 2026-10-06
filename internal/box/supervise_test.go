package box

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// The supervisor is a shell loop that runs in the guest; these run it for real, on the host's sh.
func superviseOnHost(t *testing.T, cmd, restart string) (runs int, out string) {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "runs")
	cmd = strings.ReplaceAll(cmd, "$RUNS", count)
	b, _ := exec.Command("sh", "-c", superviseScript(filepath.Join(dir, "pid"), cmd, restart)).CombinedOutput()
	n, _ := os.ReadFile(count)
	return strings.Count(string(n), "x"), string(b)
}

// A service that crashes comes back, with a line in the log each time, and one that then exits
// cleanly is done: exit 0 means it finished, not that it crashed.
func TestSupervisorRestartsACrashedService(t *testing.T) {
	runs, out := superviseOnHost(t, `echo x >> $RUNS; [ $(wc -l < $RUNS) -ge 3 ] && exit 0; exit 3`, "on-failure")
	if runs != 3 {
		t.Fatalf("ran %d times, want 3 (two crashes, then a clean exit):\n%s", runs, out)
	}
	if strings.Count(out, "exited 3; restarting in") != 2 || !strings.Contains(out, "restarting in 2s") {
		t.Fatalf("each restart must be logged, with the backoff doubling:\n%s", out)
	}
}

func TestSupervisorNeverRestartsWhenToldNot(t *testing.T) {
	if runs, out := superviseOnHost(t, `echo x >> $RUNS; exit 3`, "never"); runs != 1 {
		t.Fatalf(`restart = "never" ran it %d times:\n%s`, runs, out)
	}
}

// Five quick crashes in a row and it stops trying, so a service that cannot start does not spin.
// 1+2+4+8 seconds of backoff: skipped with -short.
func TestSupervisorGivesUp(t *testing.T) {
	if testing.Short() {
		t.Skip("waits 15s of backoff")
	}
	runs, out := superviseOnHost(t, `echo x >> $RUNS; exit 3`, "on-failure")
	if runs != 5 || !strings.Contains(out, "gave up after 5 quick crashes (exit 3): echo x") {
		t.Fatalf("ran %d times, want 5 and a give-up line:\n%s", runs, out)
	}
}

// `boxer restart` relaunches the services in the running sandbox, without recreating it.
func TestRestartServicesRelaunchesWithoutRecreating(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"start = [\"echo serving\"]\nready = \"true\"\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = io.Discard
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	if err := e.RestartServices(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	if n := launches(string(b)); n != 2 {
		t.Fatalf("restart must launch the services again, launched %d times:\n%s", n, b)
	}
	if n := strings.Count(string(b), "machine create"); n != 1 {
		t.Fatalf("restart must not recreate the sandbox, created %d times", n)
	}
	if err := e.Down(); err != nil {
		t.Fatal(err)
	}
	var be *Error
	if err := e.RestartServices(); !errors.As(err, &be) || be.Cause != "NO_SANDBOX" {
		t.Fatalf("restarting a sandbox that is not there must say so: %v", err)
	}
}

// A service another exec cannot signal (Docker under Ubuntu's AppArmor) is stopped by restarting
// the sandbox, and restart still relaunches it.
func TestRestartServicesFallsBackWhenAServiceWillNotStop(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"start = [\"echo serving\"]\nready = \"true\"\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	e.Stderr = io.Discard
	if _, err := e.Ensure(true, false); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(os.Getenv("FAKE_STATE")+".stuck", nil, 0o644)
	setBranchable(e.Scope.Key, true)
	before, _ := os.ReadFile(log)
	if err := e.RestartServices(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	after := string(b[len(before):])
	if !strings.Contains(after, "machine stop") || !strings.Contains(after, "machine start") {
		t.Fatalf("a stuck service must be stopped by restarting the sandbox:\n%s", after)
	}
	if n := launches(after); n != 1 {
		t.Fatalf("and relaunched once, launched %d times:\n%s", n, after)
	}
	if Branchable(e.Scope.Key) {
		t.Fatal("the restart ended the branchable boot, so the marker must go")
	}
}
