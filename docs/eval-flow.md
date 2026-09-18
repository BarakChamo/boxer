# boxer eval report — tier flow (`make eval-flow`)

One real development session rather than one command: a pinned Next.js app scaffolded and installed
inside the sandbox, served by `start`, waited for by `ready`, reached from the host through a
forwarded port, asked for its tools over MCP by `next-devtools-mcp` running *in the guest* and
addressed through `boxer run`, then restarted to prove the environment pack made the install
unnecessary.

The tier is slow and network-heavy, so it runs on demand and before a release, never in the default
gate. A 200 MB environment pack is what makes a repeat run a minute rather than four.

# boxer eval report — tier flow — 2026-09-19T00:40:57+08:00

| Cell | Status | Time | Notes |
| --- | --- | --- | --- |
| flow/rewrite/project/worktree/flow | pass | 54.521s |  |

**passed 1 · failed 0 · skipped 0**
