#!/bin/sh
# Installs the boxer release binary for this OS and architecture into ~/.local/bin.
#   curl -fsSL https://raw.githubusercontent.com/BarakChamo/boxer/main/install.sh | sh
# BOXER_VERSION pins a release (default: latest); BOXER_INSTALL_DIR changes the destination;
# BOXER_BASE_URL points at another asset directory (tests use a local one).
set -eu

REPO=BarakChamo/boxer
VERSION=${BOXER_VERSION:-}
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
  [ -n "$VERSION" ] || { echo "boxer: could not determine the latest release; set BOXER_VERSION" >&2; exit 1; }
fi
VERSION=${VERSION#v}

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac
case "$OS/$ARCH" in
  darwin/arm64|darwin/amd64|linux/amd64|linux/arm64) ;;
  *) echo "boxer: no release binary for $OS/$ARCH; use: go install github.com/$REPO/cmd/boxer@v$VERSION" >&2; exit 1 ;;
esac

FILE="boxer_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE=${BOXER_BASE_URL:-"https://github.com/$REPO/releases/download/v$VERSION"}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL -o "$tmp/$FILE" "$BASE/$FILE"
curl -fsSL -o "$tmp/checksums.txt" "$BASE/checksums.txt"

expected=$(grep " $FILE\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$FILE" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$FILE" | cut -d' ' -f1)
fi
[ -n "$expected" ] && [ "$expected" = "$actual" ] || { echo "boxer: checksum mismatch for $FILE" >&2; exit 1; }

tar -xzf "$tmp/$FILE" -C "$tmp" boxer
DEST=${BOXER_INSTALL_DIR:-"$HOME/.local/bin"}
mkdir -p "$DEST"
install -m 755 "$tmp/boxer" "$DEST/boxer"
echo "boxer $VERSION installed to $DEST/boxer"
case ":$PATH:" in
  *":$DEST:"*) ;;
  *) echo "add it to PATH:  export PATH=\"$DEST:\$PATH\"" ;;
esac
command -v smolvm >/dev/null 2>&1 || echo "smolvm is missing:  curl -sSL https://smolmachines.com/install.sh | bash"
