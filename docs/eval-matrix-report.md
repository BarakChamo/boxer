# boxer eval report — tier matrix — 2026-09-19T19:01:35+08:00

The same Next.js development workload, across integration levels and harnesses: 29 cells,
3 at a time, each in its own git worktree with its own sandbox and its own automatic host
port. A cell passes only when the page renders in a real browser, the change is in the
worktree, and — for the prove-and-install task — the level itself is shown to have carried the work.

**27/27 passed** (2 skipped), $1.49.

| cell | level | status | guest | host port | turns | time | notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | pass | — | 64887 | 4 | 34s |  |
| claude/rewrite/prove-and-install | rewrite | pass | linux | 64751 | 13 | 51s |  |
| claude/tool/add-a-page | tool | pass | — | 65014 | 6 | 43s |  |
| claude/tool/prove-and-install | tool | pass | linux | 64196 | 6 | 1m6s |  |
| codex/rewrite/add-a-page | rewrite | pass | — | 64686 | 0 | 52s |  |
| codex/rewrite/prove-and-install | rewrite | pass | linux | 64621 | 0 | 1m18s |  |
| opencode/plugin/add-a-page | rewrite | pass | — | 64953 | 0 | 49s |  |
| opencode/plugin/prove-and-install | rewrite | pass | linux | 64195 | 0 | 58s |  |
| copilot/user/add-a-page | rewrite | pass | — | 65196 | 0 | 1m22s |  |
| copilot/user/prove-and-install | rewrite | pass | linux | 65199 | 0 | 12m40s |  |
| kimi/tool/add-a-page | tool | pass | — | 65338 | 0 | 41s |  |
| kimi/tool/prove-and-install | tool | pass | linux | 65433 | 0 | 47s |  |
| claude/shims/add-a-page | shims | pass | — | 65503 | 5 | 32s |  |
| claude/shims/prove-and-install | shims | pass | linux | 49212 | 9 | 45s |  |
| gemini/rewrite/add-a-page | rewrite | skip | — | — | 0 | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini/rewrite/prove-and-install | rewrite | skip | — | — | 0 | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/rewrite/add-a-page | rewrite | pass | — | 49213 | 0 | 34s |  |
| grok/rewrite/prove-and-install | rewrite | pass | linux | 65112 | 0 | 48s |  |
| pi/rewrite/add-a-page | rewrite | pass | — | 49432 | 0 | 34s |  |
| pi/rewrite/prove-and-install | rewrite | pass | linux | 49513 | 0 | 56s |  |
| dsh/tool/add-a-page | tool | pass | — | 49552 | 0 | 49s |  |
| dsh/tool/prove-and-install | tool | pass | linux | 49663 | 0 | 1m6s |  |
| openhands/shell/prove-and-install | shell | pass | linux | 49365 | 0 | 51s |  |
| inside/claude/add-a-page | inside | pass | — | 49929 | 12 | 1m11s |  |
| inside/claude/prove-and-install | inside | pass | linux | 50103 | 12 | 1m10s |  |
| t3/orchestrator/add-a-page | orchestrator | pass | — | 49840 | 0 | 1m40s |  |
| t3/orchestrator/prove-and-install | orchestrator | pass | linux | 50406 | 0 | 1m30s |  |
| paperclip/orchestrator/prove-and-install | orchestrator | pass | linux | 50582 | 0 | 3m20s |  |
| herdr/orchestrator/prove-and-install | orchestrator | pass | linux | 64201 | 0 | 1m16s |  |

## By level

| level | passed |
| --- | --- |
| rewrite | 12/14 |
| tool | 6/6 |
| shims | 2/2 |
| shell | 1/1 |
| inside | 2/2 |
| orchestrator | 4/4 |

## What each agent did

### claude/rewrite — add-a-page

- level `rewrite`, pass in 34s, 4 tool calls
- tools: Bash×3, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install

- level `rewrite`, pass in 51s, 13 tool calls
- tools: Bash×9, Read×2, Write×2
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page

- level `tool`, pass in 43s, 6 tool calls
- tools: Write×1, mcp__boxer__boxer_run×5
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install

- level `tool`, pass in 1m6s, 6 tool calls
- tools: Bash×3, Write×1, mcp__boxer__boxer_run×2
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### codex/rewrite — add-a-page

- level `rewrite`, pass in 52s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.trace.log`

### codex/rewrite — prove-and-install

- level `rewrite`, pass in 1m18s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page

- level `rewrite`, pass in 49s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install

- level `rewrite`, pass in 58s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .opencode/, AGENTS.md, app/app/clsx/, opencode.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page

- level `rewrite`, pass in 1m22s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install

- level `rewrite`, pass in 12m40s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### kimi/tool — add-a-page

- level `tool`, pass in 41s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install

- level `tool`, pass in 47s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### claude/shims — add-a-page

- level `shims`, pass in 32s, 5 tool calls
- tools: Bash×4, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install

- level `shims`, pass in 45s, 9 tool calls
- tools: Bash×4, Skill×1, Write×1, mcp__boxer__boxer_run×2, mcp__boxer__boxer_status×1
- the guest reported platform `linux`
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### grok/rewrite — add-a-page

- level `rewrite`, pass in 34s, 0 tool calls
- changed: boxer.toml, .agents/, .grok/, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.trace.log`

### grok/rewrite — prove-and-install

- level `rewrite`, pass in 48s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .grok/, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.trace.log`

### pi/rewrite — add-a-page

- level `rewrite`, pass in 34s, 0 tool calls
- changed: boxer.toml, .pi/, AGENTS.md, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.trace.log`

### pi/rewrite — prove-and-install

- level `rewrite`, pass in 56s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .pi/, AGENTS.md, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.trace.log`

### dsh/tool — add-a-page

- level `tool`, pass in 49s, 0 tool calls
- changed: boxer.toml, .agents/, .dsh/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.trace.log`

### dsh/tool — prove-and-install

- level `tool`, pass in 1m6s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .dsh/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.trace.log`

### openhands/shell — prove-and-install

- level `shell`, pass in 51s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page

- level `inside`, pass in 1m11s, 12 tool calls
- tools: Bash×9, Read×2, Write×1
- changed: boxer.toml, .boxer-eval/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install

- level `inside`, pass in 1m10s, 12 tool calls
- tools: Bash×9, Read×1, Write×2
- the guest reported platform `linux`
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### t3/orchestrator — add-a-page

- level `orchestrator`, pass in 1m40s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.trace.log`

### t3/orchestrator — prove-and-install

- level `orchestrator`, pass in 1m30s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the worktree the orchestrator cut carries boxer's project layer, and the work ran in the guest (its launcher passes no trace through, so there is nothing finer to read)
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

### paperclip/orchestrator — prove-and-install

- level `orchestrator`, pass in 3m20s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the worktree the orchestrator cut carries boxer's project layer, and the work ran in the guest (its launcher passes no trace through, so there is nothing finer to read)
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-prove-and-install.agent.log`

### herdr/orchestrator — prove-and-install

- level `orchestrator`, pass in 1m16s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.trace.log`

