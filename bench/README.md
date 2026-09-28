# Benchmarks

Written for: anyone deciding whether boxer is fast enough to leave switched on, and anyone who
wants to check that claim rather than take it.

`bench/bench.sh` measures what an agent waits through when boxer intercepts a tool call, against
every other sandbox that could plausibly be in that position. `bench/report.py` turns the raw
`results.jsonl` into [RESULTS.md](RESULTS.md). Both the harness and the results are committed so
a number in the documentation can always be traced to the run that produced it.

Numbers here are from one developer machine, and hardware, host load and image cache state all
move them. Re-run before citing them anywhere that matters.

## The question

Two different questions, and conflating them is how benchmarks lie.

1. **What does boxer cost?** An agent that runs `npm test` unsandboxed is the baseline every user
   silently compares against. The honest number is the latency boxer adds to a command that would
   otherwise have run on the host, and the part of that which is boxer's own code rather than the
   VM underneath it.
2. **Is anything faster?** Only against sandboxes that a person would actually choose instead —
   which means being explicit that they do not all isolate the same thing.

## Contenders

| | What it is | Isolation boundary |
| --- | --- | --- |
| `host` | No sandbox. The denominator. | none |
| `seatbelt` | `sandbox-exec` with a workspace-write profile. The mechanism behind **Claude Code's and Codex's native sandboxes on macOS**. | process, path policy, shared kernel |
| `codex sandbox` | The Codex CLI's shipped sandbox subcommand. Seatbelt underneath. | process, path policy, shared kernel |
| `docker run` | A fresh container per command. How a per-tool-call container sandbox behaves. | container; on macOS, inside a Linux VM |
| `docker exec` | One long-lived container, exec per command. The fair warm comparison. | container; on macOS, inside a Linux VM |
| `smolvm` | `smolvm machine exec` straight into boxer's own VM, bypassing boxer. | microVM, separate kernel |
| `boxer` | `boxer run`. The product. | microVM, separate kernel |

The `smolvm` row exists to split boxer's cost in two. The gap between `smolvm` and `boxer` is
boxer's own code — config resolution, scope hashing, the host lock, the run record — and it is
the only part boxer can do anything about. Everything below it is the platform.

### Firecracker

Not benchmarked, because it does not run here: Firecracker requires KVM, so it cannot run on
macOS at all, and this repository's supported host is macOS on Apple silicon. On a Linux host the
comparable row is Firecracker through a manager such as Ignite or a Kata runtime, and the shape
of the answer should look like the `smolvm` row — a microVM boundary with a per-boot cost and a
cheap warm exec. Adding that row needs a Linux host in the harness, which is tracked as future
work rather than guessed at here.

### Boundaries are not comparable, and the table cannot say so

Seatbelt and boxer are not doing the same job. A seatbelt profile is a path-based policy enforced
by the host kernel around a host process: it shares the kernel, the network stack and the process
namespace with everything else on the machine, and it is bypassed by any kernel bug or any path
the profile did not anticipate. boxer runs the command in a different kernel.

So a row where seatbelt is faster than boxer is not a row where seatbelt is better. It is the
price of the boundary. The table reports latency; choosing what to isolate is a separate
decision, and [the security documentation](../SECURITY.md) is where that argument belongs.

## Method

- Wall clock from just before `fork` to just after reap, which is what the agent actually waits
  for: process spawn, any daemon round trip, and the work.
- 20 samples per cell by default (`BENCH_REPS`), two discarded warmups, median reported and p90
  shown when it is more than 25% above the median.
- Every contender runs the identical POSIX `sh` line with the corpus as its working directory, so
  workloads can only use relative paths. That is what keeps the mount translations honest: the
  host sees the corpus directly, boxer and smolvm see it at `/workspace`, Docker sees it at `/w`,
  and no workload can tell which.
- The corpus is generated, not committed: 400 files across 20 directories, about 1.6 MB.
- The whole run takes the same host lock as the smoke and eval tiers. A benchmark sharing a
  machine with an eval measures the eval.
- Output is discarded, so a chatty contender is not charged for the terminal.

### Workloads

