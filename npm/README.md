# boxer-cli

The npm launcher for boxer. It installs the `boxer` command by downloading the release binary for
your platform.

```sh
npm i -g https://github.com/BarakChamo/boxer/releases/download/v1.1.0/boxer-cli-1.1.0.tgz
boxer version
```

`boxer-cli` is not published to the npm registry, so `npm i -g boxer-cli` does not work. Install
the tarball attached to the [GitHub release](https://github.com/BarakChamo/boxer/releases)
instead. The other install routes are `install.sh` and `go install`:

```sh
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh
go install github.com/BarakChamo/boxer/cmd/boxer@v1.1.0
```

## What the package does

The package ships no boxer code. `postinstall.js` downloads `boxer_<version>_<os>_<arch>.tar.gz`
from the release that matches the package version, checks its sha256 against that release's
`checksums.txt`, and unpacks `boxer` into `vendor/`. `bin/boxer.js` runs it and passes through
arguments, standard streams and the exit code. A checksum mismatch fails the install.

If npm skipped the install script (`--ignore-scripts`), `bin/boxer.js` runs `postinstall.js` on
first use, with the same checksum check.

The package has no dependencies. It needs Node 18 or later and `tar` on `PATH`.

## Platforms

darwin and linux, on amd64 and arm64. On any other platform the install fails with the
`go install` line to use instead.

## Testing against a staged release

`BOXER_BASE_URL` replaces the release download directory. `scripts/install-routes.sh` uses it to
install this package against a release staged on the local machine.

## Publishing

The release workflow runs `npm pack` and attaches `boxer-cli-<version>.tgz` to the GitHub release.
It does not publish to the registry.
