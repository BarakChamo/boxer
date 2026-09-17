#!/usr/bin/env bash
# Real-smolvm smoke test of every configuration path, no LLM involved.
# Usage: evals/smoke.sh [path-to-boxer-binary]
set -euo pipefail

BOXER=${1:-"$(cd "$(dirname "$0")/.." && pwd)/bin/boxer"}
export PATH="$(dirname "$BOXER"):$HOME/.local/bin:$PATH"
export XDG_CONFIG_HOME
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
setup = [\"echo ready > /tmp/marker\"]"
cd "$WORK/a"
check "lazy provision + exit code" '[ "$(boxer run -c "cat /tmp/marker; exit 4" 2>/dev/null; echo $?)" = "ready
4" ]'
check "stdin forwarded"          '[ "$(printf "a\nb\n" | boxer run -- wc -l | tr -d " ")" = "2" ]'
mkdir -p sub; check "subdir maps into mount"  '[ "$(cd sub && boxer run -c pwd)" = "/workspace/sub" ]'
check "guest write visible on host" 'boxer run -c "echo hi > g.txt" && [ "$(cat g.txt)" = "hi" ]'
check "setup ran once"           'boxer run -- true; [ "$(boxer run -c "cat /tmp/marker")" = "ready" ]'
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
check "one package plus eight views" '[ -f "$D/boxer/plugin.json" ] && [ -f "$D/boxer/mcp.json" ] && [ "$(ls "$D" | wc -l | tr -d " ")" = "9" ]'
check "views are subsets of the package" 'for h in claude-code codex grok gemini-cli kimi dsh opencode pi; do [ -f "$D/$h/plugin.json" ] || exit 1; done'
check "package has no root hooks dir" '[ ! -d "$D/boxer/hooks" ]'
check "all json valid" 'for f in $(find "$D" -name "*.json"); do python3 -m json.tool "$f" >/dev/null || exit 1; done'
check "claude bundle has shims" '[ -x "$D/claude-code/bin/npm" ]'
check "no view but claude mentions claude" '! grep -ril claude "$D" | grep -v "/claude-code/\|/boxer/" | grep -q .'

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
check "gc reaps orphan" 'boxer gc | grep -q deleted'

echo
echo "passed $pass, failed $fail"
[ "$fail" = 0 ]
