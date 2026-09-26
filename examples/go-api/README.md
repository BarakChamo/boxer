# Go

A runnable Go HTTP service with `test` and `vet` tasks.

```sh
cp -R examples/go-api /tmp/go-api && cd /tmp/go-api
git init -q && git add -A && git commit -qm init
boxer up
curl -sk "$(boxer url)"
boxer run --task test
```

`go run .` compiles on every start (a second or two here). Do not build into `/tmp` from `setup`:
`setup` runs once per worktree, the guest's `/tmp` is emptied when the sandbox stops, and the next
start would find no binary.
