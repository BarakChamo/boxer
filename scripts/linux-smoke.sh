#!/usr/bin/env bash
# Run evals/smoke.sh on Linux from a Mac, in a throwaway Lima VM (Ubuntu LTS, vz, nested
# virtualization), with each backend installed natively:
#
#   scripts/linux-smoke.sh [smolvm] [docker] [podman]     default: all three, one at a time
#
# smolvm needs /dev/kvm in the VM, so nested virtualization: Apple M3 or later, macOS 15 or later.
# BOXER_SMOKE_RECORD=<file> appends each run's JSON line to <file> on the Mac, as smoke.sh does.
# The VM is created for the run and deleted after it, so nothing is left on the Mac but the record.
# BOXER_LIMA_KEEP=1 keeps it for debugging (limactl shell boxer-smoke).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VM=boxer-smoke
BACKENDS=${*:-smolvm docker podman}
command -v limactl >/dev/null || { echo "needs Lima: brew install lima" >&2; exit 1; }
vmsh() { limactl shell "$VM" bash -lc "$1"; }

cleanup() {
  if [ "${BOXER_LIMA_KEEP:-}" != 1 ]; then limactl delete -f "$VM" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

limactl delete -f "$VM" >/dev/null 2>&1 || true
limactl create --name="$VM" --vm-type=vz --nested-virt --cpus=4 --memory=8 --disk=40 \
  --mount-none --tty=false template:ubuntu-lts >/dev/null
limactl start "$VM" >/dev/null

vmsh 'sudo DEBIAN_FRONTEND=noninteractive apt-get -qq update >/dev/null &&
    sudo DEBIAN_FRONTEND=noninteractive apt-get -qq install -y git python3 curl lsof xz-utils docker.io podman >/dev/null 2>&1 &&
    sudo usermod -aG kvm,docker "$USER" &&
    curl -sSL https://smolmachines.com/install.sh | bash >/dev/null 2>&1 &&
    mkdir -p ~/node && curl -fsSL https://nodejs.org/dist/v24.8.0/node-v24.8.0-linux-$(uname -m | sed "s/aarch64/arm64/;s/x86_64/x64/").tar.xz | tar -xJ -C ~/node --strip-components=1 &&
    PATH=~/node/bin:$PATH npm i -g --silent --prefix ~/node portless >/dev/null &&
    git config --global user.email t@t && git config --global user.name t'
# The new groups apply to new sessions only.
limactl stop "$VM" >/dev/null && limactl start "$VM" >/dev/null

ARCH=$(vmsh 'uname -m')
GOARCH=$([ "$ARCH" = x86_64 ] && echo amd64 || echo arm64)
(cd "$ROOT" && GOOS=linux GOARCH=$GOARCH go build -o "bin/boxer-linux-$GOARCH" ./cmd/boxer)
limactl copy "$ROOT/bin/boxer-linux-$GOARCH" "$VM:/tmp/boxer"
limactl copy "$ROOT/evals/smoke.sh" "$VM:/tmp/smoke.sh"
vmsh 'mkdir -p ~/bx/bin ~/bx/evals && mv /tmp/boxer ~/bx/bin/boxer && chmod +x ~/bx/bin/boxer && mv /tmp/smoke.sh ~/bx/evals/smoke.sh'

status=0
for b in $BACKENDS; do
  echo "== $b on Linux"
  vmsh "export PATH=\$HOME/bx/bin:\$HOME/node/bin:\$HOME/.local/bin:\$PATH; cd ~/bx && BOXER_BACKEND=$b BOXER_SMOKE_RECORD=\$HOME/bx/record.jsonl bash evals/smoke.sh \$HOME/bx/bin/boxer" || status=1
done
if [ -n "${BOXER_SMOKE_RECORD:-}" ]; then
  vmsh 'cat ~/bx/record.jsonl 2>/dev/null' >> "$BOXER_SMOKE_RECORD"
fi
exit $status
