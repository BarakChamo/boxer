#!/usr/bin/env bash
# Fail when a package's statement coverage drops below its floor in .coverfloor.
# Usage: scripts/coverfloor.sh coverage.out [.coverfloor]
set -euo pipefail

profile=${1:-coverage.out}
floors=${2:-.coverfloor}
[ -f "$profile" ] || { echo "coverage profile $profile not found"; exit 2; }

# Statement coverage per package, straight from the profile. Each line after the mode header is
# "file:start.col,end.col numStatements count"; a package is covered statements over all statements.
actual=$(awk '
  NR == 1 && /^mode:/ { next }
  {
    split($1, a, ":"); path = a[1]
    sub(/\/[^\/]+$/, "", path)
    sub(/^github\.com\/[^\/]+\/[^\/]+\//, "", path)
    total[path] += $2
    if ($3 > 0) covered[path] += $2
  }
  END { for (p in total) printf "%s %.1f\n", p, total[p] ? 100 * covered[p] / total[p] : 0 }
' "$profile")

fail=0
while read -r pkg floor; do
  case "$pkg" in ''|\#*) continue ;; esac
  got=$(echo "$actual" | awk -v p="$pkg" '$1 == p { print $2 }')
  if [ -z "$got" ]; then
    echo "  ?    $pkg has no coverage data"
    continue
  fi
  if awk -v g="$got" -v f="$floor" 'BEGIN { exit !(g + 0 < f + 0) }'; then
    echo "  FAIL $pkg $got% is below its floor of $floor%"
    fail=1
  else
    echo "  ok   $pkg $got% (floor $floor%)"
  fi
done < "$floors"

[ "$fail" = 0 ] || { echo; echo "Coverage regressed. Add tests, or lower the floor deliberately and say why."; exit 1; }
echo
echo "All packages meet their coverage floor."
