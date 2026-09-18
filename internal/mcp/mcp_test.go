package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
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
