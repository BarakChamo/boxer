# Vite

A runnable Vite dev server with a URL per worktree.

```sh
cp -R examples/vite /tmp/vite && cd /tmp/vite
git init -q && git add -A && git commit -qm init    # boxer keys a sandbox to a git worktree
boxer up && boxer url
```

Keep `--strictPort`: without it Vite moves to the next free port when 5173 is taken in the guest,
while the forward still points at 5173.
