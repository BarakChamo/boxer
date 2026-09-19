package eval

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// The control cell decides whether boxer stayed out of the way, and it is the one verdict the
// oracle reaches without a VM. It is also the one that must not be lenient: a rewrite or a denial
// in mode "off" means boxer acted where it was told not to.
func TestJudgeControlCell(t *testing.T) {
	env := &Env{Tier: "t1", RunID: "judge-test", Trace: filepath.Join(t.TempDir(), "trace.log")}
	off := Cell{Harness: "claude-code", Mode: "off", Compliant: true}

	if f := Judge(env, off, Transcript{Answer: "Darwin"}); len(f) != 0 {
		t.Fatalf("a clean control cell has no findings: %v", f)
	}
	// The scripted model ran on the host, which is what "off" means; a live model that chose the
	// run tool anyway is also correct.
	if f := Judge(&Env{Tier: "t2", RunID: "j", Trace: env.Trace}, off, Transcript{Answer: "Linux"}); len(f) != 0 {
		t.Fatalf("live control may choose boxer_run: %v", f)
	}
	if f := Judge(env, off, Transcript{Answer: "Linux"}); len(f) != 1 || f[0].Check != "control" {
		t.Fatalf("scripted control must have run on the host: %v", f)
	}
	if f := Judge(env, off, Transcript{Answer: ""}); len(f) == 0 {
		t.Fatal("no answer at all is a finding")
	}
	// A hook that acted in mode off is the failure this cell exists to catch.
	trace := "t claude-code <- {\"tool_name\":\"Bash\"}\nt claude-code -> {\"command\": \"boxer run -c 'uname -s'\"}\n"
	if err := os.WriteFile(env.Trace, []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}
	f := Judge(env, off, Transcript{Answer: "Darwin"})
	if len(f) != 1 || f[0].Check != "control" {
		t.Fatalf("a rewrite in mode off must be reported: %v", f)
	}
}

// wait bounds a harness invocation, and lastAnswer reads the answer out of a noisy stream. Both
// replaced a copy per driver, so both are worth pinning.
func TestWaitAndLastAnswer(t *testing.T) {
	if err := wait(exec.Command("true"), "t"); err != nil {
		t.Fatalf("a command that exits 0: %v", err)
	}
	if err := wait(exec.Command("false"), "t"); err == nil {
		t.Fatal("a failing command must report its failure")
	}
	if err := wait(exec.Command("/nonexistent/binary"), "t"); err == nil {
		t.Fatal("a command that cannot start must report it")
	}
	raw := "$ uname -s\ntimestamp=1 level=info\n\nLinux the kernel\n"
	if got := lastAnswer(raw, "$ ", "timestamp="); got != "Linux" {
		t.Fatalf("answer: %q", got)
	}
	if got := lastAnswer("$ only noise\n", "$ "); got != "" {
		t.Fatalf("nothing but noise is no answer: %q", got)
	}
}

// A harness that spawns a child hands it the same stdout pipe, so killing the harness does not
// close it and Wait blocks until the grandchild exits. A timed-out cell must still end.
func TestWaitDoesNotHangOnASurvivingChild(t *testing.T) {
	old := harnessTimeout
	harnessTimeout = time.Second
	t.Cleanup(func() { harnessTimeout = old })

	cmd := exec.Command("sh", "-c", "sleep 300 & exec sleep 300")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	done := make(chan error, 1)
	go func() { done <- wait(cmd, "t") }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("want a timeout error, got %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("wait hung after killing a process whose child still holds the pipe")
	}
}

// The flow tier's shape: two cells, and a tier that refuses to run anywhere but its own.
func TestFlowCellsAndAvailability(t *testing.T) {
	f := Flow{}
	if _, why := f.Available("t1"); why == "" {
		t.Fatal("the flow tier must not run inside t1")
	}
	cells := f.Cells("flow")
	if len(cells) != 2 {
		t.Fatalf("mechanics and agent: %v", cells)
	}
	seen := map[string]bool{}
	for _, c := range cells {
		seen[c.Scenario] = true
		if c.Tier != "flow" || c.Harness != "flow" {
			t.Errorf("cell: %+v", c)
		}
	}
	if !seen["flow"] || !seen["flow-agent"] {
		t.Fatalf("both scenarios: %v", seen)
	}
}

// A step's outcome is printed as it happens, and a failure must carry its reason: the transcript
// is the only thing kept when a flow cell fails.
func TestFlowStepStatusAndTail(t *testing.T) {
	if got := status(nil); got != "ok" {
		t.Errorf("ok: %q", got)
	}
	if got := status(errStub{}); !strings.Contains(got, "FAIL") || !strings.Contains(got, "boom") {
		t.Errorf("a failure names itself: %q", got)
	}
	if got := lastOf("abcdef", 3); got != "def" {
		t.Errorf("tail: %q", got)
	}
	if got := lastOf("ab", 5); got != "ab" {
		t.Errorf("short input is itself: %q", got)
	}
}

type errStub struct{}

func (errStub) Error() string { return "boom" }

// httpOK gives up rather than hanging, and says what it last saw.
func TestHTTPOKGivesUp(t *testing.T) {
	err := httpOK("http://127.0.0.1:1/", 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "never answered") {
		t.Fatalf("want a timeout naming the url, got %v", err)
	}
}

// The SDLC tier's own shape: twelve ordinary jobs, each with a route to check and a marker to find
// on it, so a lifecycle cannot pass by the agent saying it worked.
func TestSDLCTasksAreCheckable(t *testing.T) {
	tasks := SDLCTasks()
	if len(tasks) < 12 {
		t.Fatalf("want at least twelve lifecycles, got %d", len(tasks))
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.Name] {
			t.Errorf("duplicate lifecycle %q", task.Name)
		}
		seen[task.Name] = true
		if task.Prompt == "" || task.Route == "" || task.Expect == "" {
			t.Errorf("%s: a lifecycle needs a job, a route and something to find there: %+v", task.Name, task)
		}
		if !strings.HasPrefix(task.Route, "/") {
			t.Errorf("%s: route %q", task.Name, task.Route)
		}
		if !strings.Contains(task.Prompt, task.Expect) {
			t.Errorf("%s: the agent must be told the marker it has to render (%q)", task.Name, task.Expect)
		}
	}
}

// The report is what someone reads afterwards, so it has to carry the counts and the per-phase
// costs rather than a bare pass or fail.
func TestSDLCReport(t *testing.T) {
	rs := []SDLCResult{
		{Task: "add-a-page", Status: "pass", UsedMCP: true, UsedBrowse: true, Spend: 0.01, Total: 90 * time.Second},
		{Task: "api-route", Status: "fail", Findings: []string{"the page does not show X"}, Total: 30 * time.Second},
	}
	out := SDLCReport(rs, 3)
	for _, want := range []string{"passed 1 of 2", "MCP used in 1", "browser used in 1", "add-a-page", "the page does not show X", "3 at a time"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report must carry %q:\n%s", want, out)
		}
	}
}
