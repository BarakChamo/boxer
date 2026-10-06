package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BarakChamo/boxer/internal/bundle"
)

// Uninstall removes what Install wrote for harness under root, and only that. Merged files keep
// everything that is not boxer's: other hooks, other MCP servers, the rest of AGENTS.md. A file
// or directory boxer's removal leaves empty is deleted, because boxer is what created it.
//
// It is the inverse of Install case by case, so the two are kept next to each other in spirit: a
// new write in Install needs its undo here, and TestUninstallUndoesInstall fails until it has one.
func Uninstall(harness, root string) (Result, error) {
	tmp, err := os.MkdirTemp("", "boxer-uninstall-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // cleanup of a temporary
	if _, err := bundle.Render(harness, "", tmp); err != nil {
		return Result{}, err
	}
	r := &Result{}
	_, _, _, _, agents, _ := parts(tmp, harness)
	switch harness {
	case "claude-code":
		r.unmergeHooks(root, filepath.Join(root, ".claude", "settings.json"))
		r.removeTree(root, filepath.Join(root, ".claude", "skills", "boxer"))
		r.removeFile(root, filepath.Join(root, ".claude", "agents", "boxed.md"))
		r.shared(root, harness, sharedMCP, func() { r.unsetIn(root, filepath.Join(root, ".mcp.json"), "mcpServers", "boxer") })
	case "codex":
		r.unmergeHooks(root, filepath.Join(root, ".codex", "hooks.json"))
		if main := mainRepo(root); main != "" {
			r.unmergeHooks(main, filepath.Join(main, ".codex", "hooks.json"))
		}
		r.shared(root, harness, sharedSkill, func() { r.removeTree(root, filepath.Join(root, ".agents", "skills", "boxer")) })
	case "gemini-cli":
		r.unmergeHooks(root, filepath.Join(root, ".gemini", "settings.json"))
		r.unsetIn(root, filepath.Join(root, ".gemini", "settings.json"), "mcpServers", "boxer")
		r.removeSection(root, filepath.Join(root, "GEMINI.md"), agents)
		r.Notes = append(r.Notes, "If tool mode added run_shell_command to tools.exclude in .gemini/settings.json, it was left there: boxer cannot tell it apart from one you added.")
	case "opencode":
		r.removeFile(root, filepath.Join(root, ".opencode", "plugins", "boxer.ts"))
		r.unsetIn(root, filepath.Join(root, "opencode.json"), "mcp", "boxer")
		r.shared(root, harness, sharedAgents, func() { r.removeSection(root, filepath.Join(root, "AGENTS.md"), agents) })
	case "grok":
		r.unmergeHooks(root, filepath.Join(root, ".grok", "hooks", "boxer.json"))
		r.shared(root, harness, sharedMCP, func() { r.unsetIn(root, filepath.Join(root, ".mcp.json"), "mcpServers", "boxer") })
		r.shared(root, harness, sharedSkill, func() { r.removeTree(root, filepath.Join(root, ".agents", "skills", "boxer")) })
	case "pi":
		r.removeFile(root, filepath.Join(root, ".pi", "extensions", "boxer.ts"))
		r.shared(root, harness, sharedAgents, func() { r.removeSection(root, filepath.Join(root, "AGENTS.md"), agents) })
	case "kimi":
		r.unsetIn(root, filepath.Join(root, ".kimi-code", "mcp.json"), "mcpServers", "boxer")
		r.shared(root, harness, sharedSkill, func() { r.removeTree(root, filepath.Join(root, ".agents", "skills", "boxer")) })
		r.Notes = append(r.Notes, "Remove the boxer [[hooks]] entries you appended to ~/.kimi-code/config.toml by hand.")
	case "dsh":
		r.removeFile(root, filepath.Join(root, ".dsh", "cordis.patch.yml"))
		r.unmergeHooks(root, filepath.Join(root, ".dsh", "hooks.json"))
		r.shared(root, harness, sharedSkill, func() { r.removeTree(root, filepath.Join(root, ".agents", "skills", "boxer")) })
	default:
		return *r, fmt.Errorf("no project-level install for %q to remove", harness)
	}
	return *r, r.err
}

// Files several harnesses install into, and which harnesses use each. Removing one when another
// harness still relies on it uninstalled that harness too: `boxer uninstall grok` deleted Claude
// Code's MCP server and the skill codex, kimi and dsh read.
var (
	sharedMCP    = []string{"claude-code", "grok"}          // .mcp.json mcpServers.boxer
	sharedSkill  = []string{"codex", "grok", "kimi", "dsh"} // .agents/skills/boxer
	sharedAgents = []string{"opencode", "pi"}               // the AGENTS.md section
)

// shared runs remove unless another harness in users is still installed in root.
func (r *Result) shared(root, harness string, users []string, remove func()) {
	var still []string
	for _, u := range users {
		if u != harness && installedHere(root, u) {
			still = append(still, u)
		}
	}
	if len(still) > 0 {
		r.Notes = append(r.Notes, "kept a file "+strings.Join(still, ", ")+" also uses; it goes with the last of them")
		return
	}
	remove()
}

// installedHere reports whether a harness's own, unshared files show it is installed in root.
func installedHere(root, h string) bool {
	has := func(rel string, needle string) bool {
		b, err := os.ReadFile(filepath.Join(root, rel))
		return err == nil && (needle == "" || strings.Contains(string(b), needle))
	}
	switch h {
	case "claude-code":
		return has(".claude/settings.json", "boxer hook")
	case "codex":
		return has(".codex/hooks.json", "boxer hook")
	case "grok":
		return has(".grok/hooks/boxer.json", "")
	case "kimi":
		return has(".kimi-code/mcp.json", "boxer")
	case "dsh":
		return has(".dsh/cordis.patch.yml", "") || has(".dsh/hooks.json", "boxer hook")
	case "opencode":
		return has(".opencode/plugins/boxer.ts", "")
	case "pi":
		return has(".pi/extensions/boxer.ts", "")
	}
	return false
}

// UninstallUser removes what User wrote for harness.
func UninstallUser(harness string) (Result, error) {
	r := &Result{}
	switch harness {
	case "claude-code":
		r.unmergeHooks(claudeHome(), filepath.Join(claudeHome(), "settings.json"))
	case "codex":
		r.removeBlock(filepath.Join(codexHome(), "config.toml"), blockStart, blockEnd)
	case "copilot":
		home := copilotHome()
		r.unmergeHooks(home, filepath.Join(home, "hooks", "boxer.json"))
		r.unsetIn(home, filepath.Join(home, "mcp-config.json"), "mcpServers", "boxer")
		r.removeTree(home, filepath.Join(home, "skills", "boxer"))
	default:
		return *r, fmt.Errorf("no user-level install for %q to remove", harness)
	}
	return *r, r.err
}

// UninstallGit removes boxer's block from the post-checkout hook, and the hook itself when boxer's
// block was all it held.
func UninstallGit(repoRoot string) (Result, error) {
	r := &Result{}
	path, err := hookPath(repoRoot)
	if err != nil {
		return *r, err
	}
	r.removeBlock(path, gitBlockStart, gitBlockEnd)
	if b, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(b)) == "#!/bin/sh" {
		if err := os.Remove(path); err != nil {
			r.keep(err)
		}
	}
	return *r, r.err
}

