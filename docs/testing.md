# Testing

boxer's claims are behavioural — *this harness's commands run in the VM*, *this level holds*, *a
worktree's sandbox is its own*. A unit test cannot establish any of them, because what is under
test is a real coding agent against a real hypervisor. So the test strategy is layered, and each
layer exists because the one below it cannot see something.

| Layer | What it can prove | What it cannot | Cost |
| --- | --- | --- | --- |
| Unit tests, fake smolvm | every code path, including failures | that smolvm behaves as the fake does | free, seconds |
| Real-daemon tests in `internal/vm` | docker, podman and Apple `container` argv, JSON and exit codes | anything on a machine without the runtime (they skip by name) | seconds |
| `make smoke` | every configuration path against a real sandbox, on each installed backend | that a harness cooperates | minutes per backend |
| `make eval-t1` | each integration works, scripted model | that a real model complies | minutes |
| `make eval-t2` | the same with live models | how often a model complies unprompted | cents |
| `make eval-adherence` | compliance when boxer only asks | that development works end to end | cents |
| `make eval-flow` / `sdlc` / `matrix` | a real workload, at every level, in parallel worktrees | — | up to ~$2 |
| `make bench` | what boxer costs, against every other sandbox on the host | that the ranking holds on other hardware | minutes |

[docs/eval-plan.md](eval-plan.md) covers the evaluation tiers in full, and
[bench/README.md](../bench/README.md) the benchmark. This page is about the two layers below
them, which are what you run while writing code.

## Unit tests

```sh
make test      # vet, race detector, coverage floor. Safe to run any time.
```

They never touch a hypervisor, so they are safe on any machine and in CI on both platforms.

### The fake smolvm

`internal/vmtest` is a shell script that stands in for the smolvm binary. It is deliberately more
than a stub, because the behaviour worth testing lives in the differences between machines and in
what happens when smolvm refuses:

- **It models any number of machines**, each with its own state, worktree, image and pack. `gc`,
  `down --all` and pack pruning have something real to sweep.
- **Its image and version are settable**, so the image detector and `doctor` can be observed.
- **Any verb can be made to fail** (`vmtest.FailVerb`, `FailExecOnce`), so error paths are
  reachable without a real VM.
- **`exec` runs locally with `sh`**, so tests see real exit codes and real output.
- **Labels are whatever was passed at create time**, as smolvm does it. A fake that modelled a
  fixed set would silently drop any label added later, and the test asserting on one would prove
  nothing.

```go
client, dir := vmtest.Install(t)          // fake smolvm on PATH
root := vmtest.Repo(t, `image = "alpine"`) // a git repo with a boxer.toml
vmtest.SetImage(t, "node:24")
vmtest.FailVerb(t, "machine create", "no space left on device")
```

The fake is the reason `make test` covers error handling at all. It is not evidence that smolvm
behaves this way — that is what `make smoke` is for.

### The scripted model

`internal/fakellm` is a model endpoint that speaks the Anthropic Messages API and the OpenAI Chat
Completions and Responses APIs, streaming or not. It plays one scenario: issue the scripted shell
commands one per turn through whatever shell tool the harness offers, then answer.

With it, a harness's hooks, its plugin and boxer itself run end to end with no model, no API key,
and the same result every time. This is what makes `t1` deterministic and free.

### Coverage floors

`.coverfloor` holds a per-package statement-coverage floor, checked by
`scripts/coverfloor.sh` in `make test` and in CI.

**A floor never goes down.** When a package gains coverage, raise its floor in the same change.
`internal/eval` is the exception and says so in the file: it is the evaluation harness itself,
proven by running the tiers rather than by unit coverage.

### The core-purity tests

`TestCoreNamesNoHarness` parses `internal/{box,vm,scope,config,decide,shim}` and fails if a
harness name appears in an identifier or a string literal. It is how "the core knows nothing about
any harness" stays true rather than remaining an intention. See
[adding-a-harness.md](adding-a-harness.md).

`TestCoreDoesNotImportCLI` does the same for presentation: nothing under `internal/` or `pkg/`
except `internal/cli` may import it, so the core — which is also a library — never decides whether
to print colour.

