package hook

import "sort"

// Signal is what one harness can tell boxer (requirements §7, "Signals, not hooks"): each field is
// derived from the dialect table, never from a per-harness code path. GitHook is the repository's
// post-checkout hook and is the same for every harness; the caller fills it in.
type Signal struct {
	Harness       string `json:"harness"`
	SessionStart  bool   `json:"session_start"`
	Rewrite       bool   `json:"rewrite"`
	BlockOnly     bool   `json:"block_only"`
	SubagentStart bool   `json:"subagent_start"`
	SessionEnd    bool   `json:"session_end"`
	MCP           bool   `json:"mcp"`
	GitHook       bool   `json:"git_hook"`
	// EffectiveIsolation is min(configured, signals present) (R-SIG-3).
	EffectiveIsolation string `json:"effective_isolation"`
}

// Signals reports, per harness, which signals boxer can receive, and the isolation that the
// configured value degrades to given them.
func Signals(isolation string, gitHook bool) []Signal {
	var out []Signal
	for _, d := range Dialects {
		has := func(purpose string) bool {
			for _, p := range d.Events {
				if p == purpose {
					return true
				}
			}
			return false
		}
		s := Signal{Harness: d.Name, SessionStart: has("session_start"), SubagentStart: has("subagent_start"), SessionEnd: has("session_end"),
			Rewrite: d.Rewrite && has("intercept"), BlockOnly: !d.Rewrite && has("intercept"), MCP: d.MCP, GitHook: gitHook}
		s.EffectiveIsolation = isolation
		if isolation == "subagent" && !s.SubagentStart {
			s.EffectiveIsolation = "session"
		}
		if s.EffectiveIsolation == "session" && !s.SessionStart && !s.MCP {
			s.EffectiveIsolation = "worktree"
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Harness < out[j].Harness })
	return out
}
