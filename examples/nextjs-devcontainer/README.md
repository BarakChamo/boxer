# Next.js from a devcontainer

A repository that already has `.devcontainer/devcontainer.json` keeps it as the source of truth.
boxer reads the image, `postCreateCommand` (as `setup`), `postStartCommand` (as `start`),
`forwardPorts` (one free host port per worktree), `remoteUser`, `containerEnv`/`remoteEnv` and
`hostRequirements`, and resolves `${localWorkspaceFolder}`, `${containerWorkspaceFolder}` and
`${localEnv:NAME}` on the host.

The `boxer.toml` beside it holds only what devcontainer has no words for:

- **the allowlist.** boxer's default network lets the guest reach the image registry and nothing
  else, and devcontainer has no key for a list of hosts, so `npm ci` needs `registry.npmjs.org`
  named here. Without it the install fails and boxer's error names the refused host and this line.
  On a container backend, which has no allowlist, set `network.mode = "on"` instead.
- **`[urls]`**, for the `web.<repo>.localhost` name the port's label asks for.

```sh
boxer doctor     # every value, and which file it came from
boxer up && boxer url
```

What boxer will not do — `features`, `build`, `dockerComposeFile`, and settings that would widen the
sandbox such as `runArgs` or `privileged` — it names in a warning rather than silently skipping.

There is no `ready` probe (devcontainer has no equivalent), so `boxer up` returns as soon as the
server is launched. Add `ready` to `boxer.toml` if `up` should wait for HTTP 200.