// UninstallConductor removes boxer's block from .conductor/settings.toml.
func UninstallConductor(root string) (Result, error) {
	r := &Result{}
	path := filepath.Join(root, ".conductor", "settings.toml")
	r.removeBlock(path, conductorTablesStart, conductorTablesEnd)
	r.removeBlock(path, conductorStart, conductorEnd)
	r.pruneDirs(root, filepath.Dir(path))
	return *r, r.err
}

// --- the undo of each write ------------------------------------------------------------------

// unmergeHooks drops every hook group that runs `boxer hook`, then events left with none, then the
// hooks table, then the file if nothing else is in it.
func (r *Result) unmergeHooks(root, path string) {
	r.editJSON(root, path, func(m map[string]any) {
		hooks, _ := m["hooks"].(map[string]any)
		for event, groups := range hooks {
			var keep []any
			for _, g := range asSlice(groups) {
				if g = withoutBoxerHooks(g); g != nil {
					keep = append(keep, g)
				}
			}
			if len(keep) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = keep
			}
		}
		if hooks != nil && len(hooks) == 0 {
			delete(m, "hooks")
		}
	})
}

// withoutBoxerHooks returns a hook group with boxer's own entries taken out, or nil when nothing
// is left. Only boxer's entries: a group can hold the user's hooks beside boxer's under one
// matcher, and dropping the whole group, as uninstall once did, deleted the user's hook too.
func withoutBoxerHooks(g any) any {
	isBoxer := func(v any) bool {
		b, _ := json.Marshal(v)
		return strings.Contains(string(b), "boxer hook")
	}
	m, ok := g.(map[string]any)
	if !ok {
		if isBoxer(g) {
			return nil
		}
		return g
	}
	inner, has := m["hooks"].([]any)
	if !has {
		if isBoxer(m) {
			return nil
		}
		return m
	}
	var rest []any
	for _, h := range inner {
		if !isBoxer(h) {
			rest = append(rest, h)
		}
	}
	if len(rest) == 0 {
		return nil
	}
	m["hooks"] = rest
	return m
}

// unsetIn removes m[section][key], then the section if it is left empty.
func (r *Result) unsetIn(root, path, section, key string) {
	r.editJSON(root, path, func(m map[string]any) {
		s, ok := m[section].(map[string]any)
		if !ok {
			return
		}
		delete(s, key)
		if len(s) == 0 {
			delete(m, section)
		}
	})
}

