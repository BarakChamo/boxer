package decide

import (
	"path"
	"slices"
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
	bad := false
	var pending []heredoc // here-documents whose bodies start at the next newline
	flushWord := func() {
		if !inWord {
			return
		}
		w := word.String()
		// Arithmetic evaluates a variable's text, and an array index in it runs any substitution
		// it holds, quoted or not: `x='a[$(npm i)]'; echo $((x))`, `let 'a[$(npm i)]=1'`. So a
		// word holding an index with a substitution is read for it wherever it appears.
		if strings.Contains(w, "[$(") || strings.Contains(w, "[`") {
			// One that does not close runs nothing: arithmetic cannot parse it either. A regex
			// such as grep '[$(]' is the common case.
			if _, inner, ok := lex(w); ok {
				subs = append(subs, inner...)
			}
		}
		if redirect {
			redirect = false
		} else {
			words = append(words, w)
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
		case c == '$' && i+1 < len(r) && r[i+1] == '\'':
			// $'...' honours backslash escapes, so `\'` does not end it. Its text is kept with the
			// `$`: a program named this way is decided when the line runs, so it is unreadable.
			j := i + 2
			for j < len(r) && r[j] != '\'' {
				if r[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(r) {
				return nil, nil, false
			}
			word.WriteString("$" + string(r[i+2:j]))
			inWord = true
			i = j
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
			// The text it substitutes is not known here: a word it is part of, the program
			// name above all, is unreadable. `$(echo npm) i` left an empty word, read as no
			// program at all.
			word.WriteString("$")
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
				word.WriteString("$")
				inWord = true
				i = j + 1
				continue
			}
			j := matchParen(r, i+1)
			if j < 0 {
				return nil, nil, false
			}
			subs = append(subs, string(r[i+2:j]))
			word.WriteString("$")
			inWord = true
			i = j
		case (c == '<' || c == '>') && i+1 < len(r) && r[i+1] == '(':
			j := matchParen(r, i+1)
			if j < 0 {
				return nil, nil, false
			}
			subs = append(subs, string(r[i+2:j]))
			// A file name whose content is a command's output: `source <(...)` runs it.
			word.WriteString("$")
			inWord = true
			i = j
		case c == '<' && i+1 < len(r) && r[i+1] == '<' && (i+2 >= len(r) || r[i+2] != '<'):
			// A here-document. Its body is not shell code, so it is skipped rather than read as
			// commands, quotes and comments; with an unquoted delimiter its substitutions run.
			flushWord()
			h, end, ok := heredocAt(r, i)
			if !ok {
				return nil, nil, false
			}
			pending = append(pending, h)
			i = end
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
		case c == '\n' && len(pending) > 0:
			flushCmd()
			for _, h := range pending {
				body, end, ok := heredocBody(r, i+1, h)
				if !ok {
					return nil, nil, false
				}
				if !h.quoted {
					inner, ok := bodySubs([]rune(body))
					if !ok {
						return nil, nil, false
					}
					subs = append(subs, inner...)
				}
				i = end
			}
			pending = nil
		case c == '(' && inWord && !strings.HasSuffix(word.String(), "=") && (i+1 >= len(r) || r[i+1] != ')'):
			// Word text then a parenthesis: zsh's glob qualifiers (`*(e:'npm i':)`) and
			// parameter flags (`${(e)x}`) run code from inside the word. A function definition
			// `f()` and an array `a=(...)` are the forms that are not.
			bad = true
			flushCmd()
		case c == ';' || c == '&' || c == '|' || c == '\n' || c == '(' || c == ')':
			flushCmd()
		case c == ' ' || c == '\t':
			// Not \r: every shell keeps it in the word, so `a\r#$(npm i)` has no comment in it.
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
	if bad {
		return nil, nil, false
	}
	return cmds, subs, true
}

// heredoc is a pending here-document: its delimiter, whether `<<-` strips leading tabs, and
// whether a quoted delimiter turned expansion off in its body.
type heredoc struct {
	delim        string
	dash, quoted bool
}

// heredocAt reads `<<[-]DELIM` starting at the first '<'. It returns the index of the delimiter's
// last character.
func heredocAt(r []rune, i int) (heredoc, int, bool) {
	var h heredoc
	i += 2
	if i < len(r) && r[i] == '-' {
		h.dash = true
		i++
	}
	for i < len(r) && (r[i] == ' ' || r[i] == '\t') {
		i++
	}
	var d strings.Builder
	start := i
	for i < len(r) && !strings.ContainsRune(" \t\n;|&<>()", r[i]) {
		switch r[i] {
		case '\'', '"':
			j := indexRune(r, i+1, r[i])
			if j < 0 {
				return h, 0, false
			}
			d.WriteString(string(r[i+1 : j]))
			h.quoted = true
			i = j
		case '\\':
			h.quoted = true
			if i+1 < len(r) {
				i++
				d.WriteRune(r[i])
			}
		default:
			d.WriteRune(r[i])
		}
		i++
	}
	if i == start {
		return h, 0, false
	}
	h.delim = d.String()
	return h, i - 1, true
}

// heredocBody returns the body that starts at from and ends at a line that is the delimiter, and
// the index of that line's last character. A body never closed is unreadable: the shell takes
// the rest of the input as text, and boxer cannot say what that hides.
func heredocBody(r []rune, from int, h heredoc) (string, int, bool) {
	for i := from; i <= len(r); {
		j := indexRune(r, i, '\n')
		if j < 0 {
			j = len(r)
		}
		line := string(r[i:j])
		if h.dash {
			line = strings.TrimLeft(line, "\t")
		}
		if line == h.delim {
			end := j
			if end >= len(r) {
				end = len(r) - 1
			}
			return string(r[from:i]), end, true
		}
		if j >= len(r) {
			break
		}
		i = j + 1
	}
	return "", 0, false
}

// bodySubs returns the command substitutions an unquoted here-document's body runs. Quotes in a
// body are text, so only backslashes, backticks and $( are read.
func bodySubs(r []rune) (subs []string, ok bool) {
	for i := 0; i < len(r); i++ {
		switch {
		case r[i] == '\\':
			i++
		case r[i] == '`':
			j := indexRune(r, i+1, '`')
			if j < 0 {
				return nil, false
			}
			subs = append(subs, string(r[i+1:j]))
			i = j
		case r[i] == '$' && i+1 < len(r) && r[i+1] == '(':
			j := matchParen(r, i+1)
			if j < 0 {
				return nil, false
			}
			if i+2 < len(r) && r[i+2] == '(' {
				_, inner, ok := lex(string(r[i+3 : j]))
				if !ok {
					return nil, false
				}
				subs = append(subs, inner...)
			} else {
				subs = append(subs, string(r[i+2:j]))
			}
			i = j
		}
	}
	return subs, true
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
	var pending []heredoc
	for i := open; i < len(r); i++ {
		// A here-document's body is text: an apostrophe in `$(cat <<'EOF' ... don't ... EOF)`, the
		// usual way an agent writes a commit message, is not a quote.
		if r[i] == '<' && i+1 < len(r) && r[i+1] == '<' && (i+2 >= len(r) || r[i+2] != '<') {
			h, end, ok := heredocAt(r, i)
			if !ok {
				return -1
			}
			pending = append(pending, h)
			i = end
			continue
		}
		if r[i] == '\n' && len(pending) > 0 {
			for _, h := range pending {
				_, end, ok := heredocBody(r, i+1, h)
				if !ok {
					return -1
				}
				i = end
			}
			pending = nil
			continue
		}
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

// wrapper describes a program that runs the program given in its arguments: the options that take
// a separate value, the options that take none, and how many positional words (a duration, a CPU
// mask) come before the program. An option that is in neither list makes the line unreadable: it
// may take a value, and taking that value for the program let `sudo --user root npm i` through.
type wrapper struct {
	takes, flags string
	positional   int
}

var wrappers = map[string]wrapper{
	"env":        {"-u --unset -C --chdir -S --split-string -P", "-i --ignore-environment -0 --null -v --debug", 0},
	"sudo":       {"-u --user -g --group -C --close-from -D --chdir -h --host -p --prompt -r --role -t --type -U --other-user -T --command-timeout -R --chroot", "-A --askpass -b --background -B --bell -E --preserve-env -H --set-home -i --login -k --reset-timestamp -K --remove-timestamp -l --list -n --non-interactive -N --no-update -P --preserve-groups -S --stdin -s --shell -v --validate", 0},
	"doas":       {"-u -C", "-n -s -L", 0},
	"nohup":      {"", "", 0},
	"nice":       {"-n --adjustment", "", 0},
	"ionice":     {"-c --class -n --classdata -p --pid -P --pgid -u --uid", "-t --ignore", 0},
	"time":       {"-f --format -o --output", "-p -a --append -v --verbose --portability -l -h -q", 0},
	"timeout":    {"-s --signal -k --kill-after", "--foreground --preserve-status -v --verbose -f -p", 1},
	"command":    {"", "-p -v -V", 0},
	"builtin":    {"", "", 0},
	"exec":       {"-a", "-c -l", 0},
	"stdbuf":     {"-i --input -o --output -e --error", "", 0},
	"xargs":      {"-I -J -R -S -n --max-args -P --max-procs -L --max-lines -d --delimiter -E -s --max-chars -a --arg-file", "-0 --null -r --no-run-if-empty -t --verbose -p --interactive -x --exit -o --open-tty -i -l -e", 0},
	"caffeinate": {"-w -t", "-d -i -m -s -u", 0},
	"chronic":    {"", "-e -v", 0},
	"unbuffer":   {"", "-p", 0},
	"setsid":     {"", "-c --ctty -f --fork -w --wait", 0},
	"strace":     {"-o -e -p -s -a -b -I -O -P -S -u -E -X", "-f -ff -c -C -d -D -F -h -i -k -q -qq -r -t -tt -ttt -T -v -V -w -x -xx -y -yy -z -Z", 0},
	"arch":       {"-d -e", "-arm64 -arm64e -x86_64 -x86_64h -i386 -32 -64 -c", 0},
	"taskset":    {"", "-a --all-tasks -c --cpu-list", 1},
	"chrt":       {"-T --sched-runtime -P --sched-period -D --sched-deadline", "-a --all-tasks -b --batch -d --deadline -f --fifo -i --idle -o --other -r --rr -R --reset-on-fork -v --verbose -m --max", 1},
	"busybox":    {"", "", 0},
	// zsh precommand modifiers and its loop shorthand: `repeat 3 npm i` runs npm three times.
	"noglob":    {"", "", 0},
	"nocorrect": {"", "", 0},
	"repeat":    {"", "", 1},
}

func init() {
	// Homebrew installs the GNU tools under a g prefix, beside the BSD ones.
	for _, n := range []string{"env", "nice", "nohup", "stdbuf", "timeout", "xargs", "time"} {
		wrappers["g"+n] = wrappers[n]
	}
}

// skipWrapper returns the index of the program in rest, a wrapper's arguments, or ok false when
// an option is one the wrapper is not known to take.
func skipWrapper(name string, rest []string) (int, bool) {
	wr := wrappers[name]
	takes, flags := strings.Fields(wr.takes), strings.Fields(wr.flags)
	j := 0
	for j < len(rest) {
		a := rest[j]
		if a == "--" {
			j++
			break
		}
		if (name == "env" || name == "sudo") && isAssignment(a) {
			j++
			continue
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		j++
		if opt, _, found := strings.Cut(a, "="); found && strings.HasPrefix(a, "--") {
			if !slices.Contains(takes, opt) && !slices.Contains(flags, opt) {
				return 0, false
			}
			continue
		}
		if slices.Contains(flags, a) || name == "nice" && isDigits(strings.TrimPrefix(a, "-")) {
			continue
		}
		if slices.Contains(takes, a) {
			j++
			continue
		}
		if strings.HasPrefix(a, "--") || len(a) < 3 {
			return 0, false
		}
		// GNU xargs's -i, -l and -e take an optional value joined to them: -i{}, -l1.
		if strings.HasSuffix(name, "xargs") && strings.ContainsRune("ile", rune(a[1])) {
			continue
		}
		// Short options run together: -Eu root, -n10, -iv.
		for k := 1; k < len(a); k++ {
			o := "-" + string(a[k])
			if slices.Contains(takes, o) {
				if k == len(a)-1 {
					j++ // its value is the next word
				}
				break
			}
			if !slices.Contains(flags, o) {
				return 0, false
			}
		}
	}
	return j + wr.positional, true
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
			// A shell started by this command first runs the file these name.
			if n, _, _ := strings.Cut(w, "="); n == "BASH_ENV" || n == "ENV" {
				return nil, false
			}
			i++
			continue
		case w == "function" || w == "coproc" && i+2 < len(words) && words[i+2] == "{":
			i += 2 // the name being defined is not a program
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
	raw := strings.TrimPrefix(words[i], "=") // zsh: `=npm` is npm's path
	name := path.Base(raw)
	if name != "[" && strings.ContainsAny(name, "$*?[{") {
		return nil, false
	}
	rest := words[i+1:]
	switch {
	case name == "eval":
		return programs(strings.Join(rest, " "), depth+1)
	case name == "trap":
		// trap LINE SIGNAL... runs LINE later, in this shell.
		j := skipOptions(rest, "")
		if j+1 < len(rest) {
			return programs(rest[j], depth+1)
		}
		return []string{raw}, true
	case name == "." || name == "source":
		// Sourcing runs a file's text in this shell. A file boxer can name stays one program,
		// like a script run by path; one made by a substitution is unreadable.
		for _, a := range rest {
			if strings.Contains(a, "$") {
				return nil, false
			}
		}
		return []string{raw}, true
	case name == "git":
		// A `!` alias defined on the command line runs a shell command, and git is passthrough.
		for j, a := range rest {
			if strings.HasPrefix(a, "--config-env") || a == "-c" && j+1 < len(rest) && strings.HasPrefix(rest[j+1], "alias.") && strings.Contains(rest[j+1], "=!") {
				return nil, false
			}
		}
		return []string{raw}, true
	case name == "zpty":
		// zpty [-b|-d|...] NAME CMD... runs CMD on a pseudo-terminal.
		j := skipOptions(rest, "")
		if j+1 < len(rest) {
			return program(rest[j+1:], depth+1)
		}
		return []string{raw}, true
	case name == "alias" || name == "shopt" && slices.Contains(rest, "expand_aliases"):
		// An alias renames a program for the lines after it, which this reader cannot follow.
		return nil, false
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
		if name == "script" {
			// BSD script: script [-adeFkqr] [-t time] FILE COMMAND...
			j := skipOptions(rest, "-t -T")
			if j+1 < len(rest) {
				return program(rest[j+1:], depth+1)
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
			line := ""
			if (a == "-S" || a == "--split-string") && j+1 < len(rest) {
				line = rest[j+1]
			} else if strings.HasPrefix(a, "-S") && len(a) > 2 {
				line = a[2:]
			} else {
				continue
			}
			// env -S has escapes of its own (`\_` is a space) and expands ${NAME}.
			if strings.ContainsAny(line, "\\$") {
				return nil, false
			}
			return programs(line, depth+1)
		}
	}

	if _, ok := wrappers[name]; ok {
		j, ok := skipWrapper(name, rest)
		if !ok {
			return nil, false
		}
		if j >= len(rest) {
			return []string{raw}, true
		}
		inner, ok := program(rest[j:], depth+1)
		// xargs appends what it reads to the command it was given, so a wrapper or a shell with
		// nothing to run gets its program from stdin: `echo npm | xargs env`.
		if ok && strings.HasSuffix(name, "xargs") && len(inner) > 0 {
			last := path.Base(inner[len(inner)-1])
			if _, w := wrappers[last]; w || shells[last] || last == "eval" {
				return nil, false
			}
		}
		return inner, ok
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
			if strings.Contains(a, "$") {
				return nil, false // a script whose name is decided when the line runs: `bash <(...)`
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
