package fakellm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, srv *httptest.Server, path string, body any) (*http.Response, []byte) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

func TestAnthropicTwoTurns(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	tools := []map[string]any{{"name": "Read"}, {"name": "Bash"}}
	_, body := post(t, srv, "/v1/messages", map[string]any{"model": "m", "tools": tools,
		"messages": []map[string]any{{"role": "user", "content": "run it"}}})
	var m map[string]any
	json.Unmarshal(body, &m)
	c := m["content"].([]any)[0].(map[string]any)
	if m["stop_reason"] != "tool_use" || c["name"] != "Bash" || c["input"].(map[string]any)["command"] != "uname -a" {
		t.Fatalf("turn 1: %s", body)
	}
	_, body = post(t, srv, "/v1/messages", map[string]any{"model": "m", "tools": tools, "messages": []map[string]any{
		{"role": "user", "content": "run it"},
		{"role": "assistant", "content": []map[string]any{c}},
		{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": c["id"], "content": "Linux sb-abc 6.1 aarch64\n"}}},
	}})
	json.Unmarshal(body, &m)
	txt := m["content"].([]any)[0].(map[string]any)["text"]
	if m["stop_reason"] != "end_turn" || txt != "Linux" {
		t.Fatalf("turn 2: %s", body)
	}
	reqs := s.Requests()
	if len(reqs) != 2 || reqs[0].Chose != "Bash" || reqs[1].Chose != "" || reqs[1].LastResult == "" {
		t.Fatalf("recorded: %+v", reqs)
	}
}

func TestAnthropicStreamShape(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	resp, body := post(t, srv, "/v1/messages?beta=true", map[string]any{"model": "m", "stream": true,
		"tools": []map[string]any{{"name": "Bash"}}, "messages": []map[string]any{{"role": "user", "content": "x"}}})
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatal(ct)
	}
	var events []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "event: ") {
			events = append(events, strings.TrimPrefix(sc.Text(), "event: "))
		}
	}
	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	if strings.Join(events, ",") != strings.Join(want, ",") {
		t.Fatalf("events %v", events)
	}
	if !strings.Contains(string(body), `"partial_json":"{\"command\":\"uname -a\"}"`) {
		t.Fatalf("args delta missing:\n%s", body)
	}
}

func TestToolPreferenceBoxerRun(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}, Tools: []string{"~boxer_run", "Bash"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	_, body := post(t, srv, "/v1/messages", map[string]any{"model": "m",
		"tools":    []map[string]any{{"name": "Bash"}, {"name": "mcp__plugin_boxer_boxer__boxer_run"}},
		"messages": []map[string]any{{"role": "user", "content": "x"}}})
	if !strings.Contains(string(body), `"name":"mcp__plugin_boxer_boxer__boxer_run"`) {
		t.Fatalf("should prefer boxer_run: %s", body)
	}
}

func TestOpenAIChat(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	tools := []map[string]any{{"type": "function", "function": map[string]any{"name": "shell"}}}
	_, body := post(t, srv, "/v1/chat/completions", map[string]any{"model": "m", "tools": tools,
		"messages": []map[string]any{{"role": "user", "content": "x"}}})
	var m map[string]any
	json.Unmarshal(body, &m)
	ch := m["choices"].([]any)[0].(map[string]any)
	tc := ch["message"].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
	if ch["finish_reason"] != "tool_calls" || tc["function"].(map[string]any)["name"] != "shell" {
		t.Fatalf("chat turn 1: %s", body)
	}
	_, body = post(t, srv, "/v1/chat/completions", map[string]any{"model": "m", "tools": tools, "stream": true,
		"messages": []map[string]any{{"role": "user", "content": "x"}, {"role": "assistant", "tool_calls": []any{tc}}, {"role": "tool", "tool_call_id": tc["id"], "content": "Linux sb-1 x"}}})
	if !strings.Contains(string(body), `"content":"Linux"`) || !strings.HasSuffix(strings.TrimSpace(string(body)), "data: [DONE]") {
		t.Fatalf("chat stream turn 2:\n%s", body)
	}
}

func TestOpenAIResponses(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	tools := []map[string]any{{"type": "function", "name": "shell"}}
	_, body := post(t, srv, "/v1/responses", map[string]any{"model": "m", "tools": tools, "stream": true,
		"input": []map[string]any{{"role": "user", "content": "x"}}})
	if !strings.Contains(string(body), "event: response.output_item.added") || !strings.Contains(string(body), `"type":"function_call"`) || !strings.Contains(string(body), "event: response.completed") {
		t.Fatalf("responses stream:\n%s", body)
	}
	_, body = post(t, srv, "/v1/responses", map[string]any{"model": "m", "tools": tools,
		"input": []map[string]any{{"role": "user", "content": "x"}, {"type": "function_call", "call_id": "c1", "name": "shell", "arguments": "{}"}, {"type": "function_call_output", "call_id": "c1", "output": "Linux sb-1"}}})
	if !strings.Contains(string(body), `"text":"Linux"`) {
		t.Fatalf("responses final: %s", body)
	}
}

