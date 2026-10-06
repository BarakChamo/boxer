package decide

import (
	"path"
	"strings"
)

// maxDepth bounds how far programs are looked for inside `sh -c`, `eval` and command
// substitution. A line nested deeper than this is treated as naming every program it could, which
// sends it to the sandbox: an unreadable command must not be the one way around the intercept.
const maxDepth = 6

// programs returns the program each simple command in cmd runs, in order, including the commands
// inside command substitution, `sh -c` and `eval`. ok is false when the line was too deeply
// nested to read, or its quoting never closed.
//
// This is a reader, not a shell. It exists because the question "which programs does this line
// run?" was answered by splitting on `&&`, `||`, `|` and `;` and taking the first word, and every
// shape that answer missed ran an intercepted program on the host: `true & npm i`,
// `sh -c 'npm i'`, `env npm i`, `if npm test; then`, `echo $(npm i)`. Quotes are honoured, so an
// operator inside a string is part of the string.
func programs(cmd string, depth int) (progs []string, ok bool) {
	if depth > maxDepth {
		return nil, false
	}
	cmds, subs, ok := lex(cmd)
	if !ok {
		return nil, false
	}
	for _, s := range subs {
		p, ok := programs(s, depth+1)
		if !ok {
			return nil, false
		}
		progs = append(progs, p...)
	}
	for _, words := range cmds {
		p, ok := program(words, depth)
		if !ok {
			return nil, false
		}
		progs = append(progs, p...)
	}
	return progs, true
}

// lex splits cmd into simple commands, each a list of words with quoting removed, and returns
// the text of every command substitution separately, to be read as commands of their own.
// Redirections and their targets are dropped. ok is false for an unterminated quote or
// substitution.
func lex(cmd string) (cmds [][]string, subs []string, ok bool) {
	var words []string
	var word strings.Builder
	inWord := false
	redirect := false // the next word is a redirection target
	flushWord := func() {
		if !inWord {
			return
		}
		if redirect {
			redirect = false
		} else {
			words = append(words, word.String())
		}
		word.Reset()
		inWord = false
	}
	flushCmd := func() {
		flushWord()
		if len(words) > 0 {
			cmds = append(cmds, words)
		}
		words = nil
	}
	r := []rune(cmd)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\\':
			if i+1 < len(r) {
				i++
				if r[i] != '\n' {
					word.WriteRune(r[i])
					inWord = true
				}
			}
		case c == '\'':
			j := indexRune(r, i+1, '\'')
			if j < 0 {
				return nil, nil, false
			}
			word.WriteString(string(r[i+1 : j]))
			inWord = true
			i = j
		case c == '"':
			j, inner, ok := doubleQuoted(r, i+1)
			if !ok {
				return nil, nil, false
			}
			subs = append(subs, inner...)
			word.WriteString(string(r[i+1 : j]))
			inWord = true
			i = j
		case c == '`':
			j := indexRune(r, i+1, '`')
			if j < 0 {
				return nil, nil, false
			}
			subs = append(subs, string(r[i+1:j]))
			inWord = true
			i = j
		case c == '$' && i+1 < len(r) && r[i+1] == '(':
			if i+2 < len(r) && r[i+2] == '(' {
				// $(( arithmetic )) runs nothing itself, but a $(...) inside it still runs: only
				// the substitutions inside the expression are read, never the expression.
				j := matchParen(r, i+2)
				if j < 0 {
					return nil, nil, false
				}
				_, inner, ok := lex(string(r[i+3 : j]))
				if !ok {
					return nil, nil, false
				}
				subs = append(subs, inner...)
				inWord = true
				i = j + 1
				continue
			}
			j := matchParen(r, i+1)
			if j < 0 {
				return nil, nil, false
			}
			subs = append(subs, string(r[i+2:j]))
			inWord = true
			i = j
		case (c == '<' || c == '>') && i+1 < len(r) && r[i+1] == '(':
			j := matchParen(r, i+1)
			if j < 0 {
				return nil, nil, false
			}
			subs = append(subs, string(r[i+2:j]))
			i = j
		case c == '<' || c == '>':
			// A redirection: an optional fd already in the word (2>), the operator, and a target.
			if inWord && isDigits(word.String()) {
				word.Reset()
				inWord = false
			} else {
				flushWord()
			}
			for i+1 < len(r) && (r[i+1] == '>' || r[i+1] == '<' || r[i+1] == '&' || r[i+1] == '|') {
				i++
			}
			// >&2 and <&- name a descriptor, not a file.
			if i+1 < len(r) && (isDigit(r[i+1]) || r[i+1] == '-') && r[i] == '&' {
				for i+1 < len(r) && (isDigit(r[i+1]) || r[i+1] == '-') {
					i++
				}
				continue
			}
			redirect = true
		case c == '&' && i+1 < len(r) && r[i+1] == '>':
			flushWord() // &> file
		case c == ';' || c == '&' || c == '|' || c == '\n' || c == '(' || c == ')':
			flushCmd()
		case c == ' ' || c == '\t' || c == '\r':
			flushWord()
		case c == '#' && !inWord:
			for i < len(r) && r[i] != '\n' {
				i++
			}
			flushCmd()
		default:
			word.WriteRune(c)
			inWord = true
		}
	}
	flushCmd()
	return cmds, subs, true
}

