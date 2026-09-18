package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/vm"
)

// T3 drives T3 Code headless: `t3 serve` on a private data dir, a bearer session minted with
// `t3 auth session issue`, and its WebSocket RPC (Effect RPC, JSON framing) to create a project,
// then one `thread.turn.start` whose bootstrap creates the thread and its worktree (spike 8).
// The Claude driver spawns the CLI through the Claude Agent SDK with process.env plus the
// instance's environment list, so the gateway variables ride on the provider instance and
// PATH/BOXER_TRACE inherit from the server started here. Two cells: the project layer with the
// stock `claude` (outside), and a `boxer shim install --harness claude` binary as the instance's
// binaryPath (inside: the harness itself runs in the guest through `boxer shell claude`).
type T3 struct{ srv *server }

func (*T3) Name() string { return "t3code" }

func (*T3) Available(tier string) (bool, string) {
	if !installed("t3") {
		return false, "t3 is not installed (npm i -g t3)"
	}
	if !installed("node") {
		return false, "node is not installed (the WebSocket client is a node script)"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (*T3) Cells(tier string) []Cell {
	return []Cell{
		{Harness: "t3code", Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier},
		{Harness: "t3code", Mode: "inside", Entry: "shim", Isolation: "worktree", Compliant: true, Tier: tier, Inside: "claude"},
	}
}

func (d *T3) base(env *Env) string { return filepath.Join(env.Work, "t3") }

var reT3Ready = regexp.MustCompile(`(T3 Code server is ready)`)

func (d *T3) Prepare(env *Env, c Cell) error {
	instance := map[string]any{"driver": "claudeAgent", "enabled": true}
	var envList []map[string]any
	add := func(kvs []string) {
		for _, kv := range kvs {
			k, v, _ := strings.Cut(kv, "=")
			envList = append(envList, map[string]any{"name": k, "value": v})
		}
	}
	if c.Inside != "" {
		// Config under the repository (the same path in the guest), the shim as the binary.
		if err := (Inside{}).Prepare(env, c); err != nil {
			return err
		}
		shims := filepath.Join(env.Work, "shims")
		if out, err := env.boxer(env.Repo, "shim", "install", "--harness", "claude", shims); err != nil {
			return fmt.Errorf("boxer shim install: %v\n%s", err, out)
		}
		// T3 creates the worktree and starts the harness in it at once, and its provider session
		// gives up while a cold VM boots. The git post-checkout hook is boxer's answer: the VM is
		// warm before the harness starts. This is the integration a user of an orchestrator wants.
		if out, err := env.boxer(env.Repo, "install", "git"); err != nil {
			return fmt.Errorf("boxer install git: %v\n%s", err, out)
		}
		add((Inside{}).guestEnvFor(env, "claude"))
		instance["config"] = map[string]any{"binaryPath": filepath.Join(shims, "claude"), "homePath": (Inside{}).cfgDir(env, "claude")}
	} else {
		if err := (Claude{}).Prepare(env, Cell{Entry: "project", Tier: c.Tier}); err != nil {
			return err
		}
		add((Claude{}).modelEnv(env))
		instance["config"] = map[string]any{"homePath": (Claude{}).home(env)}
	}
	instance["environment"] = envList
	if err := commitAll(env.Repo, "boxer eval"); err != nil {
		return err
	}
	settings, _ := json.Marshal(map[string]any{"providerInstances": map[string]any{"claudeAgent": instance}})
	userdata := filepath.Join(d.base(env), "userdata")
	if err := os.MkdirAll(userdata, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(userdata, "settings.json"), settings, 0o600); err != nil {
		return err
	}
	port, err := freePort()
	if err != nil {
		return err
	}
	srv, err := startServer(env.Repo, env.BaseEnv(), "t3", "serve", "--mode", "web", "--host", "127.0.0.1", "--port", fmt.Sprint(port), "--base-dir", d.base(env), "--no-browser", env.Repo)
	if err != nil {
		return err
	}
	d.srv = srv
	if _, err := srv.waitFor(reT3Ready, 3*time.Minute); err != nil {
		return fmt.Errorf("%v\n%s", err, tail(srv.output(), 2000))
	}
	return nil
}

// port is what `t3 serve` bound; kept in the command line.
func (d *T3) port() string { return d.srv.cmd.Args[7] }

func (d *T3) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	tok := exec.Command("t3", "auth", "session", "issue", "--base-dir", d.base(env), "--token-only", "--ttl", "1h")
	tok.Env = env.BaseEnv()
	token, err := tok.Output()
	if err != nil {
		return Transcript{Raw: d.srv.output()}, fmt.Errorf("t3 auth session issue: %v", err)
	}
	branch, _ := exec.Command("git", "-C", env.Repo, "symbolic-ref", "--short", "HEAD").Output()
	model := LiveModel("claude")
	if env.Tier == "t1" {
		model = "fake-model"
	}
	cmd := exec.Command("node", "--input-type=module", "-e", t3Client)
	cmd.Dir = env.Repo
	cmd.Env = append(os.Environ(),
		"T3_URL=ws://127.0.0.1:"+d.port()+"/ws",
		"T3_TOKEN="+strings.TrimSpace(string(token)),
		"T3_REPO="+env.Repo,
		"T3_BRANCH="+strings.TrimSpace(string(branch)),
		"T3_PROMPT="+prompt,
		"T3_MODEL="+model,
	)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return Transcript{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-time.After(12 * time.Minute): // worktree checkout, fresh VM, and for inside the harness install
		cmd.Process.Kill()
		err = fmt.Errorf("t3 turn timed out")
	}
	raw := out.String() + "\n--- t3 serve ---\n" + tail(d.srv.output(), 4000)
	tr := Transcript{Raw: raw}
	// The script's last line reports the turn.
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var res struct {
		Answer       string `json:"answer"`
		WorktreePath string `json:"worktreePath"`
		Error        string `json:"error"`
	}
	if len(lines) > 0 {
		json.Unmarshal([]byte(lines[len(lines)-1]), &res)
	}
	if res.WorktreePath != "" {
		if r, e := filepath.EvalSymlinks(res.WorktreePath); e == nil {
			res.WorktreePath = r
		}
		env.Root = res.WorktreePath
	}
	if err != nil {
		return tr, fmt.Errorf("%v: %s", err, res.Error)
	}
	if res.Error != "" {
		return tr, fmt.Errorf("t3: %s", res.Error)
	}
	if f := strings.Fields(res.Answer); len(f) > 0 {
		tr.Answer = f[0]
	}
	if env.Tier == "t2" && tr.Answer == "" {
		if q := quotaError(raw); q != "" {
			return tr, SkipError{q}
		}
	}
	return tr, nil
}

// Cleanup stops the server and reaps the VM keyed to the worktree T3 created, which lives
// outside the scratch directory and so outside ReapWork's reach.
func (d *T3) Cleanup(env *Env, c Cell) {
	d.srv.stop()
	d.srv = nil
	if env.Root == "" {
		return
	}
	client := vm.New()
	if ms, err := client.List(); err == nil {
		for _, m := range ms {
			if m.Labels["boxer.root"] == env.Root {
				client.Delete(m.Name)
			}
		}
	}
	exec.Command("git", "-C", env.Repo, "worktree", "remove", "--force", env.Root).Run()
}

// t3Client is the WebSocket client: Effect RPC over JSON. A request is {_tag: "Request", id,
// tag, payload, headers}; the reply is an Exit, or Chunks (each acknowledged) then an Exit for a
// stream. It creates the project, starts the bootstrapped turn, subscribes to the thread, and
// prints one JSON line per event, then {"answer", "worktreePath"} when the assistant's message
// stops streaming.
const t3Client = `
const env = process.env;
const ws = new WebSocket(env.T3_URL, { headers: { authorization: "Bearer " + env.T3_TOKEN } });
const pending = new Map();
let nextId = 1;
const send = (msg) => ws.send(JSON.stringify(msg));
const call = (tag, payload, onChunk) => new Promise((resolve, reject) => {
  const id = String(nextId++);
  pending.set(id, { resolve, reject, onChunk });
  send({ _tag: "Request", id, tag, payload, headers: [] });
});
const finish = (obj) => { console.log(JSON.stringify(obj)); ws.close(); process.exit(obj.error ? 1 : 0); };
ws.addEventListener("message", (ev) => {
  const msgs = JSON.parse(typeof ev.data === "string" ? ev.data : Buffer.from(ev.data).toString());
  for (const m of Array.isArray(msgs) ? msgs : [msgs]) {
    const p = pending.get(String(m.requestId));
    if (m._tag === "Chunk") { p?.onChunk?.(m.values); send({ _tag: "Ack", requestId: m.requestId }); }
    else if (m._tag === "Exit") {
      pending.delete(String(m.requestId));
      if (m.exit._tag === "Success") p?.resolve(m.exit.value); else p?.reject(new Error(JSON.stringify(m.exit)));
    } else if (m._tag === "Defect") finish({ error: "defect: " + JSON.stringify(m.defect) });
  }
});
ws.addEventListener("error", (e) => finish({ error: "websocket: " + (e.message ?? e.type) }));
ws.addEventListener("close", (e) => finish({ error: "websocket closed: " + e.code + " " + e.reason }));
ws.addEventListener("open", async () => {
  try {
    const now = () => new Date().toISOString();
    const projectId = crypto.randomUUID(), threadId = crypto.randomUUID();
    const modelSelection = { instanceId: "claudeAgent", model: env.T3_MODEL };
    await call("orchestration.dispatchCommand", { type: "project.create", commandId: crypto.randomUUID(), projectId, title: "boxer eval", workspaceRoot: env.T3_REPO, createdAt: now() });
    await call("orchestration.dispatchCommand", {
      type: "thread.turn.start", commandId: crypto.randomUUID(), threadId,
      message: { messageId: crypto.randomUUID(), role: "user", text: env.T3_PROMPT, attachments: [] },
      runtimeMode: "full-access", interactionMode: "default", createdAt: now(),
      bootstrap: {
        createThread: { projectId, title: "uname", modelSelection, runtimeMode: "full-access", interactionMode: "default", branch: null, worktreePath: null, createdAt: now() },
        prepareWorktree: { projectCwd: env.T3_REPO, baseBranch: env.T3_BRANCH, branch: "boxer-eval" },
      },
    });
    // The turn ends when the session drops its activeTurnId again; the text arrives on the
    // streaming message and the final one is empty, so the last non-empty message is the answer.
    let worktreePath = null, answer = null, started = false;
    const seen = (t) => { if (t.worktreePath) worktreePath = t.worktreePath; };
    await call("orchestration.subscribeThread", { threadId }, (values) => {
      for (const v of values) {
        console.log(JSON.stringify(v));
        if (v.kind === "snapshot") { seen(v.snapshot?.thread ?? v.thread ?? {}); continue; }
        const ev = v.event; if (!ev) continue;
        if (ev.type === "thread.meta-updated" || ev.type === "thread.created") seen(ev.payload);
        if (ev.type === "thread.message-sent" && ev.payload.role === "assistant" && ev.payload.text?.trim()) answer = ev.payload.text;
        if (ev.type === "thread.session-set") {
          const s = ev.payload.session ?? {};
          if (s.status === "error") finish({ error: JSON.stringify(s.lastError), worktreePath });
          if (s.activeTurnId) started = true;
          else if (started) finish({ answer, worktreePath });
        }
      }
    });
    finish({ error: "thread stream ended before the turn completed", answer, worktreePath });
  } catch (e) { finish({ error: String(e.message ?? e) }); }
});
`
