# boxer eval report — tier matrix — 2026-09-20T11:03:11+08:00

The same Next.js development workload across integration levels, harnesses and
orchestrators: 53 cells, 3 at a time, each in its own git worktree with its own sandbox and
its own automatic host port. Every cell is scored on each claim it makes separately —
provisioning, reaching the guest, rendering the page, leaving the change behind, and showing
that the integration level itself carried the work — so a shortfall names which claim failed.

**99.2% overall** (845 of 852 weighted checks). 52 of 53 cells scored 100%, 2 skipped, $3.06.

## By level

| level | score | cells at 100% |
| --- | --- | --- |
| bash-shim | 100.0% | 8/8 |
| inside | 100.0% | 8/8 |
| orchestrator | 100.0% | 6/6 |
| rewrite | 96.4% | 11/12 |
| shell | 100.0% | 1/1 |
| shims | 100.0% | 8/8 |
| tool | 100.0% | 10/10 |

## By harness

| harness | score | cells at 100% |
| --- | --- | --- |
| claude | 100.0% | 8/8 |
| codex | 100.0% | 8/8 |
| copilot | 92.7% | 5/6 |
| dsh | 100.0% | 2/2 |
| grok | 100.0% | 4/4 |
| herdr | 100.0% | 2/2 |
| inside | 100.0% | 8/8 |
| kimi | 100.0% | 6/6 |
| opencode | 100.0% | 2/2 |
| openhands | 100.0% | 1/1 |
| paperclip | 100.0% | 2/2 |
| pi | 100.0% | 2/2 |
| t3 | 100.0% | 2/2 |

## Every cell

