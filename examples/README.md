# Examples

Working configurations for common projects. Each directory is either a runnable project or a
config to drop into an existing one. The test suite parses every `boxer.toml` and
`devcontainer.json` here, and every example except `monorepo` has been run on a real sandbox.

| example | kind | shows |
| --- | --- | --- |
| [`nextjs`](nextjs) | config | a Next.js dev server per worktree, `[urls]`, `allowedDevOrigins`, tasks |
| [`nextjs-devcontainer`](nextjs-devcontainer) | config | the same from a `devcontainer.json` alone: users, labels, variables |
| [`vite`](vite) | runnable | Vite with `--strictPort`, and a name per worktree |
| [`python-fastapi`](python-fastapi) | runnable | uv: installing a tool (`image_setup`) versus a project (`setup`) |
| [`go-api`](go-api) | runnable | a Go service, tasks for test and vet |
| [`monorepo`](monorepo) | config | two services in one sandbox, named ports, JUnit-reporting tasks |
| [`host-prep`](host-prep) | config | installing on the host with `[prep]` and `$BOXER_TARGET_FLAGS`, and when not to |
| [`docker-backend`](docker-backend) | config | the same project on docker or podman, and what that gives up |

A runnable example must be its own git repository, because boxer keys a sandbox to a worktree:

```sh
cp -R examples/go-api /tmp/go-api && cd /tmp/go-api
git init -q && git add -A && git commit -qm init
boxer up && curl -sk "$(boxer url)"
```

Every example follows these rules:

- Servers bind `0.0.0.0`, not `127.0.0.1`: a forwarded port arrives on the guest's interface.
- Ports are `auto:`, never fixed, in anything committed: a fixed host port makes the second
  worktree fail to start.
- `ready` probes the server, so `boxer up` returns when it answers, not when it was launched.
- The allowlist names only the package registry; the image registry is allowed automatically.

Configuration reference: `site/content/docs/reference/configuration.mdx`. Guides:
`site/content/docs/guides/`.
