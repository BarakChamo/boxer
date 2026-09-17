// Package config resolves boxer.toml from the user, repository, and worktree layers and records
// where every value came from so `doctor` can explain it (R-CFG-1).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Network mirrors the [network] table.
type Network struct {
	Mode       string   `toml:"mode"`
	AllowHosts []string `toml:"allow_hosts"`
	Ports      []string `toml:"ports"`
}

// Branch mirrors the [branch] table. Inert in v1.
type Branch struct {
	Enabled bool     `toml:"enabled"`
	Base    string   `toml:"base"`
	Warm    []string `toml:"warm"`
}

// Worktree mirrors the [worktree] table. Manage is "off" (boxer only reacts to worktrees others
// create) or "detect" (a session in the main checkout shares the repository sandbox, with a
// warning, until it moves into a linked worktree).
type Worktree struct {
	Manage string `toml:"manage"`
}

// Override is a [harness.<name>] table: the keys a harness may change.
type Override struct {
	Mode        string `toml:"mode"`
	Enforcement string `toml:"enforcement"`
	Isolation   string `toml:"isolation"`
}

// Config is the fully resolved configuration.
type Config struct {
	Isolation             string   `toml:"isolation"`
	OnMissingID           string   `toml:"on_missing_id"`
	RequireWorktree       string   `toml:"require_worktree"`
	RequireLinkedWorktree bool     `toml:"require_linked_worktree"`
	CreateOn              []string `toml:"create_on"`
	DestroyOn             []string `toml:"destroy_on"`
	IdleTimeout           string   `toml:"idle_timeout"`
	ReuseExisting         bool     `toml:"reuse_existing"`
	// WarmOnSessionStart makes the SessionStart hook provision in a detached `boxer up` and
	// return at once, so the session is never blocked on a VM create.
	WarmOnSessionStart bool `toml:"warm_on_session_start"`

	Integration          string   `toml:"integration"` // outside | inside
	Mode                 string   `toml:"mode"`
	Enforcement          string   `toml:"enforcement"`
	OnSandboxUnavailable string   `toml:"on_sandbox_unavailable"`
	Intercept            []string `toml:"intercept"`
	Passthrough          []string `toml:"passthrough"`

	Image          string   `toml:"image"`
	Smolfile       string   `toml:"smolfile"`
	Setup          []string `toml:"setup"`
	MountAt        string   `toml:"mount_at"`
	CPUs           int      `toml:"cpus"`
	Memory         string   `toml:"memory"`
	EnvPassthrough []string `toml:"env_passthrough"`
	Secrets        []string `toml:"secrets"`

	Network  Network             `toml:"network"`
	Branch   Branch              `toml:"branch"`
	Worktree Worktree            `toml:"worktree"`
	Harness  map[string]Override `toml:"harness"`

	// Sources maps a top-level key to the file or "env" or "default" it came from.
	Sources map[string]string `toml:"-"`
	// Files lists the configuration files that were read, lowest priority first.
	Files []string `toml:"-"`
}

// Defaults are the values with no file present.
func Defaults() Config {
	return Config{
		Isolation:             "worktree",
		OnMissingID:           "degrade",
		RequireWorktree:       "warn",
		CreateOn:              []string{"session_start", "run", "mcp"},
		DestroyOn:             []string{},
		IdleTimeout:           "2h",
		ReuseExisting:         true,
		Integration:           "outside",
		Mode:                  "rewrite",
		Enforcement:           "both",
		OnSandboxUnavailable:  "fail",
		Intercept:             []string{"npm", "npx", "pnpm", "yarn", "bun", "bunx", "node", "deno", "python", "python3", "pip", "uv", "pytest", "cargo", "rustc", "go", "make", "cmake"},
		Passthrough:           []string{"git", "gh", "ssh", "boxer", "smolvm"},
		MountAt:               "/workspace",
		CPUs:                  4,
		Memory:                "4G",
		EnvPassthrough:        []string{"CI"},
		Network:               Network{Mode: "allowlist", AllowHosts: []string{}},
		Worktree:              Worktree{Manage: "off"},
		Harness:               map[string]Override{},
		Sources:               map[string]string{},
		RequireLinkedWorktree: false,
	}
}