| | Measures |
| --- | --- |
| `noop` (`true`) | Pure dispatch. The floor, and the number that dominates agent tool calls. |
| `stat` | One directory listing — the cheapest thing that crosses the filesystem boundary. |
| `read` | `grep -rl` over the whole corpus. Mount read throughput, where virtiofs shows up. |
| `write` | Create and delete 200 files. Mount write throughput, usually the worst case. |
| `cpu` | A 50k-iteration shell loop. No syscalls of consequence: isolates CPU overhead from I/O. |

Cold start is measured separately and once, because provisioning is slow and too variable for
cheap averaging. `docker run`'s warm and cold numbers are the same measurement by construction —
that is the point of the row.

## Running it

```sh
make bench                        # build, then bench/bench.sh, then render RESULTS.md
BENCH_REPS=50 bench/bench.sh      # more samples
BENCH_ONLY=host,boxer bench/bench.sh
```

Requires a real smolvm and, for their rows, Docker and the Codex CLI. Contenders that are not
installed are skipped and named in the output rather than silently dropped.

## Findings

Written against the committed [RESULTS.md](RESULTS.md); re-derive them if you re-run.

**We are not the fastest.** Seatbelt wins every dispatch-bound workload, by about 4x on a no-op.
It is not isolating what boxer isolates — it is a syscall filter around a host process, not
another kernel — but the latency is the latency.

**Against a warm container, boxer is now level.** `docker exec` into an already-running container
and `boxer run` into an already-running microVM cost 32 ms and 31 ms on a no-op, and boxer is
within 4% on every workload that is not I/O-bound. The mount is where the container runtime is
still ahead: 2.3x on a recursive grep, 2.6x on a 200-file write.

**We do beat the thing boxer is an alternative to.** `docker run` — a fresh container per
command, which is how a per-tool-call container sandbox behaves — costs about 253 ms against
boxer's 31 ms. A warm sandbox is the whole design, and it is worth about 8x here.

### boxer's own overhead is a toll per smolvm invocation, not a rate

The gap between raw `smolvm machine exec` and `boxer run` is **+13 to +22 ms on every workload** —
a no-op and a 200-file write pay about the same. Nothing proportional to the work or the data can
produce a constant like that.

The cause is that spawning `smolvm` costs about **10 ms before it does anything**, most of it
loading libkrun. So boxer's latency is governed by how many times it shells out, and a warm
`boxer run` now breaks down as:

| | ms |
| --- | ---: |
| boxer process, configuration, scope resolution | ~3.7 |
| `smolvm machine status` — does the sandbox exist? | ~10 |
| `smolvm machine exec` — the command | ~15 |

Under 1 ms of that is boxer's own logic: an empty Go binary starts in 2.1 ms on this host and
`boxer version` in 2.8 ms, so the wrapper is within a millisecond of the floor for *any* Go
program. There is nothing left to win on boxer's side of the line.

**The smolvm call count is the number to hold new code against.** Any feature that adds one to the
hot path costs ~10 ms of agent latency, whatever it is measuring.

Two costs have already been removed, and they are worth reading as a pattern rather than as
trivia — both were work done *after* the command had already returned, delaying the agent's
result for a cache nothing blocks on:

- The run record shelled out to `git rev-parse` and `git status --porcelain` once the command
  finished: 17 ms on an empty repository, 26 ms on this one. Now collected alongside the command,
  which is also more correct — a capsule replays the tree as it was when the command *ran*.
- The run record then made a **third** smolvm call, a second `machine status`, purely to read one
  label — asking for state the first call had already fetched in the same process. Now memoised.

A third, larger cost was not after the command but before it: every boxer command began by
spawning `git rev-parse` to find the repository, which is ~6 ms on a small repository and 9-18 ms
on this one — more than the guest command it was about to sandbox. boxer now reads the two
ordinary layouts straight off the filesystem in about 45 microseconds and falls back to git for
anything unusual.

Together: ~93 ms to ~31 ms for a warm no-op, a 67% cut, with no change to what boxer does.

One removable call is left: the existence check in `Ensure`, worth ~10 ms. Removing it means
running the command first and inferring "no sandbox" from the failure — and smolvm reports a
missing sandbox, a stopped sandbox and a guest command that exited 1 all as exit 1 with the
reason on the user's own stderr. Guessing wrong runs the command a second time, which for a
publish or a migration is far worse than 10 ms is good.
`internal/vm/probe_exec_test.go` asserts that ambiguity and fails if a future smolvm removes it,
at which point the optimisation becomes available. See [DEVSERVER.md](DEVSERVER.md) for the full
argument.

