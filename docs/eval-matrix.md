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

## The scorecard

A cell is not one question. Bringing a sandbox up, reaching the guest, rendering the page, leaving
the change behind and showing that the integration level itself carried the work are separate
claims, and a single pass/fail reports the whole scenario by its weakest part while saying nothing
about which part that was. Each claim is scored on its own and weighted by what it is worth:

| weight | claims |
| --- | --- |
| 3 | reached the guest · the level carried it · no host leak |
| 2 | the page renders · the change is in the worktree · the dependency is installed |
| 1 | the sandbox came up · a host port was forwarded · the dev server answered first · the agent finished · the dev server survived |

Claims are declared before the cell runs, so a cell that dies early reports which claims went
unanswered rather than quietly shortening its own scorecard. The report scores by level, by harness
and by claim.

## Results, 2026-09-25

Two runs of all 55 cells on the same binary, one after the other, three at a time. The first with
`BOXER_EVAL_URLS=1` — every sandbox named through portless and no address in the prompt, so each
agent had to find its own worktree's server — and the second without, as the control.

| level | default | URLs mode |
| --- | --- | --- |
| hook rewrite | 12/12 | 11/12 |
| tool mode | 10/10 | 10/10 |
| command shims | 8/8 | 8/8 |
| shell shims | 7/8 | 8/8 |
| shell substitution | 1/1 | 1/1 |
| inside | 10/10 | 6/10 |
| orchestrator | 6/6 | 6/6 |
| **overall** | **99.4%**, 54 of 55 at 100%, $1.89 | **98.7%**, 50 of 55 at 100%, $3.04 |

The default run's one miss is `codex/bash-shim`: Codex spawned its shell by absolute path in that
run, so the `bash` shim was bypassed and the work ran on the host — the known limit of that level,
and the reason the matrix scores it separately. The URLs run's rewrite miss is Grok starting its
own dev server instead of using the one it was given.

The inside difference was the one worth chasing, and it was a boxer bug rather than a URLs effect
— see "Resolved, 2026-09-26" below. Four inside cells lost
their dev server in the URLs run and none in the control, so the inside cells were run again, alone,
alternating the two modes, twice each: URLs mode 19 of 20, default 20 of 20. Every failure in both
passes is `inside/fx/add-a-page` or its dev server — 2 of 3 URLs-mode runs of that cell against
0 of 3 without, and the published run before this one lost an fx inside cell the same way with no
URLs involved. None of the failing agents read the URL variables or touched their server. The
evidence points at fx and the dev server's own stability in the guest rather than at URLs; it is
not proof, and it is recorded here as open rather than closed.

Reports: `eval-matrix-report.md` (default) and `eval-matrix-urls-report.md`.

**Resolved, 2026-09-26.** Three more rounds of the inside cells, with the eval now recording a dead
sandbox's state, failed only in the first round and reported the same thing each time: the
sandbox was running, and the dev server's log in the guest's `/tmp` did not exist. `/tmp` is
memory-backed, so the machine had been restarted under the server. The first time a harness runs
inside on a host, boxer caches the install as a pack, and packing a smolvm machine means stopping
it; boxer started it again and never restarted the `start` services. That is why only first
rounds failed, why the URLs runs (which went first) looked worse, and why fx — installing for the
first time most often — looked flaky. `boxer pack save` had the same defect. Fixed by restarting the
services after any snapshot; proven with a test that fails without the fix; verified by clearing
the pack cache and running the inside cells cold in both modes: 10 of 10 each.

## Results, 2026-09-20

**99.2%** of weighted checks, **52 of 53 cells at 100%**, three at a time, $3.06.

| level | score | cells at 100% | carried by |
| --- | --- | --- | --- |
| hook rewrite | 96.4% | 11/12 | Claude Code, Codex, OpenCode, Copilot, Grok, pi |
| tool mode | 100% | 10/10 | Claude Code, Kimi Code, DSH, Codex, Grok |
| command shims | 100% | 8/8 | Claude Code, Kimi Code, Codex, Copilot |
| shell shims | 100% | 8/8 | the same four, with `intercept = ["bash", "uname"]`; boxer never writes a `bash` shim, so only `uname` was shimmed |
| shell substitution | 100% | 1/1 | OpenHands, its terminal's shell replaced by `boxer-bash` |
| inside | 100% | 8/8 | Claude Code, Codex, OpenCode, Kimi Code in the guest |
| orchestrator | 100% | 6/6 | T3 Code, Paperclip, herdr |