// Load resolves configuration for a checkout. worktreeRoot and repoRoot may be equal or empty.
func Load(worktreeRoot, repoRoot string) (Config, error) {
	cfg := Defaults()
	var paths []string
	if u := userFile(); u != "" {
		paths = append(paths, u)
	}
	if repoRoot != "" {
		paths = append(paths, filepath.Join(repoRoot, "boxer.toml"))
	}
	if worktreeRoot != "" && worktreeRoot != repoRoot {
		paths = append(paths, filepath.Join(worktreeRoot, "boxer.toml"))
	}
	for _, p := range paths {
		if err := cfg.merge(p); err != nil {
			return cfg, err
		}
	}
	cfg.applyEnv()
	return cfg, cfg.Validate()
}

// LoadFiles is Load with explicit files, for tests and `doctor --config`.
func LoadFiles(paths ...string) (Config, error) {
	cfg := Defaults()
	for _, p := range paths {
		if err := cfg.merge(p); err != nil {
			return cfg, err
		}
	}
	cfg.applyEnv()
	return cfg, cfg.Validate()
}

func userFile() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "boxer", "boxer.toml")
}

func (c *Config) merge(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	md, err := toml.Decode(string(data), c)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if und := md.Undecoded(); len(und) > 0 {
		names := make([]string, len(und))
		for i, k := range und {
			names[i] = k.String()
		}
		return fmt.Errorf("%s: unknown key(s): %s", path, strings.Join(names, ", "))
	}
	for _, k := range md.Keys() {
		if len(k) > 0 {
			c.Sources[k[0]] = path
		}
	}
	c.Files = append(c.Files, path)
	return nil
}

// envKeys are the scalar settings that may be overridden from the environment (R-CFG: every key
// settable by environment for harnesses that only offer environment control). Lists are not
// overridable; they are repository policy.
var envKeys = map[string]func(c *Config, v string) error{
	"BOXER_ISOLATION":              func(c *Config, v string) error { c.Isolation = v; return nil },
	"BOXER_ON_MISSING_ID":          func(c *Config, v string) error { c.OnMissingID = v; return nil },
	"BOXER_REQUIRE_WORKTREE":       func(c *Config, v string) error { c.RequireWorktree = v; return nil },
	"BOXER_MODE":                   func(c *Config, v string) error { c.Mode = v; return nil },
	"BOXER_INTEGRATION":            func(c *Config, v string) error { c.Integration = v; return nil },
	"BOXER_ENFORCEMENT":            func(c *Config, v string) error { c.Enforcement = v; return nil },
	"BOXER_ON_SANDBOX_UNAVAILABLE": func(c *Config, v string) error { c.OnSandboxUnavailable = v; return nil },
	"BOXER_IMAGE":                  func(c *Config, v string) error { c.Image = v; return nil },
	"BOXER_SMOLFILE":               func(c *Config, v string) error { c.Smolfile = v; return nil },
	"BOXER_MOUNT_AT":               func(c *Config, v string) error { c.MountAt = v; return nil },
	"BOXER_MEMORY":                 func(c *Config, v string) error { c.Memory = v; return nil },
	"BOXER_CPUS": func(c *Config, v string) error {
		n, err := strconv.Atoi(v)
		c.CPUs = n
		return err
	},
	"BOXER_NETWORK_MODE":    func(c *Config, v string) error { c.Network.Mode = v; return nil },
	"BOXER_WORKTREE_MANAGE": func(c *Config, v string) error { c.Worktree.Manage = v; return nil },
	"BOXER_WARM_ON_SESSION_START": func(c *Config, v string) error {
		b, err := strconv.ParseBool(v)
		c.WarmOnSessionStart = b
		return err
	},
}

