package install

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
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
	// Two blocks, because TOML puts every key after a [table] header inside that table: the
	// top-level keys go before the user's first table, boxer's own tables after everything. One
	// block appended to the end put the executable paths inside whatever table the user's file
	// ended with, where Conductor never looks.
	top := strings.Join([]string{
		conductorStart,
		"claude_code_executable_path = " + q("claude"),
		"codex_executable_path = " + q("codex"),
		// Conductor documents claude_code_executable_path and codex_executable_path as repository
		// settings; OpenCode's path is set in the app's own preferences and this key is not in the
		// published table. It is written anyway because an unknown key is inert.
		"opencode_executable_path = " + q("opencode"),
		conductorEnd,
		"",
	}, "\n")
	tables := strings.Join([]string{
		conductorTablesStart,
		"[environment_variables]",
		"BOXER_HARNESS_SHIMS = " + fmt.Sprintf("%q", shimDir),
		"",
		"[scripts]",
		`setup = "boxer up --detach && boxer doctor"`,
		`run = "boxer run -- bash"`,
		conductorTablesEnd,
		"",
	}, "\n")
	cur, _ := os.ReadFile(path)
	user := stripBlock(stripBlock(string(cur), conductorTablesStart, conductorTablesEnd), conductorStart, conductorEnd)
	user = strings.Trim(user, "\n")
	for _, t := range []string{"environment_variables", "scripts"} {
		if regexp.MustCompile(`(?m)^\s*\[` + t + `\]`).MatchString(user) {
			return Result{}, fmt.Errorf("%s already has a [%s] table; add boxer's lines to it yourself:\n%s", path, t, tables)
		}
	}
	// The user's own top-level keys stay above their first table, after boxer's.
	head, rest := user, ""
	if loc := regexp.MustCompile(`(?m)^\s*\[`).FindStringIndex(user); loc != nil {
		head, rest = user[:loc[0]], user[loc[0]:]
	}
	out := top
	if h := strings.Trim(head, "\n"); h != "" {
		out += h + "\n"
	}
	if r := strings.Trim(rest, "\n"); r != "" {
		out += "\n" + r + "\n"
	}
	out += "\n" + tables
	var probe map[string]any
	if _, err := toml.Decode(out, &probe); err != nil {
		return Result{}, fmt.Errorf("%s: adding boxer's settings would not be valid TOML (%v); add them yourself:\n%s%s", path, err, top, tables)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return Result{}, err
	}
	r := &Result{Written: []string{path}}
	r.Notes = append(r.Notes,
		"Conductor spawns the executable each *_executable_path names: those are boxer's harness shims, so the harness runs in the sandbox (`boxer shell <harness>`).",
		"Conductor's public API drives cloud workspaces only; a local workspace is still verified by hand (docs/orchestrators.md).")
	return *r, nil
}

const (
	conductorStart       = "# >>> boxer (managed by `boxer install conductor`; do not edit inside)"
	conductorEnd         = "# <<< boxer"
	conductorTablesStart = "# >>> boxer tables (managed by `boxer install conductor`; do not edit inside)"
	conductorTablesEnd   = "# <<< boxer tables"
)

// ConductorInstalled reports whether root's Conductor settings carry boxer's block.
func ConductorInstalled(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, ".conductor", "settings.toml"))
	return err == nil && strings.Contains(string(b), conductorStart)
}
