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
| `gemini/rewrite` | Gemini CLI | hooks, rewrite, project | skipped here: it speaks only the Gemini API |
| `grok/rewrite` | Grok Build | hooks, rewrite, project | a fourth hook dialect |
| `pi/rewrite` | pi | hooks, rewrite, project | a fifth |
| `dsh/tool` | DSH | tool mode | cannot rewrite; tool mode is its only level |
| `openhands/shell` | OpenHands | shell substitution | the agent's entire shell is the sandbox |
| `inside/claude` | `boxer shell claude` | inside | the harness runs in the guest, beside the dev server |
| `t3/orchestrator` | T3 Code → Claude | orchestrator | the orchestrator cuts the worktree and launches the harness |
| `paperclip/orchestrator` | Paperclip → Claude | orchestrator | launches with an environment of its own |
| `herdr/orchestrator` | herdr → Claude | orchestrator | drives an interactive harness in a terminal pane |

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

**27 of 27**, three at a time, $1.49. Two Gemini cells skip: its CLI speaks only the Gemini API,
which the AI Gateway does not serve, so it needs `GEMINI_API_KEY` to run at all.

| level | passed | carried by |
| --- | --- | --- |
| hook rewrite | 12/12 run | Claude Code, Codex, OpenCode, Copilot, Grok, pi |
| tool mode | 6/6 | Claude Code, Kimi Code, DSH |
| PATH shims | 2/2 | Claude Code with `mode = "off"` |
| shell substitution | 1/1 | OpenHands, its terminal's shell replaced by `boxer-bash` |
| inside | 2/2 | Claude Code in the guest |
| orchestrator | 4/4 | T3 Code, Paperclip, herdr |

Every cell runs on one model rather than each harness's own (`BOXER_EVAL_MODEL`). That is deliberate
— with the model held constant the integration level is the only variable — but it means these
numbers are not comparable with tier t2, where each harness uses its own.

### What this found

Two faults in boxer, neither visible to a tier that asks one question and reads one answer:

- **Codex was never intercepted in a linked worktree.** Codex resolves project configuration to the
  main repository, so the hooks `boxer install codex` wrote into the worktree were never read: every
  command ran on the host, and one run installed a dependency and ran `node` there. Every
  orchestrator works in linked worktrees, so this was the ordinary case. The installer now writes
  the hooks to the main repository as well.
- **`boxer run` discarded input written before the sandbox attached**, and gave the guest a terminal
  only when its own stdin already was one. A harness that replaces its terminal's shell got a
  non-interactive shell that never printed a prompt; and because a sandbox takes about twenty
  seconds to attach while OpenHands configures its shell one second after spawning it, the prompt it
  parses command results out of was thrown away every time. `boxer run --tty` asks for a terminal
  explicitly and captures stdin from the start; the wrapper requests it, carries `PROMPT_COMMAND`
  across, and disables readline and rc files.

And several in the evals themselves, each of which had been reporting a pass for something it was
not measuring — the shim row installed a rewriting hook beside the shim, the tool signature ignored
boxer's own trace, orchestrator cells were judged in the wrong worktree, and a shared driver
instance crashed the run once cells ran at the same time.

### What the evidence is worth

`where.txt` is written *by the agent*, so on its own it proves nothing: a model that skips the work
can type `linux` into a file as easily as run the command. It is corroborated twice — `clsx` has to
really be in `app/package.json`, and the level's own signature has to show boxer carrying a command
— and it is never read alone. An earlier version of this tier trusted it, and an agent that had
installed nothing passed.

Two levels need their evidence read differently, and the report names which mechanism it found. The
OpenCode plugin rewrites in-process and writes no rewrite line. An orchestrator's launcher builds a
clean environment for the harness it starts, so boxer's trace variable never reaches it and no trace
is written however well the interception worked; for those the worktree the orchestrator cut is the
evidence.

## What this deliberately leaves out

Every harness and orchestrator this machine can run is now in the matrix. What is still left out:

- **Conductor** — has no command line at all; it needs the GUI, and its procedure is in
  [orchestrators.md](orchestrators.md).
- **MCP servers in the guest** — the SDLC tier covers those, with evidence; repeating them here
  would change two variables at once.

Widening the matrix is adding a row to `MatrixConfigs` in `internal/eval/matrix.go`, which is why
the configurations live in a table rather than in code.
