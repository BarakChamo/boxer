package cli

import (
	"os"
	"testing"
)

// /dev/null is a character device and not a terminal. Confusing the two requested a TTY for every
// run whose output was discarded, and a container runtime refuses that before the command starts.
func TestDevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Fatal("/dev/null reported as a terminal")
	}
	p, err := os.CreateTemp(t.TempDir(), "f")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if IsTerminal(p) {
		t.Fatal("a regular file reported as a terminal")
	}
}
