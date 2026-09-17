# boxer eval report — tier t2 — 2026-09-17T23:40:17+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| claude-code/off/plugin/worktree | fail | 10.546s | $0.0036; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3006116774guest: final answer "Linux", wanted "Darwin"; control: mode off should have run on the host, but the canary /tmp/boxer-leak-431793000 is missing; |
| claude-code/rewrite/both/worktree | pass | 8.425s | $0.0049; |
| claude-code/rewrite/plugin/repo | pass | 8.506s | $0.0031; |
| claude-code/rewrite/plugin/worktree | pass | 11.985s | $0.0032; |
| claude-code/rewrite/project/worktree | pass | 14.789s | $0.0045; |
| claude-code/tool/plugin/worktree | pass | 37.564s | $0.0039; |
| claude-code/tool/project/worktree | pass | 9.852s | $0.0034; |
| codex/off/project/worktree | pass | 9.266s | $0.0028; |
| codex/rewrite/project/worktree | pass | 9.201s | $0.0019; |
| grok/off/user/worktree | fail | 9.076s | $0.0045; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3919701667control: mode off should have run on the host, but the canary /tmp/boxer-leak-977767000 is missing; |
| grok/rewrite/project/worktree | pass | 42.101s | $0.0164; |
| grok/rewrite/user/worktree | pass | 14.728s | $0.0133; |
| grok/tool/user/worktree | fail | 17.971s | $0.0210; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2953069789deny: 1 denial(s): the agent had to be corrected; path: tool mode but neither boxer_run nor `boxer run` was used (tools: [run_terminal_command search_tool use_tool]); |
| inside-claude/inside/shell/worktree | pass | 14.783s | $0.0046; |
| inside-codex/inside/shell/worktree | fail | 15m0.661s | $0.0000; retried once after an infrastructure failure; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2361178513run: boxer shell codex timed out; |
| inside-gemini/inside/shell/worktree | skip | 122ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-kimi/inside/shell/worktree | pass | 9.753s | $0.0038; |
| inside-opencode/inside/shell/worktree | pass | 14.083s | $0.0048; |
| inside-pi/inside/shell/worktree | pass | 14.56s | $0.0005; |
| kimi/off/user/worktree | fail | 30.313s | $0.0039; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4170873678guest: final answer "Linux", wanted "Darwin"; control: mode off should have run on the host, but the canary /tmp/boxer-leak-583722000 is missing; |
| kimi/tool/user/worktree | fail | 26.964s | $0.0050; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-689454545path: tool mode but neither boxer_run nor `boxer run` was used (tools: []); |
| opencode/off/project/worktree | pass | 16.462s | $0.0049; |
| opencode/rewrite/project/worktree | pass | 21.471s | $0.0051; |
| opencode/tool/project/worktree | fail | 12.364s | $0.0048; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3901124419path: tool mode but neither boxer_run nor `boxer run` was used (tools: []); |
| pi/off/project/worktree | pass | 11.783s | $0.0008; |
| pi/rewrite/project/worktree | pass | 13.127s | $0.0006; |
| pi/tool/project/worktree | fail | 8.995s | $0.0006; kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3241181387path: tool mode but neither boxer_run nor `boxer run` was used (tools: []); |

**passed 18 · failed 8 · skipped 1**

**gateway spend this run: $0.1258** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)

_interrupted; the cell in flight is not listed. Run `boxer down --all` to reclaim its VM._
