# Plan: the first production release

boxer has been a 1.0-rc with an evaluation record for a while. What stops it being a product
someone else depends on is not features — it is that the thing we ask people to depend on has
never been used by us.

## The defect this release is built around

`pkg/boxer` is documented as "the only importable package", and `cmd/boxer` does not import it.
The CLI imports thirteen internal packages directly. So the public API is an 84-line facade that
nothing exercises, whose types are aliases into `internal/` (`type Machine = vm.Machine`) — an
embedder receives values whose shape is defined in a package they cannot name. Everything the
1.1 work added — forks, named packs, capsules, test results, run records, the event stream — is
absent from it.

That is the wrong thing to freeze at 1.0, and it is cheapest to fix now, while nobody depends on
either surface.

## Stream 1 — make the kernel real, and make the CLI use it

The forcing function is the second consumer: whatever `cmd/boxer` cannot express through
`pkg/boxer` is a hole in the kernel, and there is no way to fake it.

1. Give `pkg/boxer` its own types instead of aliases into `internal/`. A `Machine`, a `Scope`, an
   `Error` and a `RunResult` that the package owns and can keep stable.
2. Cover what the CLI actually does: run (with results), fork, pack, capsule, config resolution
   and provenance, tasks, the brief, the event stream, gc and listing.
3. Port `cmd/boxer` onto it, command by command, deleting each direct `internal/` import as it
   goes. The end state is a CLI that imports `pkg/boxer` and `internal/{hook,install,bundle,mcp,
   shim,inside}` — the harness-facing half — and nothing else.
4. A test that fails if `cmd/boxer` imports an internal package outside that allowed set, in the
   shape of `TestCoreNamesNoHarness`. The rule holds itself up or it decays.

Explicitly **not** part of this: a second Go module. A kernel released on the CLI's cadence has
no independent versioning to justify one, and Go's own guidance is single-module by default.
`apps/` and `packages/` would move directories without fixing the thing that is wrong.

## Stream 2 — what only a person can finish

Carried from the 1.1 plan, which is otherwise done:

- **Conductor** is verified by checklist, not by a run. Install the project layer, open a
  Conductor workspace, run a command, confirm the hook fired and the command ran in the guest,
  and replace the status row with what the run showed. Its HTTP API drives cloud workspaces only,
  so a driver stays impossible rather than merely unbuilt.
- **Three publishing steps are manual**: npm, the Homebrew tap (which needs a tap repository and
  a token secret), and the agentskills.io listing. The `.well-known` discovery index is written
  and conformant only once the site is served from a root domain with `BOXER_BASE_PATH=""`.

## Stream 3 — the evidence a release needs

- Re-run every tier on the release candidate: `make test`, `make smoke`, `eval-t1`, `eval-t2`,
  `eval-adherence`, `eval-flow`. The matrix and sdlc tiers are slow and expensive; run them once
  against the candidate rather than per change.
- Cover the 1.1 capabilities at the tier that can see them. Smoke covers results, capsules, packs
  and forks against real VMs today. Nothing covers an **agent** using them, and the adherence
  tier is where that belongs: does a live model reach for a named task now that tasks carry
  descriptions, and does it read a JUnit summary rather than re-running the suite?
- Capsule replay has only ever run on the machine that recorded it, which is the least
  interesting case for an artifact whose point is being handed to someone else.
- **Fix T2's tool-mode scoring before it can gate anything.** Two full runs on one tree each lost
  three cells, sharing only one; every tool-mode failure was `deny: 1 denial(s): the agent had to
  be corrected`, on a different harness each time. That is enforcement working, scored as a
  failure, and with one model there is nothing to distinguish it from a regression. Either those
  cells vote across models the way adherence does, or a single denial-and-recovery stops counting
  as a failure. Until then T2 is a signal to read, not a gate to pass.
- **Carry the benchmark forward.** [bench/](../bench/) measures boxer against no sandbox,
  seatbelt, `codex sandbox`, Docker cold and warm, and raw `smolvm machine exec`, and the numbers
  are committed with the harness. It is deliberately not in any gate — it measures the host as
  much as the code. `make bench-devserver` is the one that matters, and this slice used it to find
  and fix three defects — 415 ms DNS lookups, the fattest published images as defaults, and work
  done after a command had already returned. Starting a session is now 7.5 s against Docker's
  10.2 s and 7.2 s for no sandbox at all, so bring-up is no longer an adoption risk.
  Two items stay open and are worth a decision rather than a silent carry:
  **installs are about twice Docker** (9.0 s against 4.7 s) with the cause unattributed, and
  **a restart gains nothing from a warm build cache** where Docker halves. Neither blocks a
  release; both are the next profiling session. The `4 vCPU / 4 GB` default also costs ~11% on
  an install and is deliberately conservative because boxer runs one VM per worktree — raise it
  on purpose or document it, but do not leave it undecided.
- **The `boxer_task` MCP tool's trigger is met.** 1.1 deferred it until a model was observed
  composing a command line instead of naming a declared task. The adherence tier's `task`
  scenario now observes exactly that, on three harnesses across all four models and on twelve
  further cells intermittently. Decide it for this release rather than re-deferring it silently.

## The gate

A release happens when, on one tree, in one sitting: unit tests pass under the race detector with
the coverage floors met; lint and the vulnerability scan are clean; smoke is green against real
smolvm; T1 is green with every skip explained; flow is green; the package and skill trees have no
drift; and `install.sh` and the npm wrapper install the built artifact on a machine that had no
boxer.

T2 and adherence are **read, not passed**. Neither is deterministic — a live model decides each
cell — so "green" is not a property either can be held to, and requiring it would train whoever
runs the gate to re-run until the dice land. What they must show instead: no `leak:` finding
anywhere (a command reaching the host is never acceptable and never stochastic), no new `harness`
verdict that was not there before, and a written reason beside every failure. A benchmark run is
attached for the record and gates nothing.

## What this release deliberately does not do

No second backend, no coordinator, no fleet. A `Backend` interface arrives when a second
implementation does, and [adding-a-backend.md](adding-a-backend.md) is the contract it will have
to satisfy.
