package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func serve(t *testing.T, lines ...string) []string {
	t.Helper()
	var out bytes.Buffer
	s := &Server{Harness: "test", Resolve: box.Resolve, Version: "t"}
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(out.String()), "\n")
}

const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

// R-SIG: initialize warms the scope when create_on lists "mcp"; EOF records the last use.
func TestLifecycleSignals(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"create_on = [\"mcp\"]\n")
	// The warm-up is a detached `boxer up`; a script stands in for the binary and records argv.
	spawned := filepath.Join(t.TempDir(), "spawned")
	script := filepath.Join(t.TempDir(), "boxer")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+spawned+"\n"), 0o755)
	box.Executable = func() (string, error) { return script, nil }
	t.Cleanup(func() { box.Executable = os.Executable })
	before := time.Now().Add(-time.Second)
	serve(t, initialize)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, err := os.ReadFile(spawned); err == nil {
			if !strings.HasPrefix(string(b), "up") {
				t.Fatalf("initialize must spawn boxer up, got %q", b)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("initialize did not spawn a detached boxer up")
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = log
	e, _ := box.Resolve(dir, "test", scope.Identity{})
	if box.LastUsed(e.Scope.Key).Before(before) {
		t.Fatal("EOF must touch last-used")
	}

	for _, toml := range []string{"create_on = [\"run\"]\n", "create_on = [\"mcp\"]\nisolation = \"session\"\n"} {
		os.Remove(spawned)
		vmtest.RepoIn(t, toml)
		serve(t, initialize)
		time.Sleep(50 * time.Millisecond)
		if _, err := os.Stat(spawned); err == nil {
			t.Fatalf("initialize must not provision with %q (no mcp in create_on, or an isolation that needs a session id)", toml)
		}
	}
}

// A model passes back the guest path from its brief, a host path, or nonsense; each maps to a
// guest working directory or falls back to the server's scope with a note.
func TestRunCwdMapping(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"mount_at = \"/workspace\"\ncreate_on = [\"run\"]\n")
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	call := func(cwd string) string {
		return `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"true","cwd":"` + cwd + `"}}}`
	}
	for _, tc := range []struct{ cwd, workdir, note string }{
		{"/workspace/sub", "-w /workspace/sub", ""},
		{dir + "/sub", "-w /workspace/sub", ""},
		{"/nowhere/at/all", "-w /workspace ", "not inside a git worktree"},
	} {
		os.WriteFile(log, nil, 0o644)
		lines := serve(t, call(tc.cwd))
		var r map[string]any
		json.Unmarshal([]byte(lines[0]), &r)
		if r["result"].(map[string]any)["isError"] != false {
			t.Fatalf("cwd %s: %s", tc.cwd, lines[0])
		}
		if b, _ := os.ReadFile(log); !strings.Contains(string(b), tc.workdir) {
			t.Errorf("cwd %s: want guest workdir %q in\n%s", tc.cwd, tc.workdir, b)
		}
		if got := strings.Contains(text(r), "not inside a git worktree"); got != (tc.note != "") {
			t.Errorf("cwd %s: note presence %v, result %q", tc.cwd, got, text(r))
		}
	}
}

