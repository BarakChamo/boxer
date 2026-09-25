# What the boxer sandbox is

boxer runs commands in a sandbox: a [smolvm](https://smolmachines.com) microVM by default, or
Apple `container`, docker or podman when the repository chooses one — `boxer doctor` says which. One sandbox exists per scope
— by default per git worktree, so two branches checked out side by side never share a build cache,
a `node_modules`, or a port. The worktree is mounted into the guest read-write; everything else on
the host is not there.

Run `boxer brief` for this checkout's resolved facts (mount path, mode, intercepted programs) and
`boxer doctor` for where each of those values came from.

## What is shared and what is not

- **Shared:** the worktree. Files written by a command in the sandbox appear on the host at once,
  and the other way round.
- **Not shared:** the host's installed toolchains, the host's home directory, host network access
  beyond the configured allowlist, and anything outside the worktree.

A command that fails because a tool is missing is telling you the sandbox image lacks it: add it
to the repository's `setup` list in `boxer.toml` rather than installing it on the host.

## The three ways in

1. **Named tasks** — `boxer run --task <name>`, declared in `boxer.toml`'s `[tasks]` table. The
   deterministic path: the name is the contract, not the shell line.
2. **`boxer run`** — `boxer run -c '<shell line>'` for a shell line, `boxer run -- <prog> [args]`
   for one program.
3. **Transparent interception** — in `rewrite` mode a hook rewrites intercepted commands into the
   sandbox as they are issued, so ordinary shell use is already sandboxed. In `tool` mode the
   shell path is refused and you must use one of the two above.

`boxer status` reports whether the sandbox exists and is running; the first command creates it, so
a cold start takes longer than the ones after it.

## Servers and ports

A port the sandbox forwards lands on a host port chosen per worktree, so two worktrees can both
run a server on 3000. `boxer status --json` says where: `ports` maps each guest port to its host
port, and `urls`, when the repository enables `[urls]`, maps it to a stable name served by
[portless](https://github.com/vercel-labs/portless) — `https://<branch>.<repo>.localhost`, or the
sandbox's own two-word name in place of the branch for a detached worktree. Prefer the URL; it
does not change when the sandbox restarts. From inside the sandbox none of that applies: the server is at
`127.0.0.1:<port>` there, and `$BOXER_URLS` / `$BOXER_PORTS` carry the host-side addresses.

## Reading a refusal

```
boxer: no sandbox exists for this scope
  scope:     sb-7e1852e4a3c3 (worktree)
  worktree:  /Users/me/src/demo
  cause:     NO_SANDBOX
  fix:       boxer up
```

The `fix:` line is runnable as printed. `cause` is stable and machine-readable; the prose is not.
