package vm

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// The `smolvm` on PATH is a bash script whose whole job is to set a library path and exec the real
// binary beside it. That wrapper costs a bash process per call: measured on an M4, `smolvm
// --version` takes 22.6ms through the wrapper and 10.1ms calling the binary directly.
//
// It matters because boxer's cost is dominated by how many times it shells out. A warm `boxer run`
// makes two smolvm calls, so the wrapper alone was ~25ms of a 57ms command — more than everything
// else boxer does put together.
//
// This is deliberately a *detection*, not an assumption. boxer only skips the wrapper when it can
// see the exact shape the wrapper has: a `smolvm-bin` executable and a `lib` directory as siblings
// of the resolved script. Anything else — a differently built distribution, a future wrapper that
// does more than set a path, a package manager that installs the binary directly — falls through
// to invoking whatever is on PATH, which is always correct if slower. `BOXER_SMOLVM` still wins
// over all of it, because someone who names a binary means it.

type resolved struct {
	bin string   // what to execute
	env []string // extra environment it needs, nil for none
}

// exe returns the binary to run and any environment it needs.
//
// Deliberately not memoised. The obvious cache — a package-level sync.Once — was wrong, because
// Client.Bin differs between clients and a cache keyed on nothing hands one client another's
// answer: a test using a deliberately missing binary got a previously resolved real one and
// stopped seeing the error it was asserting. The work here is a PATH lookup and two stats, tens
// of microseconds against the 13ms process spawn it precedes, so there is nothing to save.
func (c Client) exe() resolved {
	// An explicitly named binary is used exactly as given: no detection, no substitution.
	if os.Getenv("BOXER_SMOLVM") != "" {
		return resolved{bin: c.Bin}
	}
	if r := unwrap(c.Bin); r.bin != "" {
		return r
	}
	return resolved{bin: c.Bin}
}

// unwrap returns the real binary behind the smolvm wrapper script, or an empty resolved when the
// layout is not the one this knows about.
func unwrap(name string) resolved {
	path, err := exec.LookPath(name)
	if err != nil {
		return resolved{}
	}
	// Follow symlinks: the wrapper is often linked into ~/.local/bin from the distribution.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	dir := filepath.Dir(path)
	bin := filepath.Join(dir, "smolvm-bin")
	lib := filepath.Join(dir, "lib")
	st, err := os.Stat(bin)
	if err != nil || st.IsDir() || st.Mode().Perm()&0o111 == 0 {
		return resolved{}
	}
	if d, err := os.Stat(lib); err != nil || !d.IsDir() {
		return resolved{}
	}
	// The wrapper's only other job. Keep any existing value: a caller may have set one.
	key := "LD_LIBRARY_PATH"
	if runtime.GOOS == "darwin" {
		key = "DYLD_LIBRARY_PATH"
	}
	val := lib
	if prev := os.Getenv(key); prev != "" {
		val += string(os.PathListSeparator) + prev
	}
	return resolved{bin: bin, env: []string{key + "=" + val}}
}

// command builds an *exec.Cmd for smolvm, going straight to the real binary when boxer recognises
// the wrapper.
func (c Client) command(args ...string) *exec.Cmd {
	r := c.exe()
	cmd := exec.Command(r.bin, args...)
	if len(r.env) > 0 {
		cmd.Env = append(os.Environ(), r.env...)
	}
	return cmd
}
