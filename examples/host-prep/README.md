# Installing on the host with [prep]

`[prep]` runs `npm ci` on the host (like devcontainer's `initializeCommand`) instead of through the
guest's mount, with the guest's platform passed in `$BOXER_TARGET_FLAGS` so npm fetches the Linux
binaries. Drop `boxer.toml` into a Next.js project with a lockfile.

```sh
boxer up
# on Apple Silicon: the guest-platform binary was installed
boxer run -- node -e "console.log(require.resolve('@next/swc-linux-arm64-musl'))"
```

Only correct for packages that download a prebuilt binary. A package that compiles at install time
(node-gyp) builds for the host and fails in the guest; `boxer doctor` lists any in your lockfile.
For such a project, install in the guest with `setup` instead.
