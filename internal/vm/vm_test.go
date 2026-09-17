package vm_test

import (
	"os"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func TestLifecycleAgainstFake(t *testing.T) {
	c, log := vmtest.Install(t)
	if _, ok, _ := c.Status("sb-x"); ok {
		t.Fatal("should not exist yet")
	}
	err := c.Create(vm.CreateSpec{Name: "sb-x", Image: "alpine", Volumes: []string{"/tmp/wt:/workspace"}, Labels: map[string]string{"boxer.scope": "sb-x"}, CPUs: 2, MemoryMiB: 1024, Network: "allowlist", AllowHosts: []string{"registry-1.docker.io"}})
	if err != nil {
		t.Fatal(err)
	}
	m, ok, _ := c.Status("sb-x")
	if !ok || m.Running() {
		t.Fatalf("created but stopped expected: %+v", m)
	}
	if err := c.Start("sb-x", false); err != nil {
		t.Fatal(err)
	}
	m, _, _ = c.Status("sb-x")
	if !m.Running() {
		t.Fatal("running expected")
	}
	out, code, err := c.Output("sb-x", "/workspace", "sh", "-c", "echo hi; exit 3")
	if err != nil || code != 3 || strings.TrimSpace(out) != "hi" {
		t.Fatalf("exec passthrough: %q %d %v", out, code, err)
	}
	owned, _ := c.Owned()
	if len(owned) != 1 || owned[0].Labels["boxer.scope"] != "sb-x" {
		t.Fatalf("owned: %+v", owned)
	}
	if err := c.Delete("sb-x"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Status("sb-x"); ok {
		t.Fatal("deleted")
	}
	b, _ := os.ReadFile(log)
	s := string(b)
	for _, want := range []string{"-I alpine", "-v /tmp/wt:/workspace", "--label boxer.scope=sb-x", "--allow-host registry-1.docker.io", "--cpus 2", "--mem 1024", "exec --name sb-x -i -e BOXER_INSIDE=1 -w /workspace --", "delete -n sb-x --force --cascade"} {
		if !strings.Contains(s, want) {
			t.Errorf("smolvm never invoked with %q\nlog:\n%s", want, s)
		}
	}
	if strings.Contains(s, "--net") {
		t.Error("allowlist mode must not pass --net")
	}
}

func TestMissingBinaryMessage(t *testing.T) {
	c := vm.Client{Bin: "definitely-not-smolvm-xyz"}
	_, err := c.Version()
	if err == nil || !strings.Contains(err.Error(), "install") {
		t.Fatalf("want install hint, got %v", err)
	}
}
