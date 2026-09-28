# Vite

A runnable Vite dev server on port 5173, with a URL per worktree.

```sh
cp -R examples/vite /tmp/vite && cd /tmp/vite
git init -q && git add -A && git commit -qm init    # boxer keys a sandbox to a git worktree
boxer up && boxer url
```

It shows `start` and `ready` for Vite, and `[urls]` with no server setting: Vite allows hosts under
`.localhost` by default.

Gotchas:

- Keep `--strictPort`. Without it Vite moves to the next free port when 5173 is taken in the guest,
  while the forward still points at 5173.
- There is no lockfile, so `setup` runs `npm install`. Switch to `npm ci` once you commit one.
