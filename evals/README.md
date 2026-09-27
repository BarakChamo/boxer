# evals

- `smoke.sh`: boxer alone against a real backend, no model. `BOXER_BACKEND=docker|podman|container`
  runs the same cells on another backend; a cell the backend cannot support is skipped by name,
  from `boxer doctor --json`. Prints `passed N, failed N[, skipped N]`; the last full run is
  recorded in [../docs/status.md](../docs/status.md). Takes the host eval lock. The portless cells
  run when `portless` is on PATH and skip by name when it is not.
- `boxer-eval` (`cmd/boxer-eval`, `make build`): the harness and orchestrator matrix.
  `--tier t1` plays the model with fakellm; `--tier t2` uses real providers with credentials
  from `evals/.env` (gitignored, `KEY=value` lines). One key runs every harness live:
  `AI_GATEWAY_API_KEY` (Vercel AI Gateway; Claude Code, Kimi and pi over Anthropic Messages,
  Codex over Responses, OpenCode, Grok and OpenHands over Chat Completions). Only Gemini CLI needs
  its own `GEMINI_API_KEY`. `BOXER_EVAL_MODEL` overrides the per-harness cheap model, and `BOXER_EVAL_PACE=<seconds>`
  pauses that long before the next live cell after a provider rate-limits one. A missing
  credential skips its cells and names the variable. Run one cell at a time on a shared machine:
  `bin/boxer-eval --tier t2 --cell claude-code/rewrite/plugin`.
- `--tier adherence` scenarios: `brief`, `recovery`, `multistep`, `task`, `prep` (host-side
  `[prep]` through the harness's own path into the sandbox) and `server` (find this worktree's dev
  server with nothing but the brief). `--cell /server` runs one scenario across every harness.
- `--tier matrix`: the Next.js workload at every integration level. `BOXER_EVAL_URLS=1` turns on
  `[urls]` in every sandbox and withholds the address from the prompt, so each agent has to find
  its own server. `--list` lists; it never runs anything.

Scratch: a failed cell keeps its directory as evidence; `boxer-eval` removes any eval scratch older
than three days when it starts, and an interrupted `matrix` or `sdlc` run brings down its own
sandboxes. `make clean-evals` removes everything now.

The old `harness.sh` live script is retired: `boxer-eval --tier t2` covers every harness it did,
with the same oracle as t1 (`internal/eval/oracle.go`).
