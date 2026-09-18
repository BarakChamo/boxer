# boxer eval report — tier t1 — 2026-09-19T00:53:19+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 7.029s |  |
| acp-codex/inside/acp/worktree | pass | 5.172s | retried once after an infrastructure failure; |
| acp-gemini/inside/acp/worktree | pass | 4.476s |  |
| acp-grok/inside/acp/worktree | pass | 4.766s |  |
| acp-kimi/inside/acp/worktree | pass | 4.539s |  |
| acp-opencode/inside/acp/worktree | pass | 5.796s |  |
| claude-code/off/plugin/worktree | pass | 3.333s |  |
| claude-code/rewrite/both/worktree | pass | 3.783s |  |
| claude-code/rewrite/plugin/repo | pass | 3.49s |  |
| claude-code/rewrite/plugin/session | pass | 3.624s |  |
| claude-code/rewrite/plugin/subagent | pass | 4.509s |  |
| claude-code/rewrite/plugin/worktree | pass | 4.39s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 3.532s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.548s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 4.034s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 3.288s |  |
| claude-code/rewrite/project/worktree | pass | 3.7s |  |
| claude-code/rewrite/user/worktree | pass | 3.377s |  |
| claude-code/tool/plugin/worktree | pass | 3.319s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 3.02s | 1 denial(s); |
| claude-code/tool/project/worktree | pass | 3.167s |  |
| codex/off/project/worktree | pass | 2.735s |  |
| codex/rewrite/both/worktree | pass | 2.771s |  |
| codex/rewrite/project/worktree | pass | 3.118s |  |
| codex/rewrite/user/worktree | pass | 2.917s |  |
| codex/tool/user/worktree | pass | 3.081s |  |
| codex/tool/user/worktree/noncompliant | pass | 2.96s | 1 denial(s); |
| copilot/off/user/worktree | pass | 8.14s |  |
| copilot/rewrite/user/worktree | pass | 9.148s |  |
| copilot/tool/user/worktree | pass | 7.882s |  |
| copilot/tool/user/worktree/noncompliant | pass | 8.241s | 1 denial(s); |
| dsh/off/project/worktree | pass | 1.755s |  |
| dsh/tool/project/worktree | pass | 2.723s |  |
| dsh/tool/project/worktree/noncompliant | pass | 1.691s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 3.419s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 3.59s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.15s |  |
| gemini-cli/tool/plugin/worktree | pass | 3.385s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 3.236s | 1 denial(s); |
| grok/off/user/worktree | pass | 2.659s |  |
| grok/rewrite/both/worktree | pass | 2.715s |  |
| grok/rewrite/project/worktree | pass | 2.965s |  |
| grok/rewrite/user/worktree | pass | 3.158s |  |
| grok/tool/user/worktree/noncompliant | pass | 2.621s | 1 denial(s); |
| herdr/rewrite/project/worktree | pass | 7.804s |  |
| inside-claude/inside/shell/worktree | pass | 53.846s |  |
| inside-codex/inside/shell/worktree | pass | 43.982s |  |
| inside-copilot/inside/shell/worktree | fail | 32.622s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-619077206guest: final answer "No", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| inside-gemini/inside/shell/worktree | pass | 16.458s |  |
| inside-grok/inside/shell/worktree | pass | 25.108s |  |
| inside-kimi/inside/shell/worktree | pass | 24.599s |  |
| inside-opencode/inside/shell/worktree | pass | 29.447s |  |
| inside-pi/inside/shell/worktree | pass | 24.485s |  |
| kimi/off/user/worktree | pass | 1.717s |  |
| kimi/tool/user/worktree | pass | 2.231s |  |
| kimi/tool/user/worktree/noncompliant | pass | 1.778s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 4.163s |  |
| opencode/rewrite/project/worktree | pass | 10.609s |  |
| opencode/tool/project/worktree | pass | 4.179s |  |
| opencode/tool/project/worktree/noncompliant | pass | 4.196s | 1 denial(s); |
| openhands/rewrite/sdk/worktree | pass | 6.839s |  |
| paperclip/rewrite/project/worktree | pass | 15.88s |  |
| pi/off/project/worktree | pass | 1.706s |  |
| pi/rewrite/project/worktree | pass | 2.346s |  |
| pi/tool/project/worktree | pass | 1.849s |  |
| pi/tool/project/worktree/noncompliant | pass | 1.772s | 1 denial(s); |
| t3code/inside/shim/worktree | pass | 46.55s |  |
| t3code/rewrite/project/worktree | pass | 8.31s |  |

**passed 67 · failed 1 · skipped 1**
