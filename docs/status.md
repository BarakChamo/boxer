# Status: 1.1 in progress (2026-09-25)

The stopping point for this slice: comprehensive evals run against real harnesses and real
orchestrators on this machine, every skip explained. Reports: [eval-t1.md](eval-t1.md) (scripted
model, real harness CLIs, real smolvm) and [eval-t2.md](eval-t2.md) (live models). Method:
[eval-plan.md](eval-plan.md). Release shape: [release.md](release.md). API: [api.md](api.md).

## Proven

Unit tests green with the race detector and a per-package coverage floor; lint and the
vulnerability scan clean; smoke **85/85 on smolvm and 76 passed, 9 skipped by capability on
docker, podman and Apple `container`** (which also prints and bounds cold start, warm run and
hook latency); T1 **68 pass, 0 fail, 1 skip**, on three separate runs; flow **2/2**, including the agent cell. T2 live on
`zai/glm-5.3-flash` is **40 pass, 3 fail, 14 skip** — see below, because the failures are not what
they look like. Performance against every other sandbox on this host is in
[bench/RESULTS.md](../bench/RESULTS.md). Every cell is a fresh repository and a fresh VM.

**T2 is not a release gate in its current form, and this slice is where that became clear.** Two
full runs each lost three cells, and only one cell was the same in both. Every tool-mode failure
across both runs was the same shape — `deny: 1 denial(s): the agent had to be corrected` — on a
different harness each time. That is the enforcement working, scored as a failure. T2 runs one
model, so one stochastic choice about whether to reach for the shell tool decides the cell, and
there is no way to tell that from a boxer regression. The adherence tier runs four models and
votes for exactly this reason. Before T2 can gate a release, either its tool-mode cells vote the
same way or a single denial-and-recovery stops counting as a failure. `copilot/tool/user/worktree`
failed in both runs and on a third targeted re-run; it is model behaviour, not a regression —
removing this slice's only change to the Copilot driver did not alter it, and the cell declares no
tasks, so the brief it receives is byte-identical to the one the green baseline received.

**boxer has four backends.** `backend = "smolvm"` (the default) gives a kernel per sandbox;
Apple's `container` also gives a kernel per sandbox, on Virtualization.framework, from ordinary OCI
images. `docker` and `podman` give a container — faster to start, available without a hypervisor,
and **one kernel shared by every sandbox**. They are not interchangeable for
containment: a container runtime has no per-host egress policy, so `network.mode = "allowlist"`,
boxer's default, is refused by name rather than silently widened. `boxer doctor` prints the
capability table with the boundary first. The same smoke suite runs on each — 85/85 on smolvm,
and 76 passed with 9 capability-gated cells skipped by name on docker, podman and Apple `container` — and running it on a
second backend immediately found that `ls`, `gc`, `down --all` and `watch` had always talked to
smolvm regardless of configuration, so `gc` reclaimed nothing while containers accumulated.

**Two ways to do work before the sandbox exists.** Host package caches are now mounted read-only
by default, detected from the same lockfiles that choose the image — the install still runs in the
guest, so only the download is saved and no binary can be chosen for the wrong platform. `[prep]`
is the opt-in half: devcontainer's `initializeCommand`, running on the host with the guest's
platform triple offered as `$BOXER_TARGET_FLAGS`. Verified end to end — an Alpine guest resolves
`@next/swc-linux-arm64-musl` from an install run on macOS. It is opt-in because it is only correct
for packages that ship prebuilt binaries; anything compiling at install time builds for the host
and fails later inside the guest, so `boxer doctor` warns by name and `setup` stays the default.

