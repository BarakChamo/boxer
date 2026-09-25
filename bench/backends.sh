#!/usr/bin/env bash
# Three worktrees, one Next.js dev server each, on every backend boxer has.
#
# This is the comparison that matters once there is more than one backend: not "boxer against a
# container runtime", which asks whether sandboxing is worth it, but "the same boxer against each
# of the things it can sandbox with", which asks which one to choose. Three worktrees because
# one hides the cost of sharing a machine and five saturates a ten-core host, so three is where a
# difference is still attributable.
#
# Every cell is measured from a wiped host, one cell per invocation, for the reason written at
# length in parallel-clean.sh: running cells in one process lets a runtime that keeps a VM warm
# charge later cells less than earlier ones, and the result flatters whoever went last.
#
# `raw-docker` is the no-boxer baseline — the same three dev servers started with `docker run` and
# no sandbox management at all. It is what the other rows are paying for.
#
# Usage: bench/backends.sh <smolvm|docker|podman|container|raw-docker> [n]
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=$ROOT/bin/boxer
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"

CONTENDER=${1:?smolvm|docker|podman|container|raw-docker}
N=${2:-3}
OUT=${BENCH_OUT:-"$HERE/backends.jsonl"}
GOLDEN=${GOLDEN:-/tmp/bench-golden/app}
IMAGE=mirror.gcr.io/library/node:24-alpine
CPUS=${BENCH_CPUS:-2}        # Vercel Sandbox's default shape, and modest enough for three at once
MEM_G=${BENCH_MEM_G:-4}
BASEPORT=3950

EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$@"; fi

export BOXER_NO_RECLAIM=1
WORK=$(mktemp -d)

cleanup() {
  BOXER_BACKEND=$backend boxer down --all >/dev/null 2>&1 || true
  for i in $(seq 0 9); do
    docker rm -f "bb-$i" >/dev/null 2>&1 || true
    podman rm -f "bb-$i" >/dev/null 2>&1 || true
  done
  git -C "$WORK/base" worktree prune >/dev/null 2>&1 || true
  # Remove the scratch tree. Without this every run leaves a copy of the reference worktree
  # behind — three linked worktrees each holding a full node_modules, about a gigabyte a cell —
  # and nothing ever reclaims it because it is under $TMPDIR, which macOS only prunes for files
  # older than three days. Twenty benchmark cells filled a 460 GB disk.
  rm -rf "$WORK" ${XDG_CONFIG_HOME:+"$XDG_CONFIG_HOME"} 2>/dev/null || true
}

# The boxer backend this contender exercises. raw-docker runs no boxer at all.
case $CONTENDER in
smolvm|docker|podman|container) backend=$CONTENDER ;;
raw-docker) backend="" ;;
*) echo "unknown contender $CONTENDER"; exit 2 ;;
esac
trap cleanup EXIT

sec() { python3 -c "print('%.1f' % (($(date +%s%N) - $1)/1e9))"; }
waitport() {
  python3 - "$1" <<'PY'
import sys, time, urllib.request
deadline = time.monotonic() + 600
while time.monotonic() < deadline:
    try:
        if urllib.request.urlopen("http://127.0.0.1:%s/" % sys.argv[1], timeout=2).status == 200:
            sys.exit(0)
    except Exception:
        time.sleep(0.1)
sys.exit(1)
PY
}

# --- the reference worktree ---------------------------------------------------------------------
# Built once and reused, with dependencies already installed: this measures bringing a dev server
# up, not installing node_modules, and an install would swamp the difference between backends.
if [ ! -d "$GOLDEN" ]; then
  echo "# building the reference worktree once"
  mkdir -p "$(dirname "$GOLDEN")"
  ( cd "$(dirname "$GOLDEN")" && npx --yes create-next-app@15.5.4 app --yes --ts --app --no-eslint \
      --no-tailwind --no-src-dir --no-import-alias --use-npm --skip-install >/dev/null 2>&1 )
  ( cd "$GOLDEN" && npm install --no-audit --no-fund >/dev/null 2>&1 )
