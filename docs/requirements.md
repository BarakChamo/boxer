# boxer — requirements

A harness-agnostic sandbox layer that runs coding-agent work inside smolvm microVMs, keyed to git
worktrees, enforced by harness hooks rather than by errors.

- Status: Draft for review
- Date: 2026-09-16
- Backend: [smol-machines/smolvm](https://github.com/smol-machines/smolvm) (not CelestoAI/SmolVM)

## Verification status

This document separates what was checked from what was assumed. Every requirement marked
**UNVERIFIED** must be proven before the work it governs begins.

| Fact | Status |
| --- | --- |
| Claude Code hook events, `updatedInput` rewriting, `agent_id` on subagent hooks | Verified against the hooks reference |
| smolvm CLI surface, Smolfile, volumes, network defaults, branching semantics | Verified against the repository README and `AGENTS.md` |
| smolvm has no MCP server and no prescribed agent workflow | Verified: its `AGENTS.md` is a reference manual only |
| Paperclip `ServerAdapterModule` interface and adapter file layout | Verified against its `create-agent-adapter` skill |
| `machine branch` works on macOS Apple Silicon | Verified by running it: 241ms, child isolated from source. (Was: docs say "not on Windows", never confirm Apple Silicon |
| OpenHands Remote Runtime API payloads and endpoint schemas | **UNVERIFIED.** Only endpoint names and `X-API-Key` confirmed |
| T3 Code extension or hook points | **UNVERIFIED.** Documented only as agent-agnostic |
| Codex CLI: `PreToolUse` rewrite via `permissionDecision: "allow"` + `updatedInput`, `SessionStart` `additionalContext`, `session_id`/`agent_id` | Verified against its hooks reference |
| Grok Build: `PreToolUse` can rewrite a tool's input; native worktrees and up to eight parallel subagents | Verified against xAI documentation and launch material |
| OpenCode: `tool.execute.before` can mutate `output.args.command` | Verified against its plugin docs |
| DSH: eight Claude-Code-protocol events, non-zero exit denies; **no rewrite** | Verified against `dsh-plugin-hooks` |
| Gemini CLI hooks: `BeforeTool`/`AfterTool`/`SessionStart`/`SessionEnd`; rewrite via `hookSpecificOutput.tool_input`; deny is top-level `decision`/`reason`; `excludeTools: ["run_shell_command"]` exact-name | Verified against hooks and extension references |
| Kimi Code hooks (Beta): JSON on stdin, exit code controls behavior | Verified; **rewrite support UNVERIFIED** |
| Claude Code plugins bundle `skills/`, `agents/`, `hooks/`, `.mcp.json`, `bin/`, `settings.json` | Verified against the plugin authoring docs |
| Plugin `bin/` is added to the Bash tool's `PATH` while the plugin is enabled | Verified. This *is* the shim mechanism, shipped automatically |
| Plugin `settings.json` `agent` key activates a custom agent as the main thread, applying its **tool restrictions** | Verified. Enables removing raw Bash rather than denying it |
| `claude plugin eval` measures how often the model reaches for a plugin and gets the right result | Verified. Adopted as an acceptance mechanism |
| Codex plugins bundle skills, MCP servers, apps, and (v0.129+) hooks; marketplace since 2026-03 | Verified against launch coverage and release notes |
| Codex plugin manifest `.codex-plugin/plugin.json` with `hooks` key; `PLUGIN_ROOT`/`PLUGIN_DATA` env | Verified against hooks reference |
| Claude Code `WorktreeCreate`/`WorktreeRemove` hooks **replace** built-in worktree creation when configured | Verified against hooks reference; boxer therefore does not register them |
| Claude Code plugin manifest: `agents` must not be a directory string (validator rejects it); `agents/` is auto-discovered; `--plugin-dir` loads hooks and MCP in `-p` mode; MCP tools appear as `mcp__plugin_boxer_boxer__boxer_run` | Verified by `claude plugin validate` and a headless run |
| Live eval, Claude Code, rewrite mode: agent typed `uname -a`, hook rewrote it, result `Linux sb-…`, zero denials | Verified by `evals/harness.sh` |
| Live eval, Claude Code, tool mode: agent read the injected brief and ran `boxer run -c 'uname -a'` unprompted, zero denials | Verified by `evals/harness.sh` |
| T3 Code spawns provider CLIs via ACP with `CLAUDE_CONFIG_DIR` set to a T3-owned directory; Paperclip stages managed `CLAUDE_CONFIG_DIR`/`CODEX_HOME`; user-level plugins are therefore not loaded under either | Verified in source (`ClaudeHome.ts`, `claude-local/src/server/acp.ts`) |
| Codex 0.154: hooks on by default; project hooks need trust or `--dangerously-bypass-hook-trust`; `codex exec` has no `--full-auto`, sandbox off via `--dangerously-bypass-approvals-and-sandbox`; `.codex/hooks.json` SessionStart/SessionEnd fired live | Verified by running it (usage limit stopped the turn) |
| OpenHands SDK `LocalWorkspace.execute_command(command, cwd, timeout) -> CommandResult` | Verified against source |
| Grok Build 1.0.34: `grok plugin validate` accepts the rendered bundle (1 skill dir, hooks, MCP servers); `grok plugin install <path>`; `-p` single-turn; `--permission-mode bypassPermissions` | Verified by running it (no API key for a live turn) |
| `boxer install all` on a repository with existing `.claude/settings.json`: existing keys kept, hooks merged, second run adds nothing; `BOXER_INSIDE=1` makes hooks silent and `run` direct | Verified by `evals/smoke.sh` (44 checks) |
| Gemini CLI 0.60: `gemini extensions install --consent <path>` installs the rendered extension; `extensions list` shows the context file and the `boxer` MCP server | Verified by running it (no login for a live turn) |
| Kimi Code CLI 0.43: hooks are `[[hooks]]` tables (`event`, `matcher`, `command`, `timeout`) in `~/.kimi-code/config.toml`, user level only; exit 0 allow / 2 block, JSON `permissionDecision`; no rewrite documented; MCP in `.kimi-code/mcp.json` (`mcpServers`) | Verified against docs; live turn needs a Kimi login |
| DSH is `@deepseek-ai/dsh` (the npm `dsh` package is an unrelated JS shell); hooks via a plugin reading `.dsh/hooks.json`, block-only (exit 2 or `decision: "block"`) | Verified against repository and plugin READMEs; not run |
| PATH shims must strip their own directory from `PATH`: the smolvm launcher runs `uname -s`, and a shimmed `uname` recursed until the host ran out of processes | Verified the hard way |
| smolvm 1.16.1 on Apple Silicon: exec exit code, stderr, stdin, two-way `--volume`, labels via `ls --json`, `status --json` `state`, exec 33ms, warm start 351ms, cold start with pull 24s | Verified by running it |
| smolvm pulls images inside the guest: a machine with no network can never pull; `--allow-host` implies `--net` | Verified by running it |
| Grok Build plugins and marketplaces bundle skills, hooks, MCP servers, agents, LSP servers | Verified against xAI documentation |
| Gemini CLI extensions bundle MCP servers, context files, commands, hooks, sub-agents, skills, and **excluded tools that disable defaults** | Verified against extension docs |
| OpenCode plugins are TypeScript modules exposing hooks, custom tools, and event handlers | Verified |
| DSH and Kimi Code have plugin ecosystems with marketplaces (dsh-plugin.org; HOL registry) | Verified; component lists partly **UNVERIFIED** |

## 1. Goals

1. Containerization independent of the harness or orchestrator.
2. smolvm as the only backend.
3. Sandbox sharing or isolation selectable by configuration, across worktree, session, and
   subagent granularity.
4. Deterministic, mechanical setup and teardown driven by hooks.
5. Agents pushed into the sandbox by construction, not corrected by error after the fact.
6. Cheap integration with existing harnesses.

## 2. Design principles

### 2.1 Hooks first, errors last

The single most important principle. An enforcement design whose primary mechanism is a failed
command teaches every agent session the same lesson the same way: attempt, fail, read error, retry.
That wastes a turn per session, pollutes context with a self-inflicted error, and trains the model
to expect failure as a normal step.

So the ordering is:

1. **Prevent.** Rewrite the command before it runs, so the sandboxed path is the only path taken and
   the agent never observes a difference.
2. **Inform.** Where rewriting is impossible, state the constraint in injected session context
   before the agent's first action.
3. **Refuse.** Only where neither is available, fail with an agent-readable error.

A harness that supports command rewriting must never reach step 3 in normal operation. Reaching it
is a defect in the integration, not a normal mode.

### 2.2 Two seams, named honestly

**Execution isolation** is where commands run. **File isolation** is where the agent's own read and
write tools reach.

Rewriting shell commands into the VM delivers the first and not the second: a harness's file tools
do not shell out, so they keep touching the host. This is worth having, because it is what makes
builds, tests, installs, and network egress reproducible and contained. It is not a security
boundary against a hostile agent.

`boxer` is specified for execution isolation. A future mode that runs the whole harness inside the
VM is the only way to get file isolation and is out of scope here, noted in §11.

### 2.3 One backend, no abstraction

smolvm is the stated backend. No VMM interface, no driver registry, no plugin surface for
alternative backends. An interface with one implementation is cost without benefit; add it when a
second backend actually exists.

### 2.4 The host keeps the repository

The git worktree lives on the host and is mounted into the guest. Git itself runs on the host by
default, because credentials, signing keys, and worktree metadata live there. This is configurable
but the default must not change casually: forwarding `git` into the guest breaks worktree operations
in ways that are tedious to diagnose.

## 3. Core: the `boxer` CLI

One static binary. No daemon in v1.

### 3.1 Commands

| Command | Behavior |
| --- | --- |
| `boxer up` | Ensure a VM exists for the resolved scope key. Idempotent. Prints the key. |
| `boxer run -- <cmd>` | Execute a command inside the scope's VM, mapping cwd. Exit code passthrough. |
| `boxer down` | Destroy the VM for the resolved scope key. Idempotent. |
| `boxer doctor` | Report resolved config, scope key, VM state, worktree state, enforcement mode. |
| `boxer ls` | List VMs boxer owns, with scope key, worktree, age, and last use. |
| `boxer gc` | Destroy VMs whose worktree is gone or which exceeded the idle timeout. |
| `boxer shim install` | Write the shim directory for harnesses without hooks. |
| `boxer hook <event>` | Hook entry point; reads harness JSON on stdin, writes harness JSON on stdout. |

`boxer hook` is a single dispatcher rather than a script per event, so harness integrations stay
declarative and the logic lives in one place.

### 3.2 Scope identity

A VM is identified by a scope key, derived deterministically:

```
key = "sb-" + sha256(scope_material)[0:12]
```

Where `scope_material` is, per the configured `isolation` value:

| `isolation` | Material | Effect |
| --- | --- | --- |
| `repo` | git common dir | One VM for the repository; all worktrees and agents share it |
| `worktree` | worktree root path | One VM per worktree; agents in that worktree share it |
| `session` | worktree root + harness session id | Concurrent agents in one worktree are isolated |
| `subagent` | worktree root + session id + agent id | Every subagent gets its own VM |

Requirements:

- **R-SCOPE-1.** Key derivation is pure: same inputs, same key, on any machine.
- **R-SCOPE-2.** When the configured isolation needs an identifier the harness does not supply
  (a session id, an agent id), boxer degrades to the next-coarsest scope that is satisfiable and
  records the degradation in `doctor` output. It must not silently produce a random key, which
  would leak a VM per invocation.
- **R-SCOPE-3.** Degradation behavior is configurable: `degrade` (default) or `fail`.
- **R-SCOPE-4.** The key must be recoverable from a running VM's metadata, so `gc` and `ls` work
  without a local index that can drift.

### 3.3 Worktree policy

- **R-WT-1.** `require_worktree` accepts `off`, `warn`, `require`.
- **R-WT-2.** Under `require`, a command issued from a path that is not inside a git worktree is
  refused, and under a hook-capable harness the refusal happens in `PreToolUse` with an explanation,
  not as a shell error.
- **R-WT-3.** The main checkout counts as a worktree unless `require_linked_worktree` is set, which
  demands a linked worktree specifically.
- **R-WT-4.** The worktree root is mounted into the guest at a fixed path (`/workspace` by default),
  and cwd is translated on every `run`: host `<worktree>/a/b` becomes guest `/workspace/a/b`.

### 3.4 Command translation

- **R-CMD-1.** A command in the passthrough list runs on the host unchanged.
- **R-CMD-2.** A command already prefixed with `boxer run` is never wrapped twice.
- **R-CMD-3.** Compound shell input (pipes, `&&`, subshells, heredocs) is wrapped whole and executed
  by a shell inside the guest, never split and partially forwarded.
- **R-CMD-4.** Exit status, stdout, and stderr pass through unmodified. Interleaving order is
  preserved well enough for test runners.
- **R-CMD-5.** stdin is forwarded for interactive and piped use.
- **R-CMD-6.** Signals (`SIGINT`, `SIGTERM`) propagate to the guest process.
- **R-CMD-7.** A TTY is allocated when the host has one, so progress output and colour survive.

**Rewrite decision.** On every shell-tool call the hook binary receives, in `rewrite` mode:

```text
command matches passthrough list ..................... allow unchanged
command already begins with `boxer run` .............. allow unchanged (R-CMD-2)
scope resolves and VM is reachable ................... allow, rewrite to `boxer run -- <command>`
scope resolves, VM absent, create_on permits ......... provision, then rewrite
scope does not resolve, on_missing_id = degrade ...... rewrite against the degraded scope, warn once
otherwise ............................................ block, emit §3.6 error with `fix`
```

The rewrite is the whole original command, wrapped once, never parsed into parts. On harnesses whose
hook contract accepts a rewritten input, the decision is returned as *allow with a replacement
command*; the exact field names come from each harness's manifest (§7.1). On harnesses that accept
only allow or block, the same decision tree runs but the third and fourth outcomes become **block
with the rewritten command in the `fix` line**, and a shim (§5.2) is installed so the block is
rarely reached.

- **R-CMD-8.** The rewrite decision is one function with one test suite, exercised identically by
  every harness manifest. A harness integration may translate its inputs and outputs; it may not
  contain a decision.
### 3.5 Failure policy

- **R-FAIL-1.** `on_sandbox_unavailable` accepts `fail` (default) or `passthrough`.
- **R-FAIL-2.** Under `fail`, a command that cannot be sandboxed does not run on the host. Silent
  host execution is the failure mode this project exists to prevent.
- **R-FAIL-3.** Under `passthrough`, host execution is permitted and each occurrence is logged and
  surfaced in `doctor`.

### 3.6 Error contract

Errors are read by agents, so they are structured and actionable. Every refusal prints, to stderr:

```
boxer: <one-line reason>
  scope:     <key> (<isolation>)
  worktree:  <path or "none">
  cause:     <machine-readable code>
  fix:       <the exact command to run>
```

- **R-ERR-1.** Every error carries a stable `cause` code.
- **R-ERR-2.** Every error carries a `fix` line that is a runnable command, or states plainly that
  no agent-side fix exists.
- **R-ERR-3.** Errors never include host paths outside the worktree, tokens, or environment values.

## 4. Harness survey and capability tiers

The decisive finding of this survey: **Claude Code's hook contract has become the de-facto
standard.** JSON on stdin, exit-code semantics, and matcher syntax recur across harnesses built by
five different vendors, sometimes as an explicit design goal. DSH's hook plugin states it outright:
"The protocol is Claude Code's, so your existing hook scripts run unchanged." Gemini CLI's hooks are
documented as mirroring the same contract.

That collapses what looked like N integrations into **one hook binary plus a thin manifest per
harness**.

### 4.1 Survey

| Harness | Hook events | Can rewrite a command? | Notes |
| --- | --- | --- | --- |
| Claude Code | ~25, incl. `WorktreeCreate`/`WorktreeRemove`, `SubagentStart`/`Stop` | **Yes**, `updatedInput` | Richest surface; worktree events are unique |
| Codex CLI | `PreToolUse`, `PostToolUse`, `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `SubagentStart`/`Stop`, `Pre`/`PostCompact`, `PermissionRequest`, `Interrupt`, `Stop` | **Yes**, `permissionDecision: "allow"` + `updatedInput` | Same shape as Claude Code; `hooks.json` or inline in `config.toml` |
| Grok Build | `PreToolUse`, `PostToolUse`, `SessionStart`, peers | **Yes** | Shell *and* HTTP hook runners; native worktrees, up to 8 parallel subagents each in its own worktree |
| OpenCode | `tool.execute.before`/`after`, `permission.ask`, `session.*`, `shell.env`, many more | **Yes**, mutate `output.args.command` | TypeScript plugin API rather than stdin JSON |
| Gemini CLI | Hooks v1, command and plugin hooks | **Unverified** | Explicitly mirrors the Claude Code contract; config at project/user/system/extension scope |
| Kimi Code | Hooks (Beta) | **Unverified** | JSON on stdin, exit code controls behavior |
| DSH | 8 events via `dsh-plugin-hooks` | **No.** Block only | Non-zero exit denies; env vars carry session id |
| OpenHands | No hook system; a Runtime abstraction instead | n/a | Integrate as a runtime, not a hook |
| Paperclip | No hooks; adapter interface | n/a | Integrate as a wrapping adapter |

Two cross-cutting standards sit beside these rather than inside them:

- **MCP** is near-universal but opt-in. An agent *may* call an MCP tool; nothing compels it. Useful
  for discovery and status, useless for enforcement.
- **Agent Client Protocol** (Zed and JetBrains, 50+ registered agents) standardizes how an *editor*
  drives an *agent*. It is a different seam, but a useful one: an ACP agent is a process, and a
  process can be launched already inside the sandbox.

### 4.2 Capability tiers

Enforcement strategy is selected per harness by capability, not by name.

| Tier | Capability | Strategy | Agent sees an error? |
| --- | --- | --- | --- |
| **1** | Pre-tool hook that can rewrite input | Rewrite the command into the guest | **Never** |
| **2** | Hooks that can block and inject context, but not rewrite | Inject the constraint at session start so the agent forms the sandboxed command itself; block only on violation | Only if it ignores the injected context |
| **3** | No hooks; runs a shell | PATH shims | Only on failure |
| **4** | Own execution abstraction (OpenHands) or process launcher (Paperclip, ACP) | Sandbox at that layer | n/a |

- **R-TIER-1.** Tier is detected, not assumed. `boxer doctor` reports the detected tier per
  installed harness and the mechanism it will use.
- **R-TIER-2.** Tier 2 is where §2.1's second step earns its place. Injected context must state the
  exact command form the agent should produce, because here the agent is the one constructing it.
- **R-TIER-3.** Tier 2 and Tier 3 stack: context injection prevents the first mistake, shims catch
  what the harness spawns outside its tool loop.
- **R-TIER-4.** A harness whose rewrite support is unverified is treated as Tier 2 until proven
  otherwise. Downgrading is safe; assuming rewrite and being wrong reintroduces the error-first
  behavior this project exists to prevent.

## 5. Enforcement

### 5.1 Execution modes

Rewriting is **one strategy, not the mechanism**. Three modes, selected by configuration:

| Mode | How a command reaches the guest | Agent's model of what happened |
| --- | --- | --- |
| `rewrite` | A pre-tool hook silently replaces the command | Believes it ran the command directly |
| `tool` | The agent calls a boxer tool or command explicitly | Knows it ran inside a sandbox |
| `off` | Nothing is sandboxed; coverage is recorded | Accurate, and unsandboxed |

- **R-MODE-1.** `mode` accepts `rewrite`, `tool`, `off`, defaulting to `rewrite`.
- **R-MODE-2.** Both `rewrite` and `tool` satisfy §2.1: neither makes the agent fail first. `rewrite`
  prevents the wrong command; `tool` prevents the wrong *intent* by stating the correct form before
  the first action.

**Why `tool` mode is not merely a preference.** Silent rewriting hides the execution context, so an
agent debugging a failure reasons about the wrong environment. A test that fails because the guest's
network policy blocked a registry looks, to an agent that believes it ran on the host, like a broken
test. In `tool` mode the agent knows it is in a sandbox and can reach the right conclusion. Teams
running long autonomous sessions should prefer `tool` for this reason; teams wanting zero friction
should prefer `rewrite`.

- **R-MODE-3.** Mode is selectable per harness, because tier and mode interact: a Tier 2 harness
  cannot offer `rewrite` and must use `tool`.
- **R-MODE-4.** In `rewrite` mode, `additionalContext` still discloses that commands run in a guest.
  Silent must not mean secret; a human reading the transcript needs to know.

### 5.2 The enforcement gap in `tool` mode

A tool the agent *may* call is not a tool the agent *must* call. With both a shell tool and a boxer
tool available, some calls will take the shell. Three ways to close it, in descending preference:

1. **Remove the shell tool.** Ship a tool restriction that omits raw shell access, so the agent never
   forms the intent. At least two harnesses support this natively: Claude Code through a plugin
   agent's tool restrictions, Gemini CLI through an extension's excluded-tools list. This is
   error-free by construction: nothing is denied because nothing was offered.
2. **Shim the PATH.** Intercepted binaries resolve to shims that forward into the guest. The agent's
   shell call succeeds and lands in the right place.
3. **Deny at the hook.** Last resort, and the only one that produces an error.

- **R-GAP-1.** `tool` mode must be deployed with mechanism 1 or 2. `tool` mode alone is advisory and
  must be reported as such by `doctor`.
- **R-GAP-2.** Mechanism 3 is never a default.

### 5.3 Mechanisms by tier

| Tier | Capability | `rewrite` mode | `tool` mode |
| --- | --- | --- | --- |
| 1 | Pre-tool hook can rewrite | Hook rewrite | Tool plus restricted agent or shims |
| 2 | Hooks block and inject only | Unavailable | Tool plus injected instruction plus shims |
| 3 | No hooks | Unavailable | Shims only |
| 4 | Own execution layer | n/a | Sandbox at that layer |

- **R-ENF-1.** `on_sandbox_unavailable` accepts `fail` (default) or `passthrough`.
- **R-ENF-2.** Under `fail`, a command that cannot be sandboxed does not run on the host. Silent host
  execution is the failure mode this project exists to prevent.
- **R-ENF-3.** Under `passthrough`, host execution is permitted, logged, and surfaced in `doctor`.
- **R-ENF-4.** Shims detect the guest by a marker environment variable set at VM creation and
  short-circuit to the real binary there.

### 5.4 Intercepted commands

- **R-INT-1.** `intercept` is an explicit list of binary names, defaulting to package managers,
  language runtimes, test runners, and build tools.
- **R-INT-2.** `passthrough` is an explicit list that always runs on the host, defaulting to `git`,
  `gh`, `ssh`, and `boxer` itself.
- **R-INT-3.** Neither list is inferred. An unlisted command runs on the host and is counted in
  `doctor` coverage output, so gaps are discoverable rather than silent.

## 6. Configuration

One file, `boxer.toml`, resolved from the worktree root then the repository root then the user
config directory, with later files overridden by earlier. Every key is also settable by environment
variable for harnesses that only offer environment control.

```toml
# Identity and lifecycle
isolation          = "worktree"   # repo | worktree | session | subagent
on_missing_id      = "degrade"    # degrade | fail
require_worktree   = "warn"       # off | warn | require
require_linked_worktree = false

# Lifecycle
create_on          = ["session_start", "run"]   # run = provision lazily on first sandboxed command
destroy_on         = []                    # session_end | subagent_stop | never; gc reaps orphaned worktrees
idle_timeout       = "2h"                  # gc reaps beyond this; "never" to disable
reuse_existing     = true

# Enforcement
mode               = "rewrite"    # rewrite | tool | off  (per-harness override: [harness.<name>].mode)
enforcement        = "both"       # hook | shim | both | audit
on_sandbox_unavailable = "fail"   # fail | passthrough
intercept          = ["npm", "pnpm", "yarn", "bun", "node", "python", "pytest", "cargo", "go", "make"]
passthrough        = ["git", "gh", "ssh", "boxer"]

# Guest
image              = ""           # OCI ref; default detected from lockfile, else boxer base
smolfile           = ""           # path; wins over image
setup              = []           # run once in guest after first boot, cwd = mount_at
mount_at           = "/workspace"
cpus               = 4
memory             = "4G"
env_passthrough    = ["CI", "NODE_ENV"]   # names only; never values in config
secrets            = []                   # host-resolved references, per smolvm's secret model

# Network: off by default, matching smolvm
[network]
mode               = "allowlist"  # off | allowlist | on
allow_hosts        = ["registry.npmjs.org", "github.com"]
ports              = []

# Per-harness overrides; any top-level key may appear here
[harness.claude-code]
mode               = "rewrite"
[harness.gemini-cli]
mode               = "tool"       # closes the gap by excluding the shell tool
[harness.dsh]
mode               = "tool"       # block-only hooks; shims installed by `boxer shim install`

# Branching (see §10; inert until enabled)
[branch]
enabled            = false
base               = ""
warm               = []
```

- **R-CFG-1.** `boxer doctor` prints the fully resolved configuration and the file each value came
  from. Configuration that cannot be explained is configuration that gets cargo-culted.
- **R-CFG-2.** Unknown keys are an error, not a warning, so a typo cannot silently disable
  enforcement.
- **R-GUEST-1.** With neither `image` nor `smolfile` set, the image is chosen from the worktree's
  lockfile (`bun.lock`, `package-lock.json`, `pnpm-lock.yaml`, `uv.lock`, `requirements.txt`,
  `Cargo.lock`, `go.sum`), pinned from `.tool-versions`, `.nvmrc`, or `engines` when present, else
  the current LTS. With no lockfile, the boxer base image: Debian slim with git, curl,
  build-essential, and CA certificates. `doctor` prints the chosen image and the reason.
- **R-GUEST-2.** `setup` runs once per VM creation, never per session; completion is a marker file in
  the guest overlay, so boxer keeps no host-side state. A failing step deletes the VM, so `run` then
  refuses with `fix: boxer up`. boxer never auto-runs a dependency install; that is repository
  policy and belongs in the committed `boxer.toml`.
- **R-GUEST-3.** Image pulls happen inside the guest, so the default network mode is `allowlist`
  with the registry hosts for the chosen image pre-allowed. `network.mode = "off"` is honoured, and
  `doctor` warns that it only works for an already-cached image.
- **R-GUEST-4.** smolvm pulls a registry image again for every machine, and that pull is the slow
  and flaky step (quota, stalled blobs, 20 to 75 s). boxer therefore packs each image once per host
  (`smolvm pack create`, stored under the state directory keyed by the image name) and creates
  machines `--from` the pack: the first VM for an image pays the pull, every later VM boots in
  under a second. A pack failure falls back to a direct pull and says so. Packs are not pruned yet.
  `BOXER_PACKS` overrides the pack directory (the eval shares one across its isolated state dirs).
- **R-CFG-3.** Secrets are references resolved on the host at run time, matching smolvm's model,
  which stores no secret material. Values never appear in the config file or in VM metadata.

## 7. Integration: hook-based harnesses

Because the hook contract converged (§4.1), this is **one binary and a bundle per harness**, not one
integration per vendor.

Every hook the integration installs serves one of three universal purposes. Event names differ by
harness; the purposes do not.

| Purpose | What it does | Typical events |
| --- | --- | --- |
| **Provision** | Ensure the VM for the resolved scope exists and is warm | session start, worktree create, subagent start |
| **Instruct** | Tell the agent, before its first action, how execution works here | session start context injection |
| **Reclaim** | Destroy VMs whose scope ended; release locks | session end, worktree remove, subagent stop |

- **R-PURPOSE-1.** A harness integration is complete when all three purposes are wired, or when a
  purpose is explicitly recorded as unavailable on that harness with its fallback named.
- **R-PURPOSE-2.** *Instruct* is mandatory on every harness, including Tier 1. Even when commands are
  rewritten silently, the agent is told that execution happens in a guest (R-MODE-4).

### 7.1 The shared hook binary

`boxer hook <event>` reads the harness's JSON from stdin and writes the harness's JSON to stdout.

- **R-HK-1.** The binary implements the Claude Code contract as its native dialect: common fields
  (`session_id`, `cwd`, `hook_event_name`, `tool_name`, `tool_input`), `hookSpecificOutput` for
  results, exit 0 for success, exit 2 to block with stderr as the reason.
- **R-HK-2.** Harness dialects are handled by a thin adapter selected by `--harness`, defaulting to
  autodetect from the payload. Only fields that genuinely differ are translated. OpenCode, whose
  plugin API is TypeScript rather than stdin JSON, gets a shim plugin that marshals to and from the
  same binary, so the decision logic is never duplicated.
- **R-HK-3.** Unknown events exit 0 with no output. A harness adding events must never break boxer.
- **R-HK-4.** The binary must be fast enough to sit in the tool path. Cold start under 50ms, and it
  never blocks on VM creation during `PreToolUse` beyond a configured readiness timeout.

### 7.2 Event map

Events are named in the Claude Code dialect; the adapter maps equivalents.

| Event | Purpose | Tier 1 output | Tier 2 output |
| --- | --- | --- | --- |
| `SessionStart` | Ensure the VM for session-or-coarser scope | `additionalContext` stating the sandbox exists and no action is needed | `additionalContext` stating the exact command form to use |
| `PreToolUse` (Bash) | Put the command in the guest | `updatedInput.command` rewritten | allow if already correct; block with guidance if not |
| `SubagentStart` | Per-subagent VM when `isolation = "subagent"` | none | none |
| `SubagentStop` | Destroy it per `destroy_on` | none | none |
| `WorktreeCreate` | VM for a new worktree | none | none |
| `WorktreeRemove` | Destroy that worktree's VMs | none | none |
| `SessionEnd` | Destroy per `destroy_on`; release locks | none | none |

- **R-HK-5.** `WorktreeCreate` and `WorktreeRemove` exist only in Claude Code today. Elsewhere,
  worktree lifecycle falls back to a git `post-checkout` hook plus `boxer gc`.
- **R-HK-6.** Subagent isolation requires `agent_id`, confirmed present in Claude Code and Codex.
  Where absent, §3.2's degradation rule applies.
- **R-HK-7.** Rewrites are idempotent: a command already targeting the guest is never wrapped twice,
  which matters under parallel tool calls and retries.
- **R-HK-8.** `PreToolUse` must not use exit 2 for the ordinary sandboxed path on Tier 1. Blocking
  is reserved for policy refusals such as a violated worktree requirement.

### 7.3 Per-harness notes

- **Claude Code.** Richest surface and the only one with worktree events. Ships as a plugin
  providing hooks plus `doctor`, installable without hand-editing settings.
- **Codex CLI.** Same contract; installed through `hooks.json` or an inline `[hooks]` table. Note
  that project-local `config.toml` ignores some user-level keys, so installation writes to the user
  config where the documentation requires it.
- **Grok Build.** Rewrite-capable, and offers an HTTP hook runner as well as shell. It has **native
  worktrees and up to eight parallel subagents, each in its own worktree**, which makes it the
  strongest argument for `isolation = "worktree"`: boxer keys off the worktree Grok Build already
  created rather than inventing its own.
- **OpenCode.** TypeScript plugin mutating `output.args.command`, marshalled through the shared
  binary per R-HK-2.
- **Gemini CLI, Kimi Code.** Treated as Tier 2 until rewrite support is verified (R-TIER-4).
- **DSH.** Tier 2 permanently: its hooks block but cannot rewrite. Context injection plus shims.

### 7.4 Agent Client Protocol

- **R-ACP-1.** For any ACP-registered agent, boxer offers a launcher that starts the agent process
  with the shim directory on `PATH` and the guest marker set. This covers 50+ agents with no
  per-agent code and needs no hook support at all.
- **R-ACP-2.** This is Tier 3 coverage, not Tier 1: it contains what the agent shells out to, and
  does not rewrite its tool calls.

### 7.5 Packaging

Every surveyed harness has a first-class extension bundle. boxer ships one bundle per harness, each
thin, each wrapping the same binary and the same content.

**The bundle is defined by four components, not by any harness's format:**

| Component | Purpose | Authored |
| --- | --- | --- |
| **Instruction** | Teaches the agent what the sandbox is and how to run commands in it | Once, rendered per harness |
| **Lifecycle hooks** | Provision, instruct, reclaim (§7 table) | Once; the shared binary |
| **Run tool** | An explicit `boxer_run` the agent can call in `tool` mode | Once; one MCP server |
| **Gap closer** | Removes or redirects the raw shell path (§5.2) | Per harness, by capability |

How each harness's bundle format carries those four:

| Harness | Bundle format | Instruction | Hooks | Run tool | Gap closer |
| --- | --- | --- | --- | --- | --- |
| Claude Code | plugin | `skills/` | `hooks/hooks.json` | `.mcp.json` | `bin/` on Bash `PATH`; `settings.json` agent tool restrictions |
| Codex CLI | plugin, marketplace | skills | bundled hooks (v0.129+) | MCP server | hook rewrite; tool restriction **UNVERIFIED** |
| Grok Build | plugin, marketplace | skills | hooks | MCP server | hook rewrite; tool restriction **UNVERIFIED** |
| Gemini CLI | extension | `GEMINI.md` context + skills | hooks | MCP server | **excluded tools** |
| OpenCode | TypeScript plugin | `AGENTS.md` section | `tool.execute.before` | plugin custom tool | argument mutation |
| DSH | plugin (`.dsh/hooks.json` via hooks plugin) | `.agents/skills` | `dsh-plugin-hooks` | MCP server | none in-harness; external `PATH` shims |
| Kimi Code | user `config.toml` `[[hooks]]` + project `.kimi-code/mcp.json` | `.agents/skills` | hooks, block-only | MCP server | none in-harness; external `PATH` shims |

- **R-PKG-1.** A harness bundle is a *projection* of the four components into that harness's format.
  Adding a harness means writing a bundle manifest and, where the harness's hook dialect differs, a
  translation table for the shared binary. It never means new decision logic.
- **R-PKG-2.** The instruction component is one source document rendered into each harness's
  instruction format (skill, context file, `AGENTS.md` section). Its content states: execution runs
  in a microVM keyed to the worktree; which commands are intercepted; where the worktree is mounted;
  and, in `tool` mode, the exact tool or command form to use.
- **R-PKG-3.** The run tool is one MCP server exposing `boxer_run`, `boxer_status`, and nothing else
  in v1. Every bundle registers the same server.
- **R-PKG-4.** The gap closer is chosen per harness from what it supports, in this order: remove the
  shell tool, inject `PATH`, rewrite at the hook, deny at the hook. `doctor` reports which one is in
  force and why the stronger ones were unavailable.
- **R-PKG-5.** Bundles are generated from one source tree by a `boxer package <harness>` command, so
  a change to the instruction text or the hook binary reaches every harness in one release.
- **R-PKG-7.** `boxer install <harness>` writes the same four components into the repository's
  own harness configuration (`.claude/settings.json` + `.mcp.json`, `.codex/hooks.json`,
  `.gemini/settings.json`, `.opencode/plugins/`, `.grok/settings.json`), merging with what is there
  and changing nothing on a second run. This is the layer orchestrators load when they give the
  harness a private config directory (see `docs/orchestrators.md`).
- **R-PKG-8.** Two installed layers must not fight: an already-wrapped command is allowed as is,
  provisioning is idempotent, and every guest process carries `BOXER_INSIDE=1`, on which boxer's
  hooks go silent and `boxer run` executes directly.
- **R-PKG-6.** Where a harness offers a plugin evaluation mechanism, such as `claude plugin eval`,
  the bundle ships an evaluation set that measures whether the agent uses the sandbox correctly on
  standard tasks. This is the empirical form of §12's zero-error criterion.

## 7.6 Integration model, revised 2026-09-17

After running every harness end to end, the integration surface is organised by **category**, not
by harness. The evidence: MCP is answered by all eight harnesses; the Agent Skills standard is
adopted by all eight; the hook protocol converged on one JSON wire for six (Gemini differs by three
strings, OpenCode and pi expose a TypeScript plugin API instead); plugin *packaging* did not
converge (Codex installs only from marketplaces and does not run Claude plugins; Grok reads both).

| Level | Mechanism | Harnesses | boxer code |
| --- | --- | --- | --- |
| **0 Universal** | MCP server + Agent Skill; `mode = "tool"` | all 8 | none per harness |
| **1 Converged hooks** | one `hooks.json`, one binary; provision, brief, reclaim, optional rewrite or deny | Claude, Codex, Grok, Kimi, DSH; Gemini via aliases | one dialect table |
| **2 Plugin-API shims** | 40-line TS files forwarding to the binary | OpenCode, pi | two templates, optional |

- **R-LVL-1.** Level 0 is the default integration and `tool` the default mode. A repository that
  installs nothing but the MCP server and the skill is fully supported on every harness.
- **R-LVL-2.** Command rewrite is opt-in (`mode = "rewrite"`), available where verified: Claude
  Code, Codex, Gemini CLI, OpenCode, pi. Kimi ignores `updatedInput` and its shell ignores PATH
  shims (verified 0.43.1); DSH is block-only. Both are Level 0 only, and their hook bundles are
  dropped.
- **R-LVL-3.** Gap closure in tool mode prefers the harness's native shell-tool removal (Claude
  `disallowedTools`, Gemini `excludeTools`, Kimi `tools.disabled`, OpenCode `permission.bash`, Grok
  `--disallowedTools`); the hook deny is the fallback for Codex and pi.
- **R-LVL-4.** The run tool is named like the shell tools agents already use, `bash` (alias
  `shell`), with the signature `{command, description?, timeout?, cwd?}`; `boxer_status` remains.
- **R-LVL-5.** The worktree is mounted in the guest **at its host path** by default
  (`mount_at = "<root>"`), so every path an agent types or reads is valid on both sides and no
  translation exists to get wrong. `/workspace` stays available as an opt-in.
- **R-LVL-6.** The bundle is an **Agent Plugins 1.0.0** package (agent-plugins.org, published
  2026-08-06; TSC Amazon, Cursor, Microsoft, OpenAI, Vercel; Google core maintainer): `plugin.json`,
  `skills/boxer/SKILL.md`, `mcp.json` with `mcpServers`. The spec defines exactly two portable
  component types, skills and MCP servers, which is Level 0; hooks are client-specific by the spec's
  own decision and live in the reverse-domain namespaces it reserves (`com.anthropic.claude-code/`,
  `com.google.gemini-cli/`, …), which other clients must ignore. Until each loader reads the
  standard layout, the same directory also carries the native manifests it reads today
  (`.claude-plugin/plugin.json` + `.mcp.json`, `.codex-plugin`, `.grok-plugin`,
  `gemini-extension.json`); one directory was verified to validate in Claude and Grok and to install
  in Gemini. Claude's validator rejects Gemini's `BeforeTool` key in a shared `hooks.json`, so the
  Gemini group lives in its namespace folder. Codex installs through a local marketplace wrapper
  (`codex plugin marketplace add`).
- **R-LVL-6a.** `plugin.json` and `mcp.json` are validated in `go test` against the published
  schemas (`agent-plugins.org/schemas/1.0.0/`, vendored), so the bundle is a conforming plugin for
  every client at once, deterministically.
- **R-LVL-7.** Grok's plugin hooks are discovered as zero in `grok -p` even when the plugin loads
  (`hooks: discovery complete total_hooks=0`); Grok is Level 0 until that is understood.

### Two integrations, one kernel (decided 2026-09-17)

Every product that isolates an agent runs the agent program inside the container (Docker Sandboxes,
Anthropic's devcontainer, Gemini's Docker mode, cloud sessions). No harness exposes a
bring-your-own-sandbox seam; four of eight wrap their own shell internally, and a hypervisor cannot
start inside those wrappers (measured: `boxer run` dies under Claude's strict Bash sandbox and
Codex's `workspace-write`). boxer therefore offers both placements, selected by one key:

```toml
integration = "outside"   # default: harness on the host, boxer sandboxes its commands (everything above)
integration = "inside"    # harness runs in the VM; nothing to hook, rewrite, or deny
```

- **R-INT-1.** `outside` is unchanged: Agent Plugins package, optional hooks, `mode`, `install`,
  signals. `inside` adds two entry points and removes nothing.
- **R-INT-2.** `boxer shell <harness> [args]` runs the harness interactively inside the worktree's
  VM with a TTY. `boxer acp <harness>` runs the harness's ACP server inside the VM and pipes stdio,
  so any ACP client (T3 Code, Paperclip, Zed, JetBrains) points its agent command at boxer.
- **R-INT-3.** Inside mode mounts the worktree at its host path and each harness's config directory
  at its host path, read-write, and sets `HOME` to the host home, so paths, sessions, and logins are
  the same on both sides. Claude Code's macOS Keychain login does not travel; `CLAUDE_CODE_OAUTH_TOKEN`
  from `claude setup-token` is passed through.
- **R-INT-4.** The harness is installed once per VM by a data table (npm package or binary URL),
  recorded by a marker file; the default inside image is `node:24-bookworm-slim` from the Google
  mirror; the network allowlist gains the npm registry, the Debian mirror (Codex's ACP adapter
  needs `libssl3`), and the harness's model API hosts. npm inside the guest runs with long fetch
  timeouts and the install line is retried once: idle timeouts against the npm registry were
  observed in a third of eval runs.
- **R-INT-4a.** The VM is the sandbox, so the harness's own nested sandbox is turned off by the
  table, not by the user: Codex gets `sandbox_mode = "danger-full-access"` (`-c` in shell mode;
  `CODEX_CONFIG` plus `INITIAL_AGENT_MODE=agent-full-access` for its ACP server, whose agent mode
  otherwise re-enables the workspace-write sandbox) because its bwrap sandbox cannot start on the
  mounted worktree inside the guest; Claude Code gets `IS_SANDBOX=1` so it runs as root. Approval
  policies are left as the user configured them.
- **R-INT-5.** `boxer shim install --harness <name>` writes a PATH shim named after the harness
  binary that execs `boxer shell <name>`, so an orchestrator that spawns `claude` lands inside.
- **R-INT-6.** `BOXER_INSIDE=1` is set in the guest, so outside-mode hooks committed in a
  repository are silent there; the two modes never wrap the same command twice.
- **R-INT-7.** Eval: one inside cell per harness — harness in the VM against the fake model,
  `uname -a` → `Linux`, no host canary — plus one ACP cell driving `boxer acp` with a minimal client.

### Signals, not hooks: the capability model (supersedes the two revisions above)

Scope is derived from the cwd of a command at the moment it runs; provisioning there is lazy and
idempotent. That is the only always-correct rule, because worktrees appear before the session
(orchestrators), at session start (`codex --worktree`), mid-session (`EnterWorktree`, `git worktree
add` from the agent's shell), or never. Every other input is a **signal** that lets boxer act
earlier, more precisely, or clean up sooner. Signals add speed and precision; they never carry
correctness, and none is removed from the product.

| Signal | Tells us | Breadth | Role |
| --- | --- | --- | --- |
| `boxer run` / MCP tool call | a command needs a sandbox here, now | universal | the correctness path |
| git `post-checkout` (flag 1) | a worktree just appeared at this path | universal, repo opt-in | warm the new scope before an agent touches it |
| MCP `initialize` | a session started in this cwd | universal | warm the current scope; session identity of last resort |
| MCP EOF | the session process ended | universal | reclaim per `destroy_on` |
| `SessionStart` hook | session start, `session_id`, `source` | Claude, Codex, Grok, Kimi, DSH, Gemini | brief injection, real session id |
| `SubagentStart/Stop` | an agent id is born or dies | Claude, Codex, Grok | the only source of `subagent` isolation |
| `PreToolUse` rewrite | a shell command is about to run | Claude, Codex, Gemini, OpenCode, pi | transparent interception, opt-in |
| `SessionEnd` | the harness says it is done | six harnesses | earlier reclaim than EOF |
| `WorktreeCreate` (Claude) | replaces creation | Claude | only when boxer manages worktrees |

- **R-SIG-1.** Every signal is auto-detected per harness, opt-in in configuration, and idempotent
  at the kernel: a git hook, a harness hook, an MCP start, and a first run firing for one scope
  produce one VM (per-scope lock; `Ensure` is a no-op on a running VM).
- **R-SIG-2.** `doctor` reports, per harness, which signals are live and therefore the effective
  isolation and provisioning timing the user actually gets. The brief states the same.
- **R-SIG-3.** Isolation is a ceiling: effective isolation = min(configured, signals present).
  `subagent` needs `SubagentStart`; `session` needs a session id from a hook, else the MCP server
  instance; else `worktree`. Degradation is reported, never silent.
- **R-SIG-4.** Reclaim is layered: `SessionEnd` where available → MCP EOF → `gc` (worktree gone,
  idle). `destroy_on` defaults to empty because reuse across sessions is the speed feature.
- **R-SIG-5.** boxer never creates or deletes worktrees unless `worktree.manage = true`. Off: boxer
  reacts to worktrees others create, at the first signal, and warms the cwd scope at session start
  (`warm_on_session_start`, default on). On: boxer creates the worktree at the earliest session
  signal, only when cwd is the main checkout and no orchestrator owns worktrees.
- **R-SIG-6.** Block-only hook families (Kimi, DSH) keep their `SessionStart`/`SessionEnd` hooks:
  precision is a signal even where rewrite is not available. Rewrite remains opt-in
  (`mode = "rewrite"`) where verified.
- **R-SIG-7.** The core with no signals (Agent Plugins package, lazy provisioning, `install`
  shell-disable settings, git hook, `gc`, `BOXER_INSIDE`) is correct on every harness. Signals are
  measured by the timing matrix in the eval plan, not assumed.

### Inside mode

- **R-IN-1.** `boxer shell <harness> [args]` runs the harness itself inside the guest with the
  worktree mounted at its host path, credentials mounted read-only (Codex `auth.json`, Gemini
  `oauth_creds.json`, Kimi config) or passed as a token (`CLAUDE_CODE_OAUTH_TOKEN`), and the model
  API hosts allowed. No hooks, no rewrite, no denials: the harness's own shell is already in the
  sandbox.
- **R-IN-2.** `boxer shim install --harness` writes PATH shims named after harness binaries, so an
  orchestrator that spawns `claude` by name lands inside the guest. A harness shim is not subject
  to the compound-command weakness of command shims.
- **R-IN-3.** Spike before build: guest-to-host reachability for the fake model (inside-mode T1),
  and which harness credential files survive a read-only mount.

### Git worktree hooks

`post-checkout` fires on `git worktree add` with its third argument `1`, in the new worktree
(verified). A committed hook provisions before any harness opens the worktree, for every
orchestrator that creates worktrees.

- **R-GIT-1.** `boxer install git` writes `.githooks/post-checkout` calling `boxer up --detach` and
  prints the `core.hooksPath` line; opt-in.
- **R-GIT-2.** Provisioning is idempotent and serialised by a per-scope lock, so a git hook, a
  harness hook, and a first `run` arriving together create one VM (tested with four concurrent
  callers).
- **R-GIT-3.** No git hook fires on `worktree remove`; `gc` reaps by worktree absence and idle time.

### Environment specification

smolvm speaks Smolfile only; Docker Compose support is an open request (smolvm #1307) and there is
no devcontainer support.

- **R-ENV-1.** boxer reads a `devcontainer.json` subset when present: `image`, `postCreateCommand`
  (→ `setup`), `forwardPorts` (→ `network.ports`), `remoteEnv` (→ `env_passthrough` names).
  `boxer.toml` keys override it; `doctor` prints which file each value came from.
- **R-ENV-2.** Smolfile passthrough stays as the escape hatch. Dockerfile builds and Compose are out
  of scope until smolvm or a Docker backend supports them; Compose can run inside the guest via
  smolvm's docker-in-machine.

### Backends

`vm.Client` is already the seam: list, status, create, start, exec, stop, delete, branch, with
labels as the only state.

- **R-BE-1.** The seam becomes a `Backend` interface selected by `backend = "smolvm"`; branching is
  a capability flag, not an assumption.
- **R-BE-2.** Estimates, not commitments: Docker backend one to two days (`docker run -v` at the
  host path, labels, `exec`; weaker isolation, identical agent experience); Firecracker one to two
  weeks and Linux-only (OCI to rootfs, jailer, tap networking, no branching).

## 8. Integration: OpenHands

OpenHands executes agent actions against a `Workspace`; `LocalWorkspace` runs commands with
`subprocess`. boxer integrates at that seam, not at the Remote Runtime API.

- **R-OH-1.** `adapters/openhands/boxer_workspace.py` provides `BoxerWorkspace(LocalWorkspace)`
  whose `execute_command` runs `boxer run -c <command>` from the requested `cwd`, returning a
  `CommandResult` with exit code, stdout, stderr, and timeout state.
- **R-OH-2.** File operations stay on the host; the worktree is the workspace, mounted into the
  guest as for every other harness.
- **R-OH-3.** The adapter is for the SDK and the Process sandbox. Docker and Remote sandboxes are
  not wrapped; nesting boxer inside another isolation layer is a non-goal (§11).
- **R-OH-4.** The adapter carries a test that runs without OpenHands installed.

## 9. Integration: Paperclip

Paperclip's local adapters spawn the real harness CLIs over ACP with a managed configuration
directory, so the project layer is the integration.

- **R-PC-1.** `boxer install` in the repository plus `boxer` on `PATH` where Paperclip runs is the
  complete integration for `claude-local`, `codex-local`, `gemini-local`, `opencode-local`, and
  `grok-local`. No boxer adapter package is required.
- **R-PC-2.** Paperclip's per-heartbeat worktree maps to one VM under `isolation = "worktree"`;
  `boxer gc` reaps VMs whose worktree Paperclip removed.
- **R-PC-3.** SSH and remote-sandbox execution targets are out of scope: the worktree is synced to
  another machine, which needs its own `boxer` and smolvm, or already has its own isolation.

## 10. Branching (later)

Deferred, and gated on one fact.

- **R-BR-0.** Confirm `machine branch` works on macOS Apple Silicon before any of this is planned.
  The documentation lists branching as unavailable on Windows and is silent on Apple Silicon.
  **UNVERIFIED.**
- **R-BR-1.** A base VM is warmed once per repository: dependencies installed, caches populated,
  then marked branchable.
- **R-BR-2.** New scopes fork from the base rather than cold-booting and reinstalling. A branch is a
  live fork inheriting the source's processes, memory, and disk.
- **R-BR-3.** The base is invalidated by a content hash of the dependency manifests and lockfiles;
  a stale base is rebuilt rather than silently reused.
- **R-BR-4.** Branching is an optimization only. Every operation must remain correct with
  `branch.enabled = false`, and the test suite runs both ways.
- **R-BR-5.** Measure before adopting: record cold boot, branch, and warm-command timings on the
  target hardware, and keep branching off if the gain does not clear the added invalidation
  complexity.

## 11. Non-goals

- A VMM abstraction layer, per §2.3.
- File-level isolation of the agent's own read and write tools, per §2.2. Running the whole harness
  inside the guest is the only way to get it and is a separate project.
- A hosted or multi-tenant control plane.
- Replacing devcontainers, or consuming `devcontainer.json`.
- Windows support in v1, since branching is unavailable there.

## 12. Acceptance

The work is done when all of these hold:

1. On **every Tier 1 harness** (Claude Code, Codex, Grok Build, OpenCode), a session in a linked
   worktree runs its whole task with every intercepted command executing in the guest, and **zero
   sandbox-related errors in the transcript**. This is the project's headline criterion and is
   measured per harness, not once.
2. On **every Tier 2 harness**, the injected session context is sufficient that a standard task
   produces no blocked command. A block in a routine session is a defect in the injected wording,
   not acceptable behavior.
3. One hook binary serves every hook-based harness. A new harness on the same contract is onboarded
   by adding a manifest, with **no new decision logic**.
   The bundle for that harness is produced by `boxer package`, and its instruction text is the same
   source as every other harness's.
4. Two concurrent sessions in the same worktree share one VM under `isolation = "worktree"` and get
   separate VMs under `isolation = "session"`, verified by scope key.
5. `isolation = "subagent"` gives a distinct VM per subagent, verified by `agent_id` in `ls` output.
6. Removing a worktree destroys its VMs; `gc` reclaims anything orphaned by a crash.
7. With `on_sandbox_unavailable = "fail"`, a deliberately broken smolvm install causes refusals and
   **no host execution**, proven by a canary file the guest cannot write.
8. The same repository runs sandboxed under Paperclip through the wrapping adapter, and under an
   ACP agent through the launcher, with no harness-specific configuration in either case.
9. `doctor` reports the detected tier and mechanism for every installed harness, and explains every
   resolved setting and its source file.

### Status, 2026-09-17

| Criterion | Evidence |
| --- | --- |
| Zero-error sessions (rewrite) | Claude Code live: agent typed the command, hook rewrote it, guest answered, 0 denials |
| Zero-error sessions (tool) | Claude Code live: agent read the injected brief, typed `boxer run -c` itself, 0 denials |
| One hook binary, N manifests | 7 dialects in `internal/hook`; `boxer package all` and `boxer install all` render from one template set; no bundle but Claude's mentions Claude (test) |
| Every configuration path | `evals/smoke.sh` 44 checks against real smolvm: isolation × 4 with degradation, require_worktree, create_on, on_sandbox_unavailable, mode × 3, enforcement audit, every dialect, MCP, shims, install, inside-guest guard, gc |
| Orchestrators | T3 Code and Paperclip read the project layer boxer installs; OpenHands has a Workspace adapter; nesting guarded by `BOXER_INSIDE` |
| Other harnesses live | Blocked on credentials on this machine; each bundle validated as far as its CLI allows without a model call |

