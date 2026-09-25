# boxer eval report — tier matrix — 2026-09-25T17:58:24+07:00

The same Next.js development workload across integration levels, harnesses and
orchestrators: 55 cells, 3 at a time, each in its own git worktree with its own sandbox and
its own automatic host port. Every cell is scored on each claim it makes separately —
provisioning, reaching the guest, rendering the page, leaving the change behind, and showing
that the integration level itself carried the work — so a shortfall names which claim failed.

**99.4% overall** (1262 of 1270 weighted checks). 54 of 55 cells scored 100%, 2 skipped, $1.89.

## How this run was made

| | |
| --- | --- |
| boxer | boxer v1.0.0-48-ged8def6-dirty, commit `ed8def6` (with uncommitted changes) |
| smolvm | smolvm 1.16.1 |
| model | `zai/glm-5.3-flash` — every cell, so the integration level is the only variable |
| guest image | `mirror.gcr.io/library/node:24-bookworm-slim` |
| concurrency | 3 cells at a time |
| host | darwin/arm64, 10 cpus, 24 GB |
| started | 2026-09-25T17:35:38+07:00 |

## By level

| level | score | cells at 100% |
| --- | --- | --- |
| bash-shim | 95.7% | 7/8 |
| inside | 100.0% | 10/10 |
| orchestrator | 100.0% | 6/6 |
| rewrite | 100.0% | 12/12 |
| shell | 100.0% | 1/1 |
| shims | 100.0% | 8/8 |
| tool | 100.0% | 10/10 |

## By harness

| harness | score | cells at 100% |
| --- | --- | --- |
| claude | 100.0% | 8/8 |
| codex | 95.7% | 7/8 |
| copilot | 100.0% | 6/6 |
| dsh | 100.0% | 2/2 |
| grok | 100.0% | 4/4 |
| herdr | 100.0% | 2/2 |
| inside | 100.0% | 10/10 |
| kimi | 100.0% | 6/6 |
| opencode | 100.0% | 2/2 |
| openhands | 100.0% | 1/1 |
| paperclip | 100.0% | 2/2 |
| pi | 100.0% | 2/2 |
| t3 | 100.0% | 2/2 |

## Every cell

