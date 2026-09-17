# boxer evaluation plan: harnesses and orchestrators

Two deliverables, one suite:

1. **Evals** that prove the tooling works, and keeps working, in every execution mode: command
   rewrite, and hooks/skills/tools only (no rewrite).
2. **E2E tests** for every harness and orchestrator boxer claims to support, deterministic and
   fast wherever the harness allows, live where only a real model can prove the behaviour.

Facts below were read from each project's source or docs on 2026-09-17 and are cited inline; items
marked *spike* are unverified and have a numbered spike in §7.

## 1. The core idea: three tiers, one oracle

| Tier | What runs | Model | Speed | Where |
| --- | --- | --- | --- | --- |
| T0 unit | boxer packages against a fake `smolvm` | none | seconds | every push |
| T1 deterministic e2e | real smolvm, the real harness binary, the real hooks/plugin machinery, a **fake LLM** that scripts the turns | scripted | ~1 min per harness | every push on a Mac runner |
| T2 live | same scenarios, real model through the user's login or key | real | minutes, costs money | nightly and before release |

T0 exists (`go test ./...`, 10 packages). T1 is the new work and the point of this plan: every
harness here lets the model endpoint be redirected (`ANTHROPIC_BASE_URL`, OpenAI-compatible
`model_providers`, `opencode.json` provider `baseURL`, pi `models.json`, Kimi `config.toml`
provider, OpenHands `base_url`), so a scripted server can play the model and the whole run
becomes reproducible: the "agent" always issues exactly the tool call we script, and what we
observe is purely boxer plus the harness. T2 stays for the one thing a script cannot prove: that a
real model *follows the injected brief* in tool mode without being denied first.

**One oracle for every tier**, `evals/oracle`:

| Check | How |
| --- | --- |
| Ran in the guest | Command is `uname -a`; output must start with `Linux sb-<scope key>`; host is Darwin |
| No host leak | Scenario also runs `touch /tmp/boxer-leak-<run id>`; `/tmp` is not mounted, so the file must exist in the guest and must not exist on the host |
| Zero corrections | `BOXER_TRACE` log has no `permissionDecision: deny` / `decision: deny` / `{deny}` unless the scenario expects one |
| Right path | Trace shows the expected event names for the harness dialect; rewrite mode shows `updatedInput`/`tool_input`/`command` rewrite; tool mode shows `boxer_run` tool call or agent-typed `boxer run` |
| Lifecycle | VM absent before → present after first sandboxed command (or at SessionStart when configured) → reclaimed by `destroy_on`/`gc` as configured |
| Scope | VM name equals `sha256`-derived key for the worktree; two worktrees → two VMs (or one under `isolation = repo`); orchestrator-created worktrees follow the same rule |
| Idempotence | Plugin *and* project layer installed together: exactly one rewrite, one VM, no duplicate provisioning |
| Inside guard | Same harness launched inside the guest (`BOXER_INSIDE=1`) produces no rewrite and no second VM |

## 2. Scenario matrix

Axes, applied per harness where the harness supports them:

- **Mode**: `rewrite` · `tool` (shell denied or removed; `boxer_run` MCP or `boxer run`) · `off` (control: must run on the host)
- **Entry**: plugin/extension bundle (`boxer package`) · project layer (`boxer install`) · user layer (`--user`, Claude and Codex) · both plugin and project
- **Isolation**: `worktree` · `session` (harnesses that send `session_id`) · `subagent` (Claude Code, Codex, Grok with `SubagentStart`) · `repo`
- **Failure policy**: `create_on = [session_start]` without `run` → NO_SANDBOX error contract · `on_sandbox_unavailable = passthrough` · failing `setup` → SETUP_FAILED and VM deleted
- **Block-only harnesses** (DSH, Kimi): `enforcement = both` with shims → guest; `enforcement = hook` → deny with fix

Not every cell is meaningful; the runner declares the matrix per harness in
`evals/matrix.yaml` and reports the cells it ran. The full matrix runs in T1; T2 runs `rewrite` and
`tool` with default isolation per harness.

## 3. Harness sheet

