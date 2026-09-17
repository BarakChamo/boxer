package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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
	for _, needle := range []string{"usage limit", "usage_limit", "quota", "rate limit", "rate-limited", "rate_limit", "too many requests", "free tier", "insufficient_quota", "credit balance", "no_providers_available"} {
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

// budgetUSD is the most one t2 run may spend on the gateway; BOXER_EVAL_BUDGET_USD overrides the
// default of $2, which fits four runs into a $10 day.
func budgetUSD() float64 {
	if v, err := strconv.ParseFloat(os.Getenv("BOXER_EVAL_BUDGET_USD"), 64); err == nil && v > 0 {
		return v
	}
	return 2
}

// gatewayUsed returns the gateway's lifetime spend for this key in USD (GET /v1/credits
// total_used), or -1 when the key is absent or the endpoint fails. Reading it before and after a
// cell attributes spend to that cell without parsing each harness's usage output.
func gatewayUsed() float64 {
	key, why := gatewayKey()
	if why != "" {
		return -1
	}
	req, _ := http.NewRequest("GET", gatewayRoot+"/v1/credits", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	var body struct {
		TotalUsed string `json:"total_used"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&body) != nil {
		return -1
	}
	v, err := strconv.ParseFloat(body.TotalUsed, 64)
	if err != nil {
		return -1
	}
	return v
}