### The microVM boundary is real, and it shows up in I/O

Raw smolvm costs 135 ms to create and delete 200 files against the host's 28 ms. That is
virtiofs, and no amount of work in this repository moves it. It is the price of the kernel
boundary, and it is the one cost here that is genuinely inherent rather than incidental.

### Cold start is our worst number

721 ms, against `docker exec`'s 36 ms into an already-running container. It is paid once per
sandbox rather than once per command, which is why the warm row is the one that matters across an
agent session — but a first impression is a first impression, and this is it.

## Bringing up a development stack

`bench.sh` answers "what does one command cost". That is not the question boxer exists to answer.
boxer is for a long-lived sandbox over a git worktree with a dev server in it, so there is a
second harness — `bench/devserver.sh` — which scaffolds a pinned Next.js app, serves it, and
times how long until the port returns **HTTP 200**. Ready means an answered request: not a log
line, not a running process, because that is the only definition every contender can be held to.

### What is held equal

Every one of these produced a plausible *wrong* answer before it was pinned. They are listed
because the first three versions of this benchmark were each wrong in a different way.

| | |
| --- | --- |
| resources | 10 vCPU and 8 GB to boxer and Docker alike. boxer's own default is 4/4G and Docker Desktop's VM was 10/11.7G, which by itself made `npm install` look 1.6x worse than it is. |
| image | `node:24-alpine` everywhere. Image size *is* microVM start-up time, so two contenders on different images are measuring the images. |
| npm cache | a fresh private cache per install. Otherwise the first contender warms the host's shared cache and every later one looks fast. |
| lockfile | `package-lock.json` present and identical. Without it npm re-resolves the tree, which is slower for reasons unrelated to sandboxing. |
| `.next` | deleted before every start. Next.js caches compilation *in the worktree*, so a later contender reading a warm `.next` is measuring an earlier one. |
| worktree | one reference copy, built once, copied per contender — byte-identical inputs. |

### The four scenarios

| | |
| --- | --- |
| **first-ever** | Nothing cached anywhere: no pack, no pulled image. Paid once per machine. |
| **install** | Dependencies absent, scaffold present. This is the filesystem-bound part. |
| **cold-start** | Dependencies installed, `.next` empty, sandbox down. **The number that matters** — what starting a session costs, over and over. |
| **warm-start** | Dependencies installed, `.next` warm. A restart mid-session. |

`cold-start` is also where boxer's "pre-run" answer lives. `setup` runs once per worktree and
`image_setup` is snapshotted into a pack, so a project can arrange for `cold-start` to be the
common case instead of `install`.

### Results

[DEVSERVER.md](DEVSERVER.md) has the committed table, what was making boxer slow, and what turned
out not to be. In short: starting a session costs 7.6s against 10.8s for the container runtime
and 6.8s for no sandbox at all; installing dependencies is still about twice the container
runtime.

## Several sandboxes at once

`bench/parallel.sh`. One agent per git worktree means N sandboxes live together, so the question
is not what one costs but how the Nth behaves. This is boxer's primary use case and the other two
harnesses miss it entirely — the worst bug found in this whole effort only appears with N > 1.

Memory is measured as a **delta in host memory pressure** (active + wired + compressed pages), not
as the resident set of a process. A VMM maps the whole guest address space, so `ps rss` counts
memory the guest never touched: it read 807 MB for a sandbox whose real cost was a fraction of
that, and it cannot see memory smolvm has returned through virtio-balloon at all.

It is also measured whole-system for both sides, which is the only fair way. A container runtime on
macOS keeps one Linux VM resident whether or not you are using it and its containers are then
nearly free at the margin; boxer keeps nothing resident and pays per sandbox. Charging one side
only for its containers and the other for its whole VMs is the easiest way to make this table lie.

### At Vercel Sandbox's specification

Vercel Sandbox is the closest published thing to what boxer does — an agent's code in a
Firecracker microVM rather than in a container — so the shape worth matching is its default:
**2 vCPU and 4 GB**, its ratio being fixed at 2 GB per vCPU. Each sandbox gets its own *linked git
worktree* of one repository, which is boxer's real shape and not the same test as N unrelated
repositories: linked worktrees share a single `.git`.

