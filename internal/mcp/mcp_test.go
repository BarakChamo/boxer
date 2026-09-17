package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/vmtest"
)

func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("require_worktree = \"off\"\n"), 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	r, _ := filepath.EvalSymlinks(dir)
	return r
}

func TestRoundTrip(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t)
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
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