Two Gemini cells skip: its CLI speaks only the Gemini API, which the AI Gateway does not serve. The
one cell short of 100% is a harness that looped — 239 turns without converging, on the shared eval
model — rather than anything the sandbox did; the same cell scored 100% in the three runs before it.

Every cell runs on one model (`BOXER_EVAL_MODEL`) rather than each harness's own. That is deliberate
— with the model held constant the integration level is the only variable — but it means these
numbers are not comparable with tier t2, and that a weak model occasionally loops a harness.

### What this found

Three faults in boxer, none of them visible to a tier that asks one question and reads one answer:

- **Codex was never intercepted in a linked worktree.** Codex resolves project configuration to the
  main repository, so hooks installed into the worktree were never read: every command ran on the
  host, and one run installed a dependency and ran `node` there. Every orchestrator works in linked
  worktrees, so this was the ordinary case. The installer now writes the hooks to the main
  repository as well.
- **`boxer run` discarded input written before the sandbox attached**, and gave the guest a terminal
  only when its own stdin already was one. A harness that replaces its terminal's shell got a
  non-interactive shell that never printed a prompt; and because a sandbox takes about twenty
  seconds to attach while OpenHands configures its shell one second after spawning it, the prompt it
  parses command results out of was thrown away every time. `boxer run --tty` now asks for a
  terminal explicitly and captures stdin from the start.
- **A boxer shim could be resolved by boxer itself.** The smolvm launcher runs `uname`, so a `uname`
  shim on PATH turned into `boxer run -- uname` inside a boxer already working on that sandbox, and
  the session deadlocked until it was killed — fifteen minutes with the agent's last act being a
  call to boxer's own status tool. Shim directories are marked, and boxer drops them from its PATH
  before it starts anything.

And one limit that cannot be fixed, only reported: **PATH shims do not survive a login shell.** A
login shell rebuilds PATH — on macOS `path_helper` puts the system directories first — so a harness
that runs commands through `zsh -lc` or `bash -lc`, as Codex does, reaches the host. `boxer doctor`
now detects this and says which enforcement to use instead.

### The two shim flows

Command shims and shell shims are the same PATH mechanism and fail in different places, so they are
scored as separate levels rather than one standing in for the other.

A **command shim** is a file per program in the intercept list. It catches a command however it was
spawned, but the list has to be maintained, it misses whatever is not on it, and it must never name
the runtime the harness is written in: a shimmed `node` shadows the interpreter of a node harness,
which then dies before it can load its own modules.

A **shell shim** is one file, `bash`. No list, and nothing that can shadow an interpreter. Its limit
is elsewhere: it only catches commands run through a shell the harness resolves on PATH, and a
harness that spawns `/bin/bash` by absolute path bypasses it entirely.

### What the evidence is worth

`where.txt` is written *by the agent*, so on its own it proves nothing: a model that skips the work
can type `Linux` into a file as easily as run the command. It is corroborated twice — `clsx` has to
really be in `app/package.json`, and the level's own signature has to show boxer carrying a command
— and it is never read alone. An earlier version of this tier trusted it, and an agent that had
installed nothing passed.

Two levels need their evidence read differently, and the report names which mechanism it found. The
OpenCode plugin rewrites in-process and writes no rewrite line. An orchestrator's launcher builds a
clean environment for the harness it starts, so boxer's trace variable never reaches it however well
the interception worked; for those the worktree the orchestrator cut is the evidence.

## What this deliberately leaves out

Every harness and orchestrator this machine can run is now in the matrix. What is still left out:

- **Conductor** — has no command line at all; it needs the GUI, and its procedure is in
  [orchestrators.md](orchestrators.md).
- **MCP servers in the guest** — the SDLC tier covers those, with evidence; repeating them here
  would change two variables at once.

Widening the matrix is adding a row to `MatrixConfigs` in `internal/eval/matrix.go`, which is why
the configurations live in a table rather than in code.
