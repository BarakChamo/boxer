#!/usr/bin/env bash
# Real-backend smoke test of every configuration path, no LLM involved.
#
# Usage: evals/smoke.sh [path-to-boxer-binary]
#   BOXER_BACKEND=docker evals/smoke.sh    run the same cells on another backend
#
# Every cell runs against a real backend, never a fake. A backend that is not installed skips the
# whole suite with one line saying so, rather than passing vacuously — a green run has to mean
# something ran. A cell that needs a capability this backend lacks is skipped *by name*, so the
# output doubles as a test of the capability table: if `boxer doctor` says a backend can fork and
# the fork cell skips, one of the two is lying.
set -euo pipefail

BOXER=${1:-"$(cd "$(dirname "$0")/.." && pwd)/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
export XDG_CONFIG_HOME
# The automatic sweep is asynchronous and would race the gc checks below, reaping an orphan before
# the explicit `boxer gc` sees it. The sweep has its own check at the end of this file.
export BOXER_NO_RECLAIM=1
# The line below points XDG_CONFIG_HOME at an empty directory so boxer reads no user config. That
# is right for boxer and wrong for podman, which keeps its *connection* registry — which machine,
# which socket — under the same variable, in `containers/`. Isolating it makes the podman client
# forget its VM exists, and every call fails with "unable to connect to Podman socket".
#
# So podman's own subdirectory is linked through while boxer's stays isolated. CONTAINER_HOST is
# not enough on its own: podman 5 resolves the connection from that directory, not from the URL.
# The re-exec comes first, before anything is created. It used to come after, and `exec` replaces
# the process — so the first instance's scratch directories were orphaned with no trap to remove
# them, and every run of this suite leaked two. Nothing is allocated until the process that will
# actually live is the one running.
REAL_XDG_CONFIG=${XDG_CONFIG_HOME:-$HOME/.config}
# one real-backend user at a time on this host: re-exec under boxer-eval's host lock
EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then
  export REAL_XDG_CONFIG
  exec "$EVAL" --lock-run -- "$0" "$BOXER"
fi
XDG_CONFIG_HOME=$(mktemp -d)
if [ -d "$REAL_XDG_CONFIG/containers" ]; then
  ln -s "$REAL_XDG_CONFIG/containers" "$XDG_CONFIG_HOME/containers"
fi
WORK=$(mktemp -d)
pass=0; fail=0; skipped=0
ok()   { pass=$((pass+1)); echo "  ok   $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL $1"; }
skip() { skipped=$((skipped+1)); echo "  skip $1 — needs $2"; }
check(){ if eval "$2"; then ok "$1"; else bad "$1"; fi; }

# The backend under test, and what it can do. `boxer doctor --json` is the source, so these
# checks and the capability table cannot disagree without one of them failing.
BACKEND=${BOXER_BACKEND:-smolvm}
caps() { # caps <field>: true|false, from doctor's own report
  # `doctor` exits non-zero outside a repository while still printing a complete report, so its
  # status is captured and discarded rather than piped: under `pipefail` a non-zero left-hand side
  # would fail a pipeline whose output was already correct, and the `|| echo false` fallback then
  # printed a second line. caps() returning "true\nfalse" skipped every cell on a backend that
  # supports everything, which is how this was found.
  local json
  json=$( cd "$WORK" 2>/dev/null || cd /; boxer doctor --json 2>/dev/null ) || true
  printf '%s' "$json" |
    python3 -c "import sys,json;d=json.load(sys.stdin).get('backend') or {};print(str(d.get('$1',False)).lower())" 2>/dev/null || printf false
}
# needs <capability> <cell name>: skip the cell by name when the backend lacks it.
needs() { if [ "$(caps "$1")" = true ]; then return 0; fi; skip "$2" "$1"; return 1; }

case $BACKEND in
smolvm) probe=smolvm ;;
docker|podman|container) probe=$BACKEND ;;
*) echo "unknown backend $BACKEND"; exit 2 ;;
esac
if ! command -v "$probe" >/dev/null 2>&1; then
  echo "# $BACKEND is not installed; nothing was tested"
  exit 0
fi
echo "# backend: $BACKEND"
cleanup() {
  boxer down --all >/dev/null 2>&1 || true
  # Every scratch tree this run made, including the ones created mid-suite for shims and the
  # packaged plugin. They live under $TMPDIR, which macOS only prunes after three days, so a
  # suite that runs often leaves a pile nothing reclaims.
  rm -rf "$WORK" "$XDG_CONFIG_HOME" ${SH:+"$SH"} ${D:+"$D"} 2>/dev/null || true
}
trap cleanup EXIT

