// Package obs is boxer's event stream: one event type, one configured sink, off by default.
//
// It exists because a sandbox is invisible. Everything else boxer does leaves a trace someone can
// read — a machine in `smolvm ls`, a file in the worktree — but the decision to rewrite a command,
// the pull that took forty seconds, the setup step that failed, all happened inside a hook that
// printed nothing. This package is where those become facts a dashboard, an eval or a support
// request can read back.
//
// Redaction is by construction rather than by discipline: payload keys that can carry a command
// line or an environment value are dropped on the way to the sink unless the operator has asked
// for them, so a caller cannot leak one by forgetting to. Nothing is sent off the machine unless
// an endpoint is configured, and the OpenTelemetry sink exists only in a binary built with the
// `otel` tag.
package obs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BarakChamo/boxer/internal/config"
)

// Event names. Every emitted event uses one of these.
const (
	Resolve   = "resolve"
	Provision = "provision"
	Pack      = "pack"
	Run       = "run"
	Rewrite   = "rewrite"
	Deny      = "deny"
	Hook      = "hook"
	MCP       = "mcp"
	GC        = "gc"
	Err       = "error"
)

// Outcomes. Free-form is allowed, but these are what the documented schema promises.
const (
	OK      = "ok"
	Failed  = "error"
	Denied  = "denied"
	Skipped = "skipped"
)

// Event is one thing that happened. The schema is documented in docs/events.md and is part of
// boxer's stable surface: fields are added, never repurposed.
type Event struct {
	Time     time.Time      `json:"time"`
	Name     string         `json:"event"`
	Scope    string         `json:"scope,omitempty"`
	Harness  string         `json:"harness,omitempty"`
	Duration time.Duration  `json:"-"`
	Outcome  string         `json:"outcome,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

type wireEvent struct {
	Time       string         `json:"time"`
	Name       string         `json:"event"`
	Scope      string         `json:"scope,omitempty"`
	Harness    string         `json:"harness,omitempty"`
	DurationMS float64        `json:"duration_ms,omitempty"`
	Outcome    string         `json:"outcome,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// MarshalJSON writes the documented wire form: RFC3339Nano time, duration in milliseconds.
func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(wireEvent{
		Time: e.Time.UTC().Format(time.RFC3339Nano), Name: e.Name, Scope: e.Scope, Harness: e.Harness,
		DurationMS: float64(e.Duration) / float64(time.Millisecond), Outcome: e.Outcome, Payload: e.Payload,
	})
}

// UnmarshalJSON reads the wire form back, so `boxer logs` and `boxer status` share one schema.
func (e *Event) UnmarshalJSON(b []byte) error {
	var w wireEvent
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339Nano, w.Time)
	if err != nil {
		return err
	}
	*e = Event{Time: t, Name: w.Name, Scope: w.Scope, Harness: w.Harness,
		Duration: time.Duration(w.DurationMS * float64(time.Millisecond)), Outcome: w.Outcome, Payload: w.Payload}
	return nil
}

// Config is the resolved [telemetry] table.
type Config struct {
	Enabled        bool
	Sink           string // none | file | stderr | otel
	Path           string // file sink
	RecordCommands bool
	Endpoint       string // otel sink; without it nothing leaves the machine
}

// TraceEnv is the long-standing variable that turns the file sink on at a chosen path. The eval
// suite's oracle parses that file, so its hook lines keep their historical format.
const TraceEnv = "BOXER_TRACE"

var (
	mu      sync.Mutex
	cfg     Config
	stderrW io.Writer = os.Stderr
)

// Configure installs the configuration for this process. BOXER_TRACE wins: a harness that can
// only set an environment variable still gets a file sink.
func Configure(c Config) {
	if c.Sink == "" {
		c.Sink = "none"
	}
	// enabled with no sink named is the common mistake; the file sink is what the operator meant.
	if c.Enabled && c.Sink == "none" {
		c.Sink = "file"
	}
	if trace := os.Getenv(TraceEnv); trace != "" {
		c.Enabled, c.Sink, c.Path = true, "file", trace
	}
	if c.Enabled && c.Sink == "file" && c.Path == "" {
		c.Path = DefaultPath()
	}
	mu.Lock()
	cfg = c
	mu.Unlock()
}

