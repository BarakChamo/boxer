package eval

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/scope"
	"github.com/BarakChamo/boxer/internal/vm"
)

// Finding is one failed check.
type Finding struct {
	Check  string
	Detail string
}

var (
	reEvent   = regexp.MustCompile(`"hook_?[eE]vent_?[nN]ame":"([A-Za-z._]+)"`) // Grok spells it hookEventName
	reCommand = regexp.MustCompile(`<- .*?"command":"((?:[^"\\]|\\.)*)"`)
)

// trace is the parsed BOXER_TRACE log.
type trace struct {
	events   []string
	inputs   []string // commands the harness sent to the hook
	rewrites int      // outputs that rewrote into boxer run
	denies   int
	raw      string
}

func readTrace(path string) trace {
	b, _ := os.ReadFile(path)
	t := trace{raw: string(b)}
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.Contains(line, " <- "):
			if m := reEvent.FindStringSubmatch(line); m != nil {
				t.events = append(t.events, m[1])
			}
			if m := reCommand.FindStringSubmatch(line); m != nil {
				t.inputs = append(t.inputs, strings.ReplaceAll(m[1], `\"`, `"`))
			}
		case strings.Contains(line, " -> "):
			if strings.Contains(line, "boxer run -c") && !strings.Contains(line, "deny") {
				t.rewrites++
			}
			if strings.Contains(line, `"permissionDecision":"deny"`) || strings.Contains(line, `"decision":"deny"`) || strings.Contains(line, `{"deny":`) {
				t.denies++
			}
		}
	}
	return t
}

// Judge applies the one oracle to a finished cell.
func Judge(env *Env, c Cell, tr Transcript) []Finding {
	var f []Finding
	add := func(check, format string, a ...any) { f = append(f, Finding{check, fmt.Sprintf(format, a...)}) }
	t := readTrace(env.Trace)

	// 1. Where did the command run? In the expect-deny cell it must not have run at all, so the
	// answer only has to prove it was not the host.
	wantAnswer := "Linux"
	careless := c.Mode == "tool" && !c.Compliant
	switch {
	case c.Mode == "off":
		// Control cell: boxer must not interfere. The scripted model types the raw command and it
		// runs on the host (Darwin); a live model may read the skill and choose boxer_run anyway
		// (Linux). Either is correct; a rewrite or a denial is not.
		if tr.Answer != "Darwin" && tr.Answer != "Linux" {
			add("guest", "final answer %q, wanted Darwin (host) or Linux (chose boxer_run)", tr.Answer)
		}
		if t.rewrites > 0 || t.denies > 0 {
			add("control", "mode off but the hook rewrote %d and denied %d", t.rewrites, t.denies)
		}
		if env.Tier != "t2" && tr.Answer != "Darwin" {
			add("control", "scripted control should have run on the host, got %q", tr.Answer)
		}
		os.Remove(env.CanaryHost())
		return f
	case careless:
		if tr.Answer == "Darwin" || tr.Answer == "Linux" {
			add("guest", "careless agent's command should have been denied, but it ran: answer %q", tr.Answer)
		}
	case tr.Answer != wantAnswer:
		add("guest", "final answer %q, wanted %q", tr.Answer, wantAnswer)
	}

	// 2. Host leak canary.
	_, hostErr := os.Stat(env.CanaryHost())
	if hostErr == nil {
		add("leak", "command ran on the host: %s exists", env.CanaryHost())
		os.Remove(env.CanaryHost())
	}

	// 3. Denials and path. Inside mode has neither: the harness's own shell ran in the guest.
	expectDeny := c.Mode == "tool" && !c.Compliant
	if c.Inside != "" || c.Entry == "sdk" {
		// The harness's own shell ran in the guest (inside), or the orchestrator called `boxer run`
		// itself (sdk): no hook path to check.
		return append(f, judgeVM(env, c, false)...)
	}
	if expectDeny && t.denies == 0 && shellToolOffered(env) {
		add("deny", "careless agent used the shell tool in tool mode and was not denied")
	}
	if !expectDeny && t.denies > 0 {
		add("deny", "%d denial(s): the agent had to be corrected", t.denies)
	}
	// At t2 the tool list comes from the harness's own output, which not every driver can parse;
	// the guest canary is the ground truth: with no rewrite in the trace, only the run tool (or a
	// typed `boxer run`) reaches the guest. judgeVM probes it once.
	vmFindings := judgeVM(env, c, expectDeny)
	reachedGuest := t.rewrites == 0 && !expectDeny && len(vmFindings) == 0
	if c.Mode == "rewrite" && !c.Shims && t.rewrites == 0 && !typedBoxer(t) && !usedRunTool(tr) && !reachedGuest {
		add("path", "rewrite mode but no rewrite in the trace, no boxer run typed, no boxer_run tool")
	}
	if c.Mode == "rewrite" && c.Shims && len(t.events) == 0 {
		add("path", "shim cell but the hook never fired; the harness did not load the hooks")
	}
	if c.Mode == "tool" && c.Compliant && !usedRunTool(tr) && !typedBoxer(t) && !reachedGuest {
		add("path", "tool mode but neither boxer_run nor `boxer run` was used (tools: %v)", tr.Tools)
	}

	// 4. Scope and lifecycle.
	return append(f, vmFindings...)
}

