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
	reEvent   = regexp.MustCompile(`"hook_event_name":"([A-Za-z._]+)"`)
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
	if c.Mode == "off" {
		wantAnswer = "Darwin" // control cell: the host
	}
	careless := c.Mode == "tool" && !c.Compliant
	if careless {
		if tr.Answer == "Darwin" || tr.Answer == "Linux" {
			add("guest", "careless agent's command should have been denied, but it ran: answer %q", tr.Answer)
		}
	} else if tr.Answer != wantAnswer {
		add("guest", "final answer %q, wanted %q", tr.Answer, wantAnswer)
	}

	// 2. Host leak canary.
	_, hostErr := os.Stat(env.CanaryHost())
	if c.Mode == "off" {
		if hostErr != nil {
			add("control", "mode off should have run on the host, but the canary %s is missing", env.CanaryHost())
		}
		os.Remove(env.CanaryHost())
		return f
	}
	if hostErr == nil {
		add("leak", "command ran on the host: %s exists", env.CanaryHost())
		os.Remove(env.CanaryHost())
	}

	// 3. Denials and path. Inside mode has neither: the harness's own shell ran in the guest.
	expectDeny := c.Mode == "tool" && !c.Compliant
	if c.Inside != "" {
		return append(f, judgeVM(env, c, false)...)
	}
	if expectDeny && t.denies == 0 && shellToolOffered(env) {
		add("deny", "careless agent used the shell tool in tool mode and was not denied")
	}
	if !expectDeny && t.denies > 0 {
		add("deny", "%d denial(s): the agent had to be corrected", t.denies)
	}
	if c.Mode == "rewrite" && !c.Shims && t.rewrites == 0 && !typedBoxer(t) && !usedRunTool(tr) {
		add("path", "rewrite mode but no rewrite in the trace, no boxer run typed, no boxer_run tool")
	}
	if c.Mode == "rewrite" && c.Shims && len(t.events) == 0 {
		add("path", "shim cell but the hook never fired; the harness did not load the hooks")
	}
	if c.Mode == "tool" && c.Compliant && !usedRunTool(tr) && !typedBoxer(t) {
		add("path", "tool mode but neither boxer_run nor `boxer run` was used (tools: %v)", tr.Tools)
	}

	// 4. Scope and lifecycle.
	return append(f, judgeVM(env, c, expectDeny)...)
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
	g, _ := scope.Detect(env.Repo)
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
		} else if m.Labels["boxer.root"] != env.Repo {
			add("vm", "VM %s is for %s, not %s", sc.Key, m.Labels["boxer.root"], env.Repo)
		} else if !expectDeny {
			// The canary must exist in the guest (and, checked above, not on the host).
			out, code, _ := client.Output(sc.Key, "", "sh", "-c", "test -f "+env.CanaryHost()+" && echo yes")
			if code != 0 || !strings.Contains(out, "yes") {
				add("guest-canary", "the canary was not written in the guest")
			}
		}
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