// doubleQuoted scans a "..." string starting after the opening quote. It returns the index of the
// closing quote and the command substitutions inside it, which run even when quoted.
func doubleQuoted(r []rune, from int) (end int, subs []string, ok bool) {
	for i := from; i < len(r); i++ {
		switch {
		case r[i] == '\\':
			i++
		case r[i] == '"':
			return i, subs, true
		case r[i] == '`':
			j := indexRune(r, i+1, '`')
			if j < 0 {
				return 0, nil, false
			}
			subs = append(subs, string(r[i+1:j]))
			i = j
		case r[i] == '$' && i+1 < len(r) && r[i+1] == '(':
			j := matchParen(r, i+1)
			if j < 0 {
				return 0, nil, false
			}
			if i+2 < len(r) && r[i+2] == '(' { // arithmetic: only what it substitutes
				_, inner, ok := lex(string(r[i+3 : j]))
				if !ok {
					return 0, nil, false
				}
				subs = append(subs, inner...)
			} else {
				subs = append(subs, string(r[i+2:j]))
			}
			i = j
		}
	}
	return 0, nil, false
}

// matchParen returns the index of the ')' closing the '(' at open, honouring quotes and nesting.
func matchParen(r []rune, open int) int {
	depth := 0
	for i := open; i < len(r); i++ {
		switch r[i] {
		case '\\':
			i++
		case '\'':
			j := indexRune(r, i+1, '\'')
			if j < 0 {
				return -1
			}
			i = j
		case '"':
			j, _, ok := doubleQuoted(r, i+1)
			if !ok {
				return -1
			}
			i = j
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func indexRune(r []rune, from int, c rune) int {
	for i := from; i < len(r); i++ {
		if r[i] == c {
			return i
		}
	}
	return -1
}

func isDigit(c rune) bool { return c >= '0' && c <= '9' }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !isDigit(c) {
			return false
		}
	}
	return true
}

// keywords begin or continue a compound command; the program is the word after them.
var keywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true, "do": true, "done": true,
	"while": true, "until": true, "!": true, "{": true, "}": true, "esac": true,
	"]]": true, "function": true, "coproc": true,
}

// headers name a construct whose remaining words are data, not a program: `for x in a b`,
// `case $x in`, `select x in a b`.
var headers = map[string]bool{"for": true, "case": true, "select": true, "[[": true}

// wrappers run the program given in their arguments. The value lists the options that take a
// separate argument, so the argument is not mistaken for the program.
var wrappers = map[string]string{
	"env":        "-u -C -S",
	"sudo":       "-u -g -C -D -h -p -r -t -U -T",
	"doas":       "-u -C",
	"nohup":      "",
	"nice":       "-n",
	"ionice":     "-c -n -p",
	"time":       "-f -o",
	"timeout":    "-s -k",
	"command":    "",
	"builtin":    "",
	"exec":       "-a",
	"stdbuf":     "-i -o -e",
	"xargs":      "-I -i -n -P -L -l -d -E -e -s -a",
	"caffeinate": "-w -t",
	"chronic":    "",
	"unbuffer":   "",
	"setsid":     "",
	"strace":     "-o -e -p -s",
	"arch":       "",
}

// shells run a command line given with -c, which is read as a line of its own.
var shells = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "ash": true, "fish": true}

