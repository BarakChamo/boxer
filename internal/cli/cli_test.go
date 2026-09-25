package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

// Who is reading decides everything else, so the decision is tested in every direction it can go:
// an explicit override wins, an agent in the environment wins over a terminal, and anything that
// is not a terminal is treated as a program rather than a person.
func TestDetect(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range []struct {
		name string
		env  func(string) string
		want Mode
	}{
		{"not a terminal", env(), Agent},
		{"json override", env("BOXER_OUTPUT", "json"), JSON},
		{"human override", env("BOXER_OUTPUT", "human", "CLAUDECODE", "1"), Human},
		{"text override", env("BOXER_OUTPUT", "text"), Agent},
		{"claude code", env("CLAUDECODE", "1"), Agent},
		{"boxer agent", env("BOXER_AGENT", "1"), Agent},
		{"ci", env("CI", "true"), Agent},
	} {
		if got := Detect(&buf, tc.env); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	// A file is not a terminal, and neither is /dev/null.
	f, _ := os.Create(filepath.Join(t.TempDir(), "f"))
	defer f.Close()
	if Detect(f, env()) != Agent {
		t.Error("a file is not a person")
	}
}

// Colour must not move a column: widths are measured on what is printed.
func TestTableAlignsColouredCells(t *testing.T) {
	var buf bytes.Buffer
	o := &Out{W: &buf, Mode: Human, color: true}
	o.Table([]string{"NAME", "STATE", "X"}, [][]string{{"a", o.Green("running"), "1"}, {"bbbb", o.Yellow("stopped"), "2"}})
	var starts []int
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		plain := stripANSI(line)
		starts = append(starts, strings.LastIndex(plain, " ")+1)
	}
	for _, s := range starts[1:] {
		if s != starts[0] {
			t.Fatalf("columns moved: %v\n%s", starts, buf.String())
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc:
			if r == 'm' {
				esc = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// An agent gets no colour and the hint as a command it can act on; JSON gets no hint at all.
func TestAgentOutputIsPlain(t *testing.T) {
	var buf bytes.Buffer
	o := &Out{W: &buf, Mode: Agent}
	o.Table([]string{"A"}, [][]string{{o.Green("x")}})
	o.Hint("boxer rm --gone", "why")
	if strings.Contains(buf.String(), "\x1b[") || !strings.Contains(buf.String(), "next: boxer rm --gone") {
		t.Fatalf("%q", buf.String())
	}
	buf.Reset()
	(&Out{W: &buf, Mode: JSON}).Hint("x", "y")
	if buf.Len() != 0 {
		t.Fatalf("a hint in JSON output would corrupt it: %q", buf.String())
	}
}
