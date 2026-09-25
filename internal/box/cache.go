package box

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Package caches, mounted read-only from the host.
//
// This is the cheap half of "do the work before the sandbox exists". The expensive part of
// `npm install` in a fresh guest is not the install, it is the download: the guest starts with an
// empty cache and fetches every tarball again over a network it shares with nothing. The host has
// already downloaded most of them.
//
// Mounting that cache in keeps the install *inside* the guest, which is the part that matters for
// correctness: every binary is still chosen and unpacked for the guest's own platform, so nothing
// here can produce the `invalid ELF header` that a host-side install risks. It is strictly a
// speed change with no semantic one, which is why it is the default and `[prep]` is not.
//
// Read-only on purpose, and the limit is worth stating: npm wants to *write* to its cache, so a
// read-only mount makes the host cache a source to read from rather than a cache to fill. The
// guest still keeps its own writable cache and still benefits. A writable shared mount across
// several concurrent sandboxes is a corruption question rather than a performance one, and is not
// taken here without measuring it.
type cacheMount struct {
	lockfile string // what proves the project uses this manager
	host     string // where the host keeps it; ~ is expanded, $(cmd) is asked
	guest    string // where the guest's tool looks
}

// The guest paths assume the tool runs as root, which is what every image boxer detects does.
var cacheMounts = []cacheMount{
	{"package-lock.json", "~/.npm", "/root/.npm"},
	{"pnpm-lock.yaml", "~/Library/pnpm/store", "/root/.local/share/pnpm/store"},
	{"yarn.lock", "~/.cache/yarn", "/root/.cache/yarn"},
	{"bun.lock", "~/.bun/install/cache", "/root/.bun/install/cache"},
	{"bun.lockb", "~/.bun/install/cache", "/root/.bun/install/cache"},
	{"uv.lock", "~/.cache/uv", "/root/.cache/uv"},
	{"poetry.lock", "~/Library/Caches/pypoetry", "/root/.cache/pypoetry"},
	{"Cargo.lock", "~/.cargo/registry", "/usr/local/cargo/registry"},
	{"go.sum", "$GOMODCACHE", "/go/pkg/mod"},
}

// CacheMounts returns the read-only host cache mounts for this worktree, in `host:guest:ro` form.
//
// Only caches that exist are mounted. A backend is free to reject a mount whose source is absent,
// and a project that has never used a manager has no cache worth offering anyway.
func (e *Env) CacheMounts() []string {
	if !e.Cfg.Cache.Enabled {
		return nil
	}
	want := map[string]bool{}
	for _, m := range e.Cfg.Cache.Managers {
		want[m] = true
	}
	auto := want["auto"] || len(e.Cfg.Cache.Managers) == 0

	var out []string
	seen := map[string]bool{}
	for _, c := range cacheMounts {
		if !auto && !want[managerOf(c.lockfile)] {
			continue
		}
		if _, err := os.Stat(filepath.Join(e.Scope.Root, c.lockfile)); err != nil {
			continue
		}
		host := resolveCacheDir(c.host)
		if host == "" || seen[c.guest] {
			continue
		}
		if st, err := os.Stat(host); err != nil || !st.IsDir() {
			continue
		}
		seen[c.guest] = true
		out = append(out, host+":"+c.guest+":ro")
	}
	return out
}

// managerOf names the package manager a lockfile belongs to, for `managers = [...]`.
func managerOf(lockfile string) string {
	switch lockfile {
	case "package-lock.json":
		return "npm"
	case "pnpm-lock.yaml":
		return "pnpm"
	case "yarn.lock":
		return "yarn"
	case "bun.lock", "bun.lockb":
		return "bun"
	case "uv.lock":
		return "uv"
	case "poetry.lock":
		return "poetry"
	case "Cargo.lock":
		return "cargo"
	case "go.sum":
		return "go"
	}
	return ""
}

// resolveCacheDir expands ~ and asks go where its module cache is, because that one is not a
// fixed path and guessing it wrong mounts an empty directory over a working cache.
func resolveCacheDir(p string) string {
	if p == "$GOMODCACHE" {
		out, err := exec.Command("go", "env", "GOMODCACHE").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return expandMount(p)
}
