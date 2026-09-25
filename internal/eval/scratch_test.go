package eval

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneScratchRemovesOnlyOldEvalDirectories(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	old := time.Now().Add(-4 * 24 * time.Hour)
	mk := func(name string, when time.Time) string {
		p := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Join(p, "node_modules"), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, when, when)
		return p
	}
	gone := []string{mk("boxer-sdlc-base-1", old), mk("bxm-2", old), mk("boxer-eval-3", old)}
	kept := []string{mk("boxer-sdlc-base-4", time.Now()), mk("boxer-eval-packs", old), mk("someone-elses-9", old)}
	if n := PruneScratch(time.Now()); n != len(gone) {
		t.Fatalf("pruned %d, want %d", n, len(gone))
	}
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("old eval scratch survived: %s", p)
		}
	}
	for _, p := range kept {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("removed what is not old eval scratch: %s", p)
		}
	}
}
