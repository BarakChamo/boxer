# Contributing to boxer

boxer runs a coding agent's shell commands in a smolvm microVM keyed to the git worktree. The rule
that shapes every change: **the core knows nothing about any harness.**

## The one architectural rule

`internal/{box,vm,scope,config,decide,shim}` and `pkg/boxer` must not contain a harness name.
Harness facts are rows in a table: the hook dialect table (`internal/hook`), the inside-mode
harness table (`internal/inside`), or a bundle template (`internal/bundle/templates`).
`TestCoreNamesNoHarness` parses those packages and fails if a name appears, so adding a harness
cannot grow the core.

Adding a harness therefore means: one dialect row, one install case, one template namespace, and
one eval driver. If your change needs more than that, the design is wrong; open an issue first.

## Published content is static

Anything under `skill/` or `plugin/` is published to users verbatim. It must never be rendered
from anyone's configuration. Scripts inside the skill call the installed `boxer` binary, which
resolves `boxer.toml` at run time. `TestPackageIsConfigIndependent` proves it.

## Getting set up

```sh
curl -sSL https://smolmachines.com/install.sh | bash   # smolvm, macOS arm64 or Linux
make build
make test                                              # unit tests, race, coverage; fake smolvm
make smoke                                             # real smolvm, no model
```

Unit tests use a fake smolvm (`internal/vmtest`) and need only `git` and a POSIX shell. Nothing in
`go test ./...` touches the network or a real VM.

## Evaluations

boxer's claims are evaluated, not asserted. Three tiers:

| Tier | What it proves | Cost |
| --- | --- | --- |
| `make smoke` | every configuration path against a real microVM, no model | minutes |
| `make eval-t1` | every harness CLI against a scripted model | ~15 min |
| `make eval-t2` | the same cells against live models | a few cents |

One process may drive smolvm at a time; the runner takes a host lock. Live tiers read
`AI_GATEWAY_API_KEY` from a gitignored `.env`. A pull request that changes behaviour should say
which cells were run and paste the report line.

## Style

- Plain prose in comments, commits and docs. Explain why, not what.
- Errors an agent will read are `boxer: <reason>` with `scope`, `worktree`, `cause` and a runnable
  `fix:` line. Every refusal must tell the reader the next command to type.
- Smallest diff that is correct. No abstraction with one implementation.
- Every non-trivial change leaves a test. Real bugs get a regression test that fails without the fix.

## Commits and pull requests

Conventional prose subject lines ("Wait for a concurrent creator when smolvm says the machine
already exists"), a body that explains the why, and the eval evidence where behaviour changed.
CI runs vet, lint, race, coverage, package and schema validation on Linux and macOS.

## Security

Please do not open a public issue for a vulnerability. See [SECURITY.md](SECURITY.md).
