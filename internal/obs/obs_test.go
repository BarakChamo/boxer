package obs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
)

func reset(t *testing.T) {
	t.Helper()
	t.Setenv(TraceEnv, "")
	t.Cleanup(func() { Configure(Config{}) })
}

// The promise on the tin: a default configuration writes nothing anywhere.
func TestDefaultWritesNothing(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	var errb bytes.Buffer
	stderrW = &errb
	t.Cleanup(func() { stderrW = os.Stderr })
	ConfigureFrom(config.Defaults().Telemetry)
	if On() {
		t.Fatal("telemetry must be off by default")
	}
	Emit(Event{Name: Run, Scope: "sb-1", Payload: map[string]any{"command": "rm -rf /"}})
	Trace("claude-code", "<-", "{}")
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 || errb.Len() != 0 {
		t.Fatalf("default configuration wrote %v and %q", entries, errb.String())
	}
}

func TestFileSinkRedactsUnlessAsked(t *testing.T) {
	reset(t)
	path := filepath.Join(t.TempDir(), "events.jsonl")
	Configure(Config{Enabled: true, Sink: "file", Path: path})
	Emit(Event{Name: Run, Scope: "sb-1", Harness: "codex", Outcome: OK, Duration: 1500 * time.Millisecond,
		Payload: map[string]any{"command": "npm test", "exit": 0}})

	got, err := Read(path, "", 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("read: %v %v", got, err)
	}
	e := got[0]
	if e.Name != Run || e.Scope != "sb-1" || e.Harness != "codex" || e.Duration != 1500*time.Millisecond {
		t.Fatalf("round trip lost a field: %+v", e)
	}
	if _, ok := e.Payload["command"]; ok {
		t.Fatalf("command must be elided by default: %v", e.Payload)
	}
	if e.Payload["command_len"] != float64(len("npm test")) {
		t.Fatalf("want the length of the dropped command: %v", e.Payload)
	}

	Configure(Config{Enabled: true, Sink: "file", Path: path, RecordCommands: true})
	Emit(Event{Name: Run, Scope: "sb-1", Payload: map[string]any{"command": "npm test"}})
	got, _ = Read(path, "sb-1", 0)
	if len(got) != 2 || got[1].Payload["command"] != "npm test" {
		t.Fatalf("record_commands must keep the command: %+v", got)
	}
}

// The time format and the duration unit are the schema; docs/events.md promises both.
func TestWireFormat(t *testing.T) {
	b, err := json.Marshal(Event{Time: time.Date(2026, 9, 18, 9, 41, 2, 183746000, time.UTC),
		Name: Deny, Scope: "sb-1", Outcome: Denied, Duration: 250 * time.Microsecond})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"time":"2026-09-18T09:41:02.183746Z"`) || !strings.Contains(string(b), `"duration_ms":0.25`) {
		t.Fatalf("wire form: %s", b)
	}
}

// BOXER_TRACE turns the file sink on whatever the configuration says, and its hook lines keep the
// format internal/eval's oracle parses. Events share the file and the oracle ignores them.
func TestTraceEnvKeepsTheLegacyFormat(t *testing.T) {
	reset(t)
	path := filepath.Join(t.TempDir(), "trace.log")
	t.Setenv(TraceEnv, path)
	ConfigureFrom(config.Defaults().Telemetry) // telemetry off: BOXER_TRACE still wins
	if !Tracing() {
		t.Fatal("BOXER_TRACE must enable the file sink")
	}
	Trace("claude-code", "<-", `{"hook_event_name":"PreToolUse"}`)
	Emit(Event{Name: Hook, Scope: "sb-1"})
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], " claude-code <- {\"hook_event_name\"") {
		t.Fatalf("trace line: %q", b)
	}
	if !strings.HasPrefix(lines[1], "{") {
		t.Fatalf("event line: %q", lines[1])
	}
	if got, _ := Read(path, "", 0); len(got) != 1 || got[0].Name != Hook {
		t.Fatalf("Read must skip the trace lines: %+v", got)
	}
}

func TestStderrSinkAndLine(t *testing.T) {
	reset(t)
	var errb bytes.Buffer
	stderrW = &errb
	t.Cleanup(func() { stderrW = os.Stderr })
	Configure(Config{Enabled: true, Sink: "stderr"})
	Emit(Event{Name: Provision, Scope: "sb-2", Outcome: OK})
	if !strings.Contains(errb.String(), `"event":"provision"`) {
		t.Fatalf("stderr sink: %q", errb.String())
	}
	line := Event{Time: time.Unix(0, 0).UTC(), Name: GC, Outcome: OK, Payload: map[string]any{"packs": 2}}.Line()
	if !strings.Contains(line, "gc") || !strings.Contains(line, "packs=2") {
		t.Fatalf("human line: %q", line)
	}
}

// enabled with no sink named means the file sink; an invalid sink is rejected by config.Validate.
func TestEnabledDefaultsToFile(t *testing.T) {
	reset(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ConfigureFrom(config.Telemetry{Enabled: true})
	if c := Current(); c.Sink != "file" || !strings.HasSuffix(c.Path, "boxer/events.jsonl") {
		t.Fatalf("resolved config: %+v", c)
	}
	cfg := config.Defaults()
	cfg.Telemetry.Sink = "nowhere"
	if cfg.Validate() == nil {
		t.Fatal("an unknown sink must be rejected")
	}
}
