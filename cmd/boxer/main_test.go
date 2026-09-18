package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

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
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)

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
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
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

func TestDoctorReportsDriftAgainstTheEmbeddedCopy(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "install", "claude-code"); code != 0 {
		t.Fatal(out)
	}
	var doc map[string]any
	call(t, &doc, "doctor", "--json")
	if d, _ := json.Marshal(doc["drift"]); string(d) != "null" {
		t.Fatalf("a fresh install must not drift: %s", d)
	}
	// Installed content is never edited in place, so an edited file is exactly what drift means.
	skill := filepath.Join(mustGetwd(t), ".claude", "skills", "boxer", "SKILL.md")
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, append(b, []byte("\nedited by hand\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	call(t, &doc, "doctor", "--json")
	d, _ := json.Marshal(doc["drift"])
	if !strings.Contains(string(d), ".claude/skills/boxer/SKILL.md") {
		t.Fatalf("edited skill not reported as drift: %s", d)
	}
	warns, _ := json.Marshal(doc["warnings"])
	if !strings.Contains(string(warns), "differs from the copy in boxer") {
		t.Fatalf("drift is not warned about: %s", warns)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
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

// Named tasks are the deterministic path: the agent invokes a name the repository declared, so
// whether the work is sandboxed no longer depends on the intercept list matching a composed line.
func TestTasksAndBrief(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks]\ntest = \"echo ran-the-task\"\nbuild = \"make build\"\n")

	var rows []map[string]any
	if code, _ := call(t, &rows, "tasks", "--json"); code != 0 || len(rows) != 2 || rows[0]["name"] != "build" || rows[1]["command"] != "echo ran-the-task" {
		t.Fatalf("tasks: %d %v", code, rows)
	}
	var b map[string]any
	if code, _ := call(t, &b, "brief", "--json"); code != 0 || b["mode"] != "rewrite" || b["mount_at"] == "" {
		t.Fatalf("brief: %d %v", code, b)
	}
	if tasks, _ := b["tasks"].(map[string]any); tasks["test"] != "echo ran-the-task" {
		t.Fatalf("brief carries the tasks: %v", b["tasks"])
	}
	if !strings.Contains(b["brief"].(string), "boxer sandbox") {
		t.Fatalf("brief text: %v", b["brief"])
	}
	if code, out := call(t, nil, "brief"); code != 0 || !strings.Contains(out, "Tasks (boxer run --task <name>): build, test") {
		t.Fatalf("prose brief: %d %s", code, out)
	}
	if code, out := call(t, nil, "run", "--task", "test"); code != 0 || !strings.Contains(out, "ran-the-task") {
		t.Fatalf("run --task: %d %s", code, out)
	}
	// An unknown name is a refusal an agent can act on: the fix line lists what does exist.
	code, out := call(t, nil, "run", "--task", "tset")
	if code != 1 || !strings.Contains(out, "NO_SUCH_TASK") || !strings.Contains(out, "fix:       boxer run --task build | test") {
		t.Fatalf("unknown task: %d %s", code, out)
	}
}
