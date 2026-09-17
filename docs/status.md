# Status: operational slice v0.1

The stopping point for this slice is: comprehensive evals run against real harnesses and real
orchestrators, with every skip explained. This file is rewritten from real runs at the end of
Phase 4 (see the plan in the session; phases A/B/C below).

## Proven (fill from `docs/eval-t1.md`, `docs/eval-t2.md`)

| Harness | Outside rewrite | Outside tool | Inside shell | ACP | T2 live |
| --- | --- | --- | --- | --- | --- |
| Claude Code | | | | | |
| Codex | | | | | |
| Gemini CLI | | | | | |
| OpenCode | | | | | |
| pi | | | | | |
| Kimi | | | | | |
| Grok | | | | | |
| DSH | | | | | |

| Orchestrator | Path | Result |
| --- | --- | --- |
| OpenHands | adapter | |
| Paperclip | project layer / ACP | |
| Multica | project layer | |
| T3 Code | ACP (`boxer acp`) and project layer | |
| herdr | checklist | |
| Conductor (local) | checklist | |

## Known limits

- One host drives one smolvm at a time; evals take `~/.local/state/boxer/eval.lock`.
- First `boxer shell <harness>` per host pays the harness install (until per-harness base packs).
- Claude Code's macOS Keychain login does not enter the VM; use `claude setup-token`.
- Harness-native sandboxes are turned off inside the VM by the harness table.

## Baseline before this slice (2026-09-17)

Unit tests green; smoke 44/44; T1 31/31 outside and 11/11 inside, each cell green, with one
OpenCode start-up flake and inside cells costing 30 s to 11 min from npm installs.

## What comes next

Git-hook install, `worktree.manage`, devcontainer loader, `Backend` interface, Agent Plugins
bundle collapse, MCP lifecycle signals, boxes dashboard, Homebrew tap, plugin marketplace repos.
