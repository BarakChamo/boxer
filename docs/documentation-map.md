# Documentation map

What boxer documents, where each piece lives, and who it is for. This file exists so that a gap is
visible before a reader finds it, and so that no fact is written down in two places where the two
copies can disagree.

## The four sources, and what each one owns

| Source | Audience | Owns | Does not own |
| --- | --- | --- | --- |
| `site/` (Fumadocs) | people using boxer | install, concepts, how-to, CLI/config/API reference, measured support | why a package is shaped the way it is |
| `docs/` | people working on boxer | architecture, requirements, release contract, evaluation design and results | anything a user needs to run boxer |
| `README.md` and folder `README`s | someone landing in a directory | what this directory is, what to run, where to go next | anything longer than a screen |
| Go doc comments | someone reading the code | why the code is shaped this way, which requirement it satisfies | usage a reader should get from the site |

The rule between them: **the site is the single source for user-facing tables.** The CLI reference,
the configuration keys, the JSON shapes and the support matrix exist once, on the site. `docs/` and
the READMEs link to them rather than restating them, because a table in two places drifts and the
stale copy is the one the reader finds first.

## Diátaxis placement

The site is organised by what the reader is trying to do, not by feature.

| Quadrant | Where | Pages |
| --- | --- | --- |
| Tutorial (learning) | `start/` | install, first-run, harnesses |
| How-to (a problem) | `guides/` | environment, tasks, results, worktrees, forking, enforcement, inside, orchestrators, building on boxer, troubleshooting |
| Reference (facts) | `reference/` | cli, configuration, json, mcp, go, events, glossary |
| Explanation (understanding) | `concepts/`, `evals/` | how it works, scopes and packs, security model, why evaluations |

There is deliberately **no CI guide**. Running boxer on a hosted CI runner needs nested
virtualisation that nothing in this repository has verified, and an unverified how-to is worse
than an absent one.

`guides/how-it-works` is explanation living in the how-to section — a known compromise, kept
because readers look for it there.

## The system, as a reader has to understand it

Written out so that a page can be checked against it rather than against memory.

### What boxer is

