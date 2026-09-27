# scripts

Checks that run in CI and are worth running by hand, and one that needs a Mac with Lima.

**`coverfloor.sh`** enforces the per-package statement-coverage floors in
[`.coverfloor`](../.coverfloor). A floor never goes down: when a package gains coverage, raise its
floor in the same change.

```sh
go test -coverprofile=coverage.out ./... && ./scripts/coverfloor.sh coverage.out
```

**`install-routes.sh`** stages a fake release and installs it through both published routes —
`install.sh` and the npm package — so a broken install path fails before a tag does, rather than
on someone's machine after one.

```sh
./scripts/install-routes.sh
```

Both run in `.github/workflows/ci.yml` and again in the release gate.

**`linux-smoke.sh`** runs `evals/smoke.sh` on Linux from a Mac. It creates a throwaway Lima VM
(Ubuntu LTS, `vz`, nested virtualization), installs smolvm, docker and podman natively, runs the
suite once per backend, and deletes the VM. smolvm needs nested virtualization, so an Apple M3 or
later on macOS 15 or later; `brew install lima` first.

```sh
BOXER_SMOKE_RECORD=docs/smoke-results.jsonl scripts/linux-smoke.sh          # all three
scripts/linux-smoke.sh docker                                               # one
```
