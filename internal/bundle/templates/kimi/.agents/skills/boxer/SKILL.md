---
name: boxer
description: How commands run in this repository — inside a boxer microVM sandbox mounted on the worktree. Read before running build, test, install, or script commands.
---

# boxer sandbox

[[.Instructions]]

## Commands

- `boxer run -c '<shell line>'` — run a shell line in the sandbox, from the current directory.
- `boxer run -- <program> [args]` — run one program in the sandbox.
- `boxer doctor` — show the resolved configuration, the sandbox state, and why an image was chosen.

The `boxer_run` tool does the same as `boxer run -c` and returns stdout, stderr, and the exit code.

## What is where

- Host worktree → guest `[[.MountAt]]`. The same files; edits on either side are immediately visible on the other.
- Intercepted programs: [[range $i, $p := .Intercept]][[if $i]], [[end]]`[[$p]]`[[end]].
- Host-only programs: [[range $i, $p := .Passthrough]][[if $i]], [[end]]`[[$p]]`[[end]].
