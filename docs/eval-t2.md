# boxer eval report — tier t2 — 2026-09-29T17:30:29+07:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | fail | 10.532s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1009104443run: session/prompt: Internal error: API Error: 402 A positive credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E%2Fai%3Fmodal%3Dtop-up to continue.: [session/create] sessionId=4ece95e9-1238-4c20-a31a-a66b41f4d24a phase=models durationMs=1 totalMs=215; [session/create] sessionId=4ece95e9-1238-4c20-a31a-a66b41f4d24a phase=modes durationMs=1 totalMs=216; [session/create] sessionId=4ece95e9-1238-4c20-a31a-a66b41f4d24a phase=register durationMs=2 totalMs=218; |
| acp-codex/inside/acp/worktree | skip | 15.188s | provider stopped the turn: quota":{"token_count":null,"model_usage":[]}}}} --- stderr --- |
| acp-gemini/inside/acp/worktree | skip | 111ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | fail | 6.279s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-32501770run: session/prompt: Internal error: "http_status": 402; }; [2m2026-09-29T10:28:51.458460Z[0m [31mERROR[0m git_cli: Command::output() FAILED (spawn error) [3merror[0m[2m=[0mNo such file or directory (os error 2) [3merror_kind[0m[2m=[0mNotFound [3mcwd[0m[2m=[0m/private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-32501770/repo; |
| acp-kimi/inside/acp/worktree | skip | 5.442s | provider stopped the turn: quota, API onboarding, third-party tool setup, and error codes. Use when the user asks how Kimi Code works, how to set something up, or what a Kimi Code error m |
| acp-opencode/inside/acp/worktree | fail | 6.721s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1150008110run: session/prompt: Internal error: A positive credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E%2Fai%3Fmodal%3Dtop-up to continue.: errorName: "APIError",; },; }; |
| claude-code/off/plugin/worktree | fail | 2.029s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1827061603run: exit status 1; |
| claude-code/rewrite/both/worktree | fail | 2.4s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2800323211run: exit status 1; |
| claude-code/rewrite/plugin/repo | fail | 2.215s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1693224361run: exit status 1; |
| claude-code/rewrite/plugin/worktree | fail | 7.049s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2311862924run: exit status 1; |
| claude-code/rewrite/project/worktree | fail | 2.195s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1086618691run: exit status 1; |
| claude-code/tool/plugin/worktree | fail | 2.3s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-438907773run: exit status 1; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | fail | 2.434s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2739146540run: exit status 1; |
| codex/off/project/worktree | fail | 12.673s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3256314645run: exit status 1; |
| codex/rewrite/project/worktree | fail | 14.745s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2711586078run: exit status 1; |
| copilot/off/user/worktree | fail | 6.324s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2645692082run: exit status 1; |
| copilot/rewrite/user/worktree | fail | 7.129s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3983636459run: exit status 1; |
| copilot/tool/user/worktree | fail | 5.654s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1096403495run: exit status 1; |
| dsh/off/project/worktree | fail | 2.369s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-21613719run: exit status 1; |
| dsh/tool/project/worktree | fail | 2.743s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3485984911run: exit status 1; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | fail | 2.696s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2270435810run: exit status 1; |
| grok/rewrite/project/worktree | fail | 2.765s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2828692423run: exit status 1; |
| grok/rewrite/user/worktree | fail | 3.012s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2239240196run: exit status 1; |
| grok/tool/user/worktree | fail | 2.426s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-76410486run: exit status 1; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| herdr/rewrite/project/worktree | skip | 9.009s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7 |
| inside-claude/inside/shell/worktree | skip | 8.389s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-codex/inside/shell/worktree | skip | 46.338s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-copilot/inside/shell/worktree | skip | 28.546s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-gemini/inside/shell/worktree | skip | 88ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | skip | 5.862s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-kimi/inside/shell/worktree | skip | 28.077s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-opencode/inside/shell/worktree | skip | 6.147s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| inside-pi/inside/shell/worktree | skip | 27.392s | provider stopped the turn: credit balance is required for all requests, including BYOK, so fallback providers remain available. Add credits at https://vercel.com/d?to=%2F%5Bteam%5D%2F%7E% |
| kimi/off/user/worktree | fail | 2.469s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2817107398run: exit status 1; |
| kimi/tool/user/worktree | fail | 2.453s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-791811613run: exit status 1; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | fail | 4.398s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3450198400run: exit status 1; |
| opencode/rewrite/project/worktree | fail | 11.825s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3543132251run: exit status 1; |
| opencode/tool/project/worktree | fail | 4.33s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1540021444run: exit status 1; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | skip | 8.927s | provider stopped the turn: Conversation run failed for id=b21e3509-bb91-453f-97c0-7bc695971892: litellm.APIError: APIError: OpenAIException - A positive credit balance is required for all requests, including BYOK, so fallback p |
| paperclip/rewrite/project/worktree | fail | 13.114s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-1670233136run: paperclip run failed; |
| pi/off/project/worktree | fail | 2.286s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4058094554run: exit status 1; |
| pi/rewrite/project/worktree | fail | 2.545s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3346928138run: exit status 1; |
| pi/tool/project/worktree | fail | 2.01s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-2932579976run: exit status 1; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | fail | 51.066s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-3748928214run: exit status 1: "Claude gave up after repeated API errors."; |
| t3code/rewrite/project/worktree | fail | 9.108s | kept: /private/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/boxer-eval-4284247440run: exit status 1: "Claude gave up after repeated API errors."; |

**passed 0 · failed 32 · skipped 25**
