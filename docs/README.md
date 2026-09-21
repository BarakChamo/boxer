# boxer documentation

User-facing documentation is the site under [`site/`](../site). `make docs-dev` serves it;
`make docs` builds it. It is the single source for the CLI reference, the configuration keys, the
guides and the evaluation results, so that no table exists in two places and drifts.

What lives here is the engineering record — the material that documents decisions and evidence
rather than usage.

## The record

- [architecture.md](architecture.md) — the pieces, and the rule that keeps harness names out of the core
- [requirements.md](requirements.md) — the specification, requirement by requirement, with verification status
- [release.md](release.md) — what 1.0 promises, versioning, and the release gate
- [status.md](status.md) — what the last full evaluation run proved, with dates and skips
- [orchestrators.md](orchestrators.md) — the research behind each orchestrator integration, including
  the Conductor checklist that has to be run by hand

## Plans and evaluations

- [eval-plan.md](eval-plan.md) — the tiers, the oracle, and what each one is for
- [plan-1.1.md](plan-1.1.md) — what 1.1 adds and why
- [evals/matrix/](evals/matrix) — every archived matrix run, newest first, with `runs.jsonl` for
  reading the series by machine

Reports regenerated from runs rather than written by hand: [eval-t1.md](eval-t1.md),
[eval-t2.md](eval-t2.md), [eval-adherence.md](eval-adherence.md), [eval-flow.md](eval-flow.md),
[eval-sdlc.md](eval-sdlc.md), [eval-matrix.md](eval-matrix.md).

## Pointers

These files are stubs that name their page on the site: [install.md](install.md),
[configure.md](configure.md), [integrate.md](integrate.md), [api.md](api.md),
[events.md](events.md), [troubleshooting.md](troubleshooting.md).
