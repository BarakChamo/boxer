package hook

import "testing"

func TestSignalsDeriveFromDialects(t *testing.T) {
	by := map[string]Signal{}
	for _, s := range Signals("subagent", true) {
		by[s.Harness] = s
	}
	if len(by) != len(Dialects) {
		t.Fatalf("one row per dialect, got %d", len(by))
	}
	c := by["claude-code"]
	if !c.SessionStart || !c.Rewrite || c.BlockOnly || !c.SubagentStart || !c.MCP || !c.GitHook || c.EffectiveIsolation != "subagent" {
		t.Fatalf("claude-code: %+v", c)
	}
	k := by["kimi"]
	if k.Rewrite || !k.BlockOnly || k.EffectiveIsolation != "subagent" {
		t.Fatalf("kimi: %+v", k)
	}
	g := by["gemini-cli"]
	if g.SubagentStart || g.EffectiveIsolation != "session" {
		t.Fatalf("gemini degrades subagent to session: %+v", g)
	}
	p := by["pi"]
	if p.MCP || p.EffectiveIsolation != "session" {
		t.Fatalf("pi has a session hook, no MCP: %+v", p)
	}
	if Signals("worktree", false)[0].EffectiveIsolation != "worktree" {
		t.Fatal("worktree never degrades")
	}
}