| Harness | Hook mechanism boxer uses | Headless driver | T1 model redirect | T2 credential | Status today |
| --- | --- | --- | --- | --- | --- |
| Claude Code | plugin (`--plugin-dir`) or `.claude/settings.json`; `PreToolUse` `updatedInput`; `SessionStart` context; `SubagentStart` | `claude -p --output-format stream-json --permission-mode bypassPermissions` | `ANTHROPIC_BASE_URL` + dummy `ANTHROPIC_API_KEY`, temp `CLAUDE_CONFIG_DIR` (*spike 1*: OAuth precedence) | logged in | live 2/2 |
| Codex | `.codex/hooks.json` (needs `--dangerously-bypass-hook-trust`) or user `config.toml` `[hooks]`; `updatedInput` | `codex exec --dangerously-bypass-approvals-and-sandbox -o last.txt` | temp `CODEX_HOME` with `[model_providers.fake] base_url` (*spike 2*: `wire_api` chat vs responses) | ChatGPT (quota until 2026-09-20) or `OPENAI_API_KEY` | hooks fire live; turn blocked |
| Gemini CLI | extension (`extensions install --consent`) or `.gemini/settings.json`; `BeforeTool` `tool_input`; `excludeTools` in tool mode | `gemini -p --yolo` | `GOOGLE_GEMINI_BASE_URL` (*spike 3*) | Google login or `GEMINI_API_KEY` | installs; no login |
| OpenCode | `.opencode/plugins/boxer.ts` → `boxer hook opencode`; `tool.execute.before` mutates `args.command` | `opencode run` | `opencode.json` provider with `baseURL` (OpenAI-compatible) | `opencode auth login` | installed; no creds |
| Grok Build | `.grok-plugin` plugin (validates) or `.grok/settings.json`; `updatedInput` | `grok -p --permission-mode bypassPermissions` | `XAI_BASE_URL` (*spike 4*) | `grok login` or `XAI_API_KEY` | validates + installs; not signed in |
| Kimi Code | user `config.toml` `[[hooks]]`, block-only; `.kimi-code/mcp.json`; shims | `kimi -p --auto`, `KIMI_CODE_HOME` for an isolated home | `config.toml` provider `base_url` (*spike 5*) | Kimi login | install verified |
| **pi** (new) | `.pi/extensions/boxer.ts` or `-e`; `tool_call` with mutable `event.input.command` and `{ block, reason }`; `registerTool` for `boxer_run`; `session_start` for the brief | `pi -p` | `~/.pi/agent/models.json` custom provider (OpenAI/Anthropic API) | `/login` Claude Pro/Max, or key | not integrated; needs dialect + bundle |
| DSH | hooks plugin reading `.dsh/hooks.json`, block-only; shims | `npx @deepseek-ai/dsh` (*spike 6*: headless flag) | unknown | DeepSeek key | bundle from docs only |

Sources: pi `packages/coding-agent/docs/extensions.md`; Kimi hooks and MCP docs; Codex hooks
reference; Gemini extension reference; Grok `grok --help`/`plugin validate`; Claude hooks and
plugins reference.

## 4. Orchestrator sheet

All orchestrators below spawn a harness the user already has; the plan uses **Claude Code** under
each (you are logged in), and adds Codex/others when credentials arrive.

| Orchestrator | How it launches the harness | Worktrees | Where boxer attaches | Headless driver for the test | Deterministic? |
| --- | --- | --- | --- | --- | --- |
| Paperclip | `claude-agent-acp` with managed `CLAUDE_CONFIG_DIR` seeded from `~/.claude/settings.json`+`CLAUDE.md`; `settingSources: user, project, local` | one per heartbeat | project layer (`boxer install claude-code`) or `--user`; Codex via `--user` inline hooks | `npx paperclipai test-drive --harness claude` (isolated instance, `claude_local`, worktree execution) then `paperclipai issue create` / assign / heartbeat; REST at `localhost:3100` | yes: `ANTHROPIC_BASE_URL` in adapter `env` reaches the ACP process (*spike 7*) |
| T3 Code | provider CLIs as ACP subprocesses; `CLAUDE_CONFIG_DIR` set to a T3-owned dir (`ClaudeHome.ts`) | per thread ("New worktree") | project layer only | `t3 serve` + a small WS client using `packages/contracts` (`project.create`, `thread.create`, `orchestration.dispatchCommand`) (*spike 8*: no documented prompt CLI) | yes if env reaches the subprocess (*spike 8*) |
| Conductor (installed) | local Mac app runs Claude Code in `~/conductor/workspaces/<repo>/<ws>`; public API/CLI (`conductor workspaces/sessions/messages`) is **cloud workspaces only** | one per workspace | plugin (user layer, if inherited) + project layer | local: `conductor.json` setup/run scripts assert `boxer doctor`/`boxer run` inside the workspace; prompt step manual. Cloud: full API driver, but boxer would need smolvm in the cloud workspace (out of scope) | local: semi-automated; live only |
| herdr | owns terminals; the harness runs interactively in a pane; no wrapping, no env override | none (uses your cwd; pair with `git worktree add`) | plugin or project layer, unchanged | `brew install herdr`; `herdr pane split`, `pane run w:p "claude"`, `pane send-text`, `agent wait --until done`, `pane read` | yes (same env as a terminal) |
| Multica (installed) | local daemon spawns the CLI as a subprocess in a git worktree from `.repos/`; private `TMPDIR`; `MULTICA_CLAUDE_ARGS` for extra args | one per task | project layer; plugin via `MULTICA_CLAUDE_ARGS="--plugin-dir …"` | `brew install multica-ai/tap/multica`; `multica daemon start`; `multica issue create --title … --assignee …`; `multica issue runs`, `run-messages` | live only (Multica Cloud account); daemon env for base URL (*spike 9*) |
| OpenHands | no harness; SDK `Workspace` | none (working_dir) | `adapters/openhands/boxer_workspace.py` | `pip install openhands-sdk`; a 30-line script: `Conversation(agent, workspace=BoxerWorkspace(...))` | yes: LiteLLM `base_url` to the fake server |

