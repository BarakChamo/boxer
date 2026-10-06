package install

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/BarakChamo/boxer/internal/config"
)

// tree lists every file under root with its bytes, so two states of a repository can be compared.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			b, _ := os.ReadFile(p)
			out[rel] = string(b)
		}
		return nil
	})
	return out
}

func dirs(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() && p != root {
			rel, _ := filepath.Rel(root, p)
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// On a fresh repository, uninstall leaves exactly what was there before install: no file, no
// directory, nothing boxer wrote.
func TestUninstallUndoesInstall(t *testing.T) {
	for _, h := range []string{"claude-code", "codex", "gemini-cli", "opencode", "grok", "pi", "kimi", "dsh"} {
		t.Run(h, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "README.md"), []byte("# app\n"), 0o644)
			before, beforeDirs := tree(t, root), dirs(t, root)
			if _, err := Install(h, config.Defaults(), "t", root); err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(tree(t, root), before) {
				t.Fatal("install wrote nothing; the test proves nothing")
			}
			if _, err := Uninstall(h, root); err != nil {
				t.Fatal(err)
			}
			got := tree(t, root)
			// OpenCode's file may be the user's, so a schema-only opencode.json is kept.
			if h == "opencode" {
				if m := readJSONT(t, filepath.Join(root, "opencode.json")); len(m) != 1 || m["$schema"] == nil {
					t.Errorf("opencode.json must be left holding only $schema: %v", m)
				}
				delete(got, "opencode.json")
			}
			if !reflect.DeepEqual(got, before) {
				t.Errorf("files left behind or changed: %v", keys(got))
			}
			if got := dirs(t, root); !reflect.DeepEqual(got, beforeDirs) {
				t.Errorf("directories left behind: %v", got)
			}
			// Twice is harmless.
			if _, err := Uninstall(h, root); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// What the user had in the files boxer merges into survives an install and an uninstall.
func TestUninstallKeepsTheUsersOwnConfiguration(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	settings := `{"permissions":{"allow":["Read"]},"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"fmt"}]}]}}`
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(settings), 0o644)
	os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o644)
	agents := "# Our agents\n\nBe kind.\n"
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644)

	for _, h := range []string{"claude-code", "opencode"} {
		if _, err := Install(h, config.Defaults(), "t", root); err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range []string{"claude-code", "opencode"} {
		if _, err := Uninstall(h, root); err != nil {
			t.Fatal(err)
		}
	}
	var want, got map[string]any
	json.Unmarshal([]byte(settings), &want)
	got = readJSONT(t, filepath.Join(root, ".claude", "settings.json"))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settings.json changed:\n got %v\nwant %v", got, want)
	}
	if m := readJSONT(t, filepath.Join(root, ".mcp.json")); !reflect.DeepEqual(m, map[string]any{"mcpServers": map[string]any{"other": map[string]any{"command": "x"}}}) {
		t.Errorf(".mcp.json changed: %v", m)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(b) != agents {
		t.Errorf("AGENTS.md changed:\n%q", b)
	}
	if m := readJSONT(t, filepath.Join(root, "opencode.json")); len(m) != 1 || m["$schema"] == nil {
		t.Errorf("opencode.json is kept with only $schema, since it may be the user's: %v", m)
	}
}

// The git hook: boxer's block comes out of a hook the user already had, and a hook boxer created
// goes away entirely.
func TestUninstallGit(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	hook, _ := hookPath(root)
	if _, err := Git(root); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallGit(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hook); !os.IsNotExist(err) {
		t.Fatal("a hook boxer created should be removed")
	}

	mine := "#!/bin/sh\necho mine\n"
	os.MkdirAll(filepath.Dir(hook), 0o755)
	os.WriteFile(hook, []byte(mine), 0o755)
	if _, err := Git(root); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallGit(root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(hook); string(b) != mine {
		t.Fatalf("the user's hook changed:\n%q", b)
	}
}

// The user layer: install --user then uninstall --user leaves each config directory as it was,
// including what the user had there first.
func TestUninstallUserUndoesInstallUser(t *testing.T) {
	homes := map[string]string{"CLAUDE_CONFIG_DIR": t.TempDir(), "CODEX_HOME": t.TempDir(), "COPILOT_HOME": t.TempDir()}
	for k, v := range homes {
		t.Setenv(k, v)
	}
	mine := `{"permissions":{"allow":["Read"]}}`
	os.WriteFile(filepath.Join(homes["CLAUDE_CONFIG_DIR"], "settings.json"), []byte(mine), 0o644)
	before := map[string]map[string]string{}
	for k, v := range homes {
		before[k] = tree(t, v)
	}
	for _, h := range []string{"claude-code", "codex", "copilot"} {
		if _, err := User(h, config.Defaults(), "t"); err != nil {
			t.Fatalf("%s: %v", h, err)
		}
	}
	changed := false
	for k, v := range homes {
		changed = changed || !reflect.DeepEqual(tree(t, v), before[k])
	}
	if !changed {
		t.Fatal("install --user wrote nothing; the test proves nothing")
	}
	for _, h := range []string{"claude-code", "codex", "copilot"} {
		if _, err := UninstallUser(h); err != nil {
			t.Fatalf("%s: %v", h, err)
		}
	}
	// JSON is compared by value: install already rewrites a file it merges into with its own
	// indentation, and uninstall keeps that.
	for k, v := range homes {
		got := tree(t, v)
		if !reflect.DeepEqual(keys(got), keys(before[k])) {
			t.Errorf("%s: left behind: %v", k, keys(got))
			continue
		}
		for f, body := range got {
			var a, b any
			if json.Unmarshal([]byte(body), &a) == nil && json.Unmarshal([]byte(before[k][f]), &b) == nil {
				if !reflect.DeepEqual(a, b) {
					t.Errorf("%s/%s changed: %s", k, f, body)
				}
			} else if body != before[k][f] {
				t.Errorf("%s/%s changed: %s", k, f, body)
			}
		}
	}
	var got map[string]any
	json.Unmarshal([]byte(tree(t, homes["CLAUDE_CONFIG_DIR"])["settings.json"]), &got)
	if !reflect.DeepEqual(got, map[string]any{"permissions": map[string]any{"allow": []any{"Read"}}}) {
		t.Errorf("the user's settings changed: %v", got)
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Conductor: boxer's block comes out of settings.toml; a file that held only it goes, and one that
// held the user's settings too keeps them.
func TestUninstallConductor(t *testing.T) {
	root := t.TempDir()
	if _, err := Conductor(root, "/opt/shims"); err != nil {
		t.Fatal(err)
	}
	if !ConductorInstalled(root) {
		t.Fatal("install wrote nothing")
	}
	if _, err := UninstallConductor(root); err != nil {
		t.Fatal(err)
	}
	if got := dirs(t, root); len(got) != 0 {
		t.Fatalf("left behind: %v", got)
	}
	os.MkdirAll(filepath.Join(root, ".conductor"), 0o755)
	mine := "[scripts]\nsetup = \"make\"\n"
	os.WriteFile(filepath.Join(root, ".conductor", "settings.toml"), []byte(mine), 0o644)
	Conductor(root, "/opt/shims")
	if _, err := UninstallConductor(root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".conductor", "settings.toml")); strings.TrimSpace(string(b)) != strings.TrimSpace(mine) {
		t.Fatalf("the user's settings changed:\n%q", b)
	}
	// Nothing installed: harmless, and it says so.
	if _, err := UninstallConductor(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall("nope", root); err == nil {
		t.Fatal("an unknown harness must be refused")
	}
	if _, err := UninstallUser("gemini-cli"); err == nil {
		t.Fatal("a harness with no user layer must be refused")
	}
}

// What uninstall cannot be sure is boxer's, it leaves, and says so: a settings file that is not
// JSON is an error rather than an overwrite, and an edited boxer section is a note.
func TestUninstallLeavesWhatItCannotRead(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte("{not json"), 0o644)
	if _, err := Uninstall("claude-code", root); err == nil || !strings.Contains(err.Error(), "not a JSON object") {
		t.Fatalf("got %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json")); string(b) != "{not json" {
		t.Fatal("a file uninstall cannot read must not be touched")
	}

	root = t.TempDir()
	if _, err := Install("gemini-cli", config.Defaults(), "t", root); err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "GEMINI.md")
	b, _ := os.ReadFile(agents)
	edited := strings.Replace(string(b), sectionMarker+"\n", sectionMarker+"\nA line someone added by hand.\n", 1)
	if edited == string(b) {
		t.Fatal("the test did not edit the section")
	}
	os.WriteFile(agents, []byte(edited), 0o644)
	r, err := Uninstall("gemini-cli", root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.Notes, "\n"), "differs from this release") {
		t.Fatalf("an edited section must be named: %v", r.Notes)
	}
	if b, _ := os.ReadFile(agents); string(b) != edited {
		t.Fatal("an edited section must be left as it is")
	}

	// A path that cannot be read as a file is an error, not a silent skip.
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, "GEMINI.md"), 0o755)
	if _, err := Uninstall("gemini-cli", root); err == nil {
		t.Fatal("an unreadable GEMINI.md must be an error")
	}
}

