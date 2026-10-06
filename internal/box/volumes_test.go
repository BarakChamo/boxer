package box

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// Polling is on by default where host edits do not reach dev-server watchers, and a value in
// [env] always wins, including an empty one that turns it off.
func TestGuestEnvPollsWhereWatchersMissHostEdits(t *testing.T) {
	e := &Env{}
	for backend, want := range map[string]bool{"smolvm": true, "": true, "container": true, "docker": false} {
		e.Cfg.Backend, e.Cfg.Env = backend, nil
		got := strings.Join(e.GuestEnv(), " ")
		if has := strings.Contains(got, "WATCHPACK_POLLING=true") && strings.Contains(got, "CHOKIDAR_USEPOLLING=1"); has != want {
			t.Errorf("%q: %q", backend, got)
		}
	}
	if PollsForHostEdits("podman", "darwin") == PollsForHostEdits("podman", "linux") {
		t.Error("podman polls on a Mac only")
	}
	e.Cfg.Backend, e.Cfg.Env = "smolvm", map[string]string{"WATCHPACK_POLLING": "", "NODE_ENV": "development"}
	got := e.GuestEnv()
	if strings.Join(got, " ") != "NODE_ENV=development WATCHPACK_POLLING= CHOKIDAR_USEPOLLING=1" {
		t.Fatalf("[env] must win: %v", got)
	}
}

// A lock file deleted while a process waits on it must not give the scope two holders.
func TestLockSurvivesItsFileBeingRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb-x")
	unlock, err := lockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan func(), 1)
	go func() {
		u, err := lockFile(path) // waits on the file the holder has locked
		if err == nil {
			got <- u
		}
	}()
	time.Sleep(100 * time.Millisecond)
	os.Remove(path) // the sweep, while the holder still holds it
	unlock()
	second := <-got // the waiter must end up holding the current path's lock
	// A third locker must now wait for the second, not take a fresh file of its own.
	third := make(chan struct{})
	go func() {
		u, err := lockFile(path)
		if err == nil {
			close(third)
			u()
		}
	}()
	select {
	case <-third:
		t.Fatal("two processes hold the scope lock")
	case <-time.After(200 * time.Millisecond):
	}
	second()
	<-third
}

// A long command keeps its scope looking used, so idle reclaim does not stop it mid-run.
func TestKeepAliveRefreshesLastUsed(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	old := keepAliveEvery
	keepAliveEvery = 20 * time.Millisecond
	t.Cleanup(func() { keepAliveEvery = old })
	e := &Env{}
	e.Scope.Key = "sb-long"
	stop := e.KeepAlive()
	first := LastUsed("sb-long")
	time.Sleep(150 * time.Millisecond)
	if !LastUsed("sb-long").After(first) {
		t.Fatal("a running command must keep refreshing last-used")
	}
	stop()
}