Sources: T3 `apps/server/src/provider/Drivers/ClaudeHome.ts`, `packages/contracts`; Paperclip
`cli/src/commands/test-drive.ts`, `docs/adapters/claude-local.md`, `claude-agent-acp
src/acp-agent.ts`; Conductor `conductor --help`, `/docs/api`; herdr `/docs/socket-api/`; Multica
`CLI_AND_DAEMON.md`.

## 4b. Status after the first pass (2026-09-17)

Tier T1 runs green: **31/31 outside cells** across Claude Code (9), Codex (6), Gemini CLI (5),
OpenCode (4), pi (4), Kimi (3), and **11/11 inside cells** (shell: claude, codex, gemini, opencode,
pi, kimi; ACP: claude, codex, gemini, kimi, opencode), all on 2026-09-17. Honest footnotes: one
outside cell (OpenCode tool/noncompliant) failed once in 4 s with no VM and passed on rerun, so
OpenCode start-up is a known flake; inside cells cost 30 s to 11 min each because every cell
installs its harness through npm in a fresh VM (pi 10.7 min, Claude 2.7 to 11 min), and before the
image pack was shared across cells (`BOXER_PACKS`) two cells stalled on image pulls. Run inside
cells one at a time. Every spike that mattered is closed and its answer is in a driver:

