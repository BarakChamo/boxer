# cmd

Three binaries. Only the first is shipped.

| Binary | What it is |
| --- | --- |
| `boxer` | the CLI, and every entry point the integrations use: `hook`, `mcp`, `shim`, `install`, `package` |
| `boxer-eval` | the evaluation runner — `--tier t1 \| t2 \| adherence \| flow \| sdlc \| matrix` |
| `fakellm` | the scripted model endpoint, so a tier can run with no key and no variance |

```sh
make build      # all three into bin/, version-stamped
```

`cmd/boxer` holds argument parsing, output rendering and exit codes, and nothing else. A command
that grew logic of its own would be logic the MCP server and the hooks do not share — the
behaviour belongs in `internal/`, where every entry point reaches it.

Two files are exceptions worth knowing about. `doctor.go` is long because it explains a resolved
configuration to a person, which is presentation rather than logic. `dx.go` holds the machine-level
commands — `ls`, `stop`/`rm`, `backends`, `integrations`, `url`, `completion` — and does have some
logic of its own: reading a worktree's git state, probing a backend with a real sandbox, asking the
installer which files it would write. It is here deliberately. Those are questions a person asks of
the machine, not behaviour an agent's command needs, so no hook or MCP call has a reason to share
them; and they are built only from the same public pieces every entry point uses (`vm.Backend`,
`box.Env`, `install`), so the core cannot come to depend on them. How output looks — colour,
tables, prompts, who is reading — is `internal/cli`.

User-facing documentation for these commands is the [site](../site); the reference is
`site/content/docs/reference/cli.mdx`.
