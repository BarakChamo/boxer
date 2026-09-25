#!/usr/bin/env bash
# Performance benchmark: what does boxer cost, and who is faster?
#
# Every contender runs the same shell line with the same corpus as its working directory, so a
# workload can only use relative paths. That is the only way the mount translations stay
# comparable: the host sees the corpus directly, boxer and raw smolvm see it at /workspace,
# Docker sees it at /w, and none of the workloads can tell.
#
# Usage: bench/bench.sh [path-to-boxer-binary]
#   BENCH_REPS=N      samples per cell (default 20)
#   BENCH_ONLY=a,b    only these contenders
#   BENCH_OUT=path    JSONL destination (default bench/results.jsonl)
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=${1:-"$ROOT/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
REPS=${BENCH_REPS:-20}
OUT=${BENCH_OUT:-"$HERE/results.jsonl"}
TIMEIT="$HERE/timeit.py"

# One real-VM user at a time on this host, the same lock the smoke and eval tiers take. A
# benchmark that shares a host with an eval measures the eval.
EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$@"; fi

export BOXER_NO_RECLAIM=1
XDG_CONFIG_HOME=$(mktemp -d); export XDG_CONFIG_HOME
WORK=$(mktemp -d)
CORPUS="$WORK/repo"
DOCKER_CTR=boxer-bench

cleanup() {
  (cd "$CORPUS" 2>/dev/null && boxer down >/dev/null 2>&1) || true
  docker rm -f "$DOCKER_CTR" >/dev/null 2>&1 || true
  # Remove the scratch tree. Without this, every run leaves its temporary worktrees behind —
  # each with a full node_modules — under $TMPDIR, which macOS only prunes for files older than
  # three days. Twenty benchmark cells filled a 460 GB disk before anyone noticed.
  rm -rf "$WORK" ${XDG_CONFIG_HOME:+"$XDG_CONFIG_HOME"} 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------- corpus
# Fixed, generated, committed nowhere: 400 source-shaped files across 20 directories, ~1.6 MB.
# Big enough that a filesystem crossing a VM boundary shows up, small enough to build in a second.
mkdir -p "$CORPUS"
git -C "$CORPUS" init -q
git -C "$CORPUS" -c user.email=b@b -c user.name=b commit -q --allow-empty -m init
python3 - "$CORPUS" <<'PY'
import os, sys
root = sys.argv[1]
line = "// boxer benchmark corpus line with enough text to be worth scanning\n"
for d in range(20):
    p = os.path.join(root, "pkg%02d" % d)
    os.makedirs(p, exist_ok=True)
    for f in range(20):
        with open(os.path.join(p, "file%02d.go" % f), "w") as fh:
            fh.write(line * 50)
PY
cat > "$CORPUS/boxer.toml" <<'TOML'
image = "alpine"
memory = "2G"
cpus = 4
require_worktree = "off"
TOML
cd "$CORPUS"

# ---------------------------------------------------------------- workloads
# Relative paths only. Each is a POSIX sh line; nothing here needs bash or GNU tools, because the
# guest is Alpine and the host is macOS and the workload must be the same on both.
workload_noop='true'
workload_stat='ls pkg00 >/dev/null'
workload_read='grep -rl benchmark . >/dev/null'
workload_write='mkdir -p out && i=0; while [ $i -lt 200 ]; do echo x > out/$i; i=$((i+1)); done; rm -rf out'
workload_cpu='i=0; while [ $i -lt 50000 ]; do i=$((i+1)); done'
WORKLOADS="noop stat read write cpu"

# ---------------------------------------------------------------- contenders
# Each emits the argv that runs "$1" (a sh line) with the corpus as the working directory.
# The caller has already chdir'd into the corpus on the host.

# host: no sandbox at all. The denominator for every overhead claim in RESULTS.md.
argv_host() { printf '%s\0' /bin/sh -c "$1"; }

# seatbelt: macOS sandbox_init(3) via sandbox-exec. This is the mechanism behind both Codex's and
# Claude Code's native sandboxes on macOS: one process, no kernel boundary, path-based policy.
# The profile mirrors a workspace-write policy — read anywhere, write only under the corpus.
SBPROFILE="$WORK/workspace-write.sb"
argv_seatbelt() { printf '%s\0' /usr/bin/sandbox-exec -f "$SBPROFILE" /bin/sh -c "$1"; }

# codex: the Codex CLI's own shipped sandbox subcommand, seatbelt underneath.
argv_codex() { printf '%s\0' codex sandbox -- /bin/sh -c "$1"; }

# docker-run: a fresh container per command, which is how a per-tool-call container sandbox
# behaves. On macOS this is a Linux VM plus virtiofs, so it is the closest apples-to-apples
# comparison boxer has.
argv_docker_run() { printf '%s\0' docker run --rm -v "$CORPUS:/w" -w /w alpine /bin/sh -c "$1"; }

# docker: one long-lived container, exec per command. The fair warm comparison to boxer, which
# also keeps its sandbox alive between calls.
argv_docker() { printf '%s\0' docker exec -w /w "$DOCKER_CTR" /bin/sh -c "$1"; }

# smolvm: the platform underneath boxer, driven directly. The gap between this row and the boxer
# row is exactly what boxer's own code costs — config resolution, scope hashing, locks, hooks,
# the run record.
#
# It runs the real binary rather than the `smolvm` on PATH, which is a bash wrapper that sets a
# library path and execs this. That wrapper costs ~12ms per call and boxer now skips it too, so
# going through it here would leave the row measuring a shell process boxer does not start and
# would understate boxer's own overhead by about half. Someone typing `smolvm` by hand does pay
# it; this row is a floor, not a usage estimate. Falls back to the wrapper if the layout differs.
SMOLVM_DIRECT=$(command -v smolvm || true)
if [ -n "$SMOLVM_DIRECT" ]; then
  d=$(dirname "$(readlink -f "$SMOLVM_DIRECT" 2>/dev/null || echo "$SMOLVM_DIRECT")")
  if [ -x "$d/smolvm-bin" ] && [ -d "$d/lib" ]; then
    SMOLVM_DIRECT="$d/smolvm-bin"; export DYLD_LIBRARY_PATH="$d/lib${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"
  else
    SMOLVM_DIRECT=smolvm
  fi
fi
argv_smolvm() { printf '%s\0' "${SMOLVM_DIRECT:-smolvm}" machine exec --name "$SCOPE" -i -w /workspace -- /bin/sh -c "$1"; }

# boxer: the product, warm.
argv_boxer() { printf '%s\0' boxer run -c "$1"; }

CONTENDERS="host seatbelt codex docker_run docker smolvm boxer"
[ -n "${BENCH_ONLY:-}" ] && CONTENDERS=$(echo "$BENCH_ONLY" | tr ',' ' ')

cat > "$SBPROFILE" <<SB
(version 1)
(allow default)
(deny file-write*)
(allow file-write* (subpath "$CORPUS"))
(allow file-write* (subpath "/private/tmp"))
(allow file-write* (literal "/dev/null") (literal "/dev/stdout") (literal "/dev/stderr"))
SB

have() { command -v "$1" >/dev/null 2>&1; }
skip=""
for c in $CONTENDERS; do
  case $c in
    seatbelt) have sandbox-exec || skip="$skip $c";;
    codex)    have codex        || skip="$skip $c";;
    docker|docker_run) docker info >/dev/null 2>&1 || skip="$skip $c";;
    smolvm|boxer) have smolvm   || skip="$skip $c";;
  esac
