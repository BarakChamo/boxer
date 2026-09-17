package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if len(cells) != 3 || cells[2].Mode != "tool" || cells[2].Intercept != nil || cells[2].Image != multistepImage {
		t.Fatalf("kimi cells: %+v", cells)
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
	if len(cells) != 3 || cells[0].Entry != "user" || !cells[0].Compliant || cells[2].Mode != "rewrite" || cells[2].Entry != "project" {
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
