package eval

import (
	"strings"
	"testing"
)

// The point of the matrix is coverage of integration levels, so a row that quietly duplicates
// another level's coverage, or a level that disappears, is the failure worth catching.
func TestTheMatrixCoversEveryIntegrationLevel(t *testing.T) {
	want := []string{"rewrite", "tool", "shims", "shell", "inside", "orchestrator"}
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
	for _, l := range want {
		if !seen[l] {
			t.Errorf("no configuration exercises the %q level", l)
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
	cases := []struct {
		name string
		sig  func(trace, Transcript, string) (string, error)
		tr   trace
		tx   Transcript
		host string
		ok   bool
	}{
		{"rewrite with a rewrite", sigRewrite, trace{rewrites: 1}, Transcript{}, linux, true},
		{"rewrite where the agent typed it", sigRewrite, trace{inputs: []string{"boxer run npm i"}}, Transcript{}, linux, true},
		{"rewrite proved only by the transcript", sigRewrite, trace{}, Transcript{Raw: "$ boxer run node -e x"}, linux, true},
		{"rewrite with no evidence at all", sigRewrite, trace{denies: 2}, Transcript{}, linux, false},
		{"rewrite that never left the host", sigRewrite, trace{rewrites: 1}, Transcript{}, mac, false},
		// A compliant agent in tool mode is never denied anything: it just uses the run tool. A
		// signature that insisted on a denial would fail the level for working properly.
		{"tool with the run tool and no denial", sigTool, trace{}, runTool, linux, true},
		{"tool with a denial", sigTool, trace{denies: 1}, Transcript{}, linux, true},
		{"tool that did neither", sigTool, trace{}, Transcript{}, linux, false},
		{"shims with no rewrites", sigShims, trace{}, Transcript{}, linux, true},
		{"shims that were really hooks", sigShims, trace{rewrites: 3}, Transcript{}, linux, false},
		{"inside with a silent hook", sigInside, trace{}, Transcript{}, linux, true},
		{"inside with a hook firing", sigInside, trace{events: []string{"PreToolUse"}}, Transcript{}, linux, false},
		{"no platform recorded at all", sigShims, trace{}, Transcript{}, "", false},
	}
	for _, c := range cases {
		how, err := c.sig(c.tr, c.tx, c.host)
		if (err == nil) != c.ok {
			t.Errorf("%s: wanted ok=%v, got %v", c.name, c.ok, err)
		}
		if err == nil && how == "" {
			t.Errorf("%s: passed without saying how the level carried the work", c.name)
		}
	}
}

func TestTheReportNamesEveryLevelItRan(t *testing.T) {
	rs := []MatrixResult{
		{SDLCResult: SDLCResult{Task: "add-a-page", Status: "pass"}, Config: "claude/rewrite", Level: "rewrite", Guest: "linux", Carried: "the hook rewrote the command into `boxer run`"},
		{SDLCResult: SDLCResult{Task: proveTask, Status: "fail"}, Config: "kimi/tool", Level: "tool", Signature: "nothing was denied"},
		{SDLCResult: SDLCResult{Task: proveTask, Status: "skip"}, Config: "openhands/shell", Level: "shell", Skipped: "no virtualenv"},
	}
	out := MatrixReport(rs, 3)
	for _, want := range []string{"1/2 passed", "(1 skipped)", "| rewrite | 1/1 |", "| tool | 0/1 |", "nothing was denied", "no virtualenv"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not contain %q\n%s", want, out)
		}
	}
}
