# site

boxer's documentation site: [Fumadocs](https://fumadocs.dev) on Next.js. It is the single source
for user-facing documentation — the CLI reference, the configuration keys, the JSON shapes and the
support matrix exist here and nowhere else, so that no table has a second copy to drift from.

```sh
make docs-dev     # serve with hot reload on :3000
make docs         # production build
```

## Where things are

| Path | |
| --- | --- |
| `content/docs/` | every page, as MDX. `meta.json` in a directory orders its pages |
| `src/app/docs/` | the docs layout and route |
| `src/app/(home)/` | the landing page |
| `src/lib/source.ts` | the content source adapter |
| `src/app/llms.txt`, `llms-full.txt`, `llms.mdx/` | machine-readable renderings, generated from the same content |

## Writing a page

Pages are organised by what the reader is trying to do: `start/` teaches, `guides/` solves a
problem, `concepts/` explains, `reference/` states facts, `evals/` publishes measurements. A new
page goes in the quadrant matching its purpose and is added to that directory's `meta.json`.

Two rules specific to this project:

- **A support claim comes from an evaluation run**, not from the existence of code. If no cell has
  scored it, it does not go in a support table.
- **A configuration key documented here exists in `internal/config`.** The defaults in
  `reference/configuration.mdx` are the ones `boxer doctor` prints.

The plan for what is documented where, and what is still missing, is
[../docs/documentation-map.md](../docs/documentation-map.md).

CI builds this site and typechecks it on every pull request, because a failed build is a
documentation outage rather than a cosmetic problem.
