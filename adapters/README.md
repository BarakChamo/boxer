# adapters

Integration for tools that are not harnesses. A harness is a row in boxer's own tables (see
[../docs/adding-a-harness.md](../docs/adding-a-harness.md)); what lives here is configuration for
a third-party tool that has its own plugin format.

| Directory | Tool | Shape |
| --- | --- | --- |
| [`openhands/`](openhands) | OpenHands | a shell to point the terminal tool at, plus a microagent |
| [`herdr/`](herdr) | herdr | a `herdr-plugin.toml`, linked with `herdr plugin link` |

Both integrate at a level boxer already supports — OpenHands through shell substitution, herdr
through inside mode — so neither needs code. Each file carries a comment naming the tool version
its schema was verified against, because these formats change and a silently rejected plugin looks
exactly like one that is not doing anything.

User-facing setup for both: `site/content/docs/orchestrators/`. The research behind each
one, including what was tried and rejected: [../docs/orchestrators.md](../docs/orchestrators.md).
