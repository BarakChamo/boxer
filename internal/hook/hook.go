// Package hook is the one hook binary behind every harness (R-HK-1). It reads the harness's
// JSON on stdin, normalises the event to a purpose, and writes the harness's dialect on stdout.
// It contains no decision: decide.Decide and box own those.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/decide"
	"github.com/BarakChamo/boxer/internal/obs"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Dialect describes how one harness speaks the converged hook contract.
type Dialect struct {
	Name      string
	ShellTool string // tool_name for the shell tool
	// ShellTool2 is a second shell tool name when the harness has one (Copilot: bash and powershell).
	ShellTool2 string
	// ArgsField is the JSON field carrying the tool's arguments; "" means tool_input.
	ArgsField string
	// DefaultEvent is the purpose to assume when the payload carries no event name: Copilot
	// identifies the event by the key the hook is registered under and does not repeat it on the
	// wire (verified 2026-09-18 against 1.0.86, whose preToolUse payload is
	// {sessionId, timestamp, cwd, toolName, toolArgs}).
	DefaultEvent string
	// Rewrite is false for harnesses whose hooks can only allow or block.
	Rewrite bool
	// Family selects the output JSON shape: "claude" (Claude Code, Codex, Grok, Kimi, DSH) or "gemini".
	Family string
	// MCP is true when the bundle registers boxer's MCP server, whose initialize and EOF are the
	// session signals of last resort.
	MCP bool
	// Events maps this harness's event names to purposes.
	Events map[string]string
	// RunToolHint names the MCP run tool the way this harness shows it to the model; "" means the
	// tool is visible as boxer_run. Grok lists MCP tools only through its dispatcher.
	RunToolHint string
	// NoSessionContext marks a harness that accepts a session-start hook but never shows its
	// context to the model. The brief then arrives only through the skill, which the model reads
	// after it has already tried the shell, so in tool mode its first command is denied once and
	// it recovers. Measured, not assumed: eval-adherence.md, four models, Grok 1.0.34.
	NoSessionContext bool
}

var claudeEvents = map[string]string{
	"PreToolUse":    "intercept",
	"SessionStart":  "session_start",
	"SessionEnd":    "session_end",
	"SubagentStart": "subagent_start",
	"SubagentStop":  "subagent_stop",
}

// dshEvents differs from claudeEvents twice. The bridge does not implement SessionEnd, so boxer's
// MCP server EOF is the session-end signal there. And it runs SessionStart detached and aborts
// still-running hook processes when it disposes, so a hook that waits for a sandbox is killed when
// a short run ends; UserPromptSubmit is the awaited waterfall, so that is where boxer provisions
// and hands over its instructions instead.
var dshEvents = map[string]string{
	"PreToolUse":       "intercept",
	"UserPromptSubmit": "session_start",
	"SubagentStart":    "subagent_start",
	"SubagentStop":     "subagent_stop",
}