// ConfigureFrom installs the [telemetry] table of a resolved configuration.
func ConfigureFrom(t config.Telemetry) {
	Configure(Config{Enabled: t.Enabled, Sink: t.Sink, Path: t.Path, RecordCommands: t.RecordCommands, Endpoint: t.Endpoint})
}

// DefaultPath is the file sink's path when none is configured, beside the other host state.
func DefaultPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "boxer", "events.jsonl")
}

// Current returns the configuration in force, for `doctor` and for tests.
func Current() Config {
	mu.Lock()
	defer mu.Unlock()
	return cfg
}

// On reports whether anything would be recorded, so callers can skip the work of building a payload.
func On() bool {
	c := Current()
	return c.Enabled && c.Sink != "none" && c.Sink != ""
}

// sensitive payload keys are elided unless record_commands is set: everything that can carry a
// command line the model composed, or a value from the operator's environment.
var sensitive = map[string]bool{"command": true, "argv": true, "env": true, "setup": true}

// Emit records one event. It never fails a caller: a sink that cannot be written is silently
// dropped, because telemetry must not be able to break a sandbox.
func Emit(e Event) {
	c := Current()
	if !c.Enabled || c.Sink == "none" || c.Sink == "" {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	e.Payload = redact(e.Payload, c.RecordCommands)
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	switch c.Sink {
	case "file":
		appendLine(c.Path, string(b)+"\n")
	case "stderr":
		fmt.Fprintln(stderrW, string(b))
	case "otel":
		if otelSink != nil {
			otelSink(c, e)
		}
	}
}

// redact returns a copy with the sensitive keys removed, replaced by their length so a reader can
// still tell a long command from an empty one. The original map is never modified.
func redact(p map[string]any, record bool) map[string]any {
	if len(p) == 0 {
		return nil
	}
	out := make(map[string]any, len(p))
	for k, v := range p {
		if sensitive[k] && !record {
			if s, ok := v.(string); ok {
				out[k+"_len"] = len(s)
			} else {
				out[k+"_len"] = 0
			}
			continue
		}
		out[k] = v
	}
	return out
}

// Trace writes one hook line in the historical BOXER_TRACE format ("<stamp> <harness> <- <json>").
// internal/eval's oracle parses these lines; the JSON events share the file and are ignored by it,
// because it reads only lines carrying an arrow.
func Trace(harness, arrow, text string) {
	c := Current()
	if !c.Enabled || c.Sink != "file" || c.Path == "" {
		return
	}
	appendLine(c.Path, fmt.Sprintf("%s %s %s %s\n", time.Now().UTC().Format(time.RFC3339Nano), harness, arrow, strings.TrimRight(text, "\n")))
}

// Tracing reports whether Trace would write, so the hook can avoid teeing its output for nothing.
func Tracing() bool {
	c := Current()
	return c.Enabled && c.Sink == "file" && c.Path != ""
}

func appendLine(path, line string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = io.WriteString(f, line)
	_ = f.Close()
}

// otelSink is installed by the otel build tag; nil in the default binary, which therefore carries
// no exporter and no network code for telemetry at all.
var otelSink func(Config, Event)

// Read returns the events in the file sink, oldest first, optionally filtered by scope and
// limited to the last n. Lines that are not events (the hook trace shares the file) are skipped.
func Read(path, scope string, n int) ([]Event, error) {
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var e Event
		if json.Unmarshal([]byte(line), &e) != nil || e.Name == "" {
			continue
		}
		if scope != "" && e.Scope != scope {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return out, nil
}

// Line is one event as `boxer logs` prints it without --json.
func (e Event) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %-9s", e.Time.UTC().Format(time.RFC3339), e.Name)
	if e.Scope != "" {
		fmt.Fprintf(&b, " %s", e.Scope)
	}
	if e.Harness != "" {
		fmt.Fprintf(&b, " [%s]", e.Harness)
	}
	if e.Outcome != "" {
		fmt.Fprintf(&b, " %s", e.Outcome)
	}
	if e.Duration > 0 {
		fmt.Fprintf(&b, " %dms", e.Duration.Milliseconds())
	}
	for _, k := range sortedKeys(e.Payload) {
		fmt.Fprintf(&b, " %s=%v", k, e.Payload[k])
	}
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
