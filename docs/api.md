# API

Three surfaces, one version. The CLI's `--json` output is the language-agnostic API; `pkg/boxer`
is the Go facade; the MCP server is what harnesses call. `pkg/boxer` and `boxer acp` are
experimental until 1.0. Release rules are in [release.md](release.md).

## CLI JSON

`--json` on `ls`, `status`, `down`, `gc`, `doctor` prints exactly one JSON document (an object or an
array) on stdout, indented, with snake_case fields. Human output is unchanged without the flag.
When a scoped command (`status`, `down`) is refused before it can act, the refusal is printed as
`{"error": …}` on stdout (shape under "Errors") and repeated in prose on stderr; exit 1.

The examples below were produced against the fake smolvm the unit tests use (`internal/vmtest`),
so `image` on an existing machine reads `alpine` and labels are sparse; a real smolvm fills them.

### `boxer status [--json]`

The sandbox for the current scope. Exit `0` running, `3` stopped, `4` absent, `1` refused.

```json
{
  "scope": "sb-7e1852e4a3c3",
  "isolation": "worktree",
  "worktree": "/Users/me/src/demo",
  "mount_at": "/workspace",
  "exists": false,
  "state": "absent",
  "image": "debian:bookworm-slim",
  "image_reason": "no lockfile found; boxer base",
  "labels": {},
  "last_used": null
}
```

When the machine exists, `state` is smolvm's (`running`, `stopped`), `image` is the machine's,
`image_reason` is `machine`, `labels` carries the `boxer.*` labels, and `last_used` is the
RFC 3339 UTC time of the last `boxer run` (or `null`).

### `boxer ls --json`

Every boxer-owned machine on the host, sorted by scope. Exit 0.

```json
[
  {
    "scope": "sb-7e1852e4a3c3",
    "state": "running",
    "isolation": "worktree",
    "worktree": "/Users/me/src/demo",
    "integration": "",
    "image": "alpine",
    "created_at": 1,
    "last_used": "2026-09-17T10:28:03Z"
  }
]
```

`created_at` is smolvm's Unix timestamp. `integration` is `inside` for machines created by
`boxer shell`/`boxer acp`, otherwise the outside integration (empty on older machines).

### `boxer gc [--dry-run] --json`

One row per machine gc decided about, in the `ls` shape plus `reason` and `deleted`. Stale image
and harness packs appear as rows with `pack` (the file path) instead of machine fields. Under
`--dry-run` every `deleted` is `false`; a failed delete carries `error` and exits 1.

```json
[
  {
    "scope": "sb-a28652c0877e",
    "state": "running",
    "isolation": "worktree",
    "worktree": "/Users/me/src/old",
    "integration": "",
    "image": "alpine",
    "created_at": 1,
    "last_used": "2026-09-17T10:28:19Z",
    "reason": "worktree /Users/me/src/old is gone",
    "deleted": false
  }
]
```

### `boxer down [--all] --json`

```json
{ "scope": "sb-7e1852e4a3c3", "removed": true }
```

`removed` is `false` when no sandbox existed. `--all` prints an array of the same rows.

### `boxer doctor --json`

Everything the human `doctor` prints, as one object. Exit 1 when `error` is set (settings are
still reported when the configuration loaded but the scope could not be resolved).

```json
{
  "version": "0.1.0",
  "inside": false,
  "smolvm": "smolvm 0.0.0-fake",
  "boxer_path": "",
  "git": { "toplevel": "/Users/me/src/demo", "linked": false },
  "config_files": ["/Users/me/src/demo/boxer.toml"],
  "settings": [
    { "key": "isolation", "value": "worktree", "source": "default" },
    { "key": "mode", "value": "rewrite", "source": "default" },
    "..."
  ],
  "scope": { "key": "sb-7e1852e4a3c3", "isolation": "worktree", "worktree": "/Users/me/src/demo", "degraded": false },
  "image": "debian:bookworm-slim",
  "image_reason": "no lockfile found; boxer base",
  "sandbox": { "scope": "sb-7e1852e4a3c3", "state": "running", "image": "alpine", "created_at": 1, "last_used": "2026-09-17T10:28:03Z", "isolation": "", "worktree": "", "integration": "" },
  "shims": { "on_path": [], "missing": ["npm", "npx", "..."] },
  "warnings": []
}
```

