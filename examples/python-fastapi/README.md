# Python: FastAPI with uv

A runnable FastAPI service on port 8000, installed with uv.

```sh
cp -R examples/python-fastapi /tmp/python-fastapi && cd /tmp/python-fastapi
git init -q && git add -A && git commit -qm init    # boxer keys a sandbox to a git worktree
boxer up
curl -sk "$(boxer url)"      # {"served_from":"a boxer sandbox","host":"…localhost:1355"}
```

It shows the split between `image_setup` and `setup`. Installing uv changes the image, so it is
`image_setup`: it runs once and, on smolvm, is kept in the environment pack for every worktree.
Installing the project's dependencies writes this worktree's `.venv`, which a pack cannot carry, so
it is `setup`. [The environment guide](../../site/content/docs/guides/environment.mdx) has the rule.

Gotchas:

- The allowlist needs both `pypi.org` and `files.pythonhosted.org`; pip and uv fetch the index
  from one and the files from the other.
- The `test` task only imports the app. Replace it with `uv run pytest` once there are tests.
