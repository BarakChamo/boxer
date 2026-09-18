# Plan: boxer 1.1 — a development environment, not just a command sandbox

1.0 sandboxes commands. It cannot express what a project needs in order to *run*: a database, a
cache directory, a port, an environment variable, or a dependency install that survives into the
next worktree. This release closes that gap to about the level cloud sandboxes reach, and no
further. It also makes the core able to answer everything a user interface would ask, without
building one, and adds the first evaluation that runs a whole development session rather than a
single command.

Nothing here changes what 1.0 promised. Every stable surface in [release.md](release.md) stays
stable; this is additive.

---

## What the research found

Vercel Sandbox, E2B, Modal and Daytona were read directly, along with the devcontainer
specification and Next.js's agent tooling. Six findings shaped the plan.

**Everyone separates defining an environment from materialising one.** Modal and E2B build an
image. Vercel boots a base image, runs setup in the guest, and snapshots the result. smolvm cannot
build images, so boxer takes Vercel's path: pull, run setup, snapshot to a pack. That pack is the
build artifact — the equivalent of E2B's template id or Vercel's snapshot id — not a throwaway
cache.

**A local image is the escape hatch for repositories that do build.** smolvm accepts a
`docker save` archive, a rootfs directory, or stdin as its image. A project with a Dockerfile builds
it with its own tooling and hands boxer the result, which is exactly how E2B and Modal work: the
build happens outside the sandbox runtime. Already documented and fixed in 1.0's wake — the
allowlist no longer opens a registry for an image that is never pulled.

**The devcontainer properties that need no build are the ones worth reading.** `image`,
`postCreateCommand`, `postStartCommand`, `forwardPorts`, `containerEnv`, `remoteEnv`, bind
`mounts` and `workspaceFolder` are runtime-only. `features`, `build.dockerfile` and
`dockerComposeFile` all need an image build or multi-container orchestration, which is the part
boxer cannot do and should not pretend to.

**Readiness is a command, not a sleep.** E2B replaced fixed delays with a probe polled until it
exits zero. Anything that starts a server needs that.

**Snapshots need a retention policy from day one.** Vercel ships expiry and a keep-last count.
boxer reclaims by idle time already; packs need the same discipline as they multiply.

**An MCP server that must live beside the code can run in the sandbox today.** `next-devtools-mcp`
is a stdio proxy that discovers a running dev server and forwards calls to its `/_next/mcp`
endpoint, so it has to run where the code is. `boxer run` is a clean stdio pipe — verified by hand
with a JSON-RPC server in the guest answering requests piped from the host — so an ordinary
`.mcp.json` entry reaches it:

```json
{ "mcpServers": { "next-devtools": {
    "command": "boxer", "args": ["run", "--", "npx", "-y", "next-devtools-mcp@latest"] } } }
```

That is a capability boxer has and has never claimed.

## The defect this release is built around

`setup` runs once per VM and is **never baked into a pack**. The image pack is keyed on the image
alone, so every new worktree re-runs `bun install` from scratch while a pack of the bare image sits
beside it. It is the most expensive thing about boxer today, and fixing it is the backbone of
everything below.

---

## Stream A — environment packs

The pack key becomes the image plus a hash of everything that shapes the guest: `setup`, `start`,
`env` and the mount shape. A worktree whose environment matches an existing pack starts with
dependencies already installed; changing a setup line invalidates it, exactly like a Docker layer.

- Key derivation lives beside `PackPath`, and `doctor` prints the key and whether it is a hit.
- Retention: `packs_keep_last` alongside the existing idle rule, enforced by the sweep that already
  runs. Packs are 130 to 365 MB, so this is not optional.
- `boxer up --rebuild` forces a fresh environment when someone wants one.

**Proof:** a test that two worktrees of the same repository create the second VM from the pack and
never re-run setup; a smoke check that changing a setup line invalidates it; the second worktree's
cold start measured and bounded.

## Stream B — the configuration surface

Five additions, each with a precedent in a shipping product:

