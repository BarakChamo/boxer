---
name: boxer
description: Run build, test, install and script commands inside this repository's boxer sandbox, a microVM mounted on the git worktree. Use before running any command that compiles, installs, downloads or executes project code, and whenever a command was refused with a line starting `boxer:`.
license: Apache-2.0
compatibility: Requires the boxer CLI on PATH and a git worktree.
allowed-tools: Bash(boxer:*)
metadata:
  boxer_version: "dev"
---

# boxer sandbox

Commands in this repository run inside a microVM keyed to the git worktree. The worktree is
mounted in the guest, so the files are the same files: edit on the host, build in the sandbox.

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

`boxer tasks` lists the declared names; an unknown name is refused with the list.

## When a command is refused

Any line beginning `boxer:` on stderr is an instruction, not a transient error. Its `fix:` line is
the exact command to run next. Do not retry the original command unchanged, and do not work around
the sandbox by running the command on the host.

`references/BRIEF.md` explains what the sandbox is, what is shared with the host, and what to do
when something is not visible inside it.