// The post-checkout block never changes git's exit status: a checkout that is not a new worktree
// (flag 0) succeeds, and a failure in the user's own hook is still reported.
func TestGitHookKeepsGitsExitStatus(t *testing.T) {
	gitRun := func(dir string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin") // no boxer on PATH, as on many hosts
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	root := t.TempDir()
	gitRun(root, "init", "-q")
	os.WriteFile(filepath.Join(root, "f"), []byte("a"), 0o644)
	gitRun(root, "add", "f")
	gitRun(root, "commit", "-qm", "i")
	if _, err := Git(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "f"), []byte("changed"), 0o644)
	if out, err := gitRun(root, "checkout", "--", "f"); err != nil {
		t.Fatalf("git checkout -- f failed because of the hook: %v %s", err, out)
	}
	if out, err := gitRun(root, "checkout", "-q", "-b", "other"); err != nil {
		t.Fatalf("a branch switch failed because of the hook: %v %s", err, out)
	}

	// The user's hook fails: the block must not turn that into success.
	hook, _ := hookPath(root)
	UninstallGit(root)
	os.WriteFile(hook, []byte("#!/bin/sh\nfalse\n"), 0o755)
	if _, err := Git(root); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", hook, "a", "b", "0")
	if err := cmd.Run(); err == nil {
		t.Fatal("the user's failing hook must still fail")
	}

	// A hook in another language is left alone, with the line to add.
	UninstallGit(root)
	os.WriteFile(hook, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0o755)
	if _, err := Git(root); err == nil || !strings.Contains(err.Error(), "python3") {
		t.Fatalf("a python hook must be refused: %v", err)
	}
	if b, _ := os.ReadFile(hook); strings.Contains(string(b), "boxer") {
		t.Fatal("the python hook was edited")
	}
}

