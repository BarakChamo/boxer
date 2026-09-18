# Status: v0.2.0 (2026-09-18)

The stopping point for this slice: comprehensive evals run against real harnesses and real
orchestrators on this machine, every skip explained. Reports: [eval-t1.md](eval-t1.md) (scripted
model, real harness CLIs, real smolvm) and [eval-t2.md](eval-t2.md) (live models). Method:
[eval-plan.md](eval-plan.md). Release shape: [release.md](release.md). API: [api.md](api.md).

## Proven

Unit tests green; smoke **49/49** (which now also prints and bounds cold start, warm run and hook
latency); T1 **61 pass, 0 fail, 2 skip**, and two consecutive full runs gave identical per-cell
verdicts; T2 live on `zai/glm-5.3-flash` **38 pass, 0 fail, 14 skip** for $0.20. Every cell is a
fresh repository and a fresh VM.

Skips, all of them explained: Gemini CLI and its inside and ACP cells need `GEMINI_API_KEY`
(the gateway has no Gemini-protocol endpoint), the six noncompliant cells are scripted and run at
t1 only, and Multica needs an account (`multica setup`).

Measured on Apple Silicon, smolvm 1.16.1: cold start 0.9 s from a host pack, warm `boxer run`
66 ms, rewrite hook 13 ms.

| Harness | Outside rewrite | Outside tool | Inside shell | ACP | T2 live |
| --- | --- | --- | --- | --- | --- |
| Claude Code | pass | pass | pass 13 s | pass 13 s | 7/7 live; session and subagent isolation and the four worktree timings pass at t1 |
| Codex | pass | pass | pass 22 s | pass 14 s | 2/2 live (gateway, no ChatGPT login) |
| Gemini CLI | pass | pass | pass 10 s | pass 13 s | skip: needs `GEMINI_API_KEY` (only harness the gateway cannot serve) |
| OpenCode | pass | pass | pass 11 s | pass 11 s | 3/3 live |
| pi | pass | pass | pass 15 s | no ACP server | 3/3 live |
| Kimi | block-only hooks; shims | pass | pass 17 s | pass 14 s | 2/2 live |
| GitHub Copilot CLI | pass (user hooks; `modifiedArgs`) | pass | not measured | `copilot --acp` (row present, not run) | 3/3 live for $0.0092 through BYOK on the gateway — no Copilot seat needed |
| Grok | pass (user and project hooks) | live: one denial then recovery | pass 13 s | pass 14 s | 4/4 live |
| DSH | deny-only hooks through the `dsh-hooks-claude-code` bridge; shims | pass | not in table | none | t1 3/3, t2 2/2 live; tool mode only (the bridge ignores `updatedInput`), and boxer provisions on `UserPromptSubmit` because its `SessionStart` is detached |

Inside timings are for a host that already holds the harness pack; the first `boxer shell <h>`
per host pays the install once (10 s to 11 min depending on npm) and packs the result.

