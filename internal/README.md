# internal

Everything that is not the CLI. Nothing here is importable from outside the module; the only
public surface is [`pkg/boxer`](../pkg/boxer).

| Package | Owns |
| --- | --- |
| `scope` | the identity of a sandbox: `sha256(worktree)`, widened or narrowed by `isolation` |
| `config` | `boxer.toml`, `BOXER_*` and `devcontainer.json`, merged across layers, with the source of every value |
| `decide` | given a command and a configuration: run here, run there, or refuse |
| `box` | the lifecycle above a sandbox — provision, setup, pack, run, reclaim, URLs, host-state cleanup |
| `vm` | the backend boundary: `vm.Backend` and its four implementations (smolvm, Apple `container`, docker, podman), capabilities, typed errors |
| `hook` | one dialect row per harness, translating each one's hook protocol |
| `install` | writing a repository's configuration, merged and idempotent |
| `bundle` | rendering the published skill and plugin package, from a version and nothing else |
| `inside` | running a harness in the guest: `boxer shell`, `boxer acp` |
| `mcp` | the MCP server: `boxer_run`, `boxer_status`, and the session signals |
| `shim` | programs on `PATH` that are really boxer |
| `obs` | the event stream and its sinks; silent unless `[telemetry]` turns it on |
| `sh` | POSIX shell quoting |
| `cli` | how the command line talks to whoever is reading — person, agent or program. Imported only by `cmd/boxer` |
| `junit` | parsing JUnit XML the guest wrote into a run's test summary |
| `eval`, `fakellm`, `vmtest` | the evaluation harness, the scripted model, the fake hypervisor (and `vmtest.Bare`, a Go backend with no optional capabilities, for the refusal paths) |

## The rule

**The core knows nothing about any harness, and nothing about the CLI.** `box`, `vm`, `scope`, `config`, `decide` and `shim`
contain no harness name — `TestCoreNamesNoHarness` parses them and fails if one appears. A harness
is a row in a table: a hook dialect, an inside-mode row, a package view, an install case, an eval
driver. That is what keeps a tenth harness from growing the core. Likewise nothing here but `cli` itself
imports `cli` — `TestCoreDoesNotImportCLI`.

Each package's doc comment says why it is shaped the way it is. Start with `go doc ./internal/box`.

- How one command travels through these packages: [../docs/architecture.md](../docs/architecture.md)
- Adding a harness: [../docs/adding-a-harness.md](../docs/adding-a-harness.md)
- How these are tested: [../docs/testing.md](../docs/testing.md)