// A hook group can hold the user's hook beside boxer's under one matcher; uninstall takes out
// boxer's entry and keeps the user's.
func TestUninstallKeepsAUsersHookInASharedGroup(t *testing.T) {
	root := t.TempDir()
	if _, err := Install("claude-code", config.Defaults(), "t", root); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, ".claude", "settings.json")
	m := readJSONT(t, p)
	pre := m["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)
	pre["hooks"] = append(pre["hooks"].([]any), map[string]any{"type": "command", "command": "./audit.sh"})
	b, _ := json.Marshal(m)
	os.WriteFile(p, b, 0o644)
	if _, err := Uninstall("claude-code", root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), "./audit.sh") || strings.Contains(string(got), "boxer hook") {
		t.Fatalf("uninstall must keep the user's hook and only that:\n%s", got)
	}
}

// A block whose end marker is the file's last byte (no trailing newline) is replaced, not a panic;
// `{}` is a JSON object; and &, < and > are written as they are.
func TestReplaceBlockAtEndOfFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	os.WriteFile(p, []byte("a = 1\n"+blockStart+"\nold\n"+blockEnd), 0o644)
	r := &Result{}
	if err := r.replaceBlock(p, blockStart, blockEnd, blockStart+"\nnew\n"+blockEnd+"\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "a = 1\n"+blockStart+"\nnew\n"+blockEnd+"\n" {
		t.Fatalf("%q", b)
	}
	q := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(q, []byte("{}"), 0o644)
	if err := r.mergeJSON(q, func(m map[string]any) { m["a"] = "x && <y>" }); err != nil {
		t.Fatalf("{} is a JSON object: %v", err)
	}
	if b, _ := os.ReadFile(q); !strings.Contains(string(b), "x && <y>") {
		t.Fatalf("&, < and > must be written as they are: %s", b)
	}
}

