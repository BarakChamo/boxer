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

**The inside path needs the git warm-up.** T3 creates the worktree and starts the harness in the
same step, and its provider session fails with `turn/setPermissionMode failed` while a cold VM
boots (reproduced three times: 38 s, 39 s, 32 s). `boxer install git` writes the post-checkout
hook, so `git worktree add` warms the sandbox before the harness starts; the cell then passes
(2026-09-18, 1 m 15 s, $0.0041). Any orchestrator that creates a worktree and launches into it
wants that hook.

### Multica (checklist; spike 9, corrected 2026-09-18)

- The `multica` binary (Homebrew) reads `MULTICA_CLAUDE_ARGS`, `MULTICA_CLAUDE_PATH`,
  `MULTICA_CODEX_ARGS`, `MULTICA_KEEP_ENV_AFTER_TASK`, `MULTICA_AGENT_TEMP_BASE`, and the
  per-harness `MULTICA_*_MODEL` variables (from `strings` on the binary). `MULTICA_CLAUDE_ARGS`
  is where `--plugin-dir <dist/claude-code>` goes; `MULTICA_<PROVIDER>_PATH=<shim>/claude` is the
  inside path.
- **Runtime profiles are the supported form of the same thing**, and the environment variables are
  the override of last resort: `multica runtime profile create --command-name <name>` declares a
  runtime, and `multica runtime profile set-path` points it at a binary — boxer's harness shim.
  Prefer the profile: it is per-installation configuration rather than per-process environment, so
  it survives a daemon restart and applies to every task the daemon runs.
- The daemon needs a server: `multica auth status` answers `No server configured. Run 'multica
  setup' first.` on this machine, so env inheritance could not be observed. A driver therefore
  needs a **self-hosted Multica server**; the setup is documented and scriptable
  (`multica setup`, `multica daemon start`), and was not attempted here. This is the one
  orchestrator left as a checklist for 1.0.

Checklist: `multica setup` (account or self-hosted server), `multica runtime profile create
--command-name claude` plus `runtime profile set-path <shim>/claude` (or `MULTICA_CLAUDE_PATH`),
`multica daemon start` from a shell with `boxer` on `PATH`, `multica repo add <repo with boxer
install committed>`, `multica issue create --title "uname" --description "<prompt>" --assignee
<agent>`, then `multica issue runs` and `run-messages`; assert a VM for the daemon's worktree and
`Linux` in the run output.

### herdr (driver, corrected 2026-09-18)

An earlier note here called herdr undrivable. It is not: 0.9.1 documents a socket API, a plugin
API, and a configurable pane shell, and boxer now drives it headlessly
(`internal/eval/orch_herdr.go`, cell `herdr/rewrite/project/worktree`).

```sh
bin/boxer-eval --tier t1 --cell herdr          # scripted model
bin/boxer-eval --tier t2 --cell herdr --keep   # live model
```

The driver starts a private server (`HERDR_SOCKET_PATH`, `HERDR_HOME` and `HERDR_CONFIG_PATH` keep
it clear of the user's own session), then `workspace create --cwd <repo>`, `pane split`,
`agent start boxeval --kind claude --pane <id>`, `agent prompt … --wait --until idle` and
`agent read --source recent-unwrapped`. The pane inherits the server's environment, so the project
layer, `PATH` and `BOXER_TRACE` all apply, and the oracle judges it like any other cell.

Three things are not guessable and cost a run each:

- **`agent start` refuses a pane whose shell it cannot recognise**: `agent_pane_busy`, "not an
  available shell". A shell another tool has renamed (kiro-cli rewrites `argv0` to
  `bash (kiro-cli-term)`) is refused, so the driver pins `terminal.default_shell = "/bin/sh"` with
  `shell_mode = "non_login"` in its own config file.
- **A pane runs the harness interactively**, where Claude Code's workspace-trust question blocks
  startup; headless drivers never see it. The driver accepts it once in its private config dir
  (`projects.<root>.hasTrustDialogAccepted`).
- **`agent_not_ready` is not a failure**: the name stays usable, so the driver waits for `idle`
  rather than giving up there.

**Levels.** herdr needs no boxer-specific code to sandbox a pane:

- **Level S.** `terminal.default_shell` in `~/.config/herdr/config.toml` (or `HERDR_CONFIG_PATH`)
  pointed at `boxer-bash` from `boxer shim install --shell` puts every new pane's shell in the
  sandbox for its worktree.
