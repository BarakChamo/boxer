// Package config resolves boxer.toml from the user, repository, and worktree layers and records
// where every value came from so `doctor` can explain it (R-CFG-1).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Network mirrors the [network] table.
type Network struct {
	Mode       string   `toml:"mode"`
	AllowHosts []string `toml:"allow_hosts"`
	// AllowPresets names package ecosystems whose hosts join AllowHosts: "npm", "pypi", "github"
	// and the rest of Presets. A preset is a list boxer maintains, so a repository does not have
	// to learn which CDN a registry redirects to.
	AllowPresets []string `toml:"allow_presets"`
	Ports        []string `toml:"ports"`
	// DNS is the resolver a networked guest uses. Empty (the default) means the host's own
	// caching resolver, which is what makes name lookups cost milliseconds instead of ~400ms
	// each: smolvm otherwise points the guest at public resolvers, so every lookup is an
	// internet round trip and nothing caches. "off" restores that default; an address is used
	// as given, for a host whose resolver the guest cannot reach.
	DNS string `toml:"dns"`
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

// Telemetry mirrors the [telemetry] table: boxer's event stream, off by default.
//
// Note on compatibility: merge rejects unknown keys, so a repository that adds this table is
// rejected by an older binary. That is the same decision [tasks] faces and the two must be
// reconciled before release (a config_version key, or unknown *tables* relaxed to a warning).
type Telemetry struct {
	Enabled        bool   `toml:"enabled"`
	Sink           string `toml:"sink"` // none | file | stderr | otel
	Path           string `toml:"path"`
	RecordCommands bool   `toml:"record_commands"`
	Endpoint       string `toml:"endpoint"`
}

// Task is one [tasks] entry. A bare string is the command line; a table adds what an agent and a
// run need to know about it:
//
//	test = "go test ./..."
//
//	[tasks.e2e]
//	cmd         = "npm run e2e"
//	description = "the browser suite; needs the dev server"
//	junit       = ["reports/junit.xml"]
//	timeout     = "10m"
type Task struct {
	Cmd         string            `toml:"cmd"`
	Description string            `toml:"description"`
	JUnit       []string          `toml:"junit"`
	Timeout     string            `toml:"timeout"`
	Env         map[string]string `toml:"env"`
}

// UnmarshalTOML accepts both spellings. It also rejects an unknown key itself, because a type that
// decodes its own subtree is invisible to Undecoded(): without this, boxer's promise that a
// misspelled key is an error would quietly stop at the [tasks] boundary.
func (t *Task) UnmarshalTOML(v any) error {
	switch x := v.(type) {
	case string:
		t.Cmd = x
		return nil
	case map[string]any:
		for _, k := range sortedTaskKeys(x) {
			var err error
			switch k {
			case "cmd":
				t.Cmd, err = taskString(k, x[k])
			case "description":
				t.Description, err = taskString(k, x[k])
			case "timeout":
				t.Timeout, err = taskString(k, x[k])
			case "junit":
				t.JUnit, err = taskStrings(k, x[k])
			case "env":
				t.Env, err = taskMap(k, x[k])
			default:
				err = fmt.Errorf("unknown key %q; allowed: cmd, description, junit, timeout, env", k)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("want a command string or a table")
}

func sortedTaskKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func taskString(key string, v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s: want a string", key)
	}
	return s, nil
}

func taskStrings(key string, v any) ([]string, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: want a list of strings", key)
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("%s: want a list of strings", key)
		}
		out = append(out, s)
	}
	return out, nil
}

func taskMap(key string, v any) (map[string]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: want a table of strings", key)
	}
	out := make(map[string]string, len(m))
	for k, e := range m {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("%s.%s: want a string", key, k)
		}
		out[k] = s
	}
	return out, nil
}

// Results mirrors the [results] table: where a run should look for JUnit XML, and whether failing
// tests turn a green exit red.
type Results struct {
	Auto               []string `toml:"auto"`
	FailOnTestFailures bool     `toml:"fail_on_test_failures"`
}