fi

# --- wipe ---------------------------------------------------------------------------------------
# Every runtime is emptied whichever contender is about to run, so none is measured on a host
# another left warm.
echo "# wiping"
for b in smolvm docker podman container; do
  BOXER_BACKEND=$b boxer down --all >/dev/null 2>&1 || true
done
for i in $(seq 0 9); do
  docker rm -f "bb-$i" >/dev/null 2>&1 || true
  podman rm -f "bb-$i" >/dev/null 2>&1 || true
  container delete --force "bb-$i" >/dev/null 2>&1 || true
done
pkill -f "next dev -p 39" >/dev/null 2>&1 || true
sleep 3

# --- worktrees ----------------------------------------------------------------------------------
# One repository, N linked worktrees: boxer's actual shape, and the case where a shared .git and a
# shared machine store are both under contention.
BASE="$WORK/base"
git init -q "$BASE"
: > "$BASE/.keep"
git -C "$BASE" add -A
git -C "$BASE" -c user.email=bench@local -c user.name=bench commit -qm base

dirs=""; ports=""
for i in $(seq 1 "$N"); do
  p=$((BASEPORT+i)); d="$WORK/w$i"
  git -C "$BASE" worktree add -q --detach "$d" HEAD
  cp -R "$GOLDEN" "$d/app"; rm -rf "$d/app/.next"
  cat > "$d/boxer.toml" <<TOML
require_worktree = "require"
image = "$IMAGE"
cpus = $CPUS
memory = "${MEM_G}G"
start = ["cd app && npx next dev -p $p -H 0.0.0.0"]
ready = "node -e \"fetch('http://127.0.0.1:$p/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "600s"
[network]
mode = "on"
ports = ["$p:$p"]
TOML
  dirs="$dirs $d"; ports="$ports $p"
done

# --- prime --------------------------------------------------------------------------------------
# Nobody is charged for fetching an image. boxer's equivalent of a pulled image is a built pack,
# so a backend that has packs gets one boot-and-teardown of the first worktree to build it.
echo "# priming"
case $CONTENDER in
raw-docker|docker) docker pull -q "$IMAGE" >/dev/null 2>&1 || true ;;
podman)            podman pull -q "$IMAGE" >/dev/null 2>&1 || true ;;
container)         container image pull "$IMAGE" >/dev/null 2>&1 || true ;;
esac
if [ -n "$backend" ]; then
  first=$(echo "$dirs" | awk '{print $1}')
  ( cd "$first" && BOXER_BACKEND=$backend boxer up >/dev/null 2>&1
    BOXER_BACKEND=$backend boxer down >/dev/null 2>&1 ) || true
fi
sleep 3

# --- measure ------------------------------------------------------------------------------------
t=$(date +%s%N)
if [ -n "$backend" ]; then
  for d in $dirs; do ( cd "$d" && BOXER_BACKEND=$backend boxer up >/dev/null 2>&1 ) & done
else
  i=0
  for d in $dirs; do
    i=$((i+1)); p=$((BASEPORT+i))
    docker run -d --name "bb-$i" --cpus "$CPUS" -m "${MEM_G}g" -v "$d:/w" -w /w -p "$p:$p" \
      "$IMAGE" sh -c "cd app && npx next dev -p $p -H 0.0.0.0" >/dev/null 2>&1 &
  done
fi
ok=1; for p in $ports; do waitport "$p" || ok=0; done
wait
el=$(sec "$t")

printf '%-11s n=%-2s ready %6ss  ok=%s\n' "$CONTENDER" "$N" "$el" "$ok"
echo "{\"contender\":\"$CONTENDER\",\"n\":$N,\"cpus\":$CPUS,\"mem_g\":$MEM_G,\"seconds\":$el,\"ok\":$ok}" >> "$OUT"
[ "$ok" = 1 ] || exit 1
exit 0
