package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// The worktree is mounted read-write, so code in the guest can rewrite the worktree's boxer.toml
// or devcontainer.json. A handful of keys reach the host when boxer next reads them: `prep` runs
// commands on the host, `mounts` and `build` hand it host paths, and `mode`, `enforcement`,
// `passthrough`, `intercept`, `secrets`, `env_passthrough` and `network` decide what a command may
// reach. So a change to those keys is trusted: boxer remembers a hash of them, and until a person
// approves a new one with `boxer trust`, each falls back to a value no weaker than boxer's default.
// A configuration that only tightens the sandbox (intercepts more, opens less) is not held back,
// because it cannot be used against the host; everything else (image, cpus, tasks, ready) takes
// effect without a prompt.

// HostDigest is a hash of the values a change to which could reach the host. It is stable across
// runs and across reordering within a list, so an unrelated edit does not re-prompt.
func (c Config) HostDigest() string {
	var b strings.Builder
	add := func(k string, vs ...string) {
		fmt.Fprintf(&b, "%s\x00", k)
		for _, v := range vs {
			fmt.Fprintf(&b, "%s\x00", v)
		}
	}
	addList := func(k string, vs []string) {
		s := append([]string(nil), vs...)
		sort.Strings(s)
		add(k, s...)
	}
	add("mode", c.Mode)
	add("enforcement", c.Enforcement)
	addList("intercept", c.Intercept)
	addList("intercept_also", c.InterceptAlso)
	addList("passthrough", c.Passthrough)
	addList("prep", c.Prep.Commands)
	add("prep_target", c.Prep.Target)
	addList("mounts", c.Mounts)
	add("build", c.Build, c.BuildContext)
	addList("secrets", c.Secrets)
	addList("env_passthrough", c.EnvPassthrough)
	add("network", c.Network.Mode)
	addList("allow_hosts", c.Network.AllowHosts)
	addList("allow_presets", c.Network.AllowPresets)
	add("on_sandbox_unavailable", c.OnSandboxUnavailable)
	add("telemetry_path", c.Telemetry.Path)
	for _, name := range c.harnessNamesSorted() {
		h := c.Harness[name]
		add("harness."+name, h.Mode, h.Enforcement)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func (c Config) harnessNamesSorted() []string {
	names := make([]string, 0, len(c.Harness))
	for n := range c.Harness {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// HostAffecting lists the host-affecting keys this configuration sets, for `boxer trust --show`.
func (c Config) HostAffecting() []string {
	d := Defaults()
	var set []string
	note := func(cond bool, name string) {
		if cond {
			set = append(set, name)
		}
	}
	note(len(c.Prep.Commands) > 0, "prep")
	note(len(c.Mounts) > 0, "mounts")
	note(c.Build != "", "build")
	note(c.Mode != d.Mode, "mode")
	note(c.Enforcement != d.Enforcement, "enforcement")
	note(!sameList(c.Passthrough, d.Passthrough), "passthrough")
	note(!sameList(c.Intercept, d.Intercept) || len(c.InterceptAlso) > 0, "intercept")
	note(!sameList(c.Secrets, d.Secrets), "secrets")
	note(!sameList(c.EnvPassthrough, d.EnvPassthrough), "env_passthrough")
	note(c.Network.Mode != d.Network.Mode || len(c.Network.AllowHosts) > 0 || len(c.Network.AllowPresets) > 0, "network")
	note(c.OnSandboxUnavailable != d.OnSandboxUnavailable, "on_sandbox_unavailable")
	note(c.Telemetry.Path != "", "telemetry.path")
	for _, name := range c.harnessNamesSorted() {
		if h := c.Harness[name]; h.Mode != "" || h.Enforcement != "" {
			set = append(set, "harness."+name)
		}
	}
	return set
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac, bc := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	return slices.Equal(ac, bc)
}

var networkRank = map[string]int{"off": 0, "allowlist": 1, "on": 2}

// SafeFallback returns this configuration with every host-affecting key brought to a value no
// weaker than boxer's default: the configuration boxer uses for a repository whose host-affecting
// keys are not yet trusted. A key that already tightens the sandbox is kept — more interception,
// a narrower network, a refused command cannot be turned against the host — so only a configuration
// that reaches the host or loosens it is changed. It also returns the keys it changed, for a
// warning; an empty list means the configuration was safe as written and nothing was held back.
func (c Config) SafeFallback() (Config, []string) {
	d := Defaults()
	out := c
	var changed []string
	mark := func(name string) { changed = append(changed, name) }

	// Exposing keys: reset to none. These run host commands or hand the sandbox host paths.
	if len(c.Prep.Commands) > 0 {
		out.Prep = PrepConfig{}
		mark("prep")
	}
	if len(c.Mounts) > 0 {
		out.Mounts = nil
		mark("mounts")
	}
	if c.Build != "" {
		out.Build, out.BuildContext = "", ""
		mark("build")
	}
	if !sameList(c.Secrets, d.Secrets) {
		out.Secrets = d.Secrets
		mark("secrets")
	}
	if !sameList(c.EnvPassthrough, d.EnvPassthrough) {
		out.EnvPassthrough = d.EnvPassthrough
		mark("env_passthrough")
	}
	if c.Telemetry.Path != "" {
		out.Telemetry.Path = ""
		mark("telemetry.path")
	}

	// Enforcement keys: hold back only a loosening, keep a tightening.
	if c.Mode == "off" { // "off" runs everything on the host; "tool" and "rewrite" sandbox
		out.Mode = d.Mode
		mark("mode")
	}
	if c.Enforcement == "audit" { // "audit" runs the command on the host and only logs
		out.Enforcement = d.Enforcement
		mark("enforcement")
	}
	if c.OnSandboxUnavailable == "passthrough" { // runs on the host when the sandbox cannot start
		out.OnSandboxUnavailable = d.OnSandboxUnavailable
		mark("on_sandbox_unavailable")
	}
	// Intercept at least the default set: a guest that narrowed it does not get commands run on
	// the host, and `["*"]` (everything) is kept.
	if !slices.Contains(c.Intercept, "*") {
		union := append([]string(nil), c.Intercept...)
		for _, p := range d.Intercept {
			if !slices.Contains(union, p) {
				union = append(union, p)
			}
		}
		if !sameList(union, c.Intercept) {
			out.Intercept = union
			mark("intercept")
		}
	}
	// Passthrough no more than the default: a guest-added passthrough (git, ssh, a build tool)
	// is dropped, so it cannot move a command onto the host.
	var keptPass []string
	for _, p := range c.Passthrough {
		if slices.Contains(d.Passthrough, p) {
			keptPass = append(keptPass, p)
		}
	}
	if !sameList(keptPass, c.Passthrough) {
		out.Passthrough = keptPass
		mark("passthrough")
	}
	// Network no wider than the default, and no guest-named allowlist hosts.
	if networkRank[c.Network.Mode] > networkRank[d.Network.Mode] {
		out.Network.Mode = d.Network.Mode
		mark("network")
	}
	if len(c.Network.AllowHosts) > 0 || len(c.Network.AllowPresets) > 0 {
		out.Network.AllowHosts, out.Network.AllowPresets = nil, nil
		if !slices.Contains(changed, "network") {
			mark("network")
		}
	}
	// Harness overrides can loosen (a per-harness mode = "off"); drop them and keep the base.
	if len(c.Harness) > 0 {
		out.Harness = nil
		mark("harness")
	}
	return out, changed
}
