#!/usr/bin/env bash
# Time to a Next.js dev server answering HTTP 200, boxer against no sandbox and against Docker.
#
# This is the workload boxer is FOR: a long-lived sandbox over a git worktree, with a dev server
# in it. bench.sh measures one-off commands, which boxer also supports and which is a different
# question with a different answer.
#
# ---------------------------------------------------------------------------------------------
# WHAT IS HELD EQUAL
#
# Everything that moved a number during development, because each of these produced a plausible
# wrong answer before it was pinned:
#
#   resources   10 vCPU, 8 GB to boxer and to Docker alike. boxer's own default is 4/4G and
#               Docker Desktop's VM was 10/11.7G, which alone made npm install look 1.6x worse
#               than it is.
#   image       mirror.gcr.io/library/node:24-alpine everywhere. Image size is microVM start-up
#               time, so comparing two contenders on different images measures the images.
#   npm cache   a fresh, private cache directory for every install. The first contender to run
#               otherwise warms the host's shared cache and every later one looks fast.
#   lockfile    package-lock.json present and identical. Without it npm resolves the tree from
#               scratch, which is slower and has nothing to do with sandboxing.
#   .next       deleted before every server start. Next.js caches its compilation in the
#               worktree, so a second contender reading a warm .next is measuring the first one.
#   worktree    each contender gets its own copy, so nothing leaks between them.
#   ready       an HTTP 200 from the host. Not a log line, not a live process.
#
# ---------------------------------------------------------------------------------------------
# THE FOUR SCENARIOS
#
#   1  first-ever     Nothing cached anywhere: boxer has no pack, the container has no image.
#                     What a brand-new machine pays, once.
#   2  install        Dependencies not yet installed; scaffold is present. Measures `npm install`,
#                     which is where a filesystem boundary shows up.
#   3  cold-start     Dependencies installed, .next empty, sandbox not running. **This is the
#                     number that matters**: what starting a session costs, over and over.
#   4  warm-start     Dependencies installed, .next warm, sandbox not running. A restart.
#
# Scenario 3 is also where boxer's "pre-run" answer lives: `setup` runs once per worktree and
# `image_setup` is snapshotted into a pack, so a project can arrange for scenario 3 to be the
# common case rather than scenario 2.
#
# Usage: bench/devserver.sh [path-to-boxer-binary]
#   DEV_ONLY=host,docker,boxer   contenders to run
#   DEV_SCENARIOS=3,4            scenarios to run
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=${1:-"$ROOT/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
OUT=${DEV_OUT:-"$HERE/devserver.jsonl"}

NEXT=15.5.4
IMAGE=mirror.gcr.io/library/node:24-alpine
CPUS=10
MEM_G=8
PORT=3511

EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$@"; fi

python3 -c "import socket,sys;s=socket.socket();sys.exit(0 if s.connect_ex(('registry.npmjs.org',443))==0 else 1)" 2>/dev/null || {
  echo "skipped: the npm registry is unreachable and every scenario here installs a real project"; exit 0; }

export BOXER_NO_RECLAIM=1
XDG_CONFIG_HOME=$(mktemp -d); export XDG_CONFIG_HOME
WORK=$(mktemp -d)
CTR=boxer-devbench
GOLDEN="$WORK/golden"

cleanup() {
  for d in "$WORK"/wt-*/; do (cd "$d" 2>/dev/null && boxer down >/dev/null 2>&1) || true; done
  docker rm -f "$CTR" >/dev/null 2>&1 || true
  pkill -f "next dev -p $PORT" >/dev/null 2>&1 || true
  # Remove the scratch tree. Without this, every run leaves its temporary worktrees behind —
  # each with a full node_modules — under $TMPDIR, which macOS only prunes for files older than
  # three days. Twenty benchmark cells filled a 460 GB disk before anyone noticed.
  rm -rf "$WORK" ${XDG_CONFIG_HOME:+"$XDG_CONFIG_HOME"} 2>/dev/null || true
}
trap cleanup EXIT

: > "$OUT"
sec() { python3 -c "print('%.1f' % (($(date +%s%N) - $1)/1e9))"; }
emit() { # emit <scenario> <contender> <seconds>; a negative time means the cell failed
  if [ "${3%%.*}" -lt 0 ] 2>/dev/null; then
    printf '  %-12s %-8s %9s\n' "$1" "$2" "FAILED"
    failed="$failed $1/$2"
  else
    printf '  %-12s %-8s %7s s\n' "$1" "$2" "$3"
  fi
  echo "{\"scenario\":\"$1\",\"contender\":\"$2\",\"seconds\":$3,\"image\":\"$IMAGE\",\"cpus\":$CPUS,\"mem_gb\":$MEM_G}" >> "$OUT"
}

# A cell that fails must be reported and the run must continue. The first version of this file
# had `set -e` abort the whole benchmark at the first bad cell, which silently truncated the
# results table — the table looked complete because the missing rows had never been printed.
cell() { # cell <scenario> <contender> <function...>; times it, or reports failure
  local sc=$1 c=$2; shift 2
  local t; t=$(date +%s%N)
  if "$@"; then emit "$sc" "$c" "$(sec "$t")"; else emit "$sc" "$c" -1; fi
}
failed=

waitport() {
  python3 - "$PORT" <<'PY'
import sys, time, urllib.request
deadline = time.monotonic() + 240
while time.monotonic() < deadline:
    try:
        if urllib.request.urlopen("http://127.0.0.1:%s/" % sys.argv[1], timeout=2).status == 200:
            sys.exit(0)
    except Exception:
        time.sleep(0.05)
sys.exit(1)
PY
}

# ---------------------------------------------------------------- the golden worktree
# Scaffolded and installed once on the host, then copied per contender per scenario. This is what
# makes the scenarios comparable: every contender starts from byte-identical files.
echo "# building the reference worktree once (scaffold + install on the host)"
mkdir -p "$GOLDEN"
( cd "$GOLDEN" && npx --yes create-next-app@"$NEXT" app --yes --ts --app --no-eslint --no-tailwind \
    --no-src-dir --no-import-alias --use-npm --skip-install >/dev/null 2>&1 )
( cd "$GOLDEN/app" && npm install --no-audit --no-fund >/dev/null 2>&1 )

# wt <name> <deps: yes|no> <next-cache: warm|cold>
wt() {
  local d="$WORK/wt-$1"
  rm -rf "$d"; mkdir -p "$d"
  cp -R "$GOLDEN/app" "$d/app"
  [ "$2" = no ] && rm -rf "$d/app/node_modules"
  [ "$3" = cold ] && rm -rf "$d/app/.next"
  git -C "$d" init -q
  git -C "$d" -c user.email=b@b -c user.name=b add -A >/dev/null 2>&1 || true
  git -C "$d" -c user.email=b@b -c user.name=b commit -q -m w >/dev/null 2>&1 || true
  cat > "$d/boxer.toml" <<TOML
require_worktree = "off"
image = "$IMAGE"
cpus = $CPUS
memory = "${MEM_G}G"
setup = ["cd app && npm install --no-audit --no-fund"]
start = ["cd app && npx next dev -p $PORT -H 0.0.0.0"]
ready = "node -e \"fetch('http://127.0.0.1:$PORT/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))\""
ready_timeout = "240s"
[network]
mode = "on"
ports = ["$PORT:$PORT"]
TOML
  echo "$d"
}

# noserve rewrites a worktree's config to do the install and nothing else. Without this the
# install scenario measured the install twice: `boxer run` provisions, provisioning runs `setup`,
# `setup` is the npm install, and then the timed command installed all over again — 21.7s for
# what is really a 10s operation. `setup` is also the honest thing to measure here, because it is
# boxer's own answer to "install the dependencies before anything runs".
noserve() { # noserve <dir>
  cat > "$1/boxer.toml" <<TOML
require_worktree = "off"
image = "$IMAGE"
cpus = $CPUS
memory = "${MEM_G}G"
setup = ["cd app && rm -rf /tmp/nc && npm_config_cache=/tmp/nc npm install --no-audit --no-fund"]
[network]
mode = "on"
TOML
}

host_serve() { # host_serve <dir>; deps must already be installed
  ( cd "$1/app" && npm_config_cache="$WORK/npm-$RANDOM" npx next dev -p "$PORT" -H 127.0.0.1 >/dev/null 2>&1 & )
  waitport && pkill -f "next dev -p $PORT" >/dev/null 2>&1
}

docker_up() { # docker_up <dir> <install: yes|no>
  docker rm -f "$CTR" >/dev/null 2>&1 || true
  docker run -d --name "$CTR" --cpus "$CPUS" -m "${MEM_G}g" -v "$1:/w" -w /w -p "$PORT:$PORT" \
    "$IMAGE" sleep 7200 >/dev/null
  [ "$2" = yes ] && docker exec "$CTR" sh -c "cd app && npm_config_cache=/tmp/c npm install --no-audit --no-fund" >/dev/null 2>&1
  docker exec -d "$CTR" sh -c "cd app && npx next dev -p $PORT -H 0.0.0.0" >/dev/null 2>&1
  waitport
  docker rm -f "$CTR" >/dev/null 2>&1 || true
}

CONTENDERS=${DEV_ONLY:-"host docker boxer"}
SCENARIOS=${DEV_SCENARIOS:-"1 2 3 4"}
echo "# next@$NEXT · $IMAGE · ${CPUS} vCPU / ${MEM_G}G for every sandbox · ready = HTTP 200"

# ---------------------------------------------------------------- cell bodies
b_up()      { ( cd "$1" && boxer up >/dev/null 2>&1 ) && waitport; }
b_install() { ( cd "$1" && boxer up >/dev/null 2>&1 ); }   # provision + setup, nothing else
d_install() { # d_install <dir>: container create + install, so it matches boxer's scope
  docker rm -f "$CTR" >/dev/null 2>&1 || true
  docker run -d --name "$CTR" --cpus "$CPUS" -m "${MEM_G}g" -v "$1:/w" -w /w "$IMAGE" sleep 3600 >/dev/null
  docker exec "$CTR" sh -c "cd app && rm -rf /tmp/nc && npm_config_cache=/tmp/nc npm install --no-audit --no-fund" >/dev/null 2>&1
  local rc=$?; docker rm -f "$CTR" >/dev/null 2>&1 || true; return $rc
}
b_down()    { ( cd "$1" && boxer down >/dev/null 2>&1 ) || true; }

for sc in $SCENARIOS; do
case $sc in
1) # first-ever: nothing cached on the host at all
  echo "# 1 first-ever (no pack, no pulled image; once per machine)"
  for c in $CONTENDERS; do case $c in
    boxer) d=$(wt f-boxer yes cold); boxer gc --all >/dev/null 2>&1 || true
           cell first-ever boxer b_up "$d"; b_down "$d" ;;
    docker) d=$(wt f-docker yes cold); docker rmi -f "$IMAGE" >/dev/null 2>&1 || true
           cell first-ever docker docker_up "$d" no ;;
    host)  emit first-ever host 0.0 ;;  # nothing to provision
  esac; done ;;