# `boxer doctor` prints the scope as `swift-crab (sb-7e1852e4a3c3)`: a slug for reading and the
# key beside it. These checks compare identities, so they must take the key — two different keys
# can share a slug (there are 4096 of them), and an inequality check that compares slugs passes
# by luck.
scopekey() { boxer doctor "$@" | sed -n 's/^scope:.*(\(sb-[0-9a-f]*\)).*/\1/p'; }

mkrepo() { # mkrepo <dir> <toml>
  mkdir -p "$1" && git -C "$1" init -q && git -C "$1" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
  printf '%s\n' "$2" > "$1/boxer.toml"
}
BASE='image = "alpine"
memory = "1G"
cpus = 2
require_worktree = "off"'
# boxer's default network mode is `allowlist`, which a backend without per-host egress policy
# refuses — correctly, and that refusal has its own cell. But every *other* cell then fails to
# provision for a reason that has nothing to do with what it is testing, so a backend that cannot
# do allowlist gets the stricter of the two modes it does have. "off" rather than "on": the suite
# should not quietly gain a network it was not asked for.
#
# Kept separate from BASE rather than folded into it, and written as an inline table rather than a
# [network] section, because callers compose these: a section header would capture a caller's
# later keys into the wrong table, and a second copy of BASE would duplicate keys TOML rejects.
NET=""
if [ "$(caps allowlist)" != true ]; then NET='
network = { mode = "off" }'; fi
BASE="$BASE$NET"

