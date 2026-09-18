# Releasing boxer

One repository, one version, five artifacts. Nothing here is a second module or a second build
system; split only when a consumer needs `pkg/boxer` without the CLI's dependency set.

| Artifact | What | Where |
| --- | --- | --- |
| cli | the `boxer` binary for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 | GitHub Releases, `go install`, `install.sh`, Homebrew, npm |
| plugin bundle | `boxer package all` output: one Agent Plugins package plus one view per harness | `boxer-plugins-<version>.tar.gz` on the release |
| skill | the same package's `skills/boxer/` on its own | `boxer-skill-<version>.tar.gz` on the release |
| core | `pkg/boxer`, the `--json` shapes, the MCP server | this module |
| evals | `cmd/boxer-eval`, `cmd/fakellm`, `evals/`, `adapters/` | in the repository, never released as binaries |

## What is stable at 1.0

These are a promise. Breaking one of them requires a major version.

- **CLI commands and flags.** Every command in `boxer --help` and every flag it documents. A flag
  may gain a new value; an existing value keeps its meaning.
- **`--json` output shapes.** `status`, `ls`, `doctor`, `down`, `gc --dry-run`, and any other
  command that grows `--json`. Fields may be added. Existing fields keep their name, type and
  meaning, and a field that was always present stays present. Consumers must ignore unknown
  fields. The shapes are in [api.md](api.md).
- **Exit codes and the error contract.** `boxer: <reason>` with `scope`, `worktree`, `cause` and a
  runnable `fix:` line, and the exit code table in [api.md](api.md).
- **The skill and plugin layout.** The directory names and file names inside the published
  package: `plugin.json`, `mcp.json`, `AGENTS.md`, `skills/boxer/SKILL.md` and its `scripts/` and
  `references/`, and the reverse-domain directory per harness. A harness that installs the package
  by path keeps working.
- **MCP tool names.** `boxer_run`, `boxer_status`, and their input schemas' existing fields.
- **`pkg/boxer`.** Its exported functions, types and their fields. It is the only importable
  package.
- **Configuration keys.** A key in `boxer.toml` keeps its name, type and default. New keys are
  additive.

## What is not stable

- **`internal/*`.** Every package under it, without exception. It is not importable and its shape
  changes whenever the implementation does.
- **The evaluation suite.** `cmd/boxer-eval`, `cmd/fakellm`, `evals/`, the tier names, the report
  formats and `docs/eval-*.md`. These are maintainers' tools and change with the evidence they
  gather.
- **`boxer acp`'s transport details.** The command is stable; the framing, the subprocess layout
  and which harness binary it launches are not, and they follow the Agent Client Protocol.
- **Log and trace output**, including `BOXER_TRACE`, and anything printed without `--json`.
- **The guest's contents.** Image defaults, pack layout, mount points and the installed tool set
  are implementation, chosen per image and per harness.

## Versioning and deprecation

Semantic versioning. Patch releases fix behaviour without changing a contract; minor releases add
to one; major releases break one.

A stable surface is never removed without first being deprecated for **one minor release**. In
that window the old form keeps working, using it prints a warning naming its replacement, and
`CHANGELOG.md` carries a Deprecated entry. The removal lands in the next major release. A
deprecation that ships in 1.4.0 can therefore be removed no earlier than 2.0.0, and never before
1.5.0 exists.

The version comes from the git tag: `-ldflags -X main.Version=<tag>`, which goreleaser sets and
`make build` fills from `git describe`. The same value is rendered into `plugin.json`, every
native manifest, `SKILL.md` and `AGENTS.md`, so `boxer doctor` can warn when a repository's
installed layer is older than the binary.

## Reproducibility and provenance

Binaries are built with `-trimpath` and a `mod_timestamp` taken from the commit, so rebuilding the
same tag on the same Go version produces byte-identical binaries. The archives around them and the
SBOMs carry their own timestamps and are not byte-identical between runs; compare the binaries,
not the tarballs.

