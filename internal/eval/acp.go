package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// InsideACP drives `boxer acp <harness>` with a minimal Agent Client Protocol client: initialize,
// session/new, session/prompt; answers permission requests with the first allow option and serves
// fs/read_text_file and fs/write_text_file against the host worktree (the same path in the guest).
type InsideACP struct{ Inside }

func (InsideACP) Name() string { return "inside-acp" }

func (InsideACP) Cells(tier string) []Cell {
	var cells []Cell
	for _, h := range []string{"claude", "codex", "gemini", "kimi", "opencode", "grok"} {
		cells = append(cells, Cell{Harness: "acp-" + h, Mode: "inside", Entry: "acp", Isolation: "worktree", Compliant: true, Tier: tier, Inside: h})
	}
	return cells
}

func (d InsideACP) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	h := c.Inside
	cmdArgs := []string{"acp", h}
	for _, e := range d.guestEnvFor(env, h) {
		cmdArgs = append(cmdArgs, "-e", e)
	}
	cmd := exec.Command(env.Boxer, cmdArgs...)
	cmd.Dir = env.Repo
	cmd.Env = env.BaseEnv()
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	defer cmd.Process.Kill()

	cl := &acpClient{w: stdin, r: bufio.NewReaderSize(stdout, 1<<20), repo: env.Repo}
	done := make(chan error, 1)
	go func() { done <- cl.session(prompt) }()
	select {
	case err := <-done:
		tr := Transcript{Raw: cl.log.String() + "\n--- stderr ---\n" + stderr.String(), Tools: append(env.LLMTools(), cl.tools...)}
		// the answer is the last non-empty line: harnesses print warnings as chunks before it
		lines := strings.Split(strings.TrimSpace(cl.text.String()), "\n")
		if f := strings.Fields(lines[len(lines)-1]); len(f) > 0 {
			tr.Answer = f[0]
		}
		// An agent that closes its pipe says nothing useful on its own; what it printed before
		// dying is the whole diagnosis, and a cell that fails once in a hundred runs is not worth
		// re-running by hand to find out.
		if err != nil {
			if last := lastLines(stderr.String(), 3); last != "" {
				err = fmt.Errorf("%v: %s", err, last)
			}
		}
		return tr, err
	case <-time.After(15 * time.Minute):
		return Transcript{Raw: cl.log.String() + stderr.String()}, fmt.Errorf("acp %s timed out", h)
	}
}

type acpClient struct {
	w     io.Writer
	r     *bufio.Reader
	repo  string
	id    int
	text  strings.Builder
	log   strings.Builder
	tools []string
}

func (c *acpClient) send(v any) error {
	b, _ := json.Marshal(v)
	c.log.WriteString("-> " + string(b) + "\n")
	_, err := c.w.Write(append(b, '\n'))
	return err
}

// call sends a request and pumps messages until its response arrives, serving agent requests
// and collecting notifications on the way.
func (c *acpClient) call(method string, params any) (json.RawMessage, error) {
	c.id++
	id := c.id
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("agent closed: %v", err)
		}
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		c.log.WriteString("<- " + strings.TrimSpace(string(line)) + "\n")
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		switch {
		case msg.Method != "" && msg.ID != nil: // agent → client request
			c.serve(msg.ID, msg.Method, msg.Params)
		case msg.Method != "": // notification
			c.note(msg.Method, msg.Params)
		case string(msg.ID) == fmt.Sprint(id):
			if msg.Error != nil {
				return nil, fmt.Errorf("%s: %s", method, msg.Error.Message)
			}
			return msg.Result, nil
		}
	}
}

func (c *acpClient) serve(id json.RawMessage, method string, params json.RawMessage) {
	var p map[string]any
	json.Unmarshal(params, &p)
	reply := func(result any) { c.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}) }
	switch method {
	case "session/request_permission":
		opts, _ := p["options"].([]any)
		chosen := ""
		for _, o := range opts {
			if m, ok := o.(map[string]any); ok {
				kind, _ := m["kind"].(string)
				if strings.HasPrefix(kind, "allow") || chosen == "" {
					chosen, _ = m["optionId"].(string)
					if strings.HasPrefix(kind, "allow") {
						break
					}
				}
			}
		}
		reply(map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": chosen}})
	case "fs/read_text_file":
		path, _ := p["path"].(string)
		b, err := os.ReadFile(path)
		if err != nil {
			c.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": err.Error()}})
			return
		}
		reply(map[string]any{"content": string(b)})
	case "fs/write_text_file":
		path, _ := p["path"].(string)
		content, _ := p["content"].(string)
		os.WriteFile(path, []byte(content), 0o644)
		reply(map[string]any{})
	default:
		c.send(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32601, "message": "unsupported: " + method}})
	}
}

func (c *acpClient) note(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p struct {
		Update struct {
			Kind    string         `json:"sessionUpdate"`
			Content map[string]any `json:"content"`
			Title   string         `json:"title"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	switch p.Update.Kind {
	case "agent_message_chunk":
		if t, ok := p.Update.Content["text"].(string); ok {
			c.text.WriteString(t)
		}
	case "tool_call":
		c.tools = append(c.tools, p.Update.Title)
	}
}

func (c *acpClient) session(prompt string) error {
	if _, err := c.call("initialize", map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{"fs": map[string]any{"readTextFile": true, "writeTextFile": true}}}); err != nil {
		return err
	}
	res, err := c.call("session/new", map[string]any{"cwd": c.repo, "mcpServers": []any{}})
	if err != nil {
		return err
	}
	var s struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(res, &s)
	_, err = c.call("session/prompt", map[string]any{"sessionId": s.SessionID, "prompt": []map[string]any{{"type": "text", "text": prompt}}})
	return err
}
