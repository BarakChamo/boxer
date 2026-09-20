package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The point of the matrix is coverage of integration levels, so a row that quietly duplicates
// another level's coverage, or a level that disappears, is the failure worth catching.
func TestTheMatrixCoversEveryIntegrationLevel(t *testing.T) {
	// Command shims and shell shims are the same PATH mechanism with different failure modes, so
	// they are scored as separate levels rather than one standing in for the other.
	want := []string{"rewrite", "tool", "shims", "bash-shim", "shell", "inside", "orchestrator"}
	// Two cells of the same configuration run at the same time, so a configuration that handed
	// out one shared driver instance would race; the factory has to make a new one each call.
	if c := MatrixConfigs("t2")[0]; c.Driver() == nil {
		t.Fatal("a configuration's driver factory returned nothing")
	}
	seen := map[string]bool{}
	names := map[string]bool{}
	for _, c := range MatrixConfigs("t2") {
		if c.Driver == nil || c.Signature == nil {
			t.Fatalf("%s: a configuration needs both a driver and a signature", c.Name)
		}
		if names[c.Name] {
			t.Fatalf("%s: two configurations share a name, so the report cannot tell them apart", c.Name)
		}
		names[c.Name] = true
		if c.Cell.Tier != "t2" {
			t.Fatalf("%s: every matrix cell runs live; got tier %q", c.Name, c.Cell.Tier)
		}
		seen[c.Level] = true
	}
	// One witness per level is not coverage: a level carried by a single harness tells you that
	// harness works, not that the level does. Every level that more than one harness can reach
	// must be exercised by more than one.
	perLevel := map[string]int{}
	for _, c := range MatrixConfigs("t2") {
		perLevel[c.Level]++
	}
	for _, l := range want {
		if !seen[l] {
			t.Errorf("no configuration exercises the %q level", l)
		}
		// Shell substitution has one witness and it is not for want of looking: no harness CLI here
		// exposes a shell path (only the OpenHands SDK does), and herdr's pane shell — the other
		// real seam — is rejected by its own agent launcher once the wrapper is in it.
		if l != "shell" && perLevel[l] < 2 {
			t.Errorf("the %q level has only %d configuration; a level needs more than one witness", l, perLevel[l])
		}
	}
}

// Whichever configurations run, the workload has to be identical: the level is the variable.
func TestEveryConfigurationDevelopsTheSameApplication(t *testing.T) {
	for _, c := range MatrixConfigs("t2") {
		toml := matrixTOML(c.Cell, guestPort)
		for _, want := range []string{"create-next-app", "next dev -p 3000", `ports = ["auto:3000"]`} {
			if !strings.Contains(toml, want) {
				t.Errorf("%s: the configuration does not carry %q", c.Name, want)
			}
		}
	}
}

