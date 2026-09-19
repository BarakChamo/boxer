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
dependencies, in parallel worktrees, without clashing.

Across the twelve: **145 tool calls**, of which 99 were shell commands in the sandbox, 19 browser
page reads, and 15 calls to the MCP server running inside the guest. Five lifecycles reached for
that server; the rest did the job with a shell and a browser, which is a fair result — the tools
are there when the work needs them rather than because the harness insists.

An earlier version of this report claimed all twelve used MCP. That was a measurement bug: it
matched the string anywhere in the transcript, including the server list every session prints at
startup. Tool use is now counted from the session's own `tool_use` events.

# boxer eval report — tier sdlc — 2026-09-19T13:17:12+08:00

12 development lifecycles, 3 at a time, each in its own git worktree with its own
sandbox and its own port. Every check is made from the host: the page is read with a real
browser through the forwarded port, and the change has to be in the worktree afterwards.

**passed 12 of 12 · MCP used in 5 · browser used in 10 · $0.3305 · slowest 2m29s**

| Lifecycle | Status | Port | Provision | Agent | Turns | MCP calls | Browsed | Spend |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| add-a-page | pass | 58656 | 1m4s | 26s | 9 | 0 | 1 | $0.0246 |
| api-route | pass | 58655 | 32s | 28s | 4 | 1 | 0 | $0.0090 |
| client-state | pass | 59076 | 34s | 24s | 7 | 1 | 4 | $0.0071 |
| fix-a-break | pass | 58986 | 33s | 20s | 7 | 2 | 1 | $0.0132 |
| layout-change | pass | 59556 | 33s | 36s | 13 | 0 | 2 | $0.0195 |
| dynamic-route | pass | 59462 | 33s | 30s | 5 | 0 | 1 | $0.0192 |
| server-data | pass | 59558 | 33s | 24s | 9 | 0 | 1 | $0.0150 |
| metadata | pass | 59704 | 29s | 20s | 7 | 0 | 0 | $0.0076 |
| restart-server | pass | 59149 | 35s | 1m48s | 28 | 4 | 1 | $0.0741 |
| css-module | pass | 59147 | 36s | 48s | 13 | 0 | 3 | $0.0365 |
| error-boundary | pass | 59327 | 32s | 1m56s | 24 | 7 | 4 | $0.0624 |
| install-a-dep | pass | 58657 | 1m3s | 54s | 19 | 0 | 1 | $0.0422 |

## Ports

Every lifecycle asked for guest port 3000 with `auto`, and each was given its own host port.

No collisions: 12 distinct host ports for 12 lifecycles.

## What each agent did

Read from each session's own transcript, not from what the agent said about itself.

### add-a-page — pass in 1m29s

- **tools**: Bash×7, Read×1, Write×1
- **MCP server in the guest**: not used
- **browser**: 1 page reads, e.g. http://127.0.0.1:58656/about
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/about/
- **host port**: 58656 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-add-a-page.agent.log`

### api-route — pass in 1m1s

- **tools**: Bash×2, Write×1, nextjs_call×1
- **MCP server in the guest**: nextjs_call
- **browser**: not used
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/api/
- **host port**: 58655 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-api-route.agent.log`

### client-state — pass in 59s

- **tools**: Bash×5, Write×1, browser_eval×1
- **MCP server in the guest**: browser_eval
- **browser**: 4 page reads, e.g. http://127.0.0.1:59076/counter
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/counter/
- **host port**: 59076 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-client-state.agent.log`

### fix-a-break — pass in 54s

- **tools**: Bash×2, Read×2, Write×1, nextjs_call×1, nextjs_index×1
- **MCP server in the guest**: nextjs_index, nextjs_call
- **browser**: 1 page reads, e.g. http://127.0.0.1:58986/broken
- **left in the worktree**: app/app/broken/page.tsx, app/package-lock.json, app/package.json, .mcp.json
- **host port**: 58986 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-fix-a-break.agent.log`

### layout-change — pass in 1m8s

- **tools**: Bash×11, Edit×1, Read×1
- **MCP server in the guest**: not used
- **browser**: 2 page reads, e.g. http://127.0.0.1:59556/
- **left in the worktree**: app/app/layout.tsx, app/package-lock.json, app/package.json, .mcp.json
- **host port**: 59556 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-layout-change.agent.log`

### dynamic-route — pass in 1m4s

- **tools**: Bash×4, Write×1
- **MCP server in the guest**: not used
- **browser**: 1 page reads, e.g. http://127.0.0.1:59462/items/42
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/items/
- **host port**: 59462 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-dynamic-route.agent.log`

### server-data — pass in 57s

- **tools**: Bash×5, Read×3, Write×1
- **MCP server in the guest**: not used
- **browser**: 1 page reads, e.g. http://127.0.0.1:59558/data
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/data/
- **host port**: 59558 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-server-data.agent.log`

### metadata — pass in 49s

- **tools**: Bash×5, Read×1, Write×1
- **MCP server in the guest**: not used
- **browser**: not used
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/about/
- **host port**: 59704 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-metadata.agent.log`

### restart-server — pass in 2m23s

- **tools**: Bash×21, nextjs_call×3, Read×1, Skill×1, Write×1, nextjs_index×1
- **MCP server in the guest**: nextjs_index, nextjs_call
- **browser**: 1 page reads, e.g. http://127.0.0.1:59149/restarted
- **restarted the dev server** in the sandbox
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/restarted/
- **host port**: 59149 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-restart-server.agent.log`

### css-module — pass in 1m24s

- **tools**: Bash×8, Read×3, Write×2
- **MCP server in the guest**: not used
- **browser**: 3 page reads, e.g. http://127.0.0.1:59147/styled
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/styled/
- **host port**: 59147 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-css-module.agent.log`

### error-boundary — pass in 2m29s

- **tools**: Bash×13, nextjs_call×5, Read×3, Write×1, nextjs_docs×1, nextjs_index×1
- **MCP server in the guest**: nextjs_docs, nextjs_index, nextjs_call
- **browser**: 4 page reads, e.g. http://127.0.0.1:59327/boom
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/boom/error.tsx
- **host port**: 59327 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-error-boundary.agent.log`

### install-a-dep — pass in 1m58s

- **tools**: Bash×16, Read×1, Skill×1, Write×1
- **MCP server in the guest**: not used
- **browser**: 1 page reads, e.g. http://127.0.0.1:58657/clsx
- **left in the worktree**: app/package-lock.json, app/package.json, .mcp.json, app/app/clsx/
- **host port**: 58657 · **transcript**: `/var/folders/lj/1jp7bc4d3937mqtz18vkd_pm0000gn/T/sdlc-install-a-dep.agent.log`

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
