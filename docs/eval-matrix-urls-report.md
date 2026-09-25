# boxer eval report — tier matrix — 2026-09-25T17:35:38+07:00

The same Next.js development workload across integration levels, harnesses and
orchestrators: 55 cells, 3 at a time, each in its own git worktree with its own sandbox and
its own automatic host port. Every cell is scored on each claim it makes separately —
provisioning, reaching the guest, rendering the page, leaving the change behind, and showing
that the integration level itself carried the work — so a shortfall names which claim failed.

**98.7% overall** (1254 of 1270 weighted checks). 50 of 55 cells scored 100%, 2 skipped, $3.04.

## How this run was made

| | |
| --- | --- |
| boxer | boxer v1.0.0-48-ged8def6-dirty, commit `ed8def6` (with uncommitted changes) |
| smolvm | smolvm 1.16.1 |
| model | `zai/glm-5.3-flash` — every cell, so the integration level is the only variable |
| guest image | `mirror.gcr.io/library/node:24-bookworm-slim` |
| concurrency | 3 cells at a time |
| host | darwin/arm64, 10 cpus, 24 GB |
| started | 2026-09-25T17:08:54+07:00 |

## By level

| level | score | cells at 100% |
| --- | --- | --- |
| bash-shim | 100.0% | 8/8 |
| inside | 93.9% | 6/10 |
| orchestrator | 100.0% | 6/6 |
| rewrite | 99.3% | 11/12 |
| shell | 100.0% | 1/1 |
| shims | 100.0% | 8/8 |
| tool | 100.0% | 10/10 |

## By harness

| harness | score | cells at 100% |
| --- | --- | --- |
| claude | 100.0% | 8/8 |
| codex | 100.0% | 8/8 |
| copilot | 100.0% | 6/6 |
| dsh | 100.0% | 2/2 |
| grok | 97.8% | 3/4 |
| herdr | 100.0% | 2/2 |
| inside | 93.9% | 6/10 |
| kimi | 100.0% | 6/6 |
| opencode | 100.0% | 2/2 |
| openhands | 100.0% | 1/1 |
| paperclip | 100.0% | 2/2 |
| pi | 100.0% | 2/2 |
| t3 | 100.0% | 2/2 |

## Every cell

