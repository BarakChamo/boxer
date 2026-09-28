# Adding a harness

Supporting a tenth harness must not grow the core. `internal/{box,vm,scope,config,decide,shim}`
and `pkg/boxer` contain no harness name, and `TestCoreNamesNoHarness` parses them and fails if one
appears. So adding a harness means adding rows to tables, and nothing else.

There are five tables. A harness appears in the ones that apply to it and is absent from the rest;
none of them is mandatory, and the honest outcome for some harnesses is two rows rather than five.

## Before writing anything

Establish, against the real binary, what the harness actually does. Every row below encodes an
observed fact, and a guess here is a cell that passes locally and fails in the matrix.

1. **Does it have a hook API, and can a hook rewrite a command or only allow and deny it?** This
   is the difference between `mode = "rewrite"` and `mode = "tool"`, and it is the single most
   important thing to get right.
2. **What is its shell tool called?** Claude Code's is `Bash`; Grok's is
   `run_terminal_command`; Copilot has two, `bash` and `powershell`.
3. **Which events does it emit, and under what names?** And does the payload carry the event name
   at all — Copilot identifies the event by the key the hook is registered under.
4. **Does its session-start context actually reach the model?** Grok accepts a session-start hook
   and never shows its output, which changes what the first command does. This is measurable with
   the `adherence` tier and should not be assumed.
5. **Where does it read project configuration from?** Codex resolves to the main repository, not
   the linked worktree. Getting this wrong means every command runs on the host, silently.
6. **Does it resolve commands through a login shell?** `zsh -lc` rebuilds `PATH` and demotes
   shims, so shim-based enforcement is not available for it.

Record the date and the version you verified against, in the comment next to the row. Every
existing row carries one, because these facts change under you.

## 1. The hook dialect — `internal/hook`

One row in `Dialects`. This is the level that enforces, so it is the one to add first where the
harness supports it.

```go
"grok": {Name: "grok", MCP: true, ShellTool: "run_terminal_command", Rewrite: true,
    Family: "claude", Events: claudeEvents,
    RunToolHint: "the boxer_run tool: find it with search_tool, then call it with use_tool",
    NoSessionContext: true},
```

| Field | What it encodes |
| --- | --- |
| `ShellTool`, `ShellTool2` | the tool name(s) carrying shell commands |
| `ArgsField` | the JSON field holding the tool's arguments; `""` means `tool_input` |
| `DefaultEvent` | the purpose to assume when the payload names no event |
| `Rewrite` | false for a harness whose hooks can only allow or block |
| `Family` | the output JSON shape: `claude` or `gemini` |
| `Events` | this harness's event names, mapped to boxer's purposes |
| `MCP` | whether the bundle registers boxer's MCP server |
| `RunToolHint` | how this harness shows the run tool to the model |
| `NoSessionContext` | the harness accepts a session-start hook but the model never sees it |

Most harnesses reuse `claudeEvents`. A harness that differs gets its own map with a comment saying
what was observed — `dshEvents` is the worked example, and it differs twice for reasons that took
a run to find.

`hook` contains no decision. It normalises the event, calls `decide.Decide`, and renders the
answer in the harness's dialect.

## 2. The installer — `internal/install`

One `case` in `Install`, writing exactly the files this harness reads.

```go
case "codex":
    r.copy(hooksFile, filepath.Join(root, ".codex", "hooks.json"))
    r.copyTree(skill, filepath.Join(root, ".agents", "skills", "boxer"))
```

Three rules:

- **Merge, do not overwrite.** `mergeJSON` preserves what the user already had. Someone's
  `.claude/settings.json` is theirs.
- **Idempotent.** Installing twice produces the same tree.
- **Say what you did.** `Result.Notes` carries anything invisible — the Codex main-repository
  write is the reason this field exists.

Add the user-scope equivalent in `User` when the harness has one.

## 3. The package view — `internal/bundle`

One row in `views`, plus the template files under `internal/bundle/templates/plugin/`.

```go
"gemini-cli": {Namespace: "com.google.gemini-cli", Native: []string{"gemini-extension.json"},
    Alias: map[string]string{"hooks/hooks.json": "com.google.gemini-cli/hooks/hooks.json"}},
```

`Namespace` is the reverse-domain extension directory. `Native` are the manifests the client's
loader reads at fixed paths today. `Alias` copies a package file to a path a client insists on.

The package is published content: every file is byte-identical for every user apart from the
release version. Nothing in a template may depend on the configuration of the machine that
rendered it — configuration reaches the agent at run time, through `boxer brief`, the hooks and
the MCP resource.

After editing a template, regenerate and commit:

```sh
make package
```

CI diffs the checked-in `plugin/` against a fresh render and fails when they disagree.

## 4. Inside mode — `internal/inside`

One row in `Harnesses` and one entry in `Names()`, if the harness can be installed and run in the
guest.

```go
"copilot": {Bin: "copilot",
    Install: "apt-get update -qq && apt-get install -y -qq --no-install-recommends ca-certificates && npm i -g @github/copilot",
    ACP: []string{"copilot", "--acp"}, ConfigVar: "COPILOT_HOME", ConfigDir: "~/.copilot",
    Env: []string{...}, Creds: []string{...}, LoginHint: "...", Hosts: []string{...}},
```

The fields that are easy to get wrong:

- **`Hosts`** must include every API host the harness contacts, or the allowlist silently breaks
  it. Watch a real session to collect them.
- **`Creds` and `LoginHint`** are for the login that does not travel. A Keychain login has no
  Keychain inside a Linux guest; boxer prints the hint before the run rather than after the
  failure.
- **`GuestEnv` and `Args`** turn off the harness's own nested sandbox, which cannot start inside
  the guest.
- **CA certificates.** A slim node image ships no system trust store. A harness whose model client
  is a native binary — Codex, Copilot — fails with an error that names neither TLS nor the image
  until `ca-certificates` is in its install line.

## 5. The evaluation driver — `internal/eval`

A `Driver` implementation, added to `Drivers()`. Without it the harness is undocumented as far as
the support tables are concerned: nothing is listed as supported because the code exists, only
because a cell ran and scored.

```sh
make eval-t1                                  # scripted model, real VM, free
bin/boxer-eval --tier t1 --cell newharness/rewrite
make eval-t2                                  # live models, cents
```

## Documenting it

- `site/content/docs/setup/<harness>.mdx` — the harness's setup page, with its "What it writes" table, and a row in `setup/index.mdx`.
- `site/content/docs/evals/results.mdx` — only after a matrix run scores it.
- `docs/status.md` — the run that scored it, with its date and any skip reason.

A row in the support table that no cell produced is the one thing this project's documentation
does not do.

## The check

```sh
go test ./...        # TestCoreNamesNoHarness among them
make package         # and commit the diff
make eval-t1
```

If `TestCoreNamesNoHarness` fails, a harness name reached the core. The fix is a table row, not an
exception.
