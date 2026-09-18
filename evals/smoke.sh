#!/usr/bin/env bash
# Real-smolvm smoke test of every configuration path, no LLM involved.
# Usage: evals/smoke.sh [path-to-boxer-binary]
set -euo pipefail

BOXER=${1:-"$(cd "$(dirname "$0")/.." && pwd)/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
export XDG_CONFIG_HOME
# The automatic sweep is asynchronous and would race the gc checks below, reaping an orphan before
# the explicit `boxer gc` sees it. The sweep has its own check at the end of this file.
export BOXER_NO_RECLAIM=1
XDG_CONFIG_HOME=$(mktemp -d)
WORK=$(mktemp -d)
# one real-smolvm user at a time on this host: re-exec under boxer-eval's host lock
EVAL="$(dirname "$BOXER")/boxer-eval"
if [ -z "${BOXER_EVAL_LOCKED:-}" ] && [ -x "$EVAL" ]; then exec "$EVAL" --lock-run -- "$0" "$BOXER"; fi
pass=0; fail=0
ok()   { pass=$((pass+1)); echo "  ok   $1"; }
bad()  { fail=$((fail+1)); echo "  FAIL $1"; }
check(){ if eval "$2"; then ok "$1"; else bad "$1"; fi; }
cleanup() { boxer down --all >/dev/null 2>&1 || true; }
trap cleanup EXIT

mkrepo() { # mkrepo <dir> <toml>
  mkdir -p "$1" && git -C "$1" init -q && git -C "$1" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
  printf '%s\n' "$2" > "$1/boxer.toml"
}
BASE='image = "alpine"
memory = "1G"
cpus = 2
require_worktree = "off"'

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
check "egress blocked by default allowlist" '[ "$(boxer run -c "wget -q -T 3 -O- http://example.com >/dev/null 2>&1 && echo LEAK || echo blocked")" = "blocked" ]'
check "warm run under 1s"        '[ "$( { /usr/bin/time -p boxer run -- true; } 2>&1 | awk "/real/{print (\$2 < 1.0)}")" = "1" ]'

echo "# isolation = repo shares one VM across worktrees"
mkrepo "$WORK/r" "$BASE
isolation = \"repo\""
git -C "$WORK/r" worktree add -q "$WORK/r-wt" -b wt
k1=$(cd "$WORK/r" && boxer doctor | awk '/^scope:/{print $2}'); k2=$(cd "$WORK/r-wt" && boxer doctor | awk '/^scope:/{print $2}')
check "same scope key" '[ "$k1" = "$k2" ]'

echo "# isolation = worktree separates them"
mkrepo "$WORK/w" "$BASE"
git -C "$WORK/w" worktree add -q "$WORK/w-wt" -b wt
k1=$(cd "$WORK/w" && boxer doctor | awk '/^scope:/{print $2}'); k2=$(cd "$WORK/w-wt" && boxer doctor | awk '/^scope:/{print $2}')
check "different scope keys" '[ "$k1" != "$k2" ]'

echo "# isolation = session / subagent via identity flags, with degradation"
mkrepo "$WORK/s" "$BASE
isolation = \"subagent\""
cd "$WORK/s"
k1=$(boxer doctor --session s1 --agent a1 | awk '/^scope:/{print $2}')
k2=$(boxer doctor --session s1 --agent a2 | awk '/^scope:/{print $2}')
k3=$(boxer doctor --session s1 | awk '/^scope:/{print $2}')
check "agents differ"          '[ "$k1" != "$k2" ]'
check "degrades to session"    '[ "$k3" != "$k1" ] && boxer doctor --session s1 | grep -q "degraded"'
check "on_missing_id = fail refuses" '! BOXER_ON_MISSING_ID=fail boxer run --session s1 -- true 2>/dev/null'

echo "# require_worktree"
mkrepo "$WORK/q" "image = \"alpine\"
memory = \"1G\"
cpus = 2
require_worktree = \"require\""
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
check "session start instructs + provisions" 'printf "{\"hook_event_name\":\"SessionStart\",\"source\":\"startup\",\"cwd\":\"%s\",\"session_id\":\"s\"}" "$PWD" | boxer hook claude-code | grep -q additionalContext && boxer ls | grep -q "$(boxer doctor | awk "/^scope:/{print \$2}")"'
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
echo "passed $pass, failed $fail"
[ "$fail" = 0 ]