**Linux, for the first time.** boxer had never run on a Linux host. It now has: natively on Fedora
CoreOS (aarch64, the podman machine's own OS) with rootless podman and SELinux enforcing, where
the smoke suite passes **68 with 10 skipped** and every runnable example serves HTTP 200. The
first run there failed nearly every cell, for two reasons no Mac run could have shown: SELinux
denies an unlabelled bind mount, and rootless podman maps you to the container's root so a
non-root guest user cannot write the worktree. Both are fixed — the mount is relabelled when
SELinux is enforcing, and a user that cannot write the worktree is given the right uid, or on
rootless podman the sandbox is recreated with `--userns=keep-id`. smolvm and docker on Linux are
still not run: this machine's Linux has no `/dev/kvm` for smolvm, and its `docker` is podman's
compatibility shim rather than a daemon.

**What was run before this release, 2026-09-25.** The smoke suite on all four backends, three
times over the day and once more on the final binary. The scripted-model matrix (t1, 69 cells)
three times: 68/0/1, 67/1/1 and 68/0/1 — the one failure a herdr pane that inherited the
running Claude Code session's `CLAUDE_CODE_CHILD_SESSION` marker and did not recur; the skip is
multica, which needs a server configured on the host. Live adherence scenarios across eight
harnesses and four models: `prep` 32/32; `server` 24, 21 and then 25 of 32 once the brief pointed
agents at `boxer url` — every miss either a single model on a harness where the others passed, or
Grok, which never shows the brief to the model. The same two scenarios on docker: 8/8 and 7/8. The
full Next.js matrix twice, with and without URLs: 99.4% and 98.7%. The inside gap between them
was traced on 2026-09-26 to boxer stopping the machine to cache a harness install and not
restarting the dev server — first runs only, nothing to do with URLs — fixed, and verified cold at
10 of 10 in both modes; see eval-matrix.md.

Nine defects were found by those runs rather than by reading, and each is in the changelog with the
evidence that closed it: the `/dev/null` TTY exit code, a runtime's own failures read as exit
codes, the stale docker mount after a worktree is recreated, a recreated worktree skipping setup, a
non-root user unable to write on smolvm, a sed backreference that corrupted `/etc/passwd`, one
retired hostname failing every inside sandbox, URLs invisible to an MCP server with a different
state directory, and `boxer-eval --list` running the matrix.

**The machine, from the command line.** `boxer ls -A` lists every sandbox on every backend with
its branch, whether its worktree is clean, dirty or gone, and where it serves; `boxer rm` removes
by name, by filter or from a list; `boxer backends --probe` proves each runtime can run a sandbox
rather than trusting that it answers; `boxer integrations` says which harnesses and orchestrators
are here and whether boxer is wired into each, by asking boxer's own installer what it would have
written. Output is for whoever is reading: a person gets colour, tables and a confirmation before
a bulk removal; an agent gets plain text, `next:` lines and never a prompt.

**Every worktree's dev server has a name.** With `[urls]` on, each forwarded port gets a stable
hostname through portless — `https://<branch>.<repo>.localhost:1355`, or the sandbox's two-word
name for a detached worktree — registered when the sandbox starts and removed when it is deleted.
The brief, the skill, `AGENTS.md` and `boxer_status` tell an agent to read it from `boxer status
--json` instead of assuming `localhost:<port>`. The smoke suite starts three worktrees of one
repository together — the main checkout, a branch, a detached HEAD — each serving guest port
3000, and checks that all three are reachable at three different names, each serving its own
tree, and that `down` removes exactly one route. Running portless rather than reading about it
found the two things boxer has to do itself: `portless alias` does not apply the worktree prefix
that `portless get` does, and a detached worktree gets no prefix at all.

**An exit code that depended on whether anyone was watching.** `boxer run -c "exit 4" </dev/null
>/dev/null` reported 1 on docker: boxer took `/dev/null` — a character device — for a terminal,
asked for a TTY, and the runtime refused before the command ran. That was the `capsule replay`
failure previously put down to a start race; capturing the output to look at it was what made it
pass. It is fixed, it has its own smoke cell with output discarded on purpose, and a runtime's own
failure is now reported as an error rather than as the command's exit status.

**boxer cleans up after itself.** Deleting a sandbox removes its host state — last-used stamp, run
record, lock, fork marker, URLs — and `gc` sweeps state that belongs to no sandbox after a week,
which is what catches a sandbox removed by `docker rm` or a wiped store. Before this a host that
had run the benchmarks for a few weeks held over 3,800 such files for sandboxes long gone.

Measured on Apple Silicon with smolvm 1.16.1: cold start 652 ms from a host pack, warm `boxer run`
31 ms, rewrite hook 3 ms. The benchmark suite puts that in context: boxer is about 8x faster than
a fresh container per command (`docker run`), **level with `docker exec` into an already-running
container** (0.96x-0.99x on every dispatch-bound workload, 2.3x-2.6x behind on I/O, which is the
worktree mount), and still far behind the seatbelt sandboxes Codex and Claude Code use on macOS,
which do not cross a kernel boundary at all. boxer's own cost above raw `smolvm machine exec` is
+13 to +22 ms on every workload — not a rate but a toll, because spawning `smolvm` costs ~10 ms
before it does anything. Profiling this slice removed three such costs: a `git status` and a
redundant `machine status` run *after* the command had already returned, and a `git rev-parse`
run before it that cost more than the sandboxed command itself. A warm no-op went from ~93 ms to
~31 ms, of which under 1 ms is now boxer's own code — an empty Go binary starts in 2.1 ms on this
host and `boxer version` in 2.8 ms, so the wrapper is within a millisecond of the floor.

**What the boundary costs, across all four backends.** One backend at a time, alone on the host,
three samples each, every cell from a wiped host, one Next.js dev server per worktree at 2 vCPU /
4 GB (`bench/backends.sh`, raw cells in `bench/backends.jsonl`):

| backend | boundary | one worktree | three worktrees |
| --- | --- | ---: | ---: |
| `podman` | namespace | **3.6 s** | 25.9 s |
| `docker` | namespace | 4.5 s | **21.5 s** |
| `container` (Apple) | kernel | 8.2 s | 25.6 s |
| `smolvm` | kernel | 8.5 s | 27.3 s |

The two kernel-per-sandbox backends land within 0.3 s of each other and the two shared-kernel ones
within 0.9 s, which is the useful form of the claim: **the ~4.6 s is the boundary, not smolvm**. A
second, independent microVM implementation agreeing to within 4% is evidence smolvm alone could
not provide, and it is the reason Apple `container` was worth building. At three worktrees the
spread collapses to 27% — three Next.js compiles saturate ten cores and the sandbox stops being
the bottleneck — so the boundary is a fixed startup toll rather than a running tax. Variance
tracks the boundary too: docker held +/-0.2 s across six cells where smolvm moved +/-2.3 s.

Unexplained, and left that way: podman wins one worktree and loses three to docker, on the same
driver and the same argv. No memory column, because that measurement varied 2.1x within one
configuration and any ratio from it would be noise.

Per-command latency is, however, the wrong thing to worry about, and the bring-up benchmark
(`make bench-devserver`) is where this slice found real defects. With every condition pinned —
same image, 10 vCPU and 8 GB to boxer and Docker alike, private npm cache, identical lockfile,
build cache cleared, a worktree copy each — starting a session against a prepared worktree is
**7.6 s for boxer, 10.8 s for the container runtime, 6.8 s for no sandbox at all**. Installing
dependencies is still the weak row at 9.2 s against the container runtime's 5.2 s. The container
runtime here is OrbStack, not Docker Desktop — the fastest on macOS rather than the most common.

Three defects were found and fixed to get there, and none of them was the microVM:

- **Every DNS lookup in a guest took 415 ms.** smolvm points a networked guest at public
  resolvers, so each name was an internet round trip with no caching; boxer now passes the host's
  own caching resolver and lookups cost 1-4 ms. Worth ~3.5 s of every bring-up.
- **The detected images were the fattest published.** `node:24-bookworm`, not the slim variant,
  and image size is microVM start-up time: 9.5 s against 2.5 s. Six seconds on every `boxer up`.
- **Two pieces of work ran after the command had already returned**, delaying the result for
  caches nothing waits on.

What is explicitly *not* the cause, having each been measured: the worktree mount (dev server
from the mount and from guest disk are both 7.9 s), CPU (boxer's guest beats Docker on both
parallel compute and small-file writes), and boxer's own code (~2 s of the 7.5 s, three smolvm
calls and six short guest commands). Two things remain open and unattributed: a restart gains
nothing from a warm build cache where Docker halves, and installs are about twice Docker.
[bench/DEVSERVER.md](../bench/DEVSERVER.md) has the detail. (A loaded host during the 1.1 work read 865 ms / 135 ms / 15 ms on the
same machine, which is what the generous smoke bounds are for: they catch a regression, not
jitter.) A second worktree of a repository with a `setup` list now starts in
0.7 s with its dependencies already installed, against 2.9 s before environment packs.