// editJSON applies edit to a JSON file boxer merged into. A file that ends up empty, or holding
// only the `$schema` boxer adds to opencode.json, is removed. A file that does not exist is
// already uninstalled. One that is not JSON is left alone and reported.
func (r *Result) editJSON(root, path string, edit func(map[string]any)) {
	if r.err != nil {
		return
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		r.keep(err)
		return
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		r.keep(fmt.Errorf("%s is not a JSON object; not touching it", path))
		return
	}
	before, _ := json.Marshal(m)
	edit(m)
	after, _ := json.Marshal(m)
	if string(before) == string(after) {
		return
	}
	if _, onlySchema := m["$schema"]; onlySchema && len(m) == 1 {
		// Only the schema line is left. boxer may have created the file or the user may have, and
		// nothing in it says which, so it is kept: one line left behind beats deleting a
		// user's file. The note says it can go.
		out := marshalJSON(m)
		if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
			r.keep(err)
			return
		}
		r.Written = append(r.Written, path)
		r.Notes = append(r.Notes, path+" now holds only $schema; delete it if boxer created it.")
		return
	}
	if len(m) == 0 {
		if err := os.Remove(path); err != nil {
			r.keep(err)
			return
		}
		r.Written = append(r.Written, path)
		r.pruneDirs(root, filepath.Dir(path))
		return
	}
	out := marshalJSON(m)
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, path)
}

// removeSection takes boxer's appended instruction text back out of an AGENTS.md or GEMINI.md,
// with the blank-line separator appendSection put before it, and deletes the file if that text was
// all of it. A section that no longer matches what this binary writes was edited, or came from
// another release: it is left, with a note, rather than guessed at.
func (r *Result) removeSection(root, path, src string) {
	if r.err != nil {
		return
	}
	cur, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		r.keep(err)
		return
	}
	body, err := os.ReadFile(src)
	if err != nil {
		r.keep(err)
		return
	}
	s := string(cur)
	i, n := findSection(s, string(body))
	if i < 0 {
		if strings.Contains(s, sectionMarker) {
			r.Notes = append(r.Notes, fmt.Sprintf("%s has a boxer section that differs from this release's; remove it by hand.", path))
		}
		return
	}
	start := i
	if strings.HasSuffix(s[:i], "\n\n") {
		start = i - 2
	}
	s = s[:start] + s[i+n:]
	if strings.TrimSpace(s) == "" {
		if err := os.Remove(path); err != nil {
			r.keep(err)
			return
		}
		r.Written = append(r.Written, path)
		return
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, path)
}

// removeBlock deletes a marked block and the blank line replaceBlock put before it.
func (r *Result) removeBlock(path, start, end string) {
	if r.err != nil {
		return
	}
	cur, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		r.keep(err)
		return
	}
	s := string(cur)
	i := strings.Index(s, start)
	if i < 0 {
		return
	}
	j := strings.Index(s[i:], end)
	stop := len(s)
	if j >= 0 {
		stop = i + j + len(end)
		if stop < len(s) && s[stop] == '\n' {
			stop++
		}
	}
	from := i
	if strings.HasSuffix(s[:i], "\n\n") {
		from = i - 1
	}
	s = s[:from] + s[stop:]
	if strings.TrimSpace(s) == "" {
		if err := os.Remove(path); err != nil {
			r.keep(err)
			return
		}
	} else if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, path)
}

func (r *Result) removeFile(root, path string) {
	if r.err != nil {
		return
	}
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			r.keep(err)
		}
		return
	}
	r.Written = append(r.Written, path)
	r.pruneDirs(root, filepath.Dir(path))
}

func (r *Result) removeTree(root, dir string) {
	if r.err != nil {
		return
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		r.keep(err)
		return
	}
	r.Written = append(r.Written, dir)
	r.pruneDirs(root, filepath.Dir(dir))
}

// pruneDirs removes dir and its parents up to (not including) root while they are empty: the
// .claude/skills or .agents boxer created for its own files.
func (r *Result) pruneDirs(root, dir string) {
	root = filepath.Clean(root)
	for d := filepath.Clean(dir); d != root && strings.HasPrefix(d, root+string(filepath.Separator)); d = filepath.Dir(d) {
		entries, err := os.ReadDir(d)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(d); err != nil {
			return
		}
	}
}

// findSection locates boxer's rendered instruction text in s, ignoring the release stamped into its
// `<!-- boxer_version: … -->` line, so a section installed by another release still comes out. It
// returns the start and length of the match, or -1.
func findSection(s, body string) (int, int) {
	const open, shut = "<!-- boxer_version: ", " -->"
	head, rest, ok := strings.Cut(body, open)
	if !ok {
		return strings.Index(s, body), len(body)
	}
	_, tail, ok := strings.Cut(rest, shut)
	if !ok {
		return strings.Index(s, body), len(body)
	}
	for from := 0; ; {
		i := strings.Index(s[from:], head+open)
		if i < 0 {
			return -1, 0
		}
		i += from
		after := s[i+len(head)+len(open):]
		if j := strings.Index(after, shut); j >= 0 && strings.HasPrefix(after[j+len(shut):], tail) {
			return i, len(head) + len(open) + j + len(shut) + len(tail)
		}
		from = i + 1
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