Optional fields: `smolvm_error` (smolvm missing), `image_warning` (`network.mode = off`),
`sandbox_error`, `shims` (only when enforcement uses shims), `installed_versions` (marker file →
boxer version that wrote it, for project-layer installs under the worktree), `error`. `sandbox`
is `null` when absent. A project layer written by another boxer version adds a warning naming
the file and both versions.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | success; `status`: running |
| 1 | refused or failed; the refusal is on stderr (and on stdout as JSON under `--json`) |
| 2 | usage error |
| 3 | `status`: sandbox exists but is stopped |
| 4 | `status`: no sandbox for this scope |
| n | `run`: the guest command's own exit code; 127 when it could not start |

### Errors

Every refusal is a `box.Error` (requirements §3.6) and renders the same way everywhere: in prose
on stderr, as `{"error": …}` under `--json`, and as the text of a failed MCP tool call.

| Field | Meaning |
| --- | --- |
| `reason` | one sentence for the agent |
| `cause` | stable code: `NO_REPOSITORY`, `WORKTREE_REQUIRED`, `SCOPE_UNRESOLVED`, `NO_SANDBOX`, `CREATE_FAILED`, `START_FAILED`, `SETUP_FAILED`, `HARNESS_INSTALL_FAILED` (inside mode) |
| `fix` | the exact next command, or empty when only the operator can act |
| `scope` | `{key, isolation, worktree, degraded, reason}` as far as it was resolved |

```json
{
  "error": {
    "reason": "not inside a git repository",
    "cause": "NO_REPOSITORY",
    "fix": "cd into a git worktree, or `git init`",
    "scope": { "key": "", "isolation": "", "worktree": "", "degraded": false }
  }
}
```

## Go: `pkg/boxer`

The only importable package. Thin delegation to the internals; types are aliases
(`Machine`, `Scope`, `Error`, `RunOpts`).

```go
import "github.com/BarakChamo/boxer/pkg/boxer"

b, err := boxer.Open(cwd, boxer.Options{Harness: "claude-code", SessionID: sid})
if err != nil {
    var be *boxer.Error
    if errors.As(err, &be) { log.Fatalf("%s (%s): %s", be.Reason, be.Cause, be.Fix) }
    log.Fatal(err)
}
if _, err := b.Ensure(true, false); err != nil { log.Fatal(err) }        // create if needed, start
code, err := b.Run([]string{"sh", "-c", "bun test"}, boxer.RunOpts{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
m, exists, _ := b.Status()                                              // Machine, bool
_ = b.Down()

machines, _ := boxer.List()                                             // every boxer-owned machine on the host
for _, m := range machines { fmt.Println(m.Name, m.State, boxer.LastUsed(m.Name)) }
_ = boxer.Stop(name); _ = boxer.Delete(name)
```

`Run` returns `-1` with an error when the configuration says `on_sandbox_unavailable = "passthrough"`
and the sandbox could not be provisioned; the caller decides whether to run on the host.

## MCP

`boxer mcp [--harness NAME]` speaks MCP over stdio (protocol `2025-06-18`); `serverInfo.version`
is the binary version. Two tools:

| Tool | Input | Result |
| --- | --- | --- |
| `boxer_run` | `{command: string, cwd?: string}` | stdout and stderr, then `[exit N]`; `isError` when the exit is non-zero or boxer refused |
| `boxer_status` | `{cwd?: string}` | one line: scope, isolation, state, image, worktree and mount |

Both resolve the scope from `cwd` (default: the server's working directory) with the same rules
as the CLI, so a refusal is the `box.Error` text. `cwd` is forgiving about which side of the mount
it names: a host path inside the worktree becomes the matching guest directory; a path under the
guest mount (`/workspace/...` when `mount_at` is set; the brief tells the model that path) maps back
to the host worktree; any other path falls back to the server's own directory and the result
starts with a `note:` line saying so.

The server is also a session signal for harnesses without hooks: `initialize` provisions the
server's own scope in the background when `create_on` contains `mcp` (default), and stdin EOF
records the scope's last use for `gc`.