func TestArgKeyFromSchemaAndSideRequests(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	// Codex-style tool: exec_command with a `cmd` argument.
	_, body := post(t, srv, "/v1/responses", map[string]any{"model": "m",
		"tools": []map[string]any{{"type": "function", "name": "exec_command", "parameters": map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}, "yield_time_ms": map[string]any{"type": "integer"}}}}},
		"input": []map[string]any{{"role": "user", "content": "x"}}})
	if !strings.Contains(string(body), `\"cmd\":\"uname -a\"`) {
		t.Fatalf("should use the cmd argument: %s", body)
	}
	// A side request offering only a non-shell tool gets text, not a tool call.
	_, body = post(t, srv, "/v1/chat/completions", map[string]any{"model": "m",
		"tools":    []map[string]any{{"type": "function", "function": map[string]any{"name": "session_title", "parameters": map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}}}}}},
		"messages": []map[string]any{{"role": "user", "content": "title this"}}})
	if strings.Contains(string(body), "tool_calls") || !strings.Contains(string(body), `"content":"ok"`) {
		t.Fatalf("side request should get text: %s", body)
	}
}

func TestResponsesNamespaceTools(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}, Tools: []string{"~boxer_run"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	ns := map[string]any{"type": "namespace", "name": "mcp__boxer", "tools": []map[string]any{{"type": "function", "name": "boxer_run", "parameters": map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}}}}}
	_, body := post(t, srv, "/v1/responses", map[string]any{"model": "m", "tools": []any{ns}, "input": []map[string]any{{"role": "user", "content": "x"}}})
	var m map[string]any
	json.Unmarshal(body, &m)
	item := m["output"].([]any)[0].(map[string]any)
	if item["name"] != "boxer_run" || item["namespace"] != "mcp__boxer" {
		t.Fatalf("namespace call shape: %s", body)
	}
}

func TestGemini(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	tools := []map[string]any{{"functionDeclarations": []map[string]any{{"name": "run_shell_command", "parameters": map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}}}}}}
	_, body := post(t, srv, "/v1beta/models/fake-model:generateContent", map[string]any{"contents": []map[string]any{{"role": "user", "parts": []map[string]any{{"text": "x"}}}}, "tools": tools})
	if !strings.Contains(string(body), `"functionCall":{"args":{"command":"uname -a"},"name":"run_shell_command"}`) {
		t.Fatalf("gemini call: %s", body)
	}
	_, body = post(t, srv, "/v1beta/models/fake-model:streamGenerateContent?alt=sse", map[string]any{"tools": tools, "contents": []map[string]any{
		{"role": "user", "parts": []map[string]any{{"text": "x"}}},
		{"role": "model", "parts": []map[string]any{{"functionCall": map[string]any{"name": "run_shell_command", "args": map[string]any{"command": "uname -a"}}}}},
		{"role": "user", "parts": []map[string]any{{"functionResponse": map[string]any{"name": "run_shell_command", "response": map[string]any{"output": "Linux sb-1 aarch64"}}}}},
	}})
	if !strings.HasPrefix(string(body), "data: ") || !strings.Contains(string(body), `"text":"Linux"`) {
		t.Fatalf("gemini stream final: %s", body)
	}
}

func TestRequiredArgumentsAreFilled(t *testing.T) {
	s := New(Scenario{Commands: []string{"uname -a"}})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	tools := []map[string]any{{"type": "function", "function": map[string]any{"name": "run_terminal_command", "parameters": map[string]any{"type": "object",
		"properties": map[string]any{"command": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "timeout": map[string]any{"type": "integer"}},
		"required":   []string{"command", "description"}}}}}
	_, body := post(t, srv, "/v1/chat/completions", map[string]any{"model": "m", "tools": tools, "messages": []map[string]any{{"role": "user", "content": "x"}}})
	if !strings.Contains(string(body), `\"description\":\"boxer eval\"`) || !strings.Contains(string(body), `\"command\":\"uname -a\"`) || strings.Contains(string(body), "timeout") {
		t.Fatalf("required args: %s", body)
	}
}

func TestProbesSucceed(t *testing.T) {
	srv := httptest.NewServer(New(Scenario{}).Handler())
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/v1/models")
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	_, body := post(t, srv, "/v1/messages/count_tokens", map[string]any{})
	if !strings.Contains(string(body), "input_tokens") {
		t.Fatal(string(body))
	}
}