| Key | What it does | Precedent |
| --- | --- | --- |
| `env` | variables set in the guest for every command | all four |
| `mounts` | extra host directories, `["~/.cache/pip:/root/.cache/pip:ro"]` | Vercel Drives, Modal volumes |
| `start` | commands run at every VM start, detached — this is how a database runs | devcontainer `postStartCommand` |
| `ready` | a command polled until it exits zero, with a timeout, before boxer reports up | E2B `ready_cmd` |
| `packs_keep_last` | pack retention | Vercel `keepLastSnapshots` |

`setup` installs, `start` launches, `ready` waits. No supervision, no dependency graph, no restart
policy: if a started process dies, the next command fails and says so.

**Explicitly out of scope:** compose, service graphs, health-based restarts, image builds,
devcontainer features. A project needing those needs an orchestrator, and boxer should say so
rather than grow into one.

**Proof:** a smoke cell that installs and starts a real service, waits on `ready`, and reaches it
from the host through a forwarded port.

## Stream C — devcontainers

Read `.devcontainer/devcontainer.json` (and `.devcontainer.json`), mapping only what needs no
build:

| devcontainer | boxer |
| --- | --- |
| `image` | `image` |
| `postCreateCommand` | `setup` |
| `postStartCommand` | `start` |
| `forwardPorts` | `network.ports` |
| `containerEnv`, `remoteEnv` | `env` |
| `mounts` (bind) | `mounts` |
| `workspaceFolder` | `mount_at` |

`boxer.toml` overrides anything the file says, and `doctor` prints the file each value came from.
That ends the double-maintenance problem, which is the whole reason to do this.

Refused by name, each with one line saying why: `features` (OCI artifacts with their own install
protocol), `build`/`dockerFile` (no image build — the message points at the local-image path:
build it yourself, `docker save` it, set `image` to the archive), `dockerComposeFile`
(multi-container orchestration).

The file's real format is JSON with comments and trailing commas, so the parser must handle both.

**Proof:** a fixture repository with a realistic devcontainer file whose values all appear in
`doctor` with the right provenance, and a fixture using `features` that is refused clearly while
the mappable keys still work.

## Stream D — a core a user interface can sit on

No interface ships. The test is that someone could build one in a weekend without touching boxer.

| Question a dashboard asks | Today |
| --- | --- |
| What sandboxes exist, in what state, for which worktree? | answered |
| **Which harness and session is attached?** | **no — identity goes into the scope hash and is unreadable** |
| **What is it consuming?** | **no — smolvm knows the pid, CPUs, memory, mounts and ports; boxer surfaces none** |
| **How much disk does it hold?** | **no — the data directory is known but never measured** |
| What happened recently? | the event stream, when enabled |
| **Can I watch rather than poll?** | **no** |

The work:

- **Label what is attached.** Harness, session id and agent id recorded as labels at create time,
  so a listing says "Claude Code, session 4f2a, feature-x" rather than a hash.
- **Report resources.** `ls --json` and `status --json` gain a `resources` object: allocated CPUs
  and memory, resident memory and CPU read from the machine's process, the data directory's size,
  mounts and forwarded ports.
- **`boxer watch [--json]`** — line-delimited state changes and events, so an interface tails one
  process. The event stream exists; this exposes it live and adds machine transitions.
- **Make the schema the contract.** These shapes join the stable surface in [api.md](api.md).

**Proof:** a test that a sandbox created by a harness reports that harness and session in `ls
--json`; a test that `resources` reports a non-zero footprint for a running sandbox; `watch`
emitting a create, a run and a reclaim in order.

## Stream E — Conductor, through the harnesses

Settled by research: Conductor runs each harness's real binary against an ordinary git worktree,
and its own documentation confirms a repository's `.mcp.json` is inherited by the Claude Code
sessions it launches. The project layer *is* the integration, and boxer already writes it. Hooks
specifically are documented nowhere, so this is a run, not a reading.

1. Install the project layer, open a Conductor workspace, run a command, confirm the hook fired and
   the command ran in the guest.
