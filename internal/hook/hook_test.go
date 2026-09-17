package hook

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
	r, _ := filepath.EvalSymlinks(dir)
	return r
}

func call(t *testing.T, harness string, in map[string]any) (map[string]any, string, int) {
	t.Helper()
	b, _ := json.Marshal(in)
	var out, errb bytes.Buffer
	code := Run(harness, bytes.NewReader(b), &out, &errb, func(cwd, h string, id scope.Identity) (*box.Env, error) {
		e, err := box.Resolve(cwd, h, id)
		if e != nil {
			e.Stderr = &errb
		}
		return e, err
	})
	var parsed map[string]any
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
			t.Fatalf("stdout not JSON: %q", out.String())
		}
	}
	return parsed, errb.String(), code
}

func hso(m map[string]any) map[string]any {
	h, _ := m["hookSpecificOutput"].(map[string]any)
	return h
}

func TestClaudeFamilyRewrite(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "")
	for _, h := range []string{"claude-code", "codex", "grok"} {
		tool := Dialects[h].ShellTool
		out, _, code := call(t, h, map[string]any{"hook_event_name": "PreToolUse", "tool_name": tool, "tool_input": map[string]any{"command": "bun test", "description": "tests"}, "cwd": dir, "session_id": "s"})
		u, _ := hso(out)["updatedInput"].(map[string]any)
		if code != 0 || hso(out)["permissionDecision"] != "allow" || u["command"] != "boxer run -c 'bun test'" || u["description"] != "tests" {
			t.Fatalf("%s: %v", h, out)
		}
	}
	out, _, _ := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "git status"}, "cwd": dir})
	if out != nil {
		t.Fatalf("passthrough must be silent: %v", out)
	}
	out, _, _ = call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Read", "tool_input": map[string]any{"file_path": "x"}, "cwd": dir})
	if out != nil {
		t.Fatalf("other tools must be silent: %v", out)
	}
}

func TestGeminiDialect(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "")
	out, _, _ := call(t, "gemini-cli", map[string]any{"hook_event_name": "BeforeTool", "tool_name": "run_shell_command", "tool_input": map[string]any{"command": "npm ci"}, "cwd": dir})
	ti, _ := hso(out)["tool_input"].(map[string]any)
	if ti["command"] != "boxer run -c 'npm ci'" {
		t.Fatalf("gemini rewrite: %v", out)
	}
	// Bash is not gemini's shell tool name; must be ignored.
	out, _, _ = call(t, "gemini-cli", map[string]any{"hook_event_name": "BeforeTool", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm ci"}, "cwd": dir})
	if out != nil {
		t.Fatal("wrong tool name must be silent")
	}
}

func TestOpenCodeDialect(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "")
	out, _, _ := call(t, "opencode", map[string]any{"hook_event_name": "tool.execute.before", "tool_name": "bash", "tool_input": map[string]any{"command": "go test ./..."}, "cwd": dir, "session_id": "s"})
	if out["command"] != "boxer run -c 'go test ./...'" {
		t.Fatalf("opencode rewrite: %v", out)
	}
	dir = repo(t, "mode = \"tool\"\n")
	out, _, _ = call(t, "opencode", map[string]any{"hook_event_name": "tool.execute.before", "tool_name": "bash", "tool_input": map[string]any{"command": "go test"}, "cwd": dir})
	if d, _ := out["deny"].(string); !strings.Contains(d, "boxer run -c 'go test'") {
		t.Fatalf("opencode deny: %v", out)
	}
}

func TestToolModeDenies(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "mode = \"tool\"\n")
	out, _, _ := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm test"}, "cwd": dir})
	r, _ := hso(out)["permissionDecisionReason"].(string)
	if hso(out)["permissionDecision"] != "deny" || !strings.Contains(r, "fix: boxer run -c 'npm test'") {
		t.Fatalf("deny: %v", out)
	}
	out, _, _ = call(t, "gemini-cli", map[string]any{"hook_event_name": "BeforeTool", "tool_name": "run_shell_command", "tool_input": map[string]any{"command": "npm test"}, "cwd": dir})
	if out["decision"] != "deny" || !strings.Contains(out["reason"].(string), "boxer_run") {
		t.Fatalf("gemini deny: %v", out)
	}
}

func TestBlockOnlyHarness(t *testing.T) {
	vmtest.Install(t)
	in := map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm test"}}
	in["cwd"] = repo(t, "") // enforcement=both: shims cover it, so allow
	if out, _, _ := call(t, "dsh", in); out != nil {
		t.Fatalf("dsh with shims must allow: %v", out)
	}
	in["cwd"] = repo(t, "enforcement = \"hook\"\n")
	out, _, _ := call(t, "dsh", in)
	r, _ := hso(out)["permissionDecisionReason"].(string)
	if hso(out)["permissionDecision"] != "deny" || !strings.Contains(r, "boxer run -c 'npm test'") {
		t.Fatalf("dsh hook-only must deny with fix: %v", out)
	}
}

