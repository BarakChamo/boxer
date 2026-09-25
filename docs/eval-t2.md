# boxer eval report — tier t2 — 2026-09-22T01:43:39+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 18.599s | $0.0225; |
| acp-codex/inside/acp/worktree | pass | 11.226s | $0.0112; |
| acp-gemini/inside/acp/worktree | skip | 129ms | $0.0013; inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 17.394s | $0.0209; |
| acp-kimi/inside/acp/worktree | pass | 11.068s | $0.0182; |
| acp-opencode/inside/acp/worktree | pass | 13.432s | $0.0238; |
| claude-code/off/plugin/worktree | pass | 11.244s | $0.0174; |
| claude-code/rewrite/both/worktree | pass | 14.582s | $0.0157; |
| claude-code/rewrite/plugin/repo | pass | 33.868s | $0.0413; |
| claude-code/rewrite/plugin/worktree | pass | 11.915s | $0.0222; |
| claude-code/rewrite/project/worktree | pass | 22.863s | $0.0344; |
| claude-code/tool/plugin/worktree | pass | 6.99s | $0.0122; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 21.027s | $0.0264; |
| codex/off/project/worktree | pass | 12.008s | $0.0173; |
| codex/rewrite/project/worktree | pass | 15.114s | $0.0205; |
| copilot/off/user/worktree | pass | 16.137s | $0.0154; |
| copilot/rewrite/user/worktree | pass | 14.704s | $0.0088; |
| copilot/tool/user/worktree | fail | 19.814s | 1 denial(s); $0.0191; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2308130267deny: 1 denial(s): the agent had to be corrected; |
| dsh/off/project/worktree | pass | 40.046s | $0.0150; |
| dsh/tool/project/worktree | pass | 10.401s | $0.0053; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 9.703s | $0.0143; |
| grok/rewrite/project/worktree | pass | 13.338s | $0.0151; |
| grok/rewrite/user/worktree | pass | 11.211s | $0.0340; |
| grok/tool/user/worktree | pass | 12.442s | 1 denial(s); $0.0144; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| herdr/rewrite/project/worktree | pass | 15.058s | $0.0246; |
| inside-claude/inside/shell/worktree | fail | 7.115s | $0.0058; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3349777337guest: final answer "", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| inside-codex/inside/shell/worktree | pass | 3m4.085s | $0.1607; |
| inside-copilot/inside/shell/worktree | pass | 39.182s | $0.0401; |
| inside-gemini/inside/shell/worktree | skip | 110ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 17.015s | $0.0317; |
| inside-kimi/inside/shell/worktree | pass | 27.286s | $0.0218; |
| inside-opencode/inside/shell/worktree | pass | 12.092s | $0.0176; |
| inside-pi/inside/shell/worktree | pass | 33.213s | $0.0281; |
| kimi/off/user/worktree | pass | 9.611s | $0.0098; |
| kimi/tool/user/worktree | fail | 9.957s | 1 denial(s); $0.0106; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4121760502deny: 1 denial(s): the agent had to be corrected; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 22.364s | $0.0203; |
| opencode/rewrite/project/worktree | pass | 14.96s | $0.0210; |
| opencode/tool/project/worktree | pass | 14.154s | $0.0374; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 20.515s | $0.0203; |
| paperclip/rewrite/project/worktree | pass | 2m47.173s | $0.1988; |
| pi/off/project/worktree | pass | 5.911s | $0.0043; |
| pi/rewrite/project/worktree | pass | 2m33.488s | $0.0366; |
| pi/tool/project/worktree | pass | 7.221s | $0.0069; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 54.929s | $0.0507; |
| t3code/rewrite/project/worktree | pass | 16.26s | $0.0224; |

**passed 40 · failed 3 · skipped 14**

**gateway spend this run: $1.2163** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
