# Monorepo: two services, named ports, tasks

A config pattern rather than a runnable project: an npm workspace with `web` (Next.js, 3000) and
`api` (8080), started together in one sandbox, each forwarded to its own host port and named.

```console
$ boxer up
boxer: url: https://fix-ui.web.myrepo.localhost:1355 -> guest port 3000
boxer: url: https://fix-ui.api.myrepo.localhost:1355 -> guest port 8080
$ boxer url 8080
https://fix-ui.api.myrepo.localhost:1355
```

The web app reaching the API: inside the guest they are both on `127.0.0.1`, so the web app's
server-side code calls `http://127.0.0.1:8080`. Only a browser on the host needs the names.

`[tasks]` is the part that pays off with agents: the brief lists each task with its description,
so an agent runs `boxer run --task test` rather than guessing a command line, and a task with
`junit` reports which tests failed.