// Config is the fully resolved configuration.
type Config struct {
	Isolation       string   `toml:"isolation"`
	OnMissingID     string   `toml:"on_missing_id"`
	RequireWorktree string   `toml:"require_worktree"`
	CreateOn        []string `toml:"create_on"`
	DestroyOn       []string `toml:"destroy_on"`
	IdleTimeout     string   `toml:"idle_timeout"`
	// IdleAction is what idle reclaim does to a sandbox whose worktree still exists: "stop" (the
	// default) frees its memory and keeps its disk, so the next command resumes it with its data;
	// "delete" frees both. A sandbox whose worktree is gone is always deleted.
	IdleAction string `toml:"idle_action"`
	// AutoReclaim lets an ordinary command sweep what it no longer needs — sandboxes whose
	// worktree is gone, sandboxes idle past idle_timeout, and unreferenced packs — at most once
	// every reclaim_every. Without it nothing reclaims anything until someone runs `boxer gc` by
	// hand, and a sandbox costs about half a gigabyte.
	AutoReclaim  bool   `toml:"auto_reclaim"`
	ReclaimEvery string `toml:"reclaim_every"`
	// MinFreeGB is the margin boxer refuses to spend on its own cache. Packing writes hundreds of
	// megabytes; below this the pack is skipped and the image pulled instead, because a full disk
	// breaks every VM on the host, not only boxer's.
	MinFreeGB float64 `toml:"min_free_gb"`
	// PacksKeepLast bounds the pack cache by count as well as by age: an environment that changes
	// often leaves a pack per version, each of them hundreds of megabytes, and all of them younger
	// than idle_timeout. 0 keeps every pack the idle rule allows.
	PacksKeepLast int `toml:"packs_keep_last"`
	// Deprecated keys. 1.0 accepted both and neither ever did anything, so they still load, with a
	// warning, rather than break a boxer.toml that 1.0 read without complaint. Remove in 2.0.
	RequireLinkedWorktree bool `toml:"require_linked_worktree"`
	ReuseExisting         bool `toml:"reuse_existing"`
	// WarmOnSessionStart makes the SessionStart hook provision in a detached `boxer up` and
	// return at once, so the session is never blocked on a VM create.
	WarmOnSessionStart bool `toml:"warm_on_session_start"`

	Integration          string   `toml:"integration"` // outside | inside
	Mode                 string   `toml:"mode"`
	Enforcement          string   `toml:"enforcement"`
	OnSandboxUnavailable string   `toml:"on_sandbox_unavailable"`
	Intercept            []string `toml:"intercept"`
	// InterceptAlso adds programs to Intercept without restating it, so a repository keeps the
	// default list, and whatever later releases add to it, and names only what it needs on top.
	InterceptAlso []string `toml:"intercept_also"`
	Passthrough   []string `toml:"passthrough"`

	// Prep runs on the *host*, in the worktree, before the sandbox exists — devcontainer's
	// `initializeCommand`. It is for installing dependencies faster than a guest can, and it is
	// opt-in because it is only safe for packages that ship prebuilt platform binaries: anything
	// that compiles at install time builds for the host and fails inside the guest. `setup` is
	// the always-correct alternative.
	Prep PrepConfig `toml:"prep"`

	// Cache mounts the host's package caches into the guest, read-only. The install still runs
	// in the guest, so every binary is still chosen for the guest's platform; only the download
	// is saved. Off by default: the mount is read-only at the tool's own cache path, so installing
	// anything the host has not cached fails (npm: EROFS). See internal/box/cache.go.
	Cache CacheConfig `toml:"cache"`

	// URLs names forwarded ports through portless. See URLsConfig.
	URLs URLsConfig `toml:"urls"`

	// User is the guest user boxer runs `setup`, `start` and every command as — a name or
	// uid[:gid]; empty means the image's own USER. `image_setup` always runs as root, because it
	// changes the image and that is root's job. devcontainer's remoteUser lands here.
	User string `toml:"user"`

	// Backend is what hosts the sandbox. "smolvm" gives a microVM per worktree — a kernel each,
	// which is the boundary boxer's promises are written against. "docker" and "podman" give a
	// container: faster to start, ordinary OCI images, and one kernel shared between every
	// sandbox on the machine. They also cannot enforce `network.mode = "allowlist"`, and refuse
	// it rather than run with egress boxer said it would deny. `boxer doctor` prints which
	// capabilities the chosen backend actually has.
	Backend string `toml:"backend"`

	Image    string `toml:"image"`
	Smolfile string `toml:"smolfile"`
	// ImageSetup changes the guest image and nothing else: apt packages, a global npm install, a
	// toolchain. It runs once per VM, its result is snapshotted into the environment pack, and a
	// new worktree starts from that pack without running it again.
	//
	// Setup is the other half: it prepares *this worktree*, usually by installing dependencies
	// into it. A pack can never skip it, because a pack carries the guest's filesystem and the
	// worktree is mounted from the host — a second worktree that skipped it would carry a marker
	// saying "installed" and no node_modules. devcontainer.json draws the same line between
	// onCreateCommand and postCreateCommand.
	ImageSetup []string `toml:"image_setup"`
	Setup      []string `toml:"setup"`
	MountAt    string   `toml:"mount_at"`
	// Env is set in the guest for every command. It is the project's own configuration, so it is
	// baked into the environment pack; anything secret belongs in Secrets or EnvPassthrough, which
	// are read from the host at run time and never snapshotted.
	Env map[string]string `toml:"env"`
	// Mounts are extra host directories, "host:guest" or "host:guest:ro". The worktree is always
	// mounted; these are for what a project needs beside it, usually a dependency cache.
	Mounts []string `toml:"mounts"`
	// Start runs every time the VM starts, detached, after Setup. This is how a service runs: the
	// setup list installs it, the start list launches it. Restart says what happens when one
	// crashes; there is no dependency graph between them.
	Start []string `toml:"start"`
	// Ready is polled until it exits zero before boxer reports the sandbox up, because a server
	// takes a variable time to accept connections and a fixed sleep is always wrong.
	Ready string `toml:"ready"`
	// ReadyTimeout bounds that wait.
	ReadyTimeout string `toml:"ready_timeout"`
	// Restart is what happens when a `start` service exits non-zero: "on-failure" (the default)
	// restarts it with a backoff and gives up after five quick crashes; "never" leaves it down.
	Restart string `toml:"restart"`
	// Volumes are named directories that outlive the sandbox: "name:/guest/path". Each is kept on
	// the host per scope and mounted read-write, so a database's files survive `--recreate`, idle
	// reclaim and `rm`. It is removed with its worktree, or by `boxer rm --volumes`.
	Volumes []string `toml:"volumes"`
	// Build is a Dockerfile, relative to the repository, that boxer builds into the sandbox's
	// image. It replaces `image`. On smolvm the host's docker builds it and smolvm boots the saved
	// archive, so docker must be installed even though the sandbox is a microVM.
	Build string `toml:"build"`
	// BuildContext is the directory the build sees, relative to the worktree. Empty means the
	// directory holding the Dockerfile.
	BuildContext   string   `toml:"build_context"`
	CPUs           int      `toml:"cpus"`
	Memory         string   `toml:"memory"`
	EnvPassthrough []string `toml:"env_passthrough"`
	Secrets        []string `toml:"secrets"`

	// Tasks are the repository's named command lines: `boxer run --task test`. Naming a task is
	// how an agent runs the repository's real commands without composing a shell line that the
	// intercept list may or may not catch (R-CFG-4).
	Tasks map[string]Task `toml:"tasks"`

	Results   Results             `toml:"results"`
	Network   Network             `toml:"network"`
	Telemetry Telemetry           `toml:"telemetry"`
	Worktree  Worktree            `toml:"worktree"`
	Harness   map[string]Override `toml:"harness"`

	// Sources maps a top-level key to the file, the BOXER_* variable, or "default" it came from.
	Sources map[string]string `toml:"-"`
	// Files lists the configuration files that were read, lowest priority first.
	Files []string `toml:"-"`
	// Warnings records what was read but not understood, so an older binary can still run a
	// repository whose boxer.toml was written for a newer one.
	Warnings []string `toml:"-"`
}

