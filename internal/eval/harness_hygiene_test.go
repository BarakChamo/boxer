package eval

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every harness that makes a scratch directory has to remove it.
//
// This is a source-derived check rather than a convention, because the convention failed: all six
// benchmark and smoke scripts created temp trees with `mktemp -d`, all six had a cleanup trap, and
// not one of them removed the tree. Each cell left behind a reference worktree with a full
// node_modules — about a gigabyte — under $TMPDIR, which macOS only prunes for files older than
// three days. It went unnoticed until 724 directories and 84 GB filled the disk and the machine
// stopped being able to write at all.
//
// The failure mode is what makes it worth a test: nothing was broken, every suite passed, and the
// only symptom was a number on a disk nobody was watching.
func TestEveryHarnessRemovesItsScratchDirectory(t *testing.T) {
	root := repoRootForHygiene(t)
	var checked int
	for _, dir := range []string{"bench", "evals"} {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".sh") {
				continue
			}
			path := filepath.Join(root, dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)
			if !strings.Contains(src, "mktemp -d") {
				continue
			}
			checked++
			rel := filepath.Join(dir, e.Name())
			if !regexp.MustCompile(`rm -rf [^\n]*\$WORK`).MatchString(src) {
				t.Errorf("%s calls `mktemp -d` but never removes $WORK; every run will leak it", rel)
			}
			if !strings.Contains(src, "trap cleanup EXIT") && !strings.Contains(src, "trap ") {
				t.Errorf("%s allocates a scratch directory with no EXIT trap to remove it", rel)
			}
			// Anything allocated before the script re-execs under the host lock is orphaned by
			// `exec`, which replaces the process and takes the trap with it. smoke.sh leaked two
			// directories per run that way for as long as it has existed.
			if i := strings.Index(src, "exec \"$EVAL\""); i >= 0 {
				if j := strings.Index(src, "mktemp -d"); j >= 0 && j < i {
					t.Errorf("%s runs `mktemp -d` before it re-execs; `exec` orphans it with no trap", rel)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no harness scripts were checked; the walk is looking in the wrong place")
	}
}

func repoRootForHygiene(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the repository root")
	return ""
}