| cell | level | score | guest | host port | turns | time | what fell short |
| --- | --- | --- | --- | --- | --- | --- | --- |
| claude/rewrite/add-a-page | rewrite | 100% | — | 61530 | 9 | 52s | — |
| claude/rewrite/prove-and-install | rewrite | 100% | Linux | 62566 | 10 | 1m1s | — |
| codex/rewrite/add-a-page | rewrite | 100% | — | 62706 | 0 | 40s | — |
| codex/rewrite/prove-and-install | rewrite | 100% | Linux | 49871 | 0 | 1m32s | — |
| opencode/plugin/add-a-page | rewrite | 100% | — | 62465 | 0 | 1m58s | — |
| opencode/plugin/prove-and-install | rewrite | 100% | Linux | 62808 | 0 | 1m15s | — |
| copilot/user/add-a-page | rewrite | 100% | — | 62805 | 0 | 1m11s | — |
| copilot/user/prove-and-install | rewrite | 65% | — | 62999 | 0 | 15m25s | the agent finished (copilot timed out after 15m0s); reached the guest (the agent never recorded a platform, so nothing proves the work reached the guest); the level carried it (the agent never recorded a platform, so nothing proves the work reached the guest) |
| gemini/rewrite/add-a-page | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| gemini/rewrite/prove-and-install | rewrite | skipped | — | — | — | — | GEMINI_API_KEY or GOOGLE_API_KEY is not set (Gemini CLI speaks only the Gemini API, which the AI Gateway does not serve) |
| grok/rewrite/add-a-page | rewrite | 100% | — | 61809 | 0 | 50s | — |
| grok/rewrite/prove-and-install | rewrite | 100% | Linux | 61813 | 0 | 1m35s | — |
| pi/rewrite/add-a-page | rewrite | 100% | — | 61941 | 0 | 33s | — |
| pi/rewrite/prove-and-install | rewrite | 100% | Linux | 62044 | 0 | 1m2s | — |
| claude/tool/add-a-page | tool | 100% | — | 62050 | 12 | 58s | — |
| claude/tool/prove-and-install | tool | 100% | Linux | 62195 | 9 | 44s | — |
| kimi/tool/add-a-page | tool | 100% | — | 62264 | 0 | 51s | — |
| kimi/tool/prove-and-install | tool | 100% | Linux | 62265 | 0 | 4m54s | — |
| dsh/tool/add-a-page | tool | 100% | — | 61529 | 0 | 54s | — |
| dsh/tool/prove-and-install | tool | 100% | Linux | 63185 | 0 | 1m51s | — |
| codex/tool/add-a-page | tool | 100% | — | 63255 | 0 | 49s | — |
| codex/tool/prove-and-install | tool | 100% | Linux | 63356 | 0 | 1m36s | — |
| grok/tool/add-a-page | tool | 100% | — | 63468 | 0 | 1m3s | — |
| grok/tool/prove-and-install | tool | 100% | Linux | 63564 | 0 | 1m7s | — |
| claude/shims/add-a-page | shims | 100% | — | 63647 | 4 | 38s | — |
| claude/shims/prove-and-install | shims | 100% | Linux | 63753 | 13 | 1m28s | — |
| kimi/shims/add-a-page | shims | 100% | — | 63823 | 0 | 57s | — |
| kimi/shims/prove-and-install | shims | 100% | Linux | 63913 | 0 | 47s | — |
| codex/shims/add-a-page | shims | 100% | — | 63980 | 0 | 49s | — |
| codex/shims/prove-and-install | shims | 100% | Linux | 64077 | 0 | 1m12s | — |
| copilot/shims/add-a-page | shims | 100% | — | 64158 | 0 | 1m17s | — |
| copilot/shims/prove-and-install | shims | 100% | Linux | 64287 | 0 | 1m19s | — |
| claude/bash-shim/add-a-page | bash-shim | 100% | — | 64364 | 4 | 30s | — |
| claude/bash-shim/prove-and-install | bash-shim | 100% | Linux | 64458 | 7 | 1m8s | — |
| kimi/bash-shim/add-a-page | bash-shim | 100% | — | 64534 | 0 | 1m5s | — |
| kimi/bash-shim/prove-and-install | bash-shim | 100% | Linux | 64617 | 0 | 1m2s | — |
| codex/bash-shim/add-a-page | bash-shim | 100% | — | 63032 | 0 | 46s | — |
| codex/bash-shim/prove-and-install | bash-shim | 100% | Linux | 64852 | 0 | 1m13s | — |
| copilot/bash-shim/add-a-page | bash-shim | 100% | — | 65040 | 0 | 1m5s | — |
| copilot/bash-shim/prove-and-install | bash-shim | 100% | Linux | 65157 | 0 | 1m17s | — |
| openhands/shell/prove-and-install | shell | 100% | Linux | 65226 | 0 | 1m13s | — |
| inside/claude/add-a-page | inside | 100% | — | 65337 | 7 | 1m3s | — |
| inside/claude/prove-and-install | inside | 100% | Linux | 65393 | 19 | 1m48s | — |
| inside/codex/add-a-page | inside | 100% | — | 49272 | 0 | 1m33s | — |
| inside/codex/prove-and-install | inside | 100% | Linux | 49489 | 0 | 3m48s | — |
| inside/opencode/add-a-page | inside | 100% | — | 64694 | 0 | 57s | — |
| inside/opencode/prove-and-install | inside | 100% | Linux | 51161 | 0 | 1m57s | — |
| inside/kimi/add-a-page | inside | 100% | — | 50114 | 0 | 1m34s | — |
| inside/kimi/prove-and-install | inside | 100% | Linux | 50638 | 0 | 1m7s | — |
| t3/orchestrator/add-a-page | orchestrator | 100% | — | 49741 | 0 | 2m3s | — |
| t3/orchestrator/prove-and-install | orchestrator | 100% | Linux | 50637 | 0 | 1m13s | — |
| paperclip/orchestrator/add-a-page | orchestrator | 100% | — | 50379 | 0 | 1m48s | — |
| paperclip/orchestrator/prove-and-install | orchestrator | 100% | Linux | 50925 | 0 | 1m32s | — |
| herdr/orchestrator/add-a-page | orchestrator | 100% | — | 51064 | 0 | 42s | — |
| herdr/orchestrator/prove-and-install | orchestrator | 100% | Linux | 61535 | 0 | 1m5s | — |

## The scorecard

Every claim, and how many cells made it good.

| claim | cells | met |
| --- | --- | --- |
| the sandbox came up | 53 | 53 (100%) |
| a host port was forwarded | 53 | 53 (100%) |
| the dev server answered first | 53 | 53 (100%) |
| the agent finished | 53 | 52 (98%) |
| the page renders | 53 | 53 (100%) |
| the change is in the worktree | 53 | 53 (100%) |
| no host leak | 53 | 53 (100%) |
| the dev server survived | 53 | 53 (100%) |
| the dependency is installed | 27 | 27 (100%) |
| reached the guest | 27 | 26 (96%) |
| the level carried it | 27 | 26 (96%) |

## What each agent did

### claude/rewrite — add-a-page — 100%

- level `rewrite`, 52s, 9 tool calls
- tools: Bash×7, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-add-a-page.trace.log`

### claude/rewrite — prove-and-install — 100%

- level `rewrite`, 1m1s, 10 tool calls
- tools: Bash×8, Write×2
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-rewrite-prove-and-install.trace.log`