- **Harness shims.** `boxer shim install --harness claude,codex` on `PATH` makes `agent start
  --kind claude` launch `boxer shell claude`. herdr 0.9.1 classifies a pane from its screen buffer,
  not from the process tree or an environment variable, so the wrapper does not confuse it. The
  shim also exports `HERDR_AGENT=<kind>`, which 0.9.1 ignores; no sandbox-wrapper environment
  contract exists in this version (the only `HERDR_AGENT*` string in the binary is
  `HERDR_AGENT_DETECTION_MANIFEST_CATALOG_URL`).
- **Plugin.** `adapters/herdr/herdr-plugin.toml`, linked with `herdr plugin link adapters/herdr`,
  carries a `start-agent` action that opens a pane running `boxer shell claude` and a
  `worktree.created` event that runs `boxer up --detach`, so a worktree herdr cuts has a warm
  sandbox before anything runs in it. The manifest schema was derived by validating against the
  running binary: `id`, `name`, `version`, `min_herdr_version` required, `platforms` warned about
  when absent, every `command` an argv array, `[[actions]]` with `id`/`title`/`contexts`,
  `[[panes]]` with `id`/`title`/`placement`, `[[events]]` with `on`/`command`. The Vercel sandbox
  plugin is the blueprint; ours is simpler because the worktree is already local.

**Coexistence, verified.** `herdr integration install claude` writes
`~/.claude/hooks/herdr-agent-state.sh` and a `SessionStart` group in `~/.claude/settings.json` —
the same file `boxer install claude-code --user` writes. Installed in either order, both survive:
each merges into the existing `hooks` map, and `SessionStart` ends up with both groups
(verified 2026-09-18 with `CLAUDE_CONFIG_DIR` pointed at a scratch directory, both orders,
`herdr integration status` still reporting `claude: current`).

### Conductor, local (checklist; corrected 2026-09-18)

Our earlier note named `conductor.json`. That is legacy. Conductor reads
`.conductor/settings.toml` in the repository, with `.conductor/settings.local.toml` beside it and
`~/.conductor/settings.toml` and `~/.conductor/settings.managed.toml` above it, and it has a
first-class override we had missed: **the harness executable path**.

```sh
boxer shim install --harness claude,codex,opencode
boxer install conductor       # writes .conductor/settings.toml
```

`boxer install conductor` writes a managed block with `claude_code_executable_path`,
`codex_executable_path` and `opencode_executable_path` pointing at those shims (the first two are
documented repository settings; the third is not in Conductor's published table, and OpenCode's
path is set in the app's preferences, so treat it as inert until verified), an
`[environment_variables]` table, and `[scripts]` whose `setup` warms the sandbox
(`boxer up --detach && boxer doctor`) as the workspace is created. Conductor then spawns the shim,
the shim runs `boxer shell <harness>`, and the harness itself runs in the VM keyed to the
workspace — no hook path at all.

It stays a checklist, because Conductor's public API drives cloud workspaces only.

**There are two ways in, and the cheap one probably already works.** Conductor runs each harness's
real binary against an ordinary git worktree, and its own documentation says a repository's
`.mcp.json` is inherited by the Claude Code sessions it launches. If that is true of hooks as well
— which its documentation does not say either way — then `boxer install claude-code` is the whole
integration and the shims are only for people who want the harness itself in the VM.

Ten minutes settles it:

```sh
cd your-repository
boxer install claude-code          # the project layer: .claude/settings.json + .mcp.json
boxer down --all && boxer ls       # start from nothing

# In Conductor: create a workspace on this repository, then ask the agent to run `uname -a`.

boxer ls                           # expect exactly one VM, keyed to Conductor's worktree
#   ~/conductor/workspaces/<repo>/<workspace>
```

Three outcomes, and each means something different:

| What you see | What it means |
| --- | --- |
| The agent answers `Linux`, and `boxer ls` shows a VM for Conductor's worktree | Hooks fire. The project layer is the integration; nothing else is needed |
| The agent answers `Darwin`, and `boxer ls` is empty | Conductor's sessions do not load project hooks. Use the executable-path shims above instead |
| The agent answers `Linux` but `boxer ls` is empty | Something else sandboxed it, not boxer. Worth a closer look before believing either row |

Whichever it is, the status table should say which one was seen and on what date, rather than
carrying an inference.

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
