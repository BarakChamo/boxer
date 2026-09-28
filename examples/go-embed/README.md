# Embedding boxer in Go

A Go program that uses `pkg/boxer` to run one shell line in several worktrees at once, each in its
own sandbox, and prints every exit code. It is the loop an orchestrator runs.

Run it from a boxer checkout, with each argument after the command a git worktree:

```sh
go run ./examples/go-embed -- 'npm test' ~/code/app-wt1 ~/code/app-wt2
go run ./examples/go-embed -down -- 'npm test' ~/code/app-wt1 ~/code/app-wt2   # delete each sandbox after
```

```
/Users/me/code/app-wt1                   ok
/Users/me/code/app-wt2                   exit 1
    FAIL src/cart.test.ts
```

It exits 1 when any worktree fails. What it shows, in [`main.go`](main.go):

- `boxer.Open(dir, boxer.Options{...})` resolves the sandbox for a directory. A refusal is a
  `*boxer.Error`; match on its `Cause` and show its `Fix`.
- `Ensure(true, false)` creates the sandbox if it does not exist and starts it if it is stopped. On
  a running sandbox it does nothing, so the caller tracks no state.
- `Run` returns the guest command's exit code. A non-nil error means the command did not run.
- `Down` deletes the sandbox. Leave it out to keep sandboxes warm between runs; `boxer gc` and
  `auto_reclaim` remove idle ones later.

Gotchas:

- A directory that is not in a git repository fails with `NO_REPOSITORY`.
- Each worktree reads its own `boxer.toml`, so image, setup and ports come from that repository.
  The first run in a worktree pays for creating its sandbox.
- Flags go before `--`: `-down -- 'cmd' dir...`.

To use the package in your own module, `go get github.com/BarakChamo/boxer`. The
[Go reference](../../site/content/docs/reference/go.mdx) lists every function.
