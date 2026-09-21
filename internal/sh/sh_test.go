package sh

import (
	"os/exec"
	"strings"
	"testing"
)

// The point of quoting is that a real shell reads back exactly what went in, so the test asks one.
func TestAShellReadsBackWhatWasQuoted(t *testing.T) {
	for _, in := range []string{
		"plain",
		"with a space",
		"it's quoted",
		`a "double" quote`,
		"semicolon; echo pwned",
		"$(echo substitution)",
		"back\\slash",
		"new\nline",
		"",
	} {
		out, err := exec.Command("sh", "-c", "printf %s "+Quote(in)).Output()
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got := string(out); got != in {
			t.Errorf("a shell read %q back as %q", in, got)
		}
	}
}

func TestQuoteClosesTheQuotedRunAroundEachQuote(t *testing.T) {
	if got := Quote("it's"); !strings.Contains(got, `'\''`) {
		t.Errorf("a single quote was not escaped the only way a POSIX shell accepts: %s", got)
	}
}
