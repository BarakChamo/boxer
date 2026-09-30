// Package decide holds the single decision boxer makes about a shell command: leave it alone,
// rewrite it into the sandbox, or refuse it. It is pure so every harness dialect shares one
// tested function and contains no decision of its own (R-CMD-8).
package decide

import (
	"github.com/BarakChamo/boxer/internal/sh"
	"slices"
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
	if !needsSandbox(cmd, in.Intercept, in.Passthrough) {
		return Decision{Action: Allow}
	}
	wrapped := "boxer run -c " + sh.Quote(cmd)
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

// needsSandbox is true when any program the line runs is intercepted and not passthrough,
// including programs inside command substitution, `sh -c`, `eval` and wrappers such as `env`,
// `sudo` and `timeout`. The line is then wrapped whole (R-CMD-3). boxer itself always stays on
// the host: a line that is only `boxer run ...` is already sandboxed, and wrapping it would run
// boxer inside the guest. A line boxer cannot read is sandboxed rather than let through.
func needsSandbox(cmd string, intercept, passthrough []string) bool {
	all := len(intercept) == 1 && intercept[0] == "*"
	progs, ok := programs(cmd, 0)
	if !ok {
		return len(intercept) > 0
	}
	for _, prog := range progs {
		if prog == "" || prog == "boxer" || slices.Contains(passthrough, prog) {
			continue
		}
		if all || slices.Contains(intercept, prog) {
			return true
		}
	}
	return false
}
