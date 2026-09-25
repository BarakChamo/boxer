# boxer

Coding agents run shell commands. boxer makes those commands run in a
[smolvm](https://smolmachines.com) microVM instead of on your machine — one VM per git worktree,
started automatically, with your worktree mounted in the guest at `/workspace`.

The agent does not have to know. It types `npm test`, the command runs in the sandbox, the output
comes back looking exactly as it would have. Nothing on your machine is at risk, and nothing about
the agent's experience changes.

One Go binary. smolvm holds the only state.

Full documentation: the site under [`site/`](site). `make docs-dev` serves it locally.

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
[site/content/docs/start/install.mdx](site/content/docs/start/install.mdx) has the details.

## How an agent ends up in the sandbox

Harnesses differ in what they let a third party do, so boxer has five ways in. They stack:
whichever ones your harness supports are active at once, and `boxer install <harness>` picks the
strongest without being asked.

1. **MCP and a skill — everywhere.** An MCP server (`boxer_run`, `boxer_status`) and an Agent
   Skill that tells the agent what this repository expects. Works on any harness that speaks MCP,
   including one nobody has integrated. It asks rather than enforces.
   An MCP server that must run beside the code — a dev server's own tools, say — runs *in* the
   sandbox, addressed by an ordinary entry: `"command": "boxer", "args": ["run", "--", "npx", "-y",
   "next-devtools-mcp@latest"]`. The harness stays on the host; nothing needs a bridge.
2. **Hooks — enforcement.** One binary, `boxer hook <harness>`, speaks every harness's hook
   dialect. It provisions the VM at session start and rewrites intercepted commands on every tool
   call, so the agent never sees a refusal. Where a harness can only allow or deny, it denies with
   an error naming `boxer_run`, and the agent uses the tool.
3. **The OpenCode plugin.** OpenCode loads a TypeScript plugin rather than exposing a hook API.
   The plugin does the same job in-process, and enforces the same way.
4. **PATH shims — no harness cooperation at all.** `boxer shim install` writes one file per entry
   in `intercept`, each of which re-runs itself in the guest. `boxer shim install --shell` writes
   `boxer-bash` instead, a shell that is really the sandbox: point a harness's shell setting at it
   and its whole interactive shell runs in the guest, pipelines and compound lines included.
   Nothing is recognised or rewritten, so nothing is missed. Shims hold where `PATH` does, which
   a login shell can undo.
5. **Inside mode.** `boxer shell claude`, `boxer acp gemini`: the harness itself runs in the VM.
   There is nothing to hook, because the agent is not on your machine.

Each one is explained, with the commands, in [site/content/docs/start/harnesses.mdx](site/content/docs/start/harnesses.mdx)
and [site/content/docs/guides/enforcement.mdx](site/content/docs/guides/enforcement.mdx).

`boxer package plugin` is a separate thing, and not a level: it renders one
[Agent Plugins 1.0.0](https://agent-plugins.org) package valid for every client at once, with the
native manifests today's loaders read, so the levels above install everywhere now.

## Where each harness is verified

Every row below was run, not reasoned about: T1 is the real harness CLI against a scripted model
and a real VM, T2 is the same cells against live models, and adherence measures whether a live
model follows the brief when the prompt never mentions boxer. Full matrices, dates, costs and
every skip reason: [docs/status.md](docs/status.md).

| Harness | Outside | Inside | Verified at |
| --- | --- | --- | --- |
| Claude Code | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| Codex | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| Gemini CLI | rewrite, tool | `shell`, ACP | T1; its live tier needs your own `GEMINI_API_KEY`, because Gemini CLI speaks only the Gemini API and the gateway the other harnesses share does not serve it |
| OpenCode | rewrite, tool | `shell`, ACP | T1, T2 live, adherence |
| pi | rewrite, tool | `shell` (no ACP server) | T1, T2 live, adherence |
| Grok | rewrite (recommended), tool | `shell`, ACP | T1, T2 live, adherence |
| Kimi | tool; shims for the rest | `shell`, ACP | T1, T2 live, adherence |
| GitHub Copilot CLI | rewrite, tool (user-level hooks) | `shell` (row present, not run) | T1, T2 live |
| DSH | tool, through the Claude Code hook bridge | — | T1, T2 live |

Orchestrators — OpenHands, Paperclip, T3 Code, herdr, Conductor, Multica — have their own verified
paths in [docs/orchestrators.md](docs/orchestrators.md).

## When a run fails

```sh
boxer run --junit junit.xml -- npm test   # which tests failed, not a log to scrape
boxer capsule new                         # the failure as a committable capsule.toml
boxer capsule replay capsule.toml         # does it still fail? exit 0 means yes
```

The worktree is mounted, not copied, so boxer reads the report the guest just wrote without
fetching anything. A capsule records what ran, against which commit, under which configuration,
and what outcome counts as reproducing it — enough for someone else to run, and small enough to
attach to an issue.

## Keeping and forking a sandbox

```sh
boxer pack save base       # keep this prepared guest; gc never touches a named pack
boxer fork --prepare       # make this sandbox a branch source
boxer fork --count 4       # four copy-on-write children, warm, no boot
```

A fork starts from the parent's memory and disks rather than booting, which is what makes four
warm workers cheap. They share this worktree: smolvm cannot branch a staged mount, so a fork is
for work that shards cleanly rather than for isolated copies — a second git worktree is still
how you get one of those.

## Watching what is running

```sh
boxer ls -A              # every sandbox on every backend: branch, clean/dirty/gone, where it serves
boxer ls --resources     # the same with memory and disk; --pr adds each worktree's pull request
boxer rm --gone          # remove sandboxes whose worktree was deleted; rm -i to choose
boxer backends --probe   # which runtimes are installed, answering, and actually able to run a sandbox
boxer integrations       # which harnesses and orchestrators are here, and whether boxer is wired in
boxer url                # where this worktree's dev server is
boxer watch              # a live stream: created, running, stopped, gone, and events as they happen
boxer doctor             # this worktree: what is resolved, what is cached, what is free, what egress was denied
boxer gc --all           # reclaim every stopped sandbox and unreferenced pack
```

A sandbox costs about 700 MB of disk and around a gigabyte resident, against a default allocation
of 4 GB, so boxer reclaims after itself: any command that provisions one starts a background
sweep, at most every six hours, and refuses to cache an image when free space is under five
gigabytes. `boxer watch --json` is one JSON document per line, which is what a
dashboard would read.

Output is for whoever is reading it: colour, tables and a question before anything destructive at
a terminal; plain text and never a prompt for an agent or a pipe. `--json`, or `BOXER_OUTPUT=json`,
on anything with a JSON form.

## Configuration, briefly

`boxer.toml` in the worktree, the repository, then `~/.config/boxer/`; earlier wins. A misspelled
key is an error, because silently ignoring one silently changes what is enforced; an unknown
top-level table is only a warning, so a repository that adopts a newer boxer's feature still loads
in an older one. Fifteen scalar settings are also readable as `BOXER_<KEY>`, for harnesses that
offer no other way to configure a subprocess; the lists are repository policy and are not.

A `.devcontainer/devcontainer.json`, if the repository already has one, supplies the image,
lifecycle commands, ports, environment and bind mounts; `boxer.toml` overrides it and `boxer doctor`
says which file each value came from.

```toml
isolation   = "worktree"   # one VM per worktree; repo is wider, session and subagent narrower
mode        = "rewrite"    # rewrite | tool | off
intercept   = ["npm", "bun", "node", "python", "go", "make"]   # abridged; the real default is longer
passthrough = ["git", "gh", "ssh", "boxer", "smolvm"]
image       = ""                # default: detected from the lockfile, else debian:bookworm-slim
setup       = ["bun install"]   # once per worktree; image_setup is the per-image half

[tasks]                         # named commands the agent runs by name, not by composing a shell line
test  = "bun test"
build = "bun run build"

[network]
mode        = "allowlist"       # registry hosts for the image are always allowed
allow_hosts = ["registry.npmjs.org"]
[worktree]
manage      = "off"             # detect: a session in the main checkout shares the repository VM until it enters a worktree
[harness.gemini-cli]
mode        = "tool"
[telemetry]
enabled     = false             # the event stream, off by default; sink = none | file | stderr | otel
```

Named tasks are the deterministic path: `boxer run --task test` runs what the repository's
maintainers meant, and an unknown name is refused with the list of real ones. The skill boxer
ships calls them, so an agent does not have to guess a command line for the intercept list to
catch.

## Telemetry

Off by default: with no `[telemetry]` table, boxer writes no log, no metrics and nothing to the
network. Turn it on with `enabled = true` and read it back with `boxer logs`; `boxer status --json`
carries the last few events for the scope. Command lines are elided unless `record_commands = true`,
and nothing leaves the machine unless you build with `-tags otel` and set an `endpoint`. The schema,
the event names and the redaction rules are in [site/content/docs/reference/events.mdx](site/content/docs/reference/events.mdx).

`boxer install git` adds a `post-checkout` hook (honouring `core.hooksPath`) that runs
`boxer up --detach` in every worktree `git worktree add` creates, so the VM is warm before any
agent opens it. Opt-in; `boxer install all` leaves git configuration alone.

`boxer doctor` prints every resolved value and where it came from. The rest of the keys, and what
each one changes, are in [site/content/docs/reference/configuration.mdx](site/content/docs/reference/configuration.mdx).

## Documentation

**Using boxer:** [install](site/content/docs/start/install.mdx) · [configure](site/content/docs/reference/configuration.mdx) ·
[integrate](site/content/docs/start/harnesses.mdx) · [tasks](site/content/docs/guides/tasks.mdx) ·
[run the harness inside](site/content/docs/guides/inside.mdx) ·
[security model](site/content/docs/concepts/security.mdx) ·
[troubleshoot](site/content/docs/guides/troubleshooting.mdx)

**Building on boxer:** [the three interfaces](site/content/docs/guides/building-on-boxer.mdx) ·
[JSON output](site/content/docs/reference/json.mdx) · [MCP](site/content/docs/reference/mcp.mdx) ·
[Go package](site/content/docs/reference/go.mdx)

**Working on boxer:** [architecture](docs/architecture.md) · [testing](docs/testing.md) ·
[adding a harness](docs/adding-a-harness.md) · [evaluation plan](docs/eval-plan.md) ·
[requirements](docs/requirements.md) ·
[releasing and the stability contract](docs/release.md) · [CONTRIBUTING](CONTRIBUTING.md)

## Verification

boxer's claims are evaluated rather than asserted.

```sh
make test          # unit tests, race detector, coverage floor; a fake smolvm, safe any time
make smoke         # every configuration path against a real microVM, no model
make eval-t1       # every harness CLI against a scripted model and a real VM
make eval-t2       # the same cells against live models, a few cents
```

Last full run on Apple Silicon with smolvm 1.16.1 (2026-09-18): smoke 53/53; T1 67 pass, 0 fail,
1 skip, run twice with identical per-cell verdicts; T2 live 42 pass, 0 fail, 14 skips for $0.19; adherence 80 of 96 live cells across four
models, with one harness verdict (Grok, a known finding) and no command reaching the host. One cell, Codex over ACP inside the guest,
failed once in nine runs and passed on every repeat; it is named in
[docs/status.md](docs/status.md) along with every skip reason.

## Licence

Apache-2.0. See [LICENSE](LICENSE), [SECURITY.md](SECURITY.md) and
[CONTRIBUTING.md](CONTRIBUTING.md).
