# boxer

Coding agents run shell commands. boxer makes those commands run in a
[smolvm](https://smolmachines.com) microVM instead of on your machine — one VM per git worktree,
started automatically, with your worktree mounted at the same path it has on the host.

The agent does not have to know. It types `npm test`, the command runs in the sandbox, the output
comes back looking exactly as it would have. Nothing on your machine is at risk, and nothing about
the agent's experience changes.

One Go binary. smolvm holds the only state.

## Two minutes

```sh
curl -sSL https://smolmachines.com/install.sh | bash                                  # smolvm
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh    # boxer

cd your-repository
boxer doctor                    # what would happen here, and why
boxer run -c 'uname -a'         # Linux … — that ran in the VM, not on your Mac
boxer install all               # your harnesses now use it, without being told
```

The first command in a worktree provisions its VM, which takes about a second from a warm host
pack and about twenty seconds the very first time. After that every command is about fifty
milliseconds of overhead.

`brew install BarakChamo/tap/boxer`, `npm i -g boxer-cli` and
`go install github.com/BarakChamo/boxer/cmd/boxer@latest` are the other three routes;
[docs/install.md](docs/install.md) has the details.

## How an agent ends up in the sandbox

Harnesses differ in what they let a third party do, so boxer has five ways in. They stack:
whichever ones your harness supports are active at once, and `boxer install <harness>` picks the
strongest without being asked.

1. **MCP and a skill — everywhere.** An MCP server (`boxer_run`, `boxer_status`) and an Agent
   Skill that tells the agent what this repository expects. Works on any harness that speaks MCP,
   including one nobody has integrated. It asks rather than enforces.
2. **Hooks — enforcement.** One binary, `boxer hook <harness>`, speaks every harness's hook
   dialect. It provisions the VM at session start and rewrites intercepted commands on every tool
   call, so the agent never sees a refusal. Where a harness can only allow or deny, it denies with
   an error naming `boxer_run`, and the agent uses the tool.
3. **The plugin package.** `boxer package plugin` renders one
   [Agent Plugins 1.0.0](https://agent-plugins.org) package valid for every client at once, with
   the native manifests today's loaders read, so it installs everywhere now.
4. **Shell substitution — no integration at all.** `boxer shim install --shell` writes a shell
   binary that is really the sandbox. Point a harness's `shell_path` at it and its entire
   interactive shell runs in the guest — pipelines, compound lines and all. Nothing is recognised
   or rewritten, so nothing is missed.
5. **Inside mode.** `boxer shell claude`, `boxer acp gemini`: the harness itself runs in the VM.
   There is nothing to hook, because the agent is not on your machine.

Each one is explained, with the commands, in [docs/integrate.md](docs/integrate.md).

## Where each harness is verified

Every row below was run, not reasoned about: T1 is the real harness CLI against a scripted model
and a real VM, T2 is the same cells against live models, and adherence measures whether a live
model follows the brief when the prompt never mentions boxer. Full matrices, dates, costs and
every skip reason: [docs/status.md](docs/status.md).

| Harness | Outside | Inside | Verified at |
| --- | --- | --- | --- |
| Claude Code | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| Codex | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| Gemini CLI | rewrite, tool | `shell`, ACP | T1; T2 needs `GEMINI_API_KEY` |
| OpenCode | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| pi | rewrite, tool | `shell` (no ACP server) | T1, T2 live, adherence |
| Grok | rewrite (recommended), tool | `shell`, ACP | T1, T2 live, adherence |
| Kimi | tool; shims for the rest | `shell`, ACP | T1, T2 live, adherence |
| DSH | tool, through the Claude Code hook bridge | — | T1, T2 live |

Orchestrators — OpenHands, Paperclip, T3 Code, herdr, Conductor, Multica — have their own verified
paths in [docs/orchestrators.md](docs/orchestrators.md).

## Configuration, briefly

`boxer.toml` in the worktree, the repository, then `~/.config/boxer/`; earlier wins. A misspelled
key is an error, because silently ignoring one silently changes what is enforced; an unknown
top-level table is only a warning, so a repository that adopts a newer boxer's feature still loads
in an older one. Every scalar is also `BOXER_<KEY>` in the environment.

```toml
isolation   = "worktree"   # one VM per worktree; repo is wider, session and subagent narrower
mode        = "rewrite"    # rewrite | tool | off
intercept   = ["npm", "bun", "node", "python", "go", "make"]
passthrough = ["git", "gh", "ssh", "boxer"]
setup       = ["bun install"]

[tasks]                    # named commands the agent runs by name, not by composing a shell line
test  = "bun test"
build = "bun run build"
```

Named tasks are the deterministic path: `boxer run --task test` runs what the repository's
maintainers meant, and an unknown name is refused with the list of real ones. The skill boxer
ships calls them, so an agent does not have to guess a command line for the intercept list to
catch.

`boxer doctor` prints every resolved value and where it came from. The rest of the keys, and what
each one changes, are in [docs/configure.md](docs/configure.md).

## Documentation

**Using boxer:** [install](docs/install.md) · [configure](docs/configure.md) ·
[integrate](docs/integrate.md) · [troubleshoot](docs/troubleshooting.md) · [API](docs/api.md)

**Working on boxer:** [architecture](docs/architecture.md) ·
[evaluation plan](docs/eval-plan.md) · [requirements](docs/requirements.md) ·
[releasing and the stability contract](docs/release.md) · [CONTRIBUTING](CONTRIBUTING.md)

## Verification

boxer's claims are evaluated rather than asserted.

```sh
make test          # unit tests, race detector, coverage floor; a fake smolvm, safe any time
make smoke         # every configuration path against a real microVM, no model
make eval-t1       # every harness CLI against a scripted model and a real VM
make eval-t2       # the same cells against live models, a few cents
```

Last full run on Apple Silicon with smolvm 1.16.1 (2026-09-18): smoke 49/49; T1 61 pass, 0 fail,
2 skips, with two consecutive runs giving identical per-cell verdicts; T2 live 38 pass, 0 fail,
14 skips for $0.20; adherence 16 to 17 of 18 across four models, with no command reaching the host
in any of the 72 cells. Details and skip reasons: [docs/status.md](docs/status.md).

## Licence

Apache-2.0. See [LICENSE](LICENSE), [SECURITY.md](SECURITY.md) and
[CONTRIBUTING.md](CONTRIBUTING.md).
