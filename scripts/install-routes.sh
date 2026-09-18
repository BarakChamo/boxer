#!/usr/bin/env bash
# Exercises the two download-based install routes — install.sh and the npm package — against a
# release staged on this machine and served over HTTP, so no tag, remote or network is needed.
# Each route is checked twice: once with a correct checksums.txt, once with a tampered one, which
# must abort. Usage: scripts/install-routes.sh [version]
set -euo pipefail

version=${1:-9.9.9}
root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'kill "${server:-}" 2>/dev/null || true; rm -rf "$work"' EXIT

os=$(go env GOOS)
arch=$(go env GOARCH)
stage="$work/stage"
mkdir -p "$stage"

echo "== staging boxer $version for $os/$arch"
go build -ldflags "-X main.Version=$version" -o "$work/boxer" "$root/cmd/boxer"
file="boxer_${version}_${os}_${arch}.tar.gz"
tar -czf "$stage/$file" -C "$work" boxer
( cd "$stage" && { command -v sha256sum >/dev/null && sha256sum "$file" || shasum -a 256 "$file"; } > checksums.txt )
cp "$stage/checksums.txt" "$work/checksums.good"

# A tampered checksums.txt: same shape, a digest that is not the archive's.
sed 's/^[0-9a-f]\{64\}/'"$(printf '0%.0s' $(seq 64))"'/' "$work/checksums.good" > "$work/checksums.bad"

python3 -m http.server --directory "$stage" 8731 >/dev/null 2>&1 &
server=$!
base="http://127.0.0.1:8731"
for _ in $(seq 50); do curl -fsS "$base/checksums.txt" >/dev/null 2>&1 && break; sleep 0.2; done

echo "== install.sh, good checksum"
dest="$work/bin"
BOXER_VERSION="$version" BOXER_BASE_URL="$base" BOXER_INSTALL_DIR="$dest" sh "$root/install.sh"
"$dest/boxer" --version | grep -q "$version" || { echo "install.sh: wrong version installed"; exit 1; }

echo "== install.sh, tampered checksum must abort"
cp "$work/checksums.bad" "$stage/checksums.txt"
rm -f "$dest/boxer"
if BOXER_VERSION="$version" BOXER_BASE_URL="$base" BOXER_INSTALL_DIR="$dest" sh "$root/install.sh" 2>"$work/err"; then
  echo "install.sh accepted a tampered checksum"; exit 1
fi
grep -q "checksum mismatch" "$work/err" || { echo "install.sh failed for the wrong reason:"; cat "$work/err"; exit 1; }
[ ! -e "$dest/boxer" ] || { echo "install.sh installed a binary despite the mismatch"; exit 1; }
cp "$work/checksums.good" "$stage/checksums.txt"

echo "== npm package, good checksum"
pkg="$work/npm"
cp -R "$root/npm" "$pkg"
rm -rf "$pkg/vendor" "$pkg/node_modules"
( cd "$pkg" && npm version "$version" --no-git-tag-version >/dev/null && npm pack --silent >/dev/null )
tgz=$(ls "$pkg"/boxer-cli-*.tgz)
prefix="$work/npmroot"
mkdir -p "$prefix"
( cd "$prefix" && BOXER_BASE_URL="$base" npm install --no-audit --no-fund "$tgz" >/dev/null )
out=$("$prefix/node_modules/.bin/boxer" --version)
echo "$out" | grep -q "$version" || { echo "npm launcher printed: $out"; exit 1; }
# The launcher must forward the exit code, not swallow it.
set +e
"$prefix/node_modules/.bin/boxer" definitely-not-a-command >/dev/null 2>&1
code=$?
set -e
[ "$code" -ne 0 ] || { echo "npm launcher did not forward a non-zero exit code"; exit 1; }

echo "== npm package, tampered checksum must abort"
cp "$work/checksums.bad" "$stage/checksums.txt"
rm -rf "$prefix/node_modules"
if ( cd "$prefix" && BOXER_BASE_URL="$base" npm install --no-audit --no-fund "$tgz" >"$work/nerr" 2>&1 ); then
  echo "npm postinstall accepted a tampered checksum"; exit 1
fi
grep -q "checksum mismatch" "$work/nerr" || { echo "npm install failed for the wrong reason:"; cat "$work/nerr"; exit 1; }

echo
echo "All install routes pass, and both reject a tampered checksum."
