# Next.js from a devcontainer

The [Next.js](../nextjs) setup with `devcontainer.json` as the source of truth. Copy
`.devcontainer/` and `boxer.toml` into a Next.js project.

```sh
boxer doctor     # every value, and which file it came from
boxer up
boxer url        # https://web.<repo>.localhost:1355
```

boxer reads the image, `postCreateCommand` as `setup`, `postStartCommand` as `start`,
`forwardPorts` (a free host port per worktree), `portsAttributes` labels, `remoteUser`,
`containerEnv` and `remoteEnv`, and `hostRequirements`, and resolves `${localWorkspaceFolder}`,
`${containerWorkspaceFolder}` and `${localEnv:NAME}` on the host.

The `boxer.toml` beside it holds what devcontainer has no key for:

- **The allowlist.** boxer's default network reaches only the image registry, so
  `registry.npmjs.org` is named here for `npm ci`.
- **`[urls]`**, which names the port `web.<repo>.localhost` after its label.

Gotchas:

- On docker, podman or Apple `container`, which refuse the allowlist, set `network.mode = "on"`
  in `boxer.toml` instead.
- devcontainer has no `ready` equivalent, so `boxer up` returns once the server is launched. Add
  `ready` to `boxer.toml` to wait for it to answer.
- boxer warns about keys it will not honour: `features`, `build`, `dockerComposeFile`, and settings
  that would widen the sandbox, such as `runArgs` or `privileged`.
