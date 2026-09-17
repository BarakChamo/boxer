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

OpenHands executes through its terminal tool, which spawns its own PTY shell; a workspace's
`execute_command` is not on the agent's path (the first adapter here got that wrong, and the
live run proved it: the tool ran on the host and hit a PTY error). The integration is one
setting: `TerminalTool` `shell_path` = `boxer-bash` from `boxer shim install --shell`, an
`exec boxer run -- bash` wrapper. The whole interactive shell then runs in the guest, prompt
markers included. Verified live on 2026-09-18 (`openhands/rewrite/sdk/worktree`, gateway model):
guest answered `Linux`, host canary absent. `adapters/openhands/BoxerWorkspace` remains for code
that calls `workspace.execute_command` itself.

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

## Checklists and spike answers (2026-09-17)

`boxer-eval` has a driver for OpenHands (`internal/eval/orch_openhands.go`) and a checklist
driver for each orchestrator below that reports one skipped cell naming what is missing
(`internal/eval/orch.go`). Each procedure proves the same four things: the worktree maps to one
VM (`boxer ls` shows `sb-<key>` for the task worktree), the hook or ACP path was taken (`BOXER_TRACE`
shows a `PreToolUse`/`tool.execute.before` rewrite or `boxer acp` was the agent command), the
command ran in the guest (`uname -a` starts with `Linux`), and installing both the plugin and the
project layer creates one VM, not two.

### Paperclip (spike 7: does `claude_local` pass env through to `claude-agent-acp`?)

Read from `packages/adapters/claude-local/src/server/{acp.ts,probe-env.ts,execute.ts}` at `master`:

- The adapter's `config.env` is forwarded to the `claude-agent-acp` process (`buildClaudeAcpConfig`
  merges `{...process.env, ...config.env}` for model resolution and passes `env` on the ACP config).
  The login probe lane takes only an allowlist from the caller env (`ANTHROPIC_API_KEY`,
  `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_MODEL`,
  `CLAUDE_CONFIG_DIR`, Bedrock and AWS keys); the run lane copies the config env minus a forbidden
  list. So `ANTHROPIC_BASE_URL` reaches the agent and a fake-model (T1) run is possible in
  principle; `PATH` and `BOXER_TRACE` are not caller-settable, so hooks run with the Paperclip
  server's `PATH` and the oracle must judge by the VM and the guest canary rather than the trace.
- Not automated here: `test-drive` boots a full instance with an embedded database, then needs an
  issue, an assignment, and a heartbeat over REST before a run happens.

Checklist:

1. `npm i -g paperclipai`; in a repository with `boxer install claude-code` committed and `boxer`
   on the server's `PATH`: `paperclipai test-drive --harness claude --no-browser --data-dir <tmp>`.
2. Create an issue whose body is the eval prompt (`Run the shell command \`uname -a\` exactly once
   and reply with only the first word of its output`), assign it to the CEO agent, trigger a
   heartbeat.
3. Assert: the run's worktree has a VM (`boxer ls`), the answer is `Linux`, `boxer gc` reclaims
   the VM once Paperclip removes the worktree.

### T3 Code (spike 8: WS sequence and env inheritance)

Read from `pingdotgg/t3code` `packages/contracts/src/{orchestration.ts,rpc.ts}`,
`apps/server/src/provider/Drivers/{ClaudeDriver.ts,ClaudeHome.ts}`, `ProviderInstanceEnvironment.ts`:

- Commands: `project.create {commandId, projectId, title, workspaceRoot, createdAt}` then
  `thread.turn.start {commandId, threadId, message: {messageId, role: "user", text, attachments:
  []}, bootstrap: {createThread: {projectId, title, modelSelection, runtimeMode, interactionMode,
  branch, worktreePath, createdAt}, prepareWorktree: {projectCwd, baseBranch, branch,
  requireWorktree: true}}, createdAt}`: one request creates the thread, its worktree, and the
  first turn. Delivered over the server WebSocket (`apps/server/src/ws.ts`).
