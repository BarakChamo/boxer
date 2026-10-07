package box

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BarakChamo/boxer/internal/config"
)

// trustDir holds one file per scope naming the host-affecting configuration a person approved.
// The key is the scope, because trust is per worktree: approving one does not approve its forks or
// another checkout of the same repository.
func trustPath(scopeKey string) string {
	return filepath.Join(stateRoot(), "trust", scopeKey)
}

// trusted reports whether digest is the host-affecting configuration last approved for this scope.
func trusted(scopeKey, digest string) bool {
	b, err := os.ReadFile(trustPath(scopeKey))
	return err == nil && strings.TrimSpace(string(b)) == digest
}

// Trust records digest as approved for the scope, so its host-affecting keys take effect.
func Trust(scopeKey, digest string) error {
	p := trustPath(scopeKey)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(digest+"\n"), 0o644)
}

// trustAllEnv, when set, trusts every configuration without a record: for CI and the test harness,
// where there is no person to approve one and the repository is the operator's own.
const trustAllEnv = "BOXER_TRUST"

// applyTrust holds back a scope's host-affecting configuration until a person has approved it.
// Code in the guest can rewrite the worktree's boxer.toml and devcontainer.json, which boxer reads
// on the host; a changed set of host-affecting keys is not used until `boxer trust` approves it.
// It returns the configuration to use and, when keys were held back, a warning naming them.
func applyTrust(scopeKey string, cfg config.Config) (config.Config, string) {
	fb, changed := cfg.SafeFallback()
	// A configuration that only tightens the sandbox changes nothing here, so it is never held
	// back: it cannot be used against the host.
	if len(changed) == 0 {
		return cfg, ""
	}
	if os.Getenv(trustAllEnv) == "1" || trusted(scopeKey, cfg.HostDigest()) {
		return cfg, ""
	}
	return fb,
		"this worktree's configuration sets keys that reach the host or loosen the sandbox (" + strings.Join(changed, ", ") +
			"); they are held back until you approve this configuration with `boxer trust`. " +
			"The sandbox still runs, no weaker than boxer's defaults. Set BOXER_TRUST=1 to approve every configuration (CI)."
}
