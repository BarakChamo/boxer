# On docker or podman

The [Next.js config](../nextjs) on docker or podman instead of a microVM. Drop `boxer.toml` into a
Next.js project that has a lockfile.

```sh
boxer backends --probe       # creates, uses and deletes a small sandbox on each installed backend
boxer up
boxer url
```

Hooks, the MCP server, tasks, URLs, devcontainer support and `[prep]` work as they do on smolvm.
What changes is the boundary and what boxer can enforce through it:

| | smolvm | docker, podman |
| --- | --- | --- |
| boundary | a kernel per sandbox | one kernel shared by every sandbox |
| `network.mode = "allowlist"` | enforced, and the default | refused, so the file sets `mode = "on"` |
| environment packs, forks | yes | no, so `image_setup` runs in every new sandbox |

Gotchas:

- For podman, set `backend = "podman"`. `BOXER_BACKEND` overrides the backend for one command.
- On a Mac, create the podman machine with the `applehv` provider. `libkrun` cannot bind-mount host
  paths, so the worktree cannot be mounted.