// Defaults are the values with no file present.
func Defaults() Config {
	return Config{
		Backend:              "smolvm",
		Cache:                CacheConfig{Enabled: false, Managers: []string{"auto"}},
		Prep:                 PrepConfig{Target: "auto"},
		URLs:                 URLsConfig{Provider: "portless"},
		Isolation:            "worktree",
		OnMissingID:          "degrade",
		RequireWorktree:      "warn",
		CreateOn:             []string{"session_start", "run", "mcp"},
		DestroyOn:            []string{},
		IdleTimeout:          "2h",
		AutoReclaim:          true,
		ReclaimEvery:         "6h",
		MinFreeGB:            5,
		PacksKeepLast:        5,
		ReuseExisting:        true,
		ReadyTimeout:         "60s",
		Restart:              "on-failure",
		IdleAction:           "stop",
		Integration:          "outside",
		Mode:                 "rewrite",
		Enforcement:          "both",
		OnSandboxUnavailable: "fail",
		Intercept:            []string{"npm", "npx", "pnpm", "yarn", "bun", "bunx", "node", "deno", "python", "python3", "pip", "uv", "pytest", "cargo", "rustc", "go", "make", "cmake"},
		Passthrough:          []string{"git", "gh", "ssh", "boxer", "smolvm"},
		MountAt:              "/workspace",
		CPUs:                 4,
		Memory:               "4G",
		EnvPassthrough:       []string{"CI"},
		Network:              Network{Mode: "allowlist", AllowHosts: []string{}},
		Telemetry:            Telemetry{Sink: "none"},
		Worktree:             Worktree{Manage: "off"},
		Harness:              map[string]Override{},
		Tasks:                map[string]Task{},
		Sources:              map[string]string{},
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
	// The devcontainer file is read first and overridden by everything: it is where a repository
	// already wrote its environment down, and boxer.toml is how you disagree with it.
	// ${localWorkspaceFolder} is the folder the tool opened, which for boxer is this worktree even
	// when the file itself is read from the repository root.
	workspace := worktreeRoot
	if workspace == "" {
		workspace = repoRoot
	}
	// The worktree's own file first: a branch that changes its devcontainer must get that one,
	// as it would its own boxer.toml, not the main checkout's.
	for _, root := range []string{worktreeRoot, repoRoot} {
		if root == "" {
			continue
		}
		if dc := findDevcontainer(root); dc != "" {
			if err := cfg.mergeDevcontainer(dc, workspace); err != nil {
				return cfg, err
			}
			break
		}
	}
	dcBuild := cfg.Sources["build"]
	for _, p := range paths {
		if err := cfg.merge(p); err != nil {
			return cfg, err
		}
	}
	// A build the devcontainer set gives way to an image or a Smolfile a boxer.toml names: the
	// devcontainer is the lowest layer, and `build` would otherwise win over `image` in Image().
	if dcBuild != "" && cfg.Sources["build"] == dcBuild &&
		(cfg.Sources["image"] != "" && cfg.Sources["image"] != dcBuild || cfg.Smolfile != "") {
		cfg.Build, cfg.BuildContext = "", ""
		delete(cfg.Sources, "build")
		delete(cfg.Sources, "build_context")
	}
	if err := cfg.applyEnv(); err != nil {
		return cfg, err
	}
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
	if err := cfg.applyEnv(); err != nil {
		return cfg, err
	}
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

// knownTables are the top-level keys that are tables ([network], [env], [harness.x]): the only
// names a misspelled table can be a typo of.
var knownTables = func() map[string]bool {
	m := map[string]bool{}
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if tag := f.Tag.Get("toml"); tag != "" && tag != "-" && (f.Type.Kind() == reflect.Struct || f.Type.Kind() == reflect.Map) {
			m[strings.Split(tag, ",")[0]] = true
		}
	}
	return m
}()

// knownKeys are the top-level keys this binary understands, from Config's own tags.
var knownKeys = func() map[string]bool {
	m := map[string]bool{}
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		if tag := t.Field(i).Tag.Get("toml"); tag != "" && tag != "-" {
			m[tag] = true
		}
	}
	return m
}()