### codex/rewrite — add-a-page — 100%

- level `rewrite`, 40s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-add-a-page.trace.log`

### codex/rewrite — prove-and-install — 100%

- level `rewrite`, 1m32s, 0 tool calls
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-rewrite-prove-and-install.trace.log`

### opencode/plugin — add-a-page — 100%

- level `rewrite`, 1m58s, 0 tool calls
- changed: boxer.toml, .opencode/, AGENTS.md, app/app/about/, opencode.json
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-add-a-page.trace.log`

### opencode/plugin — prove-and-install — 100%

- level `rewrite`, 1m15s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .opencode/, AGENTS.md, app/app/clsx/, opencode.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-opencode-plugin-prove-and-install.trace.log`

### copilot/user — add-a-page — 100%

- level `rewrite`, 1m11s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-add-a-page.trace.log`

### copilot/user — prove-and-install — 65%

- level `rewrite`, 15m25s, 0 tool calls
- **the agent finished**: copilot timed out after 15m0s
- **reached the guest**: the agent never recorded a platform, so nothing proves the work reached the guest
- **the level carried it**: the agent never recorded a platform, so nothing proves the work reached the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-user-prove-and-install.trace.log`

### grok/rewrite — add-a-page — 100%

- level `rewrite`, 50s, 0 tool calls
- changed: boxer.toml, .agents/, .grok/, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-add-a-page.trace.log`

### grok/rewrite — prove-and-install — 100%

- level `rewrite`, 1m35s, 0 tool calls
- the level carried it: the integration layer carried it, though boxer logged no rewrite (it does not log one here)
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .grok/, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-rewrite-prove-and-install.trace.log`

### pi/rewrite — add-a-page — 100%

- level `rewrite`, 33s, 0 tool calls
- changed: boxer.toml, .pi/, AGENTS.md, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-add-a-page.trace.log`

### pi/rewrite — prove-and-install — 100%

- level `rewrite`, 1m2s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .pi/, AGENTS.md, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-pi-rewrite-prove-and-install.trace.log`

### claude/tool — add-a-page — 100%

- level `tool`, 58s, 12 tool calls
- tools: Bash×10, Read×1, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-add-a-page.trace.log`

### claude/tool — prove-and-install — 100%

- level `tool`, 44s, 9 tool calls
- tools: Bash×5, Write×1, mcp__boxer__boxer_run×2, mcp__boxer__boxer_status×1
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-tool-prove-and-install.trace.log`

### kimi/tool — add-a-page — 100%

- level `tool`, 51s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-add-a-page.trace.log`

### kimi/tool — prove-and-install — 100%

- level `tool`, 4m54s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, node_modules/, package-lock.json, package.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-tool-prove-and-install.trace.log`

### dsh/tool — add-a-page — 100%

- level `tool`, 54s, 0 tool calls
- changed: boxer.toml, .agents/, .dsh/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-add-a-page.trace.log`

### dsh/tool — prove-and-install — 100%

- level `tool`, 1m51s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .dsh/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-dsh-tool-prove-and-install.trace.log`

### codex/tool — add-a-page — 100%

- level `tool`, 49s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-add-a-page.trace.log`

### codex/tool — prove-and-install — 100%

- level `tool`, 1m36s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-tool-prove-and-install.trace.log`

### grok/tool — add-a-page — 100%

- level `tool`, 1m3s, 0 tool calls
- changed: boxer.toml, .gstack/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-add-a-page.trace.log`

### grok/tool — prove-and-install — 100%

- level `tool`, 1m7s, 0 tool calls
- the level carried it: the agent used the `boxer_run` tool
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-grok-tool-prove-and-install.trace.log`

### claude/shims — add-a-page — 100%

- level `shims`, 38s, 4 tool calls
- tools: Bash×3, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-add-a-page.trace.log`

### claude/shims — prove-and-install — 100%

- level `shims`, 1m28s, 13 tool calls
- tools: Bash×8, Read×1, Skill×1, Write×1, mcp__boxer__boxer_run×1, mcp__boxer__boxer_status×1
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-shims-prove-and-install.trace.log`

### kimi/shims — add-a-page — 100%

- level `shims`, 57s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-add-a-page.trace.log`

### kimi/shims — prove-and-install — 100%

- level `shims`, 47s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-shims-prove-and-install.trace.log`

### codex/shims — add-a-page — 100%

- level `shims`, 49s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-add-a-page.trace.log`

### codex/shims — prove-and-install — 100%

- level `shims`, 1m12s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-shims-prove-and-install.trace.log`

### copilot/shims — add-a-page — 100%

