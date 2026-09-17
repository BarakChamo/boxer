# boxer

Runs agent shell commands inside a [smolvm](https://github.com/smol-machines/smolvm) microVM keyed
to the git worktree, and makes every coding harness use it without the agent ever having to make a
mistake first. One Go binary; smolvm is the only state.

```sh
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh   # release binary into ~/.local/bin
npm i -g boxer-cli                                     # the same binary, fetched by npm
go install github.com/BarakChamo/boxer/cmd/boxer@latest   # from source
curl -sSL https://smolmachines.com/install.sh | bash   # smolvm, if missing
boxer doctor                          # what would happen here, and why
boxer run -c 'bun test'               # first call provisions the VM (~20s), then ~50ms per command
```

Releases carry darwin/arm64, linux/amd64 and linux/arm64 binaries plus `boxer-plugins-<version>.tar.gz`.
The JSON output, Go facade (`pkg/boxer`, experimental) and MCP tools are in [docs/api.md](docs/api.md);
how releases are cut is in [docs/release.md](docs/release.md).

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
warm_on_session_start = false   # true: SessionStart starts the VM in a detached `boxer up` and never blocks the session
mode        = "rewrite"         # rewrite | tool | off
enforcement = "both"            # hook | shim | both | audit
intercept   = ["npm", "bun", "node", "python", "go", "make"]
passthrough = ["git", "gh", "ssh", "boxer"]
image       = ""                # default: detected from the lockfile, else debian:bookworm-slim
setup       = ["bun install"]   # once per VM, inside the guest
[network]
mode        = "allowlist"       # registry hosts for the image are always allowed
allow_hosts = ["registry.npmjs.org"]
[worktree]
manage      = "off"             # detect: a session in the main checkout shares the repository VM until it enters a worktree
[harness.gemini-cli]
mode        = "tool"
```

`boxer install git` adds a `post-checkout` hook (honouring `core.hooksPath`) that runs
`boxer up --detach` in every worktree `git worktree add` creates, so the VM is warm before any
agent opens it. Opt-in; `boxer install all` leaves git configuration alone.

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

`boxer package plugin --out dist` renders one [Agent Plugins 1.0.0](https://agent-plugins.org)
package, `dist/boxer`, valid for every client at once: `plugin.json`, `skills/boxer/SKILL.md`,
`mcp.json` (the `boxer mcp` server), `AGENTS.md`, and one reverse-domain directory per client
carrying its hooks and README (`com.anthropic.claude-code/`, `com.openai.codex/`, `ai.x.grok/`,
`com.google.gemini-cli/`, `ai.moonshot.kimi-code/`, `com.deepseek.dsh/`, `ai.opencode/`,
`works.earendil.pi/`). The same directory carries the native manifests each loader reads today,
so it installs everywhere now:

```sh
claude plugin install dist/boxer                                   # or: claude --plugin-dir dist/boxer
codex plugin marketplace add dist/boxer && codex plugin add boxer@boxer
grok plugin install dist/boxer
gemini extensions install dist/gemini-cli                          # Gemini reads hooks from hooks/ only; use its view
```

`boxer package <harness>` renders that client's view, the subset of the package it reads, with the
client's `[harness.<name>]` overrides applied; `boxer package all` renders the package and every
view. Kimi, DSH, OpenCode and pi have no plugin loader: their namespace README lists the files to
copy, and `boxer install <harness>` writes them into the repository. `plugin.json` and `mcp.json`
are validated against the spec's schemas in `go test`.

## Verification

```sh
go test ./...          # unit: config, scope, decide, hook dialects, mcp lifecycle, shims, package + schema conformance, inside (fake smolvm)
evals/smoke.sh         # real smolvm: every config path, every hook dialect, mcp, shims, gc   (46 checks)
boxer-eval --tier t1   # real harness + scripted model + real smolvm: 31 outside cells, 11 inside cells
cmd/boxer-eval/        # eval matrix: --tier t1 (fake model) or --tier t2 (live credentials from evals/.env)
```

The T1 tier (`cmd/boxer-eval`) starts a fake model server that always answers a shell tool call
with `uname -a`, launches the real harness CLI against it, and checks one oracle: the command ran
in the guest (`Linux`), a host canary was not written, no denials, and the VM has the right scope.
Inside cells run the same through `boxer shell <harness>` and `boxer acp <harness>`. Plan and
findings: [docs/eval-plan.md](docs/eval-plan.md).

Last run on Apple Silicon, smolvm 1.16.1 (2026-09-17): smoke 44/44; T1 49 pass, 0 fail, 4 skips
(orchestrators needing an install or account); T2 Claude Code 7/7 live, others skipped for
credentials. Inside cells start in 9 to 28 s from per-host harness packs. Full matrix and skip
reasons: [docs/status.md](docs/status.md).

Requirements: [docs/requirements.md](docs/requirements.md). Plan: [docs/plan.md](docs/plan.md).
