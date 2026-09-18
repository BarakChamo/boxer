# boxer events

boxer records what it did as a stream of JSON events. The stream is **off by default**: with no
`[telemetry]` table and no `BOXER_TRACE`, boxer writes nothing anywhere.

This document is the schema. It is part of boxer's stable surface from 1.0: fields are added,
never repurposed or removed.

## Turning it on

```toml
[telemetry]
enabled         = true      # off by default
sink            = "file"    # none | file | stderr | otel
path            = ""        # file sink; default $XDG_STATE_HOME/boxer/events.jsonl
record_commands = false     # keep command lines out of the stream unless this is true
endpoint        = ""        # otel sink only; without it nothing leaves the machine
```

`BOXER_TRACE=<path>` turns the file sink on at that path regardless of configuration, which is how
the eval suite and a harness that can set only environment variables get a stream. Setting
`enabled = true` without naming a sink means the file sink. `BOXER_TELEMETRY_SINK` overrides the
sink from the environment.

Read it back with `boxer logs [--scope NAME] [--json]`, or take the last few events for the current
scope from `boxer status --json`.

## One event

```json
{
  "time": "2026-09-18T09:41:02.183746Z",
  "event": "run",
  "scope": "sb-4f2c19ab",
  "harness": "claude-code",
  "duration_ms": 412.7,
  "outcome": "ok",
  "payload": { "exit": 0, "command_len": 24 }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `time` | RFC3339Nano, UTC | when the event was recorded |
| `event` | string | one of the names below |
| `scope` | string | the sandbox key (`sb-…`), empty before a scope is resolved |
| `harness` | string | the harness that asked, empty when boxer was invoked directly |
| `duration_ms` | number | how long the operation took; absent when instantaneous |
| `outcome` | string | `ok`, `error`, `denied`, `skipped` |
| `payload` | object | event-specific; see below |

## Event names

| `event` | Emitted when | Payload |
| --- | --- | --- |
| `resolve` | a scope is resolved from the working directory | `isolation`, `degraded`, `warnings` |
| `provision` | a sandbox is created, started, or found already running | `created`, `image`, `image_reason` |
| `pack` | an image or harness pack is created or reused | `kind` (`image` or `harness`), `image`, `path` |
| `run` | a command finishes in the guest | `exit`, `command`* |
| `rewrite` | a hook rewrote a shell command into `boxer run` | `tool`, `command`* |
| `deny` | a hook refused a command | `tool`, `reason`, `command`* |
| `hook` | a hook invocation completes | `event` (the harness's own event name), `purpose` |
| `mcp` | an MCP tool call completes | `tool`, `command`* |
| `gc` | `boxer gc` finishes a sweep | `machines`, `packs`, `dry_run` |
| `error` | an operation failed with boxer's error contract | `cause`, `reason` |

\* Elided unless `record_commands = true`. See below.

## Redaction

Redaction happens in the sink, not at the call site, so no caller can leak a command line by
forgetting to elide one. The payload keys `command`, `argv`, `env` and `setup` are removed unless
`record_commands = true`, and each is replaced by `<key>_len`, the length of what was dropped — a
reader can still tell an empty command from a long one without seeing it.

Everything else in a payload is boxer's own vocabulary: scope keys, image names, exit codes,
durations. Scope keys are derived from the worktree path, so treat them as identifying the machine
they were recorded on.

Nothing leaves the machine unless the `otel` sink is both compiled in (`go build -tags otel`) and
given an `endpoint`. The default binary contains no exporter at all.

## The `BOXER_TRACE` format

The file sink also carries the hook trace lines boxer has always written, one per hook input and
output:

```
2026-09-18T09:41:02.183746Z claude-code <- {"hook_event_name":"PreToolUse",…}
2026-09-18T09:41:02.201113Z claude-code -> {"hookSpecificOutput":{…}}
```

`internal/eval`'s oracle parses those lines, and they are unchanged from before the event stream
existed. The oracle reads only lines carrying an arrow, so JSON events share the file harmlessly.