- level `shims`, 1m17s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-add-a-page.trace.log`

### copilot/shims — prove-and-install — 100%

- level `shims`, 1m19s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-shims-prove-and-install.trace.log`

### claude/bash-shim — add-a-page — 100%

- level `bash-shim`, 30s, 4 tool calls
- tools: Bash×3, Write×1
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-add-a-page.trace.log`

### claude/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m8s, 7 tool calls
- tools: Bash×6, Write×1
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-claude-bash-shim-prove-and-install.trace.log`

### kimi/bash-shim — add-a-page — 100%

- level `bash-shim`, 1m5s, 0 tool calls
- changed: boxer.toml, .agents/, .kimi-code/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-add-a-page.trace.log`

### kimi/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m2s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .kimi-code/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-kimi-bash-shim-prove-and-install.trace.log`

### codex/bash-shim — add-a-page — 100%

- level `bash-shim`, 46s, 0 tool calls
- changed: boxer.toml, .agents/, .codex/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-add-a-page.trace.log`

### codex/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m13s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, .agents/, .codex/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-codex-bash-shim-prove-and-install.trace.log`

### copilot/bash-shim — add-a-page — 100%

- level `bash-shim`, 1m5s, 0 tool calls
- changed: boxer.toml, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-add-a-page.trace.log`

### copilot/bash-shim — prove-and-install — 100%

- level `bash-shim`, 1m17s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-copilot-bash-shim-prove-and-install.trace.log`

### openhands/shell — prove-and-install — 100%

- level `shell`, 1m13s, 0 tool calls
- the level carried it: a PATH shim carried the bare command into the guest
- changed: app/package-lock.json, app/package.json, boxer.toml, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-openhands-shell-prove-and-install.trace.log`

### inside/claude — add-a-page — 100%

- level `inside`, 1m3s, 7 tool calls
- tools: Bash×6, Write×1
- changed: boxer.toml, .boxer-eval/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-add-a-page.trace.log`

### inside/claude — prove-and-install — 100%

- level `inside`, 1m48s, 19 tool calls
- tools: Bash×15, Read×2, Write×2
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-claude-prove-and-install.trace.log`

### inside/codex — add-a-page — 100%

- level `inside`, 1m33s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-add-a-page.trace.log`

### inside/codex — prove-and-install — 100%

- level `inside`, 3m48s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, app/where.txt, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-codex-prove-and-install.trace.log`

### inside/opencode — add-a-page — 100%

- level `inside`, 57s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/, opencode.json
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-add-a-page.trace.log`

### inside/opencode — prove-and-install — 100%

- level `inside`, 1m57s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, opencode.json, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-opencode-prove-and-install.trace.log`

### inside/kimi — add-a-page — 100%

- level `inside`, 1m34s, 0 tool calls
- changed: boxer.toml, .boxer-eval/, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-add-a-page.trace.log`

### inside/kimi — prove-and-install — 100%

- level `inside`, 1m7s, 0 tool calls
- the level carried it: the harness itself ran in the guest; nothing was intercepted
- changed: app/package-lock.json, app/package.json, boxer.toml, .boxer-eval/, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-inside-kimi-prove-and-install.trace.log`

### t3/orchestrator — add-a-page — 100%

- level `orchestrator`, 2m3s, 0 tool calls
- changed: app/app/about/, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-add-a-page.trace.log`

### t3/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m13s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-t3-orchestrator-prove-and-install.trace.log`

### paperclip/orchestrator — add-a-page — 100%

- level `orchestrator`, 1m48s, 0 tool calls
- changed: .mcp.json, app/app/about/page.tsx, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-add-a-page.agent.log`

### paperclip/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m32s, 0 tool calls
- the level carried it: the agent typed `boxer run` itself, from the installed agent contract
- changed: app/package-lock.json, app/package.json, app/app/clsx/, where.txt, .mcp.json, boxer.toml
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-paperclip-orchestrator-prove-and-install.agent.log`

### herdr/orchestrator — add-a-page — 100%

- level `orchestrator`, 42s, 0 tool calls
- changed: boxer.toml, .mcp.json, app/app/about/
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-add-a-page.trace.log`

### herdr/orchestrator — prove-and-install — 100%

- level `orchestrator`, 1m5s, 0 tool calls
- the level carried it: the hook rewrote the command into `boxer run`
- changed: app/package-lock.json, app/package.json, boxer.toml, .mcp.json, app/app/clsx/, where.txt
- transcript: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.agent.log`
- trace: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/matrix-herdr-orchestrator-prove-and-install.trace.log`

