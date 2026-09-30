# boxer eval report — tier t1 — 2026-09-30T12:13:16+07:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 10.342s |  |
| acp-codex/inside/acp/worktree | pass | 6.758s |  |
| acp-gemini/inside/acp/worktree | pass | 5.357s |  |
| acp-grok/inside/acp/worktree | pass | 5.003s | $0.0361; |
| acp-kimi/inside/acp/worktree | pass | 5.511s | $0.0359; |
| acp-opencode/inside/acp/worktree | pass | 6.367s | $0.0536; |
| claude-code/off/plugin/worktree | pass | 3.086s | $0.0011; |
| claude-code/rewrite/both/worktree | pass | 3.173s | $0.0045; |
| claude-code/rewrite/plugin/repo | pass | 2.932s | $0.0057; |
| claude-code/rewrite/plugin/session | pass | 3.371s |  |
| claude-code/rewrite/plugin/subagent | pass | 4.165s |  |
| claude-code/rewrite/plugin/worktree | pass | 7.721s | $0.0132; |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 2.858s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.248s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 2.873s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 2.796s |  |
| claude-code/rewrite/project/worktree | pass | 2.854s | $0.0058; |
| claude-code/rewrite/user/worktree | pass | 3.163s |  |
| claude-code/tool/plugin/worktree | pass | 2.672s | $0.0015; |
| claude-code/tool/plugin/worktree/noncompliant | pass | 2.922s | 1 denial(s); $0.0024; |
| claude-code/tool/project/worktree | pass | 2.904s | $0.0051; |
| codex/off/project/worktree | pass | 1.974s |  |
| codex/rewrite/both/worktree | pass | 2.076s |  |
| codex/rewrite/project/worktree | pass | 2.006s |  |
| codex/rewrite/user/worktree | pass | 2.024s |  |
| codex/tool/user/worktree | pass | 2.08s |  |
| codex/tool/user/worktree/noncompliant | pass | 2.194s | 1 denial(s); |
| copilot/off/user/worktree | pass | 5.173s |  |
| copilot/rewrite/user/worktree | pass | 5.643s | $0.0257; |
| copilot/tool/user/worktree | pass | 4.9s |  |
| copilot/tool/user/worktree/noncompliant | pass | 4.898s | 1 denial(s); |
| dsh/off/project/worktree | pass | 1.765s |  |
| dsh/tool/project/worktree | pass | 2.194s | $0.0125; |
| dsh/tool/project/worktree/noncompliant | pass | 1.611s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 2.905s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 3.137s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.148s |  |
| gemini-cli/tool/plugin/worktree | pass | 2.935s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 2.944s | 1 denial(s); |
| grok/off/user/worktree | pass | 1.879s | $0.0145; |
| grok/rewrite/both/worktree | pass | 2.079s | $0.0162; |
| grok/rewrite/project/worktree | pass | 2.052s |  |
| grok/rewrite/user/worktree | pass | 2.47s |  |
| grok/tool/user/worktree/noncompliant | pass | 1.556s | 1 denial(s); |
| herdr/rewrite/project/worktree | pass | 9.125s | $0.1163; |
| inside-claude/inside/shell/worktree | pass | 50.262s | $0.1473; |
| inside-codex/inside/shell/worktree | pass | 25.228s | $0.0178; |
| inside-copilot/inside/shell/worktree | pass | 27.384s | $0.1203; |
| inside-gemini/inside/shell/worktree | pass | 19.516s | $0.0803; |
| inside-grok/inside/shell/worktree | pass | 21.119s | $0.2458; |
| inside-kimi/inside/shell/worktree | pass | 26.508s |  |
| inside-opencode/inside/shell/worktree | pass | 25.507s | $0.0556; |
| inside-pi/inside/shell/worktree | pass | 30.078s | $0.1606; |
| kimi/off/user/worktree | pass | 3.728s |  |
| kimi/tool/user/worktree | pass | 3.776s |  |
| kimi/tool/user/worktree/noncompliant | pass | 3.638s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 3.662s | $0.0003; |
| opencode/rewrite/project/worktree | pass | 12.64s |  |
| opencode/tool/project/worktree | pass | 3.753s |  |
| opencode/tool/project/worktree/noncompliant | pass | 3.671s | 1 denial(s); |
| openhands/rewrite/sdk/worktree | pass | 11.252s | $0.0740; |
| paperclip/rewrite/project/worktree | pass | 15.401s | $0.0862; |
| pi/off/project/worktree | pass | 1.774s |  |
| pi/rewrite/project/worktree | pass | 2.07s | $0.0011; |
| pi/tool/project/worktree | pass | 2.244s | $0.0003; |
| pi/tool/project/worktree/noncompliant | pass | 1.714s | 1 denial(s); |
| t3code/inside/shim/worktree | pass | 47.237s | $0.0935; |
| t3code/rewrite/project/worktree | pass | 9.658s | $0.0632; |

**passed 68 · failed 0 · skipped 1**

**gateway spend this run: $1.4965** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
