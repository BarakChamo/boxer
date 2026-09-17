#!/usr/bin/env bash
# Live evaluation: install the rendered bundle into each harness present on this machine, run a
# headless session that must execute a shell command, and assert the command ran in the guest
# (Linux, not the host OS). This is requirements §12's zero-error criterion, measured.
#
# Usage: evals/harness.sh [claude|codex|gemini|opencode|grok ...]   (default: whatever is installed)
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
export PATH="$ROOT/bin:$HOME/.local/bin:$PATH"
export XDG_CONFIG_HOME
XDG_CONFIG_HOME=$(mktemp -d)
WORK=$(mktemp -d)
DIST="$WORK/dist"
PROMPT='Run the shell command `uname -a` exactly once and then reply with only the first word of its output.'
pass=0; fail=0; skipped=0

mkrepo() { # mkrepo <dir> <mode>
  mkdir -p "$1" && git -C "$1" init -q && git -C "$1" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
  # uname is intercepted so the eval can prove where the command ran: Linux means the guest.
  cat > "$1/boxer.toml" <<EOF
image = "alpine"
memory = "1G"
cpus = 2
require_worktree = "off"
mode = "$2"
intercept = ["uname", "npm", "node", "python3", "go", "make"]
EOF
}

# judge <name> <trace> <result-word> <tool-calls> <expect-deny 0|1>
judge() {
  local name=$1 trace=$2 result=$3 tools=$4 expectdeny=$5
  local denies; denies=$(grep -c '"permissionDecision":"deny"' "$trace" 2>/dev/null || true)
  if [ "$result" != "Linux" ]; then
    echo "  FAIL $name: final answer was '$result', wanted Linux (guest)"; fail=$((fail+1)); return
  fi
  if [ "${denies:-0}" != 0 ]; then
    echo "  FAIL $name: $denies denial(s) — the agent had to be corrected"; fail=$((fail+1)); return
  fi
  # Tool mode: the agent must have taken a boxer path on its own, either the boxer_run tool or
  # the shell form `boxer run` the instruction names. Both are zero-error outcomes.
  # Only hook *inputs* (`<-`) show what the agent typed; outputs (`->`) are boxer's own rewrites.
  local typed_boxer=0; grep -q '<- .*"command":"boxer run' "$trace" && typed_boxer=1
  if [ "$expectdeny" = 1 ] && ! echo "$tools" | grep -q boxer_run && [ "$typed_boxer" = 0 ]; then
    echo "  FAIL $name: tool mode but neither boxer_run nor \`boxer run\` was used (tools: $tools)"; fail=$((fail+1)); return
  fi
  local via="rewrite hook"
  echo "$tools" | grep -q boxer_run && via="boxer_run tool"
  [ "$typed_boxer" = 1 ] && via="agent typed boxer run itself"
  echo "  ok   $name: ran in guest via $via; tools=[$tools] denies=0"; pass=$((pass+1))
}

# parse_stream reads Claude-style stream-json on stdin, prints "<result>|<tool names>".
parse_stream() {
  python3 -c '
import sys, json
tools, result = [], ""
for line in sys.stdin:
    try: e = json.loads(line)
    except Exception: continue
    if e.get("type") == "assistant":
        for c in e["message"]["content"]:
            if c.get("type") == "tool_use": tools.append(c["name"])
    elif e.get("type") == "result": result = (e.get("result") or "").strip().split()[0] if e.get("result") else ""
print(result + "|" + ",".join(tools))
'
}

