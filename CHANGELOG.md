# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Environment packs.** `setup` was run once per VM and never cached, so every worktree repeated
  `bun install` from scratch while a pack of the bare image sat beside it. The result of setup is
  now packed and keyed on the image plus the setup commands, so a second worktree of the same
  repository starts with dependencies already installed — measured at 0.7 s against 2.9 s, and the
  saving grows with the size of the install. Changing a setup line changes the key, exactly like a
  Docker layer.
- **`start` and `ready`.** `setup` installs a service, `start` launches it detached at every VM
  start, and `ready` is polled until it exits zero before boxer reports the sandbox up. A dev
  server or a database now runs in the sandbox and answers the host through a forwarded port,
  verified against a real microVM. No supervision and no dependency graph: a dead process makes the
  next command fail and says where its output is.
- **`env` and `mounts`.** `env` reaches every command including setup, and travels into the
  environment pack; secrets stay in `secrets` and `env_passthrough`, which are read from the host
  at run time and never snapshotted. `mounts` adds host directories beside the worktree, with `~`
  expanded, usually for a shared dependency cache.
- **A flow tier**: one development session end to end — scaffold a pinned Next.js app in the
  sandbox, serve it, reach it from the host, speak MCP to a server running in the guest, restart and
  find the dependencies still there. Passes in 54 s warm, 3 m 34 s cold. `make eval-flow`; it runs
  on demand and before a release, never in the default gate. It proves a capability boxer had and
  had never claimed: an MCP server that must live beside the code runs in the sandbox, addressed by
  an ordinary `.mcp.json` entry through `boxer run`.
- **Devcontainers are read.** A repository with `.devcontainer/devcontainer.json` gets its image,
  lifecycle commands, ports, environment, bind mounts and workspace folder without restating any of
  it, and `doctor` says which file each value came from. What needs an image build (`features`,
  `build`/`dockerFile`) or multi-container orchestration (`dockerComposeFile`) is refused by name
  with what to do instead. The file is parsed as the JSON-with-comments it actually is.
- **A core a user interface can sit on.** A sandbox now records which harness, session and agent it
  belongs to, so a listing names "claude-code/4f2a9c" instead of a hash the identity was hashed
  into. `boxer ls --resources` measures what each one costs: allocation from smolvm, resident
  memory and CPU from its process, and disk from its data directory in allocated blocks, the way
  `du` counts, because the disks are sparse and their length is not their cost. `boxer watch`
  streams state changes as line-delimited JSON, so an interface tails one process instead of
  polling. No interface ships; these shapes are in `docs/api.md` and stable from 1.1.
- `boxer doctor` reports the environment's cache key and whether it is cached yet, so "will this
  worktree have to run setup again" is answerable without watching a VM boot.

## [1.0.0] - 2026-09-18

First public release. What is stable, and what breaking it would cost, is in
[docs/release.md](docs/release.md): the command line and its flags, the `--json` shapes, the exit
codes and error contract, the skill and plugin layout, the MCP tool names, `pkg/boxer`, and the
configuration keys.

Evidence for this release, all run on 2026-09-18 on Apple Silicon with smolvm 1.16.1: unit tests
with the race detector and per-package coverage floors on Linux and macOS; lint and vulnerability
scan clean; smoke 53/53; T1 67 pass, 0 fail, 1 skip, run twice with identical per-cell verdicts;
T2 live 42 pass, 0 fail, 14 named skips for $0.19; adherence 80 of 96 live cells across four
models with one harness verdict, the known Grok one. `v1.0.0-rc.1` proved the publishing path:
signed checksums that verify against the release workflow's identity, and all four install routes
exercised against the published release.

### Added
- Storage reclaims itself. Any command that provisions a sandbox starts a background sweep, at
  most once every `reclaim_every` (6 hours by default), deleting sandboxes whose worktree is gone,
  sandboxes idle past `idle_timeout`, and packs nothing references. `auto_reclaim = false` turns
  it off, and `BOXER_NO_RECLAIM=1` does the same for one run.
- `min_free_gb` (5 GB by default): boxer refuses to write a pack when free space is below it and
  pulls the image instead. A cache is worth less than a working disk.
- `boxer gc --all` reclaims every stopped sandbox and every unreferenced pack whatever their age,
  and `gc` now reports how much it freed.
- `boxer doctor` prints the footprint: sandboxes, packs, bytes cached and bytes free, with a
  warning when free space is under the margin.
- `make clean-evals` reclaims the evaluation suite's scratch repositories and pack cache, which
  live outside boxer's state and which nothing reclaimed before.
- The remote, and with it the first proof of the publishing path: `v1.0.0-rc.1` published four
  platform archives, four SBOMs, the plugin, skill and npm tarballs, and a signed `checksums.txt`
  that verifies against the release workflow's OIDC identity.

### Fixed
- A release could not have succeeded: the Homebrew cask named a tap token this repository does not
  have, so the first tag would have failed with every artifact already built. The tap push is now
  skipped when the token is absent.
