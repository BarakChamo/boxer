# Next.js

A Next.js dev server per worktree, each at its own URL.

```sh
npx create-next-app@latest myapp && cd myapp
cp path/to/boxer/examples/nextjs/boxer.toml .
cp path/to/boxer/examples/nextjs/next.config.ts .     # or merge allowedDevOrigins into yours
npm i -g portless                                        # optional: the URLs
boxer up
boxer url                                                # https://myapp.localhost:1355
```

In a second worktree (`git worktree add ../myapp-fix-ui -b fix-ui`), `boxer up` gives
`https://fix-ui.myapp.localhost:1355`, and the two never share `node_modules`, a port or a `.next`.

`boxer.toml` comments explain each line. Two are required:

- `-H 0.0.0.0` on `next dev`. Bound to loopback, the forwarded port reaches nothing.
- `allowedDevOrigins` in `next.config.ts`. `*.localhost` covers every worktree's name.

Using a devcontainer instead? See [`../nextjs-devcontainer`](../nextjs-devcontainer).
