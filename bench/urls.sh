#!/usr/bin/env bash
# What [urls] costs: `boxer up` on a running sandbox, and a warm `boxer run`, with it off and on.
#
# Two linked worktrees of one repository, identical except for `[urls] enabled`, each serving one
# port. Both are brought up once, then the two are sampled in interleaved pairs (off, on, off, on)
# so that drift on the host lands on both arms rather than on whichever ran second.
#
# `boxer up` on a running sandbox is the call every hook and orchestrator makes before work; with
# [urls] on it also takes the portless lock and waits for the proxy and the route. A warm run is
# the call an agent makes for every command.
#
# Needs smolvm and portless. Removes both sandboxes and the scratch repository when it exits.
#
# Usage: bench/urls.sh [pairs]     (default 5)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
BOXER=$ROOT/bin/boxer
PAIRS=${1:-5}
OUT=${BENCH_OUT:-"$HERE/urls.jsonl"}
command -v portless >/dev/null || { echo "portless is not installed: npm install -g portless" >&2; exit 1; }

WORK=$(mktemp -d)
cleanup() {
  for w in off on; do (cd "$WORK/$w" 2>/dev/null && "$BOXER" down >/dev/null 2>&1); done
  git -C "$WORK/base" worktree prune >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

git init -q "$WORK/base"
cat > "$WORK/base/boxer.toml" <<'EOF'
image = "mirror.gcr.io/library/node:24-alpine"
start = ["node -e \"require('http').createServer((q, r) => r.end('ok')).listen(3000, '0.0.0.0')\""]
ready = "wget -q -O /dev/null http://127.0.0.1:3000/"
[network]
ports = ["auto:3000"]
EOF
git -C "$WORK/base" add -A
git -C "$WORK/base" -c user.email=bench@local -c user.name=bench commit -qm base
for w in off on; do git -C "$WORK/base" worktree add -q -b "urls-$w" "$WORK/$w"; done
printf '\n[urls]\nenabled = false\n' >> "$WORK/off/boxer.toml"
printf '\n[urls]\nenabled = true\n' >> "$WORK/on/boxer.toml"

for w in off on; do (cd "$WORK/$w" && "$BOXER" up >/dev/null) || { echo "boxer up failed in $w" >&2; exit 1; }; done

ms() { python3 -c 'import subprocess,sys,time
t=time.monotonic(); rc=subprocess.run(sys.argv[1:],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode
print("%.1f %d" % ((time.monotonic()-t)*1000, rc))' "$@"; }

for i in $(seq 1 "$PAIRS"); do
  for w in off on; do
    for what in up run; do
      if [ "$what" = up ]; then argv=("$BOXER" up); else argv=("$BOXER" run -c true); fi
      read -r t rc < <(cd "$WORK/$w" && ms "${argv[@]}")
      printf '{"urls":"%s","call":"%s","pair":%d,"ms":%s,"rc":%d}\n' "$w" "$what" "$i" "$t" "$rc" | tee -a "$OUT"
    done
  done
done
