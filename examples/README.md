# Examples

Working configurations for common projects. Each directory is either a **complete project** you
can run as it is, or a **config** to drop into an existing one; the table says which. Every
`boxer.toml` and `devcontainer.json` here is parsed by the test suite, so none of them can drift
out of date with the binary. Every one except `monorepo` has also been run on a real sandbox.

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

To run a runnable one, it needs to be its own git repository, because boxer keys a sandbox to a
worktree:

```sh
cp -R examples/go-api /tmp/go-api && cd /tmp/go-api
git init -q && git add -A && git commit -qm init
boxer up && curl -sk "$(boxer url)"
```

What every example has in common, because every one of these tripped someone:

- **Servers bind `0.0.0.0`**, never `127.0.0.1`: a forwarded port arrives on the guest's interface.
- **Ports are `auto:`**, never fixed, in anything committed: a fixed host port makes the second
  worktree fail to start.
- **`ready` is a real probe** of the server, so `boxer up` means "answering", not "launched".
- **The allowlist names the package registry** and nothing else; the image registry is allowed
  automatically.

The configuration reference is `site/content/docs/reference/configuration.mdx`, and the guides
behind these are under `site/content/docs/guides/`.
