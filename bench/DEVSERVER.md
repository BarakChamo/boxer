# How fast is boxer for real development work?

Written for: anyone deciding whether to leave boxer switched on, and anyone who wants to check
the claim rather than take it.

The short version: **for the thing boxer is built for — starting a session against a git worktree
and getting a dev server answering — boxer is now more than three seconds faster than a container
runtime, and within a second of running with no sandbox at all.** It is still slower than both at
installing dependencies.

Every number here comes from `bench/devserver.sh` on one Apple M4. Re-run before citing it
anywhere that matters; `bench/README.md` has the method and the limits.

## The results

Time until a Next.js dev server answers an HTTP request, in seconds. Lower is better.

| | no sandbox | container runtime | boxer, before | **boxer, now** |
| --- | ---: | ---: | ---: | ---: |
| **Starting a session** (deps installed, nothing running) | 6.8 | 10.8 | 11.0 | **7.6** |
| **Restarting** (deps installed, build cache warm) | 4.7 | 5.7 | 10.8 | 7.8 |
| **Installing dependencies** (`npm install`, 9,140 files) | 3.5 | 5.2 | 10.5 | 9.2 |
| **First time on a new machine** (nothing cached at all) | — | 19.1 | 17.5 | ~24 |

The two left-hand columns are re-measured on every run and move about 5% between them, so read
the *gaps* rather than the digits: a 0.5s change in one cell is inside the noise, a 3s one is not.

The container runtime here is **OrbStack**, not Docker Desktop — it is what `docker` resolves to
on this machine, and it is the fastest container runtime on macOS rather than the most common one.
Worth knowing before reading "about the same as Docker" as a modest result.

Every row gave both sandboxes the same image, the same 10 vCPU and 8 GB, an empty npm cache, an
identical lockfile, a cleared build cache and its own copy of the worktree. Getting any of that
wrong changes the answer, and earlier versions of this benchmark got all of them wrong in turn.

**Starting a session is the row that matters.** It is what you pay every time you sit down, and
boxer is now faster than the container runtime at it and about a second behind no sandbox at all —
in exchange for the command running in a different kernel.

**Installing dependencies is our weak row**, at about twice Docker. It is also the row you pay
least often: `setup` runs once per worktree and `image_setup` is snapshotted into a pack, so a
configured project does this once and then lives in the first row.

**The first-time row is noisy** — it is a single sample dominated by building the image pack,
which measured anywhere from 13 to 26 seconds across runs. Treat it as "about twenty seconds,
once per image per machine", not as a precise figure.

## What was making it slow

Six real defects, all found by measuring rather than reasoning, and all fixed. Between them they
took starting a session from 11.0s to 7.6s, a warm one-off command from 93ms to 31ms, and four
parallel provisions from 27s each to 2.7s.

### 1. Every DNS lookup in the guest took 415 milliseconds

The big one. smolvm points a networked guest at public resolvers (8.8.8.8, 1.1.1.1) unless told
otherwise, so every hostname was an internet round trip from inside the VM — and nothing cached
it. The same name, resolved a second later, cost the same again:

```
before   example.com 415ms · registry.npmjs.org 411ms · example.com again 394ms
after    example.com   4ms · registry.npmjs.org   1ms · example.com again   0ms
```

Docker was getting 1–7ms because it uses the host's resolver, which is already caching for
everything else on the machine. boxer now does the same, via smolvm's `--dns`. A dev server
resolves several names while starting, so this alone was seconds.

Two safeguards, because pointing a guest at an unreachable resolver turns slow DNS into *no* DNS:
a loopback address is never passed through (a host running dnsmasq advertises `127.0.0.1`, which
inside the guest means the guest), and `network.dns = "off"` gives the decision back to smolvm.

### 2. The default images were the largest ones published

A microVM starts by attaching the image's filesystem, so image size *is* start-up time. boxer's
lockfile detection was choosing `node:24-bookworm` — the full Debian image, not even the slim one:

