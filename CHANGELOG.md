# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Apache-2.0 licence, contribution guide, security policy and code of conduct.

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
