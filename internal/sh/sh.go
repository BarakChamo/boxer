// Package sh quotes strings for a POSIX shell.
//
// boxer builds shell command lines in two places — the setup and start lists that run in the
// guest, and the decision path that rewrites a harness's command into `boxer run` — and both have
// to quote the same way. Two copies of a quoting rule drift, and a quoting rule that drifts is how
// a path with a space becomes two arguments and a path with a quote becomes a second command.
package sh

import "strings"

// Quote wraps s in single quotes so a POSIX shell reads it as one literal argument.
//
// Single quotes take everything literally except a single quote, which cannot be escaped inside
// them at all: the sequence below closes the quoted run, emits an escaped quote, and opens a new
// one. That is the only portable spelling, and it is why this is a function rather than a format
// string somebody writes by hand each time.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
