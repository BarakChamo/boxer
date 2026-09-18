# boxer documentation

## Using boxer

- [install.md](install.md) — smolvm, the four ways to get the binary, and checking the install
- [configure.md](configure.md) — `boxer.toml`, what each key changes, per-harness overrides
- [integrate.md](integrate.md) — the five ways boxer reaches your agent, and which harness gets which
- [troubleshooting.md](troubleshooting.md) — the failures people actually hit, and what to do
- [api.md](api.md) — the `--json` shapes, exit codes, `pkg/boxer`, the MCP tools
- [orchestrators.md](orchestrators.md) — OpenHands, Paperclip, T3 Code, herdr, Conductor, Multica

## Working on boxer

- [architecture.md](architecture.md) — the pieces, and the rule that keeps harness names out of the core
- [requirements.md](requirements.md) — the specification, requirement by requirement, with verification status
- [eval-plan.md](eval-plan.md) — how boxer's claims are evaluated: the tiers, the oracle, the matrix
- [release.md](release.md) — what 1.0 promises, versioning, and the release gate
- [status.md](status.md) — what the last full evaluation run proved, with dates and skips

Evaluation reports, regenerated from runs rather than written: [eval-t1.md](eval-t1.md),
[eval-t2.md](eval-t2.md), [eval-adherence.md](eval-adherence.md).
