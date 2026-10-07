package box

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

// Code in the guest can write boxer.toml. Until a person approves it with trust, host-affecting
// keys are held back: prep does not run on the host, and the sandbox uses boxer's defaults.
func TestHostKeysAreHeldBackUntilTrusted(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("BOXER_TRUST", "") // the harness default would trust everything
	ran := filepath.Join(t.TempDir(), "prep-ran")
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"mounts = [\"/etc:/host-etc\"]\n[prep]\ncommands = [\"touch "+ran+"\"]\n")

	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Cfg.Prep.Commands) != 0 || len(e.Cfg.Mounts) != 0 {
		t.Fatal("untrusted host keys must be held back")
	}
	if len(e.Warnings) == 0 {
		t.Fatal("holding keys back must warn")
	}
	if err := e.Prep(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("untrusted prep ran on the host")
	}

	// Approve the real configuration (the digest comes from the file, not the neutered cfg).
	trustFromRepo(t, dir, e.Scope.Key)
	e2, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e2.Cfg.Prep.Commands) != 1 || len(e2.Cfg.Mounts) != 1 {
		t.Fatalf("a trusted configuration takes effect: %+v", e2.Cfg.Prep)
	}
	if err := e2.Prep(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ran); err != nil {
		t.Fatal("trusted prep must run")
	}
}

// Nothing host-affecting means nothing to trust: an ordinary repository is not gated.
func TestPlainConfigNeedsNoTrust(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("BOXER_TRUST", "")
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"image = \"alpine\"\ncpus = 2\n")
	e, err := Resolve(dir, "", scope.Identity{})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range e.Warnings {
		if len(w) > 0 && w[:4] == "this" {
			t.Fatalf("a plain configuration must not be gated: %q", w)
		}
	}
}

func trustFromRepo(t *testing.T, dir, key string) {
	t.Helper()
	cfg, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Trust(key, cfg.HostDigest()); err != nil {
		t.Fatal(err)
	}
}
