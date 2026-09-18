# Integrating boxer with your coding agent

Coding harnesses differ in what they will let a third party do. Some run your hook on every tool
call and accept a rewritten command. Some can only say yes or no. Some load plugins; some load
nothing at all. boxer therefore has five ways in, and they stack: whichever ones your harness
supports are active at once, and they do not conflict.

You do not have to choose. `boxer install <harness>` picks the strongest level that harness
supports and writes exactly the files it reads.

## The five levels

### 1. The MCP server and the skill — works everywhere

Every harness worth using speaks MCP, and most now load Agent Skills. boxer ships both: an MCP
server exposing `boxer_run` and `boxer_status`, and a skill that tells the agent, in its own
words, that shell commands here go through boxer, plus scripts it can call directly.

This is the floor. It requires no hook API, no plugin loader and no cooperation beyond loading a
server, and it is the only level that works on a harness nobody has integrated yet.

Its limit is that it asks rather than enforces: an agent that ignores the skill and calls its own
shell tool is not sandboxed. The `adherence` evaluation tier measures exactly how often that
happens — see [status.md](status.md).

### 2. Hooks — enforcement, where the harness has them

`boxer hook <harness>` is one binary that speaks every harness's hook dialect. At session start it
provisions the VM and injects the brief; on every shell call it inspects the command and, if it is
on the intercept list, rewrites it to run in the guest. Claude Code, Codex, Grok, Gemini CLI and
OpenCode all accept a rewritten command, so the agent never sees a refusal and never has to learn
anything.

A harness whose hooks can only allow or deny gets `mode = "tool"` instead: the shell is denied
with an error that names `boxer_run` as the way to proceed, and the agent uses the MCP tool. Kimi
and DSH work this way.

### 3. The plugin package — one bundle every loader accepts

`boxer package plugin --out dist` renders a single [Agent Plugins 1.0.0](https://agent-plugins.org)
package that is valid for every client at once: `plugin.json`, the skill, `mcp.json`, `AGENTS.md`,
and one reverse-domain directory per client carrying that client's hooks and README. The same
directory carries the native manifests today's loaders read, so it installs everywhere now:

```sh
claude plugin install dist/boxer
codex plugin marketplace add dist/boxer && codex plugin add boxer@boxer
grok plugin install dist/boxer
gemini extensions install dist/gemini-cli     # Gemini reads hooks from hooks/; use its view
```

Harnesses with no plugin loader — Kimi, DSH, OpenCode, pi — have their files listed in their
namespace README, and `boxer install <harness>` writes them into the repository for you.

### 4. Shell substitution — no integration at all

If a harness lets you configure which shell binary it uses, it needs no boxer code whatsoever:

```sh
boxer shim install --shell     # writes boxer-bash: exec boxer run -- bash "$@"
```

Point the harness at `boxer-bash` — OpenHands, for instance, takes a `shell_path` on its terminal
tool — and the agent's entire interactive shell runs in the guest, compound lines, pipelines,
prompt markers and all. PATH shims (`boxer shim install`) are the same trick one level down, for a
harness that resolves programs by name rather than through a shell.

This is the most complete outside-mode enforcement there is, because nothing is being recognised
or rewritten: the shell itself is in the sandbox.

### 5. Inside mode — run the harness in the VM

```sh
boxer shell claude                    # the VM for this worktree, claude installed in it, running in it
boxer shell codex -- exec "fix the tests"
boxer acp gemini                      # the harness's ACP server inside the VM, stdio piped out
boxer shim install --harness claude   # a `claude` on PATH that is really `boxer shell claude`
```

Nothing to hook, nothing to rewrite, nothing to adhere to: the agent cannot run a command on your
machine because it is not on your machine. This is how dev containers and cloud sessions do it,
on smolvm.

The worktree and each harness's configuration directory (`~/.claude`, `~/.codex`, `~/.gemini`, …)
are mounted at their host paths, so sessions and logins are shared. The exception is a login stored
in the macOS Keychain, which does not travel — see [troubleshooting.md](troubleshooting.md).

The first `boxer shell <harness>` on a machine pays for installing that harness in the guest once
and then packs the result; later worktrees start from the pack in seconds.

## Which level does each harness get

Verified by the evaluation suite; the per-cell results and their dates are in
[status.md](status.md).

| Harness | Hooks | Plugin package | Inside (`shell`) | ACP | Verified at |
| --- | --- | --- | --- | --- | --- |
| Claude Code | rewrite | yes | yes | yes | T1, T2 live, adherence |
| Codex | rewrite | yes | yes | yes | T1, T2 live, adherence |
| Gemini CLI | rewrite | yes (its own view) | yes | yes | T1; T2 needs `GEMINI_API_KEY` |
| OpenCode | rewrite | files listed, `boxer install` writes them | yes | yes | T1, T2 live, adherence |
| pi | rewrite | files listed | yes | no ACP server | T1, T2 live, adherence |
| Grok | rewrite (user and project hooks) | yes | yes | yes | T1, T2 live, adherence — use rewrite mode, see below |
| Kimi | block only, so tool mode | files listed | yes | yes | T1, T2 live, adherence |
| DSH | deny only, through the Claude Code hook bridge | profile patch layer | — | — | T1, T2 live |

Grok does not surface session-start context to the model, so in tool mode its first shell command
always costs one denial before the agent learns to use `boxer_run`. Rewrite mode is the right
default there, and boxer's own dialect records this.

## Orchestrators

A harness launched by an orchestrator usually gets its own configuration directory, so a
user-level plugin never reaches it. That is why `boxer install all` writes a project layer into
the repository: it is what those sessions actually load. Having both layers is harmless.

OpenHands, Paperclip, T3 Code, herdr, Conductor and Multica each have a verified path, in
[orchestrators.md](orchestrators.md).

## Installing into a repository

```sh
boxer install all   # .claude/settings.json + .mcp.json, .codex/hooks.json, .gemini/settings.json,
                    # .opencode/plugins/boxer.ts + opencode.json, .grok/hooks/boxer.json
```

Merged and idempotent: it adds boxer's entries to files you already have and changes nothing else.
`boxer install git` additionally adds a `post-checkout` hook that warms a sandbox whenever
`git worktree add` creates a worktree, so the VM is ready before an agent opens it. That one is
opt-in, and `boxer install all` leaves your git configuration alone.
