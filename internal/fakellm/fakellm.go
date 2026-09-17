// Package fakellm is a scripted model endpoint. It speaks the Anthropic Messages API and the
// OpenAI Chat Completions and Responses APIs, streaming or not, and plays one scenario: issue the
// scripted shell commands one per turn through whatever shell tool the harness offers, then answer.
// With it, a harness's hooks, plugins, and boxer run end to end with no model, no key, and the
// same result every time.
package fakellm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Scenario is what the fake model does.
type Scenario struct {
	// Commands are issued in order, one tool call per turn.
	Commands []string `json:"commands"`
	// Tools is the preference order for the tool to call. A name is matched exactly, or as a
	// substring when it starts with "~" (so "~boxer_run" matches "mcp__plugin_boxer_boxer__boxer_run").
	Tools []string `json:"tools"`
	// Answer is the final text. "{{os}}" is Linux or Darwin when the last tool result names one,
	// else its first word; "{{first_word}}" and "{{last_result}}" refer to the last tool result.
	Answer string `json:"answer"`
}

// DefaultTools covers the shell tool names of every harness in the plan.
var DefaultTools = []string{"Bash", "bash", "shell", "run_shell_command", "run_terminal_command", "exec_command", "execute_bash", "execute_command", "terminal", "exec", "~boxer_run"}

// Request is one recorded model call.
type Request struct {
	Path       string         `json:"path"`
	API        string         `json:"api"` // anthropic | openai-chat | openai-responses
	Turn       int            `json:"turn"`
	Stream     bool           `json:"stream"`
	Offered    []string       `json:"offered"`           // tool names the harness offered
	Schemas    map[string]any `json:"schemas,omitempty"` // raw parameter schema per offered tool
	Chose      string         `json:"chose"`             // tool called, or "" for the final answer
	ArgKey     string         `json:"arg_key"`           // argument that carried the command
	Command    string         `json:"command"`
	LastResult string         `json:"last_result"`
	At         time.Time      `json:"at"`
}

// tool is a harness-offered tool with the names of its arguments.
type tool struct {
	name     string
	args     []string
	required []string
	props    map[string]any
	schema   any
	ns       string // Responses "namespace" the tool belongs to, when any
}

// arguments builds the call arguments: the command under argKey, plus a placeholder for every
// other required argument (Grok's run_terminal_command insists on a description).
func (t tool) arguments(command string) map[string]any {
	args := map[string]any{t.argKey(): command}
	for _, r := range t.required {
		if _, done := args[r]; done {
			continue
		}
		args[r] = fromSchema(t.props[r])
		if s, ok := args[r].(string); ok && s == "ok" {
			args[r] = "boxer eval"
		}
	}
	return args
}

// argKey finds the argument that carries the shell line: command, then cmd, then the first one.
func (t tool) argKey() string {
	for _, want := range []string{"command", "cmd", "script", "input"} {
		for _, a := range t.args {
			if a == want {
				return a
			}
		}
	}
	if len(t.args) > 0 {
		return t.args[0]
	}
	return "command"
}

func toolFromSchema(name string, schema any) tool {
	t := tool{name: name, schema: schema}
	if m, ok := schema.(map[string]any); ok {
		if props, ok := m["properties"].(map[string]any); ok {
			t.props = props
			for k := range props {
				t.args = append(t.args, k)
			}
			sort.Strings(t.args)
		}
		if req, ok := m["required"].([]any); ok {
			for _, r := range req {
				if rs, ok := r.(string); ok {
					t.required = append(t.required, rs)
				}
			}
		}
	}
	return t
}

func names(ts []tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.name
	}
	return out
}

func schemas(ts []tool) map[string]any {
	out := map[string]any{}
	for _, t := range ts {
		out[t.name] = t.schema
	}
	return out
}

// Server serves one scenario and records what it saw.
type Server struct {
	Scenario Scenario
	mu       sync.Mutex
	requests []Request
}

// New returns a server for the scenario with defaults filled.
func New(s Scenario) *Server {
	if len(s.Tools) == 0 {
		s.Tools = DefaultTools
	}
	if s.Answer == "" {
		s.Answer = "{{os}}"
	}
	return &Server{Scenario: s}
}

// Requests returns a copy of everything recorded so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

func (s *Server) record(r Request) {
	r.At = time.Now()
	s.mu.Lock()
	s.requests = append(s.requests, r)
	s.mu.Unlock()
}