// program returns the programs one simple command runs: normally one, more when it hands a line
// to a shell or to eval, none when it only assigns variables or opens a loop.
func program(words []string, depth int) ([]string, bool) {
	i := 0
	for i < len(words) {
		w := words[i]
		switch {
		case isAssignment(w):
			i++
			continue
		case keywords[w]:
			i++
			continue
		case headers[w]:
			return nil, true
		}
		break
	}
	if i >= len(words) {
		return nil, true
	}
	// A program name still holding $, a glob or a brace is decided when the line runs, not here:
	// `$x i` after `x=npm`, `np[m] i`, `{npm,i}`. It cannot be read, so it is not let through.
	// The basename decides: `$HOME/.cargo/bin/cargo` is cargo, whatever HOME is.
	raw := words[i]
	name := path.Base(raw)
	if name != "[" && strings.ContainsAny(name, "$*?[{") {
		return nil, false
	}
	rest := words[i+1:]
	switch {
	case name == "eval":
		return programs(strings.Join(rest, " "), depth+1)
	case shells[name]:
		return shellLine(raw, rest, depth)
	case name == "watch":
		// watch runs its arguments as one line through sh -c.
		j := skipOptions(rest, "-n -d -c")
		if j >= len(rest) {
			return []string{raw}, true
		}
		return programs(strings.Join(rest[j:], " "), depth+1)
	case name == "su" || name == "script" || name == "flock":
		// su -c LINE, script -c LINE and flock FILE -c LINE hand a line to a shell; flock FILE
		// CMD runs CMD.
		for j, a := range rest {
			if (a == "-c" || a == "--command") && j+1 < len(rest) {
				return programs(rest[j+1], depth+1)
			}
		}
		if name == "flock" {
			j := skipOptions(rest, "-w -E")
			if j+1 < len(rest) {
				return program(rest[j+1:], depth+1)
			}
		}
		return []string{raw}, true
	case name == "find":
		// find ... -exec CMD ... ; runs CMD for each match.
		var progs []string
		for j, a := range rest {
			if (a == "-exec" || a == "-execdir" || a == "-ok" || a == "-okdir") && j+1 < len(rest) {
				p, ok := program(rest[j+1:], depth+1)
				if !ok {
					return nil, false
				}
				progs = append(progs, p...)
			}
		}
		return append([]string{raw}, progs...), true
	case name == "env":
		// env -S LINE splits LINE into a command.
		for j, a := range rest {
			if (a == "-S" || a == "--split-string") && j+1 < len(rest) {
				return programs(rest[j+1], depth+1)
			}
			if strings.HasPrefix(a, "-S") && len(a) > 2 {
				return programs(a[2:], depth+1)
			}
		}
	}

	if opts, ok := wrappers[name]; ok {
		takes := strings.Fields(opts)
		j := 0
		if name == "time" && j < len(rest) && rest[j] == "-p" {
			j++
		}
		for j < len(rest) {
			a := rest[j]
			if a == "--" {
				j++
				break
			}
			if name == "env" && isAssignment(a) {
				j++
				continue
			}
			if !strings.HasPrefix(a, "-") || a == "-" {
				break
			}
			j++
			for _, o := range takes {
				if a == o {
					j++
					break
				}
			}
		}
		if name == "timeout" && j < len(rest) {
			j++ // the duration
		}
		if j >= len(rest) {
			return []string{raw}, true
		}
		return program(rest[j:], depth+1)
	}
	return []string{raw}, true
}

// shellLine reads what a shell runs: the line after -c (past any options and --), nothing for a
// script named by path, and an unreadable line for one that reads its commands from stdin, where
// boxer cannot see them (`bash <<< 'npm i'`, `echo 'npm i' | sh`).
func shellLine(name string, rest []string, depth int) ([]string, bool) {
	seenC, takes := false, 0
	for j := 0; j < len(rest); j++ {
		a := rest[j]
		switch {
		case takes > 0:
			takes-- // an option's argument: -o pipefail, -O extglob, --rcfile file
		case a == "--":
			// Options end. With -c the next word is the line; without it, a script by path.
			if j+1 < len(rest) {
				if seenC {
					return programs(rest[j+1], depth+1)
				}
				return []string{name}, true
			}
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--rcfile", "--init-file":
				takes++
			case "--version", "--help":
				return []string{name}, true
			}
		case strings.HasPrefix(a, "-") || strings.HasPrefix(a, "+"):
			if a == "-s" {
				return nil, false // reads its commands from stdin
			}
			for _, f := range a[1:] {
				switch f {
				case 'c':
					seenC = true
				case 'o', 'O':
					takes++
				}
			}
		default:
			if seenC {
				return programs(a, depth+1) // the first word after the options is the line
			}
			return []string{name}, true // a script by path: one program, its own name
		}
	}
	if seenC {
		return []string{name}, true
	}
	return nil, false // no -c and no script: the commands come from stdin
}

// skipOptions returns the index of the first word in rest that is not an option, counting the
// listed options' arguments as part of them.
func skipOptions(rest []string, takesArg string) int {
	takes := strings.Fields(takesArg)
	j := 0
	for j < len(rest) {
		a := rest[j]
		if a == "--" {
			return j + 1
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			return j
		}
		j++
		for _, o := range takes {
			if a == o {
				j++
				break
			}
		}
	}
	return j
}

// isAssignment is NAME=value, the prefix that sets a variable for one command.
func isAssignment(w string) bool {
	eq := strings.IndexByte(w, '=')
	if eq <= 0 {
		return false
	}
	for k, c := range w[:eq] {
		letter := c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if !letter && (k == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}
