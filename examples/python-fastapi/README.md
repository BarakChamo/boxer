# Python: FastAPI with uv

A runnable FastAPI service installed with uv.

```sh
cp -R examples/python-fastapi /tmp/python-fastapi && cd /tmp/python-fastapi
git init -q && git add -A && git commit -qm init
boxer up
curl -sk "$(boxer url)"      # {"served_from":"a boxer sandbox","host":"…localhost:1355"}
```

Installing uv is `image_setup`: it changes the image, runs once and is cached. Installing the
project's dependencies is `setup`: it lands in this worktree's `.venv`, which a cached image cannot
carry. See [environment](../../site/content/docs/guides/environment.mdx).