done
for s in $skip; do CONTENDERS=$(echo "$CONTENDERS" | sed "s/\b$s\b//"); done
[ -n "$skip" ] && echo "# skipped (not installed on this host):$skip"

# ---------------------------------------------------------------- results
: > "$OUT"
HOSTINFO=$(python3 -c "
import json,platform,subprocess
def sh(c):
    try: return subprocess.check_output(c, shell=True, text=True).strip()
    except Exception: return ''
print(json.dumps({'os': platform.platform(), 'machine': platform.machine(),
                  'cpu': sh('sysctl -n machdep.cpu.brand_string') or platform.processor(),
                  'cores': sh('sysctl -n hw.ncpu'),
                  'memory_gb': sh(\"sysctl -n hw.memsize\") ,
                  'smolvm': sh('smolvm --version'), 'docker': sh(\"docker version --format '{{.Server.Version}}'\"),
                  'codex': sh('codex --version'), 'boxer': sh('boxer --version')}))")
echo "{\"kind\":\"host\",\"info\":$HOSTINFO}" >> "$OUT"
echo "# host: $(echo "$HOSTINFO" | python3 -c 'import json,sys; h=json.load(sys.stdin); print(h["cpu"], h["cores"]+" cores,", h["os"])')"
echo "# reps: $REPS per cell, 2 warmups discarded"

record() { # record <kind> <contender> <workload> <json>
  echo "{\"kind\":\"$1\",\"contender\":\"$2\",\"workload\":\"$3\",\"stats\":$4}" >> "$OUT"
}

time_one() { # time_one <reps> <warmups> <argv-emitting fn> <sh line>
  local reps=$1 warm=$2 fn=$3 line=$4
  "$fn" "$line" | xargs -0 python3 "$TIMEIT" "$reps" "$warm" --
}

# ---------------------------------------------------------------- cold start
# The first command in a sandbox that does not exist yet: what a developer waits through once per
# session, and what a per-call container sandbox pays every single call.
#
# "Cold" here means the *sandbox* is cold and the host is warm: boxer's image pack already exists,
# the container image is already pulled. That is what a developer hits day to day. It is stated
# rather than implied because the first version of this row left the pack state to whatever the
# previous run happened to leave behind, and the same row read 721ms one day and 17,132ms the
# next — the difference being a pack build, not anything about boxer. The genuinely-first-time
# cost, pack build included, is the `first-ever` row in bench/devserver.sh.
echo "# cold start, warm host (one sample each; provisioning is too variable to average cheaply)"
cold() { # cold <name> <teardown> <argv fn>
  eval "$2" >/dev/null 2>&1 || true
  local ms
  ms=$(time_one 1 0 "$3" "$workload_noop")
  record cold "$1" noop "$ms"
  printf '  %-12s %s ms\n' "$1" "$(echo "$ms" | python3 -c 'import json,sys; print(json.load(sys.stdin)["p50"])')"
}

# Build the pack before timing anything, so the boxer row measures a machine create and boot and
# not a pack build. `boxer up` then `boxer down` is the cheapest way to guarantee it exists.
if echo "$CONTENDERS" | grep -qw boxer; then
  boxer up >/dev/null 2>&1 || true
  boxer down >/dev/null 2>&1 || true
fi

for c in $CONTENDERS; do
  case $c in
    boxer)      cold boxer 'boxer down' argv_boxer;;
    docker_run) cold docker_run 'true' argv_docker_run;;
    docker)     cold docker "docker rm -f $DOCKER_CTR; docker run -d --name $DOCKER_CTR -v $CORPUS:/w -w /w alpine sleep 3600" argv_docker;;
    host|seatbelt|codex) cold "$c" 'true' "argv_$c";;
    smolvm)     ;; # measured by the boxer row: the same provisioning, one process further down
  esac
