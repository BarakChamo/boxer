# Next.js from a devcontainer

Copy `.devcontainer/` and `boxer.toml` into a Next.js project; `devcontainer.json` stays the source
of truth. boxer reads the image, `postCreateCommand` (as `setup`), `postStartCommand` (as `start`),
`forwardPorts` (one free host port per worktree), `remoteUser`, `containerEnv`/`remoteEnv` and
`hostRequirements`, and resolves `${localWorkspaceFolder}`, `${containerWorkspaceFolder}` and
`${localEnv:NAME}` on the host.

The `boxer.toml` beside it holds only what devcontainer has no words for:

- **The allowlist.** boxer's default network reaches only the image registry, and devcontainer has
  no key for allowed hosts, so `registry.npmjs.org` is named here for `npm ci`. On a container
  backend, which has no allowlist, set `network.mode = "on"` instead.
- **`[urls]`**, for the `web.<repo>.localhost` name from the port's label.

```sh
boxer doctor     # every value, and which file it came from
boxer up && boxer url
```

boxer warns about keys it will not honour: `features`, `build`, `dockerComposeFile`, and settings
that would widen the sandbox such as `runArgs` or `privileged`.

devcontainer has no `ready` equivalent, so `boxer up` returns once the server is launched. Add
`ready` to `boxer.toml` to wait for HTTP 200.
