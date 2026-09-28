# Installing on the host with [prep]

`[prep]` runs `npm ci` on the host, like devcontainer's `initializeCommand`, while the sandbox
boots. boxer passes the guest's platform in `$BOXER_TARGET_FLAGS`, so npm fetches the Linux
binaries. Drop `boxer.toml` into a Next.js project with a `package-lock.json`.

```sh
boxer up
# on Apple Silicon, the guest-platform binary was installed:
boxer run -- node -e "console.log(require.resolve('@next/swc-linux-arm64-musl'))"
```

It shows `[prep]`, `prep.target`, and the host package cache mounted read-only.

Gotchas:

- It is correct only for packages that download a prebuilt binary. A package that compiles at
  install time (node-gyp) builds for the host and fails in the guest with `invalid ELF header`.
  `boxer up` and `boxer doctor` warn about the common ones in `package-lock.json`. For such a
  project, install with `setup = ["npm ci"]` instead.
- `prep` runs once per worktree, and again only when its commands or target change.
