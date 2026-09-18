# Troubleshooting

Start with `boxer doctor`. It prints the resolved configuration and where every value came from,
whether smolvm is healthy, what this worktree's scope is, and which harnesses it can see. Nearly
every surprise below is visible in that output. `boxer doctor --json` is the form to paste into an
issue.

The failures here are the ones actually observed while evaluating boxer against real harnesses,
not a list of things that might go wrong.

## The agent is logged in on my machine but logged out in the VM

**Claude Code and Gemini CLI store their login in the macOS Keychain, and the Keychain does not
enter the guest.** Inside mode mounts `~/.claude` and `~/.gemini` at their host paths, so anything
kept in a file travels; a Keychain item does not. The harness starts in the VM and asks you to log
in again.

For Claude Code, mint a token on the host and pass it in:

```sh
claude setup-token                  # prints a token
export CLAUDE_CODE_OAUTH_TOKEN=…    # boxer forwards it into the guest
boxer shell claude
```

Codex is unaffected: its `auth.json` is a file, and it travels. Gemini has the same Keychain
behaviour as Claude on macOS and wants `GEMINI_API_KEY` instead.

## The first `boxer shell <harness>` takes minutes

Expected, once per machine per harness. Inside mode installs the harness inside the guest the
first time you ask for it, which is an npm install over smolvm's networking: anywhere from ten
seconds to about eleven minutes depending on the harness and the day npm is having. boxer then
packs the result, so every later worktree starts from that pack in seconds.

Warm it deliberately rather than discovering it mid-session:

```sh
boxer shell claude -- --version    # pays the install now, packs it, exits
```

Outside mode does not pay this cost at all. A cold VM from an existing host pack starts in about a
second.

## The image pull fails with a rate limit

Unauthenticated Docker Hub pulls are rate limited per IP address, and a machine that has been
creating sandboxes all morning — or a shared network — hits the limit. The symptom is an image
pull failing partway with a quota or "too many requests" message.

Three ways out, in order of effort: wait for the window to reset; set `image` to something you
already have packed locally, so no pull happens; or authenticate to Docker Hub through smolvm so
the higher limit applies. `boxer doctor` prints which image this scope resolves to and why.

## The disk fills up

Packs are large — 130 to 365 MB each — and there is one per image and one per harness per image.
A long evaluation session, or a week of many worktrees, can fill a small disk before anything
reclaims them.

```sh
boxer ls --json          # what exists, and when each was last used
boxer gc --dry-run       # what would be reclaimed, and why
boxer gc                 # reclaim it
boxer down --all         # stop every running sandbox first, if you want everything gone
```

`gc` prunes by `idle_timeout`, so it does nothing to something you used an hour ago. If you are
short of space now, `boxer down --all && boxer gc` is the blunt version. Run `gc` on a schedule if
you create many worktrees.

## Grok denies my first command every time

Grok does not surface session-start context to the model, so in `tool` mode the agent has not been
told about `boxer_run` when it makes its first shell call: the call is denied, the agent reads the
error, and it uses the tool from then on. One denial per session, every session.

Use rewrite mode there, which is boxer's default for Grok:

```toml
[harness.grok]
mode = "rewrite"
```

In rewrite mode nothing is denied — the command is quietly redirected into the sandbox — so the
missing context stops mattering.

## The agent ran a command on my machine anyway

Check `mode` and `enforcement` in `boxer doctor` first: `mode = "off"` or
`enforcement = "audit"` disables enforcement by design, and `audit` records what would have
happened without blocking it.

If enforcement is on, the likely cause is a command not on the `intercept` list, or one the
harness's hook API never showed us. Two fixes, both in [integrate.md](integrate.md): add the
program to `intercept`, or move up a level — shell substitution (`boxer shim install --shell`)
sandboxes the whole shell rather than recognising commands one at a time, and inside mode removes
the question entirely.

## Two boxers, one machine

smolvm is driven by one process at a time. When two boxers race to create the same sandbox, the
loser waits for the winner rather than failing. The evaluation suite takes a host lock at
`~/.local/state/boxer/eval.lock` for the same reason; a run that appears to hang at the start is
waiting for another run to finish.

## A sandbox outlives its session

DSH has no session-end signal, so nothing tells boxer the session is over. The MCP server's EOF
and `boxer gc` are the fallbacks. `boxer down --scope NAME` takes a named sandbox down from
anywhere; `boxer ls` gives you the names.

## An orchestrator gives up while the VM boots

An orchestrator that creates a worktree and launches a harness into it in one step can time out
waiting for a cold sandbox — T3 Code's provider does. Warm the worktree at creation time:

```sh
boxer install git    # a post-checkout hook running `boxer up --detach` in every new worktree
```

`warm_on_session_start = true` does the same thing one step later, at session start, returning
immediately and letting the VM finish booting while the agent reads its prompt.

## Still stuck

Every refusal boxer prints carries `scope`, `worktree`, `cause` and a runnable `fix:` line; the
`fix:` line is the intended next command. If it is wrong or missing, that is a bug worth
reporting, with `boxer doctor --json` attached.
