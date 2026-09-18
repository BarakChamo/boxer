# boxer eval report — tier t2 — 2026-09-18T10:16:07+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 25.106s | $0.0030; |
| acp-codex/inside/acp/worktree | pass | 21.16s | $0.0026; |
| acp-gemini/inside/acp/worktree | skip | 170ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 27.429s | $0.0057; |
| acp-kimi/inside/acp/worktree | pass | 27.639s | $0.0035; |
| acp-opencode/inside/acp/worktree | pass | 30.822s | $0.0029; |
| claude-code/off/plugin/worktree | pass | 18.99s | $0.0050; |
| claude-code/rewrite/both/worktree | pass | 20.518s | $0.0033; |
| claude-code/rewrite/plugin/repo | pass | 15.339s | $0.0049; |
| claude-code/rewrite/plugin/worktree | pass | 17.267s | $0.0048; |
| claude-code/rewrite/project/worktree | pass | 16.597s | $0.0037; |
| claude-code/tool/plugin/worktree | pass | 8.766s | $0.0049; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 6.691s | $0.0033; |
| codex/off/project/worktree | pass | 9.041s | $0.0019; |
| codex/rewrite/project/worktree | pass | 10.758s | $0.0029; |
| dsh/off/project/worktree | pass | 14.047s | $0.0021; |
| dsh/tool/project/worktree | pass | 13.198s | $0.0032; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 13.033s | $0.0082; |
| grok/rewrite/project/worktree | pass | 10.57s | $0.0174; |
| grok/rewrite/user/worktree | pass | 12.144s | $0.0055; |
| grok/tool/user/worktree | pass | 23.057s | 1 denial(s); $0.0179; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| inside-claude/inside/shell/worktree | pass | 36.22s | $0.0028; |
| inside-codex/inside/shell/worktree | pass | 1m2.8s | $0.0026; |
| inside-gemini/inside/shell/worktree | skip | 307ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 18.976s | $0.0116; |
| inside-kimi/inside/shell/worktree | pass | 24.181s | $0.0040; |
| inside-opencode/inside/shell/worktree | pass | 24.239s | $0.0047; |
| inside-pi/inside/shell/worktree | pass | 30.363s | $0.0005; |
| kimi/off/user/worktree | pass | 34.762s | $0.0017; |
| kimi/tool/user/worktree | pass | 37.54s | $0.0082; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 25.9s | $0.0054; |
| opencode/rewrite/project/worktree | pass | 16.854s | $0.0047; |
| opencode/tool/project/worktree | pass | 9.938s | $0.0032; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 44.447s | $0.0061; |
| paperclip/rewrite/project/worktree | pass | 3m17.319s | $0.0387; |
| pi/off/project/worktree | pass | 7.491s | $0.0008; |
| pi/rewrite/project/worktree | pass | 9.043s | $0.0008; |
| pi/tool/project/worktree | pass | 5.724s | $0.0006; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | fail | 31.892s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3176781501run: exit status 1: "turn/setPermissionMode failed"; |
| t3code/rewrite/project/worktree | pass | 27.486s | $0.0046; |

**passed 37 · failed 1 · skipped 14**

**gateway spend this run: $0.2076** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
