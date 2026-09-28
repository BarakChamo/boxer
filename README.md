# boxer

Run coding agents in parallel without them colliding. boxer gives every git worktree its own
sandbox and runs your agent's commands in it.

```console
$ cd ~/code/myapp-fix-ui   && boxer up
$ cd ~/code/myapp-add-auth && boxer up
$ boxer ls                  # some columns left out
NAME         STATE    BRANCH    SERVES
mild-lynx    running  add-auth  https://add-auth.myapp.localhost:1355
olive-comet  running  fix-ui    https://fix-ui.myapp.localhost:1355
```

Two worktrees, both running a dev server on port 3000, each in its own microVM with its own URL.
Your agent does not change. It runs `npm test`, and boxer runs it in that worktree's sandbox.

**[Documentation](https://barakchamo.github.io/boxer/docs)** ·
[Quickstart](https://barakchamo.github.io/boxer/docs/quickstart) ·
[Set up your agent](https://barakchamo.github.io/boxer/docs/setup)

## Why

I built boxer to make parallel agent work easy. Several agents on several worktrees share one
machine. Two dev servers want port 3000, one task's installs break another's, and every command
runs against your own system. boxer gives each worktree its own sandbox on the same machine,
without a hosted platform and without changing your tools. It plugs into the agent or orchestrator
you already use.

## Install

```sh
curl -sSL https://smolmachines.com/install.sh | bash                            # smolvm, the sandbox
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh    # boxer
```

Or `go install github.com/BarakChamo/boxer/cmd/boxer@latest`. macOS on Apple Silicon, Linux on
x86-64 or arm64.

## Quick start

```sh
cd your-repo
boxer install claude-code            # or codex, gemini-cli, opencode, pi, grok, ...
git add .claude .mcp.json && git commit -m "Run agent commands in boxer"
boxer doctor                         # checks the setup
```

Start Claude Code and ask it to run `node -p process.platform`. It prints `linux`, from the
sandbox. Build tools (`npm`, `node`, `python`, `go`, `make` and more) run in the sandbox. `git`,
`gh` and `ssh` stay on your machine. The [Quickstart](https://barakchamo.github.io/boxer/docs/quickstart) adds a `boxer.toml`
for your dev server.

## Set up your agent

| Agent | Setup |
| --- | --- |
| Claude Code | `boxer install claude-code` |
| Codex | `boxer install codex` |
| Gemini CLI | `boxer install gemini-cli` |
| Copilot CLI | `boxer install copilot --user` |
| OpenCode | `boxer install opencode` |
| pi | `boxer install pi` |
| Grok | `boxer install grok` |
| Kimi Code, DSH | `boxer install kimi`, `boxer install dsh`, then tool mode |
| fx, or anything else | `boxer shell <harness>` runs the agent inside the sandbox |

Each has a [setup page](https://barakchamo.github.io/boxer/docs/setup) with what it writes and how
to check it.

Using an orchestrator? [Conductor, T3 Code, Paperclip, herdr, Multica and
OpenHands](https://barakchamo.github.io/boxer/docs/orchestrators) each have a page.

## Configure your project

A `boxer.toml` at the repository root describes the sandbox. Without one, boxer picks an image from
your lockfile.

```toml
image = "mirror.gcr.io/library/node:24-alpine"
setup = ["npm ci"]                        # once per worktree
start = ["npx next dev -H 0.0.0.0 -p 3000"]   # listen on 0.0.0.0
ready = "wget -q -O /dev/null http://127.0.0.1:3000/"

[network]
allow_hosts = ["registry.npmjs.org"]      # everything else is blocked
ports = ["auto:3000"]                     # a free host port per worktree

[urls]
enabled = true                            # https://<branch>.<repo>.localhost:1355
```

[Examples](examples/) cover Next.js, Vite, Python, Go and monorepos.
[Configuration](https://barakchamo.github.io/boxer/docs/reference/configuration) lists every key.

## Backends

| Backend | Isolation | Egress allowlist |
| --- | --- | --- |
| smolvm (default) | own kernel per sandbox | yes |
| Apple `container` | own kernel per sandbox | no |
| docker, podman | one shared kernel | no |

Set `backend = "docker"` in `boxer.toml` to switch. See
[Backends](https://barakchamo.github.io/boxer/docs/backends).

## Documentation

- [What is boxer](https://barakchamo.github.io/boxer/docs) and the [Quickstart](https://barakchamo.github.io/boxer/docs/quickstart)
- [Set up your agent](https://barakchamo.github.io/boxer/docs/setup) and [your orchestrator](https://barakchamo.github.io/boxer/docs/orchestrators)
- [Set up your project](https://barakchamo.github.io/boxer/docs/configure/environment): images, dependencies, dev servers, URLs
- [Security](https://barakchamo.github.io/boxer/docs/concepts/security): what is isolated and what is not
- [CLI](https://barakchamo.github.io/boxer/docs/reference/cli), [Go package and JSON](https://barakchamo.github.io/boxer/docs/reference/building-on-boxer)
- [Evaluations](https://barakchamo.github.io/boxer/docs/evals) and [benchmarks](https://barakchamo.github.io/boxer/docs/evals/benchmarks)

## Development

```sh
make build    # bin/boxer
make test     # unit tests, race detector, coverage floors
make smoke    # every configuration path against a real sandbox
make docs-dev # the docs site on localhost
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/architecture.md](docs/architecture.md).

## License

Apache-2.0. Report vulnerabilities as described in [SECURITY.md](SECURITY.md).
