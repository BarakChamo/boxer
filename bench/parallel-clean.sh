#!/usr/bin/env bash
# Several sandboxes at once, with every cell measured from a genuinely clean host.
#
# This exists because bench/parallel.sh cannot answer the memory question honestly. It runs every
# cell in one process, so a container runtime's Linux VM — which grows to hold what its containers
# touch and does not hand that back to macOS promptly — is already large by the time the later
# cells run. Their deltas then capture only marginal growth, while boxer, which is torn down
# between cells, is charged for everything each time. The result flattered the runtime by an
# unknown amount, which is worse than flattering it by a known one.
#
# So here each cell gets a wiped host: every container removed, the image cache pruned, the
# runtime *restarted* so its VM is back to idle size, and boxer's sandboxes destroyed. The image
# is then pulled back and the pack rebuilt BEFORE the baseline is taken, so the figure is the cost
# of running N dev servers and not the cost of fetching an image.
#
# One cell per invocation, deliberately: a loop would put us back where we started.
#
# Usage: bench/parallel-clean.sh <boxer|docker> <n>
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=$ROOT/bin/boxer
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"

CONTENDER=${1:?boxer or docker}
N=${2:?how many}
OUT=${CLEAN_OUT:-"$HERE/parallel-clean.jsonl"}
GOLDEN=${GOLDEN:-/tmp/memprobe/app}   # a Next.js app with dependencies already installed
IMAGE=mirror.gcr.io/library/node:24-alpine
CPUS=${PAR_CPUS:-2}                   # Vercel Sandbox's default, and its 2 GB per vCPU ratio
MEM_G=${PAR_MEM_G:-4}
BASEPORT=3900

EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$@"; fi

export BOXER_NO_RECLAIM=1
XDG_CONFIG_HOME=$(mktemp -d); export XDG_CONFIG_HOME
WORK=$(mktemp -d)

cleanup() {
  boxer down --all >/dev/null 2>&1 || true
  for i in $(seq 0 15); do docker rm -f "cl-$i" >/dev/null 2>&1 || true; done
  git -C "$WORK/base" worktree prune >/dev/null 2>&1 || true
  # Remove the scratch tree. Without this every run leaves a copy of the reference worktree
  # behind — three linked worktrees each holding a full node_modules, about a gigabyte a cell —
  # and nothing ever reclaims it because it is under $TMPDIR, which macOS only prunes for files
  # older than three days. Twenty benchmark cells filled a 460 GB disk.
  rm -rf "$WORK" ${XDG_CONFIG_HOME:+"$XDG_CONFIG_HOME"} 2>/dev/null || true
}
trap cleanup EXIT

used_mb() {
  vm_stat 2>/dev/null | awk '
    /page size of/ { match($0, /page size of [0-9]+/); pg = substr($0, RSTART+13, RLENGTH-13) }
    /Pages active/            { gsub(/\./,""); a = $3 }
    /Pages wired down/        { gsub(/\./,""); w = $4 }
    /Pages occupied by compressor/ { gsub(/\./,""); c = $5 }
    END { printf "%d", int((a + w + c) * pg / 1048576) }'
}
swap_mb() { sysctl -n vm.swapusage | awk '{gsub(/M/,"",$6); printf "%d", $6}'; }
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

# --- wipe -------------------------------------------------------------------------------------
# Both runtimes are emptied whichever contender is about to run, so neither is measured on a host
# the other left warm.
echo "# wiping"
boxer down --all >/dev/null 2>&1 || true
for i in $(seq 0 15); do docker rm -f "cl-$i" >/dev/null 2>&1 || true; done
docker system prune -af >/dev/null 2>&1 || true
# Restart the runtime so its VM returns to idle size. orb is OrbStack's own CLI; fall back to
# nothing rather than guessing at Docker Desktop's.
if command -v orb >/dev/null 2>&1; then
  orb stop >/dev/null 2>&1 || true
  sleep 8
  orb start >/dev/null 2>&1 || true
  # Wait for the daemon rather than sleeping a guessed amount.
  for _ in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 2; done
fi
pkill -f "next dev -p 39" >/dev/null 2>&1 || true
sleep 5

# --- worktrees --------------------------------------------------------------------------------
# One repository, N linked worktrees: boxer's real shape, and the case where a shared .git and a
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

# --- prime the image, before the baseline -----------------------------------------------------
# Neither contender should be charged for a registry pull. boxer's equivalent of a pulled image is
# a built pack, so it gets one boot-and-teardown of the first worktree.
echo "# priming"
docker pull -q "$IMAGE" >/dev/null 2>&1 || true
if [ "$CONTENDER" = boxer ]; then
  first=$(echo "$dirs" | awk '{print $1}')
  ( cd "$first" && boxer up >/dev/null 2>&1; boxer down >/dev/null 2>&1 ) || true
fi
sleep 5

# --- measure ----------------------------------------------------------------------------------
base=$(used_mb); swap0=$(swap_mb)
t=$(date +%s%N)
case $CONTENDER in
boxer)
  for d in $dirs; do ( cd "$d" && boxer up >/dev/null 2>&1 ) & done
  ok=1; for p in $ports; do waitport "$p" || ok=0; done
  wait
  ;;
docker)
  i=0
  for d in $dirs; do
    i=$((i+1)); p=$((BASEPORT+i))
    docker run -d --name "cl-$i" --cpus "$CPUS" -m "${MEM_G}g" -v "$d:/w" -w /w -p "$p:$p" \
      "$IMAGE" sh -c "cd app && npx next dev -p $p -H 0.0.0.0" >/dev/null 2>&1 &
  done
  wait
  ok=1; for p in $ports; do waitport "$p" || ok=0; done
  ;;
*) echo "unknown contender $CONTENDER"; exit 2;;
esac
el=$(sec "$t")
sleep 5
mem=$(( $(used_mb) - base )); swap=$(( $(swap_mb) - swap0 ))

printf '%-7s n=%-2s ready %6ss  host %5s MB (%4s MB each)  swap +%s MB  ok=%s\n' \
  "$CONTENDER" "$N" "$el" "$mem" "$((mem/N))" "$swap" "$ok"
echo "{\"contender\":\"$CONTENDER\",\"n\":$N,\"cpus\":$CPUS,\"mem_g\":$MEM_G,\"seconds\":$el,\"host_mb\":$mem,\"swap_delta_mb\":$swap,\"clean\":true,\"ok\":$ok}" >> "$OUT"
[ "$ok" = 1 ] || exit 1
exit 0