| Orchestrator | Path | Result |
| --- | --- | --- |
| OpenHands | terminal `shell_path` = `boxer-bash` (`boxer shim install --shell`), real SDK 1.49 | t1 pass; **live pass** through the gateway (agent's terminal ran in the guest); the earlier `BoxerWorkspace`-only design did not sandbox the agent's terminal |
| Paperclip | project layer over `claude-agent-acp`, one git worktree per issue | driver (`orch_paperclip.go`): T1 pass 21 s; **live 2026-09-18 pass, 2 m 4 s, $0.0364** |
| T3 Code | project layer, and the instance's `binaryPath` on a `boxer shim` (inside) | driver (`orch_t3.go`), two cells: T1 pass 9 s / 18 s; **live 2026-09-18 pass, 18 s $0.0047 and 27 s $0.0070** |
| Multica | project layer; harness path through `multica runtime profile create --command-name` + `runtime profile set-path`, or `MULTICA_<PROVIDER>_PATH` | checklist: a driver needs a self-hosted Multica server (documented and scriptable, not attempted) |
| herdr | project layer in a pane over the socket API (`herdr server`, `workspace create`, `pane split`, `agent start/prompt/read`) | driver (`orch_herdr.go`): **T1 pass 8 s; live 2026-09-18 pass, 15 s, $0.0039**. Also level S (`terminal.default_shell` = `boxer-bash`) and a validated `herdr-plugin.toml` (`start-agent` action, `worktree.created` → `boxer up --detach`). `herdr integration install claude` and `boxer install claude-code --user` coexist in `~/.claude/settings.json`, verified both orders |
| Conductor (local) | `.conductor/settings.toml` (**not** the legacy `conductor.json`): `claude_code_executable_path`/`codex_`/`opencode_` on boxer's harness shims plus `scripts.setup` warming the sandbox, written by `boxer install conductor` | checklist: local workspaces have no API |
| ACP real client | `@agentclientprotocol/sdk` example client against `boxer acp claude` | pass: initialize, session, prompt, `Terminal` tool, `Linux` |

## Adherence

`boxer-eval --tier adherence` (third pass, stream B): does a live model follow the injected brief
when the prompt never mentions boxer? Full matrix, spend and transcript evidence in
[eval-adherence.md](eval-adherence.md). Verdict per cell: `harness` only when every model fails it.

| Harness | brief (tool) | recovery (tool) | multistep (rewrite) | Verdict |
| --- | --- | --- | --- | --- |
| Claude Code | 4/4 | 4/4 | 4/4 | pass |
| Codex | 4/4 | 3/4 (GLM passed `cwd: /workspace`) | 4/4 | model adherence |
| OpenCode | 4/4 | 4/4 | 4/4 | pass |
| pi | 4/4 | 4/4 | 4/4 | pass |
| Kimi | 2/4 (Haiku, qwen: one denial) | 4/4 | 4/4 (tool mode) | model adherence |
| Grok | 0/4 (one denial each) | 4/4 | 4/4 | **harness**: Grok does not surface session-start context; tool mode always costs one denial, use rewrite mode |

Models: GLM 5.3 flash 16/18 ($0.11), Haiku 4.5 16/18 ($0.79), qwen3.7 flash 16/18 ($0.03),
deepseek v4 flash 17/18 ($0.08). No command ran on the host in any of the 72 cells. Recommended
default: `deepseek/deepseek-v4-flash` (zero denials outside Grok, $0.0046 per cell).

## Shipped in this slice

Third pass (2026-09-18): `warm_on_session_start` (SessionStart returns in 9 ms, VM ready before the
first tool call), `worktree.manage`, `boxer install git` (post-checkout warm-up), `doctor` signal
report, rewrites carry `--session/--agent` under those isolations, Agent Plugins 1.0.0 package
(`boxer package plugin`, validated in Claude Code, Codex, Gemini, Grok; schema-conformance test),
MCP lifecycle signals (detached warm-up on `initialize`, last-used on EOF), MCP `cwd` mapping from
guest paths, install retry on a dropped exec, adherence tier with the two-model rule, gateway spend
tracking and a per-run budget, provider rate limits reported as skips, concurrent-creator wait
when two boxers race smolvm, Codex guest gets CA certificates.

First and second pass:
- `pkg/boxer` facade (experimental), `--json` on `ls`, `status`, `down`, `gc`, `doctor`.
- goreleaser, `install.sh`, npm wrapper `boxer-cli`, CI and release workflows, plugin version
  stamping with a `doctor` mismatch warning.
- Image packs once per host, harness packs once per host per harness (keyed on image, harness
  and install line), pruned by `gc`; install verified with `<bin> --version` before packing.
- Nested-sandbox rows for Codex; audit clean for Gemini, OpenCode, pi, Grok, Kimi.
- Login-travel warning for Claude; Codex `auth.json` travels; Gemini uses Keychain on macOS.
- Eval runner: infra-failure retry, kept failures, SIGINT report, host lock; T2 tier with
  credential detection; Grok driver; OpenHands and orchestrator checklist drivers.

## Known limits

- One host drives one smolvm at a time (`~/.local/state/boxer/eval.lock`).
- The first `boxer shell <harness>` on a host pays the harness install (10 s to 11 min, npm over
  smolvm's TSI networking); every later worktree starts from the host pack in seconds.
- Claude Code's Keychain login does not enter the VM (`claude setup-token`); Gemini likewise.
- Grok does not surface session-start context to the model, so tool mode costs one denial on the
  first shell command; rewrite mode is the right Grok default. Recorded in its dialect.
- DSH has no session-end signal, so nothing reclaims its sandbox at the end of a session; MCP EOF
  and `gc` are the fallbacks.
- Under `session` and `subagent` isolation the MCP run tool resolves the worktree scope (MCP
  carries no ids); hooks carry them.
- An orchestrator that creates a worktree and launches a harness into it in one step wants
  `boxer install git`: T3's provider gives up while a cold VM boots.
- Packs are 130 to 365 MB each. `gc` prunes them by `idle_timeout`; a long eval session can fill a
  small disk before that runs.

## To run the rest

```sh
# evals/.env (gitignored), then: make eval-t2
AI_GATEWAY_API_KEY=…             # one key: every harness runs live through the Vercel AI Gateway
GEMINI_API_KEY=…                 # optional: Gemini CLI only (the gateway has no Gemini-protocol endpoint)
BOXER_EVAL_MODEL=…               # optional: one gateway model id for every harness (default: cheapest per family)
npm i -g paperclipai t3          # orchestrator drivers
multica setup                    # account
python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools
```

## What comes next

devcontainer.json loader, `Backend` interface (Firecracker, Docker Sandboxes), a running-boxes
dashboard on the JSON API, Homebrew tap, plugin marketplace repos, Conductor cloud once smolvm
exists there. Landed on the `packaging` branch 2026-09-17: the Agent Plugins package collapse
(R-LVL-6, R-LVL-6a) and MCP lifecycle signals (R-SIG-0).
