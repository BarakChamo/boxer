package vm_test

import (
	"fmt"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
)

// TestPrintCapabilityMatrix is not an assertion — it prints the capability table that the
// documentation quotes, straight from the code, so the table in the docs can be regenerated
// rather than retyped. Run with: go test ./internal/vm -run CapabilityMatrix -v
func TestPrintCapabilityMatrix(t *testing.T) {
	yn := func(v bool) string {
		if v {
			return "yes"
		}
		return "—"
	}
	fmt.Printf("| backend | boundary | worktree mount | egress allowlist | secrets off argv | proves ownership | env cache | fork | egress log |\n")
	fmt.Printf("| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, n := range []string{"smolvm", "container", "docker", "podman"} {
		b, err := vm.Default(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		c := vm.CapsOf(b)
		fmt.Printf("| `%s` | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			c.Backend, c.Boundary, yn(c.HostMounts), yn(c.Allowlist), yn(c.SecretEnv),
			yn(c.ProvesOwnership), yn(c.Packs), yn(c.Branch), yn(c.Egress))
	}
}
