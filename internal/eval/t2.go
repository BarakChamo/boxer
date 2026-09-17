package eval

import (
	"fmt"
	"os"
	"strings"
)

// Tier t2 runs the same cells against real providers. Credentials come from the process
// environment (cmd/boxer-eval loads evals/.env first); a driver whose credential is absent skips
// and names the variable. Values are never printed.

// SkipError from Prepare or Run marks a cell that cannot be judged for a reason outside boxer:
// a missing credential, an exhausted quota, a login that does not travel.
type SkipError struct{ Reason string }

func (e SkipError) Error() string { return e.Reason }

// missing names the first unset variable of the alternatives, or "" when one is set.
func anySet(vars ...string) (string, bool) {
	for _, v := range vars {
		if os.Getenv(v) != "" {
			return v, true
		}
	}
	return "", false
}

func needOne(vars ...string) string {
	if _, ok := anySet(vars...); ok {
		return ""
	}
	return strings.Join(vars, " or ") + " is not set"
}

// One credential runs every live cell: AI_GATEWAY_API_KEY for the Vercel AI Gateway, which
// speaks Anthropic Messages (Claude Code, Kimi, pi), OpenAI Responses (Codex) and OpenAI Chat
// Completions (OpenCode, Grok, OpenHands). Gemini CLI speaks only the Gemini API, which the
// gateway does not serve, so it stays on GEMINI_API_KEY. Documented per agent at
// vercel.com/docs/ai-gateway/coding-agents (read 2026-09-17).
const (
	gatewayKeyVar = "AI_GATEWAY_API_KEY"
	gatewayRoot   = "https://ai-gateway.vercel.sh"                 // Anthropic Messages: SDKs append /v1/messages
	gatewayOpenAI = "https://ai-gateway.vercel.sh/coding-agent/v1" // Chat Completions surface for coding agents
	gatewayClaude = "https://ai-gateway.vercel.sh/claude-code"     // Claude Code's own endpoint (no /v1)
	gatewayCodex  = "https://ai-gateway.vercel.sh/codex/v1"        // Codex's own endpoint (Responses)
)

// liveModels are the cheapest tool-calling models per harness family (gateway prices per million
// tokens on 2026-09-17: haiku 4.5 $1/$5, gpt-5-mini $0.25/$2, kimi-k2.5 $0.6/$3, grok-4.1-fast
// $0.2/$0.5). BOXER_EVAL_MODEL overrides all of them with one gateway model id.
var liveModels = map[string]string{
	"claude":    "anthropic/claude-haiku-4.5",
	"codex":     "openai/gpt-5-mini",
	"opencode":  "anthropic/claude-haiku-4.5",
	"pi":        "anthropic/claude-haiku-4.5",
	"kimi":      "moonshotai/kimi-k2.5",
	"grok":      "spacexai/grok-4.1-fast-non-reasoning",
	"openhands": "openai/gpt-5-mini",
}

// LiveModel is the gateway model id a harness runs at t2.
func LiveModel(h string) string {
	if m := os.Getenv("BOXER_EVAL_MODEL"); m != "" {
		return m
	}
	return liveModels[h]
}

// gatewayKey returns the key, or the skip reason when it is absent.
func gatewayKey() (string, string) {
	k := os.Getenv(gatewayKeyVar)
	if k == "" {
		return "", gatewayKeyVar + " is not set"
	}
	return k, ""
}

// openAICompatible is a Chat Completions provider block for one harness at t2.
type openAICompatible struct {
	BaseURL string
	KeyVar  string
	Model   string
}

func gateway(h string) (openAICompatible, string) {
	if _, why := gatewayKey(); why != "" {
		return openAICompatible{}, why
	}
	return openAICompatible{BaseURL: gatewayOpenAI, KeyVar: gatewayKeyVar, Model: LiveModel(h)}, ""
}

// quotaError recognises a live turn stopped by the provider rather than by boxer.
func quotaError(out string) string {
	for _, needle := range []string{"usage limit", "usage_limit", "quota", "rate limit", "insufficient_quota", "credit balance"} {
		if i := strings.Index(strings.ToLower(out), needle); i >= 0 {
			start := i
			end := i + 160
			if end > len(out) {
				end = len(out)
			}
			return fmt.Sprintf("provider stopped the turn: %s", strings.TrimSpace(strings.Join(strings.Fields(out[start:end]), " ")))
		}
	}
	return ""
}