| image | `boxer up` | pack build |
| --- | ---: | ---: |
| `alpine:3.21` | 0.6s | 8.6s |
| `node:24-alpine` | 1.7s | 10.9s |
| `node:24-bookworm-slim` | 2.5s | 13.4s |
| `node:24-bookworm` ← was the default | 9.5s | 26.2s |

**Six seconds on every start, for compilers and manpages almost nothing uses.** Every detected
image is now the slim variant. Alpine is faster still and is deliberately not the default: it
links musl, so a dependency shipping only a glibc binary breaks in a way that is hard to read.
`image = "node:24-alpine"` is one line for anyone who wants it.

The cost of slim is no build toolchain, so a dependency that compiles from source needs a line of
`image_setup` — which lands in the pack and is paid once, not per start.

### 3. Provisioning several sandboxes at once destroyed the cache they shared

Only visible with more than one sandbox, which is why two benchmarks missed it. smolvm keeps its
machine records in SQLite, so concurrent `machine create` calls lose a race and one is told
`database is locked`. boxer read that as a corrupt environment pack, **deleted the pack** every
other sandbox was about to boot from, and pulled the image from the registry instead.

```
4 parallel `boxer up`   before: 27.6s 27.3s 26.9s 26.8s
                         after:  2.7s  2.8s  2.8s  2.6s
```

A 10x improvement on the case boxer exists for. `vm.Create` waits the lock out — safe, because a
create that lost the race did not happen — and a lock error never condemns a pack again.

### 4. Every smolvm call paid for a bash wrapper

The `smolvm` on PATH is a script that sets a library path and execs the real binary beside it:
22.6ms through it, 10.1ms direct. Two calls per warm command made it ~25ms of a 57ms command.
boxer now goes straight to the binary when it can see that exact layout, and otherwise runs
whatever is on PATH.

### 5. Finding the repository cost more than sandboxing the command

Every boxer command began by spawning `git rev-parse --show-toplevel --git-dir --git-common-dir`
to work out which worktree it was in. Spawning git at all costs about 6ms on a small repository
and 9-18ms on a large one — against about 15ms for the guest command it was about to run. boxer
was paying more to find the repository than to sandbox the work.

It now reads the two ordinary layouts straight off the filesystem — a `.git` directory, and a
`.git` file pointing at a linked worktree — in about **45 microseconds**, a 200x improvement, and
hands back to git for anything else: any `GIT_*` override, a bare repository, a symlinked or
unrecognisable `.git`. The rule is that a wrong answer is far worse than a slow one, because the
toplevel is hashed into the sandbox name and disagreeing with git by one symlink would strand
every existing sandbox under a new name. A differential test asserts the fast path either agrees
with git exactly or declines to answer.

**It also found a bug that had nothing to do with speed.** `git rev-parse --git-common-dir`
answers relative to the *current directory*; boxer resolved it against the toplevel. Below the
repository root the two disagreed, and disagreement is precisely how boxer decides it is in a
linked worktree — so from `src/` in a main checkout, `require_worktree = "require"` **passed**
where it should have refused. Same root cause: `worktree.manage = "detect"` did not collapse to
the repository sandbox, `isolation = "repo"` hashed a different name per directory depth, and
repository-level configuration was looked for in the wrong place.

### 6. Work done after the command had already finished

Two things ran *after* a command returned, delaying the result for caches nothing waits on: a
`git rev-parse` plus `git status --porcelain` (17–26ms), and a second `smolvm machine status` call
to read one label (19ms, because each smolvm invocation costs ~19ms of start-up before it does
anything). Both now run concurrently or not at all. A one-off `boxer run -- true` went from 93ms
to 57ms at the time, and to 31ms once the repository lookup above was fixed too.

## What is *not* slow, contrary to the obvious guesses

Worth recording, because these are where three earlier rounds of this investigation went wrong.