func (c *Config) merge(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// The decoder makes a fresh struct for each [harness.<name>] it reads, so a later layer's
	// table replaced the earlier one whole: a repository's `mode` erased the user's `enforcement`
	// for the same harness. Kept here and merged back below, field by field, as [env] merges.
	earlier := map[string]Override{}
	for k, v := range c.Harness {
		earlier[k] = v
	}
	md, err := toml.Decode(string(data), c)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for name, now := range c.Harness {
		if was, ok := earlier[name]; ok {
			if now.Mode == "" {
				now.Mode = was.Mode
			}
			if now.Enforcement == "" {
				now.Enforcement = was.Enforcement
			}
			if now.Isolation == "" {
				now.Isolation = was.Isolation
			}
			c.Harness[name] = now
		}
	}
	// An unknown top-level *table* is how a newer boxer adds a feature, so it is a warning: a
	// repository that declares one must still be usable by whatever binary is installed. Anything
	// else unknown is a typo in a key this binary owns, and stays a hard error, because a silently
	// ignored `mod = "off"` would silently disable enforcement.
	var unknown []string
	warned := map[string]bool{}
	for _, k := range md.Undecoded() {
		if top := k[0]; !knownKeys[top] && md.Type(top) == "Hash" {
			// A newer boxer's table is a warning, but a typo of one this boxer knows is not: a
			// misspelled [netwrok] left mode at its default, and an override was silently lost.
			if near := nearestKnown(top); near != "" {
				return fmt.Errorf("%s: [%s] is not a table boxer knows; did you mean [%s]?", path, top, near)
			}
			if !warned[top] {
				warned[top] = true
				c.Warnings = append(c.Warnings, fmt.Sprintf("%s: [%s] is not a table this boxer knows; ignored", path, top))
			}
			continue
		}
		unknown = append(unknown, k.String())
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%s: unknown key(s): %s", path, strings.Join(unknown, ", "))
	}
	for key, instead := range deprecatedKeys {
		if md.IsDefined(key) {
			c.Warnings = append(c.Warnings, fmt.Sprintf("%s: %s has no effect and will be removed in 2.0; %s", path, key, instead))
		}
	}
	for _, k := range md.Keys() {
		if len(k) > 0 {
			c.Sources[k[0]] = path
		}
	}
	c.Files = append(c.Files, path)
	return nil
}

