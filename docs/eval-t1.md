# boxer eval report — tier t1 — 2026-09-18T00:41:13+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 7.433s |  |
| acp-codex/inside/acp/worktree | pass | 4.519s |  |
| acp-gemini/inside/acp/worktree | pass | 4.72s |  |
| acp-grok/inside/acp/worktree | pass | 5.205s |  |
| acp-kimi/inside/acp/worktree | pass | 4.693s |  |
| acp-opencode/inside/acp/worktree | pass | 5.979s |  |
| claude-code/off/plugin/worktree | pass | 3.703s |  |
| claude-code/rewrite/both/worktree | pass | 4.06s |  |
| claude-code/rewrite/plugin/repo | pass | 3.471s |  |
| claude-code/rewrite/plugin/session | pass | 3.818s |  |
| claude-code/rewrite/plugin/subagent | pass | 4.56s |  |
| claude-code/rewrite/plugin/worktree | pass | 4.127s |  |
| claude-code/rewrite/plugin/worktree/timing-before | pass | 3.703s |  |
| claude-code/rewrite/plugin/worktree/timing-mid | pass | 4.621s |  |
| claude-code/rewrite/plugin/worktree/timing-never | pass | 3.651s |  |
| claude-code/rewrite/plugin/worktree/timing-warm | pass | 3.236s |  |
| claude-code/rewrite/project/worktree | pass | 3.736s |  |
| claude-code/rewrite/user/worktree | pass | 3.635s |  |
| claude-code/tool/plugin/worktree | pass | 3.078s |  |
| claude-code/tool/plugin/worktree/noncompliant | pass | 3.042s | 1 denial(s); |
| claude-code/tool/project/worktree | pass | 3.258s |  |
| codex/off/project/worktree | pass | 2.717s |  |
| codex/rewrite/both/worktree | pass | 2.936s |  |
| codex/rewrite/project/worktree | pass | 3.572s |  |
| codex/rewrite/user/worktree | pass | 3.212s |  |
| codex/tool/user/worktree | pass | 3.1s |  |
| codex/tool/user/worktree/noncompliant | pass | 2.932s | 1 denial(s); |
| gemini-cli/off/plugin/worktree | pass | 3.344s |  |
| gemini-cli/rewrite/plugin/worktree | pass | 3.712s |  |
| gemini-cli/rewrite/project/worktree | pass | 2.16s |  |
| gemini-cli/tool/plugin/worktree | pass | 3.557s |  |
| gemini-cli/tool/plugin/worktree/noncompliant | pass | 3.424s |  |
| grok/off/user/worktree | pass | 2.521s |  |
| grok/rewrite/both/worktree | pass | 3.02s |  |
| grok/rewrite/project/worktree | pass | 2.659s |  |
| grok/rewrite/user/worktree | pass | 3.073s |  |
| grok/tool/user/worktree/noncompliant | pass | 2.635s | 1 denial(s); |
| inside-claude/inside/shell/worktree | pass | 7.647s |  |
| inside-codex/inside/shell/worktree | pass | 6.634s |  |
| inside-gemini/inside/shell/worktree | pass | 4.808s |  |
| inside-grok/inside/shell/worktree | pass | 4.749s |  |
| inside-kimi/inside/shell/worktree | pass | 4.987s |  |
| inside-opencode/inside/shell/worktree | pass | 6.414s |  |
| inside-pi/inside/shell/worktree | pass | 8.744s |  |
| kimi/off/user/worktree | pass | 1.748s |  |
| kimi/tool/user/worktree | pass | 1.923s |  |
| kimi/tool/user/worktree/noncompliant | pass | 1.789s | 1 denial(s); |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 4.229s |  |
| opencode/rewrite/project/worktree | pass | 9.053s |  |
| opencode/tool/project/worktree | pass | 4.41s |  |
| opencode/tool/project/worktree/noncompliant | pass | 4.361s | 1 denial(s); |
| openhands/rewrite/sdk/worktree | skip | 0s | no OpenHands virtualenv: python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools (or set BOXER_OPENHANDS_PYTHON) |
| paperclip/rewrite/project/worktree | skip | 0s | paperclipai is not installed (npm i -g paperclipai, then paperclipai test-drive --harness claude --no-browser); checklist in docs/orchestrators.md |
| pi/off/project/worktree | pass | 1.808s |  |
| pi/rewrite/project/worktree | pass | 2.172s |  |
| pi/tool/project/worktree | pass | 1.845s |  |
| pi/tool/project/worktree/noncompliant | pass | 1.72s | 1 denial(s); |
| t3code/rewrite/project/worktree | skip | 0s | t3 is not installed (npm i -g t3); checklist in docs/orchestrators.md |

**passed 55 · failed 0 · skipped 4**
