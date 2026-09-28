# Go

A runnable Go HTTP service on port 8080, with `test` and `vet` tasks.

```sh
cp -R examples/go-api /tmp/go-api && cd /tmp/go-api
git init -q && git add -A && git commit -qm init    # boxer keys a sandbox to a git worktree
boxer up
curl -sk "$(boxer url)"      # served from a boxer sandbox, as …localhost:1355
boxer run --task test
```

It shows `start` compiling and running a service, `ready` probing it with `wget` (the alpine image
has no curl), and two tasks.

Gotchas:

- `go run .` compiles on every start, a second or two here. Do not build into `/tmp` from `setup`
  instead: `setup` runs once per worktree, the guest's `/tmp` is emptied when the sandbox stops, and
  the next start finds no binary.
- The module has no dependencies. With some, add `go mod download` to `setup` and
  `"proxy.golang.org"` and `"sum.golang.org"` to `network.allow_hosts`.