- **Not the worktree mount.** Running the dev server from the mount and from the guest's own disk
  measured 7.9s and 7.9s. The mount does cost about 24% on a dependency install, but it is not
  what made bring-up slow.
- **Not CPU.** An 8-worker parallel load ran in 83ms in boxer's guest against Docker's 102ms.
  boxer's guest is *faster*, and its small-file writes are too — 8,000 files in 266ms against
  Docker's 594ms.
- **Not boxer's own code.** A full trace of `boxer up` to a ready server is three smolvm calls
  and six short guest commands, about 2 seconds all told; the rest is the dev server compiling.
  boxer is a thin layer and measures like one.

## One-off commands

boxer is not primarily for one-off commands, but that path has to be fast too, and it is measured
separately by `bench/bench.sh` ([RESULTS.md](RESULTS.md)). A warm `boxer run -- true` costs
**31 ms**, against 17 ms for driving smolvm directly and 32 ms for `docker exec` into an
already-running container — and 253 ms for `docker run`, a fresh container per command, which is
what a per-tool-call container sandbox actually costs.

So on dispatch-bound work boxer is now **level with a warm container** (0.96x to 0.99x across
`true`, `ls` and a shell loop) while running in a different kernel, and 8x cheaper than the
container-per-command shape it replaces. On I/O it is still behind: 2.3x on a recursive grep,
2.6x on a 200-file write. That is the worktree mount, and it is the honest cost of the boundary.

boxer's own share above raw smolvm is +13 to +22 ms on every workload — a per-invocation toll, not
anything proportional to the work. Spawning `smolvm` costs ~10 ms before it does anything, and a
warm run makes two (one to check the sandbox exists, one to run the command). Boxer's own code is
under 1 ms of the total: an empty Go binary starts in 2.1 ms here and `boxer version` in 2.8 ms.
**The smolvm call count is the number to hold new code against** — each one on the hot path is
~10 ms of agent latency.

Cold-starting a sandbox for a one-off command is 652 ms with the image pack already built.

## Can a pack carry installed dependencies, the way a Docker layer can?

Yes, and it is a trade-off rather than a win. Worth writing down because the technique is not
obvious and the reason it loses is structural.

The trick: install into `/node_modules` from `image_setup`, so the dependencies live in the
*image* rather than the worktree. Node resolves modules by walking up from the importing file, so
`/workspace/app/x.js` finds `/node_modules` without any configuration. The pack carries it, a
second worktree needs no install at all, and — the nice part — the host worktree stays clean, so
nothing appears on disk for an editor to index.

```toml
image_setup = [
  "mkdir -p /nmbuild && cp /workspace/app/package*.json /nmbuild/ && cd /nmbuild && npm ci && mv node_modules /node_modules",
]
```

It measurably works: `require.resolve("next")` answers `/node_modules/next/...`, and no
`node_modules` ever appears on the host. And then:

| | base pack | pack with dependencies |
| --- | ---: | ---: |
| pack size | 68 MB | **228 MB** |
| `machine create` | 452 ms | 1,160 ms |
| `machine start` | 1,150 ms | 4,226 ms |
| **boot cost** | **1.6 s** | **5.4 s** |

**This is why Docker's layer caching does not transfer to a microVM.** Docker mounts layers
through overlayfs, so a bigger image costs nothing at container start. A microVM boots by
attaching the image's filesystem, so image size *is* boot time and it is paid on **every** start.
Installing once saves ~9s per worktree and adds ~3.8s to every boot of every sandbox from then on.

Break-even is two or three boots per worktree. Worth it for many short-lived worktrees; wrong for
a worktree you will restart all day. boxer does not do this for you, and should not by default.

The neighbouring idea — installing on the host and letting the guest use it — is not an
optimisation but a correctness trap: a macOS `node_modules` contains `darwin-arm64` binaries that
cannot run in a Linux guest. Fine for a pure-JavaScript tree, silently broken the moment anything
compiles.

## Several sandboxes at once