One Go binary. It puts the shell commands a coding agent runs into a
[smolvm](https://smolmachines.com) microVM instead of on the host, one VM per git worktree, with
the worktree mounted in the guest at `/workspace`. The agent is not told.

boxer keeps no state of its own beyond a few markers. smolvm holds the machines, the images and the
packs; git holds the code.

### The load-bearing decisions

Every one of these is a question a reader will eventually ask. Each needs a documented answer.

- **Why the worktree is the unit.** Agents work in worktrees; orchestrators cut one per task. Keying
  a sandbox to the worktree means parallel agent work needs no arrangement. `isolation` widens it to
  `repo` or narrows it to `session`/`subagent`.
- **Why the mount is one fixed path with the working directory mapped into it.** `/workspace` by
  default; inside mode uses the worktree's host path instead, because there the harness is in the
  guest and the paths it prints are read on the host.
- **Why five integration levels instead of one.** Harnesses differ in what they let a third party
  do. The levels stack; `boxer install` picks the strongest available.
- **Why hooks enforce and the skill only asks.** A rewritten command cannot be evaded; a skill can
  be ignored. The `adherence` tier measures how often it is.
- **Why `git`, `gh` and `ssh` pass through.** They need host credentials and the real repository.
- **Why `image_setup` and `setup` are two lists.** One is captured in the pack, the other lands in
  the worktree, which a pack cannot carry.
- **Why the pack key hashes the whole environment.** Change anything that shaped the pack and the
  next VM builds a fresh one instead of inheriting a stale one.
- **Why the core contains no harness name.** A harness is a table row. `TestCoreNamesNoHarness`
  enforces it, so a tenth harness cannot grow the core.
- **Why there is no backend interface.** One implementation. An interface with one implementation
  is a liability, recorded as out of scope until a second exists.
- **Why telemetry is off by default.** With no `[telemetry]` table boxer writes nothing anywhere.
- **Why claims are evaluated, not asserted.** The claims are behavioural, so unit tests cannot
  establish them. Six tiers do, and their results are published with the conditions that produced
  them.

### What boxer supports

- Harnesses: Claude Code, Codex, Gemini CLI, OpenCode, pi, Grok, Kimi, GitHub Copilot CLI, DSH, fx
  (inside only).
- Orchestrators: OpenHands, Paperclip, T3 Code, herdr, Conductor, Multica.
- Integration levels: MCP + skill, hooks (rewrite and tool), the OpenCode plugin, PATH shims
  (per-program and whole-shell), inside mode.
- Hosts: macOS on Apple Silicon, Linux on x86-64 and arm64.
- Config sources: `boxer.toml` at three locations, `BOXER_*`, and an existing
  `.devcontainer/devcontainer.json`.
- Machine surfaces: `--json` on the read commands, `boxer watch --json`, the MCP server, and
  `pkg/boxer`.

### What boxer does not support, and why

Each of these is measured or deliberate, and each belongs in the docs rather than in a reader's
surprise.

- **Windows.** smolvm runs there; boxer's worktree model depends on POSIX paths and mounts.
- **An adversary who knows they are sandboxed.** The microVM is a real boundary, but the threat
  model is a careless agent, not a deliberate escape.
- **PATH shims under a login shell.** `path_helper` outranks boxer. `doctor` detects it and names
  the enforcement to use instead.
- **Gemini CLI on the shared gateway.** It speaks only the Gemini API; its live cells skip.
- **fx outside the guest.** No hooks, no headless shell denial, resolves past a PATH shim.
- **Conductor automated verification.** Native app, no debugging port; its CLI drives cloud
  workspaces where there is nothing local to sandbox.
- **A second VM backend.** Out of scope for 1.0.

## What each review closed

Assessed 2026-09-21, after a documentation pass and three fresh-reader reviews that were given the
docs and forbidden the code.

### Added

| Page | Type | Why it was missing |
| --- | --- | --- |
| `concepts/security.mdx` | explanation | the boundary was three sentences on the index |
| `concepts/scopes-and-packs.mdx` | explanation | the two load-bearing mechanisms were spread across three pages |
| `guides/tasks.mdx` | how-to | `[tasks]` is the agent-facing contract and had no page |
| `guides/inside.mdx` | how-to | inside mode was described five times and documented nowhere |
| `guides/building-on-boxer.mdx` | how-to | building on boxer was a code block inside a reference page |
| `reference/mcp.mdx` | reference | the MCP server was a section of the Go page |
| `reference/glossary.mdx` | reference | `scope`, `pack`, `level`, `cell`, ACP were terms of art |
| `guides/results.mdx` | how-to | JUnit summaries and capsules, the two things that turn a failed run into an answer |
| `guides/forking.mdx` | how-to | forking changes what the parent sandbox sees, which needed a page rather than a sentence |
| `docs/testing.md` | explanation | the layers below the eval tiers were undocumented |
| `bench/README.md` | explanation | the performance claims had a number and no method |
| `docs/adding-a-backend.md` | how-to | the contract a second backend must satisfy, written before the interface exists |
| `docs/adding-a-harness.md` | how-to | "four steps" with no detail |
| `cmd/`, `internal/`, `pkg/boxer/`, `scripts/`, `npm/`, `adapters/` READMEs | orientation | a contributor opened a directory and found nothing |
| `site/README.md` | orientation | was Create-Fumadocs boilerplate describing someone else's project |

### Corrected

Facts the docs stated and the binary contradicted. Each was found by reading one against the other.

- The worktree is mounted at **`/workspace`**, not at its host path. Only inside mode uses the host
  path. This claim appeared in the README, the index, the tutorial, the architecture note and
  R-LVL-5.
- `boxer down` **deletes**. The tutorial called it a stop and promised "nothing is lost".
- `network.mode` defaults to `allowlist`, not `on`. `require_worktree` takes `require`, not
  `error`. The housekeeping defaults were four different numbers from the shipped ones.
- `pkg/boxer` was "experimental, and not covered by the stability contract" in two places and a
  1.0 promise in `release.md`. It is experimental **until** 1.0 and covered **from** 1.0.
- `mode` defaults to `rewrite`; R-LVL-1 said `tool`. The run tool is `boxer_run`; R-LVL-4 named a
  rename that never shipped. Grok rewrites; R-LVL-7 held it at Level 0. Inside mode ships; §2.2
  called it out of scope.
- PATH shims and `tool` mode **narrow** the interception gap. `SECURITY.md` said they close it.
- `enforcement = "shim"` with the default `intercept` breaks a node-based harness, because the
  defaults contain `node`. Nothing said so.
- A guest service must bind `0.0.0.0` for a forwarded port to reach the host. Nothing said so, and
  every example bound loopback.

### The one that is structural

`docs/requirements.md` is linked from the README as "the specification" and read as authoritative,
but it is a written document rather than a generated one, so it drifts against the binary the way
any prose does. A pass on 2026-09-21 found it stating `tool` as the default mode, a run tool named
`bash` that never shipped, Grok held at Level 0, inside mode out of scope, `setup` running once per
VM, and four config defaults that had moved. All are corrected.

The structural fix is not another pass. Either the config block in §5 is generated from
`config.Defaults()`, or it is labelled as illustrative and the site's reference becomes the only
statement of defaults. Until one of those happens, this file will drift again, and it is the file
a reader trusts most.

### Still open

- **`guides/environment.mdx` and `reference/configuration.mdx` overlap.** The `image_setup`/`setup`
  split is now written in four places. It is the top support burden, so repetition is defensible;
  four copies is one or two too many.
- **Three support matrices.** `README.md`, `docs/status.md` and `evals/results.mdx` each carry one.
  The README's is a summary and status.md's is the record, but the rule in this file says one copy.
- **Multica is documented from what its binary reads, not from a run.** Its cell reports itself
  skipped. The page says so.
- **Linux hosts are under-served.** Every worked example is a Mac, and the Keychain and
  `path_helper` discussions are macOS-only.
- **`docs/status.md` is written by hand from generated reports.** It was a run behind the
  scorecard in `docs/eval-matrix-report.md` until 2026-09-21. It should either be generated or
  should cite the report rather than restating its numbers.
- **`boxer watch` is documented but not covered by the stability contract**, while every page that
  mentions dashboards points at it. Either the contract grows to include it or the pages say so.
  They currently say so.

## Deployment

The site is a static export, published to GitHub Pages by `.github/workflows/pages.yml` on every
push to `main` that touches `site/`.

- `output: 'export'` with `trailingSlash: true`, so Pages serves directories rather than
  extensionless files.
- `BOXER_BASE_PATH` carries the project prefix (`/boxer`). A custom domain sets it to `""`; a local
  `next dev` leaves it unset.
- Search is a build-time index (`staticGET`) with the client in `static` mode, because Pages runs
  no server to answer a query.
- `.nojekyll`, without which Pages' Jekyll pass drops every `_next/` directory.

```sh
make docs-export     # exactly what the workflow builds, into site/out
```

The three `actions/*-pages` pins in that workflow were added without network access to verify them
against upstream. Confirm the SHAs before relying on it.

## Keeping it honest

- A support claim on the site comes from a matrix run, not from the existence of code.
- A configuration key documented on the site exists in `internal/config`.
- The rendered plugin READMEs under `build/` and `plugin/` come from
  `internal/bundle/templates/`. Edit the template; `make package` regenerates the rest.
- Evaluation reports under `docs/eval-*.md` are regenerated by `make eval-*`. Do not hand-edit.