func (c *Config) applyEnv() {
	for name, set := range envKeys {
		if v, ok := os.LookupEnv(name); ok {
			if err := set(c, v); err == nil {
				c.Sources[strings.ToLower(strings.TrimPrefix(name, "BOXER_"))] = "env"
			}
		}
	}
}

// ForHarness returns a copy with the [harness.<name>] override applied.
func (c Config) ForHarness(name string) Config {
	o, ok := c.Harness[name]
	if !ok {
		return c
	}
	out := c
	if o.Mode != "" {
		out.Mode = o.Mode
		out.Sources["mode"] = "harness." + name
	}
	if o.Enforcement != "" {
		out.Enforcement = o.Enforcement
		out.Sources["enforcement"] = "harness." + name
	}
	if o.Isolation != "" {
		out.Isolation = o.Isolation
		out.Sources["isolation"] = "harness." + name
	}
	return out
}

// Validate rejects values outside their enumerations so a typo cannot silently disable
// enforcement (R-CFG-2).
func (c Config) Validate() error {
	checks := []struct {
		key, val string
		allowed  []string
	}{
		{"isolation", c.Isolation, []string{"repo", "worktree", "session", "subagent"}},
		{"on_missing_id", c.OnMissingID, []string{"degrade", "fail"}},
		{"require_worktree", c.RequireWorktree, []string{"off", "warn", "require"}},
		{"integration", c.Integration, []string{"outside", "inside"}},
		{"mode", c.Mode, []string{"rewrite", "tool", "off"}},
		{"enforcement", c.Enforcement, []string{"hook", "shim", "both", "audit"}},
		{"on_sandbox_unavailable", c.OnSandboxUnavailable, []string{"fail", "passthrough"}},
		{"network.mode", c.Network.Mode, []string{"off", "allowlist", "on"}},
		{"worktree.manage", c.Worktree.Manage, []string{"off", "detect"}},
	}
	for _, ch := range checks {
		if !oneOf(ch.val, ch.allowed) {
			return fmt.Errorf("%s = %q; allowed: %s", ch.key, ch.val, strings.Join(ch.allowed, " | "))
		}
	}
	for _, e := range c.CreateOn {
		if !oneOf(e, []string{"session_start", "subagent_start", "run", "mcp"}) {
			return fmt.Errorf("create_on contains %q; allowed: session_start | subagent_start | run | mcp", e)
		}
	}
	for _, e := range c.DestroyOn {
		if !oneOf(e, []string{"session_end", "subagent_stop", "never"}) {
			return fmt.Errorf("destroy_on contains %q; allowed: session_end | subagent_stop | never", e)
		}
	}
	for name, o := range c.Harness {
		if o.Mode != "" && !oneOf(o.Mode, []string{"rewrite", "tool", "off"}) {
			return fmt.Errorf("harness.%s.mode = %q", name, o.Mode)
		}
	}
	if _, err := MemoryMiB(c.Memory); err != nil {
		return err
	}
	return nil
}

// MemoryMiB parses "4G", "4096M", "4096" (MiB).
func MemoryMiB(s string) (int, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	mult := 1
	switch {
	case strings.HasSuffix(s, "G"), strings.HasSuffix(s, "GB"), strings.HasSuffix(s, "GIB"):
		mult = 1024
		s = strings.TrimRight(s, "GIB")
	case strings.HasSuffix(s, "M"), strings.HasSuffix(s, "MB"), strings.HasSuffix(s, "MIB"):
		s = strings.TrimRight(s, "MIB")
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("memory %q: want e.g. 4G or 4096M", s)
	}
	return n * mult, nil
}

// Has reports whether list contains v.
func Has(list []string, v string) bool {
	return oneOf(v, list)
}

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}
