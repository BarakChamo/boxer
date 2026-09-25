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

`doctor.go` is the exception worth knowing about: it is long because it explains a resolved
configuration to a person, which is presentation rather than logic.

User-facing documentation for these commands is the [site](../site); the reference is
`site/content/docs/reference/cli.mdx`.
