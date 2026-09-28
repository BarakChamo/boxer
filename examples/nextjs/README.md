# Next.js

A Next.js dev server per worktree, each at its own URL. Drop `boxer.toml` and `next.config.ts` into
an existing Next.js project.

```sh
npx create-next-app@latest myapp && cd myapp
cp path/to/boxer/examples/nextjs/boxer.toml .
cp path/to/boxer/examples/nextjs/next.config.ts .     # or merge allowedDevOrigins into yours
npm i -g portless                                     # optional: the URLs
boxer up
boxer url                                             # https://myapp.localhost:1355
```

In a second worktree (`git worktree add ../myapp-fix-ui -b fix-ui`), `boxer up` gives
`https://fix-ui.myapp.localhost:1355`, with its own `node_modules`, host port and `.next`.

It shows `setup`, `start` and `ready` for a dev server, an `auto:` port, `[urls]`, the allowlist,
and `test` and `build` tasks. The comments in `boxer.toml` explain each line.

Gotchas:

- `-H 0.0.0.0` on `next dev` is required. Bound to loopback, the server never sees the forwarded
  port.
- `allowedDevOrigins` in `next.config.ts` stops Next.js 15 warning about cross-origin dev chunks.
  `*.localhost` covers every worktree's name.
- `npm ci` needs a `package-lock.json`. Use `npm install` in `setup` if there is none.
- Do not run the `build` task while the dev server is serving the same `.next`.

Using a devcontainer instead? See [`../nextjs-devcontainer`](../nextjs-devcontainer).
