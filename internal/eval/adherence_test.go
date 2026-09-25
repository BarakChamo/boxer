package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/config"
)

type fakeDriver struct{ cells map[string][]Cell }

func (f fakeDriver) Name() string                               { return "fake" }
func (f fakeDriver) Available(string) (bool, string)            { return true, "" }
func (f fakeDriver) Cells(tier string) []Cell                   { return f.cells[tier] }
func (f fakeDriver) Prepare(*Env, Cell) error                   { return nil }
func (f fakeDriver) Run(*Env, Cell, string) (Transcript, error) { return Transcript{}, nil }
func (f fakeDriver) Cleanup(*Env, Cell)                         {}

func TestAdherenceCells(t *testing.T) {
	// Kimi-shaped: tool only, t1 and t2 alike.
	kimi := fakeDriver{map[string][]Cell{"t2": {{Harness: "kimi", Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: true, Intercept: []string{"uname"}}}}}
	cells := AdherenceCells(kimi)
	if len(cells) != 6 || cells[2].Mode != "tool" || cells[2].Intercept != nil || cells[2].Image != multistepImage {
		t.Fatalf("kimi cells: %+v", cells)
	}
	// The server cell rides the compliant tool cell, with a node image to serve from.
	if cells[4].Scenario != "prep" || len(cells[4].Intercept) != 1 || cells[4].Intercept[0] != "*" {
		t.Fatalf("prep cell: %+v", cells[4])
	}
	if cells[5].Scenario != "server" || cells[5].Mode != "tool" || cells[5].Image != multistepImage {
		t.Fatalf("server cell: %+v", cells[5])
	}
	// The task cell intercepts everything on purpose, so a composed command line lands in the
	// guest exactly as the declared task would and the cell measures only which was chosen. A
	// narrower list lets a composed command escape to the host, where the tier's other oracles
	// report it as a sandbox escape — a false alarm on the one signal that must never be one.
	if cells[3].Scenario != "task" || len(cells[3].Intercept) != 1 || cells[3].Intercept[0] != "*" || cells[3].Shims {
		t.Fatalf("task cell: %+v", cells[3])
	}
	if cells[0].Name() != "kimi/tool/user/worktree/brief" || cells[1].Scenario != "recovery" || cells[2].Tier != "t2" {
		t.Fatalf("names: %s %s", cells[0].Name(), cells[1].Name())
	}
	// Codex-shaped: tool cell at t1 only, rewrite at t2, plus a noncompliant cell to ignore.
	codex := fakeDriver{map[string][]Cell{
		"t2": {{Harness: "codex", Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true}},
		"t1": {{Harness: "codex", Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: false}, {Harness: "codex", Mode: "tool", Entry: "user", Isolation: "worktree", Compliant: true}},
	}}
	cells = AdherenceCells(codex)
	if len(cells) != 6 || cells[0].Entry != "user" || !cells[0].Compliant || cells[2].Mode != "rewrite" || cells[2].Entry != "project" {
		t.Fatalf("codex cells: %+v", cells)
	}
	if AdherenceCells(fakeDriver{map[string][]Cell{"t2": {{Mode: "rewrite", Compliant: true}}}}) != nil {
		t.Fatal("a driver with no tool cell has no adherence cells")
	}
}

