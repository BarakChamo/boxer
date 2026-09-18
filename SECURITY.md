# Security

## Reporting a vulnerability

Please report privately through GitHub's [private vulnerability
reporting](https://github.com/BarakChamo/boxer/security/advisories/new) rather than a public issue.
Expect an acknowledgement within three working days and a fix or a plan within fourteen.

## What boxer is, and is not

boxer runs an agent's shell commands inside a smolvm microVM whose only mount is the git worktree.
That is a real boundary: a command in the guest cannot read your home directory, your keys, or any
path outside the worktree, and the network is an allowlist by default.

It is **not** a defence against a malicious agent that can choose what to run on the host. Three
paths deliberately stay on the host:

- `passthrough` programs (`git`, `gh`, `ssh`, `boxer` itself) run unsandboxed by design.
- `mode = "off"` and `enforcement = "audit"` do not sandbox anything.
- In `rewrite` mode, boxer rewrites commands it recognises. A command it does not recognise runs on
  the host. PATH shims and `tool` mode close that gap; `enforcement = "both"` is the default for
  this reason.

If your threat model is a hostile agent rather than a careless one, run the harness **inside** the
guest (`integration = "inside"`, `boxer shell <harness>`), where there is no host shell to reach.

## The trust boundary around content

A skill, a plugin, or a `boxer.toml` in a repository is executable input. `setup` commands and
`[tasks]` run in the guest; hooks and MCP servers named in a harness's configuration run **on the
host**. Treat a repository's boxer configuration with the same suspicion as its `Makefile`. boxer
never fetches configuration or content from the network.

## Secrets

`secrets` are resolved on the host at run time and passed to the guest process; they are not stored
in VM metadata or in any configuration file boxer writes. `env_passthrough` is an explicit
allowlist, empty but for `CI` by default.

Telemetry is off unless you turn it on. With `[telemetry] enabled = true`, events record scope
names, harness names, durations and outcomes; command lines are elided unless you also set
`record_commands = true`, and nothing leaves the machine unless you set an `endpoint` — which only
a binary built with `-tags otel` can even use, because the default build contains no exporter.
`BOXER_TRACE=<path>` also turns the file sink on, whatever the configuration says; it is how the
evaluation suite records hook traffic, and it writes to that path and nowhere else. The schema and
the redaction rules are in [docs/events.md](docs/events.md).

## Supported versions

Until 1.0, only the latest release. From 1.0, the latest minor of the current major.
