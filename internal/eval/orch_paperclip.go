package eval

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Paperclip drives `paperclipai test-drive`: an isolated instance (embedded database, loopback,
// no auth in local_trusted mode) whose CEO agent is claude-agent-acp with settingSources user,
// project and local, so the project layer committed by `boxer install claude-code` applies. A
// project whose primary workspace is the eval repository and whose execution policy is
// isolated_workspace + git_worktree makes Paperclip cut one worktree per issue; the oracle
// judges that worktree's VM. The gateway variables reach the agent through the adapter's
// config.env (spike 7); PATH and BOXER_TRACE are inherited from the server process, which this
// driver starts from the eval environment.
type Paperclip struct{ srv *server }

func (*Paperclip) Name() string { return "paperclip" }

func (*Paperclip) Available(tier string) (bool, string) {
	if !installed("paperclipai") {
		return false, "paperclipai is not installed (npm i -g paperclipai)"
	}
	if tier == "t2" {
		if _, why := gatewayKey(); why != "" {
			return false, why
		}
	}
	return true, ""
}

func (*Paperclip) Cells(tier string) []Cell {
	return []Cell{{Harness: "paperclip", Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier}}
}

var rePaperclipReady = regexp.MustCompile(`Paperclip is ready at (http://[^\s]+)`)

func (d *Paperclip) Prepare(env *Env, c Cell) error {
	// The Claude driver's project entry: a private config dir with the key approved, hooks and
	// the run tool under .claude/, MCP approval in settings.local.json. Committed so a worktree
	// would carry it too.
	if err := (Claude{}).Prepare(env, Cell{Entry: "project", Tier: c.Tier}); err != nil {
		return err
	}
	if err := commitAll(env.Repo, "boxer install claude-code"); err != nil {
		return err
	}
	args := []string{"test-drive", "--harness", "claude", "--no-browser", "--data-dir", filepath.Join(env.Work, "paperclip")}
	if env.Tier == "t2" {
		args = append(args, "--model", LiveModel("claude"))
	}
	// test-drive reads ANTHROPIC_API_KEY from its own environment into the agent's secret.
	srv, err := startServer(env.Repo, append(env.BaseEnv(), (Claude{}).modelEnv(env)...), "paperclipai", args...)
	if err != nil {
		return err
	}
	d.srv = srv
	// First start downloads embedded Postgres; later ones take ~20 s.
	if _, err := srv.waitFor(rePaperclipReady, 5*time.Minute); err != nil {
		return fmt.Errorf("%v\n%s", err, tail(srv.output(), 2000))
	}
	return nil
}

func (d *Paperclip) api(env *Env) string {
	m := rePaperclipReady.FindStringSubmatch(d.srv.output())
	return strings.TrimRight(m[1], "./") + "/api" // the ready line ends in a full stop
}

func (d *Paperclip) Run(env *Env, c Cell, prompt string) (Transcript, error) {
	api := d.api(env)
	var companies, agents []struct {
		ID string `json:"id"`
	}
	if err := httpJSON("GET", api+"/companies", nil, nil, &companies); err != nil || len(companies) == 0 {
		return Transcript{Raw: d.srv.output()}, fmt.Errorf("paperclip companies: %v", err)
	}
	cid := companies[0].ID
	if err := httpJSON("GET", api+"/companies/"+cid+"/agents", nil, nil, &agents); err != nil || len(agents) == 0 {
		return Transcript{Raw: d.srv.output()}, fmt.Errorf("paperclip agents: %v", err)
	}
	aid := agents[0].ID
	// The agent runs with the eval's Claude config dir (Paperclip would otherwise seed a managed
	// one from ~/.claude) and the gateway variables. env replaces the adapter's env block, so the
	// key rides along as a plain value.
	agentEnv := map[string]string{}
	for _, kv := range (Claude{}).modelEnv(env) {
		k, v, _ := strings.Cut(kv, "=")
		agentEnv[k] = v
	}
	patch := map[string]any{"adapterConfig": map[string]any{"env": agentEnv}}
	if err := httpJSON("PATCH", api+"/agents/"+aid, nil, patch, nil); err != nil {
		return Transcript{Raw: d.srv.output()}, err
	}
	// Isolated execution workspaces are an experimental instance setting, off by default; without
	// it the project's worktree policy is gated away (gateProjectExecutionWorkspacePolicy) and
	// every run uses the project checkout itself.
	if err := httpJSON("PATCH", api+"/instance/settings/experimental", nil, map[string]any{"enableIsolatedWorkspaces": true}, nil); err != nil {
		return Transcript{Raw: d.srv.output()}, err
	}
	// One project, primary workspace = the eval repository, one git worktree per task (Paperclip
	// puts it under <repo>/.paperclip/worktrees, so ReapWork reaches its VM).
	branch, _ := exec.Command("git", "-C", env.Repo, "symbolic-ref", "--short", "HEAD").Output()
	project := map[string]any{
		"name":      "boxer eval",
		"workspace": map[string]any{"sourceType": "local_path", "cwd": env.Repo, "isPrimary": true},
		"executionWorkspacePolicy": map[string]any{"enabled": true, "defaultMode": "isolated_workspace",
			"workspaceStrategy": map[string]any{"type": "git_worktree", "baseRef": strings.TrimSpace(string(branch))}},
	}
	var proj struct {
		ID string `json:"id"`
	}
	if err := httpJSON("POST", api+"/companies/"+cid+"/projects", nil, project, &proj); err != nil {
		return Transcript{Raw: d.srv.output()}, err
	}
	issue := map[string]any{"title": "uname", "description": prompt, "status": "todo", "assigneeAgentId": aid, "projectId": proj.ID}
	var iss struct {
		ID string `json:"id"`
	}
	if err := httpJSON("POST", api+"/companies/"+cid+"/issues", nil, issue, &iss); err != nil {
		return Transcript{Raw: d.srv.output()}, err
	}
	// The wake must name the issue: a bare heartbeat resolves no project and Paperclip falls back
	// to a workspace of its own ("No project or prior session workspace was available"), where
	// neither the project layer nor a worktree exists. payload.issueId becomes the run's context
	// (services/heartbeat.js, enrichWakeContextSnapshot), which resolves the issue's project and
	// its primary workspace.
	wake := map[string]any{"source": "assignment", "payload": map[string]any{"issueId": iss.ID}}
	if err := httpJSON("POST", api+"/agents/"+aid+"/wakeup", nil, wake, nil); err != nil {
		return Transcript{Raw: d.srv.output()}, err
	}

	type run struct {
		ID     string          `json:"id"`
		Status string          `json:"status"`
		Result json.RawMessage `json:"resultJson"`
	}
	var final run
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) && final.ID == "" {
		time.Sleep(3 * time.Second)
		var runs []run
		if err := httpJSON("GET", api+"/companies/"+cid+"/heartbeat-runs?agentId="+aid, nil, nil, &runs); err != nil {
			continue
		}
		for _, r := range runs {
			switch r.Status {
			case "queued", "scheduled_retry", "running":
			default:
				final = r
			}
		}
	}
	env.Root = linkedWorktree(env.Repo)
	raw := &strings.Builder{}
	if final.ID == "" {
		fmt.Fprintf(raw, "no heartbeat run finished\n%s", tail(d.srv.output(), 4000))
		return Transcript{Raw: raw.String()}, fmt.Errorf("paperclip run timed out")
	}
	var log struct {
		Content string `json:"content"`
	}
	httpJSON("GET", api+"/heartbeat-runs/"+final.ID+"/log?limitBytes=2000000", nil, nil, &log)
	fmt.Fprintf(raw, "run %s: %s\nresult: %s\n--- log ---\n%s\n--- server ---\n%s", final.ID, final.Status, final.Result, log.Content, tail(d.srv.output(), 4000))
	tr := Transcript{Raw: raw.String(), Answer: lastKernel(log.Content + string(final.Result))}
	if final.Status != "succeeded" {
		return tr, fmt.Errorf("paperclip run %s", final.Status)
	}
	return tr, nil
}

func (d *Paperclip) Cleanup(env *Env, c Cell) { d.srv.stop(); d.srv = nil }

// commitAll commits the working tree so orchestrator worktrees carry boxer.toml and the project layer.
func commitAll(repo, msg string) error {
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.email=eval@boxer", "-c", "user.name=boxer-eval", "commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v\n%s", args, err, out)
		}
	}
	return nil
}

// linkedWorktree is the one linked worktree of repo, or "" when the orchestrator made none.
func linkedWorktree(repo string) string {
	out, _ := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok && p != repo {
			if r, err := filepath.EvalSymlinks(p); err == nil {
				return r
			}
			return p
		}
	}
	return ""
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
