package hook

import (
	"bytes"
	"encoding/json"
	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vmtest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	out, _, _ := call(t, "opencode", map[string]any{"hook_event_name": "tool.execute.before", "tool_name": "bash", "tool_input": map[string]any{"command": "go test ./..."}, "cwd": dir, "session_id": "s"})
	if out["command"] != "boxer run -c 'go test ./...'" {
		t.Fatalf("opencode rewrite: %v", out)
	}
	dir = vmtest.Repo(t, vmtest.NoWorktreeCheck+"mode = \"tool\"\n")
	out, _, _ = call(t, "opencode", map[string]any{"hook_event_name": "tool.execute.before", "tool_name": "bash", "tool_input": map[string]any{"command": "go test"}, "cwd": dir})
	if d, _ := out["deny"].(string); !strings.Contains(d, "boxer run -c 'go test'") {
		t.Fatalf("opencode deny: %v", out)
	}
}

func TestToolModeDenies(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"mode = \"tool\"\n")
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
	// DSH's shell tool is named `bash`, lowercase (verified 2026-09-18 against 0.1.5-rc.2).
	in := map[string]any{"hook_event_name": "PreToolUse", "tool_name": "bash", "tool_input": map[string]any{"command": "npm test"}}
	in["cwd"] = vmtest.Repo(t, vmtest.NoWorktreeCheck) // enforcement=both: shims cover it, so allow
	if out, _, _ := call(t, "dsh", in); out != nil {
		t.Fatalf("dsh with shims must allow: %v", out)
	}
	in["cwd"] = vmtest.Repo(t, vmtest.NoWorktreeCheck+"enforcement = \"hook\"\n")
	out, _, _ := call(t, "dsh", in)
	r, _ := hso(out)["permissionDecisionReason"].(string)
	if hso(out)["permissionDecision"] != "deny" || !strings.Contains(r, "boxer run -c 'npm test'") {
		t.Fatalf("dsh hook-only must deny with fix: %v", out)
	}
}

func TestSessionStartProvisionsAndInstructs(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"destroy_on = [\"session_end\"]\n")
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"isolation = \"subagent\"\ncreate_on = [\"subagent_start\"]\n")
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
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
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"isolation = \"subagent\"\n")
	out, _, _ := call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "bun test"}, "cwd": dir, "session_id": "s1", "agent_id": "a1"})
	u, _ := hso(out)["updatedInput"].(map[string]any)
	if u["command"] != "boxer run --session s1 --agent a1 -c 'bun test'" {
		t.Fatalf("identity not threaded: %v", u["command"])
	}
	dir = vmtest.Repo(t, vmtest.NoWorktreeCheck)
	out, _, _ = call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "bun test"}, "cwd": dir, "session_id": "s1"})
	u, _ = hso(out)["updatedInput"].(map[string]any)
	if u["command"] != "boxer run -c 'bun test'" {
		t.Fatalf("worktree isolation must not add flags: %v", u["command"])
	}
}

func TestWarmOnSessionStartDoesNotBlock(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck+"warm_on_session_start = true\nisolation = \"session\"\n")
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

func TestGrokBriefNamesTheDispatcher(t *testing.T) {
	cfg := config.Defaults()
	cfg.Mode = "tool"
	brief := box.InstructionsFor(cfg, Dialects["grok"].RunToolHint)
	if !strings.Contains(brief, "search_tool") || !strings.Contains(brief, "use_tool") {
		t.Fatalf("grok brief must say how to reach the run tool:\n%s", brief)
	}
	if plain := box.Instructions(cfg); !strings.Contains(plain, "Use the boxer_run tool") {
		t.Fatalf("default brief changed:\n%s", plain)
	}
}

// Grok accepts a session-start hook but never shows its context to the model, so tool mode there
// costs one denial. The fact lives in the dialect, and only there.
func TestOnlyGrokLacksSessionContext(t *testing.T) {
	for name, d := range Dialects {
		if d.NoSessionContext != (name == "grok") {
			t.Errorf("%s: NoSessionContext=%v", name, d.NoSessionContext)
		}
	}
}

// internal/eval's oracle parses BOXER_TRACE line by line: a stamp, the harness, an arrow, and the
// JSON. The event stream shares the file, so this pins the shape the oracle needs.
func TestTraceFileKeepsItsFormat(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	trace := filepath.Join(t.TempDir(), "trace.log")
	t.Setenv("BOXER_TRACE", trace)
	call(t, "claude-code", map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "bun test"}, "cwd": dir, "session_id": "s"})
	b, _ := os.ReadFile(trace)
	var in, out int
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, " claude-code <- ") && strings.Contains(line, `"tool_name":"Bash"`) {
			in++
		}
		if strings.Contains(line, " claude-code -> ") && strings.Contains(line, `"command":"boxer run`) {
			out++
		}
	}
	if in != 1 || out != 1 {
		t.Fatalf("want one input and one output line, got %d/%d:\n%s", in, out, b)
	}
	// The rewrite is also an event, and the command it carries is elided.
	if !strings.Contains(string(b), `"event":"rewrite"`) || strings.Contains(string(b), `"command":"boxer run -c 'bun test'","tool"`) {
		t.Fatalf("event line: %s", b)
	}
}

// TestCopilotDialect covers the three things unique to Copilot's contract: arguments at toolArgs
// (as an object and as a JSON string), a second shell tool name, and modifiedArgs as the rewrite.
func TestCopilotDialect(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.Repo(t, vmtest.NoWorktreeCheck)
	args := map[string]any{"command": "bun test", "description": "tests"}
	asString, _ := json.Marshal(args)
	for name, toolArgs := range map[string]any{"object": args, "json string": string(asString)} {
		for _, tool := range []string{"bash", "powershell"} {
			// No hook_event_name and no tool_name: Copilot names the event by the key the hook is
			// registered under and spells the tool field toolName.
			out, _, code := call(t, "copilot", map[string]any{"toolName": tool, "toolArgs": toolArgs, "cwd": dir, "sessionId": "s"})
			m, _ := out["modifiedArgs"].(map[string]any)
			if code != 0 || out["permissionDecision"] != "allow" || m["command"] != "boxer run -c 'bun test'" || m["description"] != "tests" {
				t.Fatalf("%s/%s: %v", name, tool, out)
			}
		}
	}
	// Tool mode denies, and the reason carries the fix; the exit code is 0 either way, because a
	// non-zero exit fails closed in Copilot and would break the session.
	t.Setenv("BOXER_MODE", "tool")
	out, _, code := call(t, "copilot", map[string]any{"toolName": "bash", "toolArgs": args, "cwd": dir})
	if code != 0 || out["permissionDecision"] != "deny" || !strings.Contains(out["permissionDecisionReason"].(string), "boxer run") {
		t.Fatalf("tool mode: %v (code %d)", out, code)
	}
	// An unknown event and a non-shell tool are silent.
	for _, in := range []map[string]any{
		{"hook_event_name": "postToolUse", "toolName": "bash", "toolArgs": args, "cwd": dir},
		{"toolName": "str_replace_editor", "toolArgs": args, "cwd": dir},
	} {
		if out, _, code := call(t, "copilot", in); out != nil || code != 0 {
			t.Fatalf("expected silence: %v %d", out, code)
		}
	}
}