- Env: the Claude driver spawns the CLI with `process.env` merged with the provider instance's
  variable list (`mergeProviderInstanceEnvironment`), and `CLAUDE_CONFIG_DIR` set to a T3-owned
  home. So `ANTHROPIC_BASE_URL`, `BOXER_TRACE`, and `PATH` inherit from the `t3` server process:
  a T1 run is possible when the server is started from the eval's environment.
- Not automated here: `t3` (npm package `t3`, 0.0.42) is not installed; it starts the server and
  the local web app together, and Go has no WebSocket client in the standard library, so the
  driver would be a small node script.

Checklist:

1. `npm i -g t3`; start `t3` from a shell with `boxer` on `PATH` and `BOXER_TRACE` set.
2. Outside path: repository with `boxer install claude-code` committed; provider instance
   `claude` stock. Inside path: provider instance whose executable is `boxer` with arguments
   `acp claude` (agent command), `integration = "inside"` in `boxer.toml`.
3. Send `project.create` and `thread.turn.start` with `bootstrap.prepareWorktree`; prompt as above.
4. Assert: VM keyed to the new worktree; outside: trace shows the rewrite; inside: no trace, the
   harness itself ran in the guest; answer `Linux`.

### Multica (spike 9: `MULTICA_CLAUDE_ARGS`, env inheritance)

- The `multica` binary (Homebrew) reads `MULTICA_CLAUDE_ARGS`, `MULTICA_CLAUDE_PATH`,
  `MULTICA_CODEX_ARGS`, `MULTICA_KEEP_ENV_AFTER_TASK`, `MULTICA_AGENT_TEMP_BASE`, and the
  per-harness `MULTICA_*_MODEL` variables (from `strings` on the binary). `MULTICA_CLAUDE_ARGS`
  is where `--plugin-dir <dist/claude-code>` goes; `MULTICA_CLAUDE_PATH=boxer-shim/claude` is the
  inside path.
- The daemon needs a server: `multica auth status` answers `No server configured. Run 'multica
  setup' first.` on this machine, so env inheritance could not be observed. Live only until a
  Multica account exists.

Checklist: `multica setup` (account), `multica daemon start` from a shell with `boxer` on `PATH`,
`multica repo add <repo with boxer install committed>`, `multica issue create --title "uname"
--description "<prompt>" --assignee <agent>`, then `multica issue runs` and `run-messages`; assert
a VM for the daemon's worktree and `Linux` in the run output.

### herdr (no driver)

herdr owns terminal panes and runs the harness interactively with the shell's own environment,
so the plugin or project layer applies unchanged. Checklist: `herdr pane split`, `herdr pane run
w:p "claude --plugin-dir <dist/claude-code>"` in a worktree, `herdr pane send-text` the prompt,
`herdr agent wait --until done`, `herdr pane read`; assert `Linux` and a VM for the pane's cwd.

### Conductor, local (no driver)

Conductor's public API drives cloud workspaces only; the local Mac app runs Claude Code in
`~/conductor/workspaces/<repo>/<ws>`. Checklist: a `conductor.json` whose `setup` script runs
`boxer doctor` and whose `run` script runs `boxer run -c 'uname -a'`; create one workspace by
hand and read the two scripts' output in the app; assert `Linux` and a VM keyed to the workspace
path.

## Verified and not

- Verified: T3 `CLAUDE_CONFIG_DIR` isolation; Paperclip managed `CLAUDE_CONFIG_DIR`/`CODEX_HOME`
  and `--setting-sources user`; Codex hooks fire from `.codex/hooks.json` under `codex exec` with
  `--dangerously-bypass-hook-trust`; Claude Code project hooks fire in a headless session.
- Verified 2026-09-17: Grok Build reads project hooks from `.grok/hooks/*.json` (not
  `.grok/settings.json`), gated by folder trust (`--trust` once, or `GROK_FOLDER_TRUST=0` for a
  headless run); `boxer install grok` writes that path.
- Not verified: that `claude-agent-acp` loads project settings by default (Paperclip passes
  `--setting-sources user` only when it manages the AI connection, which implies the default is
  broader).
