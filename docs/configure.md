# Configuring boxer

boxer reads `boxer.toml` from three places and takes the first value it finds: the worktree, then
the repository root, then `~/.config/boxer/`. An unknown key is an error rather than a shrug, so a
typo is caught immediately; an unknown top-level table is only a warning, so a repository that
adopts a newer boxer's table still loads under an older binary. Every scalar can also be set in the environment as `BOXER_<KEY>` —
`BOXER_MODE=off`, `BOXER_ISOLATION=repo` — which is how a one-off run or an orchestrator overrides
a repository's choice.

`boxer doctor` prints the resolved value of every key and the file or variable it came from. When
something behaves unexpectedly, that output is the answer.

## A complete file

```toml
isolation   = "worktree"        # repo | worktree | session | subagent
mode        = "rewrite"         # rewrite | tool | off
enforcement = "both"            # hook | shim | both | audit
intercept   = ["npm", "bun", "node", "python", "go", "make"]
passthrough = ["git", "gh", "ssh", "boxer"]
image       = ""                # default: detected from the lockfile, else debian:bookworm-slim
setup       = ["bun install"]   # run once per VM, inside the guest
warm_on_session_start = false   # true: session start launches a detached `boxer up` and returns

[tasks]                         # named commands: `boxer run --task test`
test  = "bun test"
build = "bun run build"

[network]
mode        = "allowlist"       # the registry hosts your image needs are always allowed
allow_hosts = ["registry.npmjs.org"]

[worktree]
manage      = "off"             # detect: a session in the main checkout shares the repository
                                # VM until it moves into a worktree

idle_timeout = "2h"             # gc reclaims a sandbox unused for this long; "never" disables it
auto_reclaim = true             # an ordinary command sweeps in the background, at most every…
reclaim_every = "6h"
min_free_gb  = 5                # below this, boxer pulls images rather than caching them

[telemetry]
enabled     = false             # the event stream, off by default
sink        = "none"            # none | file | stderr | otel (otel needs a -tags otel build)
path        = ""                # file sink; default $XDG_STATE_HOME/boxer/events.jsonl
record_commands = false         # true: command lines appear in events verbatim
endpoint    = ""                # otel sink only; nothing leaves the machine without it

[harness.gemini-cli]            # any key above, for one harness only
mode        = "tool"
```

## The keys that matter most

**`isolation`** decides how many sandboxes you get. `worktree` — the default — names one VM per
git worktree, so two branches checked out side by side cannot see each other's build output.
`repo` shares one VM across every worktree of a repository, which is faster and less isolated.
`session` and `subagent` go narrower and need the harness to supply an identifier; when it does
not, `on_missing_id` decides whether boxer degrades one level or refuses.

**`mode`** decides how commands reach the sandbox. `rewrite` is transparent: the agent types
`npm test` and boxer quietly turns it into `boxer run -c 'npm test'`. `tool` denies the shell and
leaves the `boxer_run` MCP tool as the only way to run anything, which is the right setting for a
harness that can block a command but cannot rewrite one. `off` disables boxer for this scope
without uninstalling it.

**`enforcement`** decides which mechanisms are active: `hook` (the harness's own hook API),
`shim` (programs on PATH that are really boxer), `both`, or `audit`, which records what would have
happened and blocks nothing.

**`intercept` and `passthrough`** are the lists that decide which programs are sandboxed and which
are deliberately left on the host. `git`, `gh` and `ssh` are on the host by default because they
need your credentials and your real repository.

**`image`** is any OCI reference — `node:24-bookworm`, `ghcr.io/you/dev:latest`, a private
registry — and is detected from your lockfile when you leave it empty. It also takes a local
image: a `docker save` archive (`image = "./dev.tar"`), an extracted rootfs directory, or `-` for
stdin. That is the path for a repository that builds its own image: build it with your own tooling,
save it, and point boxer at the file. Nothing is pulled, so no registry host is opened.

**`setup`** runs once when a VM is created — installing dependencies, usually — and the result is
cached as an *environment pack*: the image with your setup already applied. The next worktree of
the same repository starts from that pack and runs no setup at all, which takes a second rather
than however long `bun install` takes.

The pack is keyed on the image plus the setup commands, like a Docker layer: change a setup line
and the key changes, the old pack is no longer used, and `boxer gc` reclaims it. `boxer doctor`
prints the key and whether it is cached yet.

One thing to know: snapshotting stops the VM for a moment, and the guest's `/tmp` is memory-backed,
so anything `setup` writes there is gone afterwards. That was already true across any restart.
Write to the worktree or somewhere durable like `/var/lib`.

**`tasks`** is how a repository names the commands it actually wants run: `test = "bun test"` makes
`boxer run --task test` work, `boxer tasks` lists them, and an unknown name is refused with the
real ones. This is the deterministic path, because the agent invokes a name the repository
declared instead of composing a shell line the intercept list may or may not catch. The skill boxer
ships calls tasks first, and `boxer brief` tells the agent which ones exist.

**`telemetry`** is off by default: with no table, boxer writes no log and nothing to the network.
`enabled = true` with `sink = "file"` writes one JSON event per line, which `boxer logs` reads back
and `boxer status --json` carries the tail of. Command lines are elided unless `record_commands`
says otherwise. The schema and the redaction rules are in [events.md](events.md).

**`idle_timeout`, `auto_reclaim`, `reclaim_every` and `min_free_gb`** are what keep a machine from
filling up. A sandbox's data directory is about half a gigabyte and a cached image pack is 130 to
365 MB, so storage is the cost users actually notice. By default any `boxer` command that
provisions a sandbox also starts a background sweep, at most once every `reclaim_every`, which
deletes sandboxes whose worktree is gone, sandboxes idle past `idle_timeout`, and packs nothing
references. `auto_reclaim = false` turns that off and leaves `boxer gc` to you. Separately, boxer
refuses to write a pack when free space is below `min_free_gb` and pulls the image instead: a
cache is worth less than a working disk. `boxer doctor` prints the current footprint, and
`boxer gc --all` reclaims every pack and every stopped sandbox regardless of age.

**`network`** is off by default, in keeping with smolvm. `allowlist` opens named hosts; the
registry hosts your image needs are always allowed, so a package install works without you listing
them.

## Per-harness overrides

`[harness.<name>]` accepts any key above and applies it only when that harness is the one running.
This is how a harness whose hooks can only block gets `mode = "tool"` while everything else stays
transparent. Names are the ones `boxer doctor` prints: `claude-code`, `codex`, `gemini-cli`,
`opencode`, `grok`, `kimi`, `dsh`, `pi`, `copilot`.

## Placement

```toml
integration = "outside"   # default: the harness runs on your machine, boxer sandboxes its commands
integration = "inside"    # the harness itself runs in the VM; there is nothing to hook
```

Both are explained in [integrate.md](integrate.md).

The full specification of every key, including the ones not listed here, is in
[requirements.md](requirements.md).