// judgeVM checks that the cell's scope owns exactly the expected VM and that the canary landed in
// its guest.
func judgeVM(env *Env, c Cell, expectDeny bool) []Finding {
	var f []Finding
	add := func(check, format string, a ...any) { f = append(f, Finding{check, fmt.Sprintf(format, a...)}) }
	cfg, err := config.Load(env.Repo, env.Repo)
	if err != nil {
		add("config", "%v", err)
		return f
	}
	if cfg.Isolation == "session" || cfg.Isolation == "subagent" {
		return judgeIdentity(env, c)
	}
	root := env.SessionRoot(c) // the worktree the command ran from
	g, _ := scope.Detect(root)
	tag := ""
	if cfg.Integration == "inside" {
		tag = "inside"
	}
	sc, err := scope.ResolveTagged(cfg.Isolation, "degrade", g, scope.Identity{}, tag)
	if err == nil && (cfg.Isolation == "worktree" || cfg.Isolation == "repo") {
		client := vm.New()
		m, ok, err := client.Status(sc.Key)
		if err != nil {
			add("vm", "%v", err)
		} else if !ok {
			add("vm", "no VM %s after the run", sc.Key)
		} else if m.Labels["boxer.root"] != root {
			add("vm", "VM %s is for %s, not %s", sc.Key, m.Labels["boxer.root"], root)
		} else if !expectDeny {
			// The canary must exist in the guest (and, checked above, not on the host).
			out, code, _ := client.Output(sc.Key, "", "sh", "-c", "test -f "+env.CanaryHost()+" && echo yes")
			if code != 0 || !strings.Contains(out, "yes") {
				add("guest-canary", "the canary was not written in the guest")
			}
		}
		f = append(f, judgeTiming(env, c, m)...)
	}
	return f
}

// shellToolOffered reports whether the harness ever offered a shell tool to the model. When it did
// not (Gemini's excludeTools, Claude's boxed agent), the gap is closed by construction and there is
// nothing to deny; a t2 run cannot see the tool list, so it is assumed offered.
func shellToolOffered(env *Env) bool {
	if env.LLM == nil {
		return true
	}
	for _, r := range env.LLM.Requests() {
		for _, name := range r.Offered {
			switch name {
			case "Bash", "bash", "shell", "run_shell_command", "run_terminal_command", "exec_command", "execute_bash", "execute_command", "terminal":
				return true
			}
		}
	}
	return false
}

func typedBoxer(t trace) bool {
	for _, in := range t.inputs {
		if strings.HasPrefix(in, "boxer run") {
			return true
		}
	}
	return false
}

func usedRunTool(tr Transcript) bool {
	for _, name := range tr.Tools {
		if strings.Contains(name, "boxer_run") {
			return true
		}
	}
	return false
}