// nearestKnown returns a known table name within two edits of name, or "".
func nearestKnown(name string) string {
	for k := range knownTables {
		if k != name && editDistance(k, name) <= 2 && len(name) >= 4 {
			return k
		}
	}
	return ""
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// deprecatedKeys still load, with a warning that says what to use instead.
var deprecatedKeys = map[string]string{
	"require_linked_worktree": `use require_worktree = "require"`,
	"reuse_existing":          "a scope always reuses its sandbox; delete the line",
}

// envKeys are the scalar settings that may be overridden from the environment (R-CFG: every key
// settable by environment for harnesses that only offer environment control). Lists are not
// overridable; they are repository policy.
var envKeys = map[string]func(c *Config, v string) error{
	"BOXER_BACKEND": func(c *Config, v string) error { c.Backend = v; return nil },
	"BOXER_URLS": func(c *Config, v string) error {
		// Inside the guest boxer sets BOXER_URLS to the sandbox's names ("3000=https://..."), for
		// the agent to read. That is information, not this setting, so it is left alone.
		if strings.Contains(v, "=") {
			return errNotASetting
		}
		b, err := strconv.ParseBool(v)
		if err == nil {
			c.URLs.Enabled = b
		}
		return err
	},
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
		if err == nil {
			c.CPUs = n
		}
		return err
	},
	"BOXER_NETWORK_MODE":    func(c *Config, v string) error { c.Network.Mode = v; return nil },
	"BOXER_WORKTREE_MANAGE": func(c *Config, v string) error { c.Worktree.Manage = v; return nil },
	"BOXER_TELEMETRY_SINK":  func(c *Config, v string) error { c.Telemetry.Sink = v; c.Telemetry.Enabled = v != "none"; return nil },
	"BOXER_WARM_ON_SESSION_START": func(c *Config, v string) error {
		b, err := strconv.ParseBool(v)
		if err == nil {
			c.WarmOnSessionStart = b
		}
		return err
	},
}

