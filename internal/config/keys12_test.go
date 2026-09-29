package config

import (
	"os"
	"path/filepath"
	"slices"
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
	} {
		if _, err := load(t, bad); err == nil {
			t.Errorf("%s must be refused", bad)
		}
	}
	if _, err := load(t, `volumes = ["pg-data_1:/var/lib/postgresql/data"]`); err != nil {
		t.Fatal(err)
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
