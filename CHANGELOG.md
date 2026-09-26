# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **The first `boxer shell` on a host killed the dev server.** Caching a harness install as a pack
  stops the machine, which empties the guest's `/tmp` and ends every `start` service; boxer
  restarted the machine and not the services. `boxer pack save` did the same. Services now come
  back after any snapshot. This was the "inside cells are flaky with URLs on" finding: it only ever
  happened on a host's first round, and the URLs runs went first. Found by an autopsy that saw the
  sandbox running with its `/tmp` empty; verified by running the inside cells with a cold cache in
  both modes, 10 of 10 each.
- **boxer did not work on Linux with SELinux.** The first run of the smoke suite on a Linux host —
  Fedora CoreOS, rootless podman — failed nearly every cell: SELinux denies a container an
  unlabelled bind mount, so the guest saw `Permission denied` on `/workspace`. On a Linux host with
  SELinux enforcing, boxer now adds the shared relabel (`:z`) to every docker and podman mount. It
  never does on macOS, where the mount crosses into the runtime's VM over virtiofs.
- **A non-root `user` could not write the worktree on rootless podman.** Rootless podman maps you to
  the container's root, so `remoteUser: "node"` saw a root-owned worktree. boxer now checks, as the
  user, whether it can write — rather than trusting a per-backend flag — and fixes what it finds:
  on smolvm it gives the user the mount owner's uid, on rootless podman it recreates the sandbox
  once with `--userns=keep-id` so the user is you. Files it writes are owned by you on the host.
- **The smoke suite timed a warm run with `/usr/bin/time`,** which minimal Linux hosts do not ship,
  so the cell failed for a missing timer. It uses python now.
- **`setup`, `start` and `ready` lost the image's `PATH`.** They ran through a login shell, and
  Alpine's `/etc/profile` assigns `PATH` outright, so every directory an image added — golang's
  `/usr/local/go/bin`, a Rust toolchain, a venv — vanished: `go run .` in `start` failed with
  "go: not found" while `boxer run -- go version` in the same sandbox worked. Node images keep
  node in `/usr/local/bin`, so nothing built on them noticed. Found by running the Go example.
- **A setup step that failed under the allowlist sent people to the wrong fix.** npm reports
  `ENOTFOUND` for a host it was never allowed to resolve, and boxer said "fix the `setup` list".
  It now names the refused host and the `allow_hosts` line to add, as `boxer run` already did.
- **The `[prep]` tripwire flagged sharp,** which since 0.33 ships per-platform prebuilt binaries
  that the platform flags select correctly — proven by rendering an image in an Alpine guest from
  a sharp installed on a Mac. A package whose lock entry carries platform optional dependencies is
  no longer flagged; one that really compiles (better-sqlite3, node-pty) still is.
- **`boxer rm --all` reached every backend.** A unit test's `rm --all` deleted another test's
  container on the real docker daemon. `--all` is the configured backend; `-A` is every backend.
- **`boxer backends --probe` threw away the reason a probe failed**, and tested a network mode
  nobody runs. It now quotes what the sandbox printed and uses boxer's default mode where the
  backend supports it.
- **A freshly started docker sandbox could see a stale worktree mount.** On OrbStack, a container
  created moments after the same path was removed and re-added — an orchestrator reusing a task
  name — failed its first command with `chdir to cwd ("/workspace") … no such file or directory`
  and `setup` failed for no reason of its own (3 in 5 in a tight loop). The staleness can last
  the container's whole life and can flap — readable on one exec, gone on the next — so waiting was
  not enough. boxer now proves the mount by writing through it, twice running, before anything
  else; and when a sandbox it has just created never passes, it recreates it once, because a new
  container gets a new mount. 60 of 60 afterwards, 4 of them saved by the recreate. Not
  reproducible without the tight timing, which is why it hid.
- **docker's "OCI runtime exec failed" was reported as exit 127.** The runtime could not start the
  process, and boxer passed its status through as the command's — indistinguishable from "command
  not found". It is recognised by its message now, like the daemon's own errors.
- **A non-root `user` could not write the worktree on smolvm.** smolvm mounts it owned by the host
  uid with no mapping (501:755 on a Mac), so `user = "node"` — or a devcontainer's `remoteUser` —
  read the tree and wrote nothing: `npm install` in `setup` failed on its first file. boxer now
  gives that guest user the host's uid, once per VM, which is devcontainer's
  `updateRemoteUserUID`. docker, podman and Apple's `container` map ownership and needed nothing.
- **A worktree recreated at the same path skipped `setup`.** The setup and prep markers lived in
  boxer's state keyed by path, so an orchestrator that removed `feature-a` and later added a new
  `feature-a` got a sandbox that believed it was set up and had no dependencies. The markers now
  live in the worktree's own git directory and go with it.
- **One retired hostname stopped every inside sandbox from starting.** smolvm resolves each
  allowlisted host at create and refuses the whole machine if one fails; `statsig.anthropic.com`
  stopped resolving, it was on the inside placement's list, and every inside sandbox failed with
  `CREATE_FAILED`. The host is gone from the list, and an allowlisted host that does not resolve
  is now left out with a warning naming it instead of failing the create.
