package fakellm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// gemini serves the Gemini API's generateContent and streamGenerateContent (SSE) for one model:
// contents carry text, functionCall, and functionResponse parts; tools carry functionDeclarations.
func (s *Server) gemini(w http.ResponseWriter, r *http.Request, stream bool) {
	var req struct {
		Contents []struct {
			Role  string           `json:"role"`
			Parts []map[string]any `json:"parts"`
		} `json:"contents"`
		Tools []struct {
			FunctionDeclarations []struct {
				Name       string `json:"name"`
				Parameters any    `json:"parameters"`
			} `json:"functionDeclarations"`
		} `json:"tools"`
		GenerationConfig map[string]any `json:"generationConfig"`
	}
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var offered []tool
	for _, t := range req.Tools {
		for _, fd := range t.FunctionDeclarations {
			offered = append(offered, toolFromSchema(fd.Name, fd.Parameters))
		}
	}
	results, last := 0, ""
	for _, c := range req.Contents {
		for _, p := range c.Parts {
			if fr, ok := p["functionResponse"].(map[string]any); ok {
				results++
				last = geminiResponseText(fr["response"])
			}
		}
	}
	st := s.step(offered, results, last)
	// Gemini CLI's model router asks for a JSON classification before the real turn: answer any
	// schema-constrained request with an object built from its schema, never with prose.
	if schema := jsonSchemaOf(req.GenerationConfig); schema != nil && st.command == "" {
		b, _ := json.Marshal(fromSchema(schema))
		st.text = string(b)
	}
	rec := Request{Path: r.URL.Path, API: "gemini", Turn: st.turn, Stream: stream, Offered: names(offered), Schemas: schemas(offered), Chose: st.tool, ArgKey: st.argKey, Command: st.command, LastResult: last}
	if req.GenerationConfig != nil {
		if rec.Schemas == nil {
			rec.Schemas = map[string]any{}
		}
		rec.Schemas["generationConfig"] = req.GenerationConfig
	}
	s.record(rec)

	var part map[string]any
	if st.command != "" {
		part = map[string]any{"functionCall": map[string]any{"name": st.tool, "args": st.args()}}
	} else {
		part = map[string]any{"text": st.text}
	}
	resp := map[string]any{
		"candidates": []map[string]any{{
			"content":      map[string]any{"role": "model", "parts": []map[string]any{part}},
			"finishReason": "STOP",
			"index":        0,
		}},
		"usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 5, "totalTokenCount": 15},
		"modelVersion":  "fake-model",
		"responseId":    fmt.Sprintf("fake-%d", time.Now().UnixNano()),
	}
	if !stream {
		writeJSON(w, resp)
		return
	}
	sse := newSSE(w)
	sse.data(resp)
}

// geminiResponseText flattens a functionResponse.response object into text for {{os}} matching.
func geminiResponseText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		var b strings.Builder
		for _, k := range []string{"output", "result", "content", "stdout", "error"} {
			if s, ok := x[k].(string); ok {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
		if b.Len() == 0 {
			raw, _ := json.Marshal(x)
			return string(raw)
		}
		return b.String()
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// jsonSchemaOf returns the response schema a Gemini request demands, or nil for free text.
func jsonSchemaOf(gc map[string]any) any {
	if gc == nil {
		return nil
	}
	for _, k := range []string{"responseJsonSchema", "responseSchema", "response_json_schema", "response_schema"} {
		if v, ok := gc[k]; ok && v != nil {
			return v
		}
	}
	if mt, _ := gc["responseMimeType"].(string); strings.Contains(mt, "json") {
		return map[string]any{"type": "OBJECT"}
	}
	return nil
}

// fromSchema builds the smallest value that satisfies a Gemini or JSON schema: first enum value,
// empty strings, 1 for numbers (classifiers score 1..N), false, empty arrays, nested objects.
func fromSchema(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	if enum, ok := m["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	typ, _ := m["type"].(string)
	if arr, ok := m["type"].([]any); ok && len(arr) > 0 {
		typ, _ = arr[0].(string)
	}
	switch strings.ToLower(typ) {
	case "string":
		return "ok"
	case "number", "integer":
		return 1
	case "boolean":
		return false
	case "array":
		return []any{}
	default:
		out := map[string]any{}
		if props, ok := m["properties"].(map[string]any); ok {
			for k, v := range props {
				out[k] = fromSchema(v)
			}
		}
		return out
	}
}
