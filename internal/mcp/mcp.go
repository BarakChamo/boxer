// Package mcp is a minimal stdio MCP server exposing boxer_run and boxer_status (R-PKG-3).
// It speaks newline-delimited JSON-RPC 2.0 and nothing beyond what those two tools need.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/scope"
)

const protocolVersion = "2025-06-18"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server serves one client over r/w.
type Server struct {
	Harness string
	Resolve func(cwd, harness string, id scope.Identity) (*box.Env, error)
	Version string
}

var tools = []map[string]any{
	{
		"name":        "boxer_run",
		"description": "Run a shell command inside this repository's boxer sandbox (a microVM with the worktree mounted). Use this instead of the shell tool for build, test, install, and script commands. Returns stdout, stderr, and the exit code.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "POSIX shell command line, executed with sh -c"},
				"cwd":     map[string]any{"type": "string", "description": "Host directory to run in; defaults to the repository root"},
			},
			"required": []string{"command"},
		},
	},
	{
		"name":        "boxer_status",
		"description": "Report the sandbox for the current scope: whether it exists, is running, its image, and the mount path.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"cwd": map[string]any{"type": "string"}}},
	},
}

// Serve runs until r is exhausted.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if req.ID == nil { // notification
			continue
		}
		res := s.handle(req)
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(req request) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "boxer", "version": s.Version},
		}
	case "ping":
		res.Result = map[string]any{}
	case "tools/list":
		res.Result = map[string]any{"tools": tools}
	case "tools/call":
		var p struct {
			Name string         `json:"name"`
			Args map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			res.Error = &rpcError{-32602, "invalid params"}
			return res
		}
		text, isErr := s.call(p.Name, p.Args)
		res.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
	default:
		res.Error = &rpcError{-32601, "method not found: " + req.Method}
	}
	return res
}

func (s *Server) call(name string, args map[string]any) (string, bool) {
	cwd, _ := args["cwd"].(string)
	e, err := s.Resolve(cwd, s.Harness, scope.Identity{})
	if err != nil {
		return err.Error(), true
	}
	switch name {
	case "boxer_status":
		m, ok, err := e.Exists()
		if err != nil {
			return err.Error(), true
		}
		img, why := e.Image()
		if !ok {
			return fmt.Sprintf("scope %s (%s): no sandbox yet; image would be %s (%s); mount %s", e.Scope.Key, e.Scope.Isolation, img, why, e.Cfg.MountAt), false
		}
		return fmt.Sprintf("scope %s (%s): %s, image %s, worktree %s mounted at %s", e.Scope.Key, e.Scope.Isolation, m.State, m.Image, e.Scope.Root, e.Cfg.MountAt), false
	case "boxer_run":
		cmd, _ := args["command"].(string)
		if strings.TrimSpace(cmd) == "" {
			return "command is required", true
		}
		var out bytes.Buffer
		e.Stderr = &out
		code, err := e.Run([]string{"sh", "-c", cmd}, box.RunOpts{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out})
		if err != nil {
			return err.Error(), true
		}
		return fmt.Sprintf("%s\n[exit %d]", strings.TrimRight(out.String(), "\n"), code), code != 0
	default:
		return "unknown tool " + name, true
	}
}
