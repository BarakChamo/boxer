# Status: v1.0.0 (2026-09-18)

The stopping point for this slice: comprehensive evals run against real harnesses and real
orchestrators on this machine, every skip explained. Reports: [eval-t1.md](eval-t1.md) (scripted
model, real harness CLIs, real smolvm) and [eval-t2.md](eval-t2.md) (live models). Method:
[eval-plan.md](eval-plan.md). Release shape: [release.md](release.md). API: [api.md](api.md).

## Proven

Unit tests green with the race detector and a per-package coverage floor; lint and the
vulnerability scan clean; smoke **53/53** (which also prints and bounds cold start, warm run and
hook latency); T1 **67 pass, 0 fail, 1 skip**, run twice with identical per-cell verdicts; T2 live
on `zai/glm-5.3-flash` **42 pass, 0 fail, 14 skip** for $0.19. Every cell is a fresh repository and
a fresh VM.

Measured on Apple Silicon with smolvm 1.16.1: cold start 687 ms from a host pack, warm `boxer run`
60 ms, rewrite hook 10 ms.

The publishing path is proven rather than asserted: `v1.0.0-rc.1` published four platform
archives, four SBOMs, the plugin and skill tarballs, the npm tarball and a `checksums.txt` signed
keylessly against the release workflow's OIDC identity. The signature verifies with the command in
[release.md](release.md), and both download routes install that release on a machine that had no
boxer: `install.sh` and `npm i -g` each report `boxer 1.0.0-rc.1`.

One cell has failed once in nine runs: `acp-codex/inside/acp/worktree`, after its own retry, with
`agent closed: EOF` and an empty stderr. It has passed every run since, including both runs of the
release pair. The driver now carries the agent's last output into the failure, so the next
occurrence explains itself rather than needing another nine runs.

Added 2026-09-18 (stream C): GitHub Copilot CLI as the ninth harness — **T1 4/4, T2 3/3 live for
$0.0092** — and a real herdr driver replacing its checklist — **T1 pass, T2 pass for $0.0039**.
Copilot runs at t2 with no Copilot seat: its BYOK provider variables point it at the gateway.

Skips, all of them explained: Gemini CLI and its inside and ACP cells need `GEMINI_API_KEY`
(the gateway has no Gemini-protocol endpoint), the six noncompliant cells are scripted and run at
t1 only, and Multica needs an account (`multica setup`).

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

`boxer-eval --tier adherence`: does a live model follow the injected brief when the prompt never
mentions boxer? Four models, 24 cells each, all 96 run live on 2026-09-18. Full matrix, spend and
per-cell findings in [eval-adherence.md](eval-adherence.md). A cell is blamed on the **harness**
only when every model fails it; one failure among passes is that model's adherence.

| Verdict | Cells |
| --- | --- |
| pass | 15 |
| model adherence | 9 |
| **harness** | 1 — `grok/tool/user/worktree/brief` |

Grok is the one harness finding, and it is the same one as before: Grok never surfaces
session-start context to the model, so in tool mode the first shell command always costs one
denial before the agent learns about `boxer_run`. Rewrite mode is the right Grok default, and its
dialect records this. Copilot's brief cell fails for three of four models, all with one denial and
a recovery, which is model adherence rather than a harness limit: its `recovery` cell passes for
every model.

| Model | Pass | Fail | Spend |
| --- | --- | --- | --- |
| zai/glm-5.3-flash | 22 | 2 | $0.11 |
| anthropic/claude-haiku-4.5 | 19 | 5 | $1.10 |
| alibaba/qwen3.7-flash | 18 | 6 | $0.03 |
| deepseek/deepseek-v4-flash | 21 | 3 | $0.10 |

No command reached the host in any of the 96 cells. `make eval-adherence` runs all four models;
the two-model rule needs more than one, so a single-model run can never reach a verdict.

## Shipped in this slice

Fourth pass (2026-09-18), the work towards a first public release:
- **Published content is static.** The package and the skill are byte-identical for every user but
  for the release version, and a test renders them twice under hostile configurations to prove it.
  Configuration reaches the agent at run time instead: `boxer brief [--json]`, the same text hooks
  inject, also served as an MCP resource.
- **The skill carries its `scripts/` layer** (`run`, `task`, `status`, `brief`), which is the
  deterministic command path the Agent Skills specification defines and the answer to a harness
  that hides MCP tools behind a dispatcher.
- **Named tasks.** `[tasks]` in `boxer.toml`, `boxer tasks`, `boxer run --task <name>`: the agent
  invokes a name the repository declared rather than composing a line the intercept list has to
  catch. An unknown name is refused with the real ones.
- **An opt-in event stream.** `[telemetry]` with file, stderr and OpenTelemetry sinks, `boxer logs`,
  an event tail on `boxer status --json`, command lines elided unless asked for. Off by default.
  Schema in [events.md](events.md).
- **GitHub Copilot CLI** as the ninth harness, its dialect corrected against a running binary, and
  a real herdr driver and plugin replacing that checklist.
- **Release engineering:** Apache-2.0 and the community files, a CI gate with the race detector,
  per-package coverage floors, lint and vulnerability scanning on two platforms, reproducible
  builds, SBOMs, keyless signing, the npm launcher the package always declared but never shipped,
  and the four install routes exercised against a staged release.
- **An unknown top-level table in `boxer.toml` warns rather than fails**, so a repository that
  adopts a newer boxer's table still loads under an older binary.

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
  small disk before that runs, and two back-to-back T1 runs have done exactly that.
- Copilot's inside-mode row exists but has never been run; an inside cell costs a full npm install
  in the guest.
- Conductor and Multica remain checklists: local Conductor workspaces have no API, and a Multica
  driver needs a self-hosted server.

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
