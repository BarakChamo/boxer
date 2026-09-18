# Installing boxer

boxer is a single Go binary. It needs [smolvm](https://smolmachines.com), which is what actually
runs the microVMs, and a git repository to work in. macOS on Apple Silicon and Linux on x86-64 or
arm64 are supported; Windows is not.

## smolvm first

```sh
curl -sSL https://smolmachines.com/install.sh | bash
smolvm version
```

Everything below assumes it is on your PATH. boxer keeps no state of its own: the VMs, their
images and the packs are smolvm's.

## Four routes to the binary

Pick one. They all produce the same `boxer`.

```sh
# 1. The install script: downloads the release for your platform into ~/.local/bin, after
#    checking its sha256 against the release's checksums.txt.
curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh

# 2. Homebrew, on macOS.
brew install BarakChamo/tap/boxer

# 3. npm, if node is what you already have. The package downloads the same release binary.
npm i -g boxer-cli

# 4. From source, which needs a Go toolchain.
go install github.com/BarakChamo/boxer/cmd/boxer@latest
```

`BOXER_VERSION=v1.2.3` pins the install script to a release; `BOXER_INSTALL_DIR` changes where it
puts the binary. If `~/.local/bin` is not on your PATH the script tells you so.

## Check the install

```sh
boxer doctor
```

`doctor` is the first thing to run and the first thing to paste into a bug report. It prints every
resolved configuration value and where it came from, whether smolvm is present and healthy, which
harnesses it can see, what this worktree's scope is, and what would happen if an agent ran a
command here.

## Verifying a download

Each release carries `checksums.txt`, a signature made with cosign keyless, and an SBOM per
archive. The install script and the npm package verify the checksum for you and refuse to install
on a mismatch. To check the signature yourself, see
[release.md](release.md#reproducibility-and-provenance).

## Upgrading and removing

Re-run the same route. `boxer doctor` warns when a repository has an installed plugin or skill
layer older than the binary; `boxer install <harness>` rewrites it.

To remove boxer: delete the binary, run `boxer down --all` first if any sandbox is still running,
and delete the files `boxer install` wrote into your repository (`.claude/`, `.codex/`,
`.gemini/`, `.opencode/`, `.grok/` entries — they are merged, so remove boxer's entries rather
than the files).