- **Inside placement was told to run a `boxer` it does not have.** The brief, skill and
  `AGENTS.md` sent an in-guest agent to `boxer status --json`, which does not exist in the guest,
  for URLs the guest cannot reach — the portless proxy is on the host's loopback. Inside, they now
  say the server is at `127.0.0.1:<port>`, and the harness is launched with `BOXER_PORTS` and
  `BOXER_URLS` so it can tell a person the host-side address.
- **`boxer-eval --tier matrix --list` ran the matrix.** It scaffolded a base repository and started
  live sandboxes; stopping it left them running. `--list` lists, for matrix and sdlc, and an
  interrupted matrix or sdlc run now brings down its own sandboxes and removes its scratch.
- **A discarded output made boxer report the wrong exit code.** `isTerminal` tested
  `os.ModeCharDevice`, which is true of `/dev/null`, so `boxer run … </dev/null >/dev/null` asked the
  runtime for a TTY with no terminal behind it. docker refuses that before the command runs and
  exits 1, and boxer reported the 1 as the command's status: `boxer run -c "exit 4"` returned 1.
  This was the `capsule replay` failure that every attempt to observe had closed — capturing the
  output to look at it made stdout a file, and the TTY request went away. Found by making the cell
  print its evidence, then proven against a build with the old check (exit 1) and the fix (exit 4).
- **A container runtime's own failure is no longer a guest exit status.** docker and Apple's
  `container` exit 1 for "no such container" and "is not running", podman 125 — statuses a command
  could also have. boxer now recognises the runtime's message at the head of stderr and reports an
  error, so no exit code is recorded for a command that never started (contract rule 2).
- **devcontainer `${...}` variables were passed through literally.** A bind mount of
  `${localWorkspaceFolder}/data` mounted an empty directory named after the variable, and
  `${localEnv:TOKEN}` handed the guest the literal text. Both looked configured. An unresolvable
  variable is now named, and an env value or mount that depends on one is dropped.
- **`forwardPorts` became a fixed host port**, so the second worktree of any repository with a
  devcontainer failed to start on a busy address. It is now `auto:`, as the specification's
  `requireLocalPort: false` default means.
- **boxer's host state was never removed.** A few weeks of use left 1,078 last-used stamps, 2,016
  locks, 528 run records and 222 setup markers for sandboxes that no longer existed. Deleting a
  sandbox now removes its machine state at once; `gc` sweeps what belongs to no sandbox after a
  week, and removes a lock only when nobody holds it. The setup marker survives `down`, because it
  describes the worktree — `down` then `up` must not re-run `npm ci` into an installed tree.
- **The brief called every sandbox a microVM**, including a docker container, which claims a
  stronger boundary than the agent has.

### Added
- **`examples/`**: Next.js (from `boxer.toml` and from a devcontainer), Vite, FastAPI with uv, Go,
  a two-service monorepo, host-side `[prep]`, and the docker backend. The test suite parses every
  one; every one but the monorepo pattern was brought up on a real sandbox and served through its
  URL.
- **A command line for seeing and managing what boxer has on a machine.**
  - `boxer ls` now says what a person needs in order to act: the backend, the branch and its
    distance from upstream, whether the worktree is clean, dirty or gone, and where it serves.
    `-A` lists every installed backend at once; `--pr` adds each worktree's pull request.
  - `boxer stop` and `boxer rm` act by name, by filter (`--gone`, `--stopped`, `--all`) or by
    choosing from a list (`-i`), and `rm` takes boxer's host state with the sandbox.
  - `boxer backends` reports every backend — installed, answering, how many sandboxes, what it
    can enforce — and the misconfigurations that cost real time here: a podman machine on the
    `libkrun` provider, which cannot bind-mount; Apple's `container` service not running.
    `--probe` creates, uses and deletes a real sandbox on each, the only check that proves one works.
  - `boxer integrations` reports, per harness, orchestrator and tool, whether it is on PATH and
    whether boxer is wired into this repository and this user, and whether the installed files are
    out of date. It asks boxer's own installer which files it writes, so the two cannot disagree.
    `boxer integrations add skill|plugin` installs through `npx skills` and `npx plugins`, which
    already find boxer's skill and plugin in the repository.
  - `boxer url` / `boxer open` print or open this worktree's server; `boxer completion` for bash,
    zsh and fish.
  - Output adapts to who is reading: colour, tables, hints and confirmation prompts for a person
    at a terminal; plain text, `next:` lines and no prompt, ever, for an agent or a pipe — a
    command that would ask a person refuses an agent and says what to pass. `BOXER_OUTPUT=json`
    gives every command with a JSON form that form without a flag.
  - The presentation lives in `internal/cli`, and a test fails if anything in the core imports it.
