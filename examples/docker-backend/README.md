# On docker or podman

Everything above the backend — hooks, the MCP server, tasks, URLs, devcontainer support, `[prep]` —
works the same. What changes is the boundary and what boxer can enforce through it:

| | smolvm | docker / podman |
| --- | --- | --- |
| boundary | a kernel per sandbox | one kernel shared by every sandbox |
| `network.mode = "allowlist"` | yes, the default | refused — choose `on` or `off` |
| environment packs, forks | yes | no |

```sh
boxer backends --probe       # proves docker can actually run a sandbox here
boxer up
```

On a Mac with podman, create the machine with the `applehv` provider: the `libkrun` provider cannot
bind-mount host paths, so no worktree can be mounted.
