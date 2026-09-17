# boxer implementation plan

Requirements live in [requirements.md](requirements.md). This file says how they become code, in
what order, and what proves each step. It is disposable once v1 ships.

## Shape

One Go binary, no daemon, wrapping the `smolvm` CLI. smolvm is the only state store: every VM boxer
owns carries `boxer.*` labels, read back through `machine ls --json`. Setup completion is a marker
file inside the guest overlay. boxer keeps nothing on the host except shims it was asked to write.

```text
cmd/boxer/            subcommands: up run down doctor ls gc shim hook mcp package
internal/config       boxer.toml resolution (user → repo → worktree), env overrides, provenance
internal/scope        git worktree detection, isolation key, degradation
internal/vm           smolvm wrapper: list/status/create/start/exec/stop/delete/branch
internal/decide       the one pure decision: allow | rewrite | block
internal/hook         stdin JSON → decide → per-harness output dialect; provision/instruct/reclaim
internal/mcp          stdio JSON-RPC server: boxer_run, boxer_status
internal/shim         PATH shims for the intercept list
internal/bundle       embedded per-harness templates; `boxer package <harness>` renders them
bundles/<harness>/    the templates
evals/                end-to-end checks against installed harnesses
```

## Decisions made while building

- **Rewrite is pure string logic.** The hook rewrites `<cmd>` to `boxer run -c '<cmd>'` without
  consulting VM state; `boxer run` provisions lazily or refuses with the §3.6 error. One place
  owns the failure policy.
- **No worktree hooks.** Claude Code's `WorktreeCreate` replaces worktree creation when registered.
  boxer provisions on `SessionStart` and on first `run`; `gc` reaps VMs whose worktree is gone.
- **Setup failure deletes the VM** rather than marking it broken. No host state, one fix command.
- **Network default is allowlist with registry hosts.** smolvm pulls inside the guest.
- **Images are packed once per host.** smolvm re-pulls per machine; `box.packed` packs each image
  into `<state>/packs/<hash>.smolmachine` under a lock and creates machines `--from` it. Measured:
  pack alpine 10 s once, then create 0.3 s and start 0.4 s with volumes, allowlist and labels intact.
- **Hooks call `boxer` by name.** Bundles do not vendor the binary; `doctor` checks PATH.

## Order and proof

| Step | Deliverable | Proof | Status |
| --- | --- | --- | --- |
| 0 | smolvm spike | exec/stdin/exit/mount/branch verified on Apple Silicon | done |
| 1 | config, scope, decide | table tests | done |
| 2 | vm wrapper, `up run down ls doctor gc` | unit tests against a fake `smolvm`; `evals/smoke.sh` against the real one | done |
| 3 | `hook` dialects: claude-code, codex, grok, kimi, dsh, gemini-cli, opencode | golden stdin/stdout tests per dialect; smoke | done |
| 4 | `shim install` | shim execs `boxer run` with its own dir stripped from PATH (the smolvm launcher calls `uname`; an unstripped shim fork-bombed the host) | done |
| 5 | `mcp` | initialize, tools/list, tools/call round trip; smoke | done |
| 6 | `package <harness>` for all seven | every rendered bundle is valid JSON, names only `boxer`, and no bundle but Claude's mentions Claude | done |
| 7 | evals | `evals/smoke.sh` 38 checks, real smolvm, every config path and dialect; `evals/harness.sh` live headless session per installed harness | smoke 38/38; Claude Code live 2/2 (rewrite and tool mode, zero denials); Codex, Gemini, OpenCode, Grok skipped: not installed on this machine |

### Second pass: orchestrators and adapters

| Step | Deliverable | Proof | Status |
| --- | --- | --- | --- |
| 8 | `boxer install <harness>`: project-layer hooks/tool/instruction, merged and idempotent | unit tests; smoke | done |
| 9 | `BOXER_INSIDE` guard: hooks silent and `run` direct inside a guest | unit test; smoke | done |
| 10 | idle-timeout gc via host last-used mtimes | `gc --dry-run` | done |
| 11 | OpenHands `BoxerWorkspace` | Python test with SDK stubbed | done |
| 12 | Paperclip, T3 Code: analysis and integration path | `docs/orchestrators.md`; source read | done |
| 13 | Live evals beyond Claude Code | Codex: hooks fire live, turn blocked by ChatGPT usage quota until 2026-09-20. Gemini: extension installs, context + MCP registered, no login. Grok: `grok plugin validate` passes, installs, not signed in. OpenCode: installed, no provider credentials. Kimi: project install verified, hooks are user-level TOML, no login. DSH: `@deepseek-ai/dsh` not installed; bundle from docs only | blocked on credentials; `evals/harness.sh` skips and names each |
| 14 | Kimi and DSH bundles corrected from their docs (TOML `[[hooks]]`, `.kimi-code/mcp.json`, `.dsh/hooks.json`) | bundle + install tests | done |

| 15 | `integration = "inside"`: `boxer shell`, `boxer acp`, harness install table, config-dir mounts, harness shims, `BOXER_INSIDE` | unit tests (`inside`, `shim`, `config`); inside T1 cells | done |
| 16 | Inside T1 cells: shell for claude, codex, gemini, opencode, pi, kimi; ACP for claude, codex, gemini, kimi, opencode | `boxer-eval --harness inside`, `--harness inside-acp` | done: shell 6/6, ACP 5/5 (each cell green individually on 2026-09-17; fixes on the way: libssl3 for codex-acp, npm retries, pack cache, nested-sandbox settings in the table) |

The evaluation programme across nine harnesses and orchestrators is planned separately in
[eval-plan.md](eval-plan.md).

## Learned while building

- PATH shims must remove their own directory before exec. Anything on the host that resolves an
  intercepted name (the smolvm launcher script runs `uname -s`) otherwise recurses without bound.
- `on_sandbox_unavailable = fail` plus a failing `setup` re-provisions on every command (~17s each)
  until the operator fixes `setup`. Correct, loud, and the message names the fix; not softened.
- The default network allowlist admits only the image registry. A `setup` that installs packages
  needs its mirror in `allow_hosts`; `doctor` cannot know that, the failing step says it.

## Out of v1

Branching commands beyond `boxer branch` passthrough, ACP wrapper, OpenHands and Paperclip
adapters (separate repos, shell out to `boxer`), idle-timeout gc (smolvm exposes no last-used
time; gc reaps orphaned worktrees only).
