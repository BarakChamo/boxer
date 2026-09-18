# Architecture

boxer is one Go binary and no state of its own. smolvm holds the microVMs, their images and the
packs; git holds the code; the binary holds nothing between invocations.

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
5. The command runs in the guest with the worktree mounted at its host path, so paths in output
   mean the same thing on both sides.
6. A refusal is `boxer: <reason>` with `scope`, `worktree`, `cause` and a runnable `fix:` line.
   Errors are instructions: the agent reads the fix and proceeds.

## Two placements

**Outside** — the default — runs the harness on the host and sandboxes the commands it issues.
**Inside** runs the harness itself in the guest, where there is nothing to intercept. They share
the scope, the image and the packs; they differ only in where the agent process lives.

## Why there is no backend abstraction

There is one backend, smolvm, and an interface with one implementation is a liability. A
`Backend` interface for Firecracker or Docker Sandboxes is recorded as out of scope for 1.0 and
will be introduced when a second implementation actually exists.

Full specification: [requirements.md](requirements.md). Stable surface and what may change
without notice: [release.md](release.md).