| cell | level | score | guest | host port | turns | time | what fell short |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | 100% | — | 62488 | 9 | 58s | — |
| claude/rewrite/prove-and-install | rewrite | 100% | Linux | 62489 | 12 | 1m11s | — |
| codex/rewrite/add-a-page | rewrite | 100% | — | 63027 | 0 | 35s | — |
| codex/rewrite/prove-and-install | rewrite | 100% | Linux | 63074 | 0 | 51s | — |
| opencode/plugin/add-a-page | rewrite | 100% | — | 63076 | 0 | 57s | — |
| opencode/plugin/prove-and-install | rewrite | 100% | Linux | 63224 | 0 | 41s | — |
| copilot/user/add-a-page | rewrite | 100% | — | 63245 | 0 | 50s | — |
| copilot/user/prove-and-install | rewrite | 100% | Linux | 63001 | 0 | 49s | — |
| gemini/rewrite/add-a-page | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini/rewrite/prove-and-install | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/rewrite/add-a-page | rewrite | 100% | — | 63880 | 0 | 40s | — |
| grok/rewrite/prove-and-install | rewrite | 100% | Linux | 63776 | 0 | 1m3s | — |
| pi/rewrite/add-a-page | rewrite | 100% | — | 63826 | 0 | 36s | — |
| pi/rewrite/prove-and-install | rewrite | 100% | Linux | 63857 | 0 | 37s | — |
| claude/tool/add-a-page | tool | 100% | — | 63685 | 10 | 51s | — |
| claude/tool/prove-and-install | tool | 100% | Linux | 62616 | 13 | 1m1s | — |
| kimi/tool/add-a-page | tool | 100% | — | 62617 | 0 | 44s | — |
| kimi/tool/prove-and-install | tool | 100% | Linux | 62681 | 0 | 1m3s | — |
| dsh/tool/add-a-page | tool | 100% | — | 62685 | 0 | 36s | — |
| dsh/tool/prove-and-install | tool | 100% | Linux | 62722 | 0 | 47s | — |
| codex/tool/add-a-page | tool | 100% | — | 62757 | 0 | 48s | — |
| codex/tool/prove-and-install | tool | 100% | Linux | 62790 | 0 | 58s | — |
| grok/tool/add-a-page | tool | 100% | — | 62811 | 0 | 42s | — |
| grok/tool/prove-and-install | tool | 100% | Linux | 62846 | 0 | 3m31s | — |
| claude/shims/add-a-page | shims | 100% | — | 62886 | 10 | 42s | — |
| claude/shims/prove-and-install | shims | 100% | Linux | 62912 | 8 | 39s | — |
| kimi/shims/add-a-page | shims | 100% | — | 62944 | 0 | 40s | — |
| kimi/shims/prove-and-install | shims | 100% | Linux | 62965 | 0 | 48s | — |
| codex/shims/add-a-page | shims | 100% | — | 62590 | 0 | 53s | — |
| codex/shims/prove-and-install | shims | 100% | Linux | 63338 | 0 | 1m9s | — |
| copilot/shims/add-a-page | shims | 100% | — | 63373 | 0 | 51s | — |
| copilot/shims/prove-and-install | shims | 100% | Linux | 63426 | 0 | 56s | — |
| claude/bash-shim/add-a-page | bash-shim | 100% | — | 63449 | 6 | 37s | — |
| claude/bash-shim/prove-and-install | bash-shim | 100% | Linux | 63453 | 14 | 1m5s | — |
| kimi/bash-shim/add-a-page | bash-shim | 100% | — | 63510 | 0 | 37s | — |
| kimi/bash-shim/prove-and-install | bash-shim | 100% | Linux | 63535 | 0 | 46s | — |
| codex/bash-shim/add-a-page | bash-shim | 100% | — | 63560 | 0 | 40s | — |
| codex/bash-shim/prove-and-install | bash-shim | 71% | Darwin | 63584 | 0 | 56s | reached the guest (the work ran on the host: the guest probe reported "Darwin"); the level carried it (the work ran on the host: the guest probe reported "Darwin") |
| copilot/bash-shim/add-a-page | bash-shim | 100% | — | 63620 | 0 | 56s | — |
| copilot/bash-shim/prove-and-install | bash-shim | 100% | Linux | 63649 | 0 | 56s | — |
| openhands/shell/prove-and-install | shell | 100% | Linux | 63728 | 0 | 1m3s | — |
| inside/claude/add-a-page | inside | 100% | — | 63751 | 6 | 1m27s | — |
| inside/claude/prove-and-install | inside | 100% | Linux | 63258 | 18 | 1m48s | — |
| inside/codex/add-a-page | inside | 100% | — | 63927 | 0 | 1m52s | — |
| inside/codex/prove-and-install | inside | 100% | Linux | 63961 | 0 | 1m54s | — |
| inside/opencode/add-a-page | inside | 100% | — | 63881 | 0 | 6m42s | — |
| inside/opencode/prove-and-install | inside | 100% | Linux | 64747 | 0 | 55s | — |
| inside/kimi/add-a-page | inside | 100% | — | 64372 | 0 | 1m0s | — |
| inside/kimi/prove-and-install | inside | 100% | Linux | 64462 | 0 | 48s | — |
| inside/fx/add-a-page | inside | 100% | — | 64431 | 0 | 1m21s | — |
| inside/fx/prove-and-install | inside | 100% | Linux | 64546 | 0 | 1m57s | — |
| t3/orchestrator/add-a-page | orchestrator | 100% | — | 64655 | 0 | 1m24s | — |
| t3/orchestrator/prove-and-install | orchestrator | 100% | Linux | 64683 | 0 | 1m19s | — |
| paperclip/orchestrator/add-a-page | orchestrator | 100% | — | 64346 | 0 | 1m43s | — |
| paperclip/orchestrator/prove-and-install | orchestrator | 100% | Linux | 64217 | 0 | 1m15s | — |
| herdr/orchestrator/add-a-page | orchestrator | 100% | — | 64263 | 0 | 1m1s | — |
| herdr/orchestrator/prove-and-install | orchestrator | 100% | Linux | 62494 | 0 | 1m11s | — |

## The scorecard

Every claim, and how many cells made it good.

| claim | cells | met |
| --- | --- | --- |
| the sandbox came up | 55 | 55 (100%) |
| a host port was forwarded | 55 | 55 (100%) |
| the dev server answered first | 55 | 55 (100%) |
| the agent finished | 55 | 55 (100%) |
| the agent converged | 55 | 55 (100%) |
| the page renders | 55 | 55 (100%) |
| the change is in the worktree | 55 | 55 (100%) |
| no host leak | 55 | 55 (100%) |
| the dev server survived | 55 | 55 (100%) |
| the server was not replaced | 55 | 55 (100%) |
| the sandbox was left clean | 55 | 55 (100%) |
| the dependency is installed | 28 | 28 (100%) |
| reached the guest | 28 | 27 (96%) |
| the level carried it | 28 | 27 (96%) |

## What each agent did

### claude/rewrite — add-a-page — 100%

- level `rewrite`, 58s, 9 tool calls
- tools: Bash×6, Read×2, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install — 100%

- level `rewrite`, 1m11s, 12 tool calls
- tools: Bash×9, Write×2, mcp__boxer__boxer_run×1
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### codex/rewrite — add-a-page — 100%

- level `rewrite`, 35s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.trace.log`

### codex/rewrite — prove-and-install — 100%

- level `rewrite`, 51s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page — 100%

- level `rewrite`, 57s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install — 100%

- level `rewrite`, 41s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .opencode/, AGENTS.md, app/app/clsx/, opencode.json, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page — 100%

- level `rewrite`, 50s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install — 100%

- level `rewrite`, 49s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### grok/rewrite — add-a-page — 100%

- level `rewrite`, 40s, 0 tool calls
- changed: boxer.toml, .agents/, .grok/, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.trace.log`

