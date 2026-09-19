# boxer eval report — tier matrix — 2026-09-19T18:21:55+08:00

The same Next.js development workload, across integration levels and harnesses: 29 cells,
3 at a time, each in its own git worktree with its own sandbox and its own automatic host
port. A cell passes only when the page renders in a real browser, the change is in the
worktree, and — for the prove-and-install task — the level itself is shown to have carried the work.

**26/27 passed** (2 skipped), $0.86.

| cell | level | status | guest | host port | turns | time | notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | pass | — | 57530 | 9 | 53s |  |
| claude/rewrite/prove-and-install | rewrite | pass | linux | 57445 | 8 | 37s |  |
| claude/tool/add-a-page | tool | pass | — | 57957 | 4 | 32s |  |
| claude/tool/prove-and-install | tool | pass | linux | 58629 | 9 | 51s |  |
| codex/rewrite/add-a-page | rewrite | pass | — | 57149 | 0 | 56s |  |
| codex/rewrite/prove-and-install | rewrite | pass | linux | 57665 | 0 | 49s |  |
| opencode/plugin/add-a-page | rewrite | pass | — | 57587 | 0 | 41s |  |
| opencode/plugin/prove-and-install | rewrite | pass | linux | 57805 | 0 | 57s |  |
| copilot/user/add-a-page | rewrite | pass | — | 57812 | 0 | 43s |  |
| copilot/user/prove-and-install | rewrite | pass | linux | 57148 | 0 | 1m24s |  |
| kimi/tool/add-a-page | tool | pass | — | 58065 | 0 | 45s |  |
| kimi/tool/prove-and-install | tool | pass | linux | 58146 | 0 | 1m11s |  |
| claude/shims/add-a-page | shims | pass | — | 58296 | 8 | 47s |  |
| claude/shims/prove-and-install | shims | pass | linux | 58320 | 17 | 1m3s |  |
| gemini/rewrite/add-a-page | rewrite | skip | — | — | 0 | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini/rewrite/prove-and-install | rewrite | skip | — | — | 0 | 0s | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/rewrite/add-a-page | rewrite | pass | — | 58468 | 0 | 33s |  |
| grok/rewrite/prove-and-install | rewrite | pass | linux | 58545 | 0 | 55s |  |
| pi/rewrite/add-a-page | rewrite | pass | — | 58062 | 0 | 43s |  |
| pi/rewrite/prove-and-install | rewrite | pass | linux | 58849 | 0 | 48s |  |
| dsh/tool/add-a-page | tool | pass | — | 58947 | 0 | 1m11s |  |
| dsh/tool/prove-and-install | tool | pass | linux | 59042 | 0 | 54s |  |
| openhands/shell/prove-and-install | shell | pass | linux | 59142 | 0 | 50s |  |
| inside/claude/add-a-page | inside | fail | — | 58632 | 8 | 1m42s | browser: browser open: exit status 1 |
| inside/claude/prove-and-install | inside | pass | linux | 59214 | 8 | 1m7s |  |
| t3/orchestrator/add-a-page | orchestrator | pass | — | 59843 | 0 | 2m1s |  |
| t3/orchestrator/prove-and-install | orchestrator | pass | linux | 59505 | 0 | 3m46s |  |
| paperclip/orchestrator/prove-and-install | orchestrator | pass | linux | 59687 | 0 | 2m55s |  |
| herdr/orchestrator/prove-and-install | orchestrator | pass | linux | 57154 | 0 | 1m16s |  |

## By level

| level | passed |
| --- | --- |
| rewrite | 12/14 |
| tool | 6/6 |
| shims | 2/2 |
| shell | 1/1 |
| inside | 1/2 |
| orchestrator | 4/4 |

## What each agent did

### claude/rewrite — add-a-page

- level `rewrite`, pass in 53s, 9 tool calls
- tools: Bash×7, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install

- level `rewrite`, pass in 37s, 8 tool calls
- tools: Bash×6, Write×2
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page

- level `tool`, pass in 32s, 4 tool calls
- tools: Bash×3, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install

- level `tool`, pass in 51s, 9 tool calls
- tools: Bash×3, Write×2, mcp__boxer__boxer_run×4
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### codex/rewrite — add-a-page

- level `rewrite`, pass in 56s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.trace.log`

### codex/rewrite — prove-and-install

- level `rewrite`, pass in 49s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page

- level `rewrite`, pass in 41s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install

- level `rewrite`, pass in 57s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, .opencode/, AGENTS.md, app/app/clsx/, opencode.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page

- level `rewrite`, pass in 43s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install

- level `rewrite`, pass in 1m24s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### kimi/tool — add-a-page

- level `tool`, pass in 45s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install

- level `tool`, pass in 1m11s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### claude/shims — add-a-page

- level `shims`, pass in 47s, 8 tool calls
- tools: Bash×6, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install

- level `shims`, pass in 1m3s, 17 tool calls
- tools: Bash×14, Read×1, Skill×1, Write×1
- the guest reported platform `linux`
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### grok/rewrite — add-a-page

- level `rewrite`, pass in 33s, 0 tool calls
- changed: boxer.toml, .agents/, .grok/, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.trace.log`

### grok/rewrite — prove-and-install

- level `rewrite`, pass in 55s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .grok/, .gstack/, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.trace.log`

### pi/rewrite — add-a-page

- level `rewrite`, pass in 43s, 0 tool calls
- changed: boxer.toml, .pi/, AGENTS.md, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.trace.log`

### pi/rewrite — prove-and-install

- level `rewrite`, pass in 48s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .pi/, AGENTS.md, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.trace.log`

### dsh/tool — add-a-page

- level `tool`, pass in 1m11s, 0 tool calls
- changed: boxer.toml, .agents/, .dsh/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.trace.log`

### dsh/tool — prove-and-install

- level `tool`, pass in 54s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .dsh/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.trace.log`

### openhands/shell — prove-and-install

- level `shell`, pass in 50s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page

- level `inside`, fail in 1m42s, 8 tool calls
- tools: Bash×7, Write×1
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install

- level `inside`, pass in 1m7s, 8 tool calls
- tools: Bash×6, Write×2
- the guest reported platform `linux`
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### t3/orchestrator — add-a-page

- level `orchestrator`, pass in 2m1s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.trace.log`

### t3/orchestrator — prove-and-install

- level `orchestrator`, pass in 3m46s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

### paperclip/orchestrator — prove-and-install

- level `orchestrator`, pass in 2m55s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the agent typed `boxer run` itself, from the installed agent contract
- changed: .mcp.json, app/app/clsx/page.tsx, app/package-lock.json, app/package.json, boxer.toml, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-prove-and-install.agent.log`

### herdr/orchestrator — prove-and-install

- level `orchestrator`, pass in 1m16s, 0 tool calls
- the guest reported platform `linux`
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.trace.log`

