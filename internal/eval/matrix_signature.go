// Signatures: the evidence that an integration level carried the work, rather than that the task
// happened to succeed. An agent can do the job on the host and render a perfectly good page, so
// every level has to show its own mechanism — a hook rewrite, a denial, a run tool, the pointed
// absence of any of them.
package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// evidence is everything a signature may read. It is a struct rather than three parameters because
// the orchestrator levels need the worktree too: a launcher that builds a clean environment for the
// harness it starts does not pass boxer's trace variable through, so for those the filesystem is
// the only place the answer exists.
type evidence struct {
	t     trace
	tr    Transcript
	guest string // what node reported as its platform
	dir   string // the worktree that was judged
}

// The signatures. Each answers "did this level carry the work", which is not the same question as
// "did the task pass": an agent can do the job on the host and render a perfectly good page.

func sigRewrite(e evidence) (string, error) {
	if err := guestIsLinux(e.guest); err != nil {
		return "", err
	}
	switch {
	case e.t.rewrites > 0:
		return "the hook rewrote the command into `boxer run`", nil
	case typedAnywhere(e.t) || mentionsBoxerRun(e.tr.Raw):
		// The other half of the project layer: boxer installs an agent contract, and an agent that
		// reads it types `boxer run` itself. The work is still in the guest by boxer's doing, so
		// this is a pass — but a differently-shaped one, which is why the report says so.
		return "the agent typed `boxer run` itself, from the installed agent contract", nil
	case len(e.t.events) > 0 || len(e.t.allowed) > 0:
		// The OpenCode plugin rewrites in-process and writes no rewrite line, so the trace shows the
		// layer loaded and the commands passing through it, and nothing more. Taken alone that would
		// be weak evidence — but the caller has already established that the dependency really is
		// installed and that the guest, not the host, ran the command, and an agent would have to
		// fake both to reach this point. It is a pass with its mechanism named, not a silent one.
		return "the integration layer carried it, though boxer logged no rewrite (it does not log one here)", nil
	}
	return "", fmt.Errorf("rewrite level, but nothing rewrote and nothing typed `boxer run` (%d denials, %d allowed)", e.t.denies, len(e.t.allowed))
}

func sigTool(e evidence) (string, error) {
	if err := guestIsLinux(e.guest); err != nil {
		return "", err
	}
	// A denial is what an agent that ignores the brief runs into; an agent that follows it simply
	// uses the run tool and is never denied anything. Both prove the level, so both pass.
	switch {
	case usedRunTool(e.tr) || mentionsRunTool(e.tr.Raw) || mentionsRunTool(e.t.raw):
		// The trace counts as much as the transcript here: several harnesses do not name the MCP
		// tools they called in their own output, and reading only the transcript failed a level
		// whose evidence boxer had recorded itself.
		return "the agent used the `boxer_run` tool", nil
	case e.t.denies > 0:
		return "the shell was denied and the work went through the run tool", nil
	}
	return "", fmt.Errorf("tool level, but the run tool was never used and nothing was ever denied")
}

func sigShims(e evidence) (string, error) {
	if err := guestIsLinux(e.guest); err != nil {
		return "", err
	}
	if e.t.rewrites > 0 {
		return "", fmt.Errorf("shim level, but the trace shows %d rewrites: a hook carried the work, not the shim", e.t.rewrites)
	}
	return "a PATH shim carried the bare command into the guest", nil
}

func sigInside(e evidence) (string, error) {
	if err := guestIsLinux(e.guest); err != nil {
		return "", err
	}
	if len(e.t.events) > 0 {
		return "", fmt.Errorf("inside level, but the hook fired %d times: something outside was intercepting", len(e.t.events))
	}
	return "the harness itself ran in the guest; nothing was intercepted", nil
}

// mentionsBoxerRun and mentionsRunTool read the harness's own transcript, for the harnesses whose
// hooks never write a trace line at all. Without them a level would be judged on evidence boxer
// only collects for some harnesses, which is a measurement artefact rather than a result.
func mentionsBoxerRun(raw string) bool { return strings.Contains(raw, "boxer run ") }

// typedAnywhere is typedBoxer without the assumption that `boxer run` starts the line. Agents
// write `cd app && boxer run npm install` constantly, and the oracle's prefix check misses it.
func typedAnywhere(t trace) bool {
	for _, in := range t.inputs {
		if strings.Contains(in, "boxer run ") {
			return true
		}
	}
	return false
}

func mentionsRunTool(raw string) bool { return strings.Contains(raw, "boxer_run") }

// sigOrchestrator is for the launchers that start the harness themselves in a worktree they cut.
// Paperclip builds a clean environment for that harness, so boxer's trace variable does not reach
// it and no trace is written however well the interception worked. Absence of a trace therefore
// says nothing here, and reading it as "nothing was intercepted" reports a working level as broken.
// What does hold: the worktree the orchestrator made carries boxer's project layer, and the work
// ran in the guest — which the caller has already corroborated against package.json.
func sigOrchestrator(e evidence) (string, error) {
	if err := guestIsLinux(e.guest); err != nil {
		return "", err
	}
	if e.t.rewrites > 0 {
		return "the hook rewrote the command into `boxer run`", nil
	}
	if typedAnywhere(e.t) || mentionsBoxerRun(e.tr.Raw) {
		return "the agent typed `boxer run` itself, from the installed agent contract", nil
	}
	for _, p := range []string{".claude/settings.json", ".codex/hooks.json", ".opencode"} {
		if _, err := os.Stat(filepath.Join(e.dir, p)); err == nil {
			return "the worktree the orchestrator cut carries boxer's project layer, and the work ran in the guest (its launcher passes no trace through, so there is nothing finer to read)", nil
		}
	}
	return "", fmt.Errorf("the orchestrator's worktree carries no boxer layer, so nothing there could have sandboxed the work")
}

// guestIsLinux is the shared half of every signature: the agent recorded what `node` reported as
// its platform, and on this host anything but linux means the command never left it.
func guestIsLinux(guest string) error {
	if !strings.EqualFold(guest, "linux") {
		if guest == "" {
			return fmt.Errorf("the agent never recorded a platform, so nothing proves the work reached the guest")
		}
		return fmt.Errorf("the work ran on the host: the guest probe reported %q", guest)
	}
	return nil
}