These come from `bench/parallel-clean.sh`, which measures **one cell per invocation from a wiped
host**: every container removed, the image cache pruned, the runtime restarted so its VM is back
to idle size, boxer's sandboxes destroyed, and the image pulled and the pack rebuilt *before* the
baseline is taken. Host: Apple M4, 10 cores, 24 GB. No cell swapped.

| sandboxes | boxer, ready | boxer, host memory | OrbStack, ready | OrbStack, host memory |
| ---: | ---: | ---: | ---: | ---: |
| 1 | **7.6s** | 1,024 MB | 10.2s | **748 MB** |
| 3 | 22.3s | **1,571 MB** | **18.8s** | 2,014 MB |
| 5 (median of 3-4) | **26.3s** | 3,539 MB | 27.1s | **2,227 MB** |

**Time is a wash beyond one sandbox.** boxer is a third faster to a first response at N=1, behind
at N=3, and level at N=5. Past the first sandbox the clock is Next.js compiling on ten shared
cores, not the sandbox around it.

**Memory: this harness cannot answer the question, and the numbers above should not be quoted.**
The measurement is a delta in whole-host memory pressure, which attributes to the contender every
byte the machine moved during a 25-second window. Repeated, it does not hold still:

```
boxer  n=5, 4 GB guests   3,234 / 3,539 / 3,724 MB     spread 1.2x
boxer  n=5, 2 GB guests   2,442 / 4,293 / 5,107 MB     spread 2.1x
docker n=5, 4 GB          1,557 / 2,193 / 2,262 / 2,935 MB   spread 1.9x
```

Halving each guest's allocation produced both the lowest and the highest figure in the whole
table. Any ratio computed from this — 4x, 1.6x, anything — is beneath the noise floor. The
wiped-host protocol removed the ordering artifact it was built for; it did not make the metric
precise enough to compare runtimes.

The one memory measurement here that *does* replicate is taken from inside the guest, where the
same workload reads the same way across independent runs:

```
                         4 GB guest    2 GB guest
guest has touched          1182 MB       1196 MB
  of which anon             642 MB        654 MB   <- the dev server's own heap
  of which page cache       446 MB        446 MB   <- node_modules and friends
```

So: a sandbox serving this dev server needs about **1.2 GB inside the guest, over half of it
node's own heap**, and it does not grow to fill whatever it was given. That is a number worth
holding. What a sandbox costs the *host* relative to a container is not something this harness
establishes, and claiming otherwise would be making it up.

Measuring that properly needs per-VMM accounting that sees balloon returns — not a whole-host
delta — and enough samples to bound the variance. Until then the honest statement is that a
microVM per worktree must cost more than a shared kernel, by the size of a kernel, a guest
userland and a page cache each; how much more, on this workload, is unmeasured.

**Two earlier claims in this file were wrong and are withdrawn.**

1. *"~243 MB per sandbox, crossover at about six."* Measured on sandboxes that were not running a
   real workload. A sandbox serving a dev server costs about 1 GB, most of it node.
2. *"boxer uses four times OrbStack's memory at five sandboxes."* That came from
   `bench/parallel.sh`, which runs every cell in one process. A container runtime's Linux VM grows
   to hold what its containers touch and does not return it to macOS promptly, so by the later
   cells it was already large and its *delta* captured only marginal growth — it reported 501 MB
   for five Next.js dev servers, which is not physically possible. Measured from a wiped host the
   same cell is **2,193-2,935 MB**. The old harness understated it by up to 4.5x.

Read `bench/parallel.sh`'s numbers as time-only. Use `bench/parallel-clean.sh` for memory.

Caveats that remain, and they are not small:

- **N=1 and N=3 are single samples.** They disagree in direction with each other on memory, so do
  not read a trend into them. Only N=5 was repeated.
- **The container runtime's figure is the noisy one**: 1,557 / 2,193 / 2,262 / 2,935 MB across
  four clean runs, a 1.9x spread, because how much its VM has already grown still varies even
  after a restart. boxer's spread was 3,234-3,724 MB, about 1.15x.
