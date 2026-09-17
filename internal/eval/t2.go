package eval

import (
	"fmt"
	"os"
	"os/exec"
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

// openAICompatible is the provider OpenCode and pi use live: the Vercel AI Gateway when its key is
// present, else OpenAI directly.
type openAICompatible struct {
	BaseURL string
	KeyVar  string
	Model   string // as the provider names it
}

func gateway() (openAICompatible, string) {
	if os.Getenv("AI_GATEWAY_API_KEY") != "" {
		return openAICompatible{"https://ai-gateway.vercel.sh/v1", "AI_GATEWAY_API_KEY", "anthropic/claude-sonnet-4.5"}, ""
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return openAICompatible{"https://api.openai.com/v1", "OPENAI_API_KEY", "gpt-4.1"}, ""
	}
	return openAICompatible{}, "AI_GATEWAY_API_KEY or OPENAI_API_KEY is not set"
}

// moonshot is Kimi's own platform, reached with MOONSHOT_API_KEY through Kimi's native provider type.
const moonshotBaseURL = "https://api.moonshot.ai/v1"
const moonshotModel = "kimi-k2.5"

// claudeLoggedIn reports whether Claude Code has a login on this machine (macOS Keychain item) or
// an API key in the environment.
func claudeLoggedIn() bool {
	if _, ok := anySet("ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"); ok {
		return true
	}
	return exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials").Run() == nil
}

// quotaError recognises a live turn stopped by the provider rather than by boxer.
func quotaError(out string) string {
	for _, needle := range []string{"usage limit", "usage_limit", "quota", "rate limit", "insufficient_quota", "credit balance"} {
		if i := strings.Index(strings.ToLower(out), needle); i >= 0 {
			start := i - 80
			if start < 0 {
				start = 0
			}
			end := i + 120
			if end > len(out) {
				end = len(out)
			}
			return fmt.Sprintf("provider stopped the turn: %s", strings.TrimSpace(strings.Join(strings.Fields(out[start:end]), " ")))
		}
	}
	return ""
}
