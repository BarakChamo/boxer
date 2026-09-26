# boxer

boxer runs a coding agent's shell commands in a sandbox instead of on your machine. Each git
worktree gets its own sandbox, created on the first command, with the worktree mounted at
`/workspace`. The default sandbox is a [smolvm](https://smolmachines.com) microVM with its own
kernel; Apple's `container`, docker and podman are alternatives, and
[Backends](site/content/docs/concepts/backends.mdx) says what each one enforces.

The agent does not need to know. It runs `npm test`; boxer runs it in the sandbox and returns the
same output and exit code.

boxer is one Go binary. macOS on Apple Silicon and Linux (x86-64, arm64) are supported.

## Quick start

Install smolvm, then boxer:

```sh
curl -sSL https://smolmachines.com/install.sh | bash
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh
```

The script puts `boxer` in `~/.local/bin` and prints the `PATH` line to add if that directory is
not on it. npm and `go install` also work; see
[Install](site/content/docs/start/install.mdx).

Run a command in a sandbox, from inside any git repository:

```sh
cd your-repository
boxer doctor                    # what boxer resolved here: scope, image, integrations, problems
boxer run -c 'uname -a'         # prints Linux: the command ran in the VM
```

The first command creates the worktree's VM and pulls its image, which takes longer the first
time. Later commands reuse the running VM.

Then connect your coding agent, and commit what it writes so every worktree inherits it:

```sh
boxer install claude-code       # or codex, gemini-cli, opencode, grok, kimi, pi, dsh; `all` for every one
git add .claude .mcp.json && git commit -m 'boxer'
```

From then on the agent's intercepted commands (`npm`, `node`, `python`, `go`, `make` and others)
run in the sandbox. `git`, `gh` and `ssh` stay on the host. [Connect your
harness](site/content/docs/start/harnesses.mdx) covers each agent, including Copilot, which
installs with `--user`.

## What else it does

| | |
| --- | --- |
| Configure the guest: image, setup commands, services, ports, network allowlist | [Environment](site/content/docs/guides/environment.mdx), [configuration reference](site/content/docs/reference/configuration.mdx) |
| Name the commands an agent should run (`boxer run --task test`) | [Tasks](site/content/docs/guides/tasks.mdx) |
| Summarise JUnit results and turn a failure into a replayable capsule | [Results](site/content/docs/guides/results.mdx) |
| Give each worktree's dev server a stable URL | [URLs](site/content/docs/guides/urls.mdx) |
| List, watch and reclaim sandboxes (`boxer ls`, `boxer watch`, `boxer gc`) | [Managing](site/content/docs/guides/managing.mdx) |
| Fork a warm sandbox into copy-on-write children | [Forking](site/content/docs/guides/forking.mdx) |
| Run the agent itself inside the VM (`boxer shell claude`) | [Inside mode](site/content/docs/guides/inside.mdx) |

Which harnesses, orchestrators and backends are verified, and how, is on
[Support, measured](site/content/docs/evals/results.mdx).

## Documentation

The docs site lives in [`site/`](site); `make docs-dev` serves it locally.

**Using boxer:** [install](site/content/docs/start/install.mdx) ·
[first sandbox](site/content/docs/start/first-run.mdx) ·
[connect your harness](site/content/docs/start/harnesses.mdx) ·
[how it works](site/content/docs/guides/how-it-works.mdx) ·
[configuration](site/content/docs/reference/configuration.mdx) ·
[security model](site/content/docs/concepts/security.mdx) ·
[troubleshooting](site/content/docs/guides/troubleshooting.mdx)

**Building on boxer:** [the three interfaces](site/content/docs/guides/building-on-boxer.mdx) ·
[JSON output](site/content/docs/reference/json.mdx) · [MCP](site/content/docs/reference/mcp.mdx) ·
[Go package](site/content/docs/reference/go.mdx)

**Working on boxer:** [architecture](docs/architecture.md) · [testing](docs/testing.md) ·
[adding a harness](docs/adding-a-harness.md) · [evaluation plan](docs/eval-plan.md) ·
[requirements](docs/requirements.md) · [releasing and the stability contract](docs/release.md) ·
[CONTRIBUTING](CONTRIBUTING.md)

```sh
make test          # unit tests, race detector, coverage floor; a fake smolvm, safe any time
make smoke         # every configuration path against a real microVM, no model
make eval-t1       # every harness CLI against a scripted model and a real VM
make eval-t2       # the same cells against live models, a few cents
```

## Licence

Apache-2.0. See [LICENSE](LICENSE), [SECURITY.md](SECURITY.md) and
[CONTRIBUTING.md](CONTRIBUTING.md).
