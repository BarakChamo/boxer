package eval

import (
	"os/exec"
	"strings"
)

// checklist is an orchestrator that cannot be driven headlessly on this machine yet. It reports
// one skipped cell naming what is needed, so the report never presents its absence as a pass;
// the manual procedure is in docs/orchestrators.md.
type checklist struct {
	name string
	why  func() string
}

func (c checklist) Name() string { return c.name }
func (c checklist) Available(tier string) (bool, string) {
	return false, c.why() + "; checklist in docs/orchestrators.md"
}
func (c checklist) Cells(tier string) []Cell {
	return []Cell{{Harness: c.name, Mode: "rewrite", Entry: "project", Isolation: "worktree", Compliant: true, Tier: tier}}
}
func (checklist) Prepare(*Env, Cell) error                   { return nil }
func (checklist) Run(*Env, Cell, string) (Transcript, error) { return Transcript{}, nil }
func (checklist) Cleanup(*Env, Cell)                         {}

func installed(bin string) bool { _, err := exec.LookPath(bin); return err == nil }

// Paperclip: `paperclipai test-drive --harness claude` starts an isolated instance with an
// embedded database; the claude-local adapter forwards its config env to claude-agent-acp
// (spike 7), so a fake-model run is possible in principle, but the instance lifecycle (init,
// issue, heartbeat, REST) is not automated here.
var Paperclip = checklist{"paperclip", func() string {
	if !installed("paperclipai") {
		return "paperclipai is not installed (npm i -g paperclipai, then paperclipai test-drive --harness claude --no-browser)"
	}
	return "paperclip driver not automated: run the checklist"
}}

// T3 Code: `t3` starts the server and web app; commands go over its WebSocket RPC
// (project.create, thread.turn.start with bootstrap.createThread and bootstrap.prepareWorktree).
// The Claude driver spawns the CLI with process.env plus per-instance variables (spike 8).
var T3 = checklist{"t3code", func() string {
	if !installed("t3") {
		return "t3 is not installed (npm i -g t3)"
	}
	return "t3 driver not automated: run the checklist"
}}

// Multica: the local daemon runs the CLI in a worktree per task, but only against a configured
// Multica server (`multica setup`, an account); MULTICA_CLAUDE_ARGS carries extra CLI flags (spike 9).
var Multica = checklist{"multica", func() string {
	if !installed("multica") {
		return "multica is not installed (brew install multica-ai/tap/multica)"
	}
	if out, err := exec.Command("multica", "auth", "status").CombinedOutput(); err != nil || len(out) == 0 || strings.Contains(string(out), "No server configured") {
		return "multica has no server configured (multica setup, then multica daemon start)"
	}
	return "multica driver not automated: run the checklist"
}}
