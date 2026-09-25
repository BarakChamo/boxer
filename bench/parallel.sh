#!/usr/bin/env bash
# Several sandboxes at once — boxer's primary use case, and the one the other benchmarks miss.
#
# One agent per git worktree means N sandboxes live together, so what matters is not what one
# costs but how the Nth behaves: whether time-to-ready degrades, and what the whole set holds in
# memory. A per-sandbox figure measured alone tells you neither.
#
# The comparison against a container runtime is deliberately whole-system. A container runtime on
# macOS runs one Linux VM and puts every container inside it: containers are cheap because the
# expensive thing was paid once, at boot, and it stays resident whether you are using it or not.
# boxer boots a microVM per sandbox: nothing is resident when nothing is running, and each sandbox
# carries its own kernel. Charging the runtime only for its containers and boxer for its whole VMs
# would be the easiest way to make this table lie, so both are measured as resident set totals
# from the host's point of view.
#
# Usage: bench/parallel.sh [path-to-boxer-binary]
#   PAR_N="1 3 5"     how many at once
#   PAR_ONLY=boxer    contenders
#   PAR_CPUS=2        vCPUs per sandbox
#   PAR_MEM_G=4       GB per sandbox
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=${1:-"$ROOT/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
OUT=${PAR_OUT:-"$HERE/parallel.jsonl"}

NEXT=15.5.4
IMAGE=mirror.gcr.io/library/node:24-alpine
# 2 vCPU / 4 GB is Vercel Sandbox's default, and Vercel Sandbox is the closest published thing to
# what boxer does: an agent's code in a Firecracker microVM rather than in a container. Its ratio
# is fixed at 2 GB per vCPU, so matching the shape means matching both numbers, not just the CPU.
# It is also close to boxer's own shipped default of 4 vCPU / 4 GB, which is deliberately modest
# because boxer runs one VM per worktree and ten worktrees at 10 vCPU would thrash a 10-core host.
CPUS=${PAR_CPUS:-2}
MEM_G=${PAR_MEM_G:-4}
BASEPORT=3700

EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$@"; fi

export BOXER_NO_RECLAIM=1
XDG_CONFIG_HOME=$(mktemp -d); export XDG_CONFIG_HOME
WORK=$(mktemp -d)
GOLDEN="$WORK/golden"

cleanup() {
  boxer down --all >/dev/null 2>&1 || true
  git -C "$WORK/base" worktree prune >/dev/null 2>&1 || true
  for i in $(seq 0 15); do docker rm -f "par-$i" >/dev/null 2>&1 || true; done
  pkill -f "next dev -p 37" >/dev/null 2>&1 || true
  # Remove the scratch tree. Without this, every run leaves its temporary worktrees behind —
  # each with a full node_modules — under $TMPDIR, which macOS only prunes for files older than
  # three days. Twenty benchmark cells filled a 460 GB disk before anyone noticed.
  rm -rf "$WORK" ${XDG_CONFIG_HOME:+"$XDG_CONFIG_HOME"} 2>/dev/null || true
}
trap cleanup EXIT
: > "$OUT"

# used_mb -> what the host cannot cheaply give back: active + wired + compressed pages.
#
# NOT the resident set of the hypervisor process. A VMM maps the whole guest address space, so
# `ps rss` counts memory the guest has never touched and the host has never committed: it read
# 807 MB for a sandbox whose real cost was a fraction of that. smolvm returns idle
# guest memory through virtio-balloon, which RSS cannot see at all.
#
# It also has to be whole-system to be fair. A container runtime on macOS keeps one Linux VM
# resident whether or not you are using it, and its containers are then nearly free at the margin;
# boxer keeps nothing resident and pays per sandbox. Measuring each side's own processes would flatter
# whichever one hides its fixed cost, so both are measured as a delta in host memory pressure.
used_mb() {
  vm_stat 2>/dev/null | awk '
    /page size of/ { match($0, /page size of [0-9]+/); pg = substr($0, RSTART+13, RLENGTH-13) }
    /Pages active/            { gsub(/\./,""); a = $3 }
    /Pages wired down/        { gsub(/\./,""); w = $4 }
    /Pages occupied by compressor/ { gsub(/\./,""); c = $5 }
    END { printf "%d", int((a + w + c) * pg / 1048576) }'
}
sec() { python3 -c "print('%.1f' % (($(date +%s%N) - $1)/1e9))"; }

waitport() {
  python3 - "$1" <<'PY'
import sys, time, urllib.request
deadline = time.monotonic() + 300
while time.monotonic() < deadline:
    try:
        if urllib.request.urlopen("http://127.0.0.1:%s/" % sys.argv[1], timeout=2).status == 200:
            sys.exit(0)
    except Exception:
        time.sleep(0.1)
sys.exit(1)
PY
}

echo "# building the reference worktree once"
mkdir -p "$GOLDEN"
( cd "$GOLDEN" && npx --yes create-next-app@"$NEXT" app --yes --ts --app --no-eslint --no-tailwind \
    --no-src-dir --no-import-alias --use-npm --skip-install >/dev/null 2>&1 )
( cd "$GOLDEN/app" && npm install --no-audit --no-fund >/dev/null 2>&1 )

# One repository, N linked worktrees. That is boxer's actual shape — a sandbox per worktree of one
# checkout — and it is not the same test as N unrelated repositories: linked worktrees share a
# single .git, so anything boxer gets wrong about `--git-common-dir` or about locking a shared
# store shows up here and nowhere else.
BASE="$WORK/base"
git init -q "$BASE"
: > "$BASE/.keep"
git -C "$BASE" add -A
git -C "$BASE" -c user.email=bench@local -c user.name=bench commit -qm base

wt() { # wt <n> <port>: a linked worktree with deps present and no build cache
  local d="$WORK/p$1"
  # git keeps its own registry of worktrees, so deleting the directory is not enough: a plain
  # `rm -rf` leaves the path registered and the next `worktree add` refuses it as "missing but
  # already registered". Deregister first, then prune whatever an earlier crash left behind.
  git -C "$BASE" worktree remove --force "$d" >/dev/null 2>&1 || true
  rm -rf "$d"
  git -C "$BASE" worktree prune >/dev/null 2>&1 || true
  git -C "$BASE" worktree add -q --detach "$d" HEAD
  cp -R "$GOLDEN/app" "$d/app"; rm -rf "$d/app/.next"
  cat > "$d/boxer.toml" <<TOML
require_worktree = "require"
image = "$IMAGE"
cpus = $CPUS
memory = "${MEM_G}G"
start = ["cd app && npx next dev -p $2 -H 0.0.0.0"]
ready = "node -e \"fetch('http://127.0.0.1:$2/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "300s"
[network]
mode = "on"
ports = ["$2:$2"]
TOML
  echo "$d"
}

PAR_N=${PAR_N:-"1 3 5"}
CONTENDERS=${PAR_ONLY:-"boxer docker"}
echo "# $IMAGE · ${CPUS} vCPU / ${MEM_G}G each · ready = HTTP 200 on every port"
runtime_fixed=$(ps -Ao rss,command 2>/dev/null | awk 'index($0,"OrbStack")>0 || index($0,"com.docker")>0 {s+=$1} END {printf "%d", int(s/1024)+0}')
echo "# memory is the delta in host pressure (active+wired+compressed), not process RSS"
echo "# the container runtime additionally holds ${runtime_fixed} MB resident before any container; boxer holds none"


for c in $CONTENDERS; do
for n in $PAR_N; do
  # Start n sandboxes at once and wait for all n to answer.
  case $c in
  boxer)
    boxer down --all >/dev/null 2>&1 || true; sleep 2
    dirs=""; ports=""
    for i in $(seq 1 "$n"); do
      p=$((BASEPORT+i)); dirs="$dirs $(wt "$i" "$p")"; ports="$ports $p"
    done
    # Warm the pack so this measures N boots, not one pack build.
    first=$(echo "$dirs" | awk '{print $1}')
    ( cd "$first" && boxer up >/dev/null 2>&1; boxer down >/dev/null 2>&1 ) || true
    # Baseline *after* the warm-up, not before it. Sampling first charged boxer for everything the
    # warm-up left in host memory — a pack read, an image cache, the page cache of a Next.js
    # install — and read 1,609 MB for a single sandbox whose real cost is a fraction of that. The
    # container rows have no warm-up, so sampling before it also made the two sides incomparable.
    sleep 2
    base=$(used_mb)
    t=$(date +%s%N)
    for d in $dirs; do ( cd "$d" && boxer up >/dev/null 2>&1 ) & done
    ok=1; for p in $ports; do waitport "$p" || ok=0; done
    wait
    el=$(sec "$t"); mem=$(( $(used_mb) - base ))
    printf '  %-7s n=%-2s ready %6ss  resident %5s MB  (%s MB each)\n' boxer "$n" "$el" "$mem" "$((mem/n))"
    echo "{\"contender\":\"boxer\",\"n\":$n,\"seconds\":$el,\"resident_mb\":$mem,\"ok\":$ok}" >> "$OUT"
    boxer down --all >/dev/null 2>&1 || true
    ;;
  docker)
    for i in $(seq 0 15); do docker rm -f "par-$i" >/dev/null 2>&1 || true; done; sleep 1
    # Measured the same way, as a delta. The runtime's always-resident VM is reported separately
    # by the header rather than folded in here, because it is a constant, not a per-container cost.
    base=$(used_mb)
    dirs=""; ports=""
    for i in $(seq 1 "$n"); do
      p=$((BASEPORT+i)); dirs="$dirs $(wt "$i" "$p")"; ports="$ports $p"
    done
    t=$(date +%s%N)
    i=0
    for d in $dirs; do
      i=$((i+1)); p=$((BASEPORT+i))
      docker run -d --name "par-$i" --cpus "$CPUS" -m "${MEM_G}g" -v "$d:/w" -w /w -p "$p:$p" \
        "$IMAGE" sh -c "cd app && npx next dev -p $p -H 0.0.0.0" >/dev/null 2>&1 &
    done
    wait
    ok=1; for p in $ports; do waitport "$p" || ok=0; done
    el=$(sec "$t")
    mem=$(( $(used_mb) - base ))
    printf '  %-7s n=%-2s ready %6ss  resident %5s MB  (whole runtime)\n' docker "$n" "$el" "$mem"
    echo "{\"contender\":\"docker\",\"n\":$n,\"seconds\":$el,\"resident_mb\":$mem,\"ok\":$ok}" >> "$OUT"
    for i in $(seq 0 15); do docker rm -f "par-$i" >/dev/null 2>&1 || true; done
    ;;
  esac
done
done

echo
echo "wrote $OUT"
grep -q '"ok":0' "$OUT" && { echo "FAILED: not every port answered"; exit 1; }
exit 0
