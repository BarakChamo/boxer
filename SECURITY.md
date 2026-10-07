# Security

## Reporting a vulnerability

Please report privately through GitHub's [private vulnerability
reporting](https://github.com/BarakChamo/boxer/security/advisories/new) rather than a public issue.
Expect an acknowledgement within three working days and a fix or a plan within fourteen.

## What boxer is, and is not

boxer runs an agent's shell commands inside a sandbox that mounts the git worktree and only what the
configuration adds: `mounts`, the package caches (read-only), named `volumes`, and in inside mode
the harness config directories. That is a real boundary: a command in the guest cannot read your
home directory, your keys, or any other path.

How strong the boundary is depends on the backend, and `boxer doctor` prints which one you have:

| backend | boundary | egress |
| --- | --- | --- |
| `smolvm` (default) | a kernel per sandbox | allowlist by default |
| Apple `container` | a kernel per sandbox | on or off — no allowlist |
| `docker`, `podman` | **one kernel shared by every sandbox** on the machine | on or off — no allowlist |

A backend that cannot enforce the allowlist **refuses** `network.mode = "allowlist"` rather than
running with a network boxer said it would deny. A container escape on a shared-kernel backend
reaches every other sandbox on that machine; on a kernel-per-sandbox backend it reaches one guest.
If containment is the reason you use boxer, keep the default backend.

Forwarded ports (`network.ports`) and the portless proxy behind `[urls]` bind `127.0.0.1` on every
backend, so a sandbox's servers are reachable from this machine and not from the local network. A
port spec that names an address (`"0.0.0.0:3000:3000"`) is published where it says.

It is **not** a defence against a malicious agent that can choose what to run on the host. Three
paths deliberately stay on the host:

- `passthrough` programs (`git`, `gh`, `ssh`, `boxer` and `smolvm`) run unsandboxed by design.
- `mode = "off"` and `enforcement = "audit"` do not sandbox anything.
- In `rewrite` mode, boxer rewrites commands it recognises. A command it does not recognise runs on
  the host. PATH shims and `tool` mode narrow that gap rather than closing it: a shim loses to a
  login shell, and `tool` mode depends on the agent taking the tool it is left with.
  `enforcement = "both"` is the default for this reason; the shims work once `boxer shim install`
  has written them and their directory is on the harness's `PATH`.

If your threat model is a hostile agent rather than a careless one, run the harness **inside** the
guest (`integration = "inside"`, `boxer shell <harness>`), where there is no host shell to reach.

## The trust boundary around content

A skill, a plugin, or a `boxer.toml` in a repository is executable input. `setup` commands and
`[tasks]` run in the guest; hooks and MCP servers named in a harness's configuration run **on the
host**. Treat a repository's boxer configuration with the same suspicion as its `Makefile`. boxer
never fetches configuration or content from the network.

## Secrets

`secrets` and `env_passthrough` are resolved on the host at run time and passed to the guest by
*name*, through smolvm's `--secret-env`: boxer never puts the value in an argument vector, where
`ps` would show it to every other process on the machine. They are not stored in VM metadata, in
an environment pack, or in any configuration file boxer writes. `env_passthrough` is an explicit
allowlist, empty but for `CI` by default.

One thing boxer does write: after every command it records that command, its exit code and a
bounded tail of its output to `$XDG_STATE_HOME/boxer/runs/<scope>.json`, mode 0600, so a failure
can be explained and replayed. It never leaves the machine and nothing reads it to make a
decision; delete it whenever you like.

Telemetry is off unless you turn it on. With `[telemetry] enabled = true`, events record scope
names, harness names, durations and outcomes; command lines are elided unless you also set
`record_commands = true`, and nothing leaves the machine unless you set an `endpoint` — which only
a binary built with `-tags otel` can even use, because the default build contains no exporter.
`BOXER_TRACE=<path>` also turns the file sink on, whatever the configuration says; it is how the
evaluation suite records hook traffic, and it writes to that path and nowhere else. The schema and
the redaction rules are in [docs/events.md](https://github.com/BarakChamo/boxer/blob/main/docs/events.md).

## Supported versions

Until 1.0, only the latest release. From 1.0, the latest minor of the current major.
