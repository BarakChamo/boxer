package box

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

func TestOrphanVolumesAndRemove(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	live, gone := t.TempDir(), filepath.Join(t.TempDir(), "removed")
	for key, root := range map[string]string{"sb-live": live, "sb-gone": gone} {
		d := filepath.Join(VolumeDir(), key)
		os.MkdirAll(filepath.Join(d, "pg"), 0o755)
		os.WriteFile(filepath.Join(d, rootFile), []byte(root), 0o644)
	}
	os.MkdirAll(filepath.Join(VolumeDir(), "sb-norecord"), 0o755)
	if got := OrphanVolumes(); !slices.Equal(got, []string{"sb-gone"}) {
		t.Fatalf("orphans = %v", got)
	}
	for _, bad := range []string{"", "../x", "a/b"} {
		if err := RemoveVolumes(bad); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(VolumeDir()); err != nil {
		t.Fatal("a bad key must never remove anything")
	}
	if err := RemoveVolumes("sb-gone"); err != nil {
		t.Fatal(err)
	}
	if got := OrphanVolumes(); len(got) != 0 {
		t.Fatalf("still there: %v", got)
	}
}

func TestPruneArchivesKeepsTheNewest(t *testing.T) {
	dir := t.TempDir()
	for i, n := range []string{"boxer-a-1.tar", "boxer-a-2.tar", "boxer-a-3.tar", "boxer-b-1.tar"} {
		p := filepath.Join(dir, n)
		os.WriteFile(p, nil, 0o644)
		at := time.Now().Add(time.Duration(i) * time.Minute)
		os.Chtimes(p, at, at)
	}
	pruneArchives(dir, "boxer-a", 2)
	var left []string
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		left = append(left, e.Name())
	}
	if !slices.Equal(left, []string{"boxer-a-2.tar", "boxer-a-3.tar", "boxer-b-1.tar"}) {
		t.Fatalf("left %v", left)
	}
	if !mtime(filepath.Join(dir, "missing")).IsZero() {
		t.Fatal("a missing file has no time")
	}
}

func TestReachableURLs(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := vm.Machine{Name: "sb-x", Labels: map[string]string{"boxer.port.3000": "4100", "boxer.port.8080": "4200"}}
	u := ReachableURLs(m)
	if u["3000"] != "http://127.0.0.1:4100" || u["8080"] != "http://127.0.0.1:4200" {
		t.Fatalf("got %v", u)
	}
	if len(ReachableURLs(vm.Machine{})) != 0 {
		t.Fatal("no ports, no URLs")
	}
}
