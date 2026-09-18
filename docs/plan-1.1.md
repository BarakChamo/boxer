# Plan: boxer 1.1 — a development environment, not just a command sandbox

1.0 sandboxes commands. It cannot express what a project needs in order to run: a database, a
cache directory, a port, an environment variable, or a dependency install that survives into the
next worktree. This plan closes that gap to roughly the level cloud sandboxes reach, and no
further. It also makes the core able to answer everything a user interface would ask, without
building one.

## What the research says

Vercel Sandbox, E2B, Modal and Daytona were read directly, along with the devcontainer
specification. Four conclusions shaped this plan.

**Everyone separates defining an environment from materialising one.** Modal and E2B build an
image; Vercel boots a base image, runs setup commands in the guest, and snapshots the result.
smolvm cannot build images, so boxer takes Vercel's path: pull, run setup in the guest, snapshot
to a pack. That pack is the build artifact, the equivalent of E2B's template id or Vercel's
snapshot id.

**A local image is the escape hatch for repositories that do build.** smolvm accepts a
`docker save` archive, a rootfs directory or stdin as the image, so a project with a Dockerfile can
build it with its own tooling and hand boxer the result. E2B and Modal work the same way: the build
happens outside the sandbox runtime. This is now documented and the allowlist no longer opens a
registry for an image it will never pull.

**The devcontainer properties that need no build are exactly the ones worth reading.** `image`,
`postCreateCommand`, `forwardPorts`, `remoteEnv`, `containerEnv`, `mounts` and `workspaceFolder`
are runtime-only. `features`, `build.dockerfile` and `dockerComposeFile` all require building or
orchestrating images, which is the part boxer cannot do and should not pretend to.

**Readiness is a command, not a sleep.** E2B replaced fixed delays with a probe polled until it
exits zero. A service that takes a variable time to accept connections needs that.

**Snapshots need a retention policy from the first day.** Vercel ships expiry and a keep-last
count. boxer already reclaims by idle time; packs need the same discipline as they multiply.

## The gap this closes first

`setup` runs once per VM and is never baked into a pack. The image pack is keyed on the image
alone, so every new worktree re-runs `bun install` from scratch while a pack of the bare image
sits beside it. That is the single most expensive thing about boxer today, and fixing it is the
backbone of this release.

---

## 1. Devcontainers

**Read the runtime subset; refuse the rest loudly.**

`boxer` reads `.devcontainer/devcontainer.json` (and `.devcontainer.json`) when present, mapping:

| devcontainer | boxer |
| --- | --- |
| `image` | `image` |
| `postCreateCommand` | `setup` (appended, in order) |
| `postStartCommand` | `start` (new; see §2) |
| `forwardPorts` | `network.ports` |
| `containerEnv`, `remoteEnv` | `env` |
| `mounts` (type=bind) | `mounts` |
| `workspaceFolder` | `mount_at` |

`boxer.toml` overrides anything the file says, and `boxer doctor` prints the file each value came
from, which is the whole point: a repository stops maintaining two truths.

What boxer refuses, by name, with one line saying why: `features` (OCI artifacts with their own
install protocol), `build`/`dockerFile` (no image build — the message points at the local-image
path: build it yourself, `docker save` it, set `image` to the archive), `dockerComposeFile`
(multi-container orchestration). A repository using those gets a clear message and the parts that do map still
work, rather than silence.

JSON with comments is the file's real format, so the parser must handle `//` and trailing commas.

**Why not more.** Supporting `features` means implementing an installer protocol; supporting
`build` means an image builder. Both belong to smolvm or a future backend, not to boxer.

## 2. Configuration, minimal

Five additions, each with a precedent in a product that ships:

- **`env`** — a table of variables set in the guest for every command. Distinct from
  `env_passthrough`, which forwards a host variable by name, and from `secrets`, which must never
  enter a pack.
