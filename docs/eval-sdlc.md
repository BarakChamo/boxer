# boxer eval report — tier sdlc (`make eval-sdlc`)

Twelve development lifecycles, three at a time, each in its own linked git worktree with its own
sandbox, its own dev server and its own port. A live agent does the work with the tools a developer
would have: boxer's project layer, the Next.js dev server's MCP tools **running inside the
sandbox**, and `agent-browser` on the host. Nothing is judged by the agent's own account of itself:
the page is read from the host through the forwarded port, and the change has to be in the worktree
afterwards.

The tasks are ordinary work, not puzzles — a page, an API route, a client component, a dynamic
route, server data, metadata, a layout change, a CSS module, an error boundary, installing a
dependency, restarting the dev server, and fixing a deliberately broken import using the
framework's own diagnostics.

**What it establishes.** Sandboxes do not get in the way of development: agents edit, run commands,
read the running app through MCP, look at pages in a browser, restart servers, and install
dependencies, in parallel worktrees, without clashing. Every lifecycle used the MCP server running
in its guest; all but one used a browser.

# boxer eval report — tier sdlc — 2026-09-19T10:27:45+08:00

12 development lifecycles, 3 at a time, each in its own git worktree with its own
sandbox and its own port. Every check is made from the host: the page is read with a real
browser through the forwarded port, and the change has to be in the worktree afterwards.

**passed 11 of 12 · MCP used in 12 · browser used in 11 · $0.3254 · slowest 3m54s**

| Lifecycle | Status | Provision | Agent | Total | MCP | Browser | Spend | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| add-a-page | pass | 31s | 30s | 1m1s | yes | yes | $0.0094 |  |
| api-route | pass | 30s | 12s | 43s | yes | - | $0.0104 |  |
| client-state | pass | 35s | 1m18s | 1m54s | yes | yes | $0.0250 |  |
| fix-a-break | pass | 1m2s | 1m45s | 2m48s | yes | yes | $0.0487 |  |
| layout-change | pass | 31s | 33s | 1m4s | yes | yes | $0.0208 |  |
| dynamic-route | pass | 1m2s | 46s | 1m48s | yes | yes | $0.0218 |  |
| server-data | pass | 30s | 54s | 1m24s | yes | yes | $0.0218 |  |
| metadata | pass | 16s | 57s | 1m14s | yes | yes | $0.0202 |  |
| restart-server | pass | 34s | 3m19s | 3m54s | yes | yes | $0.0692 |  |
| css-module | pass | 31s | 58s | 1m30s | yes | yes | $0.0204 |  |
| error-boundary | fail | 31s | 1m47s | 2m18s | yes | yes | $0.0308 | browser: exit status 1: ✗ Read failed with HTTP 500
 |
| install-a-dep | pass | 31s | 1m10s | 1m41s | yes | yes | $0.0268 |  |

## What twelve lifecycles found

Ordered by how much they would cost a real user.

**1. Several sandboxes exhaust memory before they exhaust anything else.** Four at once, each
running `npm install`, killed one guest outright. boxer reported `exit 137` and nothing more, which
is the out-of-memory killer speaking in its own numbers. It now says so and names the allocation,
because the fix — more `memory`, or fewer sandboxes at once — cannot be guessed from 137. Measured
while four were up: **0.6 to 1.4 GB resident and 425 to 766 MB of disk each**, so four is roughly
4 GB of memory and 2.7 GB of disk. Three at a time was comfortable on a 16 GB machine.

**2. A dev server reached through a forwarded port is cross-origin, and frameworks refuse it.**
Next.js 16 served the HTML and then refused its own dev chunks with 403, so the page rendered in
`curl` and hung in a browser. One agent diagnosed this unaided and spent its remaining turns fixing
it. The fix is one line (`allowedDevOrigins`), the symptom points nowhere near it, and it is now in
[troubleshooting.md](troubleshooting.md) for Next, Vite and Nuxt.

**3. A configuration file in the repository root can break a scaffolder.** `create-next-app`
refuses to scaffold into a directory that already contains `boxer.toml`, and says so in terms that
blame the directory rather than the file. Anything that writes a root-level configuration file has
this problem; worth knowing before telling someone to run a scaffolder in a prepared repository.

**4. Each worktree needs its own port, and nothing arranges that.** `boxer.toml` is committed, so
every worktree inherits the same `network.ports`. Here the runner writes a distinct port per
worktree, but a person doing this by hand would collide on the second one, and the failure would
look like a dev server that will not start. A per-worktree override, or a port range, is the
obvious gap.

**5. Not a boxer problem, but worth recording.** An error boundary legitimately serves HTTP 500
while rendering correctly; a browser tool that refuses non-200 pages will call that a failure.

## What it did not find

No clash between concurrent worktrees: twelve sandboxes, three at a time, each keyed to its own
worktree, with no cross-talk in files, ports, packs or state. No command escaped to the host in any
lifecycle. No sandbox outlived its lifecycle. The environment pack carried the image half of setup
across every worktree while each one installed its own dependencies, which is the split this
release exists to draw.
