package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/shim"
)

// A shim only works while its directory is first on PATH, and a login shell rebuilds PATH. The
// check has to notice that, because the failure is otherwise invisible: commands run on the host
// and nothing says so.
func TestDoctorNoticesWhenALoginShellWouldDemoteTheShims(t *testing.T) {
	shims := t.TempDir()
	if _, err := shim.Install(shims, []string{"boxer-doctor-probe"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	// A shell whose login files put another directory first is what macOS's path_helper does.
	elsewhere := t.TempDir()
	probe := filepath.Join(elsewhere, "boxer-doctor-probe")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeShell := filepath.Join(t.TempDir(), "login-shell")
	script := "#!/bin/sh\nPATH=" + elsewhere + ":$PATH\nexport PATH\nexec /bin/sh \"$@\"\n"
	if err := os.WriteFile(fakeShell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fakeShell)

	launchedPATH = shims + string(filepath.ListSeparator) + os.Getenv("PATH")
	t.Cleanup(func() { launchedPATH = "" })

	e := &box.Env{Cfg: config.Config{Enforcement: "both", Intercept: []string{"boxer-doctor-probe"}}}
	w := loginShellDemotesShims(e)
	if w == "" {
		t.Fatal("a login shell that resolves the program elsewhere was not reported")
	}
	for _, want := range []string{"login shell", probe, shims} {
		if !strings.Contains(w, want) {
			t.Errorf("the warning does not mention %q: %s", want, w)
		}
	}
}

// Nothing to warn about when boxer's shims are not on the PATH boxer was started with, or when
// enforcement does not use them at all.
func TestDoctorStaysQuietWhenShimsAreNotInPlay(t *testing.T) {
	launchedPATH = os.Getenv("PATH")
	t.Cleanup(func() { launchedPATH = "" })

	if w := loginShellDemotesShims(&box.Env{Cfg: config.Config{Enforcement: "both", Intercept: []string{"ls"}}}); w != "" {
		t.Errorf("warned without any boxer shims on PATH: %s", w)
	}
	if w := loginShellDemotesShims(&box.Env{Cfg: config.Config{Enforcement: "hook", Intercept: []string{"ls"}}}); w != "" {
		t.Errorf("warned about shims under hook enforcement: %s", w)
	}
	if w := loginShellDemotesShims(nil); w != "" {
		t.Errorf("warned with no environment at all: %s", w)
	}
}
