# Architecture

boxer is one Go binary and keeps no state it cannot lose. smolvm holds the microVMs, their images
and the packs; git holds the code; the binary holds nothing between invocations.

Under `$XDG_STATE_HOME/boxer` it keeps a handful of caches — `last-used`, `packs`, `locks`,
`events.jsonl`, `runs`, the `branchable`/`staged` markers, `owned` for a backend
that cannot label a machine, and `urls` for the names registered with portless. Every one of them
can be deleted at any moment; the cost is a delayed gc, a slower first build, a forgotten last
run, a fork that has to be prepared again, or a route that stays in portless until removed by hand.
None of them is ever read to decide what a command does, which is the property that keeps the
claim honest.

They are also removed. Deleting a sandbox removes everything that describes it; `gc` sweeps
anything belonging to no sandbox after a week, because a sandbox can be deleted by something that
is not boxer. The setup and prep markers are not here at all: they describe the worktree, so they
live in its git directory (`.git/boxer`, or `.git/worktrees/<name>/boxer`) and go when it does —
which is what makes a worktree recreated at the same path get set up again.

## The rule that shapes everything

**The core knows nothing about any harness.** `internal/{box,vm,scope,config,decide,shim}` and
`pkg/boxer` must not contain a harness name; a harness is a row in a table — a hook dialect
(`internal/hook`), an inside-mode row (`internal/inside`), or a template namespace
(`internal/bundle/templates`). `TestCoreNamesNoHarness` parses those packages and fails if a name
appears, so supporting a tenth harness cannot grow the core. Adding one means a dialect row, an
install case, a template namespace and an eval driver, and nothing else.

## The pieces

| Package | Owns |
| --- | --- |
| `internal/scope` | the identity of a sandbox: `sha256(worktree path)`, widened or narrowed by `isolation` |
| `internal/config` | `boxer.toml` and `BOXER_*`, merged across three locations; an unknown key is rejected, an unknown table warns |
| `internal/decide` | given a command and a configuration, what happens to it: run here, run there, deny |
| `internal/box` | the lifecycle above a VM: provision, setup, pack, run, reclaim |
| `internal/vm` | the smolvm process boundary, and nothing else |
| `internal/hook` | one dialect row per harness, translating each one's hook protocol |
| `internal/install` | writing a repository's configuration, merged and idempotent |
| `internal/bundle` | rendering the published skill and plugin package, from a version and nothing else |
| `internal/obs` | the event stream and its sinks; off unless `[telemetry]` turns it on |
| `internal/inside` | running a harness inside the guest: `boxer shell`, `boxer acp` |
| `internal/junit` | parsing JUnit XML from the mounted worktree into a run's test summary |
| `internal/mcp` | the MCP server: `boxer_run`, `boxer_status`, lifecycle signals |
| `internal/shim` | programs on PATH that are really boxer |
| `pkg/boxer` | the only importable package: a thin facade over the above, with no logic |

## How a command reaches the guest

1. A scope is resolved from the worktree path and the `isolation` setting. It names one VM.
2. The VM is provisioned on first use: image chosen from the lockfile if `image` is unset, `setup`
   run inside it once, the result packed so the next worktree starts from the pack.
3. The harness runs a shell command. Its hook, a PATH shim, the substituted shell, or the MCP tool
   — whichever levels are active — hands the command to boxer.
4. `decide` says run-in-guest, pass-through, or deny, from `intercept`, `passthrough` and `mode`.
5. The command runs in the guest with the worktree mounted at `mount_at` — `/workspace` by
   default, the host path under `integration = "inside"`, where the harness itself is in the guest
   and every path it prints has to be valid on both sides.
6. A refusal is `boxer: <reason>` with `scope`, `worktree`, `cause` and a runnable `fix:` line.
   Errors are instructions: the agent reads the fix and proceeds.

## One command, through the packages

The path a rewritten `npm test` takes, in order, so a change can be located by where in this list
it belongs.

