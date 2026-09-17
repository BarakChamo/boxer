# Release shape

One repository, one version, four artifacts. Nothing here is a second module or a second build
system; split only when a consumer needs `pkg/boxer` without the CLI's dependency set.

| Artifact | What | Where | Who |
| --- | --- | --- | --- |
| core | `pkg/boxer` Go facade over `internal/{box,vm,scope,config,inside}`; `boxer … --json`; the MCP server | this module | orchestrators, a running-boxes dashboard, anyone shelling out |
| cli | the `boxer` binary | GitHub Releases (darwin/arm64, linux/amd64, linux/arm64), `go install`, `install.sh`, npm wrapper that downloads the release binary | developers, orchestrator hosts |
| plugins | `boxer package all` output | attached to each release as `boxer-plugins-<version>.tar.gz`; `boxer install` writes the project layer from the same templates | harness users |
| evals | `cmd/boxer-eval`, `cmd/fakellm`, `evals/`, `adapters/openhands` | in repo, never released as binaries | maintainers, CI |

## Rules

- **One semver.** Plugins carry the CLI version they were rendered from; `boxer doctor` warns
  when a project-layer install is older than the binary. Pre-1.0 is `0.x`; `pkg/boxer` and
  `boxer acp` are experimental until 1.0.
- **`internal/` stays internal.** `pkg/boxer` is the only importable package and re-exports the
  few types it needs. It contains no logic of its own.
- **JSON is the language-agnostic API.** `ls`, `status`, `doctor`, `down`, `gc --dry-run` take
  `--json`; shapes are in [api.md](api.md). A dashboard needs nothing else.
- **Version comes from the tag.** `-ldflags -X main.Version=<tag>` (goreleaser); `make build`
  stamps `git describe`. The same value is rendered into every plugin manifest and SKILL.md.

## Cutting a release

```sh
make test && make smoke && make eval-t1     # green, report committed
git tag v0.x.y && git push --tags           # release workflow builds binaries, plugins, npm tarball
```

The workflow also attaches the npm tarball (`npm/`, package `boxer-cli`, version set from the
tag); `npm publish` is a manual step until the package name is settled. Locally,
`goreleaser check` validates the config and `goreleaser release --snapshot --clean` builds without
publishing. The rules above are R-REL-1 to R-REL-4 in [requirements.md](requirements.md).
