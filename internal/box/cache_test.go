package box

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
)

func cacheEnv(t *testing.T, cfg config.Config, lockfiles ...string) *Env {
	t.Helper()
	root := t.TempDir()
	for _, f := range lockfiles {
		if err := os.WriteFile(filepath.Join(root, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Env{Cfg: cfg, Scope: scope.Scope{Root: root}}
}

// A cache is offered only when the project proves it uses that manager *and* the host actually
// has one. Mounting a path that does not exist is how you hand a backend an error instead of a
// sandbox, and mounting a cache for a manager the project does not use is noise.
func TestOnlyCachesThatBothSidesHaveAreMounted(t *testing.T) {
	cfg := config.Defaults()
	cfg.Cache.Enabled = true
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".npm"), 0o755); err != nil {
		t.Fatal(err)
	}
	// ~/.cargo/registry deliberately not created: the lockfile is there, the cache is not.

	got := cacheEnv(t, cfg, "package-lock.json", "Cargo.lock").CacheMounts()
	if len(got) != 1 || !strings.HasSuffix(got[0], ":/root/.npm:ro") {
		t.Fatalf("want only the npm cache, got %v", got)
	}
	if !strings.HasPrefix(got[0], home) {
		t.Errorf("mount does not come from the host cache: %v", got)
	}

	// No lockfile at all: nothing to offer.
	if got := cacheEnv(t, cfg).CacheMounts(); len(got) != 0 {
		t.Errorf("a project with no lockfile got %v", got)
	}
}

// Read-only is the contract. A writable shared cache across concurrent sandboxes is a corruption
// question, and this test is what stops someone making it writable without answering that.
func TestCacheMountsAreReadOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".npm"), 0o755); err != nil {
		t.Fatal(err)
	}
	on := config.Defaults()
	on.Cache.Enabled = true
	for _, m := range cacheEnv(t, on, "package-lock.json").CacheMounts() {
		if !strings.HasSuffix(m, ":ro") {
			t.Errorf("%q is not read-only", m)
		}
	}
}

// Turning it off, and naming managers explicitly, both have to work — the second because someone
// will want their npm cache shared and their cargo cache not.
func TestCacheCanBeDisabledOrNarrowed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, d := range []string{".npm", ".cache/yarn"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Off by default: a read-only cache at the tool's own path breaks installing anything the
	// host has not cached (npm fails with EROFS), so mounting one has to be a choice.
	if got := cacheEnv(t, config.Defaults(), "package-lock.json").CacheMounts(); len(got) != 0 {
		t.Errorf("off by default, but mounted %v", got)
	}

	narrow := config.Defaults()
	narrow.Cache.Enabled = true
	narrow.Cache.Managers = []string{"yarn"}
	got := cacheEnv(t, narrow, "package-lock.json", "yarn.lock").CacheMounts()
	if len(got) != 1 || !strings.Contains(got[0], "yarn") {
		t.Errorf("want only yarn, got %v", got)
	}
}
