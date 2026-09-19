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

# boxer eval report — tier sdlc — 2026-09-19T10:54:16+08:00

12 development lifecycles, 3 at a time, each in its own git worktree with its own
sandbox and its own port. Every check is made from the host: the page is read with a real
browser through the forwarded port, and the change has to be in the worktree afterwards.

**passed 12 of 12 · MCP used in 12 · browser used in 12 · $0.3946 · slowest 6m52s**

| Lifecycle | Status | Provision | Agent | Total | MCP | Browser | Spend | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| add-a-page | pass | 32s | 1m10s | 1m43s | yes | yes | $0.0207 |  |
| api-route | pass | 39s | 13s | 52s | yes | yes | $0.0052 |  |
| client-state | pass | 30s | 18s | 49s | yes | yes | $0.0236 |  |
| fix-a-break | pass | 32s | 1m12s | 1m44s | yes | yes | $0.0221 |  |
| layout-change | pass | 39s | 1m49s | 2m29s | yes | yes | $0.0472 |  |
| dynamic-route | pass | 31s | 25s | 57s | yes | yes | $0.0159 |  |
| server-data | pass | 31s | 46s | 1m17s | yes | yes | $0.0247 |  |
| metadata | pass | 30s | 4m46s | 5m16s | yes | yes | $0.0749 |  |
| restart-server | pass | 31s | 6m20s | 6m52s | yes | yes | $0.0661 |  |
| css-module | pass | 31s | 2m21s | 2m52s | yes | yes | $0.0366 |  |
| error-boundary | pass | 31s | 1m15s | 1m47s | yes | yes | $0.0186 |  |
| install-a-dep | pass | 40s | 1m6s | 1m46s | yes | yes | $0.0390 |  |

## What twelve lifecycles found

Twenty-eight lifecycles were run in all, across four passes. The first found five things; the run
above is the sweep after all of them were fixed. Ordered by how much they would cost a real user.

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

**4. Each worktree needed its own port and nothing arranged that — fixed.** `boxer.toml` is
committed, so every worktree asked for the same host port, and the second to start failed with a
message about a busy address rather than about worktrees. `network.ports` now takes `"auto:3000"`:
boxer asks the operating system for a free host port and `boxer status` prints where it landed.
Two repositories with identical configuration were given 55426 and 55465.

**5. Not a boxer problem, but worth recording.** An error boundary legitimately serves HTTP 500
while rendering correctly, and it renders on the client, so neither a browser tool that refuses
non-200 pages nor a raw fetch of the HTML sees the result. Navigating first and reading afterwards
does, which is also what a person does.

## What it did not find

No clash between concurrent worktrees: twelve sandboxes, three at a time, each keyed to its own
worktree, with no cross-talk in files, ports, packs or state. No command escaped to the host in any
lifecycle. No sandbox outlived its lifecycle. The environment pack carried the image half of setup
across every worktree while each one installed its own dependencies, which is the split this
release exists to draw.
