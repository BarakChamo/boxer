package main

import (
	"os"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// A named pack is an artifact someone meant to keep, so the sweep that reclaims the automatic
// pack cache must never take it. That is the whole reason named packs live in their own
// directory, and it is the regression worth a test.
func TestNamedPackSurvivesGCAndRecreatesTheSandbox(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)

	if code, out := call(t, nil, "pack", "save", "base"); code != 1 || !strings.Contains(out, "NO_SANDBOX") {
		t.Fatalf("there is nothing to save before `up`: %d %s", code, out)
	}
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	if code, out := call(t, nil, "pack", "save", "base"); code != 0 || !strings.Contains(out, "saved pack base") {
		t.Fatalf("save: %d %s", code, out)
	}
	var rows []box.NamedPack
	if code, _ := call(t, &rows, "pack", "ls", "--json"); code != 0 || len(rows) != 1 || rows[0].Name != "base" {
		t.Fatalf("ls: %d %v", code, rows)
	}
	if code, out := call(t, nil, "gc", "--all"); code != 0 {
		t.Fatalf("gc: %d %s", code, out)
	}
	if _, err := os.Stat(rows[0].Path); err != nil {
		t.Fatalf("gc --all must not reclaim a named pack: %v", err)
	}
	code, out := call(t, nil, "pack", "use", "base")
	if code != 0 || !strings.Contains(out, "recreated from pack base") {
		t.Fatalf("use: %d %s", code, out)
	}
	if code, out := call(t, nil, "pack", "rm", "base"); code != 0 {
		t.Fatalf("rm: %d %s", code, out)
	}
	if code, out := call(t, nil, "pack", "use", "base"); code != 1 || !strings.Contains(out, "NO_SUCH_PACK") {
		t.Fatalf("a pack that is gone is an error, not a silent pull: %d %s", code, out)
	}
	// A name that would escape its directory is refused before it becomes a path.
	if code, out := call(t, nil, "pack", "save", "../escape"); code != 2 || !strings.Contains(out, "pack name") {
		t.Fatalf("name validation: %d %s", code, out)
	}
}
