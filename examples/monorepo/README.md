# Monorepo: two services, named ports, tasks

A config for an npm workspace with `web` (Next.js on 3000) and `api` (on 8080), started together in
one sandbox, each forwarded to its own host port and named through `urls.names`. Drop `boxer.toml`
at the workspace root. It is the one example that has not been run on a real sandbox.

```sh
boxer up
boxer url 8080
boxer run --task test
```

On a worktree on branch `fix-ui` of a repository named `myrepo`:

```console
$ boxer up
boxer: url: https://fix-ui.web.myrepo.localhost:1355 -> guest port 3000
boxer: url: https://fix-ui.api.myrepo.localhost:1355 -> guest port 8080
$ boxer url 8080
https://fix-ui.api.myrepo.localhost:1355
```

It shows two `start` lines, one `ready` command that waits for both, `urls.names`, and tasks with
`description`, `junit` and `timeout`. `boxer run --task test` summarises the two JUnit reports it
names.

Gotchas:

- Inside the guest both services are on `127.0.0.1`, so the web app's server-side code calls
  `http://127.0.0.1:8080`. Only a browser on the host needs the names.
- Each workspace's `dev` script must accept the `--host`/`--hostname` and `--port` flags the
  `start` lines pass, and bind `0.0.0.0`.
- Two dev servers need more memory than one, so the file sets `memory = "6G"`.