### grok/rewrite — prove-and-install — 100%

- level `rewrite`, 1m3s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .grok/, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.trace.log`

### pi/rewrite — add-a-page — 100%

- level `rewrite`, 36s, 0 tool calls
- changed: boxer.toml, .pi/, AGENTS.md, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.trace.log`

### pi/rewrite — prove-and-install — 100%

- level `rewrite`, 37s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .pi/, AGENTS.md, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page — 100%

- level `tool`, 51s, 10 tool calls
- tools: Bash×8, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install — 100%

- level `tool`, 1m1s, 13 tool calls
- tools: Bash×4, Write×1, mcp__boxer__boxer_run×7, mcp__boxer__boxer_status×1
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### kimi/tool — add-a-page — 100%

- level `tool`, 44s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install — 100%

- level `tool`, 1m3s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### dsh/tool — add-a-page — 100%

- level `tool`, 36s, 0 tool calls
- changed: boxer.toml, .agents/, .dsh/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.trace.log`

### dsh/tool — prove-and-install — 100%

- level `tool`, 47s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .dsh/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.trace.log`

### codex/tool — add-a-page — 100%

- level `tool`, 48s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.trace.log`

### codex/tool — prove-and-install — 100%

- level `tool`, 58s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.trace.log`

### grok/tool — add-a-page — 100%

- level `tool`, 42s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.trace.log`

### grok/tool — prove-and-install — 100%

- level `tool`, 3m31s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .gstack/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.trace.log`

### claude/shims — add-a-page — 100%

- level `shims`, 42s, 10 tool calls
- tools: Bash×9, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install — 100%

- level `shims`, 39s, 8 tool calls
- tools: Bash×5, Write×2, mcp__boxer__boxer_run×1
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### kimi/shims — add-a-page — 100%

- level `shims`, 40s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.trace.log`

### kimi/shims — prove-and-install — 100%

- level `shims`, 48s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.trace.log`

### codex/shims — add-a-page — 100%

- level `shims`, 53s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.trace.log`

### codex/shims — prove-and-install — 100%

- level `shims`, 1m9s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.trace.log`

### copilot/shims — add-a-page — 100%

- level `shims`, 51s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.trace.log`

### copilot/shims — prove-and-install — 100%

- level `shims`, 56s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.trace.log`

### claude/bash-shim — add-a-page — 100%

- level `bash-shim`, 37s, 6 tool calls
- tools: Bash×5, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.trace.log`

### claude/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m5s, 14 tool calls
- tools: Bash×11, Read×1, Skill×1, Write×1
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.trace.log`

### kimi/bash-shim — add-a-page — 100%

- level `bash-shim`, 37s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.trace.log`

### kimi/bash-shim — prove-and-install — 100%

- level `bash-shim`, 46s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.trace.log`

### codex/bash-shim — add-a-page — 100%

- level `bash-shim`, 40s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.trace.log`

### codex/bash-shim — prove-and-install — 71%

- level `bash-shim`, 56s, 0 tool calls
- **reached the guest**: the work ran on the host: the guest probe reported "Darwin"
- **the level carried it**: the work ran on the host: the guest probe reported "Darwin"
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.trace.log`

### copilot/bash-shim — add-a-page — 100%

- level `bash-shim`, 56s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.trace.log`

### copilot/bash-shim — prove-and-install — 100%

- level `bash-shim`, 56s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.trace.log`

### openhands/shell — prove-and-install — 100%

- level `shell`, 1m3s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page — 100%

- level `inside`, 1m27s, 6 tool calls
- tools: Bash×5, Write×1
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install — 100%

- level `inside`, 1m48s, 18 tool calls
- tools: Bash×12, Read×4, Write×2
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### inside/codex — add-a-page — 100%

- level `inside`, 1m52s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.trace.log`

### inside/codex — prove-and-install — 100%

- level `inside`, 1m54s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.trace.log`

### inside/opencode — add-a-page — 100%

- level `inside`, 6m42s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, opencode.json, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.trace.log`

### inside/opencode — prove-and-install — 100%

- level `inside`, 55s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, opencode.json, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.trace.log`

### inside/kimi — add-a-page — 100%

- level `inside`, 1m0s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.trace.log`

### inside/kimi — prove-and-install — 100%

- level `inside`, 48s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.trace.log`

### inside/fx — add-a-page — 100%

- level `inside`, 1m21s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-add-a-page.trace.log`

### inside/fx — prove-and-install — 100%

- level `inside`, 1m57s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-prove-and-install.trace.log`

### t3/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m24s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.trace.log`

### t3/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m19s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

### paperclip/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m43s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-add-a-page.agent.log`

### paperclip/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m15s, 0 tool calls
- the level carried it: the agent typed `boxer run` itself, from the installed agent contract
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-prove-and-install.agent.log`

### herdr/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m1s, 0 tool calls
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.trace.log`

### herdr/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m11s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.trace.log`

