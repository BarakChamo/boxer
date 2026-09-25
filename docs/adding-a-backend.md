# Adding a backend

boxer has four backends — smolvm, Apple's `container`, docker and podman — behind `vm.Backend`.
This page is the
contract each of them answers, and it was written before the interface existed, deliberately: the
interface then fell out of two real implementations rather than one imagined one.

It is still the page to read before adding a fifth. Nothing below changed when docker arrived,
which is the strongest thing that can be said for it.

**What the current backends answer.** Every rule below is satisfied by smolvm; the container
backends refuse what they cannot honour, by name, rather than degrading:

| | smolvm | Apple `container` | docker / podman |
| --- | --- | --- | --- |
| boundary | a kernel per sandbox | **a kernel per sandbox** | one shared kernel |
| worktree | mounted | mounted | mounted |
| `network.mode = "allowlist"` | enforced | **refused** | **refused** — see rule 3 |
| environment cache (packs) | yes | refused | refused; `image_setup` re-runs per worktree |
| fork / branch | yes | refused | refused |
| egress denials reported | yes | refused | refused |
| secrets kept out of argv | `--secret-env` | a 0600 `--env-file` | a 0600 `--env-file` |

Apple's `container` is the one worth studying before adding a fifth: it has smolvm's boundary and
docker's refusals, so it belongs to neither group the interface was first written against, and it
needed no new concepts. A contract that survives a backend it did not anticipate is doing its job.

Real-daemon behaviour is covered by `evals/smoke.sh`, which runs the same 65 cells against any
installed backend (`BOXER_BACKEND=docker make smoke`) and skips capability-gated cells **by name**,
so its output is also a test of the capability table. There is no fake docker: a fake CLI would
only prove boxer's argv matches what the fake was written to expect, and both are written from the
same reading of the docs, so they would be wrong together.

## The concrete second implementation

The nearest real candidate is **smol cloud** — the same vendor, the same `Machine` API, reached
with `SMOL_CLOUD_TOKEN` and an `smk_…` key instead of a local hypervisor. It is worth naming
because it breaks boxer's central assumption:

| | smolvm local | smol cloud |
| --- | --- | --- |
| Host mounts (`MountSpec`) | yes | **no** |
| `run`, `pullImage`, `listImages` | yes | no |
| Public URL / `endpoint()` | no (ports only) | yes |
| `branch()` | yes | yes |
| Where the worktree lives | mounted, one copy | copied in, two copies |

boxer's whole ergonomics rest on the mount: there is no sync step, no manifest, no fingerprint,
no copy-back, and a JUnit report the guest writes is already a file on the host. A cloud backend
has none of that. It needs an archive of the worktree, a decision about what to ship back, and an
answer for every feature that assumed one copy of the tree.

That is the shape of every remote execution tool, and it is worth reading one that has solved it
before writing our own: crabbox's delegated-runner contract is the reference.

## The contract

A backend must answer these, and a backend that cannot answer one must say so rather than guess.

**1. Lifecycle.** Create, start, stop, delete, and status, for a machine named by a scope key.
Delete tolerates an already-deleted machine. Create is safe to race: two boxers provisioning one
scope must not produce two machines, and the loser must recognise "already exists" rather than
fail the user's command.

**2. Command execution returns an exit code.** *Backend success is not command success.* A
backend that reports "the job completed" without the exit status of the command boxer asked for
has failed, and boxer must treat a missing or unparseable exit code as a backend failure — never
as a pass. This is the rule that decides whether a green CI run means anything.

**3. It rejects what it cannot honour.** A backend that does not support a TTY, a timeout, a
staged mount, or a forwarded secret must refuse the option, naming it, rather than run the
command without it. Silently degrading is worse than failing: the command appears to work and
the property the user asked for is gone. Every option boxer's CLI accepts needs a per-backend
answer of supported, refused, or emulated-and-documented.

**4. Workspace transport is explicit.** Either the backend mounts the worktree, in which case
host and guest see one set of files and boxer's existing promises hold, or it copies it, in which
case the contract has to state: what is sent (tracked plus non-ignored, as git reports it), what
bounds the size, what comes back, and when. Everything boxer says about "the same files" is
conditional on this answer, and the agent brief has to change with it — as it already does for a
staged fork.

**5. Secrets never reach an argument vector.** boxer forwards `secrets` and `env_passthrough` by
name, through smolvm's `--secret-env`, because a value in argv is a value in `ps`. A backend with
no equivalent must say so, and boxer must warn rather than fall back silently.

**6. Ownership is provable.** Every machine boxer creates carries its labels, and boxer deletes
only what it can prove it owns. A name is not proof. A backend whose API cannot attach and read
back metadata cannot be swept safely, and `gc` must leave its machines alone.

**7. Streaming, not buffering.** Output reaches the caller as the command produces it. A backend
that can only return the whole log at the end is usable but is a visible downgrade, and belongs
behind a documented capability rather than in the default path.

## What the interface looks like

It landed after the second implementation, as this page said it should. `vm.Backend` is the nine
methods every backend must have — `Name`, `Version`, `Status`, `List`, `Create`, `Start`, `Stop`,
`Delete`, `Exec` — and what only some can do is an optional interface found by type assertion:
`Packer`, `Brancher`, `EgressReporter`, `DiskReporter`. `vm.CapsOf` derives the capability table
from which of those a backend implements, plus the facts no method expresses (boundary, allowlist,
ownership, whether the mount is owned by the host uid), which a backend states with `Declare()`.
`boxer doctor` and `boxer backends` print that table; a feature a backend lacks is refused by name
with `vm.Unsupported`.

What a new backend has to get right, learned from the four that exist:

- **Typed errors.** Classify the runtime's own stderr once, in the driver, into `ErrNotFound`,
  `ErrNotRunning` and the rest. Call sites use `errors.Is`, never a substring.
- **The runtime's failures are not exit codes.** docker and Apple's `container` exit 1 for "no
  such container", podman 125, docker 127 for "OCI runtime exec failed". Recognise the message at
  the head of stderr and return an error; otherwise a command that never ran is recorded as having
  failed (contract rule 2).
- **`Start` waits for running.** A daemon that has accepted a start has not necessarily started
  the process.
- **Ownership is a label read back.** A runtime with no labels uses the registry in `owned.go`,
  whose failure mode is leaking, never deleting something boxer did not create.
- **Run the smoke suite on it.** `BOXER_BACKEND=<name> make smoke`. Every backend so far has had a
  defect that only the real runtime showed.

## What does not change

The enforcement layers. Hooks, shims, the substituted shell, MCP and inside mode all sit above
`boxer run` and know nothing about where the command executes. That is the property worth
protecting with every backend added: whatever else is negotiable, the agent still cannot opt
out of the sandbox.
