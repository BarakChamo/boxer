# boxer eval report — tier t2 — 2026-09-18T18:15:16+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| acp-claude/inside/acp/worktree | pass | 16.642s | $0.0031; |
| acp-codex/inside/acp/worktree | pass | 19.062s | $0.0016; |
| acp-gemini/inside/acp/worktree | skip | 142ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| acp-grok/inside/acp/worktree | pass | 15.354s | $0.0093; |
| acp-kimi/inside/acp/worktree | pass | 15.896s | $0.0045; |
| acp-opencode/inside/acp/worktree | pass | 15.137s | $0.0026; |
| claude-code/off/plugin/worktree | pass | 9.781s | $0.0036; |
| claude-code/rewrite/both/worktree | pass | 39.706s | $0.0018; |
| claude-code/rewrite/plugin/repo | pass | 13.555s | $0.0023; |
| claude-code/rewrite/plugin/worktree | pass | 25.793s | $0.0050; |
| claude-code/rewrite/project/worktree | pass | 10.33s | $0.0039; |
| claude-code/tool/plugin/worktree | pass | 7.862s | $0.0049; |
| claude-code/tool/plugin/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| claude-code/tool/project/worktree | pass | 27.145s | $0.0031; |
| codex/off/project/worktree | pass | 12.755s | $0.0011; |
| codex/rewrite/project/worktree | pass | 11.783s | $0.0018; |
| copilot/off/user/worktree | pass | 15.858s | $0.0015; |
| copilot/rewrite/user/worktree | pass | 19.217s | $0.0024; |
| copilot/tool/user/worktree | pass | 21.574s | $0.0025; |
| dsh/off/project/worktree | pass | 1m7.021s | $0.0043; |
| dsh/tool/project/worktree | pass | 21.516s | $0.0032; |
| dsh/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| gemini-cli/off/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/rewrite/project/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini-cli/tool/plugin/worktree/noncompliant | skip | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/off/user/worktree | pass | 16.398s | $0.0056; |
| grok/rewrite/project/worktree | pass | 18.334s | $0.0080; |
| grok/rewrite/user/worktree | pass | 18.354s | $0.0116; |
| grok/tool/user/worktree | pass | 22.966s | 1 denial(s); $0.0206; |
| grok/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| herdr/rewrite/project/worktree | pass | 15.763s | $0.0062; |
| inside-claude/inside/shell/worktree | pass | 15.463s | $0.0028; |
| inside-codex/inside/shell/worktree | pass | 19.335s | $0.0026; |
| inside-gemini/inside/shell/worktree | skip | 109ms | inside gemini: GEMINI_API_KEY or GOOGLE_API_KEY is not set |
| inside-grok/inside/shell/worktree | pass | 17.746s | $0.0124; |
| inside-kimi/inside/shell/worktree | pass | 32.081s | $0.0046; |
| inside-opencode/inside/shell/worktree | pass | 20.109s | $0.0048; |
| inside-pi/inside/shell/worktree | pass | 17.069s | $0.0004; |
| kimi/off/user/worktree | pass | 9.835s | $0.0046; |
| kimi/tool/user/worktree | pass | 3m28.558s | $0.0095; |
| kimi/tool/user/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| multica/rewrite/project/worktree | skip | 0s | multica has no server configured (multica setup, then multica daemon start); checklist in docs/orchestrators.md |
| opencode/off/project/worktree | pass | 54.592s | $0.0052; |
| opencode/rewrite/project/worktree | pass | 18.745s | $0.0049; |
| opencode/tool/project/worktree | pass | 10.089s | $0.0030; |
| opencode/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| openhands/rewrite/sdk/worktree | pass | 41.47s | $0.0010; |
| paperclip/rewrite/project/worktree | pass | 1m5.205s | $0.0066; |
| pi/off/project/worktree | pass | 40.229s | $0.0006; |
| pi/rewrite/project/worktree | pass | 7.591s | $0.0007; |
| pi/tool/project/worktree | pass | 18.719s | $0.0007; |
| pi/tool/project/worktree/noncompliant | skip | 0s | noncompliant cells are scripted; t1 only |
| t3code/inside/shim/worktree | pass | 1m2.036s | $0.0043; |
| t3code/rewrite/project/worktree | pass | 19.478s | $0.0047; |

**passed 42 · failed 0 · skipped 14**

**gateway spend this run: $0.1878** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
