package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Conductor writes `.conductor/settings.toml`, the file Conductor's local Mac app reads for a
// repository (with `settings.local.toml` beside it and `~/.conductor/settings.{toml,managed.toml}`
// above it). The overrides that matter to boxer are the per-harness executable paths: Conductor
// spawns the binary they name, so pointing them at the shims `boxer shim install --harness` writes
// puts the harness itself inside the sandbox. `scripts.setup` warms the sandbox as the workspace is
// created, so the harness does not start against a cold VM.
//
// `conductor.json` is the legacy name and is not written.
func Conductor(root, shimDir string) (Result, error) {
	if shimDir == "" {
		return Result{}, fmt.Errorf("conductor: no shim directory; run `boxer shim install --harness claude,codex,opencode <dir>` first")
	}
	path := filepath.Join(root, ".conductor", "settings.toml")
	q := func(bin string) string { return fmt.Sprintf("%q", filepath.Join(shimDir, bin)) }
	block := strings.Join([]string{
		conductorStart,
		"claude_code_executable_path = " + q("claude"),
		"codex_executable_path = " + q("codex"),
		"opencode_executable_path = " + q("opencode"),
		"",
		"[environment_variables]",
		"BOXER_HARNESS_SHIMS = " + fmt.Sprintf("%q", shimDir),
		"",
		"[scripts]",
		`setup = "boxer up --detach && boxer doctor"`,
		`run = "boxer run -- bash"`,
		conductorEnd,
		"",
	}, "\n")
	r := &Result{}
	if err := r.replaceBlock(path, conductorStart, conductorEnd, block); err != nil {
		return *r, err
	}
	r.Notes = append(r.Notes,
		"Conductor spawns the executable each *_executable_path names: those are boxer's harness shims, so the harness runs in the sandbox (`boxer shell <harness>`).",
		"Conductor's public API drives cloud workspaces only; a local workspace is still verified by hand (docs/orchestrators.md).")
	return *r, nil
}

const (
	conductorStart = "# >>> boxer (managed by `boxer install conductor`; do not edit inside)"
	conductorEnd   = "# <<< boxer"
)

// ConductorInstalled reports whether root's Conductor settings carry boxer's block.
func ConductorInstalled(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, ".conductor", "settings.toml"))
	return err == nil && strings.Contains(string(b), conductorStart)
}