func TestRoundTrip(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"create_on = [\"run\"]\n")
	in := strings.Join([]string{
		initialize,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"boxer_status","arguments":{"cwd":"` + dir + `"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"echo hi; exit 2","cwd":"` + dir + `"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"echo ok","cwd":"` + dir + `"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"nope"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	s := &Server{Harness: "test", Resolve: box.Resolve, Version: "t"}
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(got) != 6 {
		t.Fatalf("want 6 responses (notification ignored), got %d:\n%s", len(got), out.String())
	}
	// Runs answer when they finish, not in the order asked: a client matches responses by id.
	lines := make([]string, 6)
	for _, l := range got {
		var withID struct{ ID int }
		_ = json.Unmarshal([]byte(l), &withID)
		lines[withID.ID-1] = l
	}
	var r map[string]any
	json.Unmarshal([]byte(lines[0]), &r)
	if r["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("initialize: %s", lines[0])
	}
	json.Unmarshal([]byte(lines[1]), &r)
	if len(r["result"].(map[string]any)["tools"].([]any)) != 2 {
		t.Fatalf("tools/list: %s", lines[1])
	}
	json.Unmarshal([]byte(lines[2]), &r)
	if !strings.Contains(text(r), "no sandbox yet") {
		t.Fatalf("status before run: %s", lines[2])
	}
	json.Unmarshal([]byte(lines[3]), &r)
	if !strings.Contains(text(r), "hi") || !strings.Contains(text(r), "[exit 2]") || r["result"].(map[string]any)["isError"] != true {
		t.Fatalf("run failing: %s", lines[3])
	}
	json.Unmarshal([]byte(lines[4]), &r)
	if !strings.Contains(text(r), "ok") || r["result"].(map[string]any)["isError"] != false {
		t.Fatalf("run ok: %s", lines[4])
	}
	json.Unmarshal([]byte(lines[5]), &r)
	if r["error"] == nil {
		t.Fatalf("unknown method: %s", lines[5])
	}
}

func text(r map[string]any) string {
	c := r["result"].(map[string]any)["content"].([]any)[0].(map[string]any)
	return c["text"].(string)
}

// The third transport for the one brief: published content is static, so a harness that reads
// neither hooks nor `boxer brief` can still fetch the configuration as a resource.
func TestBriefResource(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"mount_at = \"/src\"\n")
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"boxer://brief"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"boxer://nope"}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	s := &Server{Resolve: box.Resolve, Version: "t"}
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var r map[string]any
	json.Unmarshal([]byte(lines[0]), &r)
	if len(r["result"].(map[string]any)["resources"].([]any)) != 1 {
		t.Fatalf("resources/list: %s", lines[0])
	}
	json.Unmarshal([]byte(lines[1]), &r)
	c := r["result"].(map[string]any)["contents"].([]any)[0].(map[string]any)
	if !strings.Contains(c["text"].(string), "/src") {
		t.Fatalf("the resource must carry the resolved brief: %s", lines[1])
	}
	json.Unmarshal([]byte(lines[2]), &r)
	if r["error"] == nil {
		t.Fatalf("an unknown resource must be an error: %s", lines[2])
	}
}

// The model is the one who hits these: a tool that does not exist, a command that is empty, a
// sandbox policy that refuses, and a smolvm that is broken. Each has to come back as an MCP error
// result rather than as a dead server, because a crashed stdio server ends the session.
func TestErrorPaths(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"create_on = [\"session_start\"]\n")
	call := func(body string) map[string]any {
		var r map[string]any
		json.Unmarshal([]byte(serve(t, body)[0]), &r)
		return r
	}
	for _, tc := range []struct{ name, body, want string }{
		{"unknown tool", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`, "unknown tool"},
		{"empty command", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"  "}}}`, "is required"},
		{"run refused", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"true","cwd":"` + dir + `"}}}`, "NO_SANDBOX"},
	} {
		r := call(tc.body)
		res := r["result"].(map[string]any)
		if res["isError"] != true || !strings.Contains(text(r), tc.want) {
			t.Errorf("%s: %v", tc.name, r["result"])
		}
	}
	// Malformed JSON-RPC: a parse error and an invalid params error, neither of them fatal.
	lines := serve(t, `{not json`, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":7}`)
	if len(lines) != 2 || !strings.Contains(lines[0], "parse error") || !strings.Contains(lines[1], "invalid params") {
		t.Fatalf("malformed input: %v", lines)
	}
	// smolvm itself failing is reported through the tool result, not by dying.
	vmtest.FailVerb(t, "machine status", "daemon not reachable")
	r := call(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_status","arguments":{"cwd":"` + dir + `"}}}`)
	if r["result"].(map[string]any)["isError"] != true || !strings.Contains(text(r), "daemon not reachable") {
		t.Fatalf("status with a broken smolvm: %v", r["result"])
	}
}