- A candidate tag published as the latest release, which is what `install.sh` and Homebrew hand to
  anyone asking for the current version. Candidates are prereleases.
- CI's linter could never have run: the configuration declares version 2 while the workflow
  installed version 1 through an action that only installs version 1.
- The toolchain carried five known standard-library vulnerabilities, four of them reachable.

### Added
- `[tasks]` in `boxer.toml` with `boxer tasks` and `boxer run --task <name>`: the repository names
  the commands it wants run, so whether work is sandboxed no longer depends on the intercept list
  matching a shell line the agent composed.
- `boxer brief [--json]`: the resolved brief for a checkout, so published content can stay static
  and still tell an agent what this repository expects. Hooks inject the same text, and the MCP
  server serves it as a resource.
- The skill ships its `scripts/` layer, per the Agent Skills specification: `run`, `task`,
  `status` and `brief` call the installed CLI, so every harness has a deterministic command path
  with no per-harness code.
- An opt-in event stream (`[telemetry]`) with file, stderr and OpenTelemetry sinks, `boxer logs`,
  an event tail on `boxer status --json`, and command lines elided unless asked for. Off by
  default: with no table, boxer writes no log and nothing to the network. Schema in
  `docs/events.md`.
- Apache-2.0 licence, contribution guide, security policy and code of conduct.
- `npm/bin/boxer.js`, the launcher `package.json` had always declared but the repository never
  contained, so packed tarballs installed a command that could not run.
- `scripts/install-routes.sh` and a CI job running it on Linux and macOS: `install.sh` and the npm
  package against a release staged on the machine, each of them also with a tampered checksum that
  must abort.
- Release artifacts: darwin/amd64 binaries, reproducible builds (`-trimpath` and a commit-derived
  timestamp), a syft SBOM per archive, keyless cosign signing of `checksums.txt`, the skill as its
  own tarball, and a Homebrew tap.
- `make release-gate`, the mechanical half of the release gate, enforced on the tag before
  anything is published.
- User-facing documentation: `docs/install.md`, `docs/configure.md`, `docs/integrate.md` and
  `docs/troubleshooting.md`, with `docs/architecture.md` and a `docs/README.md` index.

### Changed
- Published content is static. The package and the skill are byte-identical for every user but for
  the release version, `boxer install` copies them verbatim, and `doctor` reports drift instead of
  editing installed files in place. Configuration reaches the agent at runtime through
  `boxer brief`.
- An unknown top-level table in `boxer.toml` is a warning rather than an error, so a repository
  that adopts a newer boxer's table still loads under an older binary. A misspelled scalar key
  stays fatal.
- `docs/release.md` now states what is stable at 1.0 and what is not, sets a deprecation window of
  one minor release, and gives the release gate as a runnable checklist.
- The README opens with a two-minute quickstart and the five integration levels.

### Fixed
- A truncated pack (an interrupted pack write) made every sandbox on the host fail with an I/O
  error that named neither the pack nor a fix. An empty pack is now rewritten, and a broken one is
  deleted at create time and the image pulled instead.
- `boxer install` reported success having written nothing when a copy or an append failed.

## [0.2.0] - 2026-09-18

### Added
- Worktree timing: `warm_on_session_start`, `worktree.manage`, `boxer install git` (a post-checkout
  hook that warms the sandbox when a worktree is created), and a `doctor` signal report.
- One Agent Plugins 1.0.0 package with per-client views, validated against the vendored schemas.
- MCP lifecycle signals: `initialize` warms the scope, EOF records last use, and a guest path in
  `boxer_run`'s `cwd` maps back to the host worktree.
- An adherence evaluation tier measuring whether a live model follows the injected brief, with a
  two-model rule that separates a harness fault from a model's behaviour.
- Gateway spend tracking and a per-run budget for live evaluations.
- `boxer shim install --shell` writes `boxer-bash`, so any harness with a configurable shell binary
  is sandboxed with no boxer code (verified with OpenHands).
- `boxer down --scope NAME` takes a sandbox down by name from anywhere.
- Per-host harness packs: the first `boxer shell <harness>` pays the install, later worktrees start
  in seconds.

### Changed
- DeepSeek Harness support was rebuilt from the installed product: it has no `.dsh/hooks.json`, and
  integrates through a profile patch, an MCP client entry and the Claude Code hook bridge.
- Grok's missing session-start context is recorded in its dialect, so tool mode there expects one
  denial instead of failing.

### Fixed
- Codex guest installs lacked CA certificates, so TLS to a model provider failed silently.
- A guest install could be packed while broken, because npm exits 0 without optional native
  binaries; installs now keep optional packages and verify the entry point before packing.
- Two boxer processes racing smolvm now wait for the winner instead of failing.

## [0.1.0] - 2026-09-17

### Added
- First working slice: a microVM per git worktree, hooks in eight dialects, an MCP run tool, agent
  skills, project and user installs, `rewrite`/`tool`/`off` modes, and an inside mode where the
  harness itself runs in the guest (`boxer shell`, `boxer acp`).
- Evaluation suite: a scripted model server, a deterministic tier across every harness, and a live
  tier.
