# boxer eval report — tier matrix — 2026-09-19T14:58:20+08:00

The same Next.js development workload, across integration levels and harnesses: 18 cells,
3 at a time, each in its own git worktree with its own sandbox and its own automatic host
port. A cell passes only when the page renders in a real browser, the change is in the
worktree, and — for the prove-and-install task — the level itself is shown to have carried the work.

**15/18 passed** (0 skipped), $0.53.

| cell | level | status | guest | host port | turns | time | notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | pass | — | 52194 | 10 | 45s |  |
| claude/rewrite/prove-and-install | rewrite | pass | linux | 51043 | 10 | 1m9s |  |
| claude/tool/add-a-page | tool | pass | — | 52107 | 9 | 43s |  |
| claude/tool/prove-and-install | tool | pass | linux | 51044 | 15 | 56s |  |
| codex/rewrite/add-a-page | rewrite | pass | — | 51528 | 0 | 1m59s |  |
| codex/rewrite/prove-and-install | rewrite | fail | linux | 51627 | 0 | 46s | signature: rewrite level, but nothing rewrote and nothing typed `boxer run` (0 denials, 0 allowed) |
| opencode/plugin/add-a-page | rewrite | pass | — | 51775 | 0 | 46s |  |
| opencode/plugin/prove-and-install | rewrite | fail | linux | 51928 | 0 | 42s | signature: rewrite level, but nothing rewrote and nothing typed `boxer run` (0 denials, 3 allowed) |
| copilot/user/add-a-page | rewrite | pass | — | 51934 | 0 | 42s |  |
| copilot/user/prove-and-install | rewrite | pass | linux | 52105 | 0 | 57s |  |
| kimi/tool/add-a-page | tool | pass | — | 51459 | 0 | 34s |  |
| kimi/tool/prove-and-install | tool | pass | linux | 52402 | 0 | 1m10s |  |
| claude/shims/add-a-page | shims | pass | — | 52404 | 5 | 35s |  |
| claude/shims/prove-and-install | shims | pass | linux | 52333 | 12 | 46s |  |
| openhands/shell/prove-and-install | shell | fail | — | 52565 | 0 | 15m22s | agent: openhands timed out (transcript: /var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log) |
| inside/claude/add-a-page | inside | pass | — | 52562 | 11 | 1m11s |  |
| inside/claude/prove-and-install | inside | pass | linux | 52762 | 10 | 2m11s |  |
| t3/orchestrator/prove-and-install | orchestrator | pass | linux | 51379 | 0 | 3m55s |  |

## By level

| level | passed |
| --- | --- |
| rewrite | 6/8 |
| tool | 4/4 |
| shims | 2/2 |
| shell | 0/1 |
| inside | 2/2 |
| orchestrator | 1/1 |

## What each agent did

### claude/rewrite — add-a-page

- level `rewrite`, pass in 45s, 10 tool calls
- tools: Bash×8, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install

- level `rewrite`, pass in 1m9s, 10 tool calls
- tools: Bash×9, Write×1
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page

- level `tool`, pass in 43s, 9 tool calls
- tools: Bash×6, Read×2, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install

- level `tool`, pass in 56s, 15 tool calls
- tools: Read×1, Write×2, mcp__boxer__boxer_run×11, mcp__boxer__boxer_status×1
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### codex/rewrite — add-a-page

- level `rewrite`, pass in 1m59s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`

### codex/rewrite — prove-and-install

- level `rewrite`, fail in 46s, 0 tool calls
- the guest reported platform `linux`
- **the level did not prove itself**: rewrite level, but nothing rewrote and nothing typed `boxer run` (0 denials, 0 allowed)
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page

- level `rewrite`, pass in 46s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install

- level `rewrite`, fail in 42s, 0 tool calls
- the guest reported platform `linux`
- **the level did not prove itself**: rewrite level, but nothing rewrote and nothing typed `boxer run` (0 denials, 3 allowed)
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page

- level `rewrite`, pass in 42s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install

- level `rewrite`, pass in 57s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### kimi/tool — add-a-page

- level `tool`, pass in 34s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install

- level `tool`, pass in 1m10s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, node_modules/, package-lock.json, package.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### claude/shims — add-a-page

- level `shims`, pass in 35s, 5 tool calls
- tools: Bash×4, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install

- level `shims`, pass in 46s, 12 tool calls
- tools: Bash×7, Skill×1, Write×1, mcp__boxer__boxer_run×2, mcp__boxer__boxer_status×1
- the guest reported platform `linux`
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### openhands/shell — prove-and-install

- level `shell`, fail in 15m22s, 0 tool calls
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page

- level `inside`, pass in 1m11s, 11 tool calls
- tools: Bash×10, Write×1
- changed: boxer.toml, .boxer-eval/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install

- level `inside`, pass in 2m11s, 10 tool calls
- tools: Bash×8, Write×2
- the guest reported platform `linux`
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### t3/orchestrator — prove-and-install

- level `orchestrator`, pass in 3m55s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

