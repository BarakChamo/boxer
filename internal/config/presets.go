package config

import (
	"maps"
	"slices"
)

// Presets are the hosts a package manager reaches to install, by ecosystem. Each list is what the
// tool fetches from by default, including the CDN a registry redirects downloads to, because a
// guest that can resolve the registry but not its CDN fails halfway through an install.
var Presets = map[string][]string{
	"npm":      {"registry.npmjs.org"},
	"yarn":     {"registry.yarnpkg.com", "repo.yarnpkg.com", "registry.npmjs.org"},
	"pypi":     {"pypi.org", "files.pythonhosted.org"},
	"crates":   {"crates.io", "index.crates.io", "static.crates.io"},
	"rust":     {"static.rust-lang.org", "sh.rustup.rs"},
	"go":       {"proxy.golang.org", "sum.golang.org", "storage.googleapis.com"},
	"rubygems": {"rubygems.org", "index.rubygems.org", "rubygems.global.ssl.fastly.net"},
	"maven":    {"repo.maven.apache.org", "repo1.maven.org"},
	"gradle":   {"services.gradle.org", "downloads.gradle.org", "plugins.gradle.org", "plugins-artifacts.gradle.org", "repo.maven.apache.org"},
	"debian":   {"deb.debian.org", "security.debian.org"},
	"alpine":   {"dl-cdn.alpinelinux.org"},
	"github":   {"github.com", "api.github.com", "codeload.github.com", "objects.githubusercontent.com", "raw.githubusercontent.com"},
}

// PresetNames is every preset, sorted.
func PresetNames() []string { return slices.Sorted(maps.Keys(Presets)) }
