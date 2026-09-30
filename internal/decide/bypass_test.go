package decide

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every line here ran an intercepted program on the host before the reader: each is a way an
// agent's command reached `npm` without the hook seeing it.
var escapes = []string{
	"true & npm i",
	"sleep 0 & npm test",
	"boxer version && npm i",
	"sh -c 'npm i'",
	`bash -lc "npm run build"`,
	"bash -c 'cd x; npm test'",
	"env npm i",
	"env -u HOME npm i",
	"env FOO=1 npm test",
	"sudo npm i -g x",
	"sudo -u root npm i",
	"nohup npm run dev",
	"nice -n 5 npm test",
	"time npm test",
	"time -p npm test",
	"timeout 60 npm test",
	"timeout -s KILL 60 npm test",
	"command npm test",
	"exec npm test",
	"xargs npm i < pkgs",
	"echo a | xargs -I{} npm i {}",
	"if npm test; then echo ok; fi",
	"until npm test; do sleep 1; done",
	"while ! npm test; do sleep 1; done",
	"for f in a b; do npm test; done",
	"echo $(npm bin)",
	"echo `npm bin`",
	`echo "$(npm bin)"`,
	"diff <(npm ls) <(true)",
	"eval npm test",
	"eval 'npm test'",
	"x=$(npm -v) && echo $x",
	"FOO=bar; npm test",
	"npm test 2>&1 | tail",
	"npm test >out.log 2>&1",
	"{ npm test; }",
	"! npm test",
	"git status && npm test",
	"cd sub\nnpm test",
	`sh -c "sh -c 'npm i'"`,
}

// And these stay on the host: nothing intercepted runs, whatever the line mentions.
var stays = []string{
	"git commit -m 'fix npm test && more'",
	`echo "npm && node"`,
	"grep npm package.json",
	"ls | grep node",
	"cat package.json # npm test",
	"echo $((1 + 2))",
	"boxer run -c 'cd x && npm test'",
	"boxer run -- npm test",
	"git log --grep='npm i'",
	"echo npm > notes.txt",
	"for pkg in npm node; do echo $pkg; done",
	"bash script.sh",
	"sudo git push",
	"env",
	"FOO=1 BAR=2",
	`printf '%s\n' "$(git rev-parse HEAD)"`,
}

func TestEscapesAreSandboxed(t *testing.T) {
	for _, c := range escapes {
		if d := Decide(with(c, nil)); d.Action != Rewrite {
			t.Errorf("%q runs npm on the host: %+v", c, d)
		}
	}
}

func TestHostCommandsStayOnTheHost(t *testing.T) {
	for _, c := range stays {
		if d := Decide(with(c, nil)); d.Action != Allow {
			t.Errorf("%q runs nothing intercepted, but was %+v", c, d)
		}
	}
}