func TestVerdict(t *testing.T) {
	r := func(status string) Result { return Result{Status: status} }
	cases := map[string][]Result{
		"pass":             {r("pass"), r("pass")},
		"harness":          {r("fail"), r("fail")},
		"model adherence":  {r("fail"), r("pass"), r("skip")},
		"fail (one model)": {r("fail"), r("skip")},
		"not judged":       {r("skip")},
	}
	for want, rs := range cases {
		if got := Verdict(rs); got != want {
			t.Errorf("Verdict(%v) = %q, want %q", rs, got, want)
		}
	}
	rep := AdherenceReport([]Result{
		{Cell: Cell{Harness: "h", Mode: "tool", Entry: "e", Isolation: "i", Compliant: true, Scenario: "brief"}, Model: "a", Status: "fail", Denials: 2, CostUSD: 0.01, Findings: []Finding{{"deny", "2 denial(s)"}}},
		{Cell: Cell{Harness: "h", Mode: "tool", Entry: "e", Isolation: "i", Compliant: true, Scenario: "brief"}, Model: "b", Status: "pass"},
	})
	for _, want := range []string{"| h/tool/e/i/brief | fail · 2 · $0.0100 | pass · 0 · $0.0000 | model adherence |", "| a | 0 | 1 | 0 | $0.0100 |", "on `a`: deny: 2 denial(s)"} {
		if !strings.Contains(rep, want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
}

func TestTraceAllowedAndUnboxed(t *testing.T) {
	trace := strings.Join([]string{
		`claude-code <- {"hook_event_name":"SessionStart"}`,
		`claude-code -> {"hookSpecificOutput":{"additionalContext":"brief"}}`,
		`claude-code <- {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"cat package.json"}}`,
		`claude-code <- {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"npm install"}}`,
		`claude-code -> {"hookSpecificOutput":{"permissionDecision":"allow","updatedInput":{"command":"boxer run -c 'npm install'"}}}`,
		`claude-code <- {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"uname -a"}}`,
		`claude-code -> {"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"no"}}`,
		`claude-code <- {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"bash -c \"npm test\""}}`,
		`claude-code <- {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"boxer run -c 'npm test'"}}`,
	}, "\n") + "\n"
	path := filepath.Join(t.TempDir(), "trace.log")
	os.WriteFile(path, []byte(trace), 0o644)
	tr := readTrace(path)
	if tr.rewrites != 1 || tr.denies != 1 || len(tr.inputs) != 5 || len(tr.allowed) != 3 {
		t.Fatalf("rewrites %d denies %d inputs %v allowed %v", tr.rewrites, tr.denies, tr.inputs, tr.allowed)
	}
	got := unboxed(tr, Cell{}.intercept())
	if len(got) != 1 || got[0] != `bash -c "npm test"` {
		t.Fatalf("unboxed = %v", got)
	}
}

func TestParseGrokUseTool(t *testing.T) {
	s := `{"type":"tool_call","toolName":"search_tool","rawInput":{"query":"boxer"}}
{"type":"tool_call","toolName":"use_tool","rawInput":{"tool_input":{"command":"uname -a"},"tool_name":"boxer__boxer_run"}}
{"type":"text","data":"Linux"}
`
	tr := parseGrokStream(s)
	if !usedRunTool(tr) || tr.Answer != "Linux" || tr.Tools[1] != "use_tool:boxer__boxer_run" {
		t.Fatalf("%+v", tr)
	}
}

func TestPromptPerScenario(t *testing.T) {
	e := &Env{RunID: "1"}
	if !strings.Contains(e.Prompt(), "shell command") {
		t.Fatal("matrix prompt changed")
	}
	e.Scenario = "brief"
	if p := e.Prompt(); strings.Contains(p, "shell") || strings.Contains(p, "boxer run") || strings.Contains(p, "sandbox") || !strings.Contains(p, e.Command()) {
		t.Fatalf("brief prompt: %s", p)
	}
	e.Scenario = "multistep"
	if p := e.Prompt(); !strings.Contains(p, "npm") || strings.Contains(p, "boxer") {
		t.Fatalf("multistep prompt: %s", p)
	}
}

// The dialect's one-denial budget is for the mechanical matrix only: in the adherence tier the
// denial is what the cell measures, so Grok's brief cell must still fail.
func TestGrokBriefStillFailsInAdherence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.log")
	os.WriteFile(path, []byte(`grok -> {"hookSpecificOutput":{"permissionDecision":"deny"}}`+"\n"), 0o644)
	deny := func(scenario string) bool {
		c := Cell{Harness: "grok", Mode: "tool", Compliant: true, Tier: "t2", Scenario: scenario}
		for _, f := range Judge(&Env{Tier: "t2", Trace: path, Repo: t.TempDir()}, c, Transcript{Answer: "Linux"}) {
			if f.Check == "deny" {
				return true
			}
		}
		return false
	}
	if !deny("brief") {
		t.Error("a denial in the brief scenario must stay a finding for grok")
	}
	if deny("") {
		t.Error("the mechanical matrix budgets grok's one denial")
	}
}

// The server scenario's oracle: the page's text proves the agent reached this worktree's server,
// and with named URLs on, the Host it was reached by must be the name.
func TestJudgeServer(t *testing.T) {
	env := &Env{RunID: "4242"}
	if f := judgeServer(env, Transcript{Answer: "nothing useful"}); len(f) != 1 || f[0].Check != "answer" {
		t.Fatalf("no page text: %+v", f)
	}
	// A live model phrases it; the text is found in the transcript.
	named := judgeServer(env, Transcript{Answer: "The page says so", Raw: `"text":"4242@adherence.localhost:1355"`})
	port := judgeServer(env, Transcript{Answer: "4242@127.0.0.1:61234"})
	if ServerURLs() {
		if len(named) != 0 || len(port) != 1 || port[0].Check != "url" {
			t.Fatalf("with portless: named %+v, port %+v", named, port)
		}
	} else if len(named) != 0 || len(port) != 0 {
		t.Fatalf("without portless either address is right: named %+v, port %+v", named, port)
	}
}

// The server fixture is a boxer.toml written by string formatting, with a JavaScript one-liner
// quoted inside TOML inside Go. It once shipped a `ready` probe the image could not run, and every
// cell failed in prepare — measuring the fixture, not a harness. Parse what it writes, and check
// the probe uses a program the image has.
func TestServerFixtureIsValidConfig(t *testing.T) {
	dir := t.TempDir()
	body := serverToml(multistepImage, Cell{Isolation: "worktree", Mode: "tool"}, "4242")
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFiles(filepath.Join(dir, "boxer.toml"))
	if err != nil {
		t.Fatalf("%v\n%s", err, body)
	}
	if len(cfg.Start) != 1 || !strings.Contains(cfg.Start[0], "4242@") || !strings.Contains(cfg.Ready, "node -e") {
		t.Fatalf("start %q ready %q", cfg.Start, cfg.Ready)
	}
	if len(cfg.Network.Ports) != 1 || cfg.Network.Ports[0] != "auto:3000" {
		t.Fatalf("ports %v", cfg.Network.Ports)
	}
}

// The matrix's URLs mode must still write a valid configuration, and must actually withhold the
// port: a prompt that hands it over tests nothing about finding it.
func TestMatrixURLsMode(t *testing.T) {
	if !ServerURLs() {
		t.Skip("portless not installed")
	}
	t.Setenv("BOXER_EVAL_URLS", "1")
	for _, c := range []Cell{{Mode: "rewrite", Isolation: "worktree"}, {Inside: "claude", Mode: "inside"}} {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte(matrixTOML(c, 3000)), 0o644)
		cfg, err := config.LoadFiles(filepath.Join(dir, "boxer.toml"))
		if err != nil || !cfg.URLs.Enabled {
			t.Fatalf("%+v: %v enabled=%v", c, err, cfg.URLs.Enabled)
		}
	}
	if p := matrixPrompt(SDLCTask{Prompt: "x"}, "61234", "hook"); strings.Contains(p, "61234") {
		t.Fatalf("the host port leaked into the prompt:\n%s", p)
	}
}

// The prep fixture, like the server one, is TOML built by string formatting: parse it.
func TestPrepFixtureIsValidConfig(t *testing.T) {
	env := &Env{Repo: filepath.Join(t.TempDir(), "repo")}
	if err := env.mkrepo(Cell{Isolation: "worktree", Mode: "rewrite", Intercept: []string{"*"}, Scenario: "prep"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFiles(filepath.Join(env.Repo, "boxer.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Prep.Commands) != 1 || !strings.Contains(cfg.Prep.Commands[0], `"$BOXER_TARGET_OS"`) {
		t.Fatalf("prep: %q", cfg.Prep.Commands)
	}
	if f := judgePrep(env, Transcript{Answer: "linux/musl"}); len(f) != 1 || f[0].Check != "prep" {
		t.Fatalf("no prep-runs.log must be reported as prep not running: %+v", f)
	}
}
