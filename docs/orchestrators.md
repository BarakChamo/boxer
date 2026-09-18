# boxer under orchestrators

How boxer behaves when a harness is launched by something other than the user's terminal: T3
Code, Paperclip, OpenHands, or any tool that spawns `claude`, `codex`, `gemini`, `opencode`, or
`grok` as a subprocess. Findings are from reading each project's source on 2026-09-17 and from
headless driver runs on 2026-09-18 (`paperclipai` 2026.916.0, `t3` 0.0.42); verify against the
version you run.

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

## Drivers and checklists (2026-09-18)

`boxer-eval` drives OpenHands (`internal/eval/orch_openhands.go`), Paperclip
(`orch_paperclip.go`) and T3 Code (`orch_t3.go`) headlessly; Multica keeps a checklist driver that
reports one skipped cell naming what is missing (`orch.go`). Each proves the same four things: the
task worktree maps to one VM (`boxer ls` shows `sb-<key>` for it), the hook or ACP path was taken,
the command ran in the guest (`uname -a` starts with `Linux`), and the canary never appeared on the
host.

An orchestrator cuts its own worktree, and its driver only learns the path at run time, so the
driver records it in `Env.Root`. `Env.SessionRoot` prefers it over the checkout, and the oracle
resolves the scope, the configuration and the VM from there (`internal/eval/oracle.go`,
`timing.go`). Without that the oracle would judge the main checkout's VM and miss the one the run
actually used.

### Paperclip (driver; spike 7: does `claude_local` pass env through to `claude-agent-acp`?)

Read from `packages/adapters/claude-local/src/server/{acp.ts,probe-env.ts,execute.ts}` at `master`:

- The adapter's `config.env` is forwarded to the `claude-agent-acp` process (`buildClaudeAcpConfig`
  merges `{...process.env, ...config.env}` for model resolution and passes `env` on the ACP config).
  The login probe lane takes only an allowlist from the caller env (`ANTHROPIC_API_KEY`,
  `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_MODEL`,
  `CLAUDE_CONFIG_DIR`, Bedrock and AWS keys); the run lane copies the config env minus a forbidden
  list. So `ANTHROPIC_BASE_URL` reaches the agent and a fake-model (T1) run is possible in
  principle; `PATH` and `BOXER_TRACE` are not caller-settable, so hooks run with the Paperclip
  server's `PATH` and the oracle must judge by the VM and the guest canary rather than the trace.
Automated since 2026-09-18 as the cell `paperclip/rewrite/project/worktree`
(`internal/eval/orch_paperclip.go`):

```sh
bin/boxer-eval --tier t1 --cell paperclip          # scripted model
bin/boxer-eval --tier t2 --cell paperclip --keep   # live model
```

The driver commits `boxer install claude-code` into the eval repository, starts
`paperclipai test-drive --harness claude --no-browser --data-dir <work>/paperclip` (plus `--model`
at t2) from the eval environment so the agent's hooks inherit `PATH`, then over REST: reads the
company and its CEO agent, PATCHes the agent's `adapterConfig.env` with the gateway variables,
creates a project whose primary workspace is the repository, files the issue, wakes the agent, and
polls `heartbeat-runs` until one finishes.

Two things have to be right, and neither is obvious from the API:

- **The wake must name the issue.** `POST /api/agents/<id>/wakeup` with
  `{"source":"assignment","payload":{"issueId":…}}` puts the issue in the run's context
  (`enrichWakeContextSnapshot`), which is what resolves its project. A bare
  `heartbeat/invoke` resolves no project, logs `No project or prior session workspace was
  available`, and runs the turn in a Paperclip-owned fallback directory — where there is no project
  layer and no worktree, so the command ran on the host (`Darwin`, canary leaked).
- **Isolated workspaces are off by default.** `enableIsolatedWorkspaces` is an experimental
  instance setting; until `PATCH /api/instance/settings/experimental` turns it on,
  `gateProjectExecutionWorkspacePolicy` drops the project's `isolated_workspace` + `git_worktree`
  policy and every run uses the project checkout itself. With it on, Paperclip cuts
  `<repo>/.paperclip/worktrees/<issue-key>` per issue and the VM is keyed to that.

