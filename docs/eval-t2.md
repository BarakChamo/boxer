# boxer eval report — tier t2 — 2026-09-30T18:40:30+07:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 14.238s | $0.0031; |
| acp-codex/inside/acp/worktree | pass | 8.39s | $0.0016; |
| acp-gemini/inside/acp/worktree | skip | 105ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 8.254s | $0.0058; |
| acp-kimi/inside/acp/worktree | pass | 6.586s | $0.0045; |
| acp-opencode/inside/acp/worktree | pass | 8.26s | $0.0036; |
| claude-code/off/plugin/worktree | pass | 5.407s | $0.0032; |
| claude-code/rewrite/both/worktree | pass | 5.582s | $0.0031; |
| claude-code/rewrite/plugin/repo | pass | 5.41s | $0.0035; |
| claude-code/rewrite/plugin/worktree | pass | 6.376s | $0.0030; |
| claude-code/rewrite/project/worktree | pass | 5.615s | $0.0050; |
| claude-code/tool/plugin/worktree | pass | 4.962s | $0.0030; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 6.316s | $0.0031; |
| codex/off/project/worktree | pass | 5.225s | $0.0029; |
| codex/rewrite/project/worktree | pass | 6.245s | $0.0029; |
| copilot/off/user/worktree | pass | 24.128s | $0.0025; |
| copilot/rewrite/user/worktree | pass | 8.704s | $0.0073; |
| copilot/tool/user/worktree | fail | 7.275s | $0.0020; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1806193618guest: final answer "The", wanted "Linux"; path: tool mode but neither boxer_run nor `boxer run` was used (tools: []); guest-canary: the canary was not written in the guest; |
| dsh/off/project/worktree | pass | 8.452s | $0.0025; |
| dsh/tool/project/worktree | pass | 6.084s | $0.0025; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 5.859s | $0.0058; |
| grok/rewrite/project/worktree | pass | 12.583s | $0.0146; |
| grok/rewrite/user/worktree | pass | 6.423s | $0.0097; |
| grok/tool/user/worktree | pass | 6.869s | $0.0139; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| herdr/rewrite/project/worktree | pass | 32.886s | $0.0039; |
| inside-claude/inside/shell/worktree | pass | 15.272s | $0.0028; |
| inside-codex/inside/shell/worktree | pass | 28.728s | $0.0026; |
| inside-copilot/inside/shell/worktree | pass | 28.932s | $0.0065; |
| inside-gemini/inside/shell/worktree | skip | 106ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 9.988s | $0.0059; |
| inside-kimi/inside/shell/worktree | pass | 28.531s | $0.0036; |
| inside-opencode/inside/shell/worktree | pass | 21.503s | $0.0036; |
| inside-pi/inside/shell/worktree | pass | 35.796s | $0.0003; |
| kimi/off/user/worktree | pass | 8.628s | $0.0062; |
| kimi/tool/user/worktree | pass | 5.272s | $0.0037; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 7.067s | $0.0039; |
| opencode/rewrite/project/worktree | fail | 13.024s | $0.0032; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2210114204path: rewrite mode but no rewrite in the trace, no boxer run typed, no boxer_run tool; guest-canary: the canary was not written in the guest; |
| opencode/tool/project/worktree | pass | 20.077s | $0.0041; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 10.797s | $0.0072; |
| paperclip/rewrite/project/worktree | pass | 24.685s | $0.0101; |
| pi/off/project/worktree | pass | 3.707s | $0.0007; |
| pi/rewrite/project/worktree | pass | 4.572s | $0.0008; |
| pi/tool/project/worktree | fail | 17.157s | 1 denial(s); $0.0009; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2268464001deny: 1 denial(s): the agent had to be corrected; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 45.699s | $0.0040; |
| t3code/rewrite/project/worktree | pass | 11.439s | $0.0046; |

**passed 40 · failed 3 · skipped 14**

**gateway spend this run: $0.1876** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
