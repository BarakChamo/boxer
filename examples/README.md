# Examples

Working configurations for common projects. Each directory is a runnable project, a config to drop
into an existing one, or a program that uses boxer as a library. The test suite loads every
`boxer.toml` and `devcontainer.json` here, and every `boxer.toml` except `monorepo`'s has been run
on a real sandbox.

| example | kind | shows |
| --- | --- | --- |
| [`nextjs`](nextjs) | config | a Next.js dev server per worktree, `[urls]`, `allowedDevOrigins`, tasks |
| [`nextjs-devcontainer`](nextjs-devcontainer) | config | the same from `devcontainer.json` plus a short `boxer.toml`: users, port labels, variables |
| [`vite`](vite) | runnable | Vite with `--strictPort`, and a name per worktree |
| [`python-fastapi`](python-fastapi) | runnable | uv: installing a tool (`image_setup`) versus a project (`setup`) |
| [`go-api`](go-api) | runnable | a Go service, tasks for test and vet |
| [`monorepo`](monorepo) | config | two services in one sandbox, named ports, JUnit-reporting tasks |
| [`host-prep`](host-prep) | config | installing on the host with `[prep]` and `$BOXER_TARGET_FLAGS`, and when not to |
| [`docker-backend`](docker-backend) | config | the same project on docker or podman, and what that gives up |
| [`go-embed`](go-embed) | program | `pkg/boxer` running one command in several worktrees at once |

A runnable example must be its own git repository, because boxer keys a sandbox to a worktree:

```sh
cp -R examples/go-api /tmp/go-api && cd /tmp/go-api
git init -q && git add -A && git commit -qm init
boxer up && curl -sk "$(boxer url)"
```

Every example follows these rules:

- Servers bind `0.0.0.0`. A forwarded port arrives on the guest's interface, so a server on
  `127.0.0.1` never sees it.
- Ports are `auto:`. A fixed host port makes the second worktree fail to start.
- `ready` probes the server, so `boxer up` returns when the server answers.
- The allowlist names only the package registry. The image registry is allowed automatically.

The [configuration reference](../site/content/docs/reference/configuration.mdx) lists every key,
and [the guides](../site/content/docs/guides/) cover each task.
