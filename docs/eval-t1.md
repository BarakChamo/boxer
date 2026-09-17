# boxer eval report — tier t1 — 2026-09-17T19:31:42+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 25.428s |  |
| acp-codex/inside/acp/worktree | pass | 15.06s |  |
| acp-gemini/inside/acp/worktree | pass | 13.183s |  |
| acp-grok/inside/acp/worktree | pass | 17.58s |  |
| acp-kimi/inside/acp/worktree | pass | 13.453s |  |
| acp-opencode/inside/acp/worktree | pass | 13.136s |  |
| claude-code/off/plugin/worktree | pass | 4.123s |  |
| claude-code/rewrite/both/worktree | pass | 4.104s |  |
| claude-code/rewrite/plugin/repo | pass | 4.941s |  |
| claude-code/rewrite/plugin/worktree | pass | 4.652s |  |
| claude-code/rewrite/project/worktree | pass | 4.305s |  |
| claude-code/rewrite/user/worktree | pass | 4.604s |  |
| claude-code/tool/plugin/worktree | pass | 4.198s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 4.857s |  |
| claude-code/tool/project/worktree | pass | 4.105s |  |
| codex/off/project/worktree | pass | 3.575s |  |
| codex/rewrite/both/worktree | pass | 3.566s |  |
| codex/rewrite/project/worktree | pass | 3.762s |  |
| codex/rewrite/user/worktree | pass | 2.972s |  |
| codex/tool/user/worktree | pass | 2.909s |  |
| codex/tool/user/worktree/noncompliant | pass | 3.018s |  |
| gemini-cli/off/plugin/worktree | pass | 7.558s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 5.754s |  |
| gemini-cli/rewrite/project/worktree | pass | 3.284s |  |
| gemini-cli/tool/plugin/worktree | pass | 6.507s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 5.71s |  |
| grok/off/user/worktree | pass | 3.642s |  |
| grok/rewrite/both/worktree | pass | 3.381s |  |
| grok/rewrite/project/worktree | pass | 3.811s |  |
| grok/rewrite/user/worktree | pass | 4.929s |  |
| grok/tool/user/worktree/noncompliant | pass | 3.541s |  |
| inside-claude/inside/shell/worktree | pass | 16.591s |  |
| inside-codex/inside/shell/worktree | pass | 9.35s |  |
| inside-gemini/inside/shell/worktree | pass | 9.909s |  |
| inside-grok/inside/shell/worktree | pass | 12.11s |  |
| inside-kimi/inside/shell/worktree | pass | 9.417s |  |
| inside-opencode/inside/shell/worktree | pass | 13.643s |  |
| inside-pi/inside/shell/worktree | pass | 27.976s |  |
| kimi/off/user/worktree | pass | 3.175s |  |
| kimi/tool/user/worktree | pass | 3.04s |  |
| kimi/tool/user/worktree/noncompliant | pass | 3.586s |  |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 8.353s |  |
| opencode/rewrite/project/worktree | pass | 15.212s |  |
| opencode/tool/project/worktree | pass | 9.887s |  |
| opencode/tool/project/worktree/noncompliant | pass | 10.705s |  |
| openhands/rewrite/sdk/worktree | skip | 0s | no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON) |
| paperclip/rewrite/project/worktree | skip | 0s | paperclipai is not installed (npm i -g paperclipai, then paperclipai test-drive --harness claude --no-browser); checklist in docs/orchestrators.md |
| pi/off/project/worktree | pass | 3.244s |  |
| pi/rewrite/project/worktree | pass | 3.084s |  |
| pi/tool/project/worktree | pass | 2.642s |  |
| pi/tool/project/worktree/noncompliant | pass | 2.413s |  |
| t3code/rewrite/project/worktree | skip | 0s | t3 is not installed (npm i -g t3); checklist in docs/orchestrators.md |

**passed 49 · failed 0 · skipped 4**
