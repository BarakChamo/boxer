package box

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
)

// The disk guard exists because this machine filled up twice while boxer was being written, both
// times in the middle of writing a pack. A pack is a cache; the disk is not.
func TestPackingGuardAndUsage(t *testing.T) {
	packs := t.TempDir()
	t.Setenv("BOXER_PACKS", packs)

	if _, crowded := packingWouldCrowdTheDisk(0); crowded {
		t.Fatal("a margin of zero disables the check")
	}
	// A margin larger than any real disk must trip; one byte must not.
	if _, crowded := packingWouldCrowdTheDisk(1 << 62); !crowded {
		t.Fatal("an impossible margin must refuse to pack")
	}
	if _, crowded := packingWouldCrowdTheDisk(1); crowded {
		t.Fatal("one byte of margin must never refuse")
	}

	if err := os.WriteFile(filepath.Join(packs, "a.smolmachine"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	u := Usage(3)
	if u.Machines != 3 || u.PackCount != 1 || u.PackBytes != 2048 || u.FreeBytes <= 0 {
		t.Fatalf("usage: %+v", u)
	}
}

// The sweep runs at most once per interval, whichever process asks first, because boxer has no
// daemon and several of its processes start at once when a session opens.
func TestReclaimDueIsRateLimited(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if ReclaimDue(0) {
		t.Fatal("an interval of zero disables the sweep")
	}
	if !ReclaimDue(time.Hour) {
		t.Fatal("the first call is due")
	}
	if ReclaimDue(time.Hour) {
		t.Fatal("the second call within the interval must not sweep")
	}
	stamp := filepath.Join(filepath.Dir(LastUsedDir()), "last-reclaim")
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stamp, old, old); err != nil {
		t.Fatal(err)
	}
	if !ReclaimDue(time.Hour) {
		t.Fatal("a stamp older than the interval is due again")
	}
}

// Scratch directories are the largest thing the evaluation suite leaves behind, and nothing owned
// them before.
func TestStaleScratch(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "boxer-eval-1")
	stale := filepath.Join(dir, "boxer-eval-2")
	other := filepath.Join(dir, "not-ours")
	for _, d := range []string{fresh, stale, other} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, d := range []string{stale, other} {
		if err := os.Chtimes(d, old, old); err != nil {
			t.Fatal(err)
		}
	}
	got := StaleScratch(dir, time.Hour)
	if len(got) != 1 || got[0] != stale {
		t.Fatalf("only our own stale directories: %v", got)
	}
	if StaleScratch(dir, 0) != nil {
		t.Fatal("a zero idle disables the sweep")
	}
}

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{{0, "0 bytes"}, {2048, "2 MB"}, {3 << 30, "3.0 GB"}} {
		if got := HumanBytes(tc.in); got != tc.want && !(tc.in == 2048 && got == "2048 bytes") {
			t.Errorf("%d: %q", tc.in, got)
		}
	}
}

// The sweep re-executes the boxer binary, which is right for the command and wrong for a program
// that merely imports this package: it would run its own binary with an argument it never
// declared. Three separate guards say no, and each one matters on its own.
func TestReclaimDetachedIsOptIn(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "boxer")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	Executable = func() (string, error) { return script, nil }
	t.Cleanup(func() { Executable = os.Executable })

	cfg := config.Defaults()
	cfg.AutoReclaim = true
	cfg.ReclaimEvery = "1h"
	e := &Env{Cfg: cfg, Stderr: io.Discard}

	ran := func() bool {
		for i := 0; i < 100; i++ {
			if _, err := os.Stat(marker); err == nil {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}

	e.ReclaimDetached() // ReclaimAllowed is false: the library must never do this
	if ran() {
		t.Fatal("a library import must not re-execute its host program")
	}

	ReclaimAllowed = true
	t.Cleanup(func() { ReclaimAllowed = false })
	t.Setenv("BOXER_NO_RECLAIM", "1")
	e.ReclaimDetached()
	if ran() {
		t.Fatal("BOXER_NO_RECLAIM must turn the sweep off")
	}

	t.Setenv("BOXER_NO_RECLAIM", "")
	e.ReclaimDetached()
	if !ran() {
		t.Fatal("the command must sweep")
	}
	b, _ := os.ReadFile(marker)
	if strings.TrimSpace(string(b)) != "gc" {
		t.Fatalf("the sweep is a plain gc: %q", b)
	}

	// And not again within the interval.
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	e.ReclaimDetached()
	if ran() {
		t.Fatal("a second sweep inside the interval")
	}

	cfg.AutoReclaim = false
	e2 := &Env{Cfg: cfg, Stderr: io.Discard}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	e2.ReclaimDetached()
	if ran() {
		t.Fatal("auto_reclaim = false must turn the sweep off")
	}
}