// Dialects is every harness the hook binary speaks.
var Dialects = map[string]Dialect{
	"claude-code": {Name: "claude-code", MCP: true, ShellTool: "Bash", Rewrite: true, Family: "claude", Events: claudeEvents},
	"codex":       {Name: "codex", MCP: true, ShellTool: "Bash", Rewrite: true, Family: "claude", Events: claudeEvents},
	// Grok sends Claude-compatible field names but its own tool name (verified 2026-09-17).
	"grok": {Name: "grok", MCP: true, ShellTool: "run_terminal_command", Rewrite: true, Family: "claude", Events: claudeEvents,
		RunToolHint: "the boxer_run tool: find it with search_tool, then call it with use_tool", NoSessionContext: true},
	"kimi": {Name: "kimi", MCP: true, ShellTool: "Bash", Rewrite: false, Family: "claude", Events: claudeEvents},
	// DSH has no hooks of its own; the shipped @deepseek-ai/dsh-hooks-claude-code bridge runs a
	// Claude Code hooks.json, so the payloads and the output shape are Claude Code's. It honours
	// deny and ask but logs and ignores updatedInput, and it does not implement SessionEnd. Its
	// shell tool is named `bash` (verified 2026-09-18 against 0.1.5-rc.2).
	"dsh": {Name: "dsh", MCP: true, ShellTool: "bash", Rewrite: false, Family: "claude", Events: dshEvents},
	"gemini-cli": {Name: "gemini-cli", MCP: true, ShellTool: "run_shell_command", Rewrite: true, Family: "gemini", Events: map[string]string{
		"BeforeTool":   "intercept",
		"SessionStart": "session_start",
		"SessionEnd":   "session_end",
	}},
	// OpenCode has a TypeScript plugin API, not stdin hooks; the rendered plugin forwards to this
	// dialect so the decision still lives in one binary.
	"opencode": {Name: "opencode", MCP: true, ShellTool: "bash", Rewrite: true, Family: "opencode", Events: map[string]string{
		"tool.execute.before": "intercept",
		"session.created":     "session_start",
	}},
	// pi's extension API: tool_call with a mutable event.input and { block } — the rendered
	// extension forwards to this dialect, same wire as OpenCode.
	// GitHub Copilot CLI (verified 2026-09-18 against 1.0.86: `copilot help config` documents
	// `hooks` keyed by event name with the schema of .github/hooks/*.json, `copilot help
	// environment` documents COPILOT_HOME and COPILOT_AUTO_UPDATE). Two hazards shape this row:
	// a hook that times out fails OPEN while a non-zero exit fails CLOSED, so boxer's hook must
	// stay fast and always exit 0 (Run only returns non-zero for an unknown harness name, which
	// cannot happen from an installed hooks file); and its rewrite is `modifiedArgs`, which
	// replaces the tool arguments wholesale, not Claude's hookSpecificOutput.updatedInput.
	// Only preToolUse is wired: the brief reaches the model through the skill and the MCP server.
	"copilot": {Name: "copilot", MCP: true, ShellTool: "bash", ShellTool2: "powershell", ArgsField: "toolArgs",
		Rewrite: true, Family: "copilot", DefaultEvent: "preToolUse", Events: map[string]string{"preToolUse": "intercept"}},
	"pi": {Name: "pi", ShellTool: "bash", Rewrite: true, Family: "opencode", Events: map[string]string{
		"tool_call":     "intercept",
		"session_start": "session_start",
		"session_end":   "session_end",
	}},
}

// Input is the union of fields boxer reads from any harness.
type Input struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	// ToolNameAlt is the same field in Copilot's spelling.
	ToolNameAlt string         `json:"toolName"`
	ToolInput   map[string]any `json:"tool_input"`
	SessionID   string         `json:"session_id"`
	// SessionIDAlt is the same field in Copilot's spelling.
	SessionIDAlt string `json:"sessionId"`
	AgentID      string `json:"agent_id"`
	// ToolArgs is Copilot's argument field: an object, or a JSON string holding one.
	ToolArgs json.RawMessage `json:"toolArgs"`
	CWD      string          `json:"cwd"`
	Source   string          `json:"source"`
	Reason   string          `json:"reason"`
}

// toolArgs returns the tool's arguments in the dialect's field. Copilot puts them at toolArgs,
// which the documented contract allows to be either an object or a JSON string holding one.
func toolArgs(d Dialect, in Input) map[string]any {
	if d.ArgsField != "toolArgs" {
		return in.ToolInput
	}
	m := map[string]any{}
	if len(in.ToolArgs) == 0 {
		return m
	}
	if json.Unmarshal(in.ToolArgs, &m) == nil {
		return m
	}
	var s string
	if json.Unmarshal(in.ToolArgs, &s) == nil {
		_ = json.Unmarshal([]byte(s), &m)
	}
	return m
}

// Resolver lets tests substitute box.Resolve.
type Resolver func(cwd, harness string, id scope.Identity) (*box.Env, error)

