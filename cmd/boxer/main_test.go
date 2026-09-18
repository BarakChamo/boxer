package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("require_worktree = \"off\"\n"), 0o644)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	r, _ := filepath.EvalSymlinks(dir)
	t.Chdir(r)
	return r
}

// call runs the CLI and decodes stdout as JSON into v when v is non-nil.
func call(t *testing.T, v any, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errb)
	if v != nil {
		if err := json.Unmarshal(out.Bytes(), v); err != nil {
			t.Fatalf("%v: not JSON: %v\n%s%s", args, err, out.String(), errb.String())
		}
	}
	return code, out.String() + errb.String()
}

func TestJSONOutputsAndStatusExitCodes(t *testing.T) {
	vmtest.Install(t)
	dir := repo(t)

	var st map[string]any
	if code, _ := call(t, &st, "status", "--json"); code != exitAbsent || st["exists"] != false || st["state"] != "absent" {
		t.Fatalf("absent: %d %v", code, st)
	}
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if code, _ := call(t, &st, "status", "--json"); code != 0 || st["state"] != "running" || st["worktree"] != dir {
		t.Fatalf("running: %d %v", code, st)
	}
	var ls []map[string]any
	if code, _ := call(t, &ls, "ls", "--json"); code != 0 || len(ls) != 1 || ls[0]["scope"] != st["scope"] || ls[0]["worktree"] != dir {
		t.Fatalf("ls: %d %v", code, ls)
	}
	var gc []any
	if code, _ := call(t, &gc, "gc", "--dry-run", "--json"); code != 0 || len(gc) != 0 {
		t.Fatalf("gc should keep a live worktree: %d %v", code, gc)
	}
	var doc map[string]any
	if code, _ := call(t, &doc, "doctor", "--json"); code != 0 || doc["version"] != Version || doc["sandbox"] == nil || doc["scope"] == nil {
		t.Fatalf("doctor: %d %v", code, doc)
	}
	var down map[string]any
	if code, _ := call(t, &down, "down", "--json"); code != 0 || down["removed"] != true {
		t.Fatalf("down: %d %v", code, down)
	}
	if code, _ := call(t, &st, "status", "--json"); code != exitAbsent {
		t.Fatalf("after down: %d", code)
	}
	if code, _ := call(t, &down, "down", "--json"); code != 0 || down["removed"] != false {
		t.Fatalf("second down: %d %v", code, down)
	}
}

func TestStatusStoppedExitCodeAndHumanDoctor(t *testing.T) {
	client, _ := vmtest.Install(t)
	repo(t)
	call(t, nil, "up")
	var st map[string]any
	call(t, &st, "status", "--json")
	if err := client.Stop(st["scope"].(string)); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, &st, "status", "--json"); code != exitStopped || st["state"] != "stopped" {
		t.Fatalf("stopped: %d %v", code, st)
	}
	code, out := call(t, nil, "doctor")
	if code != 0 || !strings.Contains(out, "sandbox:   stopped, image alpine") || !strings.Contains(out, "smolvm:    smolvm 0.0.0-fake") {
		t.Fatalf("doctor human output: %d\n%s", code, out)
	}
	if !strings.Contains(out, "signals:") || !strings.Contains(out, "claude-code  yes      yes      -      yes") || !strings.Contains(out, "boxer install git") {
		t.Fatalf("doctor signal table: %s", out)
	}
	var d map[string]any
	call(t, &d, "doctor", "--json")
	if rows, _ := d["signals"].([]any); len(rows) == 0 || rows[0].(map[string]any)["effective_isolation"] != "worktree" {
		t.Fatalf("doctor --json signals: %v", d["signals"])
	}
}

func TestDoctorWarnsOnInstalledVersionMismatch(t *testing.T) {
	vmtest.Install(t)
	repo(t)
	if code, out := call(t, nil, "install", "claude-code"); code != 0 {
		t.Fatal(out)
	}
	Version = "9.9.9"
	defer func() { Version = "dev" }()
	var doc map[string]any
	call(t, &doc, "doctor", "--json")
	warns, _ := json.Marshal(doc["warnings"])
	if !strings.Contains(string(warns), "installed by boxer dev; this is 9.9.9") {
		t.Fatalf("no mismatch warning: %s", warns)
	}
	if doc["installed_versions"].(map[string]any)[".claude/skills/boxer/SKILL.md"] != "dev" {
		t.Fatalf("installed_versions: %v", doc["installed_versions"])
	}
}

// A dashboard has machine names from `ls`, not worktrees: `down --scope` must work from a
// directory that is not a repository at all.
func TestDownByScopeNameOutsideAnyRepo(t *testing.T) {
	_, log := vmtest.Install(t)
	dir := t.TempDir() // no git repository here
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	if code := run([]string{"down", "--scope", "sb-deadbeef", "--json"}, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"scope": "sb-deadbeef"`) || !strings.Contains(out.String(), `"removed": true`) {
		t.Fatalf("json: %s", out.String())
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "machine delete -n sb-deadbeef") {
		t.Fatalf("smolvm log: %s", b)
	}
}
