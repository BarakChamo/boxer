# boxer eval report — tier t2 — 2026-09-18T11:26:28+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 15.405s | $0.0051; |
| acp-codex/inside/acp/worktree | pass | 17.959s | $0.0026; |
| acp-gemini/inside/acp/worktree | skip | 157ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 14.769s | $0.0091; |
| acp-kimi/inside/acp/worktree | pass | 10.729s | $0.0035; |
| acp-opencode/inside/acp/worktree | pass | 13.291s | $0.0047; |
| claude-code/off/plugin/worktree | pass | 21.065s | $0.0049; |
| claude-code/rewrite/both/worktree | pass | 15.224s | $0.0049; |
| claude-code/rewrite/plugin/repo | pass | 13.013s | $0.0033; |
| claude-code/rewrite/plugin/worktree | pass | 27.356s | $0.0032; |
| claude-code/rewrite/project/worktree | pass | 11.921s | $0.0048; |
| claude-code/tool/plugin/worktree | pass | 11.518s | $0.0030; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 15.046s | $0.0049; |
| codex/off/project/worktree | pass | 11.518s | $0.0029; |
| codex/rewrite/project/worktree | pass | 13.758s | $0.0029; |
| dsh/off/project/worktree | pass | 31.004s | $0.0029; |
| dsh/tool/project/worktree | pass | 8.427s | $0.0024; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 15.729s | $0.0084; |
| grok/rewrite/project/worktree | pass | 10.755s | $0.0125; |
| grok/rewrite/user/worktree | pass | 11.829s | $0.0086; |
| grok/tool/user/worktree | pass | 27.509s | 1 denial(s); $0.0187; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| inside-claude/inside/shell/worktree | pass | 12.35s | $0.0028; |
| inside-codex/inside/shell/worktree | pass | 15.621s | $0.0026; |
| inside-gemini/inside/shell/worktree | skip | 164ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 25.488s | $0.0130; |
| inside-kimi/inside/shell/worktree | pass | 20.604s | $0.0041; |
| inside-opencode/inside/shell/worktree | pass | 12.691s | $0.0032; |
| inside-pi/inside/shell/worktree | pass | 12.824s | $0.0005; |
| kimi/off/user/worktree | pass | 14.138s | $0.0050; |
| kimi/tool/user/worktree | pass | 14.97s | $0.0082; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 11.065s | $0.0030; |
| opencode/rewrite/project/worktree | pass | 16.206s | $0.0047; |
| opencode/tool/project/worktree | pass | 10.573s | $0.0050; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 51.569s | $0.0061; |
| paperclip/rewrite/project/worktree | pass | 35.462s | $0.0130; |
| pi/off/project/worktree | pass | 5.9s | $0.0007; |
| pi/rewrite/project/worktree | pass | 9.616s | $0.0013; |
| pi/tool/project/worktree | pass | 5.237s | $0.0007; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 1m25.233s | $0.0070; |
| t3code/rewrite/project/worktree | pass | 14.916s | $0.0046; |

**passed 38 · failed 0 · skipped 14**

**gateway spend this run: $0.1986** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