// A cwd that is not a worktree and a server whose own directory is not one either: there is no
// scope to fall back to, so the call reports the resolution failure.
func TestResolveFailureOutsideAnyRepository(t *testing.T) {
	vmtest.Install(t)
	vmtest.Repo(t, vmtest.NoWorktreeCheck) // sets XDG isolation
	t.Chdir(t.TempDir())
	var r map[string]any
	json.Unmarshal([]byte(serve(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_status","arguments":{"cwd":"/nowhere"}}}`)[0]), &r)
	if r["result"].(map[string]any)["isError"] != true || !strings.Contains(text(r), "git repository") {
		t.Fatalf("outside any repository: %v", r["result"])
	}
}

// The paths a real client walks past: a notification carries no id and gets no reply, a
// resources/read with unreadable params is an error rather than a panic, and under session
// isolation the server declines to warm a VM it cannot name.
func TestNotificationsAndSessionIsolation(t *testing.T) {
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"isolation = \"session\"\ncreate_on = [\"mcp\"]\n")
	lines := serve(t,
		``,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":"not an object"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	if len(lines) != 3 {
		t.Fatalf("a notification must get no reply: %v", lines)
	}
	if !strings.Contains(lines[0], "protocolVersion") {
		t.Fatalf("initialize: %s", lines[0])
	}
	if !strings.Contains(lines[1], "error") {
		t.Fatalf("malformed resources/read: %s", lines[1])
	}
	ls, err := client.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 0 {
		t.Fatalf("session isolation must not warm a worktree-keyed VM: %v", ls)
	}
}

// Where a server is reachable is the question an agent asks status most, and the answer is
// different in every worktree, so the tool says it rather than sending the agent to the CLI: the
// named URL where there is one, the forwarded host port where there is not.
func TestStatusSaysWhereServersAre(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"network = { ports = [\"auto:3000\", \"auto:4000\"] }\n")
	call := func() string {
		var r map[string]any
		// The run, answered, then the status: a run answers when it finishes, so a client that
		// wants the status after it waits for that answer first.
		serve(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"true","cwd":"`+dir+`"}}}`)
		json.Unmarshal([]byte(serve(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"boxer_status","arguments":{"cwd":"`+dir+`"}}}`)[0]), &r)
		return text(r)
	}
	out := call()
	if !strings.Contains(out, "guest port 3000: http://127.0.0.1:") || !strings.Contains(out, "guest port 4000: http://127.0.0.1:") {
		t.Fatalf("forwarded ports: %s", out)
	}
	key := strings.Fields(strings.TrimPrefix(out, "scope "))[0]
	reg := filepath.Join(os.Getenv("XDG_STATE_HOME"), "boxer", "urls")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, guest := range map[string]string{"web.demo": "3000", "admin.demo": "9000"} {
		b, _ := json.Marshal(map[string]string{"scope": key, "guest": guest, "host": "1", "name": name, "url": "https://" + name + ".localhost"})
		if err := os.WriteFile(filepath.Join(reg, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out = call()
	for _, want := range []string{"guest port 3000: https://web.demo.localhost", "guest port 4000: http://127.0.0.1:", "guest port 9000: https://admin.demo.localhost"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// Under tool mode the model learns what the shell tool refuses before its first command: from the
// server's instructions, and from boxer_run's own description.
func TestToolModeTellsTheModelUpFront(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"mode = \"tool\"\nintercept = [\"*\"]\n")
	in := initialize + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	s := &Server{Harness: "test", Resolve: box.Resolve, Version: "t"}
	if err := s.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var r map[string]any
	json.Unmarshal([]byte(lines[0]), &r)
	if ins, _ := r["result"].(map[string]any)["instructions"].(string); !strings.Contains(ins, "Do not run") || !strings.Contains(ins, "boxer_run") {
		t.Fatalf("initialize must carry the brief as instructions: %s", lines[0])
	}
	json.Unmarshal([]byte(lines[1]), &r)
	var desc string
	for _, tl := range r["result"].(map[string]any)["tools"].([]any) {
		if m := tl.(map[string]any); m["name"] == "boxer_run" {
			desc = m["description"].(string)
		}
	}
	if !strings.Contains(desc, "refuses every command except git") || !strings.Contains(desc, "from the start") {
		t.Fatalf("boxer_run must say what the shell refuses here: %q", desc)
	}
	// The shared table is not changed by it: another repository gets the plain description.
	if strings.Contains(tools[0]["description"].(string)+tools[1]["description"].(string), "refuses") {
		t.Fatal("the package-level tool table was modified")
	}
}

// A run's output is bounded to its start and its end, with what was dropped counted.
func TestRunOutputKeepsItsStartAndEnd(t *testing.T) {
	h := &headTail{max: 4}
	for _, w := range []string{"ab", "cdef", "ghij", "kl"} {
		_, _ = h.Write([]byte(w))
	}
	if got := h.String(); got != "abcd\n[boxer: 4 bytes of output omitted]\nijkl" {
		t.Fatalf("%q", got)
	}
	small := &headTail{max: 4}
	_, _ = small.Write([]byte("abcdef"))
	if got := small.String(); got != "abcdef" {
		t.Fatalf("output within both halves is kept whole: %q", got)
	}
}

// A task runs by name through the tool, since there is no boxer in the guest to run `boxer run --task`.
func TestRunATaskByName(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks.hello]\ncmd = \"echo task-ran\"\n")
	s := &Server{Resolve: box.Resolve, Version: "t"}
	if out, isErr := s.call("boxer_run", map[string]any{"task": "hello", "cwd": dir}); isErr || !strings.Contains(out, "task-ran") {
		t.Fatalf("%v %s", isErr, out)
	}
	if out, isErr := s.call("boxer_run", map[string]any{"task": "nope", "cwd": dir}); !isErr || !strings.Contains(out, "hello") {
		t.Fatalf("an unknown task names the known ones: %s", out)
	}
	if _, isErr := s.call("boxer_run", map[string]any{"task": "hello", "command": "ls", "cwd": dir}); !isErr {
		t.Fatal("command and task together are refused")
	}
}

// The server runs in its own repository only: a cwd in another one is refused.
func TestACwdInAnotherRepositoryIsRefused(t *testing.T) {
	vmtest.Install(t)
	other := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	s := &Server{Resolve: box.Resolve, Version: "t"}
	if out, isErr := s.call("boxer_run", map[string]any{"command": "true", "cwd": other}); !isErr || !strings.Contains(out, "another repository") {
		t.Fatalf("%v %s", isErr, out)
	}
}

// A long run does not hold up the rest: ping is answered while boxer_run is still going.
func TestPingIsAnsweredDuringARun(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	pr, pw := io.Pipe()
	var out safeBuffer
	s := &Server{Resolve: box.Resolve, Version: "t"}
	done := make(chan struct{})
	go func() { _ = s.Serve(pr, &out); close(done) }()
	fmt.Fprintf(pw, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boxer_run","arguments":{"command":"sleep 3","cwd":%q}}}`+"\n", dir)
	fmt.Fprintln(pw, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), `"id":2`) {
		if time.Now().After(deadline) {
			t.Fatalf("ping waited for the run: %s", out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = pw.Close()
	<-done
	if !strings.Contains(out.String(), `"id":1`) {
		t.Fatal("the run's answer was lost")
	}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *safeBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

// A task's own variables reach its command.
func TestATaskRunsWithItsEnv(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks.hello]\ncmd = \"echo $GREETING\"\nenv = { GREETING = \"task-env\" }\n")
	s := &Server{Resolve: box.Resolve, Version: "t"}
	if out, isErr := s.call("boxer_run", map[string]any{"task": "hello", "cwd": dir}); isErr || !strings.Contains(out, "task-env") {
		t.Fatalf("%v %s", isErr, out)
	}
}

// Outside a repository there is nothing to keep to; a server in one refuses a cwd outside any.
func TestSameRepositoryEdges(t *testing.T) {
	plain := t.TempDir()
	repo := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	t.Chdir(plain)
	if !sameRepository(repo) {
		t.Fatal("a server outside any repository allows anything")
	}
	t.Chdir(repo)
	if sameRepository(plain) {
		t.Fatal("a directory outside any repository is not this one")
	}
}

// A cwd under the mount but pointing out of the worktree with `..` is not this repository.
func TestMountRelativeCwdCannotEscapeTheRepository(t *testing.T) {
	vmtest.Install(t)
	other := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	s := &Server{Resolve: box.Resolve, Version: "t"}
	// MountAt defaults to /workspace; /workspace/../<escape> joins to the worktree's parent.
	escape := "/workspace/../" + filepath.Base(other)
	if out, isErr := s.call("boxer_run", map[string]any{"command": "true", "cwd": escape}); !isErr && !strings.Contains(out, dir) {
		t.Fatalf("mount-relative cwd escaped: %v %s", isErr, out)
	}
}
