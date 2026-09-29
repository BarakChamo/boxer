package boxer_test

import (
	"testing"

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
