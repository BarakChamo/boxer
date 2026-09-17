package eval

import (
	"fmt"

	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Worktree is the linked worktree a timing cell creates: before the session (Prepare) or by the
// agent's own shell (mid). It is nested in the repository because Claude Code resets a shell cwd
// that leaves the project directory, and a nested linked worktree is a worktree like any other.
func (e *Env) Worktree() string { return filepath.Join(e.Repo, "wt") }

// SessionRoot is the worktree the cell's command runs from.
func (e *Env) SessionRoot(c Cell) string {
	if c.Timing == "before" || c.Timing == "mid" {
		return e.Worktree()
	}
	return e.Repo
}

// AddWorktree creates the linked worktree the way an orchestrator would, before the session.
func (e *Env) AddWorktree() error {
	cmd := exec.Command("git", "-C", e.Repo, "worktree", "add", "-q", e.Worktree(), "-b", "eval-wt")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %v\n%s", err, out)
	}
	return nil
}

// commands are the scripted model's shell lines. The mid-session cell creates a worktree and
// moves into it first; the harness's shell keeps the cwd, so the third line runs from the new
// worktree and boxer keys its VM off that path.
func (e *Env) commands(c Cell) []string {
	if c.Timing == "mid" {
		return []string{
			fmt.Sprintf("git worktree add %s -b eval-wt", e.Worktree()),
			"cd " + e.Worktree(),
			e.Command(),
		}
	}
	return []string{e.Command()}
}

// ReapWork deletes every boxer VM rooted under this cell's scratch directory: `boxer down` from
// the repository only reaches the scope a bare invocation resolves, not a session, subagent or
// second worktree VM.
func (e *Env) ReapWork() {
	client := vm.New()
	ms, err := client.List()
	if err != nil {
		return
	}
	for _, m := range ms {
		if root := m.Labels["boxer.root"]; root != "" && strings.HasPrefix(root, e.Work+"/") {
			_ = client.Delete(m.Name)
		}
	}
}

var (
	reSession = regexp.MustCompile(`"session_id":"([^"]+)"`)
	reAgent   = regexp.MustCompile(`"agent_id":"([^"]+)"`)
)

// judgeIdentity is the oracle for session and subagent isolation: the ids in the shell tool's
// hook payload name exactly the VM the command ran in, and a subagent's VM is not the session's.
func judgeIdentity(env *Env, c Cell) []Finding {
	var f []Finding
	add := func(check, format string, a ...any) { f = append(f, Finding{check, fmt.Sprintf(format, a...)}) }
	var id scope.Identity
	for _, line := range strings.Split(readTrace(env.Trace).raw, "\n") {
		if !strings.Contains(line, " <- ") || !strings.Contains(line, `"PreToolUse"`) || !strings.Contains(line, `"command"`) {
			continue
		}
		if m := reSession.FindStringSubmatch(line); m != nil {
			id.SessionID = m[1]
		}
		if m := reAgent.FindStringSubmatch(line); m != nil {
			id.AgentID = m[1]
		}
	}
	if id.SessionID == "" {
		add("identity", "no session_id in the shell tool's hook payload")
		return f
	}
	if c.Isolation == "subagent" && id.AgentID == "" {
		add("identity", "no agent_id in the shell tool's hook payload; the command did not run in a subagent")
		return f
	}
	g, _ := scope.Detect(env.Repo)
	want, err := scope.Resolve(c.Isolation, "fail", g, id)
	if err != nil {
		add("identity", "%v", err)
		return f
	}
	client := vm.New()
	ms, _ := client.List()
	var mine []string
	for _, m := range ms {
		if m.Labels["boxer.root"] == env.Repo {
			mine = append(mine, m.Name)
		}
	}
	m, ok, err := client.Status(want.Key)
	if err != nil || !ok {
		add("vm", "no VM %s for %s %v (VMs for this repo: %v)", want.Key, c.Isolation, id, mine)
		return f
	}
	if out, code, _ := client.Output(want.Key, "", "sh", "-c", "test -f "+env.CanaryHost()+" && echo yes"); code != 0 || !strings.Contains(out, "yes") {
		add("guest-canary", "the canary was not written in the %s VM %s", c.Isolation, want.Key)
	}
	switch c.Isolation {
	case "session":
		if len(mine) != 1 {
			add("scope", "one VM per session wanted, got %v", mine)
		}
	case "subagent":
		session, _ := scope.Resolve("session", "fail", g, scope.Identity{SessionID: id.SessionID})
		if session.Key == want.Key || m.Labels["boxer.isolation"] != "subagent" {
			add("scope", "subagent VM %s must differ from the session's %s", want.Key, session.Key)
		}
	}
	return f
}

// judgeTiming adds the timing matrix's checks for the cell's row, given the VM the command ran in.
func judgeTiming(env *Env, c Cell, m vm.Machine) []Finding {
	var f []Finding
	add := func(check, format string, a ...any) { f = append(f, Finding{check, fmt.Sprintf(format, a...)}) }
	g, _ := scope.Detect(env.SessionRoot(c))
	switch c.Timing {
	case "before", "mid":
		if !g.Linked {
			add("timing", "%s is not a linked worktree", env.SessionRoot(c))
		}
		if c.Timing == "mid" {
			// The session started in the main checkout, which owns the first VM; the new worktree got a second.
			main, _ := scope.Detect(env.Repo)
			first, _ := scope.Resolve("worktree", "fail", main, scope.Identity{})
			if _, ok, _ := vm.New().Status(first.Key); !ok {
				add("timing", "the main checkout's VM %s is gone; the mid-session worktree should add a VM, not replace one", first.Key)
			} else if first.Key == m.Name {
				add("timing", "the command in the new worktree ran in the main checkout's VM")
			}
		}
	case "never":
		if g.Linked {
			add("timing", "never: the session should run in the main checkout")
		}
	case "warm":
		var sessionIn, sessionOut, firstTool time.Time
		for _, line := range strings.Split(readTrace(env.Trace).raw, "\n") {
			ts, _ := time.Parse(time.RFC3339Nano, strings.SplitN(line, " ", 2)[0])
			switch {
			case strings.Contains(line, ` <- `) && strings.Contains(line, `"SessionStart"`) && sessionIn.IsZero():
				sessionIn = ts
			case strings.Contains(line, ` -> `) && strings.Contains(line, `"SessionStart"`) && sessionOut.IsZero():
				sessionOut = ts
			case strings.Contains(line, ` <- `) && strings.Contains(line, `"PreToolUse"`) && firstTool.IsZero():
				firstTool = ts
			}
		}
		if sessionIn.IsZero() || firstTool.IsZero() {
			add("timing", "trace lacks a SessionStart or PreToolUse timestamp")
			return f
		}
		created := time.Unix(m.CreatedAt, 0)
		fmt.Fprintf(env.Log, "  warm: SessionStart hook returned in %s; VM created %s after session start, %s before the first tool call\n",
			sessionOut.Sub(sessionIn).Round(time.Millisecond), created.Sub(sessionIn).Round(time.Second), firstTool.Sub(created).Round(time.Second))
		if sessionOut.Sub(sessionIn) > 1500*time.Millisecond {
			add("timing", "SessionStart hook blocked for %s with warm_on_session_start", sessionOut.Sub(sessionIn))
		}
		if created.After(firstTool) {
			add("timing", "VM created at %s, after the first tool call at %s: the warm-up did not land first", created.Format(time.TimeOnly), firstTool.Format(time.TimeOnly))
		}
	}
	return f
}
