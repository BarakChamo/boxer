package eval

import (
	"os"
	"strings"
	"testing"
)

func TestInfra(t *testing.T) {
	cases := []struct {
		r    Result
		want bool
	}{
		{Result{Findings: []Finding{{"run", "boxer run: cause: START_FAILED"}}}, true},
		{Result{Findings: []Finding{{"run", "npm ERR! code EIDLETIMEOUT"}}}, true},
		{Result{Findings: []Finding{{"run", "claude timed out"}}}, true},
		{Result{Findings: []Finding{{"run", "initialize: agent closed: EOF"}}}, true},
		{Result{Findings: []Finding{{"run", "session/prompt: agent closed: EOF"}}, Raw: `<- {"method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk"}}}`}, false},
		{Result{Findings: []Finding{{"guest", `final answer "Darwin", wanted "Linux"`}}}, false},
		{Result{Status: "pass"}, false},
	}
	for _, c := range cases {
		if got := infra(c.r); got != c.want {
			t.Errorf("infra(%v) = %v, want %v", c.r.Findings, got, c.want)
		}
	}
}

func TestHostLockNestedIsNoop(t *testing.T) {
	t.Setenv("BOXER_EVAL_LOCKED", "1")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	unlock, err := HostLock(nil)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if _, err := os.Stat(LockPath()); err == nil {
		t.Fatal("nested HostLock must not touch the lock file")
	}
}

func TestSessionRootPrefersOrchestratorWorktree(t *testing.T) {
	e := &Env{Repo: "/repo"}
	if got := e.SessionRoot(Cell{}); got != "/repo" {
		t.Errorf("no orchestrator worktree: got %q, want /repo", got)
	}
	if got := e.SessionRoot(Cell{Timing: "before"}); got != "/repo/wt" {
		t.Errorf("timing cell: got %q, want /repo/wt", got)
	}
	// An orchestrator cuts its own worktree and its driver records it; the oracle judges that one.
	e.Root = "/elsewhere/task-1"
	for _, c := range []Cell{{}, {Timing: "before"}} {
		if got := e.SessionRoot(c); got != e.Root {
			t.Errorf("Env.Root set: got %q, want %q", got, e.Root)
		}
	}
}

// TestParseCopilotJSON uses lines captured from copilot 1.0.86 `--output-format json`.
func TestParseCopilotJSON(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"session.mcp_servers_loaded","data":{"servers":[{"name":"boxer"}]}}`,
		`{"type":"tool.execution_start","data":{"toolCallId":"call_0","toolName":"bash","arguments":{"command":"uname -a"}}}`,
		`{"type":"tool.execution_complete","data":{"toolCallId":"call_0","success":true}}`,
		`{"type":"assistant.message","data":{"text":"Linux\n"}}`,
		`{"type":"result","exitCode":0}`,
	}, "\n")
	tr := parseCopilotJSON(raw)
	if tr.Answer != "Linux" || len(tr.Tools) != 1 || tr.Tools[0] != "bash" {
		t.Fatalf("%+v", tr)
	}
}
