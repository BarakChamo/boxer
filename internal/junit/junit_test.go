package junit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParsesBothRoots(t *testing.T) {
	nested := `<testsuites><testsuite name="cart"><testcase name="a"/><testcase name="b"><failure message="no"/></testcase></testsuite></testsuites>`
	s, err := Parse([]byte(nested))
	if err != nil {
		t.Fatal(err)
	}
	if s.Tests != 2 || s.Failures != 1 || s.Passed() != 1 {
		t.Fatalf("nested: %+v", s)
	}
	if len(s.Failed) != 1 || s.Failed[0] != "cart/b" {
		t.Fatalf("failed names: %v", s.Failed)
	}
	bare := `<testsuite name="api"><testcase name="x"><error message="boom"/></testcase></testsuite>`
	s, err = Parse([]byte(bare))
	if err != nil {
		t.Fatal(err)
	}
	if s.Tests != 1 || s.Errors != 1 || s.Failed[0] != "api/x" {
		t.Fatalf("bare: %+v", s)
	}
}

// Frameworks disagree about what the tests= and failures= attributes count, so the parse ignores
// them. A report whose attributes claim everything passed while a case carries <failure> is the
// case this exists for.
func TestCountsComeFromCasesNotAttributes(t *testing.T) {
	x := `<testsuite name="s" tests="99" failures="0"><testcase name="a"><failure/></testcase></testsuite>`
	s, err := Parse([]byte(x))
	if err != nil {
		t.Fatal(err)
	}
	if s.Tests != 1 || s.Failures != 1 || !s.Bad() {
		t.Fatalf("%+v", s)
	}
}

func TestSkippedIsNotAFailure(t *testing.T) {
	s, err := Parse([]byte(`<testsuite name="s"><testcase name="a"><skipped/></testcase></testsuite>`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 1 || s.Bad() || s.Passed() != 0 {
		t.Fatalf("%+v", s)
	}
}

func TestMalformedXMLIsAnError(t *testing.T) {
	if _, err := Parse([]byte("<testsuite>")); err == nil {
		t.Fatal("truncated XML must not parse as an empty pass")
	}
}

// A report older than the run describes a different run. Counting it would let a command that
// never wrote a report inherit yesterday's green.
func TestCollectIgnoresAReportOlderThanTheRun(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "junit.xml")
	if err := os.WriteFile(p, []byte(`<testsuite name="s"><testcase name="a"/></testsuite>`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	s, err := Collect(dir, []string{"junit.xml"}, time.Now().Add(-time.Minute))
	if err != nil || len(s.Files) != 0 || s.Tests != 0 {
		t.Fatalf("stale report must be ignored: %+v %v", s, err)
	}
	now := time.Now()
	if err := os.Chtimes(p, now, now); err != nil {
		t.Fatal(err)
	}
	s, err = Collect(dir, []string{"junit.xml", "missing/*.xml"}, now.Add(-time.Minute))
	if err != nil || s.Tests != 1 || len(s.Files) != 1 {
		t.Fatalf("fresh report must count, and a pattern matching nothing is not an error: %+v %v", s, err)
	}
}