// The signatures are the tier's whole claim, so they are the part most worth testing directly: a
// signature that passes on an empty trace proves nothing at all.
func TestASignatureFailsWhenItsLevelDidNotCarryTheWork(t *testing.T) {
	linux, mac := "linux", "darwin"
	runTool := Transcript{Tools: []string{"boxer_run"}}
	layered := t.TempDir()
	if err := os.MkdirAll(filepath.Join(layered, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layered, ".claude", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		sig  func(evidence) (string, error)
		ev   evidence
		ok   bool
	}{
		{"rewrite with a rewrite", sigRewrite, evidence{t: trace{rewrites: 1}, guest: linux}, true},
		{"rewrite where the agent typed it", sigRewrite, evidence{t: trace{inputs: []string{"cd app && boxer run npm i"}}, guest: linux}, true},
		{"rewrite proved only by the transcript", sigRewrite, evidence{tr: Transcript{Raw: "$ boxer run node -e x"}, guest: linux}, true},
		{"rewrite with no evidence at all", sigRewrite, evidence{t: trace{denies: 2}, guest: linux}, false},
		{"rewrite that never left the host", sigRewrite, evidence{t: trace{rewrites: 1}, guest: mac}, false},
		// A compliant agent in tool mode is never denied anything: it just uses the run tool. A
		// signature that insisted on a denial would fail the level for working properly.
		{"tool with the run tool and no denial", sigTool, evidence{tr: runTool, guest: linux}, true},
		{"tool with a denial", sigTool, evidence{t: trace{denies: 1}, guest: linux}, true},
		// Several harnesses never name the MCP tools they called; boxer's own trace does.
		{"tool proved only by the trace", sigTool, evidence{t: trace{raw: `{"name":"boxer_run"}`}, guest: linux}, true},
		{"tool that did neither", sigTool, evidence{guest: linux}, false},
		{"shims with no rewrites", sigShims, evidence{guest: linux}, true},
		{"shims that were really hooks", sigShims, evidence{t: trace{rewrites: 3}, guest: linux}, false},
		{"inside with a silent hook", sigInside, evidence{guest: linux}, true},
		{"inside with a hook firing", sigInside, evidence{t: trace{events: []string{"PreToolUse"}}, guest: linux}, false},
		{"no platform recorded at all", sigShims, evidence{}, false},
		// An orchestrator's launcher passes no trace through, so the worktree it cut is the
		// evidence: boxer's layer in it, and the work having run in the guest.
		{"orchestrator with the layer in its worktree", sigOrchestrator, evidence{guest: linux, dir: layered}, true},
		{"orchestrator with no layer anywhere", sigOrchestrator, evidence{guest: linux, dir: t.TempDir()}, false},
		{"orchestrator that ran on the host", sigOrchestrator, evidence{guest: mac, dir: layered}, false},
	}
	for _, c := range cases {
		how, err := c.sig(c.ev)
		if (err == nil) != c.ok {
			t.Errorf("%s: wanted ok=%v, got %v", c.name, c.ok, err)
		}
		if err == nil && how == "" {
			t.Errorf("%s: passed without saying how the level carried the work", c.name)
		}
	}
}

func TestTheReportScoresEachClaimRatherThanTheCell(t *testing.T) {
	// A cell that rendered the page and left the change behind but could not show which layer
	// carried the work is not the same result as one whose sandbox never started, and the report
	// has to be able to say so.
	whole := checks{
		{Name: "the sandbox came up", Weight: 1, Passed: true},
		{Name: "the page renders", Weight: 2, Passed: true},
		{Name: "the level carried it", Weight: 3, Passed: true},
	}
	partial := checks{
		{Name: "the sandbox came up", Weight: 1, Passed: true},
		{Name: "the page renders", Weight: 2, Passed: true},
		{Name: "the level carried it", Weight: 3, Passed: false, Detail: "nothing was denied"},
	}
	if p := whole.percent(); p != 100 {
		t.Errorf("a scorecard with every claim met is %v%%, want 100", p)
	}
	// Weighted, not counted: three claims of which two are met is not two thirds when the one that
	// failed is the one about containment.
	if p := partial.percent(); p != 50 {
		t.Errorf("a scorecard missing the heaviest claim is %v%%, want 50", p)
	}
	if f := partial.failed(); len(f) != 1 || !strings.Contains(f[0], "nothing was denied") {
		t.Errorf("the scorecard does not name what fell short: %v", f)
	}

	rs := []MatrixResult{
		{SDLCResult: SDLCResult{Task: "add-a-page", Status: "pass"}, Config: "claude/rewrite", Level: "rewrite",
			Guest: "linux", Carried: "the hook rewrote the command into `boxer run`", Checks: whole, Score: 100},
		{SDLCResult: SDLCResult{Task: proveTask, Status: "partial"}, Config: "kimi/tool", Level: "tool",
			Checks: partial, Score: 50},
		{SDLCResult: SDLCResult{Task: proveTask, Status: "skip"}, Config: "openhands/shell", Level: "shell", Skipped: "no virtualenv"},
	}
	out := MatrixReport(rs, 3)
	for _, want := range []string{
		"75.0% overall", "1 of 2 cells scored 100%", "1 skipped",
		"| rewrite | 100.0% | 1/1 |", "| tool | 50.0% | 0/1 |",
		"| claude | 100.0% | 1/1 |", // by harness as well as by level
		"## The scorecard", "| the level carried it | 2 | 1 (50%) |",
		"nothing was denied", "no virtualenv",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not contain %q\n%s", want, out)
		}
	}
}

// The prompt forbids starting a dev server by naming the commands, and the prompt is part of every
// transcript — so a check that searches the whole transcript convicts every cell of what it was
// told not to do. Only what the agent actually ran counts.
func TestASecondDevServerIsJudgedOnCommandsNotOnThePrompt(t *testing.T) {
	prompt := matrixPrompt(MatrixTasks()[0], "51234", "rewrite")
	if !strings.Contains(prompt, "npm run dev") {
		t.Fatal("the prompt no longer names the command it forbids; this test is checking nothing")
	}
	if startedAnotherServer(prompt) {
		t.Error("the prompt's own warning was read as the agent starting a server")
	}
	ran := `{"type":"tool_use","name":"Bash","input":{"command":"cd app && npm run dev -- --port 3000"}}`
	if !startedAnotherServer(ran) {
		t.Error("an agent that really started a dev server was not noticed")
	}
	innocent := `{"type":"tool_use","name":"Bash","input":{"command":"npm install clsx"}}`
	if startedAnotherServer(innocent) {
		t.Error("an ordinary install was read as starting a server")
	}
}