want=("$@"); [ ${#want[@]} -eq 0 ] && want=(claude codex gemini opencode grok kimi)
(cd "$ROOT" && boxer package all --out "$DIST" >/dev/null)

for h in "${want[@]}"; do
  if ! command -v "$h" >/dev/null 2>&1; then echo "  skip $h: not installed"; skipped=$((skipped+1)); continue; fi
  echo "# $h"
  case $h in
    claude)
      if ! claude plugin validate "$DIST/claude-code" >/dev/null 2>&1; then
        echo "  FAIL claude: plugin manifest invalid"; claude plugin validate "$DIST/claude-code"; fail=$((fail+1)); continue
      fi
      for mode in rewrite tool; do
        R="$WORK/claude-$mode"; mkrepo "$R" "$mode"; T="$R/trace.log"
        out=$(cd "$R" && BOXER_TRACE="$T" env -u CLAUDECODE claude -p "$PROMPT" --plugin-dir "$DIST/claude-code" \
              --permission-mode bypassPermissions --output-format stream-json --verbose --max-turns 8 2>/dev/null | parse_stream)
        judge "claude ($mode mode)" "$T" "${out%%|*}" "${out#*|}" "$([ "$mode" = tool ] && echo 1 || echo 0)"
        (cd "$R" && boxer down >/dev/null 2>&1) || true
      done ;;
    codex)
      R="$WORK/codex"; mkrepo "$R" rewrite; T="$R/trace.log"
      (cd "$R" && boxer install codex >/dev/null)
      # Project hooks need trust (bypass flag is the documented non-interactive path), and Codex's
      # own seatbelt sandbox must be off: boxer replaces it, a hypervisor cannot run inside it.
      (cd "$R" && BOXER_TRACE="$T" codex exec --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust \
          --skip-git-repo-check -o "$R/last.txt" "$PROMPT" >"$R/codex.out" 2>&1)
      if grep -q "usage limit" "$R/codex.out"; then
        echo "  skip codex: hooks fired (see trace) but the ChatGPT usage limit stopped the turn"; skipped=$((skipped+1))
        (cd "$R" && boxer down >/dev/null 2>&1) || true; continue
      fi
      out=$(awk '{print $1; exit}' "$R/last.txt" 2>/dev/null)
      judge codex "$T" "$out" "" 0
      (cd "$R" && boxer down >/dev/null 2>&1) || true ;;
    gemini)
      R="$WORK/gemini"; mkrepo "$R" rewrite; T="$R/trace.log"
      gemini extensions uninstall boxer >/dev/null 2>&1 || true
      (cd "$R" && yes 2>/dev/null | gemini extensions install --consent "$DIST/gemini-cli" >/dev/null 2>&1) || true
      if ! gemini extensions list 2>&1 | grep -q boxer; then   # list prints to stderr
        echo "  FAIL gemini: extension did not install"; fail=$((fail+1)); continue
      fi
      if [ ! -f "$HOME/.gemini/oauth_creds.json" ] && [ -z "${GEMINI_API_KEY:-}" ] && [ -z "${GOOGLE_API_KEY:-}" ]; then
        echo "  skip gemini: extension installs, but no login or GEMINI_API_KEY for a live turn"; skipped=$((skipped+1))
        gemini extensions uninstall boxer >/dev/null 2>&1 || true; continue
      fi
      out=$(cd "$R" && BOXER_TRACE="$T" yes | gemini -p "$PROMPT" --yolo 2>/dev/null | tail -1 | awk '{print $1}')
      gemini extensions uninstall boxer >/dev/null 2>&1 || true
      judge gemini "$T" "$out" "" 0
      (cd "$R" && boxer down >/dev/null 2>&1) || true ;;
    opencode)
      R="$WORK/opencode"; mkrepo "$R" rewrite; T="$R/trace.log"
      (cd "$R" && boxer install opencode >/dev/null)
      if opencode auth list 2>/dev/null | grep -q "0 credentials" && [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ]; then
        echo "  skip opencode: plugin installed, but no provider credentials for a live turn (opencode auth login)"; skipped=$((skipped+1)); continue
      fi
      out=$(cd "$R" && BOXER_TRACE="$T" opencode run "$PROMPT" 2>/dev/null | tail -1 | awk '{print $1}')
      judge opencode "$T" "$out" "" 0
      (cd "$R" && boxer down >/dev/null 2>&1) || true ;;
    kimi)
      # Kimi hooks are user-level TOML and block-only; a live run needs a Kimi login in
      # $KIMI_CODE_HOME plus the hooks snippet appended there. Validate what can be validated.
      R="$WORK/kimi"; mkrepo "$R" rewrite
      (cd "$R" && boxer install kimi >/dev/null) && [ -f "$R/.kimi-code/mcp.json" ] && python3 -m json.tool "$R/.kimi-code/mcp.json" >/dev/null \
        && echo "  ok   kimi: project mcp.json and skill installed (live turn needs a Kimi login; skipped)" && skipped=$((skipped+1)) \
        || { echo "  FAIL kimi: install failed"; fail=$((fail+1)); } ;;
    grok)
      R="$WORK/grok"; mkrepo "$R" rewrite; T="$R/trace.log"
      if ! grok plugin validate "$DIST/grok" >/dev/null 2>&1; then
        echo "  FAIL grok: plugin manifest invalid"; grok plugin validate "$DIST/grok"; fail=$((fail+1)); continue
      fi
      grok plugin install "$DIST/grok" >/dev/null 2>&1 || true
      (cd "$R" && BOXER_TRACE="$T" grok -p "$PROMPT" --permission-mode bypassPermissions >"$R/grok.out" 2>&1) || true
      grok plugin uninstall boxer >/dev/null 2>&1 || true
      if grep -qiE "not signed in|log ?in|api key|unauthori[sz]ed|not authenticated|credential" "$R/grok.out" || [ ! -s "$R/grok.out" ]; then
        echo "  skip grok: plugin validates and installs, but no XAI_API_KEY or login for a live turn"; skipped=$((skipped+1))
        (cd "$R" && boxer down >/dev/null 2>&1) || true; continue
      fi
      out=$(tail -1 "$R/grok.out" | awk '{print $1}')
      judge grok "$T" "$out" "" 0
      (cd "$R" && boxer down >/dev/null 2>&1) || true ;;
  esac
done

echo
echo "passed $pass, failed $fail, skipped $skipped"
[ "$fail" = 0 ]