// The oracle is a real shell. Each line runs in bash with PATH holding only stubs that record
// their own name, so "did an intercepted program run" is answered by bash rather than by a
// second reading of the same rules. Decide must sandbox exactly the lines where one did.
func TestDecideAgreesWithARealShell(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	work := filepath.Join(dir, "work")
	for _, d := range []string{bin, filepath.Join(work, "sub"), filepath.Join(work, "x")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(dir, "ran")
	record := "#!/bin/sh\necho \"$(basename \"$0\")\" >> " + log + "\nexit 0\n"
	for _, p := range defaults.Intercept {
		os.WriteFile(filepath.Join(bin, p), []byte(record), 0o755)
	}
	// Passthrough and host programs: stubs that run nothing, and sudo/doas that run their argument.
	for _, p := range []string{"git", "boxer", "gh", "ssh"} {
		os.WriteFile(filepath.Join(bin, p), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nwhile [ \"${1#-}\" != \"$1\" ]; do [ \"$1\" = -u ] && shift; shift; done\nexec \"$@\"\n"), 0o755)
	// Real tools, so wrappers resolve their program through PATH as they would for an agent.
	for _, p := range []string{"sh", "bash", "env", "nohup", "nice", "timeout", "xargs", "echo", "true", "sleep", "cat", "grep", "ls", "tail", "diff", "printf", "basename", "dirname"} {
		if real, err := exec.LookPath(p); err == nil {
			os.Symlink(real, filepath.Join(bin, p))
		}
	}
	os.WriteFile(filepath.Join(work, "package.json"), []byte("npm"), 0o644)
	os.WriteFile(filepath.Join(work, "pkgs"), []byte("a\n"), 0o644)
	os.WriteFile(filepath.Join(work, "script.sh"), []byte("echo hi\n"), 0o644)

	checked := 0
	lines := append(append([]string{}, escapes...), stays...)
	for c := range edges {
		lines = append(lines, c)
	}
	for _, c := range lines {
		if _, err := exec.LookPath("timeout"); err != nil && strings.Contains(c, "timeout") {
			continue // no timeout on this host; the table tests still cover the line
		}
		os.Remove(log)
		cmd := exec.Command(bash, "-c", c)
		cmd.Dir = work
		cmd.Env = []string{"PATH=" + bin, "HOME=" + dir}
		cmd.Stdin = strings.NewReader("a\n")
		done := make(chan struct{})
		go func() { _ = cmd.Run(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("%q did not finish", c)
		}
		ran, _ := os.ReadFile(log)
		ranIntercepted := len(strings.TrimSpace(string(ran))) > 0
		sandboxed := Decide(with(c, nil)).Action != Allow
		// A line that runs npm only on some condition — a case arm, a function body — did not run it
		// here, and is still sandboxed: that is the safe direction, and it is named, not tolerated.
		if !ranIntercepted && sandboxed && mayRun[c] {
			checked++
			continue
		}
		if ranIntercepted != sandboxed {
			t.Errorf("%q: bash ran %q; Decide sandboxed = %v", c, strings.TrimSpace(string(ran)), sandboxed)
		}
		checked++
	}
	if checked < len(lines)-4 {
		t.Fatalf("only %d lines checked", checked)
	}
}

// Nothing a harness sends may panic the hook, and an unreadable line is never let through.
func FuzzDecide(f *testing.F) {
	for _, s := range append(append([]string{}, escapes...), stays...) {
		f.Add(s)
	}
	f.Add(`echo "unterminated`)
	f.Add("$(((((")
	f.Fuzz(func(t *testing.T, s string) {
		d := Decide(with(s, nil))
		if d.Action == Rewrite && !strings.HasPrefix(d.Command, "boxer run -c ") {
			t.Fatalf("rewrote %q to %q", s, d.Command)
		}
	})
}

// A line too deep, or unterminated, is sandboxed rather than let through.
func TestUnreadableLinesAreSandboxed(t *testing.T) {
	deep := "npm i"
	for i := 0; i < maxDepth+2; i++ {
		deep = "sh -c " + quoteSingle(deep)
	}
	nested := "npm i"
	for i := 0; i < maxDepth+2; i++ {
		nested = "echo $(" + nested + ")"
	}
	for _, c := range []string{deep, nested, `echo "npm i`, "echo $(npm i", "echo `npm i", "echo 'npm i",
		"diff <(npm ls", `echo "$(npm i"`, "echo \"`npm i", `echo $(echo 'x)`, `echo $(echo "x)`} {
		if Decide(with(c, nil)).Action != Rewrite {
			t.Errorf("%q must be sandboxed", c)
		}
	}
	if Decide(with(`echo "unterminated`, func(i *Input) { i.Intercept = nil })).Action != Allow {
		t.Error("with nothing intercepted, nothing is sandboxed")
	}
}

func quoteSingle(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// The quoting, nesting and redirection shapes the tables above do not reach, each checked
// against a real shell's reading of it.
var edges = map[string]bool{
	`echo "a $(echo "b $(npm -v)") c"`:  true,  // nested substitution with quotes inside
	"echo \"`npm -v`\"":                 true,  // backticks inside double quotes
	`echo "$(echo ')' ; npm -v)"`:       true,  // a quoted paren inside a substitution
	`echo $(echo "(" && npm -v)`:        true,  // a quoted paren, unquoted substitution
	`echo $( (npm -v) )`:                true,  // a subshell inside a substitution
	`echo "$((1+2))" && echo "\"npm\""`: false, // arithmetic and an escaped quote
	"echo $((2*3))":                     false,
	`echo \$\(npm i\)`:                  false, // escaped, so not a substitution
	"npm test >&2":                      true,
	"echo x 2>/dev/null; npm test":      true,
	"echo x &>/dev/null && npm test":    true,
	"echo x 2> err.log":                 false,
	"echo a \\\n&& npm test":            true, // a continued line
	"zsh -c":                            false,
	"bash -x script.sh":                 false,
	"env -u HOME -- npm test":           true,
	"nice -5 npm test":                  true,
	"time -p npm test":                  true,
	"1x=2 npm test":                     false, // not an assignment: bash runs 1x=2 as the program
	"timeout":                           false,
	"sudo":                              false,
	"f() { npm test; }":                 true,
	"case $x in a) npm test;; esac":     true,
	"echo \"$(true)\" '$(npm i)'":       false, // single quotes stop substitution
}

// mayRun are lines that run an intercepted program only on some condition the oracle's shell does
// not meet.
// A login shell (`bash -lc`) reads /etc/profile first, and on macOS that runs path_helper, which
// rebuilds PATH without the oracle's stubs: there it runs no npm the oracle can see, though for an
// agent it runs npm.
var mayRun = map[string]bool{"case $x in a) npm test;; esac": true, "f() { npm test; }": true, `bash -lc "npm run build"`: true}

func TestReaderEdgeCases(t *testing.T) {
	for c, want := range edges {
		if got := Decide(with(c, nil)).Action != Allow; got != want {
			t.Errorf("%q: sandboxed = %v, want %v", c, got, want)
		}
	}
	if isDigits("") || !isDigits("12") || isDigits("1a") {
		t.Error("isDigits")
	}
}
