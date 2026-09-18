# What the boxer sandbox is

boxer runs commands in a [smolvm](https://smolmachines.com) microVM. One sandbox exists per scope
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

## Reading a refusal

```
boxer: no sandbox exists for this scope
  scope:     sb-7e1852e4a3c3 (worktree)
  worktree:  /Users/me/src/demo
  cause:     NO_SANDBOX
  fix:       boxer up
```

The `fix:` line is runnable as printed. `cause` is stable and machine-readable; the prose is not.
