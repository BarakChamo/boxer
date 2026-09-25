# npm

The `boxer-cli` package: one install route among four, for people whose package manager is npm.

```sh
npm i -g boxer-cli
```

It ships no boxer code. `postinstall.js` downloads the release binary matching the package's
version, verifies its sha256 against the release's `checksums.txt`, and unpacks it into
`vendor/`; `bin/boxer.js` execs it. The binary and the checksum come from the same release, so a
tampered asset fails the install rather than running.

No dependencies, by design — a launcher that pulls a tree of its own into every install is a
supply chain around a single binary. `tar` is expected on `PATH`.

Supported: darwin and linux, amd64 and arm64. Anything else gets an error naming `go install`
instead of a broken install.

`BOXER_BASE_URL` overrides the asset directory, which is how `scripts/install-routes.sh` tests
this against a staged release without publishing one.

Publishing stays manual: the release workflow attaches the tarball to the GitHub release and stops
there.
