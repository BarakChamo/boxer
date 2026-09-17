package decide

import "testing"

var defaults = Input{
	Mode:        "rewrite",
	Intercept:   []string{"npm", "bun", "node", "python", "pytest", "cargo", "go", "make"},
	Passthrough: []string{"git", "gh", "ssh", "boxer"},
}

func with(cmd string, f func(*Input)) Input {
	in := defaults
	in.Command = cmd
	if f != nil {
		f(&in)
	}
	return in
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want Decision
	}{
		{"passthrough", with("git status", nil), Decision{Action: Allow}},
		{"unlisted stays on host", with("ls -la", nil), Decision{Action: Allow}},
		{"intercepted", with("npm test", nil), Decision{Action: Rewrite, Command: "boxer run -c 'npm test'"}},
		{"path prefix", with("./node_modules/.bin/node x.js", nil), Decision{Action: Rewrite, Command: "boxer run -c './node_modules/.bin/node x.js'"}},
		{"env prefix", with("CI=1 bun run check", nil), Decision{Action: Rewrite, Command: "boxer run -c 'CI=1 bun run check'"}},
		{"compound wrapped whole", with("cd apps/api && bun test | tail", nil), Decision{Action: Rewrite, Command: "boxer run -c 'cd apps/api && bun test | tail'"}},
		{"compound all passthrough", with("git add -A && git commit -m x", nil), Decision{Action: Allow}},
		{"already boxed", with("boxer run -c 'npm test'", nil), Decision{Action: Allow}},
		{"quotes escaped", with("node -e 'console.log(1)'", nil), Decision{Action: Rewrite, Command: `boxer run -c 'node -e '\''console.log(1)'\'''`}},
		{"mode off", with("npm test", func(i *Input) { i.Mode = "off" }), Decision{Action: Allow}},
		{"mode tool blocks with fix", with("npm test", func(i *Input) { i.Mode = "tool" }), Decision{
			Action: Block,
			Reason: "this repository runs commands in a sandbox; use the boxer_run tool or `boxer run`",
			Fix:    "boxer run -c 'npm test'",
		}},
		{"star intercepts unlisted", with("ls", func(i *Input) { i.Intercept = []string{"*"} }), Decision{Action: Rewrite, Command: "boxer run -c 'ls'"}},
		{"star respects passthrough", with("git log", func(i *Input) { i.Intercept = []string{"*"} }), Decision{Action: Allow}},
		{"empty", with("   ", nil), Decision{Action: Allow}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.in)
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
