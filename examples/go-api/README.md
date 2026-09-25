# Go

```sh
cd examples/go-api
git init -q && git add -A && git commit -qm init
boxer up
curl -sk "$(boxer url)"
boxer run --task test
```

`go run .` compiles on every start, which for a small service is a second or two. Do not move
the build into `setup` and the binary into `/tmp`: `setup` runs once per worktree, the guest's
`/tmp` is emptied when the sandbox stops, and the next start would find no binary.
