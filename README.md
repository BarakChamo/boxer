# boxer

Run a coding agent's shell commands in a sandbox, one per git worktree. For Claude Code, Codex,
Gemini CLI, Copilot, OpenCode and the agents and orchestrators around them.

```console
$ boxer run -c 'uname -s && pwd'
Linux
/workspace
```

The agent keeps working as before. It runs `npm test`, boxer runs it in the worktree's sandbox from
the matching directory, and the exit code, stdout and stderr come back unchanged. The default
sandbox is a [smolvm](https://smolmachines.com) microVM with its own kernel. Apple `container`,
docker and podman are also supported.

macOS on Apple Silicon and Linux on x86-64 and arm64.

## Motivation

I built boxer to make parallel agent work easy. Running several agents at once, each on its own
worktree, works until their environments collide: two dev servers want the same port, one task's
dependency install breaks another's, and every task shares the host system. Each workstream needed
its own isolated environment on the same machine, without a hosted platform or a change of tools.

boxer is a single, self-contained utility for that. It gives each git worktree its own local
sandbox and runs the agent's commands inside it. It does not replace a harness, orchestrator or
editor. It integrates with them through hooks, PATH shims, an MCP tool or a Go package, so the same
isolation applies whether the agent is Claude Code in a terminal, a task in an orchestrator, or
your own automation.

## Install

```sh
curl -sSL https://smolmachines.com/install.sh | bash                                     # smolvm, the default backend
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh       # boxer, into ~/.local/bin
```

Or with Go, or from the npm tarball on the release:

```sh
go install github.com/BarakChamo/boxer/cmd/boxer@latest
npm i -g https://github.com/BarakChamo/boxer/releases/download/v1.0.0/boxer-cli-1.0.0.tgz
```

To use docker, podman or Apple `container` instead of smolvm, install it and set
`backend = "docker"` in `boxer.toml`, with `network.mode = "on"` or `"off"`: only smolvm enforces
the default allowlist, and the others refuse it rather than open the network. [Install](site/content/docs/start/install.mdx) has the
details for each.

## Quick start

```sh
cd your-repository
boxer doctor                        # what boxer resolved here, and anything wrong
boxer run -c 'uname -a'             # Linux: the first command creates the sandbox
boxer install claude-code           # hooks, MCP tool and skill for your agent
git add .claude .mcp.json && git commit -m 'Use boxer'
```

Commit what `boxer install` writes. A worktree is cut from a branch, so an integration that is
not committed is not in the next worktree.

From then on, the agent's `npm`, `node`, `python`, `go`, `make` and the rest of the intercept list
run in the sandbox. `git`, `gh` and `ssh` stay on the host.

## Connect your agent

| Agent | Command | How it is enforced |
| --- | --- | --- |
| Claude Code, Codex, Gemini CLI, Grok | `boxer install <harness>` | hooks rewrite each intercepted command |
| Copilot CLI | `boxer install copilot --user` | the same, in `~/.copilot` |
| OpenCode, pi | `boxer install opencode`, `boxer install pi` | a plugin or extension rewrites it in-process |
| Kimi Code, DSH | `boxer install <harness>` and `mode = "tool"` | shell calls are denied; the `boxer_run` tool is the way in |
| fx, or any agent you want off the host | `boxer shell <harness>` | the agent itself runs in the sandbox |
| Any agent that loads skills | `npx skills add BarakChamo/boxer --skill boxer` | instructions only, not enforced |

Orchestrators (T3 Code, Paperclip, herdr, Conductor, Multica, OpenHands) create a worktree per task
and use the same integrations. [Orchestrators](site/content/docs/guides/orchestrators.mdx) has the
setup for each. [Skills and plugins](site/content/docs/guides/skills-and-plugins.mdx) covers the skill,
the plugin package, and `npx plugins`.

## Configure

A `boxer.toml` at the repository root describes the guest. Without one, boxer picks an image from
your lockfile.

```toml
image = "mirror.gcr.io/library/node:24-alpine"
setup = ["npm ci --no-audit --no-fund"]          # once per worktree
start = ["npx next dev -p 3000 -H 0.0.0.0"]      # services, started with the sandbox
ready = "wget -q -O /dev/null http://127.0.0.1:3000/"

[network]
allow_hosts = ["registry.npmjs.org"]             # the default network reaches the image registry only
ports = ["auto:3000"]                            # a free host port per worktree

[urls]
enabled = true                                   # https://<branch>.<repo>.localhost:1355 via portless

[tasks.test]
cmd = "npm test"
description = "the unit tests"
```

An existing `.devcontainer/devcontainer.json` is read too. [Examples](examples/) has working
configurations for Next.js, Vite, Python, Go, monorepos, devcontainers and the container backends,
and [Configuration](site/content/docs/reference/configuration.mdx) lists every key.

## Commands

```sh
# Run
boxer run -c '<shell line>'         # in this worktree's sandbox
boxer run --task test               # a task declared in boxer.toml
boxer up | down | status            # create and start, delete, inspect this worktree's sandbox

# See and clean up
boxer ls                            # sandboxes with their worktree, branch, git state and URL (-A: every backend)
boxer url                           # where this worktree's dev server is
boxer rm --gone                     # remove sandboxes whose worktree was deleted
boxer gc                            # reclaim idle sandboxes and unused caches

# Check
boxer doctor                        # resolved configuration, integrations, disk used
boxer backends --probe              # create and run a real sandbox on each installed backend
boxer integrations                  # which harnesses have boxer wired in
```

Every command that reports state takes `--json`. Output adapts to the reader: tables and prompts at a
terminal, plain text for an agent or a pipe. [CLI reference](site/content/docs/reference/cli.mdx).

## Backends

| Backend | Boundary | Egress allowlist | Snapshots and forks |
| --- | --- | --- | --- |
| smolvm (default) | a kernel per sandbox | yes | yes |
| Apple `container` | a kernel per sandbox | no | no |
| docker, podman | one kernel shared by every sandbox | no | no |

A backend that cannot enforce the allowlist refuses it by name rather than running with an open
network. [Backends](site/content/docs/concepts/backends.mdx) compares them.

## Security and performance

boxer protects your machine from what an agent runs. The worktree is mounted read-write, and a few
paths stay on the host on purpose, such as `git` and the MCP servers a harness names. Forwarded
ports bind to `127.0.0.1`. The threat model is a careless or destructive agent, not one trying to
escape: [Security](site/content/docs/concepts/security.mdx).

A command in a running smolvm sandbox takes 33 ms, within 4 ms of `docker exec` into a warm container,
and a Next.js session starts in 7.6 s against 6.8 s with no sandbox. File I/O through the mount is
the main cost. [Benchmarks](site/content/docs/evals/benchmarks.mdx) has the method and every number.

## Build on boxer

```go
b, err := boxer.Open(dir, boxer.Options{})   // github.com/BarakChamo/boxer/pkg/boxer
if err != nil {
	return err
}
if _, err := b.Ensure(true, false); err != nil { // create and start, or do nothing if running
	return err
}
code, err := b.Run([]string{"sh", "-c", "npm test"}, boxer.RunOpts{Stdout: os.Stdout, Stderr: os.Stderr})
```

[`examples/go-embed`](examples/go-embed) runs one command across several worktrees in parallel.
Programs in other languages use `--json`; harness integrations use `boxer hook` and `boxer mcp`.
[Building on boxer](site/content/docs/guides/building-on-boxer.mdx).

## How it is tested

Every harness and orchestrator integration is run for real, with a scripted model and with live
ones. The smoke suite runs every configuration path on every backend, and passes on all seven
backend and host pairs (2026-09-27). The Next.js matrix, live agents in parallel worktrees on
smolvm, scores 99.4% on a pre-release build. [Evaluations](site/content/docs/evals/index.mdx) explains the
tiers, and [Results](site/content/docs/evals/results.mdx) has the results per harness.

## Documentation

The docs site is in [`site/`](site); `make docs-dev` serves it on localhost. Start with
[Introduction](site/content/docs/index.mdx), [Install](site/content/docs/start/install.mdx) and
[Connect your harness](site/content/docs/start/harnesses.mdx).

## Development

```sh
make build      # bin/boxer
make test       # unit tests with the race detector and per-package coverage floors
make smoke      # every configuration path against a real sandbox
make eval-t1    # every harness against a scripted model
```

[CONTRIBUTING.md](CONTRIBUTING.md), [architecture](docs/architecture.md), [testing](docs/testing.md),
[adding a harness](docs/adding-a-harness.md).

## License

Apache-2.0. Report vulnerabilities as described in [SECURITY.md](SECURITY.md).