- **`mounts`** — extra host directories, `["~/.cache/pip:/root/.cache/pip", "…:…:ro"]`. Today only
  the worktree is mounted, so a shared cache or dataset cannot be reached.
- **`start`** — commands run every time the VM starts, after `setup`, detached. This is how a
  database runs: `setup` installs it, `start` launches it. No supervision, no dependency graph, no
  compose. If the process dies, the next command fails and says so.
- **`ready`** — a command polled until it exits zero, with a timeout, before boxer reports the
  sandbox up. Copied from E2B, and the only honest way to wait for a service.
- **Environment packs** — the pack key becomes the image plus a hash of `setup`, `start`, `env`
  and the mount shape. A worktree whose environment matches an existing pack starts from it with
  dependencies already installed. Changing a setup line invalidates it, exactly like a Docker
  layer.

Retention follows Vercel: `packs_keep_last` and the existing idle rule, enforced by the sweep
that already runs.

**Explicitly not in scope:** compose, service dependency graphs, health-based restart policies,
image builds, features. If a project needs those it needs a real orchestrator, and boxer should
say so rather than grow one.

## 3. Conductor, through the harnesses

The research settles this. Conductor runs each harness's real binary against an ordinary git
worktree, and its own documentation confirms that a repository's `.mcp.json` is inherited by the
Claude Code sessions it launches. Project-layer configuration is therefore the integration, and
boxer already writes it.

The cheapest path, in order:

1. **Verify empirically.** Install boxer's project layer in a repository, open a Conductor
   workspace, run a command, and confirm the hook fired and the command ran in the guest. Hooks
   specifically are not documented either way, so this is a run, not a reading.
2. **Keep the settings file as the inside-mode option**, pointing the executable paths at boxer's
   harness shims for people who want the whole harness in the VM.
3. **Add `scripts.setup` guidance** that warms the sandbox as the workspace is created, which is
   already what boxer writes.

If the hook fires, Conductor moves from "checklist" to "supported through the project layer", and
its row cites the run. If it does not, the shim path is the answer and the row says that instead.
Conductor's HTTP API drives cloud workspaces only, so a driver for local workspaces remains
impossible, not merely unbuilt.

## 4. A core a user interface can sit on, with no interface yet

What a dashboard needs, and what is missing today:

| Question | Today |
| --- | --- |
| What sandboxes exist, in what state? | `boxer ls --json` answers it |
| Which worktree is this one for? | yes, from labels |
| **Which harness and session is attached?** | **no: identity goes into the scope hash and is unreadable** |
| **What is it consuming?** | **no: smolvm knows the pid, allocated CPUs and memory, mounts and ports; boxer surfaces none** |
| **How much disk does it hold?** | **no: the data directory is known but never measured** |
| What happened recently? | the event stream, when enabled |
| **Can I watch it live?** | **no: every answer is a poll** |

The 1.1 work is therefore:

- **Label what is attached.** Record harness, session id and agent id as labels at create time, so
  a listing can say "Claude Code, session 4f2a, feature-x worktree" instead of a hash.
- **Report resources.** `boxer ls --json` and `status --json` gain a `resources` object: allocated
  CPUs and memory, resident memory and CPU percentage read from the machine's process, the data
  directory's size on disk, mounts and forwarded ports.
- **`boxer watch [--json]`** — a line-delimited stream of state changes and events, so an
  interface tails one process instead of polling. The event stream already exists; this exposes it
  as a live channel and adds machine state transitions.
- **Make the schema the contract.** These shapes join the stable surface in `docs/api.md`, because
  an interface built on them must not break at 1.2.

No interface ships in 1.1. The test of this work is that someone could build one in a weekend
without touching boxer.

## 5. A flow tier: one real development session, end to end

Every existing cell is one command in and one answer out. A development session is not that: it
scaffolds a project, starts a server, breaks something, reads a tool's view of the failure, fixes
it, and checks the page again. The flow tier runs that session, once, per harness.

