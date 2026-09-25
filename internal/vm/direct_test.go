package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Skipping the wrapper is a detection, never an assumption: a distribution that does not have the
// exact `smolvm-bin` + `lib` shape beside the resolved script must fall through to whatever is on
// PATH, which is always correct if slower.
func TestUnwrapOnlyFiresOnTheLayoutItKnows(t *testing.T) {
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "smolvm")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec true\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	// No sibling binary: fall through.
	if r := unwrap("smolvm"); r.bin != "" {
		t.Errorf("unwrapped with no smolvm-bin present: %+v", r)
	}
	// A binary but no lib directory: still fall through, because the wrapper's other job is the
	// library path and guessing it wrong is a broken exec, not a slow one.
	bin := filepath.Join(dir, "smolvm-bin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec true\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := unwrap("smolvm"); r.bin != "" {
		t.Errorf("unwrapped with no lib directory: %+v", r)
	}
	// A non-executable sibling is not a binary.
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := unwrap("smolvm"); r.bin != "" {
		t.Errorf("unwrapped a non-executable sibling: %+v", r)
	}
	// The full shape: now it fires, and it carries the library path.
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	r := unwrap("smolvm")
	// Compared through EvalSymlinks: on macOS the temp directory is under /var, which is a symlink
	// to /private/var, and unwrap resolves it deliberately so the sibling lookup is done on the
	// real path.
	want, _ := filepath.EvalSymlinks(bin)
	if r.bin != want {
		t.Fatalf("want %s, got %+v", want, r)
	}
	if len(r.env) != 1 || !strings.HasSuffix(r.env[0], string(os.PathSeparator)+"lib") {
		t.Fatalf("want a library path in the environment, got %+v", r.env)
	}
}

// An explicitly named binary is used exactly as given: someone who sets it means it.
func TestExplicitBinaryIsNeverSubstituted(t *testing.T) {
	t.Setenv("BOXER_SMOLVM", "/opt/custom/smolvm")
	if r := (Client{Bin: "/opt/custom/smolvm"}).exe(); r.bin != "/opt/custom/smolvm" || r.env != nil {
		t.Fatalf("an explicit binary was rewritten: %+v", r)
	}
}