The publishing path is proven rather than asserted: `v1.0.0-rc.1` published four platform
archives, four SBOMs, the plugin and skill tarballs, the npm tarball and a `checksums.txt` signed
keylessly against the release workflow's OIDC identity. The signature verifies with the command in
[release.md](release.md), and both download routes install that release on a machine that had no
boxer: `install.sh` and `npm i -g` each report `boxer 1.0.0-rc.1`.

One cell has failed once in nine runs: `acp-codex/inside/acp/worktree`, after its own retry, with
`agent closed: EOF` and an empty stderr. It has passed every run since, including both runs of the
release pair. The driver now carries the agent's last output into the failure, so the next
occurrence explains itself rather than needing another nine runs.

Added 2026-09-18 (stream C): GitHub Copilot CLI as the ninth harness — **T1 4/4, T2 3/3 live for
$0.0092** — and a real herdr driver replacing its checklist — **T1 pass, T2 pass for $0.0039**.
Copilot runs at t2 with no Copilot seat: its BYOK provider variables point it at the gateway.

Skips, all of them explained: Gemini CLI and its inside and ACP cells need `GEMINI_API_KEY`
(the gateway has no Gemini-protocol endpoint), the six noncompliant cells are scripted and run at
t1 only, and Multica needs an account (`multica setup`).