// errNotASetting marks an environment value that is something else boxer put there, not an
// override; it is skipped without an error.
var errNotASetting = fmt.Errorf("not a setting")

// envSource is the Sources key an override is recorded under: the top-level key doctor looks up,
// so BOXER_NETWORK_MODE is credited to "network", not to a "network_mode" nothing reads.
var envSource = map[string]string{
	"BOXER_NETWORK_MODE": "network", "BOXER_WORKTREE_MANAGE": "worktree", "BOXER_TELEMETRY_SINK": "telemetry",
}

// applyEnv applies the BOXER_* overrides. A value that does not parse is an error naming the
// variable: it used to be swallowed after it had already been half applied, so BOXER_CPUS=four
// set cpus to 0 and BOXER_URLS=yes turned URLs off, both silently.
func (c *Config) applyEnv() error {
	for _, name := range sortedEnvKeys() {
		v, ok := os.LookupEnv(name)
		if !ok {
			continue
		}
		if err := envKeys[name](c, v); err == errNotASetting {
			continue
		} else if err != nil {
			return fmt.Errorf("%s=%q: %v", name, v, err)
		}
		// Name the variable, not just "env": doctor's job is to explain a value well enough
		// that the reader knows what to change.
		key := envSource[name]
		if key == "" {
			key = strings.ToLower(strings.TrimPrefix(name, "BOXER_"))
		}
		c.Sources[key] = name
	}
	return nil
}

