# On docker or podman

The Next.js config on docker (or podman) instead of a microVM. Drop `boxer.toml` into a Next.js
project. Hooks, the MCP server, tasks, URLs, devcontainer support and `[prep]` work the same; the
boundary and what boxer can enforce through it change:

| | smolvm | docker / podman |
| --- | --- | --- |
| boundary | a kernel per sandbox | one kernel shared by every sandbox |
| `network.mode = "allowlist"` | yes, the default | refused — choose `on` or `off` |
| environment packs, forks | yes | no |

```sh
boxer backends --probe       # creates, uses and deletes a small sandbox on each installed backend
boxer up
```

For podman, set `backend = "podman"`. On a Mac, create the podman machine with the `applehv`
provider: `libkrun` cannot bind-mount host paths, so the worktree cannot be mounted.
