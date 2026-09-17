package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShimExecsBoxerRunWithItselfOffPath(t *testing.T) {
	shims := t.TempDir()
	paths, err := Install(shims, []string{"npm", "*", "boxer", "smolvm", "../evil"})
	if err != nil || len(paths) != 1 || filepath.Base(paths[0]) != "npm" {
		t.Fatalf("%v %v", paths, err)
	}
	// A fake boxer elsewhere on PATH records its argv and the PATH it received.
	bindir := t.TempDir()
	os.WriteFile(filepath.Join(bindir, "boxer"), []byte("#!/bin/sh\necho \"boxer $*\"\necho \"PATH=$PATH\"\n"), 0o755)
	cmd := exec.Command(paths[0], "run", "check")
	cmd.Env = append(os.Environ(), "PATH="+shims+":"+bindir+":/usr/bin:/bin")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if lines[0] != "boxer run -- npm run check" {
		t.Fatalf("argv: %q", lines[0])
	}
	if strings.Contains(lines[1], shims) {
		t.Fatalf("shim dir must be stripped from PATH, got %s", lines[1])
	}
	if !strings.Contains(lines[1], bindir) {
		t.Fatalf("rest of PATH must survive: %s", lines[1])
	}
}

func TestHarnessShimExecsBoxerShell(t *testing.T) {
	shims := t.TempDir()
	paths, err := InstallHarness(shims, []string{"claude"})
	if err != nil || len(paths) != 1 {
		t.Fatalf("%v %v", paths, err)
	}
	bindir := t.TempDir()
	os.WriteFile(filepath.Join(bindir, "boxer"), []byte("#!/bin/sh\necho \"boxer $*\"\n"), 0o755)
	cmd := exec.Command(paths[0], "-p", "hi")
	cmd.Env = append(os.Environ(), "PATH="+shims+":"+bindir+":/usr/bin:/bin")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "boxer shell claude -- -p hi" {
		t.Fatalf("%q %v", out, err)
	}
}
