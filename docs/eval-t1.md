# boxer eval report — tier t1 — 2026-09-18T00:07:52+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 7.127s |  |
| acp-codex/inside/acp/worktree | pass | 4.652s |  |
| acp-gemini/inside/acp/worktree | pass | 4.633s |  |
| acp-grok/inside/acp/worktree | pass | 5.251s |  |
| acp-kimi/inside/acp/worktree | pass | 4.496s |  |
| acp-opencode/inside/acp/worktree | pass | 6.073s |  |
| claude-code/off/plugin/worktree | pass | 3.506s |  |
| claude-code/rewrite/both/worktree | pass | 3.831s |  |
| claude-code/rewrite/plugin/repo | pass | 3.526s |  |
| claude-code/rewrite/plugin/session | fail | 3.94s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1568656591scope: one VM per session wanted, got [sb-4dd696fc7504 sb-7119f82ed6f6]; |
| claude-code/rewrite/plugin/subagent | pass | 8.411s |  |
| claude-code/rewrite/plugin/worktree | pass | 3.686s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 3.96s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.872s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 3.655s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 3.204s |  |
| claude-code/rewrite/project/worktree | pass | 3.806s |  |
| claude-code/rewrite/user/worktree | pass | 3.551s |  |
| claude-code/tool/plugin/worktree | pass | 3.301s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 3.117s |  |
| claude-code/tool/project/worktree | pass | 3.392s |  |
| codex/off/project/worktree | pass | 2.56s |  |
| codex/rewrite/both/worktree | pass | 2.754s |  |
| codex/rewrite/project/worktree | pass | 3.492s |  |
| codex/rewrite/user/worktree | pass | 3s |  |
| codex/tool/user/worktree | pass | 9.835s |  |
| codex/tool/user/worktree/noncompliant | fail | 1.825s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-352102166vm: no VM sb-2f6ab8b86a9e after the run; |
| gemini-cli/off/plugin/worktree | pass | 3.353s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 3.753s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.179s |  |
| gemini-cli/tool/plugin/worktree | pass | 3.474s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 3.384s |  |
| grok/off/user/worktree | fail | 127ms | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1308435362prepare: open /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1308435362/dist/grok/hooks/hooks.json: no such file or directory; |
| grok/rewrite/both/worktree | pass | 2.886s |  |
| grok/rewrite/project/worktree | pass | 3.023s |  |
| grok/rewrite/user/worktree | fail | 98ms | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3449934306prepare: open /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3449934306/dist/grok/hooks/hooks.json: no such file or directory; |
| grok/tool/user/worktree/noncompliant | fail | 144ms | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2253370550prepare: open /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2253370550/dist/grok/hooks/hooks.json: no such file or directory; |
| inside-claude/inside/shell/worktree | pass | 8.268s |  |
| inside-codex/inside/shell/worktree | pass | 6.748s |  |
| inside-gemini/inside/shell/worktree | pass | 5.032s |  |
| inside-grok/inside/shell/worktree | pass | 4.855s |  |
| inside-kimi/inside/shell/worktree | pass | 5.144s |  |
| inside-opencode/inside/shell/worktree | pass | 6.393s |  |
| inside-pi/inside/shell/worktree | pass | 8.547s |  |
| kimi/off/user/worktree | pass | 1.728s |  |
| kimi/tool/user/worktree | pass | 1.964s |  |
| kimi/tool/user/worktree/noncompliant | pass | 1.785s |  |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 4.334s |  |
| opencode/rewrite/project/worktree | pass | 9.059s |  |
| opencode/tool/project/worktree | pass | 4.357s |  |
| opencode/tool/project/worktree/noncompliant | pass | 4.122s |  |
| openhands/rewrite/sdk/worktree | skip | 0s | no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON) |
| paperclip/rewrite/project/worktree | skip | 0s | paperclipai is not installed (npm i -g paperclipai, then paperclipai test-drive --harness claude --no-browser); checklist in docs/orchestrators.md |
| pi/off/project/worktree | pass | 1.725s |  |
| pi/rewrite/project/worktree | pass | 1.921s |  |
| pi/tool/project/worktree | pass | 1.931s |  |
| pi/tool/project/worktree/noncompliant | pass | 1.793s |  |
| t3code/rewrite/project/worktree | skip | 0s | t3 is not installed (npm i -g t3); checklist in docs/orchestrators.md |

**passed 50 · failed 5 · skipped 4**
