// Package decide holds the single decision boxer makes about a shell command: leave it alone,
// rewrite it into the sandbox, or refuse it. It is pure so every harness dialect shares one
// tested function and contains no decision of its own (R-CMD-8).
package decide

import (
	"strings"
)

// Action is what the caller should do with the command.
type Action int

const (
	// Allow runs the command unchanged on the host.
	Allow Action = iota
	// Rewrite replaces the command with the sandboxed form in Decision.Command.
	Rewrite
	// Block refuses the command; Decision.Reason and Fix explain to the agent.
	Block
)

// Input is everything the decision depends on. VM state is deliberately absent: `boxer run`
// owns provisioning and the failure policy.
type Input struct {
	Command     string
	Mode        string // rewrite | tool | off
	Intercept   []string
	Passthrough []string
}

// Decision is the outcome. Command is set only for Rewrite.
type Decision struct {
	Action  Action
	Command string
	Reason  string
	Fix     string
}

// Decide applies the decision tree from requirements §3.4.
func Decide(in Input) Decision {
	cmd := strings.TrimSpace(in.Command)
	if in.Mode == "off" || cmd == "" {
		return Decision{Action: Allow}
	}
	if isBoxer(cmd) {
		return Decision{Action: Allow}
	}
	if !needsSandbox(cmd, in.Intercept, in.Passthrough) {
		return Decision{Action: Allow}
	}
	wrapped := "boxer run -c " + shellQuote(cmd)
	switch in.Mode {
	case "tool":
		return Decision{
			Action: Block,
			Reason: "this repository runs commands in a sandbox; use the boxer_run tool or `boxer run`",
			Fix:    wrapped,
		}
	default:
		return Decision{Action: Rewrite, Command: wrapped}
	}
}

// needsSandbox is true when any segment of a compound command starts with an intercepted
// program and no segment is exempt by the passthrough list. Compound input is wrapped whole
// (R-CMD-3), so one intercepted segment decides for all.
func needsSandbox(cmd string, intercept, passthrough []string) bool {
	all := len(intercept) == 1 && intercept[0] == "*"
	for _, seg := range splitSegments(cmd) {
		prog := firstProgram(seg)
		if prog == "" || contains(passthrough, prog) {
			continue
		}
		if all || contains(intercept, prog) {
			return true
		}
	}
	return false
}

// splitSegments breaks on the shell operators that sequence commands. Quoting is not honoured;
// a quoted `&&` produces a spurious empty segment, which firstProgram ignores.
func splitSegments(cmd string) []string {
	r := strings.NewReplacer("&&", "\n", "||", "\n", "|", "\n", ";", "\n")
	return strings.Split(r.Replace(cmd), "\n")
}

// firstProgram returns the program name of one segment, skipping leading environment
// assignments, `cd dir &&`-style prefixes are already split off, and path prefixes.
func firstProgram(seg string) string {
	for _, tok := range strings.Fields(seg) {
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "=") {
			continue
		}
		if tok == "(" || tok == "{" || tok == "!" {
			continue
		}
		tok = strings.TrimLeft(tok, "({")
		if i := strings.LastIndex(tok, "/"); i >= 0 {
			tok = tok[i+1:]
		}
		return tok
	}
	return ""
}

func isBoxer(cmd string) bool {
	return firstProgram(cmd) == "boxer"
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// shellQuote wraps s in single quotes for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