- One host, one architecture. `docker exec` on macOS crosses a VM boundary too; on a Linux host
  this whole comparison would look different.

## Known limits

- One host, one architecture, one run. There is no cross-machine variance in these numbers.
- Docker on macOS runs inside its own Linux VM, so `docker exec` measures a VM boundary too, not
  a container boundary. On a Linux host that row would be far cheaper and the comparison would
  change. This is the single biggest caveat in the table.
- Image and pack cache state affect cold start more than anything else measured here.
- The `cpu` workload uses `sh` arithmetic, which measures the guest's shell as much as its CPU;
  Alpine's `ash` and macOS's `sh` are different programs. Treat it as a sanity check that
  virtualization is not costing CPU, not as a CPU benchmark.

## The backend matrix (2026-09-28)

The same script on build v1.0.0-55-g34ca729 (smolvm 1.16.1, docker 29.4.0 in OrbStack, `container`
1.4.1), three samples per cell, raw cells in `bench/backends.jsonl`. Podman was not rerun: no
podman machine was set up on the host.

| backend | n=1 samples | median | n=3 samples | median |
| --- | --- | ---: | --- | ---: |
| `docker` | 4.3 / 4.3 / 4.3 | **4.3s** | 10.9 / 11.7 / 10.2 | **10.9s** |
| `container` (Apple) | 5.3 / 8.3 / 5.3 | 5.3s | 16.2 / 14.4 / 14.1 | 14.4s |
| `smolvm` | 6.7 / 6.3 / 6.4 | 6.4s | 13.9 / 15.7 / 15.6 | 15.6s |

- **Every n=3 cell is 40 to 50% faster than on 2026-09-25, docker included.** The golden app
  (Next.js 15.5.4, created 2026-09-25) and the script are unchanged, and docker's start-up path
  did not change between the two builds, so the drop is host state rather than boxer. What
  differed on the host is not recorded, so the cause is not established. Compare rows within one
  run, not across the two.
- **The kernel boundary now costs about 1 to 2s on the first sandbox**: 5.3 to 6.4s against 4.3s.
- `container` again has one slow n=1 sample (8.3s) among two fast ones, as it did on 2026-09-25.

## The backend matrix (2026-09-25)

One backend at a time, alone on the host, three samples each at one and three worktrees, every
cell from a wiped host. `bench/backends.sh`, raw cells in `bench/backends-2026-09-25.jsonl`. macOS 26.5.2,
Apple Silicon, 10 cores; a Next.js dev server per worktree at 2 vCPU / 4 GB.

| backend | n=1 samples | median | n=3 samples | median |
| --- | --- | ---: | --- | ---: |
| `podman` | 3.6 / 3.7 / 3.6 | **3.6s** | 24.5 / 25.9 / 26.2 | 25.9s |
| `docker` | 4.5 / 4.3 / 4.5 | 4.5s | 21.3 / 21.5 / 21.5 | **21.5s** |
| `container` (Apple) | 5.4 / 8.2 / 8.2 | 8.2s | 25.6 / 25.6 / 25.5 | 25.6s |
| `smolvm` | 10.5 / 8.2 / 8.5 | 8.5s | 27.3 / 25.5 / 28.3 | 27.3s |

What it supports, and what it does not:

- **The kernel boundary costs ~4.6s on the first sandbox** — 8.2-8.5s for the two kernel-per-sandbox
  backends against 3.6-4.5s for the two that share one. Both microVM backends land within 0.3s of
  each other, which is the stronger form of the claim: it is the boundary, not the implementation.
- **At three sandboxes the spread collapses to 27%**, 21.5s to 27.3s. Three Next.js compiles on ten
  cores dominate and the sandbox stops being the bottleneck.
- **The shared-kernel backends are near-deterministic** (docker +/-0.2s over six cells) and the
  microVM ones are not (smolvm +/-2.3s at n=1). Boot variance is a property of the boundary too.
- **Not explained:** podman wins n=1 and loses n=3 to docker, on the same driver and the same argv.
  Its machine is `applehv` with different storage, but that is a hypothesis and is not tested here.

No memory column, on purpose. The whole-host measurement varied 2.1x within one configuration
earlier in this work, so any ratio drawn from it would be noise. See the withdrawn claims above.
