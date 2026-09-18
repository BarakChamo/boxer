# Configuring boxer

boxer reads `boxer.toml` from three places and takes the first value it finds: the worktree, then
the repository root, then `~/.config/boxer/`. Unknown keys are an error rather than a shrug, so a
typo is caught immediately. Every scalar can also be set in the environment as `BOXER_<KEY>` —
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

[network]
mode        = "allowlist"       # the registry hosts your image needs are always allowed
allow_hosts = ["registry.npmjs.org"]

[worktree]
manage      = "off"             # detect: a session in the main checkout shares the repository
                                # VM until it moves into a worktree

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

**`setup`** runs once when a VM is created — installing dependencies, usually. The result is
packed, so the next worktree starts from the pack rather than repeating the work.

**`network`** is off by default, in keeping with smolvm. `allowlist` opens named hosts; the
registry hosts your image needs are always allowed, so a package install works without you listing
them.

## Per-harness overrides

`[harness.<name>]` accepts any key above and applies it only when that harness is the one running.
This is how a harness whose hooks can only block gets `mode = "tool"` while everything else stays
transparent. Names are the ones `boxer doctor` prints: `claude-code`, `codex`, `gemini-cli`,
`opencode`, `grok`, `kimi`, `dsh`, `pi`.

## Placement

```toml
integration = "outside"   # default: the harness runs on your machine, boxer sandboxes its commands
integration = "inside"    # the harness itself runs in the VM; there is nothing to hook
```

Both are explained in [integrate.md](integrate.md).

The full specification of every key, including the ones not listed here, is in
[requirements.md](requirements.md).
