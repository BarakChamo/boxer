# boxer eval report — tier t1 — 2026-09-30T00:27:23+07:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 7.243s |  |
| acp-codex/inside/acp/worktree | pass | 5.809s | $0.0016; |
| acp-gemini/inside/acp/worktree | pass | 4.274s | $0.0048; |
| acp-grok/inside/acp/worktree | pass | 5.019s | $0.0129; |
| acp-kimi/inside/acp/worktree | pass | 4.083s | $0.0046; |
| acp-opencode/inside/acp/worktree | pass | 5.955s | $0.0092; |
| claude-code/off/plugin/worktree | pass | 3.184s |  |
| claude-code/rewrite/both/worktree | pass | 3.317s |  |
| claude-code/rewrite/plugin/repo | pass | 3.032s |  |
| claude-code/rewrite/plugin/session | pass | 3.181s |  |
| claude-code/rewrite/plugin/subagent | pass | 4.206s |  |
| claude-code/rewrite/plugin/worktree | pass | 7.475s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 3.232s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.17s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 3.105s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 2.762s |  |
| claude-code/rewrite/project/worktree | pass | 3.266s |  |
| claude-code/rewrite/user/worktree | pass | 3.058s |  |
| claude-code/tool/plugin/worktree | pass | 2.79s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 2.806s | 1 denial(s); |
| claude-code/tool/project/worktree | pass | 2.811s |  |
| codex/off/project/worktree | pass | 1.933s |  |
| codex/rewrite/both/worktree | pass | 2.132s |  |
| codex/rewrite/project/worktree | pass | 1.998s |  |
| codex/rewrite/user/worktree | pass | 2.03s |  |
| codex/tool/user/worktree | pass | 2.09s |  |
| codex/tool/user/worktree/noncompliant | pass | 1.931s | 1 denial(s); |
| copilot/off/user/worktree | pass | 4.999s |  |
| copilot/rewrite/user/worktree | pass | 6.677s |  |
| copilot/tool/user/worktree | pass | 4.769s |  |
| copilot/tool/user/worktree/noncompliant | pass | 5.149s | 1 denial(s); |
| dsh/off/project/worktree | pass | 1.712s |  |
| dsh/tool/project/worktree | pass | 2.142s |  |
| dsh/tool/project/worktree/noncompliant | pass | 1.671s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 2.854s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 2.978s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.04s |  |
| gemini-cli/tool/plugin/worktree | pass | 2.903s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 2.798s | 1 denial(s); |
| grok/off/user/worktree | pass | 2.222s |  |
| grok/rewrite/both/worktree | pass | 2.047s |  |
| grok/rewrite/project/worktree | pass | 2.198s |  |
| grok/rewrite/user/worktree | pass | 2.827s |  |
| grok/tool/user/worktree/noncompliant | pass | 1.995s | 1 denial(s); |
| herdr/rewrite/project/worktree | pass | 8.782s |  |
| inside-claude/inside/shell/worktree | pass | 50.102s |  |
| inside-codex/inside/shell/worktree | pass | 39.221s | $0.0000; |
| inside-copilot/inside/shell/worktree | pass | 28.435s | $0.0000; |
| inside-gemini/inside/shell/worktree | pass | 19.882s |  |
| inside-grok/inside/shell/worktree | pass | 20.717s |  |
| inside-kimi/inside/shell/worktree | pass | 24.295s |  |
| inside-opencode/inside/shell/worktree | pass | 25.707s |  |
| inside-pi/inside/shell/worktree | pass | 28.65s |  |
| kimi/off/user/worktree | pass | 2.481s |  |
| kimi/tool/user/worktree | pass | 3.083s |  |
| kimi/tool/user/worktree/noncompliant | pass | 3.358s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 3.597s |  |
| opencode/rewrite/project/worktree | pass | 9.455s |  |
| opencode/tool/project/worktree | pass | 3.636s |  |
| opencode/tool/project/worktree/noncompliant | pass | 3.647s | 1 denial(s); $0.0000; |
| openhands/rewrite/sdk/worktree | pass | 9.854s | $0.0172; |
| paperclip/rewrite/project/worktree | pass | 14.17s | $0.0244; |
| pi/off/project/worktree | pass | 1.708s |  |
| pi/rewrite/project/worktree | pass | 2.073s |  |
| pi/tool/project/worktree | pass | 1.744s |  |
| pi/tool/project/worktree/noncompliant | pass | 1.685s | 1 denial(s); |
| t3code/inside/shim/worktree | pass | 1m0.114s | $0.0154; |
| t3code/rewrite/project/worktree | pass | 9.69s | $0.0650; |

**passed 68 · failed 0 · skipped 1**

**gateway spend this run: $0.1552** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
