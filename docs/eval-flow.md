# boxer eval report — tier flow (`make eval-flow`)

Two cells, both against a real microVM.

**`flow`** is the mechanics, and needs no model: a pinned Next.js 16 app is scaffolded and
installed inside the sandbox, served by `start`, waited for by `ready`, reached from the host
through a forwarded port, asked for its tool list over MCP by `next-devtools-mcp` running *in the
guest*, then restarted.

**`flow-agent`** is the same environment with a live agent in it. Claude Code is given an ordinary
`.mcp.json` entry whose command is `boxer run -- next-devtools-mcp`, and asked for the app's
routes. What the transcript shows, and what the oracle checks:

- the harness reports `"mcp_servers":[{"name":"next-devtools","status":"connected"}]` and lists
  `mcp__next-devtools__nextjs_index`, `nextjs_call`, `nextjs_docs` and `browser_eval` alongside its
  own tools, with boxer's skill loaded in the same session;
- the agent calls those tools — eleven calls in the run below — finds the dev server on its port,
  and asks it for `get_routes`;
- it answers with the scaffold's real routes, `/` and `/favicon.ico`;
- and it reads boxer's injected brief mid-session, correcting itself from a host path to
  `/workspace`.

That is the claim the tier exists to make: **an MCP server that must live beside the code runs in
the sandbox, and the harness cannot tell the difference.**

The tier is slow and network-heavy, so it runs on demand and before a release, never in the default
gate. A 191 MB environment pack is what makes a repeat run a minute rather than four.

# boxer eval report — tier flow — 2026-09-19T08:53:17+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| flow/rewrite/project/worktree/flow | pass | 40.525s |  |
| flow/rewrite/project/worktree/flow-agent | pass | 1m40.406s | $0.0117; |

**passed 2 · failed 0 · skipped 0**

**gateway spend this run: $0.0117** (per-cell figures are in the notes; BOXER_EVAL_BUDGET_USD caps a run)
