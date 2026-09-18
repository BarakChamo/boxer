# boxer eval report — tier t2 — 2026-09-18T11:03:55+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | fail | 17.942s | $0.0041; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4044258921run: session/new: Internal error; |
| acp-codex/inside/acp/worktree | pass | 28.047s | $0.0016; |
| acp-gemini/inside/acp/worktree | skip | 288ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 36.472s | $0.0135; |
| acp-kimi/inside/acp/worktree | pass | 41.624s | $0.0010; |
| acp-opencode/inside/acp/worktree | pass | 1m9.076s | $0.0030; |
| claude-code/off/plugin/worktree | pass | 20.741s | $0.0050; |
| claude-code/rewrite/both/worktree | pass | 23.646s | $0.0030; |
| claude-code/rewrite/plugin/repo | pass | 22.254s | $0.0049; |
| claude-code/rewrite/plugin/worktree | pass | 35.868s | $0.0049; |
| claude-code/rewrite/project/worktree | pass | 19.212s | $0.0048; |
| claude-code/tool/plugin/worktree | pass | 22.234s | $0.0048; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 22.809s | $0.0048; |
| codex/off/project/worktree | pass | 18.475s | $0.0019; |
| codex/rewrite/project/worktree | pass | 15.844s | $0.0028; |
| dsh/off/project/worktree | pass | 11.355s | $0.0025; |
| dsh/tool/project/worktree | pass | 10.025s | $0.0038; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 12.743s | $0.0113; |
| grok/rewrite/project/worktree | pass | 11.381s | $0.0135; |
| grok/rewrite/user/worktree | pass | 12.67s | $0.0091; |
| grok/tool/user/worktree | pass | 19.714s | 1 denial(s); $0.0147; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| inside-claude/inside/shell/worktree | pass | 1m20.349s | $0.0029; |
| inside-codex/inside/shell/worktree | pass | 1m9.071s | $0.0016; |
| inside-gemini/inside/shell/worktree | skip | 145ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 40.16s | $0.0057; |
| inside-kimi/inside/shell/worktree | pass | 35.869s | $0.0050; |
| inside-opencode/inside/shell/worktree | pass | 37.302s | $0.0027; |
| inside-pi/inside/shell/worktree | pass | 54.635s | $0.0008; |
| kimi/off/user/worktree | pass | 30.202s | $0.0071; |
| kimi/tool/user/worktree | pass | 10.907s | $0.0068; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 10.918s | $0.0030; |
| opencode/rewrite/project/worktree | pass | 21.569s | $0.0048; |
| opencode/tool/project/worktree | pass | 12.714s | $0.0035; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | fail | 4m8.452s | $0.0054; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3712876991guest: final answer "", wanted "Linux"; guest-canary: the canary was not written in the guest; |
| paperclip/rewrite/project/worktree | pass | 1m57.08s | $0.0183; |
| pi/off/project/worktree | pass | 5.569s | $0.0006; |
| pi/rewrite/project/worktree | pass | 5.276s | $0.0007; |
| pi/tool/project/worktree | pass | 5.59s | $0.0005; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 1m31.221s | $0.0071; |
| t3code/rewrite/project/worktree | pass | 23.932s | $0.0075; |

**passed 36 · failed 2 · skipped 14**

**gateway spend this run: $0.1990** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
