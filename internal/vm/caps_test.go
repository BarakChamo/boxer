package vm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// The capability set is derived from the method set, not declared, so that it cannot drift from
// what the backend can actually do. These two assertions are the whole contract: smolvm claims
// everything because it implements everything, and a backend that implements only the core claims
// only the core — without either of them having written a capability list.
func TestCapabilitiesComeFromTheMethodSetRatherThanAList(t *testing.T) {
	full := vm.CapsOf(vm.New())
	if !full.Packs || !full.Branch || !full.Egress || !full.DiskUsage {
		t.Errorf("smolvm implements all four optional interfaces, but claims %+v", full)
	}
	if full.Boundary != "kernel" || !full.HostMounts || !full.Allowlist || !full.SecretEnv {
		t.Errorf("smolvm's declared facts are wrong: %+v", full)
	}

	bare := vm.CapsOf(vmtest.NewBare())
	if bare.Packs || bare.Branch || bare.Egress || bare.DiskUsage {
		t.Errorf("a core-only backend claims an optional interface: %+v", bare)
	}
	// Declare supplies what no method set expresses, and must not be able to overwrite the name.
	if bare.Backend != "bare" || bare.Boundary != "namespace" {
		t.Errorf("declared facts not carried through: %+v", bare)
	}
	// Not declared by Bare, so it must read as absent rather than inheriting smolvm's default.
	if bare.Allowlist || bare.SecretEnv {
		t.Errorf("a backend that declared nothing inherited a capability it never claimed: %+v", bare)
	}
}

// A refusal has to name the backend, name the feature, and carry the fix — and be recognisable as
// a refusal rather than as a generic failure, so callers can tell "this cannot work here" from
// "this went wrong".
func TestARefusalNamesTheBackendTheFeatureAndTheFix(t *testing.T) {
	err := vm.Unsupported(vmtest.NewBare(), "fork a sandbox", "set backend = \"smolvm\"")
	if !errors.Is(err, vm.ErrUnsupported) {
		t.Fatalf("not recognisable as a refusal: %v", err)
	}
	for _, want := range []string{"bare", "fork a sandbox", "smolvm"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err, want)
		}
	}
}
