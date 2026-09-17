# boxer

Runs agent shell commands inside a [smolvm](https://github.com/smol-machines/smolvm) microVM keyed
to the git worktree, and makes every coding harness use it without the agent ever having to make a
mistake first. One Go binary; smolvm is the only state.

```sh
go build -o bin/boxer ./cmd/boxer     # or: go install ./cmd/boxer
curl -sSL https://smolmachines.com/install.sh | bash   # smolvm, if missing
boxer doctor                          # what would happen here, and why
boxer run -c 'bun test'               # first call provisions the VM (~20s), then ~50ms per command
```

## How it works

- **Scope.** `sha256(worktree path)` names one VM per worktree. `isolation` can widen to `repo`
  or narrow to `session` / `subagent` when the harness supplies ids; missing ids degrade one level
  or fail, per `on_missing_id`.
- **Hooks first, errors last.** `boxer hook <harness>` is one binary speaking every harness's hook
  dialect. At session start it provisions the VM and injects the agent brief; on each shell call it
  rewrites intercepted commands to `boxer run` (Claude Code, Codex, Grok Build, Gemini CLI,
  OpenCode) or, where a harness can only block, stays silent and lets PATH shims do the same job.
- **Three modes.** `rewrite` (transparent), `tool` (shell denied, `boxer_run` MCP tool is the way),
  `off`. Per-harness overrides in `[harness.<name>]`.
- **Errors are instructions.** Every refusal is `boxer: <reason>` plus `scope`, `worktree`,
  `cause`, and a runnable `fix:` line.

## Configuration

`boxer.toml` in the worktree, the repository, then `~/.config/boxer/`; earlier wins. Unknown keys are
errors. Every scalar is also `BOXER_<KEY>` in the environment. `boxer doctor` prints each value and
where it came from.

```toml
isolation   = "worktree"        # repo | worktree | session | subagent
mode        = "rewrite"         # rewrite | tool | off
enforcement = "both"            # hook | shim | both | audit
intercept   = ["npm", "bun", "node", "python", "go", "make"]
passthrough = ["git", "gh", "ssh", "boxer"]
image       = ""                # default: detected from the lockfile, else debian:bookworm-slim
setup       = ["bun install"]   # once per VM, inside the guest
[network]
mode        = "allowlist"       # registry hosts for the image are always allowed
allow_hosts = ["registry.npmjs.org"]
[harness.gemini-cli]
mode        = "tool"
```

## Two placements

```toml
integration = "outside"   # default: your harness runs on the host; boxer sandboxes the commands it runs
integration = "inside"    # your harness runs inside the VM; nothing to hook or rewrite
```

**Outside** is everything below: the Agent Plugins package, optional hooks, `install`. **Inside** is
how Docker Sandboxes, dev containers, and cloud sessions do it, on smolvm:

```sh
boxer shell claude                   # VM for this worktree, claude installed in it once, claude runs inside
boxer shell codex -- exec "fix the tests"
boxer acp gemini                     # the harness's ACP server inside the VM, stdio piped: point T3 Code,
                                     # Paperclip, Zed, or JetBrains at this command
boxer shim install --harness claude  # a `claude` on PATH that is really `boxer shell claude`, for orchestrators
```

The worktree and each harness's config directory (`~/.claude`, `~/.codex`, `~/.gemini`, …) are
mounted at their host paths, so sessions and logins are shared; Claude Code's macOS Keychain login
does not travel, pass `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`. Harnesses without an ACP
server (pi, Grok) have `shell` only.

## Installing into a repository

```sh
boxer install all      # .claude/settings.json + .mcp.json, .codex/hooks.json, .gemini/settings.json,
                       # .opencode/plugins/boxer.ts + opencode.json, .grok/hooks/boxer.json — merged, idempotent
```

This project layer is what orchestrators load: T3 Code and Paperclip launch harnesses with their
own config directories, so a user-level plugin never reaches those sessions. Having both layers is
harmless — see [docs/orchestrators.md](docs/orchestrators.md). OpenHands uses
[adapters/openhands](adapters/openhands/) instead.

## Harness bundles

`boxer package <harness>|all --out dist` renders one plugin per harness from the same four
components — instruction, lifecycle hooks, run tool, gap closer:

| Harness | Bundle | Install |
| --- | --- | --- |
| Claude Code | plugin: skill, hooks, `.mcp.json`, `bin/` shims, `boxed` agent without Bash | `claude plugin install dist/claude-code` |
| Codex CLI | plugin: skill, hooks, MCP | plugin flow, or copy `hooks/hooks.json` to `.codex/` |
| Gemini CLI | extension: `GEMINI.md`, hooks, MCP, `excludeTools` in tool mode | `gemini extensions install dist/gemini-cli` |
| Grok Build | plugin: skill, hooks, MCP | `/plugin` |
| OpenCode | `.opencode/plugins/boxer.ts`, `opencode.json` MCP entry, `AGENTS.md` section | copy into the repo |
| DSH, Kimi Code | hooks (block-only), skill, MCP; shims required | see each bundle's README |

## Verification

```sh
go test ./...          # unit: config, scope, decide, hook dialects, mcp, shims, bundles, inside (fake smolvm)
evals/smoke.sh         # real smolvm: every config path, every hook dialect, mcp, shims, gc   (44 checks)
boxer-eval --tier t1   # real harness + scripted model + real smolvm: 31 outside cells, 11 inside cells
cmd/boxer-eval/        # eval matrix: --tier t1 (fake model) or --tier t2 (live credentials from evals/.env)
```

The T1 tier (`cmd/boxer-eval`) starts a fake model server that always answers a shell tool call
with `uname -a`, launches the real harness CLI against it, and checks one oracle: the command ran
in the guest (`Linux`), a host canary was not written, no denials, and the VM has the right scope.
Inside cells run the same through `boxer shell <harness>` and `boxer acp <harness>`. Plan and
findings: [docs/eval-plan.md](docs/eval-plan.md).

Last run on Apple Silicon, smolvm 1.16.1: smoke 44/44; T1 outside 31/31; T1 inside shell 6/6
(claude, codex, gemini, opencode, pi, kimi) and ACP 5/5 (claude, codex, gemini, kimi, opencode).
Images are packed once per host, so the first VM for an image pays the pull and later ones boot in
under a second. Claude Code live 2/2 — in `rewrite` mode
the agent typed `uname -a` and the hook put it in the guest; in `tool` mode the agent read the
injected brief and typed `boxer run -c 'uname -a'` itself; zero denials in either. Codex: project
hooks fire under `codex exec` (a usage limit stopped the turn). Gemini: extension installs and
registers context + MCP. Grok: plugin passes `grok plugin validate` and installs. OpenCode and
Kimi: project install verified. Live turns on those need a login or API key on the machine; the
eval skips and says which. Set `BOXER_TRACE=/path` to log every hook input and output.

Requirements: [docs/requirements.md](docs/requirements.md). Plan: [docs/plan.md](docs/plan.md).
