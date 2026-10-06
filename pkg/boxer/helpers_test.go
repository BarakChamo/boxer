package boxer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
	"github.com/BarakChamo/boxer/pkg/boxer"
)

func TestPortsAndURLs(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := boxer.Machine{Name: "sb-1", Labels: map[string]string{"boxer.port.3000": "41234", "boxer.scope": "sb-1"}}
	if p := boxer.Ports(m); len(p) != 1 || p["3000"] != "41234" {
		t.Fatalf("Ports: %v", p)
	}
	if u := boxer.URLs(m); u["3000"] != "http://127.0.0.1:41234" {
		t.Fatalf("URLs: %v", u)
	}
	if p := boxer.Ports(boxer.Machine{}); len(p) != 0 {
		t.Fatalf("no ports: %v", p)
	}
}

// ListAll tags every machine with its backend; with nothing installed it is empty, not an error.
func TestListAllTagsTheBackend(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ms, err := boxer.ListAll()
	if err != nil || len(ms) != 0 {
		t.Fatalf("no backends installed: %v %v", ms, err)
	}
}

func TestListAllFindsTheSandbox(t *testing.T) {
	client, _ := vmtest.Install(t)
	if err := client.Create(vm.CreateSpec{Name: "sb-listall", Labels: map[string]string{"boxer.scope": "sb-listall"}}); err != nil {
		t.Fatal(err)
	}
	// Other installed backends are listed too, read-only; one that is not answering is an error
	// beside the result, not instead of it.
	ms, _ := boxer.ListAll()
	for _, m := range ms {
		if m.Name == "sb-listall" {
			if m.Backend != "smolvm" {
				t.Fatalf("backend = %q", m.Backend)
			}
			return
		}
	}
	t.Fatalf("ListAll missed the smolvm sandbox: %v", ms)
}

// Delete clears what boxer kept about the sandbox, as `boxer rm` does, and keeps its volumes.
func TestDeleteForgetsTheScope(t *testing.T) {
	client, _ := vmtest.Install(t)
	if err := client.Create(vm.CreateSpec{Name: "sb-del", Labels: map[string]string{"boxer.scope": "sb-del"}}); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(box.LastUsedDir(), "sb-del")
	vol := filepath.Join(box.VolumeDir(), "sb-del", "data")
	for _, d := range []string{filepath.Dir(stamp), vol} {
		os.MkdirAll(d, 0o755)
	}
	os.WriteFile(stamp, nil, 0o644)
	if err := boxer.Delete("sb-del"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := client.Status("sb-del"); ok {
		t.Fatal("the machine must be gone")
	}
	if _, err := os.Stat(stamp); !os.IsNotExist(err) {
		t.Fatal("the last-used stamp must go with it")
	}
	if _, err := os.Stat(vol); err != nil {
		t.Fatal("volumes are kept")
	}
}

// Stop and Delete refuse a machine boxer did not make, as `boxer stop` and `boxer rm` do.
func TestStopAndDeleteRefuseForeignMachines(t *testing.T) {
	client, _ := vmtest.Install(t)
	if err := client.Create(vm.CreateSpec{Name: "mydev"}); err != nil {
		t.Fatal(err)
	}
	if err := boxer.Delete("mydev"); err == nil {
		t.Fatal("Delete must refuse a machine without boxer's label")
	}
	if err := boxer.Stop("mydev"); err == nil {
		t.Fatal("Stop must refuse a machine without boxer's label")
	}
	if _, ok, _ := client.Status("mydev"); !ok {
		t.Fatal("the foreign machine was deleted")
	}
}
