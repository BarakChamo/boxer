# boxer eval report — tier t1 — 2026-09-18T00:28:01+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 7.47s |  |
| acp-codex/inside/acp/worktree | pass | 4.802s |  |
| acp-gemini/inside/acp/worktree | pass | 4.713s |  |
| acp-grok/inside/acp/worktree | pass | 5.296s |  |
| acp-kimi/inside/acp/worktree | pass | 4.588s |  |
| acp-opencode/inside/acp/worktree | pass | 6.194s |  |
| claude-code/off/plugin/worktree | pass | 3.439s |  |
| claude-code/rewrite/both/worktree | pass | 3.736s |  |
| claude-code/rewrite/plugin/repo | pass | 3.671s |  |
| claude-code/rewrite/plugin/session | pass | 3.723s |  |
| claude-code/rewrite/plugin/subagent | pass | 4.821s |  |
| claude-code/rewrite/plugin/worktree | pass | 4.077s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 3.848s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.971s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 3.442s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 3.375s |  |
| claude-code/rewrite/project/worktree | pass | 3.466s |  |
| claude-code/rewrite/user/worktree | pass | 3.398s |  |
| claude-code/tool/plugin/worktree | pass | 3.254s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 3.555s | 1 denial(s); |
| claude-code/tool/project/worktree | pass | 3.126s |  |
| codex/off/project/worktree | pass | 2.538s |  |
| codex/rewrite/both/worktree | fail | 2.591s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-452052448guest: final answer "Chunk", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| codex/rewrite/project/worktree | fail | 2.67s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2326826467guest: final answer "Chunk", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| codex/rewrite/user/worktree | fail | 2.397s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4232031999guest: final answer "Chunk", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| codex/tool/user/worktree | pass | 2.538s |  |
| codex/tool/user/worktree/noncompliant | pass | 2.215s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 3.566s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 3.769s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.231s |  |
| gemini-cli/tool/plugin/worktree | pass | 3.352s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 3.232s |  |
| grok/off/user/worktree | pass | 2.667s |  |
| grok/rewrite/both/worktree | pass | 3.07s |  |
| grok/rewrite/project/worktree | pass | 2.859s |  |
| grok/rewrite/user/worktree | pass | 2.918s |  |
| grok/tool/user/worktree/noncompliant | pass | 2.381s | 1 denial(s); |
| inside-claude/inside/shell/worktree | pass | 8.092s |  |
| inside-codex/inside/shell/worktree | pass | 6.021s |  |
| inside-gemini/inside/shell/worktree | pass | 4.521s |  |
| inside-grok/inside/shell/worktree | pass | 4.874s |  |
| inside-kimi/inside/shell/worktree | pass | 5.141s |  |
| inside-opencode/inside/shell/worktree | pass | 5.914s |  |
| inside-pi/inside/shell/worktree | pass | 8.557s |  |
| kimi/off/user/worktree | pass | 1.794s |  |
| kimi/tool/user/worktree | pass | 1.974s |  |
| kimi/tool/user/worktree/noncompliant | pass | 1.775s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 4.402s |  |
| opencode/rewrite/project/worktree | pass | 9.188s |  |
| opencode/tool/project/worktree | pass | 4.317s |  |
| opencode/tool/project/worktree/noncompliant | pass | 4.255s | 1 denial(s); |
| openhands/rewrite/sdk/worktree | skip | 0s | no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON) |
| paperclip/rewrite/project/worktree | skip | 0s | paperclipai is not installed (npm i -g paperclipai, then paperclipai test-drive --harness claude --no-browser); checklist in docs/orchestrators.md |
| pi/off/project/worktree | pass | 1.757s |  |
| pi/rewrite/project/worktree | pass | 2.157s |  |
| pi/tool/project/worktree | pass | 1.917s |  |
| pi/tool/project/worktree/noncompliant | pass | 1.789s | 1 denial(s); |
| t3code/rewrite/project/worktree | skip | 0s | t3 is not installed (npm i -g t3); checklist in docs/orchestrators.md |

**passed 52 · failed 3 · skipped 4**