### Hermetic by construction

A unit test must not reach a real runtime, a real proxy or the source tree. Three ways that went
wrong, each now guarded:

- **A test that listed every backend deleted another test's container.** `rm --all` once implied
  every backend, and a unit test's `rm --all` reached the real docker daemon. `--all` is now the
  configured backend only, and tests that exercise listing name the fake backend explicitly.
- **The fake runs guest commands in the process's own directory**, which during `go test` is the
  package source. A probe's write once landed as `cmd/boxer/probe.txt`. A test whose command
  writes a file changes into a temporary directory first.
- **Every harness removes its scratch.** `TestEveryHarnessRemovesItsScratchDirectory` reads the
  benchmark and smoke scripts and fails if one allocates a temporary directory without removing
  it, or allocates before re-executing under the host lock, which orphaned two per run.

External programs a test needs — portless, `npx`, `gh`, podman — are small scripts put first on
`PATH` that record their arguments (`fakePortless` in `internal/box`, `fakeBin` in `cmd/boxer`).

## Smoke

```sh
make smoke                          # real smolvm, no model
BOXER_BACKEND=docker make smoke     # the same cells on another backend
scripts/linux-smoke.sh              # from a Mac: smolvm, docker and podman on Linux, in a Lima VM
```

`BOXER_SMOKE_RECORD=docs/smoke-results.jsonl` appends the run to the record the published tables
are checked against.

Every configuration path against a real sandbox: provision, setup, pack, run, ports, network
modes, reclaim, test-result summaries, capsule replay, named packs, forking, devcontainer
variables and users, three worktrees at three portless URLs, a worktree recreated at the same
path, host-state cleanup, and the management commands. A cell a backend cannot support is skipped
**by name**, from the capability table `boxer doctor --json` reports, so the output doubles as a
test of that table. It needs smolvm
installed and is serialised by a host lock, because several suites on one machine contend for the
same hypervisor.

The fork, named-pack and egress cells matter more than most: they are the ones that exercise
smolvm verbs (`machine branch`, `machine update`, `machine sync`, `machine egress-events`,
`pack create --from-vm`) whose behaviour the fake can only assert. The egress cells exist because
the fake was once fed boxer's own guess at smolvm's JSON and agreed with it while the feature
reported nothing against a real guest.

This is the layer that catches a divergence between the fake and the real thing. Run it before
anything that touches `internal/vm` or `internal/box` lands.

## Benchmarks

```sh
make bench     # real smolvm, plus Docker and Codex if they are installed
```

Latency against every other sandbox on the host — no sandbox at all, seatbelt (which is what
Codex's and Claude Code's native sandboxes use on macOS), `codex sandbox`, Docker both cold and
warm, and raw `smolvm machine exec`. The harness, its method and the committed results live in
[bench/](../bench/). It is not part of any gate: it measures the host as much as the code, and a
number that moves with the machine cannot fail a build.

The row worth watching while developing is raw `smolvm` against `boxer`. Everything else is the
platform; that gap alone is boxer's own code.

## Writing a test

- **A test that needs a repository uses `vmtest.Repo`**, which gives a real git worktree with a
  `boxer.toml`. Resolution depends on git, so a temporary directory is not enough.
- **A test for a refusal asserts on `Cause`**, not on the message. `Cause` is the stable
  identifier; `Reason` and `Fix` are prose and are allowed to improve.
- **A test for an integration level belongs in `internal/eval`**, not in a unit test. Whether a
  harness cooperates is not a property of boxer's code.

## What CI runs

`test` on Ubuntu and macOS (gofmt, `go vet`, `go mod tidy -diff`, race detector, coverage floor),
`lint` (golangci-lint, govulncheck), `package` (every rendered view is valid JSON, the checked-in
`plugin/` matches a fresh render, Agent Skills conformance, goreleaser config), `install-routes`
(`install.sh` and the npm package against a staged release), and `docs` (the site builds and
typechecks).

Nothing needing a hypervisor or a model runs in CI. `make smoke` and the evaluation tiers are part
of the release gate in [release.md](release.md), run by a person before tagging, with the result
recorded in the release notes.