2. Keep `.conductor/settings.toml` as the inside-mode option, pointing the executable paths at
   boxer's harness shims.
3. Update the status row from "checklist" to what the run actually showed.

Conductor's HTTP API drives cloud workspaces only, so a local driver stays impossible rather than
merely unbuilt. Note that its published settings table documents the Claude Code and Codex
executable paths; the OpenCode key boxer also writes is not in that table and is inert until shown
otherwise.

## Stream F — the flow tier: one real development session

Every cell today is one command in and one answer out. A development session scaffolds a project,
serves it, breaks it, reads a tool's view of the failure, fixes it, and checks again. The flow tier
runs that, once, per harness.

1. **Scaffold.** The agent runs `create-next-app` at a pinned version in the sandbox. This alone
   exercises the npm registry through the allowlist, a large install in the guest, and the mount:
   the files must land in the host worktree, because that is what the user edits.
2. **Serve.** `next dev` runs in the guest through `start`, `ready` waits for it, and the host
   reaches the page through a forwarded port. Both halves verified by hand already.
3. **Diagnose through MCP.** A deliberate error is introduced. The agent asks the dev server's own
   tools what is wrong (`get_errors`, `get_compilation_issues`) through `next-devtools-mcp` running
   *in the sandbox*, fixes it, and the page returns 200.
4. **Persist.** The sandbox goes down and comes back; dependencies survive in the environment pack
   and the server returns without reinstalling.

`boxer install` should offer to write that `.mcp.json` entry, since step 3 is the first time boxer
is the transport for someone else's MCP server.

**Determinism and cost.** Versions pinned, because a moving scaffold is not an oracle. Environment
packs make repeats cheap: the install happens once per host, not once per cell. The tier is slow
and network-heavy, so it runs before a release and on demand, never in the default gate, with a
separate unpinned canary allowed to fail loudly when upstream changes.

**What the oracle checks:** the scaffold landed in the host worktree; no command ran on the host;
the page answered 200 through the forwarded port; the MCP call actually happened, read from the
trace rather than from the model's prose; the error was fixed; the sandbox survived a restart with
its dependencies intact.

## Stream G — gaps and corrections

- **The requirements document contradicts itself** on devcontainers: one section specifies reading
  a subset, another lists consuming `devcontainer.json` as out of scope. Stream C resolves it in
  favour of reading; the document must be corrected in the same change.
- **Two inside-mode rows have never been run**, DSH and Copilot. Run them or mark them unverified.
- **Gemini's live tier needs its own key**, because the gateway speaks no Gemini protocol. That
  belongs in the README, not only in the status report.
- **Three publishing steps remain manual**: npm, the Homebrew tap (which needs a tap repository and
  a token secret), and the agentskills.io listing.
- **A `Backend` interface** stays deferred, but the pack work must not leak smolvm's pack format
  into the configuration surface.

---

## Order of work

1. **Stream A**, environment packs. Biggest win, and everything composes with it.
2. **Stream B**, the configuration surface, with the pack key covering the new keys.
3. **Stream C**, the devcontainer reader, once the keys it maps onto exist.
4. **Stream D**, labels, resources and `watch`. Independent; can run in parallel from the start.
5. **Stream E**, Conductor verification. An afternoon and a status line.
6. **Stream F**, the flow tier, last: it consumes everything above and is the acceptance test for
   the release.
7. **Stream G** lands alongside whichever stream touches the same file.

## The gate stays the gate

Unit tests with the race detector and per-package coverage floors on two platforms; lint and the
vulnerability scan clean; smoke against a real microVM; T1 twice with identical per-cell verdicts;
T2 and adherence live; the flow tier before the tag. A stable surface added here is stable from
1.1 onward, so `api.md` grows in the same change as the code.

## What 1.1 deliberately does not do

Compose or service graphs. Image builds or devcontainer features. A user interface. A backend other
than smolvm. Windows. Each is a real piece of work with its own design, and bolting a half version
of any of them onto this release would make the next one harder.