```
harness
  │  PreToolUse hook: {"tool_name":"Bash","tool_input":{"command":"npm test"}}
  ▼
cmd/boxer hook claude-code
  │
  ▼
internal/hook          dialect row → normalise the event to a purpose
  │
  ├─► internal/scope    git rev-parse → sha256(worktree) → sb-7e1852e4a3c3
  ├─► internal/config   devcontainer → user → repo → worktree → BOXER_*
  │
  ▼
internal/decide        intercept? passthrough? mode? → Rewrite("boxer run -c 'npm test'")
  │
  ▼  the harness runs the rewritten line, which re-enters boxer:
cmd/boxer run -c 'npm test'
  │
  ▼
internal/box           Ensure: exists? running? pack? image_setup? setup? start? ready?
  │
  ▼
internal/vm            smolvm machine exec …
  │
  ▼
guest                  sh -c 'npm test' in /workspace/<mapped cwd>
```

`internal/obs` observes each step and writes nothing unless telemetry is on.

Two properties of this path are worth stating because they constrain changes to it:

**`decide` is pure and has no VM state.** It answers "what should happen to this command" from the
command and the configuration, and nothing else. Provisioning and the failure policy belong to
`boxer run`, which is downstream. This is what lets one tested function serve nine hook dialects.

**The rewrite re-enters through the CLI, not through a library call.** The hook emits a command
line; the harness runs it; that invocation resolves its own scope and configuration. It costs a
process and buys the property that every level — hook, shim, substituted shell, MCP tool — converges
on the same code path, so a bug cannot exist at one level and not another.

## Two placements

**Outside** — the default — runs the harness on the host and sandboxes the commands it issues.
**Inside** runs the harness itself in the guest, where there is nothing to intercept. They share
the scope, the image and the packs; they differ only in where the agent process lives.

## The backend interface, and what it cost to get one

There are now four backends — smolvm, Apple's `container`, docker and podman — behind a
`vm.Backend` interface, and
the interface was written *after* the second implementation rather than before it, which is what
this section used to say would happen.

The shape is a small core plus optional interfaces found by type assertion: nine methods every
backend must have, and `Packer`, `Brancher`, `EgressReporter` and `DiskReporter` for what only
some can do. A fat interface was rejected on purpose — it would make a container backend stub
`Branch` and return "unsupported", and a backend that *forgets* to refuse would still compile.
With optional interfaces, "cannot do this" is a fact the type system holds.

Two things this surfaced that were previously invisible:

- **A container backend cannot enforce `network.mode = "allowlist"`**, boxer's default. It refuses
  rather than running wide open. That is the difference between the backends stated plainly, and
  it is why `boxer doctor` prints the boundary — one kernel per sandbox, or one shared — before
  anything else.
- **Host-wide commands were hard-coded to smolvm.** `ls`, `gc`, `down --all` and `watch` asked
  smolvm for machines regardless of configuration, so on another backend `gc` reclaimed nothing
  while containers accumulated. Running the existing smoke suite against a second backend found
  it in minutes; no amount of reading would have.

The contract every backend answers is [adding-a-backend.md](adding-a-backend.md), written before
the interface and unchanged by it — which is the point of having written it first. The one rule
still untested is the fourth, workspace transport: all three backends mount the worktree, so
nothing here has yet had to answer what happens when a backend copies it instead.

## Where the evidence lives

boxer does not assert support; it measures it. `internal/eval` is the harness, `internal/fakellm`
the scripted model, `internal/vmtest` the fake hypervisor, and the tiers are described in
[eval-plan.md](eval-plan.md). What holds today, with dates and skip reasons, is
[status.md](status.md).

Full specification: [requirements.md](requirements.md). Stable surface and what may change
without notice: [release.md](release.md). How the layers of testing fit together:
[testing.md](testing.md). Adding a tenth harness: [adding-a-harness.md](adding-a-harness.md).
Adding a second backend, when one exists: [adding-a-backend.md](adding-a-backend.md).
