# boxer eval report — tier t2 — 2026-09-30T15:23:05+07:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 12.2s | $0.0030; |
| acp-codex/inside/acp/worktree | pass | 8.546s | $0.0029; |
| acp-gemini/inside/acp/worktree | skip | 108ms | $0.0028; inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 9.443s | $0.0096; |
| acp-kimi/inside/acp/worktree | pass | 7.583s | $0.0059; |
| acp-opencode/inside/acp/worktree | pass | 8.047s | $0.0034; |
| claude-code/off/plugin/worktree | pass | 5.468s | $0.0031; |
| claude-code/rewrite/both/worktree | pass | 6.034s | $0.0018; |
| claude-code/rewrite/plugin/repo | pass | 7.052s | $0.0016; |
| claude-code/rewrite/plugin/worktree | pass | 7.26s | $0.0033; |
| claude-code/rewrite/project/worktree | pass | 5.561s | $0.0017; |
| claude-code/tool/plugin/worktree | pass | 5.736s | $0.0017; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 5.01s | $0.0015; |
| codex/off/project/worktree | pass | 5.966s | $0.0029; |
| codex/rewrite/project/worktree | pass | 6.423s | $0.0018; |
| copilot/off/user/worktree | pass | 8.395s | $0.0028; |
| copilot/rewrite/user/worktree | pass | 9.93s | $0.0024; |
| copilot/tool/user/worktree | fail | 10.236s | 1 denial(s); $0.0042; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3374912089deny: 1 denial(s): the agent had to be corrected; |
| dsh/off/project/worktree | pass | 4.319s | $0.0024; |
| dsh/tool/project/worktree | pass | 9.018s | $0.0025; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 5.191s | $0.0093; |
| grok/rewrite/project/worktree | pass | 7.168s | $0.0098; |
| grok/rewrite/user/worktree | pass | 6.187s | $0.0058; |
| grok/tool/user/worktree | fail | 9.396s | 1 denial(s); $0.0142; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1766343778guest: final answer "Darwin", wanted "Linux"; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| herdr/rewrite/project/worktree | pass | 11.858s | $0.0037; |
| inside-claude/inside/shell/worktree | pass | 51.898s | $0.0028; |
| inside-codex/inside/shell/worktree | pass | 28.15s | $0.0017; |
| inside-copilot/inside/shell/worktree | pass | 29.647s | $0.0060; |
| inside-gemini/inside/shell/worktree | skip | 104ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 24.018s | $0.0059; |
| inside-kimi/inside/shell/worktree | pass | 25.24s | $0.0036; |
| inside-opencode/inside/shell/worktree | pass | 27.893s | $0.0036; |
| inside-pi/inside/shell/worktree | pass | 27.387s | $0.0005; |
| kimi/off/user/worktree | pass | 7.83s | $0.0022; |
| kimi/tool/user/worktree | fail | 7.406s | 1 denial(s); $0.0068; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-542341744deny: 1 denial(s): the agent had to be corrected; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 6.759s | $0.0034; |
| opencode/rewrite/project/worktree | pass | 12.779s | $0.0040; |
| opencode/tool/project/worktree | pass | 7.599s | $0.0039; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 11.047s | $0.0044; |
| paperclip/rewrite/project/worktree | pass | 48.779s | $0.0229; |
| pi/off/project/worktree | pass | 3.521s | $0.0008; |
| pi/rewrite/project/worktree | pass | 4.633s | $0.0008; |
| pi/tool/project/worktree | pass | 4.499s | $0.0008; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 53.471s | $0.0041; |
| t3code/rewrite/project/worktree | pass | 11.284s | $0.0046; |

**passed 40 · failed 3 · skipped 14**

**gateway spend this run: $0.1868** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
