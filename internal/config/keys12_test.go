package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func load(t *testing.T, body string) (Config, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "boxer.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadFiles(p)
}

func TestNewKeysDefaultAndValidate(t *testing.T) {
	d := Defaults()
	if d.Restart != "on-failure" || d.IdleAction != "stop" {
		t.Fatalf("defaults: restart %q, idle_action %q", d.Restart, d.IdleAction)
	}
	for _, bad := range []string{
		`restart = "always"`,
		`idle_action = "pause"`,
		`volumes = ["pgdata"]`,
		`volumes = ["pg data:/x"]`,
		`volumes = ["pg:relative"]`,
		`volumes = ["pg:/a", "pg:/b"]`,
		"[network]\nallow_presets = [\"npmm\"]",
		"build = \"Dockerfile\"\nsmolfile = \"Smolfile\"",
		`user = "node; rm -rf /"`,
		`user = "a b"`,
		`intercept_also = ["npm i"]`,
		"intercept_also = [\"x\\nrm\"]",
		`passthrough = ["git;rm"]`,
		"[network]\nports = [\"70000:3000\"]",
		`mount_at = "workspace"`,
		"[env]\n\"A-B\" = \"1\"",
		`secrets = ["TOKEN=x"]`,
		`packs_keep_last = -1`,
		`reclaim_every = "0s"`,
	} {
		if _, err := load(t, bad); err == nil {
			t.Errorf("%s must be refused", bad)
		}
	}
	for _, good := range []string{`volumes = ["pg-data_1:/var/lib/postgresql/data"]`, `user = "first.last"`, `user = "1000:1000"`, `user = "node:staff"`,
		`intercept_also = ["g++", "python3.12"]`, "[network]\nports = [\"127.0.0.1:8080:3000\", \"65535:3000/udp\"]"} {
		if _, err := load(t, good); err != nil {
			t.Fatal(err)
		}
	}
}

// intercept_also adds to the default list without restating it, and says nothing twice.
func TestInterceptAlsoExtendsTheDefault(t *testing.T) {
	c, err := load(t, `intercept_also = ["bundle", "npm", "rake"]`)
	if err != nil {
		t.Fatal(err)
	}
	got := c.Intercepted()
	if !slices.Equal(got[:len(Defaults().Intercept)], Defaults().Intercept) {
		t.Fatalf("the default list must be kept: %v", got)
	}
	if !slices.Contains(got, "bundle") || !slices.Contains(got, "rake") || len(got) != len(Defaults().Intercept)+2 {
		t.Fatalf("got %v", got)
	}
}

// A preset expands into its hosts beside allow_hosts, once each.
func TestAllowPresetsExpand(t *testing.T) {
	c, err := load(t, "[network]\nallow_hosts = [\"example.com\", \"pypi.org\"]\nallow_presets = [\"pypi\", \"npm\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.com", "pypi.org", "files.pythonhosted.org", "registry.npmjs.org"}
	if got := c.Network.AllowedHosts(); !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for name, hosts := range Presets {
		if len(hosts) == 0 {
			t.Errorf("preset %s is empty", name)
		}
	}
}

// What the 1.4 review found loading without complaint.
func TestReviewConfigFindings(t *testing.T) {
	for _, bad := range []string{
		"[netwrok]\nmode = \"on\"",
		"cpus = 0",
		`idle_timeout = "2 hours"`,
		`reclaim_every = "6"`,
		`ready_timeout = "x"`,
		"[prep]\ntarget = \"linux-arm64\"",
		"[cache]\nmanagers = [\"nmp\"]",
		"[network]\nports = [\"banana\"]",
	} {
		if _, err := load(t, bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
	for _, good := range []string{`idle_timeout = "never"`, "[network]\nports = [\"auto:3000\", \"8080:3000\", \"127.0.0.1:9000:9000\", \"3000-3002:3000-3002\", \"5353:5353/udp\", \"[::1]:8080:80\"]", "[prep]\ntarget = \"linux/arm64/musl\"", "[futuretable]\nx = 1"} {
		if _, err := load(t, good); err != nil {
			t.Errorf("%q must load: %v", good, err)
		}
	}
	// An environment value that does not parse is an error, and one that is not a setting at all
	// (the guest's BOXER_URLS map) is ignored.
	t.Setenv("BOXER_CPUS", "four")
	if _, err := load(t, ""); err == nil || !strings.Contains(err.Error(), "BOXER_CPUS") {
		t.Errorf("BOXER_CPUS=four: %v", err)
	}
	t.Setenv("BOXER_CPUS", "2")
	t.Setenv("BOXER_URLS", "3000=https://web.app.localhost:1355")
	if c, err := load(t, "[urls]\nenabled = true"); err != nil || !c.URLs.Enabled {
		t.Errorf("the guest's BOXER_URLS map is not this setting: %v %v", c.URLs.Enabled, err)
	}
	t.Setenv("BOXER_NETWORK_MODE", "on")
	if c, _ := load(t, ""); c.Sources["network"] != "BOXER_NETWORK_MODE" {
		t.Errorf("doctor reads the source under the top-level key: %v", c.Sources)
	}
}

// A [harness.x] table in a later layer merges with the earlier one rather than replacing it.
func TestHarnessTablesMergeAcrossLayers(t *testing.T) {
	dir := t.TempDir()
	user, repo := filepath.Join(dir, "user.toml"), filepath.Join(dir, "repo.toml")
	os.WriteFile(user, []byte("[harness.claude]\nenforcement = \"hook\"\n"), 0o644)
	os.WriteFile(repo, []byte("[harness.claude]\nmode = \"tool\"\n"), 0o644)
	c, err := LoadFiles(user, repo)
	if err != nil {
		t.Fatal(err)
	}
	if h := c.Harness["claude"]; h.Mode != "tool" || h.Enforcement != "hook" {
		t.Fatalf("got %+v", h)
	}
}

// [harness.claude-code] configures `boxer shell claude` too, and a misspelt table is refused.
func TestHarnessTablesByEitherName(t *testing.T) {
	RegisterHarnessAlias("claude", "claude-code") // what package inside registers
	c, err := load(t, "[harness.claude-code]\nmode = \"tool\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ForHarness("claude"); got.Mode != "tool" || c.Sources["mode"] == "harness.claude-code" {
		t.Fatalf("mode %q, and the caller's sources must be left alone: %q", got.Mode, c.Sources["mode"])
	}
	if _, err := load(t, "[harness.cladue-code]\nmode = \"tool\"\n"); err == nil {
		t.Fatal("a misspelt harness table must be refused")
	}
}