| Learned | Where it lives now |
| --- | --- |
| Claude needs a pre-approved dummy key in a private `CLAUDE_CONFIG_DIR` | `eval/claude.go` |
| Codex: `wire_api = "responses"`, `exec_command` with `cmd`, Responses **namespace** tools called as `{name, namespace}`, hook trust needed even for a fresh user home, stdin must be closed | `fakellm`, `eval/codex.go` |
| Gemini: `GEMINI_CLI_HOME`, `GEMINI_CLI_TRUST_WORKSPACE`, the model router wants schema-shaped JSON, `GOOGLE_GEMINI_BASE_URL` | `fakellm/gemini.go`, `eval/gemini.go` |
| OpenCode trusts `PWD` over cwd; `external_directory` permission; stdin must be closed | `eval/opencode.go`, `Env.BaseEnv` |
| pi has no config-dir variable; HOME override needs a smolvm HOME-restoring wrapper | `eval/pi.go` |
| Kimi ignores `updatedInput` and PATH shims: tool mode only | `eval/kimi.go`, Kimi README |
| Grok: `run_terminal_command` requires `description`; plugin hooks discovered as zero in `-p` | `fakellm` required-args; open |
| Docker Hub anonymous quota exhausted by per-cell pulls; `mirror.gcr.io` has none | `Env.mkrepo`, `registryHosts` |
| Two concurrent provisions raced smolvm; per-scope lock added | `box.Ensure`, concurrency test |
| smolvm re-pulls the image for every machine; pulls stalled or hit quota in a third of inside cells | `box.packed`: one `smolvm pack create` per image per host, machines created `--from` it |
| Inside: `codex-acp` (Rust) needs `libssl3`, absent from the slim node image; the `@zed-industries` package is deprecated | `inside.Harnesses["codex"].Install`, `deb.debian.org` allowed |
| Inside: npm idle timeouts against the registry through TSI | long fetch timeouts, install line retried once |
| Inside: Kimi prints `To resume this session: kimi -r …` after the answer | `eval/inside.go` parser |
| Inside: Codex's bwrap sandbox fails on the mounted worktree (`bwrap: Can't mkdir …/.agents: Permission denied`); codex-acp's default agent mode re-enables it | `inside.Harnesses["codex"]` `Args`/`GuestEnv` (R-INT-4a) |
| Inside: codex-acp streams a `Warning: Model metadata …` chunk before the answer | ACP client reads the last text line |

The matrix is reorganised by integration level (requirements §7.6): Level 0 cells (MCP + skill,
tool mode) run for every harness; Level 1 cells (hooks, rewrite) only where rewrite is verified;
Level 2 cells for OpenCode and pi. Kimi and DSH have Level 0 cells only.

### Packaging standard

Agent Plugins 1.0.0 packages skills + MCP servers only; hooks stay client-specific. The eval gains
a T0 conformance cell (schema validation of `plugin.json`/`mcp.json`) and a Level 0 cell per harness
that installs **the standard directory** rather than a native bundle, so adoption by each loader is
measured, not assumed. Launch clients: Codex/ChatGPT, Cursor, Copilot, Kiro, VS Code; Claude Code
installs spec-shaped plugins today; Gemini's maintainer joined at launch.

### Timing matrix (signals are measured, not assumed)

For each harness, cells run the four worktree timings — created before the session, at session
start, mid-session by the agent's shell, never — crossed with signal sets: core only; core + git
hook; core + harness hooks; all three. The oracle adds: exactly one VM per scope across all
signals, the scope key follows the cwd of each command (main checkout, then the new worktree after
the agent moves), no host leak in any timing, reclaim by the first available layer. `doctor`'s
signal report for the harness must match what the trace shows fired.

### Inside mode cells, second pass (2026-09-17, harness packs)

After the first successful install of a harness boxer packs that VM (`smolvm pack create
--from-vm`) and every later VM for the same harness is created from the pack (R-GUEST-4). One cell
at a time, real smolvm 1.16.1, fake model, node image already packed:

| Cell | First VM for the harness on this host (install + pack) | Next worktree, same harness |
| --- | --- | --- |
| inside-kimi | 24.4 s | 5.3 s |
| inside-gemini | 17.0 s | |
| inside-opencode | 39.3 s | |
| inside-grok (new) | 30.8 s | |
| inside-pi | 36.3 s (10.7 min in the first pass) | |
| acp-grok (new) | | 6.0 s (from the inside-grok pack) |
| boxer acp claude, SDK example client | 62 s (install 45 s) | |

The pack itself costs about 7 s (stop, `pack create --from-vm` at 4.6 s, start) and is 130 to 365 MB
per harness under `BOXER_PACKS` or `~/.local/state/boxer/packs`; `boxer gc` prunes packs no
machine references after `idle_timeout`. Timings for claude and codex are in the rows below once
their reruns land.

### ACP with a real client

Client: the ACP TypeScript SDK's own example client (`@agentclientprotocol/sdk@1.4.0`,
`dist/examples/client.js`, which spawns the agent and drives `initialize`, `session/new`,
`session/prompt` through the SDK's zod-validated connection). The only edits: the spawn line reads
the agent command from `AGENT_CMD` instead of the bundled example agent, and the prompt from
`PROMPT`. No `acp` CLI or `example-client` package exists on npm, and Zed is not installed here, so
this is the closest off-the-shelf headless client. Agent: `bin/boxer acp claude -e
CLAUDE_CONFIG_DIR=<repo>/.boxer-eval/claude -e ANTHROPIC_BASE_URL=<fakellm> -e ANTHROPIC_API_KEY=<pre-approved
fake key> ...`, the same environment the eval cell uses, in a fresh worktree with
`integration = "inside"`. Transcript, trimmed of npm output:

```
fake model at http://192.168.1.4:58244
boxer: Claude Code's macOS Keychain login does not enter the VM; run `claude setup-token` ...
boxer: installing claude in the sandbox (once per host)
boxer: caching mirror.gcr.io/library/node:24-bookworm-slim with claude installed (once per host)
Connected to agent (protocol v1)
[session/create] sessionId=4928d089-... phase=sdk-initialize durationMs=194 totalMs=203
Created session: 4928d089-c3ff-4896-8e32-d99b642d53dc
User: Run uname -a and reply with only the first word of its output.
Terminal (pending)
Tool call `toolu_fake_0` updated: completed
Linux
Agent completed with: end_turn
real 1m2.348s
```

The fake model log shows two calls (the `uname -a` tool call, then the answer). claude-agent-acp
executed `Terminal` without a `session/request_permission` round trip and made no `fs/*` request,
so the SDK validated every message boxer relayed and nothing in the minimal eval client needed
changing. The first attempt failed inside `smolvm machine exec` during the npm install
(`WARN failed to reset socket write timeout ... Error: io operation failed: connection closed`,
after 5 s); the retry passed. That exec drop is a smolvm flake boxer surfaces as
`HARNESS_INSTALL_FAILED` with the rerun command; it is not retried automatically. The login hint
above fired because the first version checked only the host environment; it now also sees
credentials passed with `-e`.

### Inside mode cells

`integration = "inside"` adds, per harness, a cell that runs the harness inside the VM against the
fake model (guest reaches the host's loopback through smolvm's TSI networking) and asserts `Linux`
from `uname -a`; and one cell that drives `boxer acp <harness>` with a minimal ACP client. Outside
cells are unchanged.

## 5. Work items

Ordered; each lands with its own proof.

| # | Item | Proof |
| --- | --- | --- |
| 1 | **`evals/fakellm`**: Go server speaking Anthropic Messages (incl. SSE) and OpenAI Chat Completions + Responses. Scenario JSON: ordered turns (`tool_call bash <cmd>` → `text <answer>`); asserts the tool schemas it is offered (so we see `boxer_run` present in tool mode); records every request | unit test per protocol; `claude -p` against it returns the scripted answer |
| 2 | **`evals/runner`** (Go, replaces `harness.sh`): reads `matrix.yaml`; per harness a driver with `Prepare(repo, cell)`, `Run(prompt) (transcript)`, `Cleanup()`; shared `oracle`; `--tier t1|t2`; emits `evals/report.md` matrix + JUnit XML | runner passes T1 for Claude Code with fakellm |
| 3 | **Leak canary + scope checks in the oracle**; property test for `decide` (random compound commands: wrapped-once, never double-wrapped, passthrough never wrapped) | `go test` property runs 10k cases |
| 4 | **pi dialect + bundle + install**: `internal/hook` dialect `pi` (rewrite → `{command}`, deny → `{deny}`, context → `{context}` as OpenCode); template `.pi/extensions/boxer.ts` (`tool_call` mutates `event.input.command`, `registerTool("boxer_run")`, `session_start` prints the brief); `install pi` | unit + smoke + T1 |
| 5 | Drivers: Claude Code, Codex, Gemini, OpenCode, Grok, Kimi, pi, DSH (optional) | T1 green for every harness whose spike passes; T2 green where credentials exist |
| 5a | MCP lifecycle: provision on `initialize`, `instructions`, reclaim on EOF; `install` writes the native shell-disable setting; hook code moves to the optional `boxer-rewrite` extension | T1 Level 0 cells pass on all six harnesses with no hooks installed |
| 5c | Inside mode: `integration` key, `boxer shell`, `boxer acp`, harness install table, config-dir mounts, harness shims | inside T1 cells per harness; ACP cell |
| 5b | Collapse the eight bundles into one Agent Plugins directory with namespaced hook folders and native compatibility manifests; Codex local-marketplace wrapper; schema conformance test | `go test`; `claude plugin validate`, `grok plugin validate`, `gemini extensions install`, `codex plugin marketplace add` all accept the one directory |
| 6 | Orchestrator drivers: Paperclip (test-drive), herdr, Multica, OpenHands; T3 WS client; Conductor setup-script assertions + manual checklist | each proves: worktree → VM key, project-layer hooks fire, `gc` after worktree removal, plugin+project double install idempotent |
| 7 | **CI**: T0 on Linux and macOS on every push; T1 on a macOS arm64 runner with smolvm (*spike 10*: Hypervisor.framework on GitHub-hosted runners; fallback self-hosted Mac mini) on every push; T2 nightly with secrets, skips named | workflows green; report artifact |
| 8 | Requirements §12 status table generated from the last report, not hand-written | `boxer` repo doc updated by CI |

Existing pieces reused: `evals/smoke.sh` (44 checks; stays as the T1 "boxer alone" lane until the
runner subsumes it), `BOXER_TRACE`, `vmtest.Install`, `bundle.Render`, `install.Install/User`.

## 6. What "always works" means here

- T0 and T1 gate every push. A harness release that changes a hook field breaks T1 the same day,
  not when a user hits it.
- Each harness driver pins the harness version it was verified against (`matrix.yaml`), and a
  weekly job runs T1 against `latest`; a diff between pinned-green and latest-red is a filed issue,
  not a silent skip.
- Every skip in the report names the missing credential or spike. A skip is never a pass.

## 7. Spikes (decide before their work item)

| # | Question | Decides |
| --- | --- | --- |
| 1 | Does `claude -p` honour `ANTHROPIC_BASE_URL` + `ANTHROPIC_API_KEY` when a subscription login exists, or must T1 use an empty `CLAUDE_CONFIG_DIR`? | Claude T1 |
| 2 | Codex `[model_providers]`: `wire_api = "chat"` or `"responses"` for a fake server; does hook loading work with `--ignore-user-config` off and a temp `CODEX_HOME`? | Codex T1 |
| 3 | Gemini CLI custom API base env name and whether tool calling works against it | Gemini T1 |
| 4 | Grok Build `XAI_BASE_URL` or equivalent | Grok T1 |
| 5 | Kimi provider `base_url` with an isolated `KIMI_CODE_HOME`; exact shell `tool_name` for the hook matcher | Kimi T1 + dialect correctness |
| 6 | DSH headless mode and the hooks plugin's actual event/decision shape | whether DSH gets a T1 lane or stays "bundle only" |
| 7 | Paperclip `claude_local` adapter `env` passes `ANTHROPIC_BASE_URL` to `claude-agent-acp` | Paperclip T1 vs T2-only |
| 8 | T3: minimal WS sequence to create a project, thread with worktree, and dispatch a prompt; does the ACP subprocess inherit env for a base URL | T3 T1 vs T2-only |
| 9 | Multica daemon env inheritance (`ANTHROPIC_BASE_URL`), `MULTICA_CLAUDE_ARGS` accepts `--plugin-dir` | Multica T1 vs T2-only |
| 10 | smolvm on GitHub-hosted macOS arm64 runners | hosted vs self-hosted T1 |

## 8. What I need from you

**Installs I will do**: `pi` (`@earendil-works/pi-coding-agent`), `herdr` (`brew`), `multica` CLI
(`brew multica-ai/tap/multica`), `t3` (`npx t3@latest`), `paperclipai` (npx, isolated by
`test-drive`), `openhands-sdk` (pip, in a venv), `@deepseek-ai/dsh` (npx).

**Logins/keys, only when T2 for that row matters** (T1 needs none):

| For | Provide |
| --- | --- |
| Codex | ChatGPT quota reset (2026-09-20) or `OPENAI_API_KEY` |
| Gemini | run `gemini` once, Google login, or `GEMINI_API_KEY` |
| OpenCode | `opencode auth login` (Anthropic Pro/Max works), or `ANTHROPIC_API_KEY` |
| Grok | `grok login --device-code`, or `XAI_API_KEY` |
| Kimi | `kimi` then `/login` |
| pi | `pi` then `/login` → Anthropic Claude Pro/Max |
| OpenHands T2 | `ANTHROPIC_API_KEY` (LiteLLM; a Claude Code login does not apply) |
| Multica | already installed; confirm the desktop app is signed in so the daemon registers |
| Conductor | already installed; one workspace created by hand for the local checks |
| Cloud Conductor | not planned: smolvm is not in their cloud workspaces |

## 9. Order of execution

1. fakellm + runner + oracle, proven on Claude Code (spike 1). This is the foundation; everything
   else is a driver.
2. pi dialect and bundle (the only harness missing from boxer).
3. Harness drivers in the order their spikes clear: OpenCode and pi (documented base-URL
   support), then Codex, Gemini, Grok, Kimi.
4. Orchestrators: OpenHands (SDK, fully deterministic), Paperclip (`test-drive`), herdr, Multica,
   T3 (WS spike), Conductor (setup-script lane + checklist).
5. CI tiers and the generated status table.

Estimated size: fakellm ~600 lines, runner + drivers ~1,500 lines Go, pi bundle ~150 lines, docs.