- **Stable URLs for dev servers, through portless: `[urls] enabled = true`.** Each forwarded port
  gets a name that is the same for the life of the sandbox — `https://fix-ui.myapp.localhost:1355`
  for the worktree on `fix-ui` — instead of an `auto:` host port that differs in every worktree.
  boxer points [portless](https://github.com/vercel-labs/portless) at the port the sandbox already
  publishes (`portless alias`), registers the name when the sandbox starts, and removes it when the
  sandbox is deleted by `down`, `gc` or `down --all`; `gc` also removes a route whose sandbox
  disappeared without boxer. Two things portless does not do, boxer does, both found by running it:
  `portless get` applies the worktree prefix and `portless alias` does not, so registering the name
  `get` returned gave a 404; and a detached worktree gets no prefix at all, so it collided with the
  main checkout. boxer registers the full prefixed name, and uses the sandbox's two-word name for a
  detached worktree or a name another sandbox — or a person — already holds. Verified on all four
  backends: three worktrees of one repository started together, each serving guest port 3000,
  each reachable at its own URL, and `down` removing exactly one route. Off by default.
- **Agents are told where a dev server is.** The brief, the skill, `AGENTS.md` and the
  `boxer_status` tool now say to read `urls` (or `ports`) from `boxer status --json` rather than
  assume `localhost:<port>`, which is wrong in every worktree once `auto:` is in use. `status`,
  `ls --json` and `brief --json` carry the URLs and the configured forwards. A live `server`
  scenario in the adherence tier checks that an agent told nothing else finds its own worktree's
  server, by name when names exist. Its first run found a real defect: codex called `boxer_status`
  exactly as told, and was given the port, because the harness launched boxer's MCP server with a
  different `XDG_STATE_HOME` from the one that registered the name. A port boxer's registry does
  not name is now looked up in portless's own route table, under `$HOME`, by host port; `down`
  removes routes to the sandbox's ports the same way, and `down --all` prunes dead routes.
- **`user`**: the guest user `setup`, `start` and every command run as. `image_setup` stays root.
- **More of devcontainer.json.** `${localWorkspaceFolder}`, `${containerWorkspaceFolder}`,
  `${localEnv:NAME[:default]}` and `${containerEnv:NAME}` (in `remoteEnv`) are resolved;
  `remoteUser`/`containerUser` set `user`; `hostRequirements.cpus`/`memory` raise the defaults;
  `portsAttributes.label` names the URL and `requireLocalPort` keeps a port fixed; `appPort` is read.
  `runArgs`, `privileged`, `capAdd`, `securityOpt`, `workspaceMount`, `overrideCommand: false`,
  `postAttachCommand`, `init` and a required GPU are refused by name instead of silently ignored.
- **Four backends: `backend = "smolvm" | "docker" | "podman" | "container"`.** The sandbox host is
  now an interface with four implementations rather than a struct with one, and they do not fall
  into two tidy buckets — which is what makes the capability model worth having:

  | | boundary | egress allowlist | environment cache |
  | --- | --- | --- | --- |
  | `smolvm` (default) | a kernel per sandbox | yes | packs |
  | `container` (Apple) | **a kernel per sandbox** | no | no |
  | `docker` / `podman` | one shared kernel | no | no |

  Apple's `container` runs one lightweight VM per container on Virtualization.framework, so it has
  smolvm's boundary with docker's inputs and refusals — it belongs to neither group, and needed no
  new concepts to support, which is the best evidence the interface is the right shape. boxer's sandbox
  host is now an interface with two implementations rather than a struct with one. smolvm stays
  the default and the only one that gives a kernel per sandbox; a container starts faster, uses
  ordinary OCI images, and runs where there is no hypervisor — including Linux CI.

  **They are not interchangeable for containment, and boxer says so rather than implying
  otherwise.** `network.mode = "allowlist"` is boxer's default and a large part of what it
  promises; an OCI daemon has "no network" and "the whole internet" and nothing between. So the
  container backends **refuse** that setting by name instead of starting a sandbox that reads as
  enforced and is not. `boxer doctor` prints the capability table, boundary first:

  ```
  backend:   docker (29.4.0) — one shared kernel
    worktree mount     yes
    egress allowlist   no    — network.mode = "allowlist" is refused; use "off" or "on"
    environment cache  no    — image_setup runs again for every worktree
    fork               no    — needs a backend that can branch a running machine
  ```

  What a backend cannot do is refused by name, never silently degraded: packs, forks, named packs,
  egress reporting and disk accounting each say which backend to use instead. The same 65-cell
  smoke suite runs on every one of them (`BOXER_BACKEND=podman make smoke`), with capability-gated
  cells skipped *by name* so the output doubles as a test of that table. Measured: smolvm 65/0,
  podman 56/0/9, docker and Apple `container` 56/0/9 individually.

  Benchmarked across all four, one backend at a time on a wiped host, three samples each: a single
  Next.js worktree reaches HTTP 200 in 3.6 s on podman, 4.5 s on docker, 8.2 s on Apple
  `container` and 8.5 s on smolvm. The two kernel-per-sandbox backends agreeing within 0.3 s is
  the point — the ~4.6 s is the cost of the boundary, not of smolvm. At three worktrees the spread
  falls to 27% as the compiles, not the sandboxes, become the bottleneck.
- **`[prep]` — commands that run on the host, in the worktree, before the sandbox exists.** This
  is devcontainer's `initializeCommand`, which boxer previously ignored. A dependency install on
  the host is roughly twice as fast as the same install in a guest, and boxer derives the guest's
  platform triple from the image and offers it as `$BOXER_TARGET_FLAGS`, so
  `npm ci $BOXER_TARGET_FLAGS` fetches Linux binaries on a Mac. Verified end to end: an Alpine
  guest resolves `@next/swc-linux-arm64-musl` from an install run on macOS.

  Opt-in, because it is only safe for packages that ship **prebuilt** platform binaries. Anything
  that compiles at install time builds for the host whatever the flags say, and fails much later
  inside the guest as `invalid ELF header`. `boxer doctor` warns by name when it finds one, and
  `setup` — which runs in the guest and is always right — stays the default answer.
- **Host package caches are mounted read-only into the guest**, detected from the same lockfiles
  that pick the image. The install still happens in the guest, so every binary is still chosen for
  the guest's platform; only the download is saved. On by default because it can change a duration
  and not a result. `[cache] enabled = false` turns it off.
- **`updateContentCommand`** from a devcontainer is now read, appended to `setup` ahead of
  `postCreateCommand`, in specification order.

### Fixed
- **boxer recorded an exit code for commands that never ran.** When the backend itself failed —
  the container not yet running, the daemon unreachable — `Exec` returned an error *and* a code of
  1, and the run record stored that 1 as though the command had exited with it. Nothing exited
  with anything; the command had not run. A stale record is recoverable, a fabricated one is not,
  and `capsule replay` was the only thing that noticed, because it is the only feature that
  compares a recorded exit against a fresh one. Runs that error are no longer recorded.
- **`Start` waits for a container to be running before boxer's first `exec`.** `docker start`
  returns when the daemon has accepted the start, which is not a promise that the process is up.

  This entry used to say that race was why `capsule replay` failed when unobserved — 5 of 5 runs
  failed unobserved, 3 of 3 passed observed — and that waiting fixed it. That diagnosis was wrong.
  The failure came back, and the real cause was the `/dev/null` TTY bug under *Fixed* above: "every
  attempt to observe it passed" because observing meant capturing stdout, which stopped boxer
  asking for a TTY. The wait is kept because it is correct, not because it fixed anything measured.
- **`boxer ls`, `gc`, `down --all` and `watch` always talked to smolvm**, whatever `backend` said.
  With a container backend they asked smolvm for its machines, were told there were none, and
  reported nothing to reclaim while containers accumulated — `gc` cannot clean up what it does not
  look at. Found by running the smoke suite against a second backend, which is the whole reason to
  have one.
- **`require_worktree = "require"` did not hold below the repository root.** `git rev-parse
  --git-common-dir` answers relative to the *current directory*, and boxer resolved it against the
  toplevel instead — so from `src/` in a main checkout the common directory came back as a path
  two levels above the repository, which does not match the git directory, which is how boxer
  decides it is in a *linked* worktree. Every worktree-linked decision inverted below the root:
  `require` passed where it should have refused, `worktree.manage = "detect"` did not collapse to
  the repository sandbox, `isolation = "repo"` hashed a different sandbox name per directory
  depth, and repository-level configuration was looked for in the wrong directory. Resolved
  against the current directory now, as git intends, with a regression test that runs `Resolve`
  from a subdirectory.
- **Discovering the repository cost more than the command being sandboxed.** Every boxer command
  began by spawning `git rev-parse`, which is ~6ms on a small repository and 9-18ms on a large
  one, against ~15ms for the guest command itself. boxer now reads the two ordinary layouts — a
  `.git` directory and a `.git` file pointing at a linked worktree — from the filesystem directly,
  in about 45 microseconds, and hands back to git for anything else: any `GIT_*` override, a bare
  repository, a symlinked or unrecognisable `.git`. A warm `boxer run -- true` went from 39ms to
  **29ms**, of which 25ms is now smolvm's two process launches and under 1ms is boxer's own code.
  A differential test asserts the fast path either agrees with git exactly or declines to answer,
  because its result is hashed into the sandbox name.
- **The readiness probe waited a fixed 250ms between attempts**, which on average threw away half
  of that after the service was already answering. It now starts at 25ms and backs off to the same
  250ms ceiling, so a service that comes up quickly is noticed quickly and one that takes a minute
  is polled no harder than before.
- **Provisioning several sandboxes at once destroyed the cache they shared.** smolvm keeps its
  machine records in SQLite, so concurrent `machine create` calls lose a race and one of them is
  told `database is locked`. boxer read that as a corrupt environment pack: it deleted the pack
  every other sandbox was about to boot from and pulled the image from the registry instead. Since
  boxer's whole shape is one sandbox per worktree, several provisions arriving together is the
  ordinary case rather than a corner — four parallel `boxer up` calls took **27 seconds each**
  instead of 2.7, and left no pack behind. `vm.Create` now waits out the lock, which is safe
  because a create that lost the race did not happen, and a lock error never condemns a pack. A
  10x improvement on the case boxer exists for.
- **Every smolvm call paid for a bash wrapper.** The `smolvm` on PATH is a script whose whole job
  is to set a library path and exec the real binary beside it, and that costs a shell process per
  call: 22.6 ms through the wrapper against 10.1 ms direct. A warm `boxer run` makes two calls, so
  this was ~25 ms of a 57 ms command — more than everything else boxer does. boxer now goes
  straight to the binary when it can see the exact layout the wrapper has (a `smolvm-bin`
  executable and a `lib` directory beside the resolved script) and otherwise runs whatever is on
  PATH, which is always correct if slower. `boxer run -- true` went from 57 ms to 38 ms.
- **Every DNS lookup in a sandbox took 415 milliseconds.** smolvm points a networked guest at
  public resolvers unless told otherwise, so every hostname was an internet round trip from inside
  the VM and nothing cached it — the same name resolved a second later cost the same again. boxer
  now points the guest at the host's own caching resolver, which is already caching for everything
  else on the machine: lookups went to 1-4ms, and 0ms once warm. A dev server resolves several
  names while starting, so this was seconds on every bring-up. `network.dns` overrides it, a
  loopback resolver is never passed through (inside the guest it would name the guest), and
  `network.dns = "off"` restores smolvm's default.
- **The default guest images were the largest ones published.** Lockfile detection chose
  `node:24-bookworm` — the full Debian image, not the slim one — and a microVM starts by attaching
  the image's filesystem, so image size is start-up latency. `boxer up` took 9.5s on that image
  against 2.5s on `node:24-bookworm-slim`: six seconds on every start, for compilers almost
  nothing uses. Every detected image is now its slim variant. Alpine is faster still and
  deliberately not the default, because musl breaks a dependency that ships only a glibc binary in
  a way that is hard to read.
- **Every warm run made a third smolvm call it did not need.** `boxer run` shelled out to
  `smolvm machine status` twice: once in `Ensure` to check the sandbox is there, and once *after
  the command had returned*, to read a single label for the run record — state the first call had
  already fetched in the same process. Each smolvm invocation costs about 19 ms of CLI start-up
  before it does anything, so this was 19 ms of agent latency for a cache nothing blocks on. The
  machine is now memoised for the life of the command and the memo is cleared wherever boxer
  changes the machine.
- **Every run waited for `git status` after its command had already finished.** The run record
  collected the worktree's git state once the command returned, which put `git rev-parse` and
  `git status --porcelain` — 17 ms on an empty repository, 26 ms on this one — between the agent
  and its result, for a cache nothing blocks on. It now runs alongside the command, which is also
  the more correct answer: a capsule replays the tree as it was when the command ran.

  Together the two fixes took a warm no-op from ~93 ms to ~57 ms, a 39% cut, with no change to
  what boxer does. [bench/](bench/) has the measurement and the remaining breakdown.
- **The smoke suite compared scope slugs instead of scope keys.** `boxer doctor` prints
  `swift-crab (sb-7e1852e4a3c3)`, and five cells took field 2, which became the slug when slugs
  landed. Equality checks still passed for the right reason, but the two *inequality* checks —
  that separate worktrees get separate sandboxes — were comparing display strings drawn from a
  pool of 4096, so they could have passed by luck. All five now go through one `scopekey` helper.
  The same mistake in the new benchmark made every raw-smolvm sample fail in a flat 20 ms, which
  would have published "raw smolvm is 4.6x faster than boxer" had the harness not counted
  failures.
- **A shim on `bash` sandboxed the host's own tooling and hung.** `intercept` is a list of program
  names, and nothing stopped it naming a shell. Every `#!/usr/bin/env bash` script on the host then
  resolved through the shim — smolvm's own launcher among them — and the sandboxed copy waited
  forever for a sandbox it had no business being in. A stranded `boxer run -- bash … smolvm machine
  status` was found still running two days after the eval that started it. `sh` was already
  excluded; `bash` is now too, and a test holds both. An interactive guest shell has always been
  `boxer-bash`, which is what to install instead.
- **The egress-denial reporting was written against a guessed schema and reported nothing.**
  smolvm returns `{"timestamp","operation","dest"}`, and every event it returns is already a
  denial; boxer was decoding `{host,port,allowed,at}`, which left every host blank. Now correct,
  proved against a real guest by two smoke cells, and scoped to the failing command's own window
  rather than everything the machine remembers.
- **Forwarded secrets were visible to `ps`.** `env_passthrough` was passed as `-e KEY=value`, which
  put every forwarded value into smolvm's argument vector, where any other process on the host
  could read it. Both lists now go through smolvm's `--secret-env`, which passes the variable's
  name and lets smolvm read the value itself. `secrets` was declared in the configuration and
  never read at all; it works now, and reaches `setup` and `image_setup` as well as ordinary
  commands.
- **Codex was not sandboxed in a linked worktree.** Codex resolves project configuration to the
  main repository, so the hooks `boxer install codex` wrote into a worktree were never read and
  every command ran on the host. Orchestrators work almost entirely in linked worktrees, which made
  this the ordinary case rather than a corner. The installer now writes the hooks to the main
  repository as well, and says so.
- **Input written before the sandbox attached was discarded.** A sandbox takes seconds to attach,
  and opening the guest terminal flushed whatever was queued on the caller's. A harness that
  configures its shell immediately after spawning it — OpenHands writes the prompt it parses
  command results out of one second in — lost that configuration every time and then waited for
  output that could never end. `boxer run --tty` now asks for a terminal explicitly and captures
  stdin from the moment it starts.
- **A boxer shim could be resolved by boxer itself.** The smolvm launcher runs `uname`, so a
  `uname` shim on PATH became `boxer run -- uname` inside a boxer already working on that sandbox,
  and the session deadlocked until it was killed. Shim directories are marked, and boxer drops them
  from its own PATH before starting anything.

### Added
- **A bring-up benchmark, because the per-command one answers a different question.**
  `make bench-devserver` scaffolds a pinned Next.js app, serves it and times how long until the
  port returns HTTP 200, across four named scenarios and with every condition pinned: the same
  image and the same 10 vCPU / 8 GB for boxer and Docker alike, a private npm cache per install,
  an identical lockfile, the build cache cleared before every start, and a worktree copy per
  contender. Each of those is pinned because leaving it loose produced a different, plausible,
  wrong answer. This is the harness that found the DNS and image defects below; starting a
  session now costs 7.5 s against Docker's 10.2 s and 7.2 s for no sandbox at all.
- **Performance benchmarks, committed with their harness.** `make bench` measures boxer against
  no sandbox at all, macOS seatbelt (the mechanism behind Codex's and Claude Code's native
  sandboxes), `codex sandbox`, Docker cold and warm, and raw `smolvm machine exec` — the last of
  which isolates boxer's own cost from the platform's. The harness, the method, the raw samples
  and the rendered results are all in [bench/](bench/) so a performance claim can be checked
  rather than believed. Firecracker is documented as not runnable on this project's macOS host
  rather than estimated.

- **Named packs.** `boxer pack save|ls|use|rm <name>` keeps a prepared guest as an artifact
  someone meant to keep — a toolchain assembled by hand, a state worth returning to. `gc` sweeps
  the automatic pack cache by age and count and never touches a named one. A named pack carries
  what is installed, not what is running: smolvm's real `.smolcheckpoint` includes RAM but is
  refused on any machine holding a host mount, and boxer always mounts the worktree.
- **Forking.** `boxer fork` branches the running sandbox into copy-on-write children that start
  from its memory and disks rather than booting — warm workers for work that shards cleanly.
  Children share the parent's worktree, because smolvm refuses to branch a machine whose mount is
  staged, so two children writing one file race as two host processes would; a second git
  worktree is still the way to get an isolated copy. Preparing is explicit because it restarts
  the sandbox. `boxer run --scope <name>` drives a child; `boxer gc` reclaims one whose parent is
  gone.
- **Test results.** `boxer run --junit junit.xml`, a task's own `junit` list, or a `[results]`
  table, and a failed run answers "which tests failed?" instead of leaving a log. Parsing is on
  the host, reading the file the guest just wrote, because the worktree is mounted rather than
  copied. Counts come from the cases and never from the `tests=` attributes, which are the part
  that lies, and a report older than the run is ignored. `--fail-on-test-failures` turns a green
  exit into 1 and never replaces a non-zero one.
- **Capsules.** Every run leaves a record — command, directory, exit code, image, pack, HEAD,
  dirty flag, output tails, test summary — under `$XDG_STATE_HOME/boxer/runs`, reported as
  `last_run` by `boxer status --json`. `boxer capsule new` turns it into a committable
  `capsule.toml` (plus a patch when the tree was dirty), and `boxer capsule replay` runs it again
  and says whether the failure reproduced. Exit 0 means it did: a capsule is a question, not a
  test.
- **Tasks say what they are for.** A `[tasks]` entry can be a table — `cmd`, `description`,
  `junit`, `timeout`, `env` — and the string form keeps working. The description is what
  `boxer tasks` prints and what the brief hands the agent, because choosing the right task is the
  decision an agent actually has to make. `timeout` bounds the command with smolvm's own
  `--timeout`.
- **Sandboxes have names.** `swift-crab` beside `sb-7e1852e4a3c3`, in `ls`, `status`, `doctor`,
  `watch` and every refusal, and accepted anywhere `--scope` is. Derived from the same hash, so
  nothing stores it and every boxer agrees on it.
- **Egress denials are named.** A command that fails under `network.mode = "allowlist"` now says
  which host the allowlist refused, and `boxer doctor` lists them. Inside the guest a blocked host
  is an ordinary DNS or connect error with no policy in it, so this was the one diagnosis the
  sandbox could not give you itself.
- **A published skill and a discovery index.** `skills/boxer/SKILL.md` at the installer path skill
  managers read, and `.well-known/agent-skills/index.json` on the docs site with a SHA-256 digest
  of the exact published bytes. Both are rendered from the same template as the plugin, and CI
  rejects drift between the three.
- **fx**, Vercel Labs' native coding agent, as an inside-mode harness. fx has no hooks, its shell
  cannot be denied headlessly, and it resolves commands past a PATH shim, so inside the guest is
  the level at which boxer genuinely contains it. Its installer is fetched and run under bash — the
  script uses `set -o pipefail`, which the guest's `/bin/sh` rejects — with the Keychain disabled,
  because a VM has none.
- **`boxer doctor` reports enforcement that will not hold.** A login shell rebuilds PATH — on macOS
  `path_helper` puts the system directories first — so PATH shims silently miss for a harness that
  runs commands through `zsh -lc`. boxer cannot out-rank `path_helper`; it can say so, and now
  names the enforcement to use instead.
- **A matrix evaluation tier**: the same development workload at every integration level, across
  nine harnesses and three orchestrators, with each cell scored on thirteen weighted claims rather
  than passed or failed as a whole. Every run is archived with the conditions it was made under.
- **A documentation site** built with Fumadocs, under `site/`. `make docs`.

### Added
- **Environment packs.** `setup` was run once per VM and never cached, so every worktree repeated
  `bun install` from scratch while a pack of the bare image sat beside it. The result of setup is
  now packed and keyed on the image plus the setup commands, so a second worktree of the same
  repository starts with dependencies already installed — measured at 0.7 s against 2.9 s, and the
  saving grows with the size of the install. Changing a setup line changes the key, exactly like a
  Docker layer.
- `boxer up --rebuild` drops the cached environment and runs setup again, and `packs_keep_last`
  (5 by default) bounds the pack cache by count as well as by age — an environment that changes
  often leaves a pack per version, all of them younger than `idle_timeout`.
- **`start` and `ready`.** `setup` installs a service, `start` launches it detached at every VM
  start, and `ready` is polled until it exits zero before boxer reports the sandbox up. A dev
  server or a database now runs in the sandbox and answers the host through a forwarded port,
  verified against a real microVM. No supervision and no dependency graph: a dead process makes the
  next command fail and says where its output is.
- **`env` and `mounts`.** `env` reaches every command including setup, and travels into the
  environment pack; secrets stay in `secrets` and `env_passthrough`, which are read from the host
  at run time and never snapshotted. `mounts` adds host directories beside the worktree, with `~`
  expanded, usually for a shared dependency cache.
- **A flow tier**: one development session end to end — scaffold a pinned Next.js app in the
  sandbox, serve it, reach it from the host, speak MCP to a server running in the guest, restart and
  find the dependencies still there. Passes in 54 s warm, 3 m 34 s cold. `make eval-flow`; it runs
  on demand and before a release, never in the default gate. It proves a capability boxer had and
  had never claimed: an MCP server that must live beside the code runs in the sandbox, addressed by
  an ordinary `.mcp.json` entry through `boxer run`.
- **Devcontainers are read.** A repository with `.devcontainer/devcontainer.json` gets its image,
  lifecycle commands, ports, environment, bind mounts and workspace folder without restating any of
  it, and `doctor` says which file each value came from. What needs an image build (`features`,
  `build`/`dockerFile`) or multi-container orchestration (`dockerComposeFile`) is refused by name
  with what to do instead. The file is parsed as the JSON-with-comments it actually is.
- **A core a user interface can sit on.** A sandbox now records which harness, session and agent it
  belongs to, so a listing names "claude-code/4f2a9c" instead of a hash the identity was hashed
  into. `boxer ls --resources` measures what each one costs: allocation from smolvm, resident
  memory and CPU from its process, and disk from its data directory in allocated blocks, the way
  `du` counts, because the disks are sparse and their length is not their cost. `boxer watch`
  streams state changes as line-delimited JSON, so an interface tails one process instead of
  polling. No interface ships; these shapes are in `docs/api.md` and stable from 1.1.
- `boxer doctor` reports the environment's cache key and whether it is cached yet, so "will this
  worktree have to run setup again" is answerable without watching a VM boot.

## [1.0.0] - 2026-09-18

First public release. What is stable, and what breaking it would cost, is in
[docs/release.md](docs/release.md): the command line and its flags, the `--json` shapes, the exit
codes and error contract, the skill and plugin layout, the MCP tool names, `pkg/boxer`, and the
configuration keys.

Evidence for this release, all run on 2026-09-18 on Apple Silicon with smolvm 1.16.1: unit tests
with the race detector and per-package coverage floors on Linux and macOS; lint and vulnerability
scan clean; smoke 53/53; T1 67 pass, 0 fail, 1 skip, run twice with identical per-cell verdicts;
T2 live 42 pass, 0 fail, 14 named skips for $0.19; adherence 80 of 96 live cells across four
models with one harness verdict, the known Grok one. `v1.0.0-rc.1` proved the publishing path:
signed checksums that verify against the release workflow's identity, and all four install routes
exercised against the published release.

### Added
- Storage reclaims itself. Any command that provisions a sandbox starts a background sweep, at
  most once every `reclaim_every` (6 hours by default), deleting sandboxes whose worktree is gone,
  sandboxes idle past `idle_timeout`, and packs nothing references. `auto_reclaim = false` turns
  it off, and `BOXER_NO_RECLAIM=1` does the same for one run.
- `min_free_gb` (5 GB by default): boxer refuses to write a pack when free space is below it and
  pulls the image instead. A cache is worth less than a working disk.
- `boxer gc --all` reclaims every stopped sandbox and every unreferenced pack whatever their age,
  and `gc` now reports how much it freed.
- `boxer doctor` prints the footprint: sandboxes, packs, bytes cached and bytes free, with a
  warning when free space is under the margin.
- `make clean-evals` reclaims the evaluation suite's scratch repositories and pack cache, which
  live outside boxer's state and which nothing reclaimed before.
- The remote, and with it the first proof of the publishing path: `v1.0.0-rc.1` published four
  platform archives, four SBOMs, the plugin, skill and npm tarballs, and a signed `checksums.txt`
  that verifies against the release workflow's OIDC identity.

### Fixed
- Copilot CLI could not reach any model from inside a sandbox: its model client reads the system
  trust store rather than node's bundled one, and a slim image has no CA certificates. Its guest
  install now adds them, as Codex's already did. Found by running the inside cell that the harness
  table had claimed for a release without ever exercising.
- A release could not have succeeded: the Homebrew cask named a tap token this repository does not
  have, so the first tag would have failed with every artifact already built. The tap push is now
  skipped when the token is absent.
- A candidate tag published as the latest release, which is what `install.sh` and Homebrew hand to
  anyone asking for the current version. Candidates are prereleases.
- CI's linter could never have run: the configuration declares version 2 while the workflow
  installed version 1 through an action that only installs version 1.
- The toolchain carried five known standard-library vulnerabilities, four of them reachable.

### Added
- `[tasks]` in `boxer.toml` with `boxer tasks` and `boxer run --task <name>`: the repository names
  the commands it wants run, so whether work is sandboxed no longer depends on the intercept list
  matching a shell line the agent composed.
- `boxer brief [--json]`: the resolved brief for a checkout, so published content can stay static
  and still tell an agent what this repository expects. Hooks inject the same text, and the MCP
  server serves it as a resource.
- The skill ships its `scripts/` layer, per the Agent Skills specification: `run`, `task`,
  `status` and `brief` call the installed CLI, so every harness has a deterministic command path
  with no per-harness code.
- An opt-in event stream (`[telemetry]`) with file, stderr and OpenTelemetry sinks, `boxer logs`,
  an event tail on `boxer status --json`, and command lines elided unless asked for. Off by
  default: with no table, boxer writes no log and nothing to the network. Schema in
  `docs/events.md`.
- Apache-2.0 licence, contribution guide, security policy and code of conduct.
- `npm/bin/boxer.js`, the launcher `package.json` had always declared but the repository never
  contained, so packed tarballs installed a command that could not run.
- `scripts/install-routes.sh` and a CI job running it on Linux and macOS: `install.sh` and the npm
  package against a release staged on the machine, each of them also with a tampered checksum that
  must abort.
- Release artifacts: darwin/amd64 binaries, reproducible builds (`-trimpath` and a commit-derived
  timestamp), a syft SBOM per archive, keyless cosign signing of `checksums.txt`, the skill as its
  own tarball, and a Homebrew tap.
- `make release-gate`, the mechanical half of the release gate, enforced on the tag before
  anything is published.
- User-facing documentation: `docs/install.md`, `docs/configure.md`, `docs/integrate.md` and
  `docs/troubleshooting.md`, with `docs/architecture.md` and a `docs/README.md` index.

### Changed
- Published content is static. The package and the skill are byte-identical for every user but for
  the release version, `boxer install` copies them verbatim, and `doctor` reports drift instead of
  editing installed files in place. Configuration reaches the agent at runtime through
  `boxer brief`.
- An unknown top-level table in `boxer.toml` is a warning rather than an error, so a repository
  that adopts a newer boxer's table still loads under an older binary. A misspelled scalar key
  stays fatal.
- `docs/release.md` now states what is stable at 1.0 and what is not, sets a deprecation window of
  one minor release, and gives the release gate as a runnable checklist.
- The README opens with a two-minute quickstart and the five integration levels.

### Fixed
- A truncated pack (an interrupted pack write) made every sandbox on the host fail with an I/O
  error that named neither the pack nor a fix. An empty pack is now rewritten, and a broken one is
  deleted at create time and the image pulled instead.
- `boxer install` reported success having written nothing when a copy or an append failed.

## [0.2.0] - 2026-09-18

### Added
- Worktree timing: `warm_on_session_start`, `worktree.manage`, `boxer install git` (a post-checkout
  hook that warms the sandbox when a worktree is created), and a `doctor` signal report.
- One Agent Plugins 1.0.0 package with per-client views, validated against the vendored schemas.
- MCP lifecycle signals: `initialize` warms the scope, EOF records last use, and a guest path in
  `boxer_run`'s `cwd` maps back to the host worktree.
- An adherence evaluation tier measuring whether a live model follows the injected brief, with a
  two-model rule that separates a harness fault from a model's behaviour.
- Gateway spend tracking and a per-run budget for live evaluations.
- `boxer shim install --shell` writes `boxer-bash`, so any harness with a configurable shell binary
  is sandboxed with no boxer code (verified with OpenHands).
- `boxer down --scope NAME` takes a sandbox down by name from anywhere.
- Per-host harness packs: the first `boxer shell <harness>` pays the install, later worktrees start
  in seconds.

### Changed
- DeepSeek Harness support was rebuilt from the installed product: it has no `.dsh/hooks.json`, and
  integrates through a profile patch, an MCP client entry and the Claude Code hook bridge.
- Grok's missing session-start context is recorded in its dialect, so tool mode there expects one
  denial instead of failing.

### Fixed
- Codex guest installs lacked CA certificates, so TLS to a model provider failed silently.
- A guest install could be packed while broken, because npm exits 0 without optional native
  binaries; installs now keep optional packages and verify the entry point before packing.
- Two boxer processes racing smolvm now wait for the winner instead of failing.

## [0.1.0] - 2026-09-17

### Added
- First working slice: a microVM per git worktree, hooks in eight dialects, an MCP run tool, agent
  skills, project and user installs, `rewrite`/`tool`/`off` modes, and an inside mode where the
  harness itself runs in the guest (`boxer shell`, `boxer acp`).
- Evaluation suite: a scripted model server, a deterministic tier across every harness, and a live
  tier.