// The executable paths are top-level keys, so they must land above the user's first table; and a
// table boxer would add twice is refused with the lines to add, not written as invalid TOML.
func TestConductorSettingsStayValidTOML(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".conductor", "settings.toml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("theme = \"dark\"\n\n[ui]\nfont = 12\n"), 0o644)
	if _, err := Conductor(root, "/opt/shims"); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if _, err := toml.DecodeFile(p, &m); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}
	if m["claude_code_executable_path"] != "/opt/shims/claude" || m["theme"] != "dark" {
		t.Fatalf("top-level keys must stay top-level: %v", m)
	}
	if ui := m["ui"].(map[string]any); len(ui) != 1 {
		t.Fatalf("nothing of boxer's may land in [ui]: %v", ui)
	}
	// Twice is the same file.
	before, _ := os.ReadFile(p)
	Conductor(root, "/opt/shims")
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Fatalf("a second install changed the file:\n%s\n---\n%s", before, after)
	}
	UninstallConductor(root)
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "boxer") || !strings.Contains(string(b), "[ui]") {
		t.Fatalf("uninstall must leave the user's settings:\n%s", b)
	}

	os.WriteFile(p, []byte("[scripts]\nsetup = \"make\"\n"), 0o644)
	if _, err := Conductor(root, "/opt/shims"); err == nil || !strings.Contains(err.Error(), "[scripts]") {
		t.Fatalf("an existing [scripts] must be refused: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "[scripts]\nsetup = \"make\"\n" {
		t.Fatalf("the refused file was changed: %s", b)
	}
}

// A worktree of a bare repository has no main working tree, so nothing is written above it.
func TestMainRepoOfABareRepository(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "proj.git")
	for _, args := range [][]string{{"init", "-q", "--bare", bare}, {"-C", bare, "worktree", "add", "-q", "--orphan", "-b", "w", filepath.Join(dir, "wt")}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Skipf("git %v: %v %s", args, err, out)
		}
	}
	if m := mainRepo(filepath.Join(dir, "wt")); m != "" {
		t.Fatalf("a bare repository has no main working tree, got %q", m)
	}
}

// Uninstalling one harness leaves what another still uses, and uninstalling them all still leaves
// the repository as it was.
func TestUninstallKeepsFilesAnotherHarnessUses(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# app\n"), 0o644)
	before := tree(t, root)
	all := []string{"claude-code", "codex", "gemini-cli", "opencode", "grok", "pi", "kimi", "dsh"}
	for _, h := range all {
		if _, err := Install(h, config.Defaults(), "t", root); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Uninstall("grok", root); err != nil {
		t.Fatal(err)
	}
	if m := readJSONT(t, filepath.Join(root, ".mcp.json")); m["mcpServers"].(map[string]any)["boxer"] == nil {
		t.Error("Claude Code's MCP server went with grok")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "boxer")); err != nil {
		t.Error("the skill codex, kimi and dsh read went with grok")
	}
	if _, err := Uninstall("pi", root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); !strings.Contains(string(b), sectionMarker) {
		t.Error("OpenCode's AGENTS.md section went with pi")
	}
	for _, h := range all {
		if _, err := Uninstall(h, root); err != nil {
			t.Fatal(err)
		}
	}
	got := tree(t, root)
	delete(got, "opencode.json") // kept with only $schema, as it may be the user's
	if !reflect.DeepEqual(got, before) {
		t.Errorf("uninstalling every harness left: %v", keys(got))
	}
}