The two tables below are the record of that 2026-09-18 run, kept for its timings and costs. They
are not the current results: those are the site's `evals/results` page, the one copy the
[documentation map](documentation-map.md) allows.

| Harness | Outside rewrite | Outside tool | Inside shell | ACP | T2 live |
| --- | --- | --- | --- | --- | --- |
| Claude Code | pass | pass | pass 13 s | pass 13 s | 7/7 live; session and subagent isolation and the four worktree timings pass at t1 |
| Codex | pass | pass | pass 22 s | pass 14 s | 2/2 live (gateway, no ChatGPT login) |
| Gemini CLI | pass | pass | pass 10 s | pass 13 s | skip: needs `GEMINI_API_KEY` (only harness the gateway cannot serve) |
| OpenCode | pass | pass | pass 11 s | pass 11 s | 3/3 live |
| pi | pass | pass | pass 15 s | no ACP server | 3/3 live |
| Kimi | block-only hooks; shims | pass | pass 17 s | pass 14 s | 2/2 live |
| GitHub Copilot CLI | pass (user hooks; `modifiedArgs`) | pass | pass (1.1; its guest needs CA certificates, like Codex) | `copilot --acp` | 3/3 live for $0.0092 through BYOK on the gateway — no Copilot seat needed |
| Grok | pass (user and project hooks) | live: one denial then recovery | pass 13 s | pass 14 s | 4/4 live |
| DSH | deny-only hooks through the `dsh-hooks-claude-code` bridge; shims | pass | not in table | none | t1 3/3, t2 2/2 live; tool mode only (the bridge ignores `updatedInput`), and boxer provisions on `UserPromptSubmit` because its `SessionStart` is detached |

Inside timings are for a host that already holds the harness pack; the first `boxer shell <h>`
per host pays the install once (10 s to 11 min depending on npm) and packs the result.

