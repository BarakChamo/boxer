package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func repo(t *testing.T, toml string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("require_worktree = \"off\"\n"+toml), 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	r, _ := filepath.EvalSymlinks(dir)
	t.Chdir(r) // the server's own working directory is the session's scope
	return r
}

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
	dir := repo(t, "create_on = [\"mcp\"]\n")
	before := time.Now().Add(-time.Second)
	serve(t, initialize)
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "machine create") || !strings.Contains(string(b), "machine start") {
		t.Fatalf("initialize must provision the cwd scope:\n%s", b)
	}
	e, _ := box.Resolve(dir, "test", scope.Identity{})
	if box.LastUsed(e.Scope.Key).Before(before) {
		t.Fatal("EOF must touch last-used")
	}

	_, log = vmtest.Install(t)
	repo(t, "create_on = [\"run\"]\n")
	serve(t, initialize)
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "machine create") {
		t.Fatalf("create_on without mcp must not provision on initialize:\n%s", b)
	}
}

// A model passes back the guest path from its brief, a host path, or nonsense; each maps to a
// guest working directory or falls back to the server's scope with a note.
func TestRunCwdMapping(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := repo(t, "mount_at = \"/workspace\"\ncreate_on = [\"run\"]\n")
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
	dir := repo(t, "create_on = [\"run\"]\n")
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
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("want 6 responses (notification ignored), got %d:\n%s", len(lines), out.String())
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
