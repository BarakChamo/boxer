// Package install places boxer's hooks, run tool, and instruction into a repository's own
// harness configuration. A user-level plugin is invisible to a harness that an orchestrator
// launches with its own config directory (T3 Code sets CLAUDE_CONFIG_DIR; Paperclip manages
// CLAUDE_CONFIG_DIR and CODEX_HOME), so the project layer is the one every launcher reads.
//
// Installing both the plugin and the project layer is safe: the second hook sees a command that
// already begins with `boxer run` and allows it, provisioning is idempotent, and a boxer inside
// the guest sees BOXER_INSIDE and does nothing.
package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BarakChamo/boxer/internal/bundle"
	"github.com/BarakChamo/boxer/internal/config"
)

// Result reports what Install did. A write failure is kept here rather than returned at every
// call site: the file writers are a long list of one-liners, and threading an error through each
// of them buried the shape of the installer. The first failure stops the later writes and is
// returned by Install, which is what matters — `boxer install` used to report success having
// written nothing at all.
type Result struct {
	Written []string
	Notes   []string
	err     error
}

// keep records the first failure.
func (r *Result) keep(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Install writes harness's project-level configuration under root.
func Install(harness string, cfg config.Config, version, root string) (Result, error) {
	tmp, err := os.MkdirTemp("", "boxer-install-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp)
	if _, err := bundle.Render(harness, version, tmp); err != nil {
		return Result{}, err
	}
	r := &Result{}
	toolMode := cfg.Mode == "tool"
	ns, hooksFile, hooks, skill, agents, server := parts(tmp, harness)
	switch harness {
	case "claude-code":
		if err := r.mergeJSON(filepath.Join(root, ".claude", "settings.json"), func(m map[string]any) {
			mergeHooks(m, hooks["hooks"])
		}); err != nil {
			return *r, err
		}
		if err := r.mergeJSON(filepath.Join(root, ".mcp.json"), func(m map[string]any) {
			setIn(m, "mcpServers", "boxer", server)
		}); err != nil {
			return *r, err
		}
		r.copyTree(skill, filepath.Join(root, ".claude", "skills", "boxer"))
		r.copy(filepath.Join(tmp, "agents", "boxed.md"), filepath.Join(root, ".claude", "agents", "boxed.md"))
		r.Notes = append(r.Notes, "PATH shims are not part of project settings; run `boxer shim install` where the agent's shell starts.")
	case "codex":
		r.copy(hooksFile, filepath.Join(root, ".codex", "hooks.json"))
		r.copyTree(skill, filepath.Join(root, ".agents", "skills", "boxer"))
		r.Notes = append(r.Notes,
			"Codex loads project hooks only after they are trusted: run /hooks once, or pass --dangerously-bypass-hook-trust to `codex exec`.",
			"Add the run tool to .codex/config.toml:\n  [mcp_servers.boxer]\n  command = \"boxer\"\n  args = [\"mcp\", \"--harness\", \"codex\"]")
	case "gemini-cli":
		ext := readJSON(filepath.Join(tmp, "gemini-extension.json"))
		if err := r.mergeJSON(filepath.Join(root, ".gemini", "settings.json"), func(m map[string]any) {
			mergeHooks(m, hooks["hooks"])
			if servers, ok := ext["mcpServers"].(map[string]any); ok {
				setIn(m, "mcpServers", "boxer", servers["boxer"])
			}
			if toolMode {
				tools, _ := m["tools"].(map[string]any)
				if tools == nil {
					tools = map[string]any{}
				}
				tools["exclude"] = appendOnce(toStrings(tools["exclude"]), "run_shell_command")
				m["tools"] = tools
			}
		}); err != nil {
			return *r, err
		}
		r.appendSection(filepath.Join(root, "GEMINI.md"), agents)
	case "opencode":
		r.copy(filepath.Join(ns, "plugins", "boxer.ts"), filepath.Join(root, ".opencode", "plugins", "boxer.ts"))
		oc := readJSON(filepath.Join(ns, "opencode.json"))
		if err := r.mergeJSON(filepath.Join(root, "opencode.json"), func(m map[string]any) {
			if _, ok := m["$schema"]; !ok {
				m["$schema"] = oc["$schema"]
			}
			if mcp, ok := oc["mcp"].(map[string]any); ok {
				setIn(m, "mcp", "boxer", mcp["boxer"])
			}
		}); err != nil {
			return *r, err
		}
		r.appendSection(filepath.Join(root, "AGENTS.md"), agents)
	case "grok":
		if err := r.mergeJSON(filepath.Join(root, ".grok", "hooks", "boxer.json"), func(m map[string]any) {
			mergeHooks(m, hooks["hooks"])
		}); err != nil {
			return *r, err
		}
		if err := r.mergeJSON(filepath.Join(root, ".mcp.json"), func(m map[string]any) {
			setIn(m, "mcpServers", "boxer", server)
		}); err != nil {
			return *r, err
		}
		r.copyTree(skill, filepath.Join(root, ".agents", "skills", "boxer"))
		r.Notes = append(r.Notes, "Grok Build runs project hooks only after the folder is trusted: launch with --trust once, or set GROK_FOLDER_TRUST=0 for headless runs.")
	case "pi":
		r.copy(filepath.Join(ns, "extensions", "boxer.ts"), filepath.Join(root, ".pi", "extensions", "boxer.ts"))
		r.appendSection(filepath.Join(root, "AGENTS.md"), agents)
		r.Notes = append(r.Notes, "pi loads project extensions after the project is trusted; for a one-off run use `pi -e .pi/extensions/boxer.ts`.")
	case "kimi":
		if err := r.mergeJSON(filepath.Join(root, ".kimi-code", "mcp.json"), func(m map[string]any) {
			setIn(m, "mcpServers", "boxer", server)
		}); err != nil {
			return *r, err
		}
		r.copyTree(skill, filepath.Join(root, ".agents", "skills", "boxer"))
		toml, _ := os.ReadFile(filepath.Join(ns, "hooks.toml"))
		r.Notes = append(r.Notes,
			"Kimi hooks live in the user config only; append this to ~/.kimi-code/config.toml:\n"+strings.TrimSpace(string(toml)),
			"Kimi cannot rewrite tool input and its Bash tool ignores PATH shims: set [harness.kimi] mode = \"tool\" in boxer.toml so the hook denies shell use and boxer_run is the way in.")
	case "dsh":
		r.copy(filepath.Join(ns, "cordis.patch.yml"), filepath.Join(root, ".dsh", "cordis.patch.yml"))
		r.copy(hooksFile, filepath.Join(root, ".dsh", "hooks.json"))
		r.copyTree(skill, filepath.Join(root, ".agents", "skills", "boxer"))
		r.Notes = append(r.Notes,
			"DSH has no project-level plugin config: boot it with the patch layer, `dsh --profile headless --patch .dsh/cordis.patch.yml \"<task>\"`, or copy those rows into $DSH_HOME/cordis.patch.yml to apply them to every profile.",
			"The patch mounts @deepseek-ai/dsh-hooks-claude-code over .dsh/hooks.json; that bridge honours deny and ask but ignores updatedInput, so DSH cannot rewrite a command. Set [harness.dsh] mode = \"tool\" in boxer.toml, or run `boxer shim install` and prepend the directory to PATH.")
	default:
		return *r, fmt.Errorf("no project-level install for %q; use `boxer package %s` and follow its README", harness, harness)
	}
	return *r, r.err
}

// User writes the user-level configuration files that orchestrators seed their managed harness
// homes from: Paperclip copies `~/.claude/settings.json` (not `plugins/`) and `~/.codex/config.toml`
// (not `hooks.json`). Codex user-level hooks also need no project trust.
func User(harness string, cfg config.Config, version string) (Result, error) {
	tmp, err := os.MkdirTemp("", "boxer-install-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp)
	if _, err := bundle.Render(harness, version, tmp); err != nil {
		return Result{}, err
	}
	r := &Result{}
	_, _, hooks, _, _, _ := parts(tmp, harness)
	switch harness {
	case "claude-code":
		if err := r.mergeJSON(filepath.Join(claudeHome(), "settings.json"), func(m map[string]any) {
			mergeHooks(m, hooks["hooks"])
		}); err != nil {
			return *r, err
		}
		r.Notes = append(r.Notes, "Settings hooks travel with orchestrators that seed a managed config dir from ~/.claude (Paperclip); plugins do not.")
	case "codex":
		block, err := codexHooksTOML(hooks["hooks"])
		if err != nil {
			return *r, err
		}
		if err := r.replaceBlock(filepath.Join(codexHome(), "config.toml"), blockStart, blockEnd, block); err != nil {
			return *r, err
		}
		r.Notes = append(r.Notes, "User-level hooks need no project trust and ride along when an orchestrator copies config.toml into a managed CODEX_HOME (Paperclip).")
	default:
		return *r, fmt.Errorf("no user-level install for %q; user-level layers exist for claude-code and codex", harness)
	}
	return *r, r.err
}

// parts locates one harness's pieces in its rendered view: the extension directory, its hooks
// file, the shared skill directory and AGENTS.md, and the shared MCP server entry with `--harness` added
// so the [harness.<name>] overrides apply.
func parts(tmp, harness string) (ns, hooksFile string, hooks map[string]any, skill, agents string, server map[string]any) {
	ns = filepath.Join(tmp, bundle.Namespace(harness))
	hooksFile = filepath.Join(ns, "hooks", "hooks.json")
	hooks = readJSON(hooksFile)
	skill = filepath.Join(tmp, "skills", "boxer")
	agents = filepath.Join(tmp, "AGENTS.md")
	servers, _ := readJSON(filepath.Join(tmp, "mcp.json"))["mcpServers"].(map[string]any)
	server, _ = servers["boxer"].(map[string]any)
	if server != nil {
		server["args"] = append(toStrings(server["args"]), "--harness", harness)
	}
	return
}

func claudeHome() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func codexHome() string {
	if d := os.Getenv("CODEX_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

const (
	blockStart = "# >>> boxer hooks (managed by `boxer install codex --user`; do not edit inside)"
	blockEnd   = "# <<< boxer hooks"
)

// codexHooksTOML encodes the hooks.json shape as the inline `[hooks]` tables Codex accepts in
// config.toml: one `[[hooks.<Event>]]` array entry per matcher group.
func codexHooksTOML(hooks any) (string, error) {
	events, ok := hooks.(map[string]any)
	if !ok {
		return "", fmt.Errorf("hooks.json: unexpected shape")
	}
	var names []string
	for e := range events {
		names = append(names, e)
	}
	slices.Sort(names)
	var b strings.Builder
	b.WriteString(blockStart + "\n")
	for _, event := range names {
		for _, g := range events[event].([]any) {
			group := g.(map[string]any)
			fmt.Fprintf(&b, "[[hooks.%s]]\n", event)
			if m, ok := group["matcher"].(string); ok {
				fmt.Fprintf(&b, "matcher = %q\n", m)
			}
			var entries []string
			for _, h := range group["hooks"].([]any) {
				hm := h.(map[string]any)
				e := fmt.Sprintf("{ type = %q, command = %q", hm["type"], hm["command"])
				if t, ok := hm["timeout"].(float64); ok {
					e += fmt.Sprintf(", timeout = %d", int(t))
				}
				entries = append(entries, e+" }")
			}
			fmt.Fprintf(&b, "hooks = [%s]\n\n", strings.Join(entries, ", "))
		}
	}
	b.WriteString(blockEnd + "\n")
	return b.String(), nil
}

// replaceBlock writes block into path, replacing the text between a previous start and end
// marker or appending.
func (r *Result) replaceBlock(path, blockStart, blockEnd, block string) error {
	cur, _ := os.ReadFile(path)
	s := string(cur)
	if i := strings.Index(s, blockStart); i >= 0 {
		if j := strings.Index(s[i:], blockEnd); j >= 0 {
			s = s[:i] + block + s[i+j+len(blockEnd)+1:]
		} else {
			s = s[:i] + block
		}
	} else {
		if len(s) > 0 && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if len(s) > 0 {
			s += "\n"
		}
		s += block
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		return err
	}
	r.Written = append(r.Written, path)
	return nil
}

// --- helpers --------------------------------------------------------------------------------

func readJSON(path string) map[string]any {
	m := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func (r *Result) mergeJSON(path string, edit func(map[string]any)) error {
	m := readJSON(path)
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 && len(m) == 0 {
		return fmt.Errorf("%s exists but is not a JSON object; not touching it", path)
	}
	edit(m)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	r.Written = append(r.Written, path)
	return nil
}

// mergeHooks adds boxer's hook groups to m["hooks"], event by event, skipping groups that
// already invoke boxer so a second install changes nothing.
func mergeHooks(m map[string]any, ours any) {
	existing, _ := m["hooks"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for event, groups := range ours.(map[string]any) {
		cur, _ := existing[event].([]any)
		for _, g := range groups.([]any) {
			if !hasBoxer(cur) {
				cur = append(cur, g)
			}
		}
		existing[event] = cur
	}
	m["hooks"] = existing
}

func hasBoxer(groups []any) bool {
	b, _ := json.Marshal(groups)
	return strings.Contains(string(b), "boxer hook")
}

func setIn(m map[string]any, section, key string, v any) {
	s, _ := m[section].(map[string]any)
	if s == nil {
		s = map[string]any{}
	}
	s[key] = v
	m[section] = s
}

// copyTree copies a published directory verbatim, executable bits included: the skill's scripts
// are code the agent runs.
func (r *Result) copyTree(src, dst string) {
	_ = filepath.WalkDir(src, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		r.copy(p, filepath.Join(dst, rel))
		return nil
	})
}

// copy writes src to dst, preserving the executable bit. It reports failure: an install that
// cannot write the skill it promised used to print the harness name and nothing else, and the
// operator learned about it from the agent ignoring a sandbox that was never configured.
func (r *Result) copy(src, dst string) {
	if r.err != nil {
		return
	}
	mode := os.FileMode(0o644)
	b, err := os.ReadFile(src)
	if err == nil {
		if st, serr := os.Stat(src); serr == nil && st.Mode()&0o111 != 0 {
			mode = 0o755
		}
		err = os.MkdirAll(filepath.Dir(dst), 0o755)
	}
	if err == nil {
		err = os.WriteFile(dst, b, mode)
	}
	if err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, dst)
}

const sectionMarker = "# boxer sandbox"

// appendSection appends the rendered instruction file to dst unless dst already carries it.
func (r *Result) appendSection(dst, src string) {
	if r.err != nil {
		return
	}
	body, err := os.ReadFile(src)
	if err != nil {
		r.keep(err)
		return
	}
	cur, _ := os.ReadFile(dst)
	if strings.Contains(string(cur), sectionMarker) {
		// Installed content is never edited in place: `doctor` reports drift and a re-install by
		// hand is the fix, because this file belongs to the repository, not to boxer.
		return
	}
	sep := ""
	if len(cur) > 0 && !strings.HasSuffix(string(cur), "\n\n") {
		sep = "\n\n"
	}
	if err := os.WriteFile(dst, append(append(cur, []byte(sep)...), body...), 0o644); err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, dst)
}

// Drift lists the installed files under root whose bytes differ from what this binary would
// write. Published content is static, so a comparison is the whole check: nothing rewrites an
// installed file in place, and a mismatch means the repository was configured by another release
// (R-PKG-9).
func Drift(root, version string) []string {
	tmp, err := os.MkdirTemp("", "boxer-drift-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmp)
	if _, err := bundle.Render(bundle.Package, version, tmp); err != nil {
		return nil
	}
	var stale []string
	for _, dir := range []string{".claude/skills/boxer", ".agents/skills/boxer"} {
		stale = append(stale, treeDrift(filepath.Join(tmp, "skills", "boxer"), root, dir)...)
	}
	for _, f := range [][2]string{
		{".claude/agents/boxed.md", "agents/boxed.md"},
		{"GEMINI.md", "AGENTS.md"},
		{"AGENTS.md", "AGENTS.md"},
	} {
		cur, err := os.ReadFile(filepath.Join(root, f[0]))
		if err != nil {
			continue
		}
		want, err := os.ReadFile(filepath.Join(tmp, f[1]))
		if err != nil {
			continue
		}
		// A context file belongs to the repository and only carries boxer's section, so an
		// untouched one is not drift; the question is whether that section is the current one.
		if f[1] == "AGENTS.md" && !strings.Contains(string(cur), sectionMarker) {
			continue
		}
		if !bytes.Contains(cur, bytes.TrimSpace(want)) {
			stale = append(stale, f[0])
		}
	}
	slices.Sort(stale)
	return stale
}

// treeDrift compares every file of the published skill with its installed copy, ignoring a
// directory that was never installed.
func treeDrift(src, root, dir string) []string {
	if _, err := os.Stat(filepath.Join(root, dir)); err != nil {
		return nil
	}
	var stale []string
	_ = filepath.WalkDir(src, func(p string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		want, _ := os.ReadFile(p)
		got, gerr := os.ReadFile(filepath.Join(root, dir, rel))
		if gerr != nil || !bytes.Equal(want, got) {
			stale = append(stale, filepath.Join(dir, rel))
		}
		return nil
	})
	return stale
}

func toStrings(v any) []string {
	var out []string
	if arr, ok := v.([]any); ok {
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// appendOnce adds s unless the list already has it; order is meaningful in the files we merge, so
// this is not a set.
func appendOnce(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}
