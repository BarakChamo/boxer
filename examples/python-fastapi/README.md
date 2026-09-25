# Python: FastAPI with uv

```sh
cd examples/python-fastapi
git init -q && git add -A && git commit -qm init
boxer up
curl -sk "$(boxer url)"      # {"served_from":"a boxer sandbox","host":"…localhost:1355"}
```

The split worth copying: installing **uv** is `image_setup` (it changes the image, runs once, and is
cached), installing **the project's dependencies** is `setup` (it lands in this worktree's `.venv`,
which no cached image can carry).
