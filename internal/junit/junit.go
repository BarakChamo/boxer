// Package junit parses JUnit XML into the one question a failed run has to answer: which tests
// failed. It reads the files on the host, because the worktree is mounted and whatever the guest
// wrote under it is already a host file.
package junit

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Summary is the parsed result of one or more report files.
type Summary struct {
	Tests    int
	Failures int
	Errors   int
	Skipped  int
	// Failed names every failing and erroring case, "suite/case", sorted.
	Failed []string
	// Files are the reports that were actually read.
	Files []string
}

// Passed is the count that is neither failed, errored, nor skipped.
func (s Summary) Passed() int { return s.Tests - s.Failures - s.Errors - s.Skipped }

// Bad reports whether anything failed or errored. A skip is not a failure.
func (s Summary) Bad() bool { return s.Failures+s.Errors > 0 }

// Line is the one-line form for a terminal.
func (s Summary) Line() string {
	return fmt.Sprintf("tests %d passed, %d failed, %d error, %d skipped (%d file(s))",
		s.Passed(), s.Failures, s.Errors, s.Skipped, len(s.Files))
}

// suite decodes both roots. <testsuites> nests <testsuite>; a bare <testsuite> decodes into the
// same struct with no children, because encoding/xml ignores the root element's name unless the
// type declares an XMLName.
type suite struct {
	Name   string  `xml:"name,attr"`
	Suites []suite `xml:"testsuite"`
	Cases  []struct {
		Name      string `xml:"name,attr"`
		Classname string `xml:"classname,attr"`
		Failure   *struct {
			Message string `xml:"message,attr"`
		} `xml:"failure"`
		Error *struct {
			Message string `xml:"message,attr"`
		} `xml:"error"`
		Skipped *struct{} `xml:"skipped"`
	} `xml:"testcase"`
}

// Parse reads one report. Counts come from the cases and never from the tests=/failures=
// attributes: frameworks disagree about what those mean, and they are the part that lies.
func Parse(b []byte) (Summary, error) {
	var root suite
	if err := xml.Unmarshal(b, &root); err != nil {
		return Summary{}, err
	}
	var s Summary
	walk(root, "", &s)
	sort.Strings(s.Failed)
	return s, nil
}

func walk(su suite, parent string, s *Summary) {
	name := su.Name
	if parent != "" && name != "" {
		name = parent + "/" + name
	} else if name == "" {
		name = parent
	}
	for _, c := range su.Cases {
		s.Tests++
		id := c.Name
		if c.Classname != "" {
			id = c.Classname + "." + c.Name
		}
		if name != "" {
			id = name + "/" + id
		}
		switch {
		case c.Failure != nil:
			s.Failures++
			s.Failed = append(s.Failed, id)
		case c.Error != nil:
			s.Errors++
			s.Failed = append(s.Failed, id)
		case c.Skipped != nil:
			s.Skipped++
		}
	}
	for _, child := range su.Suites {
		walk(child, name, s)
	}
}

// Collect parses every report matching the patterns, resolved against root. Only files modified
// at or after since are read: a stale green report from yesterday reporting success is the one
// failure mode that would make this actively harmful.
//
// A pattern that matches nothing is not an error — a run that failed before it wrote its report
// is exactly when this is called.
func Collect(root string, patterns []string, since time.Time) (Summary, error) {
	var out Summary
	seen := map[string]bool{}
	for _, pat := range patterns {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}
		if !filepath.IsAbs(pat) {
			pat = filepath.Join(root, pat)
		}
		matches, err := filepath.Glob(pat)
		if err != nil {
			return out, fmt.Errorf("junit pattern %q: %w", pat, err)
		}
		for _, m := range matches {
			if seen[m] {
				continue
			}
			st, err := os.Stat(m)
			if err != nil || st.IsDir() || st.ModTime().Before(since) {
				continue
			}
			b, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			s, err := Parse(b)
			if err != nil {
				return out, fmt.Errorf("%s: %w", m, err)
			}
			seen[m] = true
			out.Tests += s.Tests
			out.Failures += s.Failures
			out.Errors += s.Errors
			out.Skipped += s.Skipped
			out.Failed = append(out.Failed, s.Failed...)
			out.Files = append(out.Files, m)
		}
	}
	sort.Strings(out.Failed)
	sort.Strings(out.Files)
	return out, nil
}