Two smaller ones: the ready line is `Paperclip is ready at http://127.0.0.1:3100.` — the trailing
full stop is part of the line, and the port is fixed, so only one instance runs at a time. And
`adapterConfig.env` replaces the adapter's environment block, so `BOXER_TRACE` does not reach
`claude-agent-acp`: the trace file stays empty and the oracle judges by the VM, the guest canary
and the answer instead (`reachedGuest` in `oracle.go` covers exactly this case).

Proved live on 2026-09-18 (`zai/glm-5.3-flash`): pass in 2 m 4 s for $0.0364 — worktree
`.paperclip/worktrees/TES-1-uname`, one VM keyed to it, canary in that guest and not on the host,
answer `Linux`. Scripted (t1): pass in 21 s.

### T3 Code (driver; spike 8: WS sequence and env inheritance)

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
Automated since 2026-09-18 as two cells (`internal/eval/orch_t3.go`), against `t3` 0.0.42:

```sh
bin/boxer-eval --tier t2 --cell t3code/rewrite --keep   # project layer, stock claude
bin/boxer-eval --tier t2 --cell t3code/inside  --keep   # the harness itself in the guest
```

The driver writes `<base-dir>/userdata/settings.json` with one provider instance
(`providerInstances.claudeAgent`: `driver: "claudeAgent"`, an `environment` list of
`{name, value}` pairs, and a `config` carrying `homePath` and, for the inside cell, `binaryPath`),
starts `t3 serve --mode web --host 127.0.0.1 --port <free> --base-dir <work>/t3 --no-browser <repo>`
and waits for `T3 Code server is ready`, mints a bearer token with
`t3 auth session issue --base-dir … --token-only --ttl 1h`, and talks Effect RPC over
`ws://127.0.0.1:<port>/ws` from a small node script (Go has no WebSocket client in its standard
library). The framing is `{_tag:"Request", id, tag, payload, headers}` out; `Chunk` (each
acknowledged with `{_tag:"Ack", requestId}`), `Exit` and `Defect` back.
`orchestration.dispatchCommand` carries `project.create` and `thread.turn.start`;
`orchestration.subscribeThread` is the event stream.

The one thing that is not guessable is **when the turn is over**. The assistant's text rides the
`thread.message-sent` event with `streaming: true`; the final, non-streaming one has an empty
`text`. Waiting for a non-empty non-streaming message hangs until the driver's own timeout — the
first attempt spent 12 minutes there with the answer already on the wire. The turn ends when
`thread.session-set` drops `activeTurnId` back to `null` after having set it, and the answer is the
last non-empty assistant message.

T3 puts the worktree at `<base-dir>/worktrees/<repo>/<branch>` and reports it on the thread as
`worktreePath`; the driver hands that to the oracle as `Env.Root`. The inside cell installs a shim
with `boxer shim install --harness claude <work>/shims` and names it as the instance's
`binaryPath`, with the eval's Claude config dir under the repository as `homePath`: T3 spawns the
shim, the shim runs `boxer shell claude`, and the harness itself runs in the guest — no hook path,
so the oracle's inside branch applies.

Proved live on 2026-09-18 (`zai/glm-5.3-flash`): `t3code/rewrite/project/worktree` pass in 18 s for
$0.0047, `t3code/inside/shim/worktree` pass in 27 s for $0.0070. Both: one VM keyed to the worktree
T3 created, canary in that guest and not on the host, answer `Linux`. Scripted (t1): 9 s and 18 s.

### Multica (checklist; spike 9: `MULTICA_CLAUDE_ARGS`, env inheritance)

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
- Verified 2026-09-18 by live runs, not by reading: `claude-agent-acp` under Paperclip does load
  the project layer (the hook rewrote the command into the guest with no user-level install);
  Paperclip's per-issue worktree and T3's per-thread worktree each map to exactly one VM, keyed to
  the worktree the orchestrator made.
- Not verified: how either orchestrator behaves when the plugin is installed at the user level as
  well — the double-install idempotence argument above is still read from source only.