This is boxer's primary use case, and the worst bug in this whole effort only appeared with more
than one: several `boxer up` calls arriving together lost a race on smolvm's SQLite store, boxer
read `database is locked` as a corrupt environment pack, **deleted the pack** every other sandbox
was about to boot from, and pulled the image from the registry instead.

```
4 parallel `boxer up`   before: 27.6s 27.3s 26.9s 26.8s
                         after:  2.7s  2.8s  2.8s  2.6s
```

A 10x improvement on the case boxer exists for, and the pack now survives. `bench/parallel.sh`
covers it, and [README.md](README.md#several-sandboxes-at-once) has the memory accounting — the
short version being that at Vercel Sandbox's default 2 vCPU / 4 GB, each sandbox on its own
linked worktree and every cell measured from a wiped host:

| sandboxes | boxer, ready | boxer, memory | OrbStack, ready | OrbStack, memory |
| ---: | ---: | ---: | ---: | ---: |
| 1 | **7.6s** | 1,024 MB | 10.2s | **748 MB** |
| 3 | 22.3s | **1,571 MB** | **18.8s** | 2,014 MB |
| 5 | **26.3s** | 3,539 MB | 27.1s | **2,227 MB** |

boxer is a third faster to a first response at one sandbox and level by five. **The memory
columns should not be quoted**: repeated, the whole-host delta they come from varies by up to
2.1x within one configuration, so no ratio drawn from it means anything. What does replicate is
measured inside the guest — a sandbox serving this dev server touches about 1.2 GB, over half of
it node's own heap, and does not grow to fill whatever allocation it was given.

Two earlier claims here were wrong and are withdrawn: "~243 MB per sandbox, crossover at six"
(measured without a real workload) and "four times OrbStack's memory at five" (an artifact of
measuring every cell in one process, which let the runtime's already-grown VM hide its own cost).
[README.md](README.md#at-vercel-sandboxs-specification) has the corrected numbers and the method.

## The last 13ms, and why it is still there

boxer's one-off cost now accounts for itself exactly: ~3.7ms of boxer process, ~10ms for
`machine status` to check the sandbox exists, ~15ms for the command. Removing the existence check
would take `boxer run -- true` from 31ms to ~21ms, clearly ahead of a container exec.

It is not removed, because the way to remove it is to run the command and treat "no such sandbox"
as the cue to provision — and smolvm reports a missing sandbox as exit 1 with the reason on
stderr, which is byte-for-byte how it reports a guest command that exited 1:

```
missing sandbox:  code=1  err=<nil>  stderr="Error: vm not found: sb-..."
stopped sandbox:  code=1  err=<nil>  stderr="Error: agent operation failed: connect: ..."
guest exited 1:   code=1  err=<nil>  stderr=<whatever the command wrote>
```

The only discriminator is text on a stream that belongs to the user's command. Guess wrong and
boxer provisions and runs the command a **second** time; for a publish or a migration that is far
worse than 13ms is good. `internal/vm/probe_exec_test.go` asserts the ambiguity and fails if a
future smolvm removes it, at which point the optimisation is available.

## Still open

- **Restarting does not benefit from a warm build cache.** the container runtime halves (10.8s → 5.7s) when
  `.next` is already built; boxer barely moves (7.6s → 7.8s, i.e. not at all outside the noise). Inside the guest, the dev server
  takes the same time whether or not the cache is there, and it is the same on the mount and on
  guest-local disk — so this is not mount latency. The likely cause is file timestamps the guest
  reports differently, making the framework rebuild what it could have reused. Unproven.
- **Dependency installs are about twice Docker** and the cause is not yet attributed. The
  measured parts — network throughput about 1.3x slower, the egress allowlist about 10%, the
  mount about 24% — compound but do not obviously add up to 2x.
- **The 4 vCPU / 4 GB default** costs about 11% on an install against 10/8. It is deliberately
  conservative: boxer runs one VM per worktree, so ten worktrees at 10 vCPU would thrash the
  host. Worth deciding on purpose rather than by default.
