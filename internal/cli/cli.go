// Package cli is how boxer's command line talks to whoever is running it. It is presentation
// only — who is reading, how to colour and align a table, how to ask a question — and it imports
// nothing from boxer, so the core cannot come to depend on how the CLI looks. `cmd/boxer` is the
// only thing that uses it; TestCoreDoesNotImportCLI keeps it that way.
//
// The one decision that shapes everything here is who is reading. A person at a terminal wants
// colour, aligned columns, a hint about what to do next and a question when something is
// destructive. An agent wants none of that: colour codes are noise in its context, a prompt is a
// hang, and a hint is only useful when it is the exact command. Guessing wrong in the first
// direction wastes an agent's tokens; guessing wrong in the second hangs it. So the default is
// derived — a terminal and no agent in the environment means a person — and every guess can be
// overridden: --json, BOXER_OUTPUT=json|text|human, NO_COLOR.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// Mode is who the output is for.
type Mode int

const (
	// Human is a person at a terminal: colour, tables, hints, prompts.
	Human Mode = iota
	// Agent is a coding agent, or anything else reading the output as text: plain, no colour,
	// never a prompt.
	Agent
	// JSON is a program: one JSON document on stdout, nothing else there.
	JSON
)

func (m Mode) String() string {
	switch m {
	case Human:
		return "human"
	case Agent:
		return "agent"
	}
	return "json"
}

// agentEnv names variables that mean an agent is reading, whether or not there is a terminal:
// some harnesses run their shell on a pseudo-terminal, so a terminal alone does not mean a person.
// The list holds only what has been observed. Everything else an agent runs has no terminal, and
// that is caught below; an agent that does have one can set BOXER_AGENT=1.
var agentEnv = []string{
	"CLAUDECODE",   // Claude Code sets it in every shell it spawns; observed, not assumed
	"BOXER_AGENT",  // boxer's own, for any agent that sets nothing of its own
	"BOXER_INSIDE", // a command running inside a boxer guest
}

// Detect decides the mode for a writer, from the environment and whether it is a terminal.
// BOXER_OUTPUT wins; then an agent in the environment; then whether stdout is a terminal.
func Detect(stdout io.Writer, getenv func(string) string) Mode {
	switch strings.ToLower(getenv("BOXER_OUTPUT")) {
	case "json":
		return JSON
	case "text", "agent", "plain":
		return Agent
	case "human":
		return Human
	}
	for _, k := range agentEnv {
		if getenv(k) != "" {
			return Agent
		}
	}
	if getenv("CI") != "" {
		return Agent
	}
	if f, ok := stdout.(*os.File); ok && IsTerminal(f) {
		return Human
	}
	return Agent
}

// IsTerminal reports whether f is a terminal, by asking the kernel for its attributes. A test of
// os.ModeCharDevice is wrong: /dev/null is a character device, and taking it for a terminal once
// made boxer ask a container runtime for a TTY it refused.
func IsTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ioctlGetTermios, uintptr(unsafe.Pointer(&t)))
	return errno == 0
}

// Out is a writer that knows its mode.
type Out struct {
	W     io.Writer
	Mode  Mode
	color bool
}

// New returns an Out for w, detecting the mode unless asJSON forces it.
func New(w io.Writer, asJSON bool) *Out {
	m := Detect(w, os.Getenv)
	if asJSON {
		m = JSON
	}
	return &Out{W: w, Mode: m, color: m == Human && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"}
}

// Printf writes formatted text.
func (o *Out) Printf(format string, a ...any) { fmt.Fprintf(o.W, format, a...) }

// Colours. Each is a no-op unless the output is for a person with colour on.
func (o *Out) paint(code, s string) string {
	if !o.color || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (o *Out) Bold(s string) string   { return o.paint("1", s) }
func (o *Out) Dim(s string) string    { return o.paint("2", s) }
func (o *Out) Green(s string) string  { return o.paint("32", s) }
func (o *Out) Yellow(s string) string { return o.paint("33", s) }
func (o *Out) Red(s string) string    { return o.paint("31", s) }
func (o *Out) Cyan(s string) string   { return o.paint("36", s) }

// State colours a sandbox state the way a person scans for it: running is fine, stopped is
// attention, anything else is a problem.
func (o *Out) State(s string) string {
	switch s {
	case "running", "ok", "yes", "clean", "ready":
		return o.Green(s)
	case "stopped", "dirty", "warn", "absent", "missing":
		return o.Yellow(s)
	case "", "-":
		return o.Dim("-")
	}
	return o.Red(s)
}

// Hint prints the next thing to run. For a person it is a dim aside; for an agent it is the line
// it should act on, in the same `fix:` shape as boxer's refusals.
func (o *Out) Hint(cmd, why string) {
	switch o.Mode {
	case Human:
		fmt.Fprintf(o.W, "%s %s  %s\n", o.Dim("→"), o.Cyan(cmd), o.Dim(why))
	case Agent:
		fmt.Fprintf(o.W, "next: %s  # %s\n", cmd, why)
	}
}

// Table renders rows with aligned columns. Widths are measured on the text as printed, not on its
// colour codes, so a coloured cell does not push its column out.
func (o *Out) Table(header []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	width := make([]int, len(header))
	for i, h := range header {
		width[i] = utf8.RuneCountInString(h)
	}
	for _, r := range rows {
		for i := range header {
			if i < len(r) {
				if n := visibleLen(r[i]); n > width[i] {
					width[i] = n
				}
			}
		}
	}
	line := func(cells []string, style func(string) string) {
		var b strings.Builder
		for i := range header {
			c := ""
			if i < len(cells) {
				c = cells[i]
			}
			if style != nil {
				c = style(c)
			}
			b.WriteString(c)
			if i < len(header)-1 {
				b.WriteString(strings.Repeat(" ", width[i]-visibleLen(c)+2))
			}
		}
		fmt.Fprintln(o.W, strings.TrimRight(b.String(), " "))
	}
	line(header, o.Bold)
	for _, r := range rows {
		line(r, nil)
	}
}

// visibleLen is a string's length without its ANSI escape sequences.
func visibleLen(s string) int {
	n, esc := 0, false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc:
			if r == 'm' {
				esc = false
			}
		default:
			n++
		}
	}
	return n
}
