---
name: boxer
description: Run build, test, install and script commands inside this repository's boxer sandbox, a microVM or container mounted on the git worktree. Use before running any command that compiles, installs, downloads or executes project code, and whenever a command was refused with a line starting `boxer:`.
license: Apache-2.0
compatibility: Requires the boxer CLI on PATH and a git worktree.
allowed-tools: Bash(boxer:*)
metadata:
  boxer_version: "[[.Version]]"
---

# boxer sandbox

Commands in this repository run inside a sandbox — a microVM or a container — keyed to the git worktree. The worktree is
normally mounted in the guest, so the files are the same files: edit on the host, build in the
sandbox. A sandbox prepared for forking is the exception, and the brief says so when it applies —
read the brief rather than assuming.

How this repository is configured — where the worktree is mounted, which programs are
intercepted, which stay on the host, and which mode is in force — is a property of the checkout,
not of this skill. Ask the binary:

```sh
scripts/brief          # the current brief, in prose
boxer brief --json     # the same facts as JSON
```

## Running something

Prefer a named task: the repository declares them, so the command is the one its maintainers
meant and nothing depends on the agent composing the right shell line.

```sh
scripts/task test            # run the task named `test`
scripts/run 'npm ci'         # an arbitrary shell line in the sandbox
scripts/status               # is the sandbox up, and where is the worktree mounted
```

`boxer tasks` lists the declared names with what each one is for; an unknown name is refused with
the list. Read the descriptions before choosing: picking the right task is the decision worth
getting right, and a task may also declare where its test report lands, so
`boxer run --task test` summarises which tests failed without being told where to look.

## Reaching a dev server

A server the sandbox runs is forwarded to the host, but not to the port it listens on: every
worktree gets its own host port, so `localhost:3000` is wrong in all but at most one of them.
Ask where it landed:

```sh
boxer url                    # the address of this worktree's server, and nothing else
boxer url 8080               # of a particular guest port
scripts/status               # JSON: `urls` (guest port to URL) when this repository names them,
                             # `ports` (guest port to host port) always
```

Use `urls` when it is present — a stable name such as `https://fix-ui.myapp.localhost:1355`,
the same for as long as the sandbox exists — and `ports` otherwise, as `http://127.0.0.1:<host>`.
Use the same address in a browser, a test, and a `curl`.

If `BOXER_INSIDE` is set, you are running inside the sandbox and there is no `boxer` to ask. A
server you started is at `http://127.0.0.1:<its port>` from where you are; `$BOXER_URLS` (or
`$BOXER_PORTS`) says where a person on the host reaches it — give them that, not a localhost
address, which on the host is somewhere else.

## When a command is refused

Any line beginning `boxer:` on stderr is an instruction, not a transient error. Its `fix:` line is
the exact command to run next. Do not retry the original command unchanged, and do not work around
the sandbox by running the command on the host.

`references/BRIEF.md` explains what the sandbox is, what is shared with the host, and what to do
when something is not visible inside it.