| Orchestrator | Path | Result |
| --- | --- | --- |
| OpenHands | terminal `shell_path` = `boxer-bash` (`boxer shim install --shell`), real SDK 1.49 | t1 pass; **live pass** through the gateway (agent's terminal ran in the guest); the earlier `BoxerWorkspace`-only design did not sandbox the agent's terminal |
| Paperclip | project layer over `claude-agent-acp`, one git worktree per issue | driver (`orch_paperclip.go`): T1 pass 21 s; **live 2026-09-18 pass, 2 m 4 s, $0.0364** |
| T3 Code | project layer, and the instance's `binaryPath` on a `boxer shim` (inside) | driver (`orch_t3.go`), two cells: T1 pass 9 s / 18 s; **live 2026-09-18 pass, 18 s $0.0047 and 27 s $0.0070** |
| Multica | project layer; harness path through `multica runtime profile create --command-name` + `runtime profile set-path`, or `MULTICA_<PROVIDER>_PATH` | checklist: a driver needs a self-hosted Multica server (documented and scriptable, not attempted) |
| herdr | project layer in a pane over the socket API (`herdr server`, `workspace create`, `pane split`, `agent start/prompt/read`) | driver (`orch_herdr.go`): **T1 pass 8 s; live 2026-09-18 pass, 15 s, $0.0039**. Also level S (`terminal.default_shell` = `boxer-bash`) and a validated `herdr-plugin.toml` (`start-agent` action, `worktree.created` → `boxer up --detach`). `herdr integration install claude` and `boxer install claude-code --user` coexist in `~/.claude/settings.json`, verified both orders |
| Conductor (local) | `.conductor/settings.toml` (**not** the legacy `conductor.json`): `claude_code_executable_path`/`codex_`/`opencode_` on boxer's harness shims plus `scripts.setup` warming the sandbox, written by `boxer install conductor` | checklist: local workspaces have no API |
| ACP real client | `@agentclientprotocol/sdk` example client against `boxer acp claude` | pass: initialize, session, prompt, `Terminal` tool, `Linux` |

## Added after the crabbox review (2026-09-21)

Ten cells were added to `make smoke`, all passing against real smolvm, because every one of them
exercises something the fake can only assert: JUnit summaries, `--fail-on-test-failures`, the run
record, capsule replay, a named pack surviving `gc --all` and rebuilding a sandbox, the fork
refusal, a shared fork running in the guest, a repeated fork, and reclaim.

They earned their place immediately. Six real defects reached the real-VM tier and nothing else:

- **A batch branch hangs for ten minutes.** `machine branch --name-prefix`/`--count` waits for the
  source's workload to call `smolvm-branch-ready`, which an ordinary development sandbox never
  does; smolvm then gives up after 600 s. Only a single `--name` branch checkpoints the source
  wherever it happens to be. `vm.Branch` now takes one named branch per child, and the fake
  refuses a batch outright so a future change cannot quietly reintroduce the hang.
- **`boxer fork` crashed when there was nothing to fork.** `--count` is how many children the scope
  should have, so asking twice for two is a no-op — and it indexed `names[0]` on an empty slice,
  and emitted JSON `null` where a caller expects an array.
- **A data race in the run record.** `os/exec` gives stdout and stderr one pipe when they are the
  same writer; teeing each into its own `io.MultiWriter` split that into two goroutines writing one
  caller buffer. Caught by `go test -race`, not by smoke.
- **The egress schema was guessed wrong, and the feature silently did nothing.** smolvm 1.16.1
  returns `{"timestamp","operation","dest"}` — `resolve` for a blocked lookup, `connect` with a
  `host:port` for a blocked connection — and every event it returns is already a denial, so there
  is no `allowed` flag. boxer was unmarshalling into `{host,port,allowed,at}`, which left every
  host empty and reported nothing. The fake had been fed the same invented shape, so it agreed.
- **Denials were attributed to the wrong command.** The machine remembers every denial, so a
  failed run named hosts refused by earlier commands while saying "during this command". Denials
  are now filtered to the run's own window; `doctor`, which is asked about the sandbox rather
  than a command, still takes the lot.
- **The fake invented ownership labels.** Every unlabelled machine came back from the fake's
  `machine ls` wearing a `boxer.scope` label, so no test could see that `gc --all` would delete a
  machine boxer never created. Removing it is what made the ownership test mean anything.

Two more were in the smoke script rather than in boxer, and both are the same trap the gc checks
were already commented for: `boxer … | grep -q` makes boxer die of SIGPIPE, and `pipefail` then
fails a check that passed.

**A second planned design was cut by the evidence.** `boxer fork` was designed to give each child
a staged copy of the worktree, so parallel children could not clobber each other, and that was
the default. smolvm refuses to branch a staged mount at all — *"multiple descendants would
synchronize into the same host directory"* — and the refusal arrives only after the staging half
has already happened, leaving a sandbox that no longer sees host edits and still cannot fork. The
smoke cells missed it because they all used `--shared`. Staged mode, `[fork] mode`, `--shared`,
`boxer fork sync` and the brief's staged rewrite are all gone; a fork is now a warm worker over
the parent's live worktree, and the guide says plainly what that is and is not good for.

**One planned change was reverted by the evidence.** `start` was moved onto smolvm's own
persistent workload (`machine create --init`), which is tidier and deletes a marker file. The
flow tier failed both cells: `--init` launches when the *machine* starts, which is before `setup`
has run, so `start = ["cd app && npx next dev"]` against a worktree that `setup` scaffolds died
immediately and was never relaunched. `start` means "after setup" and only a post-setup launch
can mean it. The nohup-and-marker implementation is back, with a comment saying why, and the flow
tier passes again.

**T1 was run three times on this slice.** The third is the one that counts:
**68 pass, 0 fail, 1 skip** — the recorded baseline, with `acp-codex` failing once and passing on
its own retry, and Multica skipped for want of a server as always.

The first two runs each lost one cell, a different one each time, and both passed on every re-run
in isolation (acp-codex 3/3, copilot 15/15). The copilot one —
`copilot/tool/user/worktree/noncompliant`, "no VM after the run" — is worth naming because the
diagnosis is not "flake": the machine was still on the host afterwards in state `created`, which
is the race OpenCode's driver already documents and guards. A turn whose only tool call is denied
can end while `boxer hook` is still provisioning, and the oracle then looks too early. Copilot's
driver now waits for its hook the same way OpenCode's does.

One hedge was removed rather than kept: `machine branch` *does* copy the source's labels onto a
child (verified against 1.16.1), so a fork is owned by the ordinary label rule and `Owned` needed
no special case for it. The name pattern stays, but only for telling a child from a sandbox when
deciding what `fork sync`, `fork rm` and the start-drift recreate may touch.

## Flow

`boxer-eval --tier flow` (`make eval-flow`), added in 1.1: one development session end to end
rather than one command. A pinned Next.js 16 app is scaffolded and installed inside the sandbox,
served by `start`, waited for by `ready`, reached from the host through a forwarded port, asked for
its tools over MCP by `next-devtools-mcp` running *in the guest* and addressed through `boxer run`,
then restarted to prove the environment pack made the install unnecessary.

Two cells, both **pass**; re-run on 2026-09-21 after this slice at 1 m 10 s and 1 m 37 s for
$0.0110. The agent cell's turn cap went from eight to twelve in the same change: a warm sandbox
already has the dev server running, so the session spends its first turns discovering that
`npm run dev` refuses, and it was running out of turns holding the right answer. The cap was
measuring the budget rather than the integration. Report: [eval-flow.md](eval-flow.md). Slow and network-heavy, so it runs on demand and
before a release, never in the default gate.

The agent cell is the one that settles the capability boxer had and never claimed. Claude Code was
given an ordinary entry — `{"command": "boxer", "args": ["run", "--", "next-devtools-mcp"]}` —
and its own session record reports the server **connected**, listing
`mcp__next-devtools__nextjs_index`, `nextjs_call`, `nextjs_docs` and `browser_eval` beside its own
tools with boxer's skill loaded. It then made eleven calls to them, found the dev server on its
port, asked for `get_routes`, and answered with the scaffold's real routes. It also read boxer's
injected brief mid-session and moved from a host path to `/workspace`.

**An MCP server that must live beside the code runs in the sandbox, and the harness cannot tell the
difference.**

## Development lifecycles

`boxer-eval --tier sdlc` (`make eval-sdlc`), added in 1.1: twelve development lifecycles, three at
a time, each in its own linked worktree with its own sandbox, dev server and port. A live agent does
ordinary work — a page, an API route, a client component, a dynamic route, server data, metadata, a
layout change, a CSS module, an error boundary, a dependency install, a dev-server restart, and a
fix driven by the framework's own diagnostics — with boxer's project layer, the Next.js MCP tools
running **inside** the sandbox, and `agent-browser` on the host. Nothing is taken on the agent's
word: the page is read from the host through the forwarded port and the change must be in the
worktree.

**12 of 12 pass** on 2026-09-19 for $0.33, slowest lifecycle 2 m 29 s. Across them: 145 tool calls
— 99 shell commands in the sandbox, 19 browser page reads, 15 calls to the MCP server running in
the guest — with every worktree given its own host port and none colliding. Report, per-lifecycle
detail and findings: [eval-sdlc.md](eval-sdlc.md).

## The same workload at every integration level

`boxer-eval --tier matrix` (`make eval-matrix`), added in 1.1: the SDLC workload again, but with the
integration level as the variable instead of the task. Seven levels — hook rewrite, tool mode,
command shims, bash shim, shell substitution, inside mode and orchestrator — across every harness
and orchestrator, each cell in its own worktree with its own sandbox and automatic host port. A passing task
is not enough: on the discriminating task each cell must also show its level's signature in boxer's
own trace (a rewrite, a denial, or the pointed absence of either), and `where.txt` must say `linux`,
which is how a level that quietly ran on the host is caught. Tier and exclusions:
[eval-matrix.md](eval-matrix.md).

**94.0% of weighted checks on 2026-09-21, 47 of 55 cells at 100%**, 2 skipped, for $1.56, three at
a time, every cell on `zai/glm-5.3-flash` so that the level is the only variable. Every level is
carried by more than one harness except shell substitution, where OpenHands is the only harness
with a configurable terminal shell. Each cell is scored on every claim it makes rather than passed
or failed as a whole, so a shortfall names the claim.

By level: rewrite 100% (12/12), shell 100% (1/1), tool 96.5% (9/10), bash-shim 95.7% (7/8), inside
93.0% (8/10), command shims 91.3% (6/8), orchestrator 79.7% (4/6). The two cells worth reading
rather than counting are `paperclip/orchestrator/*`, where the orchestrator's own run failed, and
`inside/fx/prove-and-install`, where the model recorded no platform and installed nothing.

The earlier 2026-09-20 run scored 99.2% over 53 cells for $3.06. It is not comparable: the model,
the cell set and the level set all changed. A score is only comparable with one made under the
conditions printed beside it.

Getting there found three faults in boxer the single-command tiers could not see. **Codex was never
intercepted in a linked worktree**, so every command ran on the host and every orchestrator was
affected. **`boxer run` discarded input written before the sandbox attached**, silently losing the
shell configuration a harness writes immediately after spawning its terminal. And **a boxer shim
could be resolved by boxer itself** — the smolvm launcher runs `uname` — which deadlocked a whole
session. All three are fixed, with tests. One limit is reported rather than fixed: PATH shims do not
survive a login shell, and `boxer doctor` now says so.

Concurrency is the point: twelve worktrees, three sandboxes at a time, no clash in files, ports,
packs or state, and no command reached the host. Measured cost of four concurrent sandboxes of a
node project: **about 4 GB resident and 2.7 GB of disk**, roughly 1 GB and 0.7 GB each, against a
default allocation of 4 GB apiece. Allocation is what a host has to be able to promise, so four
defaults do not fit on a 16 GB machine even though their resident use would.

## Adherence

`boxer-eval --tier adherence`: does a live model follow the injected brief when the prompt never
mentions boxer? Four models, 32 cells each, all 128 run live on 2026-09-22. Full matrix, spend and
per-cell findings in [eval-adherence.md](eval-adherence.md). A cell is blamed on the **harness**
only when every model fails it; one failure among passes is that model's adherence.

| Verdict | Cells |
| --- | --- |
| pass | 16 |
| model adherence | 12 |
| **harness** | 4 — `grok/tool/…/brief`, and the `task` cell on Copilot, Grok and Kimi |

The fourth scenario, `task`, is new in this slice and is the one that found something. It declares
a `[tasks.test]` with a description, tells the agent nothing about boxer, and asks for the test
suite. **Five of eight harnesses used the declared task; Copilot, Grok and Kimi never did, on any
model.** All three are harnesses where the brief does not reliably reach the model, which was
already documented for Grok and Kimi — Copilot is new. Twelve further cells split, working for
some models and not others. So "declaring a task with a description gets it used" is true on most
harnesses and false on three, and the documentation says the specific thing rather than the
general one.

That is also the second independent observation of models composing command lines instead of
naming declared tasks, which is the trigger [plan-1.1](plan-production.md) set for adding a
`boxer_task` MCP tool. The trigger is met; the tool is not in this slice.

The cell took three attempts to become trustworthy, which is worth recording. The first version
left composed commands uninterceptable, so they ran on the host and the cell reported
`leak: command ran on the host` — the signal that everywhere else means a sandbox escape. The
second narrowed `intercept` to `sh`, which *removed* `uname` from the default list and moved the
false alarm rather than fixing it. Only `intercept = ["*"]` works: both paths land in the guest
and the sole variable is whether the task was named. No named list can do it, because whatever it
holds, a composing agent picks something else. The final run has **zero `leak:` findings in 128
cells**.

Grok is the one harness finding, and it is the same one as before: Grok never surfaces
session-start context to the model, so in tool mode the first shell command always costs one
denial before the agent learns about `boxer_run`. Rewrite mode is the right Grok default, and its
dialect records this. Copilot's brief cell fails for three of four models, all with one denial and
a recovery, which is model adherence rather than a harness limit: its `recovery` cell passes for
every model.

| Model | Pass | Fail | Spend |
| --- | --- | --- | --- |
| zai/glm-5.3-flash | 26 | 6 | $0.34 |
| anthropic/claude-haiku-4.5 | 20 | 12 | $1.78 |
| alibaba/qwen3.7-flash | 24 | 8 | $0.49 |
| deepseek/deepseek-v4-flash | 25 | 7 | $0.61 |

No command reached the host in any of the 128 cells. `make eval-adherence` runs all four models;
the two-model rule needs more than one, so a single-model run can never reach a verdict.

## Shipped in this slice

Fourth pass (2026-09-18), the work towards a first public release:
- **Published content is static.** The package and the skill are byte-identical for every user but
  for the release version, and a test renders them twice under hostile configurations to prove it.
  Configuration reaches the agent at run time instead: `boxer brief [--json]`, the same text hooks
  inject, also served as an MCP resource.
- **The skill carries its `scripts/` layer** (`run`, `task`, `status`, `brief`), which is the
  deterministic command path the Agent Skills specification defines and the answer to a harness
  that hides MCP tools behind a dispatcher.
- **Named tasks.** `[tasks]` in `boxer.toml`, `boxer tasks`, `boxer run --task <name>`: the agent
  invokes a name the repository declared rather than composing a line the intercept list has to
  catch. An unknown name is refused with the real ones.
- **An opt-in event stream.** `[telemetry]` with file, stderr and OpenTelemetry sinks, `boxer logs`,
  an event tail on `boxer status --json`, command lines elided unless asked for. Off by default.
  Schema in [events.md](events.md).
- **GitHub Copilot CLI** as the ninth harness, its dialect corrected against a running binary, and
  a real herdr driver and plugin replacing that checklist.
- **Release engineering:** Apache-2.0 and the community files, a CI gate with the race detector,
  per-package coverage floors, lint and vulnerability scanning on two platforms, reproducible
  builds, SBOMs, keyless signing, the npm launcher the package always declared but never shipped,
  and the four install routes exercised against a staged release.
- **An unknown top-level table in `boxer.toml` warns rather than fails**, so a repository that
  adopts a newer boxer's table still loads under an older binary.

Third pass (2026-09-18): `warm_on_session_start` (SessionStart returns in 9 ms, VM ready before the
first tool call), `worktree.manage`, `boxer install git` (post-checkout warm-up), `doctor` signal
report, rewrites carry `--session/--agent` under those isolations, Agent Plugins 1.0.0 package
(`boxer package plugin`, validated in Claude Code, Codex, Gemini, Grok; schema-conformance test),
MCP lifecycle signals (detached warm-up on `initialize`, last-used on EOF), MCP `cwd` mapping from
guest paths, install retry on a dropped exec, adherence tier with the two-model rule, gateway spend
tracking and a per-run budget, provider rate limits reported as skips, concurrent-creator wait
when two boxers race smolvm, Codex guest gets CA certificates.

First and second pass:
- `pkg/boxer` facade (experimental), `--json` on `ls`, `status`, `down`, `gc`, `doctor`.
- goreleaser, `install.sh`, npm wrapper `boxer-cli`, CI and release workflows, plugin version
  stamping with a `doctor` mismatch warning.
- Image packs once per host, harness packs once per host per harness (keyed on image, harness
  and install line), pruned by `gc`; install verified with `<bin> --version` before packing.
- Nested-sandbox rows for Codex; audit clean for Gemini, OpenCode, pi, Grok, Kimi.
- Login-travel warning for Claude; Codex `auth.json` travels; Gemini uses Keychain on macOS.
- Eval runner: infra-failure retry, kept failures, SIGINT report, host lock; T2 tier with
  credential detection; Grok driver; OpenHands and orchestrator checklist drivers.

## Known limits

- One host drives one smolvm at a time (`~/.local/state/boxer/eval.lock`).
- The first `boxer shell <harness>` on a host pays the harness install (10 s to 11 min, npm over
  smolvm's TSI networking); every later worktree starts from the host pack in seconds.
- Claude Code's Keychain login does not enter the VM (`claude setup-token`); Gemini likewise.
- Grok does not surface session-start context to the model, so tool mode costs one denial on the
  first shell command; rewrite mode is the right Grok default. Recorded in its dialect.
- DSH has no session-end signal, so nothing reclaims its sandbox at the end of a session; MCP EOF
  and `gc` are the fallbacks.
- Under `session` and `subagent` isolation the MCP run tool resolves the worktree scope (MCP
  carries no ids); hooks carry them.
- An orchestrator that creates a worktree and launches a harness into it in one step wants
  `boxer install git`: T3's provider gives up while a cold VM boots.
- Packs are 130 to 365 MB each. `gc` prunes them by `idle_timeout`; a long eval session can fill a
  small disk before that runs, and two back-to-back T1 runs have done exactly that.
- DSH has no inside-mode row: it is a plugin stack rather than a single binary, so there is nothing
  for `boxer shell` to launch. That is a gap in the table rather than an untested claim.
- Conductor and Multica remain checklists: local Conductor workspaces have no API, and a Multica
  driver needs a self-hosted server.

## To run the rest

```sh
# evals/.env (gitignored), then: make eval-t2
AI_GATEWAY_API_KEY=…             # one key: every harness runs live through the Vercel AI Gateway
GEMINI_API_KEY=…                 # optional: Gemini CLI only (the gateway has no Gemini-protocol endpoint)
BOXER_EVAL_MODEL=…               # optional: one gateway model id for every harness (default: cheapest per family)
npm i -g paperclipai t3          # orchestrator drivers
multica setup                    # account
python3 -m venv .venv-openhands && .venv-openhands/bin/pip install openhands-sdk openhands-tools
```

## What comes next

devcontainer.json loader, `Backend` interface (Firecracker, Docker Sandboxes), a running-boxes
dashboard on the JSON API, Homebrew tap, plugin marketplace repos, Conductor cloud once smolvm
exists there. Landed on the `packaging` branch 2026-09-17: the Agent Plugins package collapse
(R-LVL-6, R-LVL-6a) and MCP lifecycle signals (R-SIG-0).
