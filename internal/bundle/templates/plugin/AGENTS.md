# boxer sandbox
<!-- boxer_version: [[.Version]] -->

This repository runs commands inside a boxer sandbox: a microVM or container keyed to the git worktree, with
the worktree mounted into the guest. Files you edit on the host are normally the same files the
sandbox sees; a sandbox prepared for forking works on a copy instead, and the brief says so when
it does.

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
- `boxer status` — whether the sandbox exists and where the worktree is mounted. With `--json`,
  `urls` and `ports` say where a server the sandbox runs is reachable: a different place in every
  worktree, so read it rather than assuming `localhost:<port>`.
  Inside the sandbox (`BOXER_INSIDE` is set) there is no `boxer`: a server is at
  `127.0.0.1:<port>` from there, and `$BOXER_URLS` or `$BOXER_PORTS` is where the host reaches it.
- `boxer url [port]` — the address of this worktree's server, ready to use.
- `boxer ls` / `boxer rm <name>` — every sandbox on this machine and its worktree's state; remove one.
- `boxer doctor` — the resolved configuration, the sandbox state, and why an image was chosen.
- `boxer up` / `boxer down` — create or remove the sandbox for this worktree explicitly.

The `boxer_run` tool does the same as `boxer run -c` and returns stdout, stderr, and the exit code;
`boxer_status` is the tool form of `boxer status`. The brief is also served as the MCP resource
`boxer://brief`.

Any line beginning `boxer:` on stderr is an instruction, not a transient error: its `fix:` line is
the exact command to run next.