The scenario, in a fresh worktree:

1. **Scaffold.** The agent runs `create-next-app` in the sandbox. This alone exercises the npm
   registry through the allowlist, a large install in the guest, and the mount: the files must
   appear in the host worktree, because that is what the user edits.
2. **Serve.** `next dev` runs in the guest and the host reaches the page through a forwarded port.
   Proven to work today: a background server started inside the guest survives the command that
   launched it, and `curl` from the host reaches it.
3. **Diagnose through MCP.** A deliberate error is introduced. The agent asks the Next.js dev
   server's own MCP tools what is wrong (`get_errors`, `get_compilation_issues`), fixes it, and the
   page returns 200 again.
4. **Persist.** The sandbox goes down and comes back. Dependencies survive in the environment pack
   and the server returns without reinstalling anything.

**The finding that makes step 3 interesting.** `next-devtools-mcp` is a stdio proxy that discovers
a running dev server and forwards tool calls to its `/_next/mcp` endpoint. It therefore has to run
next to the code — which, here, is inside the sandbox. `boxer run` turns out to be a clean stdio
pipe, so a harness on the host addresses it with nothing new:

```json
{ "mcpServers": { "next-devtools": {
    "command": "boxer", "args": ["run", "--", "npx", "-y", "next-devtools-mcp@latest"] } } }
```

That is a capability boxer has and has never claimed: **an MCP server that must live beside the
code runs in the sandbox, with boxer as the transport.** Verified by hand with a JSON-RPC echo
server through `boxer run`. The flow tier is what turns it into a claim we can make, and
`boxer install` should offer to write that entry.

**Determinism and cost.** Versions are pinned, because `@latest` drifts and a moving scaffold is
not an oracle. The environment pack from §2 makes repeat runs cheap: the install happens once per
host, not once per cell. The tier is slow and network-heavy, so it runs before a release and on
demand, never in the default gate, and a separate unpinned canary cell may fail loudly when the
upstream template changes.

**What the oracle checks:** the scaffold landed in the host worktree; no command ran on the host
(the existing canary); the page answered 200 through the forwarded port; the MCP call actually
happened, from the trace rather than from the model's prose; the error was fixed; and the sandbox
survived a restart with its dependencies intact.

**Why it is worth the minutes it costs.** It is the first cell where boxer's own features have to
work together rather than in isolation — ports, mounts, background processes, packs, an MCP server
in the guest — and it is the closest thing to what a user will actually do on the first day.

## 6. Other gaps worth closing

- **The requirements document contradicts itself** on devcontainers: one section specifies reading
  a subset, another lists consuming `devcontainer.json` as out of scope. This plan resolves it in
  favour of reading the subset; the document must be corrected in the same change.
- **Two inside-mode rows have never been run**: DSH and Copilot. Either run them or mark them
  unverified in the table.
- **Gemini's live tier still needs a Gemini key**, because the gateway speaks no Gemini protocol.
  Worth stating in the README rather than only in the status report.
- **Three publishing steps remain manual**: the npm package, the Homebrew tap (which needs a tap
  repository and a token), and the agentskills.io listing.
- **A `Backend` interface** for Firecracker or Docker Sandboxes stays deferred, but the pack work
  here should not assume smolvm's pack format in the configuration surface.

## Order of work

1. Environment packs keyed on the environment hash. Biggest win, and everything else composes with
   it.
2. `env`, `mounts`, `start`, `ready`. The configuration surface, with the pack key covering them.
3. Devcontainer reader, once the keys it maps onto all exist.
4. Labels, resources and `boxer watch`. Independent of the rest; could run in parallel.
5. Conductor verification, which is an afternoon and a status-report line.
6. The flow tier, last, because it consumes everything above and is the acceptance test for it.

Each step keeps the release gate green: unit tests with the race detector, smoke against a real
microVM, T1 twice with identical verdicts, and the live tiers before a tag.
