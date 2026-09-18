# boxer sandbox
<!-- boxer_version: dev -->

This repository runs commands inside a boxer sandbox: a microVM keyed to the git worktree, with
the worktree mounted into the guest. Files you edit on the host are the same files the sandbox
sees.

How this checkout is configured — where the worktree is mounted, which programs are intercepted,
which always run on the host, and whether the shell path is rewritten or refused — is resolved at
run time, not written here. Ask the binary:

- `boxer brief` — the current brief in prose; `boxer brief --json` for the same facts as data.
- `boxer tasks` — the command lines this repository declares.

## Commands

- `boxer run --task <name>` — run a declared task in the sandbox. Prefer this: the name is the
  contract, so nothing depends on composing the right shell line.
- `boxer run -c '<shell line>'` — run a shell line in the sandbox, from the current directory.
- `boxer run -- <program> [args]` — run one program in the sandbox.
- `boxer status` — whether the sandbox exists and where the worktree is mounted.
- `boxer doctor` — the resolved configuration, the sandbox state, and why an image was chosen.
- `boxer up` / `boxer down` — create or remove the sandbox for this worktree explicitly.

The `boxer_run` tool does the same as `boxer run -c` and returns stdout, stderr, and the exit code;
`boxer_status` is the tool form of `boxer status`. The brief is also served as the MCP resource
`boxer://brief`.

Any line beginning `boxer:` on stderr is an instruction, not a transient error: its `fix:` line is
the exact command to run next.