done

# ---------------------------------------------------------------- warm matrix
# Everything is warm from here: boxer's sandbox is up, Docker's container is up, and the
# process-level contenders have nothing to warm.
boxer run -- true >/dev/null 2>&1 || true
# The key, not the slug: `doctor` prints `swift-crab (sb-7e1852e4a3c3)` and `machine exec`
# only answers to the second one. Taking $2 here got a slug, every sample failed in 20ms, and
# the table would have published "raw smolvm is 4x faster than boxer" — which is why the
# failure count below is fatal rather than a footnote.
SCOPE=$(boxer status --json 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)["scope"])')
docker inspect "$DOCKER_CTR" >/dev/null 2>&1 || \
  docker run -d --name "$DOCKER_CTR" -v "$CORPUS:/w" -w /w alpine sleep 3600 >/dev/null 2>&1 || true

broken=""
echo "# warm (p50 ms)"
printf '  %-12s' workload; for c in $CONTENDERS; do printf ' %10s' "$c"; done; echo
for w in $WORKLOADS; do
  eval "line=\$workload_$w"
  printf '  %-12s' "$w"
  for c in $CONTENDERS; do
    s=$(time_one "$REPS" 2 "argv_$c" "$line")
    record warm "$c" "$w" "$s"
    printf ' %10s' "$(echo "$s" | python3 -c 'import json,sys
d=json.load(sys.stdin)
print(("%.1f" % d["p50"]) if not d["failures"] else "ERR(%d)" % d["failures"])')"
    if [ "$(echo "$s" | python3 -c 'import json,sys; print(json.load(sys.stdin)["failures"])')" != "0" ]; then
      broken="$broken $c/$w"
    fi
  done
  echo
done

# ---------------------------------------------------------------- boxer-only paths
# Not a comparison: nobody else has these. They are here because they are on the critical path of
# every agent tool call boxer intercepts, and a regression in them is invisible in the rows above.
echo "# boxer interception paths (p50 ms)"
if echo "$CONTENDERS" | grep -qw boxer; then
  HOOKIN="{\"hook_event_name\":\"PreToolUse\",\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"npm test\"},\"cwd\":\"$CORPUS\",\"session_id\":\"s\"}"
  for pair in "hook-rewrite:boxer hook claude-code" "brief:boxer brief" "status:boxer status --json"; do
    name=${pair%%:*}; cmd=${pair#*:}
    s=$(python3 - "$REPS" "$HOOKIN" $cmd <<'PY'
import json, subprocess, sys, time
reps, payload, argv = int(sys.argv[1]), sys.argv[2], sys.argv[3:]
stdin = payload if argv[1:2] == ["hook"] else ""
xs = []
for i in range(reps + 2):
    t = time.monotonic()
    subprocess.run(argv, input=stdin, text=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if i >= 2:
        xs.append((time.monotonic() - t) * 1000)
xs.sort()
print(json.dumps({"n": len(xs), "failures": 0, "min": round(xs[0], 1),
                  "p50": round(xs[len(xs)//2], 1), "p90": round(xs[int(.9*(len(xs)-1))], 1),
                  "max": round(xs[-1], 1)}))
PY
)
    record path boxer "$name" "$s"
    printf '  %-12s %s\n' "$name" "$(echo "$s" | python3 -c 'import json,sys; print(json.load(sys.stdin)["p50"])')"
  done
fi

if [ -n "$broken" ]; then
  echo
  echo "FAILED: the command did not succeed in these cells, so their timings measure an error path"
  echo "  $broken"
  exit 1
fi

echo
echo "wrote $OUT"
echo "render with: python3 bench/report.py $OUT > bench/RESULTS.md"
