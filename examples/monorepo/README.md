# Monorepo: two services, named ports, tasks

A config for an npm workspace with `web` (Next.js, 3000) and `api` (8080), started together in one
sandbox, each forwarded to its own host port and named via `urls.names`. Output below is from a
worktree on branch `fix-ui` of a repo named `myrepo`.

```console
$ boxer up
boxer: url: https://fix-ui.web.myrepo.localhost:1355 -> guest port 3000
boxer: url: https://fix-ui.api.myrepo.localhost:1355 -> guest port 8080
$ boxer url 8080
https://fix-ui.api.myrepo.localhost:1355
```

Inside the guest both services are on `127.0.0.1`, so the web app's server-side code calls
`http://127.0.0.1:8080`. Only a browser on the host needs the names.

The agent's brief lists each `[tasks]` entry with its description, so an agent runs
`boxer run --task test` instead of guessing a command line. A task with `junit` reports which tests
failed.
