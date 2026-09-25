# scripts

Two checks that run in CI and are worth running by hand.

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
