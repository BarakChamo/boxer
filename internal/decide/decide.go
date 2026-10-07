// Package decide holds the single decision boxer makes about a shell command: leave it alone,
// rewrite it into the sandbox, or refuse it. It is pure so every harness dialect shares one
// tested function and contains no decision of its own (R-CMD-8).
package decide

import (
	"path"
	"slices"
	"strings"

	"github.com/BarakChamo/boxer/internal/sh"
)

// containment are the BOXER_ variables that change where a command runs or what it may reach.
// Setting one from a command line is refused; the others (BOXER_OUTPUT, BOXER_BACKEND, ...) only
// change how boxer reports or which runtime it uses, and the docs show them set that way.
var containment = map[string]bool{
	"BOXER_INSIDE": true, "BOXER_MODE": true, "BOXER_ENFORCEMENT": true, "BOXER_ON_SANDBOX_UNAVAILABLE": true,
	"BOXER_INTEGRATION": true, "BOXER_NETWORK_MODE": true, "BOXER_SHIM": true, "BOXER_SMOLVM": true,
}

// setsBoxerEnv reports a line that assigns a containment variable: as a prefix (`BOXER_X=1 cmd`),
// as an argument to export, env, declare, typeset, readonly or local, through `printf -v` or
// `read`, or as a bare name those export, at any depth the reader reaches. Only setting counts:
// `grep -rn BOXER_MODE= .` and `echo "BOXER_INSIDE=1"` name one without setting it.
func setsBoxerEnv(cmd string, depth int) bool {
	if depth > maxDepth {
		return false
	}
	cmds, subs, ok := lex(cmd)
	if !ok {
		return false
	}
	for _, sub := range subs {
		if setsBoxerEnv(sub, depth+1) {
			return true
		}
	}
	for _, words := range cmds {
		if setsInWords(words, depth) {
			return true
		}
	}
	return false
}

// varName is the variable a word names or assigns: `BOXER_MODE+=x`, `BOXER_MODE[0]=x` and
// `BOXER_MODE` are all BOXER_MODE.
func varName(w string) string {
	name, _, _ := strings.Cut(w, "=")
	name = strings.TrimSuffix(name, "+")
	if k := strings.IndexByte(name, '['); k > 0 {
		name = name[:k]
	}
	return name
}

func setsInWords(words []string, depth int) bool {
	k := 0
	for k < len(words) {
		w := words[k]
		switch {
		case strings.Contains(w, "=") && isAssignment(varName(w)+"=x"):
			if containment[varName(w)] {
				return true
			}
			k++
			continue
		case keywords[w]:
			k++
			continue
		}
		if _, ok := wrappers[path.Base(w)]; ok && path.Base(w) != "env" {
			j, ok := skipWrapper(path.Base(w), words[k+1:])
			if !ok {
				j = 0
			}
			for _, a := range words[k+1 : k+1+min(j, len(words)-k-1)] {
				if isAssignment(a) && containment[varName(a)] { // sudo VAR=value cmd
					return true
				}
			}
			k += 1 + j
			continue
		}
		break
	}
	if k >= len(words) {
		return false
	}
	args := words[k+1:]
	switch path.Base(words[k]) {
	case "export", "declare", "typeset", "readonly", "local":
		for _, a := range args {
			if strings.HasPrefix(a, "-") {
				continue
			}
			// A bare name exports whatever this line set it to; a nameref (`declare -n r=BOXER_MODE`)
			// makes another name set it.
			_, val, _ := strings.Cut(a, "=")
			if containment[varName(a)] || containment[val] {
				return true
			}
		}
	case "env":
		for _, a := range args {
			if isAssignment(a) && containment[varName(a)] {
				return true
			}
		}
		for j, a := range args {
			if (a == "-S" || a == "--split-string") && j+1 < len(args) && setsBoxerEnv(args[j+1], depth+1) {
				return true
			}
		}
		if j, ok := skipWrapper("env", args); ok && j < len(args) {
			return setsInWords(args[j:], depth)
		}
	case "printf":
		for j, a := range args {
			if a == "-v" && j+1 < len(args) && containment[varName(args[j+1])] {
				return true
			}
		}
	case "read", "mapfile", "readarray":
		for _, a := range args {
			if !strings.HasPrefix(a, "-") && containment[varName(a)] {
				return true
			}
		}
	case "sh", "bash", "zsh", "dash", "ksh", "ash", "eval", "trap", "watch", "su", "script":
		for _, a := range args {
			if !strings.HasPrefix(a, "-") && setsBoxerEnv(a, depth+1) {
				return true
			}
		}
	}
	return false
}

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
	// A line that sets one of boxer's own variables is changing how boxer treats it:
	// `BOXER_INSIDE=1 boxer run -c 'npm i'` ran on the host, and `export BOXER_MODE=off` would
	// switch off every later command in a persistent shell. Neither is an agent's to decide.
	if setsBoxerEnv(cmd, 0) {
		return Decision{
			Action: Block,
			Reason: "this line sets one of boxer's own variables (BOXER_*); they are the operator's configuration, not a command's",
			Fix:    "run the command without the BOXER_ setting",
		}
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
	for _, raw := range progs {
		prog := path.Base(raw)
		if prog == "" || prog == "." || prog == "boxer" || slices.Contains(passthrough, prog) {
			continue
		}
		if all || slices.Contains(intercept, prog) {
			return true
		}
	}
	return false
}

// NamedByPath reports whether the line runs an intercepted program by a path rather than by name
// (`/opt/homebrew/bin/npm i`, `./node_modules/.bin/node x`), which a PATH shim never sees. It looks
// at the program positions the reader found, including inside `sh -c` and substitutions; a path
// elsewhere in the line (`cd packages/node && npm test`) does not count. A line it cannot read
// counts, so a block-only harness refuses it rather than trust a shim.
func NamedByPath(cmd string, intercept []string) bool {
	progs, ok := programs(cmd, 0)
	if !ok || choosesPath(cmd) {
		return true
	}
	all := len(intercept) == 1 && intercept[0] == "*"
	for _, raw := range progs {
		if strings.Contains(raw, "/") && (all || slices.Contains(intercept, path.Base(raw))) {
			return true
		}
	}
	return false
}

// choosesPath reports a line that changes where a bare name is looked up, which a PATH shim then
// never sees: `PATH=/opt/homebrew/bin npm i`, `export PATH=...`, `env -P dir npm`, `command -p npm`,
// `hash -p /real/npm npm`. Called only on a line programs could read, so every part of it lexes
// and the nesting is bounded.
func choosesPath(cmd string) bool {
	cmds, subs, _ := lex(cmd)
	for _, sub := range subs {
		if choosesPath(sub) {
			return true
		}
	}
	for _, words := range cmds {
		for j, w := range words {
			if varName(w) == "PATH" && strings.Contains(w, "=") {
				return true
			}
			if j+1 < len(words) && words[j+1] == "-p" && (w == "command" || w == "hash") || w == "env" && j+1 < len(words) && words[j+1] == "-P" {
				return true
			}
			if shells[path.Base(w)] || w == "eval" {
				for _, a := range words[j+1:] {
					if !strings.HasPrefix(a, "-") && choosesPath(a) {
						return true
					}
				}
			}
		}
	}
	return false
}