Each release carries a syft SBOM per archive and a `checksums.txt` signed with cosign keyless,
against the release workflow's GitHub OIDC identity — there is no private key anywhere:

```sh
cosign verify-blob checksums.txt \
  --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/BarakChamo/boxer/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## The release gate

Every line is a command someone other than the author can run. The mechanical half runs in
`.github/workflows/release.yml` on the tag, and locally as `make release-gate`. The rest needs a
real microVM, real harness CLIs and real credentials, so a person runs it and records the result
in the release notes.

| # | Gate | Command | Passes when |
| --- | --- | --- | --- |
| 1 | Unit, race, coverage | `make test` on Linux and macOS | green; every package meets its floor in `.coverfloor` |
| 2 | Lint, vulnerabilities, tidy, format | `make lint`, `govulncheck ./...`, `go mod tidy -diff`, `make fmt-check` | clean, and the tree is not left dirty |
| 3 | Package and skill validation | `go run ./cmd/boxer package all --out dist/gate`; `npx @agentskills/skills-ref validate` | every manifest validates against the vendored Agent Plugins schemas; the checked-in package matches what the templates render |
| 4 | Release configuration | `goreleaser check`; `goreleaser release --snapshot --clean --skip=publish,sign` | valid; four archives, four SBOMs and a checksums file are produced |
| 5 | Smoke | `make smoke` | every check passes; cold start, warm run and hook latency within their printed bounds |
| 6 | T1, twice | `make eval-t1` twice | zero failures both times, and the two runs give identical per-cell verdicts; every skip has a named reason |
| 7 | T2 live | `make eval-t2` | zero failures; skips only for a named missing credential |
| 8 | Adherence | `make eval-adherence` | no harness verdict regresses against the previous release; per-model numbers recorded in `docs/eval-adherence.md` |
| 8a | Flow | `make eval-flow` | the development session passes: scaffold, serve, reach it from the host, MCP in the guest, restart with the environment intact. Slow and network-heavy, so it runs here rather than in CI |
| 9 | Orchestrators | the orchestrator cells in T1 and T2 | OpenHands, Paperclip, T3 Code and herdr pass live; the Conductor and Multica checklists in [orchestrators.md](orchestrators.md) are current |
| 10 | Install routes | `make install-routes`, plus each route on a machine that has never had boxer | `install.sh`, npm, Homebrew and `go install` each produce a binary that passes `boxer doctor`; a tampered checksum aborts both download routes |

Gates 1 to 4 and the staged half of 10 are enforced by the workflow. Gates 5 to 9 and the
clean-machine half of 10 are the human half; a release whose notes do not name their results has
not passed the gate.

Then: regenerate `docs/status.md`, `docs/eval-t1.md`, `docs/eval-t2.md`, `docs/eval-flow.md` and
`docs/eval-adherence.md` from those runs, commit them, update `CHANGELOG.md`, tag, and let the
workflow publish. Two steps stay manual because they need an account the workflow does not hold:

```sh
npm login && cd npm && npm publish      # boxer-cli; the tarball is already on the release
```

and the agentskills.io listing. The Homebrew tap is automatic when `HOMEBREW_TAP_TOKEN` exists as
a repository secret; without it the cask is still built and attached to the release, and only the
push to the tap is skipped, so a release never fails for a missing secret.

A tag naming a candidate (`v1.0.0-rc.1`) publishes as a prerelease, so `install.sh` and Homebrew
keep handing out the last stable version.

## Cutting a release

```sh
make release-gate                           # the mechanical half, about two minutes
make smoke && make eval-t1 && make eval-t2  # the half that needs a real VM
git tag v1.2.3 && git push --tags           # the workflow runs the gate again, then publishes
```

Locally, `goreleaser release --snapshot --clean --skip=publish,sign` builds every artifact without
publishing. Signing is skipped locally on purpose: keyless cosign wants an OIDC identity, and the
only one that should ever sign a boxer release is the release workflow.
