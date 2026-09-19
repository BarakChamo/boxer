# Eval tier: matrix

The SDLC tier proves that development works in a boxer sandbox. It proves it for exactly one
configuration: Claude Code, the project hook layer, rewrite mode. Every other way into the sandbox
— tool mode, PATH shims, shell substitution, inside mode, an orchestrator — is covered only by
single-answer cells in t1 and t2, where a cell asks one question and reads one answer.

That leaves the interesting question open. A level can look fine for `uname -a` and fall apart the
moment an agent installs a dependency, restarts a dev server and checks a page in a browser. The
matrix tier asks that question: **the same Next.js workload, at every integration level.**

```
make eval-matrix                      # the whole matrix, three cells at a time
bin/boxer-eval --tier matrix --cell claude/rewrite --limit 1   # one configuration
```

## What is in it

| configuration | harness | level | why it is in the set |
| --- | --- | --- | --- |
| `claude/rewrite` | Claude Code | hooks, rewrite, project | the baseline everything else is compared against |
| `claude/tool` | Claude Code | tool mode | the level that carries harnesses which cannot rewrite |
| `codex/rewrite` | Codex | hooks, rewrite, project | a different hook payload and a different CLI |
| `opencode/plugin` | OpenCode | TypeScript plugin | not a hook at all; a different mechanism |
| `copilot/user` | Copilot CLI | hooks, rewrite, user scope | its hooks exist only at user scope |
| `kimi/tool` | Kimi Code | tool mode, user scope | cannot rewrite by design, so tool mode is its real path |
| `claude/shims` | Claude Code | PATH shims | nothing hooks anything; the shim is the whole mechanism |
| `openhands/shell` | OpenHands | shell substitution | the agent's entire shell is the sandbox |
| `inside/claude` | `boxer shell claude` | inside | the harness runs in the guest, beside the dev server |
| `t3/orchestrator` | T3 Code → Claude | orchestrator | the orchestrator cuts the worktree and launches the harness |

Each configuration runs in its own linked git worktree with its own sandbox and its own automatic
host port, three at a time.

## The two tasks

- **`add-a-page`** — write a route that renders a marker and verify it. Every configuration can do
  this, which is the point: it isolates the level rather than the task.
- **`prove-and-install`** — install `clsx`, use it on a page, and write what `node` reports as its
  platform into `where.txt`. `npm`, `node` and `next` are intercepted, so this is the task that
  catches a level which silently ran on the host: the file has to say `linux`.

## Each level has to show its signature

A passing task is not evidence that the level did the carrying — an agent can do the job on the host
and render a perfectly good page. So on the `prove-and-install` task every cell is also checked
against boxer's own trace:

| level | signature required |
| --- | --- |
| rewrite | at least one rewrite into `boxer run`, and `where.txt` says `linux` |
| tool | at least one denial, and `where.txt` says `linux` |
| shims | no rewrites at all, and `where.txt` says `linux` — the shim carried it |
| shell substitution | same as shims: no hook rewrote anything, and the work still reached the guest |
| inside | no hook events whatsoever, and `where.txt` says `linux` |
| orchestrator | the orchestrator's own worktree got the rewrite |

Plus, for every cell: the page renders the marker when read from the host with a real browser, the
change is in the worktree, and the host leak canary is absent.

## Results, 2026-09-19

**15 of 18** on the last full run ($0.53, three at a time), by level:

| level | passed | |
| --- | --- | --- |
| rewrite | 6/8 | codex fails; see below |
| tool | 4/4 | both Claude Code and Kimi Code |
| shims | 2/2 | |
| inside | 2/2 | |
| orchestrator | 1/1 | T3 Code driving Claude, in its own worktree |
| shell substitution | 0/1 | OpenHands works but does not finish in 15 minutes |

Every cell ran on one model (`BOXER_EVAL_MODEL`), not each harness's own. That is deliberate — with
the model held constant the integration level is the only variable — but it means these numbers are
not comparable with tier t2, where each harness uses its own.

### Codex does not get intercepted

`codex/rewrite` fails the discriminating task, reproducibly, on two different models: sixteen shell
commands ran and **boxer's hook fired zero times** — no trace file was created at all. One run
recorded platform `darwin`, meaning the dependency install and `node` ran on the host.

The passes codex does collect come from the agent reading the installed agent contract and typing
`boxer run` itself. That is persuasion, not containment, and it is not a level.

What has been ruled out: the hooks fire correctly at tier t1, in a live standalone repository, and
in a live linked git worktree (`SessionStart`, `PreToolUse` and a rewrite all appear in the trace).
So it is not worktrees, not hook trust, and not the model. **The cause is still open.**

### OpenHands is too slow rather than broken

Shell substitution executes — the terminal really is `boxer-bash`, and the transcript shows commands
running through it — but the loop does not finish this workload inside fifteen minutes.

### What the evidence is worth

`where.txt` is written *by the agent*, so on its own it proves nothing: a model that skips the work
can type `linux` into a file as easily as run the command. It is corroborated twice — `clsx` has to
really be in `app/package.json`, and the level's own signature has to show boxer carrying a command
— and it is never read alone. An earlier version of this tier trusted it, and an agent that had
installed nothing passed.

The OpenCode plugin rewrites in-process and writes no rewrite line, so its evidence is the layer
being loaded plus the corroboration above. The report names that mechanism rather than reporting it
as an ordinary hook rewrite.

## What this deliberately leaves out

A representative sample, not a full cross product. Left out, with reasons:

- **Gemini** — needs your own key; the gateway has no Gemini protocol.
- **DSH** — a plugin stack rather than a single binary: a profile patch plus the Claude hook bridge.
- **Paperclip, herdr** — a config patch and an issue id; pane-driven, respectively.
- **pi, Grok** — rewrite level, already covered by two rows, and Grok costs a denial on every first
  command.
- **MCP servers in the guest** — the SDLC tier covers those, with evidence; repeating them here
  would change two variables at once.

Widening the matrix is adding a row to `MatrixConfigs` in `internal/eval/matrix.go`, which is why
the configurations live in a table rather than in code.