func sortedEnvKeys() []string {
	out := make([]string, 0, len(envKeys))
	for k := range envKeys {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
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

// PrepConfig is the host-side preparation step.
type PrepConfig struct {
	Commands []string `toml:"commands"`
	// Target is the platform the guest runs, exposed to the commands as BOXER_TARGET_*.
	// "auto" (the default) derives it from the image; "none" exposes nothing; or give it
	// explicitly as os/cpu/libc, e.g. "linux/arm64/musl".
	Target string `toml:"target"`
}

// CacheConfig selects which host package caches are offered to the guest.
type CacheConfig struct {
	Enabled bool `toml:"enabled"`
	// Managers is "auto" (detect from lockfiles, the default) or an explicit list: npm, pnpm,
	// yarn, bun, uv, poetry, cargo, go.
	Managers []string `toml:"managers"`
}

// URLsConfig gives each sandbox's forwarded ports a stable, named URL through portless
// (github.com/vercel-labs/portless), so a dev server is `https://fix-ui.myapp.localhost` in every
// worktree instead of a host port that differs in each. Off by default: it needs portless on the
// host and a proxy running, and without it nothing about a sandbox changes.
type URLsConfig struct {
	Enabled bool `toml:"enabled"`
	// Provider is what serves the names. "portless" is the only one; the key exists so a config
	// that says which one it means stays correct if there is ever a second.
	Provider string `toml:"provider"`
	// Name is the base hostname, before any worktree prefix. Empty means the repository's
	// directory name, which is what portless itself would pick.
	Name string `toml:"name"`
	// Names labels guest ports: {"3000" = "web", "8080" = "api"} gives web.myapp and api.myapp.
	// A single forwarded port needs no label; several without one are named by their number.
	Names map[string]string `toml:"names"`
}

// Validate rejects values outside their enumerations so a typo cannot silently disable
// enforcement (R-CFG-2).
func (c Config) Validate() error {
	checks := []struct {
		key, val string
		allowed  []string
	}{
		{"backend", c.Backend, []string{"smolvm", "docker", "podman", "container"}},
		{"urls.provider", c.URLs.Provider, []string{"portless"}},
		{"isolation", c.Isolation, []string{"repo", "worktree", "session", "subagent"}},
		{"on_missing_id", c.OnMissingID, []string{"degrade", "fail"}},
		{"require_worktree", c.RequireWorktree, []string{"off", "warn", "require"}},
		{"integration", c.Integration, []string{"outside", "inside"}},
		{"mode", c.Mode, []string{"rewrite", "tool", "off"}},
		{"enforcement", c.Enforcement, []string{"hook", "shim", "both", "audit"}},
		{"on_sandbox_unavailable", c.OnSandboxUnavailable, []string{"fail", "passthrough"}},
		{"network.mode", c.Network.Mode, []string{"off", "allowlist", "on"}},
		{"restart", c.Restart, []string{"on-failure", "never"}},
		{"idle_action", c.IdleAction, []string{"stop", "delete"}},
		{"worktree.manage", c.Worktree.Manage, []string{"off", "detect"}},
		{"telemetry.sink", c.Telemetry.Sink, []string{"none", "file", "stderr", "otel"}},
	}
	for _, ch := range checks {
		if !slices.Contains(ch.allowed, ch.val) {
			return fmt.Errorf("%s = %q; allowed: %s", ch.key, ch.val, strings.Join(ch.allowed, " | "))
		}
	}
	for _, e := range c.CreateOn {
		if !slices.Contains([]string{"session_start", "subagent_start", "run", "mcp"}, e) {
			return fmt.Errorf("create_on contains %q; allowed: session_start | subagent_start | run | mcp", e)
		}
	}
	for _, e := range c.DestroyOn {
		if !slices.Contains([]string{"session_end", "subagent_stop", "never"}, e) {
			return fmt.Errorf("destroy_on contains %q; allowed: session_end | subagent_stop | never", e)
		}
	}
	for name, o := range c.Harness {
		for _, ch := range checks {
			val := map[string]string{"mode": o.Mode, "enforcement": o.Enforcement, "isolation": o.Isolation}[ch.key]
			if val != "" && !slices.Contains(ch.allowed, val) {
				return fmt.Errorf("harness.%s.%s = %q; allowed: %s", name, ch.key, val, strings.Join(ch.allowed, " | "))
			}
		}
	}
	for _, name := range c.TaskNames() {
		t := c.Tasks[name]
		if strings.TrimSpace(t.Cmd) == "" {
			return fmt.Errorf("tasks.%s is empty; give it a command line", name)
		}
		if t.Timeout != "" {
			if _, err := time.ParseDuration(t.Timeout); err != nil {
				return fmt.Errorf("tasks.%s.timeout = %q: %w", name, t.Timeout, err)
			}
		}
	}
	if _, err := SplitAllow(c.Network.AllowHosts); err != nil {
		return err
	}
	for _, p := range c.Network.AllowPresets {
		if _, ok := Presets[p]; !ok {
			return fmt.Errorf("network.allow_presets contains %q; allowed: %s", p, strings.Join(PresetNames(), " | "))
		}
	}
	seen := map[string]bool{}
	for _, v := range c.Volumes {
		name, guest, ok := strings.Cut(v, ":")
		if !ok || !volumeName.MatchString(name) || !strings.HasPrefix(guest, "/") || strings.Contains(guest, ":") {
			return fmt.Errorf("volumes entry %q: want name:/guest/path, the name made of letters, digits, '-' and '_'", v)
		}
		if seen[name] {
			return fmt.Errorf("volumes names %q twice", name)
		}
		seen[name] = true
	}
	if c.CPUs < 1 {
		return fmt.Errorf("cpus = %d; it must be at least 1", c.CPUs)
	}
	for key, d := range map[string]string{"idle_timeout": c.IdleTimeout, "reclaim_every": c.ReclaimEvery, "ready_timeout": c.ReadyTimeout} {
		if d == "" || key == "idle_timeout" && d == "never" {
			continue
		}
		// A duration that did not parse was not an error until the moment it was used: a bad
		// reclaim_every switched the sweep off, a bad ready_timeout fell back to a minute.
		if _, err := time.ParseDuration(d); err != nil {
			return fmt.Errorf("%s = %q: want a Go duration such as \"2h\" or \"90s\"", key, d)
		}
	}
	if t := c.Prep.Target; t != "" && t != "auto" && t != "none" && strings.Count(t, "/") != 2 {
		return fmt.Errorf("prep.target = %q; allowed: auto | none | os/cpu/libc, such as linux/arm64/musl", t)
	}
	for _, m := range c.Cache.Managers {
		if !slices.Contains(cacheManagers, m) {
			return fmt.Errorf("cache.managers contains %q; allowed: %s", m, strings.Join(cacheManagers, " | "))
		}
	}
	for _, p := range c.Network.Ports {
		if !portSpec.MatchString(p) {
			return fmt.Errorf("network.ports entry %q: want \"auto:3000\", \"8080:3000\" or \"127.0.0.1:8080:3000\"", p)
		}
	}
	if c.Build != "" && c.Smolfile != "" {
		return fmt.Errorf("build and smolfile both describe the image; keep one")
	}
	if _, err := MemoryMiB(c.Memory); err != nil {
		return err
	}
	// The name reaches the guest's shell, sed and awk unquoted; nothing else is a user name.
	if c.User != "" && !userName.MatchString(c.User) {
		return fmt.Errorf("user = %q; want a user name or uid, optionally with :group", c.User)
	}
	return nil
}

var userName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*(:[A-Za-z0-9_][A-Za-z0-9_.-]*)?$`)

var volumeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// cacheManagers are the names cache.managers takes; box.managerOf maps lockfiles onto them.
var cacheManagers = []string{"auto", "npm", "pnpm", "yarn", "bun", "uv", "poetry", "cargo", "go"}

// portSpec is a forward: auto:GUEST, or [ADDR:]HOST:GUEST with optional one-to-one ranges and a
// protocol, as docker's -p takes it; ADDR may be IPv4 or a bracketed IPv6 address.
var portSpec = regexp.MustCompile(`^(auto:\d+|(\[[0-9A-Fa-f:.]+\]:|(\d{1,3}\.){3}\d{1,3}:)?\d+(-\d+)?:\d+(-\d+)?|\d+(-\d+)?)(/(tcp|udp|sctp))?$`)

// Intercepted is the full list of programs that go to the sandbox: intercept plus intercept_also.
func (c Config) Intercepted() []string {
	out := slices.Clone(c.Intercept)
	for _, p := range c.InterceptAlso {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// AllowedHosts is allow_hosts plus the hosts of every preset in allow_presets, without repeats.
func (n Network) AllowedHosts() []string {
	out := slices.Clone(n.AllowHosts)
	for _, p := range n.AllowPresets {
		for _, h := range Presets[p] {
			if !slices.Contains(out, h) {
				out = append(out, h)
			}
		}
	}
	return out
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
	return slices.Contains(list, v)
}

// TaskNames lists the declared task names, sorted, for listings and error messages.
func (c Config) TaskNames() []string {
	out := make([]string, 0, len(c.Tasks))
	for n := range c.Tasks {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}
