# evals

- `smoke.sh`: boxer alone against real smolvm, no model (44 checks). Takes the host eval lock.
- `boxer-eval` (`cmd/boxer-eval`, `make build`): the harness and orchestrator matrix.
  `--tier t1` plays the model with fakellm; `--tier t2` uses real providers with credentials
  from `evals/.env` (gitignored, `KEY=value` lines). One key runs every harness live:
  `AI_GATEWAY_API_KEY` (Vercel AI Gateway; Claude Code, Kimi and pi over Anthropic Messages,
  Codex over Responses, OpenCode, Grok and OpenHands over Chat Completions). Only Gemini CLI needs
  its own `GEMINI_API_KEY`. `BOXER_EVAL_MODEL` overrides the per-harness cheap model. A missing credential skips its cells and names the
  variable. Run one cell at a time on a shared machine: `bin/boxer-eval --tier t2 --cell claude-code/rewrite/plugin`.

The old `harness.sh` live script is retired: `boxer-eval --tier t2` covers every harness it did,
with the same oracle as t1 (`internal/eval/oracle.go`).
