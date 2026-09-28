# pkg/boxer

The only importable package. Open the sandbox for a working directory, make sure it exists, run
commands in it, take it down, and list what is running on the host.

```go
import "github.com/BarakChamo/boxer/pkg/boxer"

b, _ := boxer.Open(cwd, boxer.Options{Harness: "claude-code"})
b.Ensure(true, false)
code, err := b.Run([]string{"sh", "-c", "npm test"}, boxer.RunOpts{Stdout: os.Stdout})
```

It is a thin facade over `internal/`, with the types aliased and no logic of its own, so it cannot
drift from what the CLI does.

**Experimental until 1.0**, and a promise from 1.0 onwards: [the stability
contract](../../docs/release.md) covers this package's exported functions, types and their fields
alongside the CLI and the JSON output. Until the 1.0 tag, it may change.

Reference: `site/content/docs/reference/go.mdx`. Worked examples and the decisions around them:
`site/content/docs/reference/building-on-boxer.mdx`.