// Run handles one hook invocation and returns the process exit code.
func Run(harness string, stdin io.Reader, stdout, stderr io.Writer, resolve Resolver) int {
	d, ok := Dialects[harness]
	if !ok {
		fmt.Fprintf(stderr, "boxer: unknown harness %q; known: %s\n", harness, strings.Join(names(), ", "))
		return 1
	}
	if vm.Inside() {
		return 0 // already in a guest: the outer boxer owns isolation, never wrap twice
	}
	// BOXER_TRACE is read before any configuration is resolved: the hook may never get far
	// enough to load one, and the eval suite still needs the invocation recorded.
	obs.Configure(obs.Config{})
	start := time.Now()
	raw, _ := io.ReadAll(stdin)
	if obs.Tracing() {
		// Every hook invocation is appended for eval and support; the output is traced as it is written.
		obs.Trace(harness, "<-", strings.TrimSpace(string(raw)))
		stdout = io.MultiWriter(stdout, traceWriter{harness})
	}
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		fmt.Fprintf(stderr, "boxer: hook input is not JSON: %v\n", err)
		return 0 // never break a session over our own parse failure
	}
	if in.HookEventName == "" {
		in.HookEventName = d.DefaultEvent
	}
	if in.SessionID == "" {
		in.SessionID = in.SessionIDAlt
	}
	if in.ToolName == "" {
		in.ToolName = in.ToolNameAlt
	}
	purpose := d.Events[in.HookEventName]
	if purpose == "" {
		return 0
	}
	defer func() {
		obs.Emit(obs.Event{Name: obs.Hook, Harness: harness, Duration: time.Since(start), Outcome: obs.OK,
			Payload: map[string]any{"event": in.HookEventName, "purpose": purpose}})
	}()
	e, err := resolve(in.CWD, harness, scope.Identity{SessionID: in.SessionID, AgentID: in.AgentID})
	if err != nil {
		// Outside a repository or misconfigured: say so once at session start, stay silent otherwise.
		if purpose == "session_start" {
			fmt.Fprintln(stderr, err)
		}
		if purpose == "intercept" && e != nil && e.Cfg.Mode != "off" && e.Cfg.OnSandboxUnavailable == "fail" {
			return deny(d, stdout, err.Error(), "")
		}
		return 0
	}
	switch purpose {
	case "intercept":
		return intercept(d, e, in, stdout, stderr)
	case "session_start", "subagent_start":
		return provision(d, e, purpose, in.HookEventName, stdout, stderr)
	case "session_end", "subagent_stop":
		if config.Has(e.Cfg.DestroyOn, purpose) {
			if err := e.Down(); err != nil {
				fmt.Fprintf(stderr, "boxer: reclaim failed: %v\n", err)
			}
		}
		return 0
	}
	return 0
}

func intercept(d Dialect, e *box.Env, in Input, stdout, stderr io.Writer) int {
	if in.ToolName != d.ShellTool && (d.ShellTool2 == "" || in.ToolName != d.ShellTool2) {
		return 0
	}
	args := toolArgs(d, in)
	cmd, _ := args["command"].(string)
	dec := decide.Decide(decide.Input{Command: cmd, Mode: e.Cfg.Mode, Intercept: e.Cfg.Intercept, Passthrough: e.Cfg.Passthrough})
	if e.Cfg.Enforcement == "audit" && dec.Action != decide.Allow {
		fmt.Fprintf(stderr, "boxer: audit: would %s: %s\n", []string{"allow", "rewrite", "block"}[dec.Action], cmd)
		return 0
	}
	switch dec.Action {
	case decide.Allow:
		return 0
	case decide.Rewrite:
		if !d.Rewrite {
			// Block-only harness: shims on PATH already make the bare command sandboxed.
			if e.Cfg.Enforcement == "shim" || e.Cfg.Enforcement == "both" {
				return 0
			}
			return deny(d, stdout, "this repository runs commands in a sandbox", withIdentity(dec.Command, e))
		}
		return rewrite(d, stdout, withIdentity(dec.Command, e), args)
	default:
		return deny(d, stdout, dec.Reason, withIdentity(dec.Fix, e))
	}
}

// withIdentity threads the session and agent ids the hook received into the `boxer run` the
// shell will execute, so that run resolves the same session or subagent scope the hook did.
func withIdentity(cmd string, e *box.Env) string {
	flags := e.IdentityArgs()
	if len(flags) == 0 || !strings.HasPrefix(cmd, "boxer run ") {
		return cmd
	}
	return "boxer run " + strings.Join(flags, " ") + " " + strings.TrimPrefix(cmd, "boxer run ")
}

