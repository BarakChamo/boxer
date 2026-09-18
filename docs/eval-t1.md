# boxer eval report — tier t1 — 2026-09-18T09:35:04+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 11.927s |  |
| acp-codex/inside/acp/worktree | pass | 8.101s |  |
| acp-gemini/inside/acp/worktree | pass | 6.848s |  |
| acp-grok/inside/acp/worktree | pass | 8.417s |  |
| acp-kimi/inside/acp/worktree | pass | 7.081s |  |
| acp-opencode/inside/acp/worktree | pass | 10.496s |  |
| claude-code/off/plugin/worktree | pass | 3.758s |  |
| claude-code/rewrite/both/worktree | pass | 4.233s |  |
| claude-code/rewrite/plugin/repo | pass | 4.004s |  |
| claude-code/rewrite/plugin/session | pass | 4.299s |  |
| claude-code/rewrite/plugin/subagent | pass | 5.794s |  |
| claude-code/rewrite/plugin/worktree | pass | 4.348s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 4.577s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 5.868s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 4.34s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 3.921s |  |
| claude-code/rewrite/project/worktree | pass | 4.161s |  |
| claude-code/rewrite/user/worktree | pass | 4.306s |  |
| claude-code/tool/plugin/worktree | pass | 3.813s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 3.926s | 1 denial(s); |
| claude-code/tool/project/worktree | pass | 3.532s |  |
| codex/off/project/worktree | pass | 3.022s |  |
| codex/rewrite/both/worktree | pass | 3.221s |  |
| codex/rewrite/project/worktree | pass | 3.832s |  |
| codex/rewrite/user/worktree | pass | 3.346s |  |
| codex/tool/user/worktree | pass | 3.364s |  |
| codex/tool/user/worktree/noncompliant | pass | 3.074s | 1 denial(s); |
| dsh/off/project/worktree | pass | 2.417s |  |
| dsh/tool/project/worktree | pass | 3.007s |  |
| dsh/tool/project/worktree/noncompliant | pass | 2.538s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 4.022s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 4.582s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.742s |  |
| gemini-cli/tool/plugin/worktree | pass | 4.171s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 3.947s |  |
| grok/off/user/worktree | pass | 2.852s |  |
| grok/rewrite/both/worktree | pass | 3.086s |  |
| grok/rewrite/project/worktree | pass | 3.067s |  |
| grok/rewrite/user/worktree | pass | 3.347s |  |
| grok/tool/user/worktree/noncompliant | pass | 2.858s | 1 denial(s); |
| inside-claude/inside/shell/worktree | pass | 12.392s |  |
| inside-codex/inside/shell/worktree | pass | 10.11s |  |
| inside-gemini/inside/shell/worktree | pass | 8.754s |  |
| inside-grok/inside/shell/worktree | pass | 8.251s |  |
| inside-kimi/inside/shell/worktree | pass | 7.607s |  |
| inside-opencode/inside/shell/worktree | pass | 10.501s |  |
| inside-pi/inside/shell/worktree | pass | 13.42s |  |
| kimi/off/user/worktree | pass | 2.314s |  |
| kimi/tool/user/worktree | pass | 2.619s |  |
| kimi/tool/user/worktree/noncompliant | pass | 2.366s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 4.896s |  |
| opencode/rewrite/project/worktree | pass | 9.648s |  |
| opencode/tool/project/worktree | pass | 4.86s |  |
| opencode/tool/project/worktree/noncompliant | pass | 7.163s | 1 denial(s); |
| openhands/rewrite/sdk/worktree | skip | 0s | no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON) |
| paperclip/rewrite/project/worktree | pass | 15.446s |  |
| pi/off/project/worktree | pass | 2.25s |  |
| pi/rewrite/project/worktree | pass | 2.926s |  |
| pi/tool/project/worktree | pass | 2.368s |  |
| pi/tool/project/worktree/noncompliant | pass | 2.224s | 1 denial(s); |
| t3code/inside/shim/worktree | pass | 19.201s |  |
| t3code/rewrite/project/worktree | pass | 8.789s |  |

**passed 61 · failed 0 · skipped 2**