func TestSessionStartProvisionsAndInstructs(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := repo(t, "")
	out, errs, code := call(t, "claude-code", map[string]any{"hook_event_name": "SessionStart", "source": "startup", "cwd": dir, "session_id": "s1"})
	ctx, _ := hso(out)["additionalContext"].(string)
	if code != 0 || !strings.Contains(ctx, "boxer sandbox") || hso(out)["hookEventName"] != "SessionStart" {
		t.Fatalf("instruct: %v %s", out, errs)
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "machine create") || !strings.Contains(string(b), "machine start") {
		t.Fatalf("session start must provision:\n%s", b)
	}
	out, _, _ = call(t, "gemini-cli", map[string]any{"hook_event_name": "SessionStart", "cwd": dir})
	if _, has := hso(out)["hookEventName"]; has {
		t.Fatal("gemini context carries no hookEventName")
	}
}

func TestSessionEndReclaimsOnlyWhenConfigured(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "destroy_on = [\"session_end\"]\n")
	call(t, "codex", map[string]any{"hook_event_name": "SessionStart", "cwd": dir, "session_id": "s1"})
	e, _ := box.Resolve(dir, "codex", scope.Identity{SessionID: "s1"})
	if _, ok, _ := e.Exists(); !ok {
		t.Fatal("provisioned")
	}
	call(t, "codex", map[string]any{"hook_event_name": "SessionEnd", "reason": "other", "cwd": dir, "session_id": "s1"})
	if _, ok, _ := e.Exists(); ok {
		t.Fatal("session_end should have reclaimed")
	}
}

func TestSubagentIsolation(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := repo(t, "isolation = \"subagent\"\ncreate_on = [\"subagent_start\"]\n")
	call(t, "claude-code", map[string]any{"hook_event_name": "SubagentStart", "cwd": dir, "session_id": "s1", "agent_id": "a1", "agent_type": "Explore"})
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "--label boxer.isolation=subagent") {
		t.Fatalf("subagent VM:\n%s", b)
	}
}

func TestOutsideRepoIsSilentForIntercept(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, errs, code := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm test"}, "cwd": t.TempDir()})
	// Default on_sandbox_unavailable=fail: refuse rather than silently run on host.
	if code != 0 || hso(out)["permissionDecision"] != "deny" {
		t.Fatalf("outside repo under fail policy: %v %s", out, errs)
	}
}

func TestInsideGuestIsSilent(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "")
	t.Setenv("BOXER_INSIDE", "1")
	out, _, code := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm test"}, "cwd": dir})
	if code != 0 || out != nil {
		t.Fatalf("inside a guest the hook must not wrap again: %v", out)
	}
	out, _, _ = call(t, "claude-code", map[string]any{"hook_event_name": "SessionStart", "cwd": dir})
	if out != nil {
		t.Fatal("inside a guest session start must not provision")
	}
}

func TestUnknownHarnessAndBadJSON(t *testing.T) {
	var out, errb bytes.Buffer
	if Run("nope", strings.NewReader("{}"), &out, &errb, box.Resolve) != 1 {
		t.Fatal("unknown harness is exit 1")
	}
	if Run("claude-code", strings.NewReader("not json"), &out, &errb, box.Resolve) != 0 {
		t.Fatal("bad json must not break the session")
	}
}

func TestSessionIsolationRewriteCarriesIdentity(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t, "isolation = \"subagent\"\n")
	out, _, _ := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "bun test"}, "cwd": dir, "session_id": "s1", "agent_id": "a1"})
	u, _ := hso(out)["updatedInput"].(map[string]any)
	if u["command"] != "boxer run --session s1 --agent a1 -c 'bun test'" {
		t.Fatalf("identity not threaded: %v", u["command"])
	}
	dir = repo(t, "")
	out, _, _ = call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "bun test"}, "cwd": dir, "session_id": "s1"})
	u, _ = hso(out)["updatedInput"].(map[string]any)
	if u["command"] != "boxer run -c 'bun test'" {
		t.Fatalf("worktree isolation must not add flags: %v", u["command"])
	}
}

func TestWarmOnSessionStartDoesNotBlock(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := repo(t, "warm_on_session_start = true\nisolation = \"session\"\n")
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "boxer")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+marker+"\n"), 0o755)
	box.Executable = func() (string, error) { return script, nil }
	t.Cleanup(func() { box.Executable = os.Executable })
	out, _, code := call(t, "claude-code", map[string]any{"hook_event_name": "SessionStart", "cwd": dir, "session_id": "s1"})
	if code != 0 || hso(out)["additionalContext"] == nil {
		t.Fatalf("session start must still brief the agent: %v", out)
	}
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "machine create") {
		t.Fatal("the hook itself must not create the VM when warming in the background")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if b, err := os.ReadFile(marker); err == nil {
			if strings.TrimSpace(string(b)) != "up --harness claude-code --session s1" {
				t.Fatalf("detached up args: %q", b)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no detached boxer up")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
