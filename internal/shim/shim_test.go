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

func TestShellWrapperExecsBashInTheSandbox(t *testing.T) {
	shims := t.TempDir()
	path, err := InstallShell(shims)
	if err != nil || filepath.Base(path) != "boxer-bash" {
		t.Fatalf("%v %v", path, err)
	}
	bindir := t.TempDir()
	os.WriteFile(filepath.Join(bindir, "boxer"), []byte("#!/bin/sh\necho \"boxer $*\"\n"), 0o755)
	cmd := exec.Command(path, "-i")
	// PROMPT_COMMAND is what a harness driving a terminal relies on: it installs a prompt carrying
	// a metadata block and reads the command's result back out of it. The wrapper has to carry it
	// into the guest, and --norc has to stop the image's rc files reassigning the prompt after.
	cmd.Env = append(os.Environ(), "PATH="+bindir+":/usr/bin:/bin", "PROMPT_COMMAND=export PS1=[MARK]", "TERM=xterm")
	out, err := cmd.Output()
	got := strings.TrimSpace(string(out))
	for _, want := range []string{"boxer run --tty --", "PROMPT_COMMAND=export PS1=[MARK]", "TERM=xterm", "bash --noediting --norc -i"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the shell wrapper does not carry %q: %q %v", want, got, err)
		}
	}
	// An unset variable must not arrive as an empty one, which would blank the prompt outright.
	cmd = exec.Command(path, "-i")
	cmd.Env = []string{"PATH=" + bindir + ":/usr/bin:/bin"}
	out, _ = cmd.Output()
	if strings.Contains(string(out), "PROMPT_COMMAND=") {
		t.Fatalf("an unset PROMPT_COMMAND was passed anyway: %q", out)
	}
}

// A boxer that resolves its own shims calls into itself: the shim runs `boxer run`, and the inner
// boxer waits for the sandbox the outer one is already working on. The smolvm launcher runs
// `uname`, so a `uname` shim on PATH was enough to deadlock a whole session until it was killed.
func TestBoxerRefusesToResolveItsOwnShims(t *testing.T) {
	shims := t.TempDir()
	if _, err := Install(shims, []string{"uname", "npm"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shims, Marker)); err != nil {
		t.Fatalf("an installed shim directory is not marked as one: %v", err)
	}
	ordinary := t.TempDir()
	path := strings.Join([]string{shims, ordinary}, string(filepath.ListSeparator))
	got := SanitizePath(path)
	if strings.Contains(got, shims) {
		t.Errorf("the shim directory survived sanitising: %q", got)
	}
	if !strings.Contains(got, ordinary) {
		t.Errorf("sanitising dropped a directory that is not boxer's: %q", got)
	}
	// A PATH with nothing of boxer's in it comes back unchanged, empty entries aside.
	if got := SanitizePath(ordinary); got != ordinary {
		t.Errorf("an unrelated PATH was rewritten: %q", got)
	}
}

// A shim on a shell interpreter sandboxes every `#!/usr/bin/env bash` script on the host,
// smolvm's own launcher among them, and the sandboxed copy hangs. One such invocation was found
// still running two days after the eval that started it, so the exclusion is a test, not a
// comment.
func TestNeverShimsShellsOrItsOwnTools(t *testing.T) {
	dir := t.TempDir()
	written, err := Install(dir, []string{"sh", "bash", "boxer", "smolvm", "*", "npm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || filepath.Base(written[0]) != "npm" {
		t.Fatalf("wrote %v, want only npm", written)
	}
	for _, p := range []string{"sh", "bash", "boxer", "smolvm", "*"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("%s was shimmed", p)
		}
	}
}

// Installing again removes boxer's shims for programs no longer listed, and nothing else.
func TestInstallRemovesShimsNoLongerListed(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, []string{"npm", "node"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mine"), []byte("#!/bin/sh\necho mine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, []string{"npm"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "node")); !os.IsNotExist(err) {
		t.Fatal("the node shim must go")
	}
	for _, keep := range []string{"npm", "mine", Marker} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Fatalf("%s must stay: %v", keep, err)
		}
	}
}

// The shim takes its own directory out of PATH however it is written there, and leaves every
// other entry as it was: a glob in an entry was expanded, and a trailing slash kept the shim on it.
func TestShimStripsItsDirectoryExactly(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, []string{"npm"}); err != nil {
		t.Fatal(err)
	}
	stub := t.TempDir()
	if err := os.WriteFile(filepath.Join(stub, "boxer"), []byte("#!/bin/sh\nprintf '%s\\n' \"$PATH\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(stub, "g1"), 0o755)
	_ = os.MkdirAll(filepath.Join(stub, "g2"), 0o755)
	path := dir + "/:" + stub + ":" + filepath.Join(stub, "g*") + ":/usr/bin:/bin"
	cmd := exec.Command(filepath.Join(dir, "npm"))
	cmd.Env = []string{"PATH=" + path}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := stub + ":" + filepath.Join(stub, "g*") + ":/usr/bin:/bin"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("PATH handed on:\n got %s\nwant %s", got, want)
	}
}

// Harness shims are boxer's, so boxer drops them from its own PATH; they are not program shims,
// so nothing reads them as npm's.
func TestHarnessShimsHaveTheirOwnMarker(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallHarness(dir, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	if got := SanitizePath(dir + ":/usr/bin"); got != "/usr/bin" {
		t.Fatalf("boxer must not run its own harness shims: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, Marker)); err == nil {
		t.Fatal("a directory of harness shims is not the program shims")
	}
}
