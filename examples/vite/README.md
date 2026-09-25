# Vite

```sh
cd examples/vite
git init -q && git add -A && git commit -qm init    # boxer keys a sandbox to a git worktree
boxer up && boxer url
```

`--strictPort` is the line worth copying: without it Vite moves to the next free port when 5173 is
taken in the guest, and the forward is still pointing at 5173.