// Handler routes the three APIs plus the small endpoints CLIs probe at startup.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/messages/count_tokens"):
			writeJSON(w, map[string]any{"input_tokens": 12})
		case strings.HasSuffix(p, "/messages"):
			s.anthropic(w, r)
		case strings.HasSuffix(p, "/chat/completions"):
			s.openaiChat(w, r)
		case strings.HasSuffix(p, "/responses"):
			s.openaiResponses(w, r)
		case strings.Contains(p, ":streamGenerateContent"):
			s.gemini(w, r, true)
		case strings.Contains(p, ":generateContent"):
			s.gemini(w, r, false)
		case strings.HasSuffix(p, "/models"):
			writeJSON(w, map[string]any{"object": "list", "data": []map[string]any{
				{"id": "fake-model", "object": "model", "owned_by": "fakellm", "created": 0, "display_name": "fake-model", "type": "model"},
			}})
		default:
			// Anything else a CLI probes (telemetry, feature flags) gets an empty success.
			writeJSON(w, map[string]any{})
		}
	})
	return mux
}

// step decides the turn from how many tool results the conversation already holds.
type step struct {
	turn    int
	command string // "" means final answer
	tool    string
	ns      string
	argKey  string
	text    string
	def     tool
}

// args are the call arguments for the chosen tool.
func (st step) args() map[string]any { return st.def.arguments(st.command) }

func (s *Server) step(offered []tool, toolResults int, lastResult string) step {
	if toolResults < len(s.Scenario.Commands) {
		t, ok := s.pick(offered)
		if !ok {
			// A side request (title generation, summaries) that offers no shell tool: answer briefly.
			return step{turn: toolResults, text: "ok"}
		}
		return step{turn: toolResults, command: s.Scenario.Commands[toolResults], tool: t.name, ns: t.ns, argKey: t.argKey(), def: t}
	}
	ans := strings.ReplaceAll(s.Scenario.Answer, "{{last_result}}", strings.TrimSpace(lastResult))
	first := ""
	if f := strings.Fields(lastResult); len(f) > 0 {
		first = f[0]
	}
	ans = strings.ReplaceAll(ans, "{{first_word}}", first)
	osWord := first
	for _, w := range []string{"Linux", "Darwin"} {
		if strings.Contains(lastResult, w) {
			osWord = w
			break
		}
	}
	ans = strings.ReplaceAll(ans, "{{os}}", osWord)
	return step{turn: toolResults, text: ans}
}

func (s *Server) pick(offered []tool) (tool, bool) {
	for _, want := range s.Scenario.Tools {
		for _, have := range offered {
			if strings.HasPrefix(want, "~") && strings.Contains(have.name, want[1:]) {
				return have, true
			}
			if have.name == want {
				return have, true
			}
		}
	}
	return tool{}, false
}

// --- Anthropic Messages -------------------------------------------------------------------------

