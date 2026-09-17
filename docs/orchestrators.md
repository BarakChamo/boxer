# boxer under orchestrators

How boxer behaves when a harness is launched by something other than the user's terminal: T3
Code, Paperclip, OpenHands, or any tool that spawns `claude`, `codex`, `gemini`, `opencode`, or
`grok` as a subprocess. Findings are from reading each project's source on 2026-09-17; verify
against the version you run.

## The problem: plugins do not follow the harness

A boxer plugin installed at the user level (`~/.claude`, `~/.codex`, …) is invisible to a harness
that an orchestrator launches with its own configuration directory:

| Orchestrator | What it does | Effect on a user-level plugin |
| --- | --- | --- |
| T3 Code | Spawns provider CLIs as subprocesses via ACP; sets `CLAUDE_CONFIG_DIR` to a T3-owned directory per instance (`apps/server/src/provider/Drivers/ClaudeHome.ts`); Codex gets a managed home too | Not loaded |
| Paperclip | Spawns `claude-agent-acp` / Codex ACP with a managed `CLAUDE_CONFIG_DIR` / `CODEX_HOME`, staged into the execution workspace, optionally over SSH or into a remote sandbox | Not loaded |
| OpenHands | Does not spawn a harness CLI at all; its own agent executes actions against a `Workspace` | Not applicable |

The **project layer** is what every launcher reads: `.claude/settings.json`, `.mcp.json`,
`.codex/hooks.json`, `.gemini/settings.json`, `.opencode/plugins/`, `.grok/hooks/boxer.json`.

```sh
boxer install all        # writes hooks, run tool, and instruction into this repository
```

That is the whole integration for T3 Code and Paperclip: boxer on `PATH` where the orchestrator
runs, `boxer install` committed in the repository. Per-thread worktrees (both orchestrators create
one per task) map to one VM each under the default `isolation = "worktree"`; `boxer gc` reaps VMs
whose worktree was removed.

## Both layers installed: no clash

A repository with `boxer install` committed and a developer with the plugin installed runs every
hook twice. This is designed to be harmless:

1. The second `PreToolUse` sees a command that already begins with `boxer run` and allows it
   unchanged (R-CMD-2).
2. Provisioning is idempotent: the second `SessionStart` finds the VM running.
3. A boxer process inside the guest sees `BOXER_INSIDE=1` (set on every `exec`) and does nothing:
   hooks stay silent, `boxer run` executes directly. So even a harness that an orchestrator
   launches *inside* a boxer VM cannot wrap commands a second time.

## Harness-native sandboxes

Codex has its own seatbelt/landlock sandbox. A hypervisor cannot start inside it, so boxer
replaces it rather than nesting: run Codex with `--sandbox danger-full-access` (or
`--dangerously-bypass-approvals-and-sandbox` in `codex exec`) when boxer is the sandbox. The same
applies to any harness that confines the process: boxer's isolation is the VM, and the host-side
`boxer` process needs `/dev/hypervisor` access.

## OpenHands

OpenHands executes through a `Workspace`. `adapters/openhands/boxer_workspace.py` is a
`LocalWorkspace` whose `execute_command` runs `boxer run -c <command>`; files stay on the host,
commands land in the guest. Use it with the SDK directly or with `RUNTIME=process`; the Docker and
Remote sandboxes already isolate execution and would only nest boxer inside them.

## Paperclip

No boxer adapter package: Paperclip's `claude-local`, `codex-local`, `gemini-local`,
`opencode-local`, and `grok-local` adapters spawn the real CLIs. What each one loads, from source:

- **Claude.** `claude-agent-acp` opens sessions with `settingSources: ["user", "project", "local"]`
  (`src/acp-agent.ts`), so the project layer from `boxer install claude-code` applies. Paperclip's
  managed `CLAUDE_CONFIG_DIR` is seeded from the host's `~/.claude/settings.json` and `CLAUDE.md`,
  never from `plugins/`: a user-level plugin does not travel, `boxer install claude-code --user`
  (hooks in `settings.json`) does.
- **Codex.** Project `.codex/hooks.json` loads only when the project layer is trusted, and
  `codex-local` passes no trust bypass. Its managed `CODEX_HOME` is seeded from `config.toml`,
  `config.json`, `instructions.md`, not `hooks.json`. So the reliable form is
  `boxer install codex --user`: inline `[hooks]` tables in `~/.codex/config.toml`, which need no
  trust and are copied into the managed home.
- **Reclaim.** Paperclip ends runs by ending the process; `SessionEnd` is not guaranteed. `boxer
  gc` reaps VMs whose worktree is gone, or idle past `idle_timeout`.

A wrapping adapter would add only Codex hook trust and `boxer up`/`down` around each run; the
`--user` layer and gc cover both without forking Paperclip's adapter code. SSH and remote-sandbox
execution targets sync the worktree to another machine; install `boxer` and smolvm there, or leave
those targets to their own isolation.

## Verified and not

- Verified: T3 `CLAUDE_CONFIG_DIR` isolation; Paperclip managed `CLAUDE_CONFIG_DIR`/`CODEX_HOME`
  and `--setting-sources user`; Codex hooks fire from `.codex/hooks.json` under `codex exec` with
  `--dangerously-bypass-hook-trust`; Claude Code project hooks fire in a headless session.
- Not verified: that `claude-agent-acp` loads project settings by default (Paperclip passes
  `--setting-sources user` only when it manages the AI connection, which implies the default is
  broader); Grok Build's project settings path.