echo "# run, exit code, stdin, cwd, mount, setup-once"
mkrepo "$WORK/a" "$BASE
setup = [\"mkdir -p /var/lib/boxer-smoke && echo ready > /var/lib/boxer-smoke/marker\"]"
cd "$WORK/a"
# /var/lib, not /tmp: the guest's /tmp is tmpfs and empties on stop/start, and caching the
# environment stops the VM to snapshot it. Setup that writes to /tmp loses it.
check "lazy provision + exit code" '[ "$(boxer run -c "cat /var/lib/boxer-smoke/marker; exit 4" 2>/dev/null; echo $?)" = "ready
4" ]'
check "stdin forwarded"          '[ "$(printf "a\nb\n" | boxer run -- wc -l | tr -d " ")" = "2" ]'
mkdir -p sub; check "subdir maps into mount"  '[ "$(cd sub && boxer run -c pwd)" = "/workspace/sub" ]'
check "guest write visible on host" 'boxer run -c "echo hi > g.txt" && [ "$(cat g.txt)" = "hi" ]'
check "setup ran once"           'boxer run -- true; [ "$(boxer run -c "cat /var/lib/boxer-smoke/marker")" = "ready" ]'
needs allowlist "egress blocked by default allowlist" && check "egress blocked by default allowlist" '[ "$(boxer run -c "wget -q -T 3 -O- http://example.com >/dev/null 2>&1 && echo LEAK || echo blocked")" = "blocked" ]'
# Timed with python rather than /usr/bin/time, which Fedora CoreOS and other minimal Linux hosts do
# not ship: there this cell failed for a missing timer, not a slow boxer.
check "warm run under 1s"        '[ "$(python3 -c "import subprocess,time;t=time.monotonic();subprocess.run([\"boxer\",\"run\",\"--\",\"true\"],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);print(int(time.monotonic()-t<1.0))")" = 1 ]'

echo "# isolation = repo shares one VM across worktrees"
mkrepo "$WORK/r" "$BASE
isolation = \"repo\""
git -C "$WORK/r" worktree add -q "$WORK/r-wt" -b wt
k1=$(cd "$WORK/r" && scopekey); k2=$(cd "$WORK/r-wt" && scopekey)
check "same scope key" '[ "$k1" = "$k2" ]'

echo "# isolation = worktree separates them"
mkrepo "$WORK/w" "$BASE"
git -C "$WORK/w" worktree add -q "$WORK/w-wt" -b wt
k1=$(cd "$WORK/w" && scopekey); k2=$(cd "$WORK/w-wt" && scopekey)
check "different scope keys" '[ "$k1" != "$k2" ]'

echo "# isolation = session / subagent via identity flags, with degradation"
mkrepo "$WORK/s" "$BASE
isolation = \"subagent\""
cd "$WORK/s"
k1=$(scopekey --session s1 --agent a1)
k2=$(scopekey --session s1 --agent a2)
k3=$(scopekey --session s1)
check "agents differ"          '[ "$k1" != "$k2" ]'
check "degrades to session"    '[ "$k3" != "$k1" ] && boxer doctor --session s1 | grep -q "degraded"'
check "on_missing_id = fail refuses" '! BOXER_ON_MISSING_ID=fail boxer run --session s1 -- true 2>/dev/null'

echo "# require_worktree"
mkrepo "$WORK/q" "image = \"alpine\"
memory = \"1G\"
cpus = 2
require_worktree = \"require\"$NET"
check "main checkout refused"  '(cd "$WORK/q" && { boxer run -- true 2>&1 || true; } | grep -q WORKTREE_REQUIRED)'
git -C "$WORK/q" worktree add -q "$WORK/q-wt" -b wt
check "linked worktree accepted" '(cd "$WORK/q-wt" && boxer run -- true)'

echo "# create_on without run refuses; boxer up then works"
mkrepo "$WORK/c" "$BASE
create_on = [\"session_start\"]"
cd "$WORK/c"
check "NO_SANDBOX error contract" '{ boxer run -- true 2>&1 || true; } | grep -q "cause:     NO_SANDBOX"'
check "boxer up provisions"       'boxer up >/dev/null && boxer run -- true'

echo "# on_sandbox_unavailable = passthrough runs on host with warning"
mkrepo "$WORK/p" "$BASE
create_on = [\"session_start\"]
on_sandbox_unavailable = \"passthrough\""
cd "$WORK/p"
check "host fallback" '[ "$(boxer run -- uname -s 2>/dev/null)" = "$(uname -s)" ]'

echo "# hooks: rewrite / tool / off / audit, every dialect"
mkrepo "$WORK/h" "$BASE"
cd "$WORK/h"
pre() { printf '{"hook_event_name":"%s","tool_name":"%s","tool_input":{"command":"%s"},"cwd":"%s","session_id":"s"}' "$1" "$2" "$3" "$PWD"; }
check "claude rewrite"  '[ "$(pre PreToolUse Bash "npm test" | boxer hook claude-code)" = "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"allow\",\"updatedInput\":{\"command\":\"boxer run -c '"'"'npm test'"'"'\"}}}" ]'
check "codex rewrite"   'pre PreToolUse Bash "go test" | boxer hook codex | grep -q updatedInput'
check "grok rewrite"    'pre PreToolUse run_terminal_command "go test" | boxer hook grok | grep -q updatedInput'
check "gemini rewrite"  'pre BeforeTool run_shell_command "go test" | boxer hook gemini-cli | grep -q "\"tool_input\":{\"command\":\"boxer run"'
check "opencode rewrite" '[ "$(pre tool.execute.before bash "go test" | boxer hook opencode)" = "{\"command\":\"boxer run -c '"'"'go test'"'"'\"}" ]'
check "dsh allows (shims cover)" '[ -z "$(pre PreToolUse Bash "go test" | boxer hook dsh)" ]'
check "passthrough silent"  '[ -z "$(pre PreToolUse Bash "git status" | boxer hook claude-code)" ]'
check "tool mode denies with fix" 'BOXER_MODE=tool bash -c "$(declare -f pre); pre PreToolUse Bash \"npm test\" | boxer hook claude-code" | grep -q "\"permissionDecision\":\"deny\".*fix: boxer run -c"'
check "gemini tool mode top-level deny" 'BOXER_MODE=tool bash -c "$(declare -f pre); pre BeforeTool run_shell_command \"npm test\" | boxer hook gemini-cli" | grep -q "^{\"decision\":\"deny\""'
check "off mode silent"   '[ -z "$(BOXER_MODE=off bash -c "$(declare -f pre); pre PreToolUse Bash \"npm test\" | boxer hook claude-code")" ]'
check "audit logs, allows" '[ -z "$(BOXER_ENFORCEMENT=audit bash -c "$(declare -f pre); pre PreToolUse Bash \"npm test\" | boxer hook claude-code" 2>/dev/null)" ]'
check "session start instructs + provisions" 'printf "{\"hook_event_name\":\"SessionStart\",\"source\":\"startup\",\"cwd\":\"%s\",\"session_id\":\"s\"}" "$PWD" | boxer hook claude-code | grep -q additionalContext && boxer ls | grep -q "$(scopekey)"'
check "hook survives bad json"   'echo "nope" | boxer hook claude-code; [ $? = 0 ]'

echo "# mcp"
check "boxer_run over MCP" 'printf "%s\n" "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"boxer_run\",\"arguments\":{\"command\":\"uname -s\",\"cwd\":\"$PWD\"}}}" | boxer mcp | grep -q "Linux"'

echo "# shims"
mkrepo "$WORK/m" "$BASE
intercept = [\"uname\", \"npm\"]"
cd "$WORK/m"
SH=$(mktemp -d); boxer shim install "$SH" >/dev/null
check "shim execs boxer run" 'grep -q "exec boxer run -- npm" "$SH/npm"'
check "bare command via shim runs in guest" '[ "$(PATH="$SH:$PATH" uname -s 2>/dev/null)" = "Linux" ]'

echo "# package"
D=$(mktemp -d); boxer package all --out "$D" >/dev/null
check "one package plus nine views" '[ -f "$D/boxer/plugin.json" ] && [ -f "$D/boxer/mcp.json" ] && [ "$(ls "$D" | wc -l | tr -d " ")" = "10" ]'
check "views are subsets of the package" 'for h in claude-code codex grok gemini-cli kimi dsh opencode pi copilot; do [ -f "$D/$h/plugin.json" ] || exit 1; done'
check "package has no root hooks dir" '[ ! -d "$D/boxer/hooks" ]'
check "all json valid" 'for f in $(find "$D" -name "*.json"); do python3 -m json.tool "$f" >/dev/null || exit 1; done'
# Published content is static: shims are written by `boxer shim install` into the machine that
# runs the harness, never rendered into a package someone else installs.
check "package carries no shims" '! find "$D" -type d -name bin | grep -q .'
# DSH's only hook mechanism is the dsh-hooks-claude-code bridge, so its view names the package.
check "no view but claude mentions claude" '! grep -ril claude "$D" | grep -v "/claude-code/\|/boxer/\|/com.deepseek.dsh/" | grep -q .'

echo "# install (project layer)"
mkrepo "$WORK/i" "$BASE"
cd "$WORK/i"
printf '{"permissions":{"allow":["Read"]}}\n' > .claude.json.keep
mkdir -p .claude && printf '{"permissions":{"allow":["Read"]}}\n' > .claude/settings.json
check "install all writes project files" 'boxer install all >/dev/null && [ -f .codex/hooks.json ] && [ -f .gemini/settings.json ] && [ -f .opencode/plugins/boxer.ts ] && [ -f .claude/skills/boxer/SKILL.md ]'
check "existing settings preserved"     'python3 -c "import json;d=json.load(open(\".claude/settings.json\"));assert d[\"permissions\"][\"allow\"]==[\"Read\"];assert \"PreToolUse\" in d[\"hooks\"]"'
check "install is idempotent"          'boxer install claude-code >/dev/null; python3 -c "import json;d=json.load(open(\".claude/settings.json\"));assert len(d[\"hooks\"][\"PreToolUse\"])==1"'
check "project hook rewrites"          'pre PreToolUse Bash "npm test" | boxer hook claude-code | grep -q updatedInput'
check "inside guest hook is silent"    '[ -z "$(pre PreToolUse Bash "npm test" | BOXER_INSIDE=1 boxer hook claude-code)" ]'
check "inside guest run executes directly" '[ "$(BOXER_INSIDE=1 boxer run -- uname -s)" = "$(uname -s)" ]'

echo "# results, capsules, named packs, forks"
mkrepo "$WORK/rc" "$BASE"
cd "$WORK/rc"
# The report lives in the worktree, which is mounted, so the host writes it once and the guest
# only has to copy it into place — no XML to quote through two shells.
printf '%s' '<testsuite name="cart"><testcase name="ok"/><testcase name="bad"><failure/></testcase></testsuite>' > "$WORK/rc/red.xml"
check "junit summary names the failing test" '
  boxer run --junit junit.xml -c "cat red.xml > junit.xml" > "$WORK/junit.out" 2>&1
  grep -q "FAIL cart/bad" "$WORK/junit.out"'
check "fail-on-test-failures reddens a green command" '
  ! boxer run --junit junit.xml --fail-on-test-failures -c "cat red.xml > junit.xml" >/dev/null 2>&1'
# The one check the fake could never make: it was fed boxer own guess at smolvm egress schema,
# agreed with it, and against a real guest the feature reported nothing at all. The default
# network mode is allowlist and the list is empty but for the image registry, so an ordinary
# fetch is refused and the denial has to come back from the real `machine egress-events`.
needs egress "a failed run names the host the allowlist refused" && check "a failed run names the host the allowlist refused" '
  boxer run -c "wget -T 3 -q -O- https://registry.npmjs.org/ >/dev/null" > "$WORK/denied.out" 2>&1 || true
  grep -q "allowlist refused registry.npmjs.org" "$WORK/denied.out"'
needs egress "doctor lists the denial too" && check "doctor lists the denial too" '
  boxer doctor > "$WORK/denied-doctor.out" 2>&1
  grep -q "^denied:.*registry.npmjs.org" "$WORK/denied-doctor.out"'
# /dev/null is a character device, and boxer once took it for a terminal: it asked the runtime for
# a TTY with none behind it, the runtime refused before the command ran, and that refusal came
# back as the command's exit status. Every CI job and script that discards output is this shape.
check "exit code survives discarded output" '[ "$(boxer run -c "exit 4" </dev/null >/dev/null 2>&1; echo $?)" = 4 ]'
check "a run leaves a record" 'boxer run -c "exit 4" >/dev/null 2>&1; boxer status --json | grep -q last_run'
check "capsule replays the failure" '
  git add -A && git -c user.email=t@t -c user.name=t commit -q -m c
  # stdin and stdout on /dev/null, deliberately: that is the shape that failed. Capturing stdout
  # to look at it is what used to make this pass. stderr is not part of the TTY decision.
  boxer run -c "exit 4" </dev/null >/dev/null 2>"$WORK/capsule-run.out"; echo "run exit $?" >> "$WORK/capsule-run.out"
  boxer capsule new >/dev/null
  boxer capsule replay capsule.toml > "$WORK/replay.out" 2>&1
  # On failure, the evidence: this cell has been red without a diagnosis before, because every
  # attempt to observe the run from outside made it pass.
  grep -q reproduced "$WORK/replay.out" || { sed "s/^/       /" "$WORK/capsule-run.out" "$WORK/replay.out" capsule.toml; false; }'
needs packs "named pack survives gc --all" && check "named pack survives gc --all" '
  boxer pack save smoke-base >/dev/null && boxer gc --all >/dev/null 2>&1
  boxer pack ls | grep -q smoke-base'
needs packs "a sandbox from a named pack runs" && check "a sandbox from a named pack runs" '
  boxer pack use smoke-base >/dev/null && [ "$(boxer run -- uname -s)" = Linux ]'
# Not `boxer fork | grep -q`: grep exits at the first match, boxer dies of SIGPIPE, and pipefail
# then fails a check that actually passed. Same trap as gc below.
needs branch "fork refuses before preparation" && check "fork refuses before preparation" '
  boxer fork > "$WORK/fork.out" 2>&1 || true
  grep -q NOT_FORKABLE "$WORK/fork.out"'
needs branch "a fork runs in the guest" && check "a fork runs in the guest" '
  boxer fork --prepare --json > "$WORK/fork1.json" 2>/dev/null
  child=$(python3 -c "import json;print(json.load(open(\"$WORK/fork1.json\"))[0])")
  # An empty name would fall back to this worktree own sandbox and pass for the wrong reason.
  [ -n "$child" ] && [ "$(boxer run --scope "$child" -- uname -s)" = Linux ]'
needs branch "a second fork is a second child" && check "a second fork is a second child" '
  boxer fork --count 2 --json > "$WORK/fork2.json" 2>/dev/null
  # --count is how many children there should be, so asking for two when one exists makes one.
  [ "$(python3 -c "import json;print(len(json.load(open(\"$WORK/fork2.json\"))))")" -ge 1 ]
  [ "$(boxer fork ls --json | python3 -c "import json,sys;print(len(json.load(sys.stdin)))")" = 2 ]'
needs branch "fork rm --all reclaims them" && check "fork rm --all reclaims them" 'boxer fork rm --all >/dev/null 2>&1; [ -z "$(boxer fork ls --json | tr -d "[] \n")" ]'
boxer pack rm smoke-base >/dev/null 2>&1 || true
boxer down >/dev/null 2>&1 || true

echo "# gc"
mkrepo "$WORK/g" "$BASE"; (cd "$WORK/g" && boxer up >/dev/null); rm -rf "$WORK/g"
# Not `boxer gc | grep -q deleted`: grep -q exits at the first match, gc dies of SIGPIPE writing
# its summary line, and pipefail then fails a check that actually passed.
check "gc reaps orphan" 'boxer gc > "$WORK/gc.out" 2>&1; grep -q deleted "$WORK/gc.out"'
check "gc reports what it freed" 'boxer gc --all --dry-run | grep -q "would reclaim"'
check "doctor reports the footprint" 'cd "$WORK/p2" 2>/dev/null || mkrepo "$WORK/p2" "$BASE"; cd "$WORK/p2"; boxer doctor | grep -q "^storage:"'

# The sweep itself, with the guard lifted: provisioning stamps the state directory and reaps the
# orphan without anyone asking. This is what keeps a host from filling up.
echo "# automatic reclaim"
mkrepo "$WORK/r" "$BASE"; (cd "$WORK/r" && boxer up >/dev/null); rm -rf "$WORK/r"
mkrepo "$WORK/r2" "$BASE"
check "provisioning sweeps in the background" '
  stamp="${XDG_STATE_HOME:-$HOME/.local/state}/boxer/last-reclaim"; rm -f "$stamp"
  (cd "$WORK/r2" && BOXER_NO_RECLAIM= boxer up >/dev/null)
  for i in $(seq 1 50); do [ -f "$stamp" ] && break; sleep 0.2; done
  [ -f "$stamp" ]'
check "the sweep reaped the orphan" '
  for i in $(seq 1 50); do boxer ls --json | grep -q "$WORK/r\"" || break; sleep 0.2; done
  ! boxer ls --json | grep -q "$WORK/r\""'

STATE="${XDG_STATE_HOME:-$HOME/.local/state}/boxer"

# What a devcontainer asks for and boxer now honours: the specification's variables resolved on
# the host, remoteUser as the user commands run as, and forwardPorts as a per-worktree port.
echo "# devcontainer"
mkrepo "$WORK/dc" "$BASE"
mkdir -p "$WORK/dc/.devcontainer" "$WORK/dc/data"; echo from-host > "$WORK/dc/data/f"
cat > "$WORK/dc/.devcontainer/devcontainer.json" <<'JSON'
{
  // comments and variables, as real files have them
  "image": "alpine",
  "remoteUser": "guest",
  "mounts": ["source=${localWorkspaceFolder}/data,target=/data,type=bind"],
  "remoteEnv": { "FROM_HOST": "${localEnv:BOXER_SMOKE_VAR:unset}" },
  "forwardPorts": [3000]
}
JSON
cd "$WORK/dc"
check "devcontainer: remoteUser runs the commands" '[ "$(BOXER_SMOKE_VAR=x boxer run -- whoami 2>/dev/null)" = "guest" ]'
# Not just "runs as": writes as. smolvm mounts the worktree owned by the host uid, unmapped, so a
# non-root guest user could read the tree and write nothing — `npm install` as `node` failed on
# its first file. boxer now gives the user the host's uid there.
check "devcontainer: remoteUser can write the worktree" 'boxer run -c "echo w > by-user.txt" >/dev/null 2>&1 && [ "$(cat by-user.txt 2>/dev/null)" = w ]'
# The uid change edits /etc/passwd with sed, and a generator once turned its \1 backreference into
# a control character: the uid was right and the password field was garbage.
check "devcontainer: the user's passwd entry is intact" '[ "$(boxer run -c "grep ^guest: /etc/passwd | cut -d: -f2" 2>/dev/null)" = x ]'
check "devcontainer: \${localEnv} resolves"       '[ "$(BOXER_SMOKE_VAR=x boxer run -c "echo \$FROM_HOST" 2>/dev/null)" = "x" ]'
check "devcontainer: \${localWorkspaceFolder} mounts" '[ "$(boxer run -- cat /data/f 2>/dev/null)" = "from-host" ]'
check "devcontainer: forwardPorts is per worktree" 'boxer brief --json | grep -q "auto:3000"'
K=$(scopekey)
boxer down >/dev/null 2>&1 || true
# An orchestrator reusing a task name removes the worktree and adds a new one at the same path. The
# new one must be set up; it used to inherit the old one's marker and start with no dependencies.
mkrepo "$WORK/re" "$BASE
setup = [\"echo done > setup-ran.txt\"]"
git -C "$WORK/re" worktree add -q -b t1 "$WORK/re-wt" HEAD 2>/dev/null
cp "$WORK/re/boxer.toml" "$WORK/re-wt/"
(cd "$WORK/re-wt" && boxer up > "$WORK/re-up1.out" 2>&1; echo "up1 rc=$?" >> "$WORK/re-up1.out"; boxer down >> "$WORK/re-up1.out" 2>&1)
git -C "$WORK/re" worktree remove --force "$WORK/re-wt" >> "$WORK/re-up1.out" 2>&1
git -C "$WORK/re" worktree add -q -b t2 "$WORK/re-wt" HEAD >> "$WORK/re-up1.out" 2>&1
cp "$WORK/re/boxer.toml" "$WORK/re-wt/"
check "a worktree recreated at the same path is set up again" '(cd "$WORK/re-wt" && boxer up > "$WORK/re-up2.out" 2>&1) && [ -f "$WORK/re-wt/setup-ran.txt" ] || { sed "s/^/       /" "$WORK/re-up1.out" "$WORK/re-up2.out"; ls -la "$WORK/re-wt" | sed "s/^/       /"; false; }'
(cd "$WORK/re-wt" && boxer down >/dev/null 2>&1) || true
# Machine state goes with the sandbox; the setup marker, which describes the worktree, does not.
check "down removes the sandbox's host state" '[ ! -e "$STATE/runs/$K.json" ] && [ ! -e "$STATE/last-used/$K" ] && [ ! -e "$STATE/locks/$K" ]'

# Three worktrees of one repository, each serving the same guest port, each at its own name. The
# worktrees are the three shapes that matter: the main checkout, a branch, and a detached HEAD —
# the last is what orchestrators create, and the one portless alone gives no name of its own.
echo "# urls (portless)"
if ! command -v portless >/dev/null 2>&1; then
  skip "three worktrees, three URLs" "portless on PATH"
else
  U="$WORK/u"
  # node rather than busybox: alpine's busybox is built without the httpd applet.
  UT='image = "mirror.gcr.io/library/node:24-alpine"
memory = "1G"
cpus = 2
require_worktree = "off"
network = { mode = "on", ports = ["auto:3000"] }
start = ["node -e \"require('"'"'http'"'"').createServer((q,s)=>s.end(require('"'"'fs'"'"').readFileSync('"'"'www/index.html'"'"'))).listen(3000,'"'"'0.0.0.0'"'"')\""]
ready = "wget -q -O- http://127.0.0.1:3000/ >/dev/null"
[urls]
enabled = true
name = "boxersmoke"'
  mkrepo "$U" "$UT"; git -C "$U" add -A; git -C "$U" -c user.email=t@t -c user.name=t commit -qm toml
  git -C "$U" worktree add -q -b smoke-branch "$WORK/u-branch" HEAD
  git -C "$U" worktree add -q --detach "$WORK/u-detached" HEAD
  for d in "$U" "$WORK/u-branch" "$WORK/u-detached"; do mkdir -p "$d/www"; basename "$d" > "$d/www/index.html"; done
  # Together, as an orchestrator would start them.
  for d in "$U" "$WORK/u-branch" "$WORK/u-detached"; do (cd "$d" && boxer up >/dev/null 2>&1) & done; wait
  url() { (cd "$1" && boxer status --json | python3 -c "import sys,json;print((json.load(sys.stdin).get('urls') or {}).get('3000',''))"); }
  U1=$(url "$U"); U2=$(url "$WORK/u-branch"); U3=$(url "$WORK/u-detached")
  echo "  ..   $U1 · $U2 · $U3"
  check "urls: every worktree has one"          '[ -n "$U1" ] && [ -n "$U2" ] && [ -n "$U3" ]'
  check "urls: all three are different"          '[ "$U1" != "$U2" ] && [ "$U2" != "$U3" ] && [ "$U1" != "$U3" ]'
  check "urls: a branch is named for its branch" 'case "$U2" in *smoke-branch.boxersmoke.*) true;; *) false;; esac'
  check "urls: each serves its own worktree"     '[ "$(curl -sk --max-time 10 "$U1/")" = u ] && [ "$(curl -sk --max-time 10 "$U2/")" = u-branch ] && [ "$(curl -sk --max-time 10 "$U3/")" = u-detached ]'
  check "urls: the brief sends the agent to them" '(cd "$U" && boxer brief) | grep -q "\`urls\`"'
  # A harness may hand boxer a different XDG_STATE_HOME from the one that registered the name —
  # one stripped it from the MCP server it launched, and status reported the port. Both status and
  # down must work from a state directory that has never heard of the route.
  OTHER="$WORK/other-state"; mkdir -p "$OTHER"
  check "urls: status finds it from another state dir" '[ "$(cd "$WORK/u-branch" && XDG_STATE_HOME="$OTHER" boxer status --json | python3 -c "import sys,json;print((json.load(sys.stdin).get(\"urls\") or {}).get(\"3000\",\"\"))")" = "$U2" ]'
  (cd "$WORK/u-branch" && XDG_STATE_HOME="$OTHER" boxer down >/dev/null 2>&1)
  check "urls: down removes only its own route"  '[ "$(curl -sk --max-time 10 -o /dev/null -w "%{http_code}" "$U2/")" != 200 ] && [ "$(curl -sk --max-time 10 "$U1/")" = u ]'
  boxer down --all >/dev/null 2>&1 || true
  check "urls: down --all leaves no route behind" '! portless list 2>/dev/null | grep -q boxersmoke && [ -z "$(ls "$STATE/urls" 2>/dev/null)" ]'
fi

# The management commands against the real backend: a probe that proves it runs a sandbox, a
# listing that says which backend and what state the worktree is in, and removal by name.
echo "# cli"
check "backends --probe runs a real sandbox" 'boxer backends "$BACKEND" --probe --json > "$WORK/probe.json" 2>&1 && python3 -c "import json,sys;r=json.load(open(sys.argv[1]))[0];sys.exit(0 if r[\"probe\"][\"ok\"] else 1)" "$WORK/probe.json"'
mkrepo "$WORK/cl" "$BASE"; (cd "$WORK/cl" && boxer up >/dev/null 2>&1); echo x > "$WORK/cl/dirt"
check "ls says backend and worktree state" 'boxer ls --json | python3 -c "import json,sys;import os;r=[x for x in json.load(sys.stdin) if os.path.realpath(x[\"worktree\"])==os.path.realpath(sys.argv[1])][0];sys.exit(0 if r[\"backend\"]==sys.argv[2] and r[\"git\"][\"dirty\"] else 1)" "$WORK/cl" "$BACKEND"'
CLK=$(cd "$WORK/cl" && scopekey)
check "rm by name removes it" 'boxer rm "$CLK" >/dev/null 2>&1 && ! boxer ls --json | grep -q "$CLK"'

echo "# performance (numbers printed; the bounds are generous, they catch a regression, not jitter)"
mkrepo "$WORK/p" "$BASE"
cd "$WORK/p"
ms() { python3 -c "import subprocess,time,sys; t=time.monotonic(); subprocess.run(sys.argv[1:], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); print(int((time.monotonic()-t)*1000))" "$@"; }
COLD=$(ms boxer up)
WARM=$(ms boxer run -- true)
for i in 1 2 3; do W=$(ms boxer run -- true); [ "$W" -lt "$WARM" ] && WARM=$W; done
HOOK=$(python3 -c "
import subprocess,time,os,sys
payload='{\"hook_event_name\":\"PreToolUse\",\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"npm test\"},\"cwd\":\"%s\",\"session_id\":\"s\"}' % os.getcwd()
t=time.monotonic()
subprocess.run(['boxer','hook','claude-code'], input=payload, text=True, stdout=subprocess.DEVNULL)
print(int((time.monotonic()-t)*1000))")
echo "  ..   cold start ${COLD}ms · warm run ${WARM}ms · rewrite hook ${HOOK}ms"
check "cold start under 120s"  '[ "$COLD" -lt 120000 ]'
check "warm run under 1500ms"  '[ "$WARM" -lt 1500 ]'
check "rewrite hook under 500ms" '[ "$HOOK" -lt 500 ]'
boxer down >/dev/null 2>&1 || true

echo
if [ "$skipped" -gt 0 ]; then echo "passed $pass, failed $fail, skipped $skipped (capabilities this backend lacks)"; else
echo "passed $pass, failed $fail"; fi
# BOXER_SMOKE_RECORD=<file> appends this run as one JSON line. The published tables are checked
# against the latest line per backend and host (TestPublishedNumbersMatchTheEvidence), so a number
# in the docs is a number a run produced rather than one somebody typed.
if [ -n "${BOXER_SMOKE_RECORD:-}" ]; then
  printf '{"backend":"%s","host":"%s","passed":%d,"failed":%d,"skipped":%d,"version":"%s","date":"%s"}\n' \
    "$BACKEND" "$(uname -s | tr A-Z a-z)" "$pass" "$fail" "$skipped" "$(boxer version 2>/dev/null | awk '{print $2}')" "$(date -u +%Y-%m-%d)" >> "$BOXER_SMOKE_RECORD"
fi
[ "$fail" = 0 ]