func (s *Server) anthropic(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string           `json:"model"`
		Stream   bool             `json:"stream"`
		Tools    []map[string]any `json:"tools"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var offered []tool
	for _, t := range req.Tools {
		if n, ok := t["name"].(string); ok {
			offered = append(offered, toolFromSchema(n, t["input_schema"]))
		}
	}
	results, last := 0, ""
	for _, m := range req.Messages {
		var blocks []map[string]any
		if json.Unmarshal(m.Content, &blocks) == nil {
			for _, b := range blocks {
				if b["type"] == "tool_result" {
					results++
					last = blockText(b["content"])
				}
			}
		}
	}
	st := s.step(offered, results, last)
	s.record(Request{Path: r.URL.Path, API: "anthropic", Turn: st.turn, Stream: req.Stream, Offered: names(offered), Schemas: schemas(offered), Chose: st.tool, ArgKey: st.argKey, Command: st.command, LastResult: last})

	id := fmt.Sprintf("msg_fake_%d", time.Now().UnixNano())
	var content []map[string]any
	stop := "end_turn"
	if st.command != "" {
		content = []map[string]any{{"type": "tool_use", "id": fmt.Sprintf("toolu_fake_%d", st.turn), "name": st.tool, "input": st.args()}}
		stop = "tool_use"
	} else {
		content = []map[string]any{{"type": "text", "text": st.text}}
	}
	usage := map[string]any{"input_tokens": 10, "output_tokens": 5, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
	if !req.Stream {
		writeJSON(w, map[string]any{"id": id, "type": "message", "role": "assistant", "model": req.Model, "content": content, "stop_reason": stop, "stop_sequence": nil, "usage": usage})
		return
	}
	sse := newSSE(w)
	sse.event("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": id, "type": "message", "role": "assistant", "model": req.Model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}})
	if st.command != "" {
		sse.event("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": content[0]["id"], "name": st.tool, "input": map[string]any{}}})
		args, _ := json.Marshal(st.args())
		sse.event("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(args)}})
	} else {
		sse.event("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
		sse.event("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": st.text}})
	}
	sse.event("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	sse.event("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 5}})
	sse.event("message_stop", map[string]any{"type": "message_stop"})
}

func blockText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		var b strings.Builder
		for _, x := range c {
			if m, ok := x.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	}
	return ""
}

// --- OpenAI Chat Completions -------------------------------------------------------------------

func (s *Server) openaiChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
		Tools  []struct {
			Function struct {
				Name       string `json:"name"`
				Parameters any    `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var offered []tool
	for _, t := range req.Tools {
		offered = append(offered, toolFromSchema(t.Function.Name, t.Function.Parameters))
	}
	results, last := 0, ""
	for _, m := range req.Messages {
		if m.Role == "tool" {
			results++
			var str string
			if json.Unmarshal(m.Content, &str) == nil {
				last = str
			} else {
				var blocks []map[string]any
				if json.Unmarshal(m.Content, &blocks) == nil {
					last = blockText(anySlice(blocks))
				}
			}
		}
	}
	st := s.step(offered, results, last)
	s.record(Request{Path: r.URL.Path, API: "openai-chat", Turn: st.turn, Stream: req.Stream, Offered: names(offered), Schemas: schemas(offered), Chose: st.tool, ArgKey: st.argKey, Command: st.command, LastResult: last})

	id := fmt.Sprintf("chatcmpl_fake_%d", time.Now().UnixNano())
	created := time.Now().Unix()
	var message map[string]any
	finish := "stop"
	var toolCalls []map[string]any
	if st.command != "" {
		args, _ := json.Marshal(st.args())
		toolCalls = []map[string]any{{"id": fmt.Sprintf("call_fake_%d", st.turn), "type": "function", "function": map[string]any{"name": st.tool, "arguments": string(args)}}}
		message = map[string]any{"role": "assistant", "content": nil, "tool_calls": toolCalls}
		finish = "tool_calls"
	} else {
		message = map[string]any{"role": "assistant", "content": st.text}
	}
	if !req.Stream {
		writeJSON(w, map[string]any{"id": id, "object": "chat.completion", "created": created, "model": req.Model,
			"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})
		return
	}
	sse := newSSE(w)
	chunk := func(delta map[string]any, finish any) {
		sse.data(map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": req.Model,
			"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}})
	}
	chunk(map[string]any{"role": "assistant", "content": ""}, nil)
	if st.command != "" {
		tc := toolCalls[0]
		chunk(map[string]any{"tool_calls": []map[string]any{{"index": 0, "id": tc["id"], "type": "function", "function": map[string]any{"name": st.tool, "arguments": ""}}}}, nil)
		chunk(map[string]any{"tool_calls": []map[string]any{{"index": 0, "function": map[string]any{"arguments": tc["function"].(map[string]any)["arguments"]}}}}, nil)
	} else {
		chunk(map[string]any{"content": st.text}, nil)
	}
	chunk(map[string]any{}, finish)
	sse.raw("[DONE]")
}

func anySlice(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, m := range in {
		out[i] = m
	}
	return out
}

// --- OpenAI Responses ---------------------------------------------------------------------------

func (s *Server) openaiResponses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model  string            `json:"model"`
		Stream bool              `json:"stream"`
		Tools  []json.RawMessage `json:"tools"`
		Input  json.RawMessage   `json:"input"`
	}
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var offered []tool
	for _, raw := range req.Tools {
		var t map[string]any
		if json.Unmarshal(raw, &t) != nil {
			continue
		}
		name, _ := t["name"].(string)
		if name == "" {
			continue
		}
		if t["type"] == "namespace" {
			// Codex groups an MCP server's tools under one namespace; the model calls
			// "<namespace>__<tool>" (Codex's MCP naming). Offer each inner tool that way.
			if inner, ok := t["tools"].([]any); ok {
				for _, it := range inner {
					if im, ok := it.(map[string]any); ok {
						if n, _ := im["name"].(string); n != "" {
							tl := toolFromSchema(namespacedName(name, n), im["parameters"])
							tl.ns = name
							offered = append(offered, tl)
						}
					}
				}
			}
			continue
		}
		tl := toolFromSchema(name, t["parameters"])
		if tl.schema == nil {
			tl.schema = t // no parameters: keep the whole definition for inspection
		}
		offered = append(offered, tl)
	}
	results, last := 0, ""
	var items []map[string]any
	if json.Unmarshal(req.Input, &items) == nil {
		for _, it := range items {
			if it["type"] == "function_call_output" {
				results++
				if o, ok := it["output"].(string); ok {
					last = o
				} else {
					last = blockText(it["output"])
				}
			}
		}
	}
	st := s.step(offered, results, last)
	s.record(Request{Path: r.URL.Path, API: "openai-responses", Turn: st.turn, Stream: req.Stream, Offered: names(offered), Schemas: schemas(offered), Chose: st.tool, ArgKey: st.argKey, Command: st.command, LastResult: last})

	id := fmt.Sprintf("resp_fake_%d", time.Now().UnixNano())
	var item map[string]any
	if st.command != "" {
		args, _ := json.Marshal(st.args())
		item = map[string]any{"type": "function_call", "id": fmt.Sprintf("fc_fake_%d", st.turn), "call_id": fmt.Sprintf("call_fake_%d", st.turn), "name": st.tool, "arguments": string(args), "status": "completed"}
		if st.ns != "" {
			item["namespace"] = st.ns
		}
	} else {
		item = map[string]any{"type": "message", "id": fmt.Sprintf("msg_fake_%d", st.turn), "role": "assistant", "status": "completed",
			"content": []map[string]any{{"type": "output_text", "text": st.text, "annotations": []any{}}}}
	}
	resp := map[string]any{"id": id, "object": "response", "created_at": time.Now().Unix(), "model": req.Model, "status": "completed",
		"output": []map[string]any{item}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}
	if !req.Stream {
		writeJSON(w, resp)
		return
	}
	sse := newSSE(w)
	seq := 0
	ev := func(typ string, body map[string]any) {
		body["type"] = typ
		body["sequence_number"] = seq
		seq++
		sse.event(typ, body)
	}
	inProgress := map[string]any{}
	for k, v := range resp {
		inProgress[k] = v
	}
	inProgress["status"] = "in_progress"
	inProgress["output"] = []any{}
	ev("response.created", map[string]any{"response": inProgress})
	ev("response.in_progress", map[string]any{"response": inProgress})
	added := map[string]any{}
	for k, v := range item {
		added[k] = v
	}
	if st.command != "" {
		added["arguments"] = ""
		added["status"] = "in_progress"
		ev("response.output_item.added", map[string]any{"output_index": 0, "item": added})
		ev("response.function_call_arguments.delta", map[string]any{"output_index": 0, "item_id": item["id"], "delta": item["arguments"]})
		ev("response.function_call_arguments.done", map[string]any{"output_index": 0, "item_id": item["id"], "arguments": item["arguments"]})
	} else {
		added["content"] = []any{}
		added["status"] = "in_progress"
		ev("response.output_item.added", map[string]any{"output_index": 0, "item": added})
		ev("response.content_part.added", map[string]any{"output_index": 0, "item_id": item["id"], "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
		ev("response.output_text.delta", map[string]any{"output_index": 0, "item_id": item["id"], "content_index": 0, "delta": st.text})
		ev("response.output_text.done", map[string]any{"output_index": 0, "item_id": item["id"], "content_index": 0, "text": st.text})
		ev("response.content_part.done", map[string]any{"output_index": 0, "item_id": item["id"], "content_index": 0, "part": map[string]any{"type": "output_text", "text": st.text, "annotations": []any{}}})
	}
	ev("response.output_item.done", map[string]any{"output_index": 0, "item": item})
	ev("response.completed", map[string]any{"response": resp})
}

// namespacedName is how a Responses "namespace" tool's inner function is called back: by its bare
// name, with the namespace carried in the function_call item's "namespace" field. Verified against
// Codex 0.154, which rejects ns__tool and ns.tool as "unsupported call".
func namespacedName(ns, tool string) string {
	return tool
}

// --- plumbing -----------------------------------------------------------------------------------

func decode(r *http.Request, v any) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type sse struct {
	w http.ResponseWriter
	f http.Flusher
}

func newSSE(w http.ResponseWriter) *sse {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	f, _ := w.(http.Flusher)
	return &sse{w: w, f: f}
}

func (s *sse) event(name string, v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", name, b)
	s.flush()
}

func (s *sse) data(v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "data: %s\n\n", b)
	s.flush()
}

func (s *sse) raw(line string) {
	fmt.Fprintf(s.w, "data: %s\n\n", line)
	s.flush()
}

func (s *sse) flush() {
	if s.f != nil {
		s.f.Flush()
	}
}