2) # install: the dependency install, which is the filesystem-bound part
  echo "# 2 install (dependencies absent; scaffold present; fresh npm cache)"
  for c in $CONTENDERS; do case $c in
    # Both sandbox rows include creating the sandbox, because "install the dependencies" from a
    # standing start is the comparable unit. The host row has nothing to create.
    boxer) d=$(wt i-boxer no cold); noserve "$d"
           cell install boxer b_install "$d"; b_down "$d" ;;
    docker) d=$(wt i-docker no cold); cell install docker d_install "$d" ;;
    host)  d=$(wt i-host no cold)
           t=$(date +%s%N)
           ( cd "$d/app" && npm_config_cache="$WORK/nc-host" npm install --no-audit --no-fund >/dev/null 2>&1 )
           emit install host "$(sec "$t")" ;;
  esac; done ;;

3) # cold-start: THE number. deps installed, .next empty, sandbox down.
  echo "# 3 cold-start (deps installed, .next empty, nothing running) <- the session-start cost"
  for c in $CONTENDERS; do case $c in
    boxer) d=$(wt c-boxer yes cold)
           ( cd "$d" && boxer up >/dev/null 2>&1 ) && waitport; b_down "$d"   # build the pack
           rm -rf "$d/app/.next"
           cell cold-start boxer b_up "$d"; b_down "$d" ;;
    docker) d=$(wt c-docker yes cold); cell cold-start docker docker_up "$d" no ;;
    host)  d=$(wt c-host yes cold); cell cold-start host host_serve "$d" ;;
  esac; done ;;

4) # warm-start: deps installed, .next warm. A restart mid-session.
  echo "# 4 warm-start (deps installed, .next warm) <- a restart"
  for c in $CONTENDERS; do case $c in
    boxer) d=$(wt w-boxer yes cold)
           ( cd "$d" && boxer up >/dev/null 2>&1 ) && waitport; b_down "$d"
           cell warm-start boxer b_up "$d"; b_down "$d" ;;
    docker) d=$(wt w-docker yes cold); docker_up "$d" no || true
           cell warm-start docker docker_up "$d" no ;;
    host)  d=$(wt w-host yes cold); host_serve "$d" || true
           cell warm-start host host_serve "$d" ;;
  esac; done ;;
esac
done

echo
echo "wrote $OUT"
[ -n "$failed" ] && { echo "FAILED cells:$failed"; exit 1; }
exit 0