func provision(d Dialect, e *box.Env, purpose, event string, stdout, stderr io.Writer) int {
	ctx := box.InstructionsFor(e.Cfg, d.RunToolHint)
	for _, w := range e.Warnings {
		ctx += "\nNote: " + w + "."
	}
	if purpose == "session_start" && e.Cfg.WarmOnSessionStart {
		// Provision in a detached `boxer up`; the first run waits on the per-scope lock if it
		// arrives before the create finishes, and nothing here blocks the session.
		if err := e.UpDetached(); err != nil {
			fmt.Fprintf(stderr, "boxer: warm-up did not start: %v\n", err)
		}
	} else if config.Has(e.Cfg.CreateOn, purpose) {
		if _, err := e.Ensure(true, false); err != nil {
			fmt.Fprintln(stderr, err)
			ctx += "\nThe sandbox is not available yet; the `boxer:` message above says why."
		}
	}
	if purpose == "session_start" {
		return context(d, stdout, event, ctx)
	}
	return 0
}

// --- output dialects -------------------------------------------------------------------------

// rewrite replaces the command and keeps every other field of the original input: the rewritten
// input replaces the tool's input wholesale, and a schema with other required fields (Grok's
// run_terminal_command needs description) would otherwise reject it.
func rewrite(d Dialect, w io.Writer, cmd string, original map[string]any) int {
	obs.Emit(obs.Event{Name: obs.Rewrite, Harness: d.Name, Outcome: obs.OK, Payload: map[string]any{"tool": d.ShellTool, "command": cmd}})
	input := map[string]any{}
	for k, v := range original {
		input[k] = v
	}
	input["command"] = cmd
	switch d.Family {
	case "opencode":
		return emit(w, map[string]any{"command": cmd})
	case "copilot":
		// modifiedArgs replaces the tool arguments; permissionDecision must still be allow.
		return emit(w, map[string]any{"permissionDecision": "allow", "modifiedArgs": input})
	case "gemini":
		return emit(w, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "BeforeTool", "tool_input": input}})
	default:
		return emit(w, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse", "permissionDecision": "allow", "updatedInput": input}})
	}
}

func deny(d Dialect, w io.Writer, reason, fix string) int {
	obs.Emit(obs.Event{Name: obs.Deny, Harness: d.Name, Outcome: obs.Denied, Payload: map[string]any{"tool": d.ShellTool, "reason": reason}})
	msg := reason
	if fix != "" {
		msg += "\nfix: " + fix
	}
	switch d.Family {
	case "opencode":
		return emit(w, map[string]any{"deny": msg})
	case "copilot":
		return emit(w, map[string]any{"permissionDecision": "deny", "permissionDecisionReason": msg})
	case "gemini":
		return emit(w, map[string]any{"decision": "deny", "reason": msg})
	default:
		return emit(w, map[string]any{"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": msg}})
	}
}

// context hands the session brief back. The event name is echoed rather than fixed at
// "SessionStart": a harness that provisions on another event (DSH uses UserPromptSubmit) has its
// hook-specific fields discarded when the name does not match the event that fired.
func context(d Dialect, w io.Writer, event, text string) int {
	if d.Family == "opencode" {
		return emit(w, map[string]any{"context": text})
	}
	out := map[string]any{"hookEventName": event, "additionalContext": text}
	if d.Family == "gemini" {
		delete(out, "hookEventName")
	}
	return emit(w, map[string]any{"hookSpecificOutput": out})
}

func emit(w io.Writer, v any) int {
	b, _ := json.Marshal(v)
	fmt.Fprintln(w, string(b))
	return 0
}

type traceWriter struct{ harness string }

func (t traceWriter) Write(p []byte) (int, error) {
	obs.Trace(t.harness, "->", string(p))
	return len(p), nil
}

func names() []string {
	var out []string
	for k := range Dialects {
		out = append(out, k)
	}
	return out
}
