# Installing on the host with [prep]

The slow part of a first start is installing dependencies through the guest's mount. `[prep]` runs
that install on the host instead — devcontainer's `initializeCommand` — with the guest's platform
passed in, so the binaries npm fetches are the Linux ones.

```sh
boxer up
boxer run -- node -e "console.log(require.resolve('@next/swc-linux-arm64-musl'))"   # proves it
```

It is opt-in for a reason: it is only correct for dependencies that download a prebuilt binary.
Read the comments in `boxer.toml` before using it on a project with native modules, and run
`boxer doctor`, which lists the packages it would get wrong.