| cell | level | score | guest | host port | turns | time | what fell short |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | 100% | — | 59287 | 13 | 1m7s | — |
| claude/rewrite/prove-and-install | rewrite | 100% | Linux | 59286 | 11 | 1m11s | — |
| codex/rewrite/add-a-page | rewrite | 100% | — | 59448 | 0 | 53s | — |
| codex/rewrite/prove-and-install | rewrite | 100% | Linux | 59488 | 0 | 1m25s | — |
| opencode/plugin/add-a-page | rewrite | 100% | — | 59560 | 0 | 55s | — |
| opencode/plugin/prove-and-install | rewrite | 100% | Linux | 59562 | 0 | 1m12s | — |
| copilot/user/add-a-page | rewrite | 100% | — | 59768 | 0 | 49s | — |
| copilot/user/prove-and-install | rewrite | 100% | Linux | 59793 | 0 | 51s | — |
| gemini/rewrite/add-a-page | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini/rewrite/prove-and-install | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/rewrite/add-a-page | rewrite | 89% | — | 59829 | 0 | 1m6s | the server was not replaced (the agent started its own dev server) |
| grok/rewrite/prove-and-install | rewrite | 100% | Linux | 59880 | 0 | 1m7s | — |
| pi/rewrite/add-a-page | rewrite | 100% | — | 59908 | 0 | 28s | — |
| pi/rewrite/prove-and-install | rewrite | 100% | Linux | 59954 | 0 | 41s | — |
| claude/tool/add-a-page | tool | 100% | — | 59959 | 6 | 41s | — |
| claude/tool/prove-and-install | tool | 100% | Linux | 60004 | 12 | 51s | — |
| kimi/tool/add-a-page | tool | 100% | — | 60042 | 0 | 1m0s | — |
| kimi/tool/prove-and-install | tool | 100% | Linux | 60048 | 0 | 1m1s | — |
| dsh/tool/add-a-page | tool | 100% | — | 60098 | 0 | 39s | — |
| dsh/tool/prove-and-install | tool | 100% | Linux | 60144 | 0 | 56s | — |
| codex/tool/add-a-page | tool | 100% | — | 60147 | 0 | 51s | — |
| codex/tool/prove-and-install | tool | 100% | Linux | 60190 | 0 | 57s | — |
| grok/tool/add-a-page | tool | 100% | — | 60250 | 0 | 1m0s | — |
| grok/tool/prove-and-install | tool | 100% | Linux | 60256 | 0 | 44s | — |
| claude/shims/add-a-page | shims | 100% | — | 60302 | 8 | 39s | — |
| claude/shims/prove-and-install | shims | 100% | Linux | 60352 | 13 | 53s | — |
| kimi/shims/add-a-page | shims | 100% | — | 60377 | 0 | 55s | — |
| kimi/shims/prove-and-install | shims | 100% | Linux | 60382 | 0 | 2m24s | — |
| codex/shims/add-a-page | shims | 100% | — | 59443 | 0 | 58s | — |
| codex/shims/prove-and-install | shims | 100% | Linux | 60472 | 0 | 57s | — |
| copilot/shims/add-a-page | shims | 100% | — | 60525 | 0 | 55s | — |
| copilot/shims/prove-and-install | shims | 100% | Linux | 60574 | 0 | 1m2s | — |
| claude/bash-shim/add-a-page | bash-shim | 100% | — | 60622 | 7 | 40s | — |
| claude/bash-shim/prove-and-install | bash-shim | 100% | Linux | 60626 | 17 | 1m5s | — |
| kimi/bash-shim/add-a-page | bash-shim | 100% | — | 60682 | 0 | 40s | — |
| kimi/bash-shim/prove-and-install | bash-shim | 100% | Linux | 60449 | 0 | 44s | — |
| codex/bash-shim/add-a-page | bash-shim | 100% | — | 61650 | 0 | 34s | — |
| codex/bash-shim/prove-and-install | bash-shim | 100% | Linux | 61851 | 0 | 45s | — |
| copilot/bash-shim/add-a-page | bash-shim | 100% | — | 61948 | 0 | 51s | — |
| copilot/bash-shim/prove-and-install | bash-shim | 100% | Linux | 62064 | 0 | 1m26s | — |
| openhands/shell/prove-and-install | shell | 100% | Linux | 62086 | 0 | 1m0s | — |
| inside/claude/add-a-page | inside | 100% | — | 62203 | 6 | 1m7s | — |
| inside/claude/prove-and-install | inside | 89% | Linux | 61560 | 25 | 4m4s | the page renders (the dev server stopped answering on 61560 (http://127.0.0.1:61560/ never answered: Get "http://127.0.0.1:61560/": read tcp 127.0.0.1:61932->127.0.0.1:61560: read: connection reset by peer)); the dev server survived (http://127.0.0.1:61560/ never answered: Get "http://127.0.0.1:61560/": read tcp 127.0.0.1:61998->127.0.0.1:61560: read: connection reset by peer) |
| inside/codex/add-a-page | inside | 72% | — | 60756 | 0 | 3m57s | the page renders (the dev server stopped answering on 60756 (http://127.0.0.1:60756/ never answered: Get "http://127.0.0.1:60756/": read tcp 127.0.0.1:61146->127.0.0.1:60756: read: connection reset by peer)); the dev server survived (http://127.0.0.1:60756/ never answered: Get "http://127.0.0.1:60756/": read tcp 127.0.0.1:61208->127.0.0.1:60756: read: connection reset by peer); the server was not replaced (the agent started its own dev server) |
| inside/codex/prove-and-install | inside | 100% | Linux | 60763 | 0 | 2m9s | — |
| inside/opencode/add-a-page | inside | 100% | — | 60911 | 0 | 1m1s | — |
| inside/opencode/prove-and-install | inside | 100% | Linux | 61120 | 0 | 1m2s | — |
| inside/kimi/add-a-page | inside | 83% | — | 61284 | 0 | 4m46s | the page renders (the dev server stopped answering on 61284 (http://127.0.0.1:61284/ never answered: Get "http://127.0.0.1:61284/": read tcp 127.0.0.1:61813->127.0.0.1:61284: read: connection reset by peer)); the dev server survived (http://127.0.0.1:61284/ never answered: Get "http://127.0.0.1:61284/": read tcp 127.0.0.1:61849->127.0.0.1:61284: read: connection reset by peer) |
| inside/kimi/prove-and-install | inside | 100% | Linux | 61327 | 0 | 1m3s | — |
| inside/fx/add-a-page | inside | 83% | — | 60708 | 0 | 6m18s | the page renders (the dev server stopped answering on 60708 (http://127.0.0.1:60708/ never answered: Get "http://127.0.0.1:60708/": read tcp 127.0.0.1:61457->127.0.0.1:60708: read: connection reset by peer)); the dev server survived (http://127.0.0.1:60708/ never answered: Get "http://127.0.0.1:60708/": read tcp 127.0.0.1:61558->127.0.0.1:60708: read: connection reset by peer) |
| inside/fx/prove-and-install | inside | 100% | Linux | 61698 | 0 | 3m2s | — |
| t3/orchestrator/add-a-page | orchestrator | 100% | — | 62051 | 0 | 1m1s | — |
| t3/orchestrator/prove-and-install | orchestrator | 100% | Linux | 61532 | 0 | 1m12s | — |
| paperclip/orchestrator/add-a-page | orchestrator | 100% | — | 62425 | 0 | 1m30s | — |
| paperclip/orchestrator/prove-and-install | orchestrator | 100% | Linux | 62207 | 0 | 1m40s | — |
| herdr/orchestrator/add-a-page | orchestrator | 100% | — | 62322 | 0 | 50s | — |
| herdr/orchestrator/prove-and-install | orchestrator | 100% | Linux | 59293 | 0 | 1m23s | — |

## The scorecard

Every claim, and how many cells made it good.

| claim | cells | met |
| --- | --- | --- |
| the sandbox came up | 55 | 55 (100%) |
| a host port was forwarded | 55 | 55 (100%) |
| the dev server answered first | 55 | 55 (100%) |
| the agent finished | 55 | 55 (100%) |
| the agent converged | 55 | 55 (100%) |
| the page renders | 55 | 51 (93%) |
| the change is in the worktree | 55 | 55 (100%) |
| no host leak | 55 | 55 (100%) |
| the dev server survived | 55 | 51 (93%) |
| the server was not replaced | 55 | 53 (96%) |
| the sandbox was left clean | 55 | 55 (100%) |
| the dependency is installed | 28 | 28 (100%) |
| reached the guest | 28 | 28 (100%) |
| the level carried it | 28 | 28 (100%) |

## What each agent did

### claude/rewrite — add-a-page — 100%

- level `rewrite`, 1m7s, 13 tool calls
- tools: Bash×10, Read×2, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install — 100%

- level `rewrite`, 1m11s, 11 tool calls
- tools: Bash×8, Skill×1, Write×2
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### codex/rewrite — add-a-page — 100%

- level `rewrite`, 53s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.trace.log`

### codex/rewrite — prove-and-install — 100%

- level `rewrite`, 1m25s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page — 100%

- level `rewrite`, 55s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install — 100%

- level `rewrite`, 1m12s, 0 tool calls
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, .opencode/, AGENTS.md, app/app/clsx/, opencode.json, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page — 100%

- level `rewrite`, 49s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install — 100%

- level `rewrite`, 51s, 0 tool calls
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### grok/rewrite — add-a-page — 89%

- level `rewrite`, 1m6s, 0 tool calls
- **the server was not replaced**: the agent started its own dev server
- changed: boxer.toml, .agents/, .grok/, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.trace.log`

### grok/rewrite — prove-and-install — 100%

- level `rewrite`, 1m7s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .grok/, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.trace.log`

### pi/rewrite — add-a-page — 100%

- level `rewrite`, 28s, 0 tool calls
- changed: boxer.toml, .pi/, AGENTS.md, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.trace.log`

### pi/rewrite — prove-and-install — 100%

- level `rewrite`, 41s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .pi/, AGENTS.md, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page — 100%

- level `tool`, 41s, 6 tool calls
- tools: Bash×5, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install — 100%

- level `tool`, 51s, 12 tool calls
- tools: Bash×7, Read×2, Write×1, mcp__boxer__boxer_run×2
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### kimi/tool — add-a-page — 100%

- level `tool`, 1m0s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install — 100%

- level `tool`, 1m1s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### dsh/tool — add-a-page — 100%

- level `tool`, 39s, 0 tool calls
- changed: boxer.toml, .agents/, .dsh/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.trace.log`

### dsh/tool — prove-and-install — 100%

- level `tool`, 56s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .dsh/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.trace.log`

### codex/tool — add-a-page — 100%

- level `tool`, 51s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.trace.log`

### codex/tool — prove-and-install — 100%

- level `tool`, 57s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.trace.log`

### grok/tool — add-a-page — 100%

- level `tool`, 1m0s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.trace.log`

### grok/tool — prove-and-install — 100%

- level `tool`, 44s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.trace.log`

### claude/shims — add-a-page — 100%

- level `shims`, 39s, 8 tool calls
- tools: Bash×6, Write×1, mcp__boxer__boxer_status×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install — 100%

- level `shims`, 53s, 13 tool calls
- tools: Bash×8, Read×1, Skill×1, Write×2, mcp__boxer__boxer_run×1
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### kimi/shims — add-a-page — 100%

- level `shims`, 55s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.trace.log`

### kimi/shims — prove-and-install — 100%

- level `shims`, 2m24s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.trace.log`

### codex/shims — add-a-page — 100%

- level `shims`, 58s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.trace.log`

### codex/shims — prove-and-install — 100%

- level `shims`, 57s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.trace.log`

### copilot/shims — add-a-page — 100%

- level `shims`, 55s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.trace.log`

### copilot/shims — prove-and-install — 100%

- level `shims`, 1m2s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.trace.log`

### claude/bash-shim — add-a-page — 100%

- level `bash-shim`, 40s, 7 tool calls
- tools: Bash×6, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.trace.log`

### claude/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m5s, 17 tool calls
- tools: Bash×9, Read×2, Skill×1, Write×1, mcp__boxer__boxer_run×4
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.trace.log`

### kimi/bash-shim — add-a-page — 100%

- level `bash-shim`, 40s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.trace.log`

### kimi/bash-shim — prove-and-install — 100%

- level `bash-shim`, 44s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.trace.log`

### codex/bash-shim — add-a-page — 100%

- level `bash-shim`, 34s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.trace.log`

### codex/bash-shim — prove-and-install — 100%

- level `bash-shim`, 45s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.trace.log`

### copilot/bash-shim — add-a-page — 100%

- level `bash-shim`, 51s, 0 tool calls
- changed: boxer.toml, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.trace.log`

### copilot/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m26s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.trace.log`

### openhands/shell — prove-and-install — 100%

- level `shell`, 1m0s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page — 100%

- level `inside`, 1m7s, 6 tool calls
- tools: Bash×5, Write×1
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install — 89%

- level `inside`, 4m4s, 25 tool calls
- tools: Bash×24, Write×1
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- **the page renders**: the dev server stopped answering on 61560 (http://127.0.0.1:61560/ never answered: Get "http://127.0.0.1:61560/": read tcp 127.0.0.1:61932->127.0.0.1:61560: read: connection reset by peer)
- **the dev server survived**: http://127.0.0.1:61560/ never answered: Get "http://127.0.0.1:61560/": read tcp 127.0.0.1:61998->127.0.0.1:61560: read: connection reset by peer
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### inside/codex — add-a-page — 72%

- level `inside`, 3m57s, 0 tool calls
- **the page renders**: the dev server stopped answering on 60756 (http://127.0.0.1:60756/ never answered: Get "http://127.0.0.1:60756/": read tcp 127.0.0.1:61146->127.0.0.1:60756: read: connection reset by peer)
- **the dev server survived**: http://127.0.0.1:60756/ never answered: Get "http://127.0.0.1:60756/": read tcp 127.0.0.1:61208->127.0.0.1:60756: read: connection reset by peer
- **the server was not replaced**: the agent started its own dev server
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.trace.log`

### inside/codex — prove-and-install — 100%

- level `inside`, 2m9s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.trace.log`

### inside/opencode — add-a-page — 100%

- level `inside`, 1m1s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, opencode.json, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.trace.log`

### inside/opencode — prove-and-install — 100%

- level `inside`, 1m2s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, opencode.json, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.trace.log`

### inside/kimi — add-a-page — 83%

- level `inside`, 4m46s, 0 tool calls
- **the page renders**: the dev server stopped answering on 61284 (http://127.0.0.1:61284/ never answered: Get "http://127.0.0.1:61284/": read tcp 127.0.0.1:61813->127.0.0.1:61284: read: connection reset by peer)
- **the dev server survived**: http://127.0.0.1:61284/ never answered: Get "http://127.0.0.1:61284/": read tcp 127.0.0.1:61849->127.0.0.1:61284: read: connection reset by peer
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.trace.log`

### inside/kimi — prove-and-install — 100%

- level `inside`, 1m3s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.trace.log`

### inside/fx — add-a-page — 83%

- level `inside`, 6m18s, 0 tool calls
- **the page renders**: the dev server stopped answering on 60708 (http://127.0.0.1:60708/ never answered: Get "http://127.0.0.1:60708/": read tcp 127.0.0.1:61457->127.0.0.1:60708: read: connection reset by peer)
- **the dev server survived**: http://127.0.0.1:60708/ never answered: Get "http://127.0.0.1:60708/": read tcp 127.0.0.1:61558->127.0.0.1:60708: read: connection reset by peer
- changed: boxer.toml, .boxer-eval/, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-add-a-page.trace.log`

### inside/fx — prove-and-install — 100%

- level `inside`, 3m2s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-fx-prove-and-install.trace.log`

### t3/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m1s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.trace.log`

### t3/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m12s, 0 tool calls
- the level carried it: the agent typed `boxer run` itself, from the installed agent contract
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

### paperclip/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m30s, 0 tool calls
- changed: .mcp.json, app/app/about/page.tsx, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-add-a-page.agent.log`

### paperclip/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m40s, 0 tool calls
- the level carried it: the worktree the orchestrator cut carries boxer's project layer, and the work ran in the guest (its launcher passes no trace through, so there is nothing finer to read)
- changed: .mcp.json, app/app/clsx/page.tsx, app/package-lock.json, app/package.json, boxer.toml, site/public/.well-known/agent-skills/boxer/SKILL.md, site/public/.well-known/agent-skills/index.json, skills/boxer/SKILL.md, skills/boxer/references/BRIEF.md, skills/boxer/scripts/brief, skills/boxer/scripts/run, skills/boxer/scripts/status, skills/boxer/scripts/task, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-prove-and-install.agent.log`

### herdr/orchestrator — add-a-page — 100%

- level `orchestrator`, 50s, 0 tool calls
- changed: boxer.toml, .mcp.json, app/app/about/, site/, skills/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.trace.log`

### herdr/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m23s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, site/, skills/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.trace.log`

