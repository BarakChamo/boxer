package main

import (
	"bytes"
	"encoding/json"
	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// Telemetry off by default; with the file sink on, a run leaves events that `boxer logs` reads
// back and `status --json` carries the tail of.
func TestTelemetryOffByDefaultThenLogs(t *testing.T) {
	vmtest.Install(t)
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(state, "boxer", "events.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("a default run must write no event log: %v", err)
	}
	if code, out := call(t, nil, "logs"); code != 1 || !strings.Contains(out, "telemetry") {
		t.Fatalf("logs without a stream must say how to turn one on: %d %s", code, out)
	}

	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[telemetry]\nenabled = true\n")
	if code, out := call(t, nil, "run", "-c", "true"); code != 0 {
		t.Fatal(out)
	}
	var events []map[string]any
	if code, out := call(t, &events, "logs", "--json"); code != 0 || len(events) == 0 {
		t.Fatalf("logs: %d %s", code, out)
	}
	names := map[string]bool{}
	for _, e := range events {
		names[e["event"].(string)] = true
		if p, ok := e["payload"].(map[string]any); ok {
			if _, leaked := p["command"]; leaked {
				t.Fatalf("command lines must be elided by default: %v", e)
			}
		}
	}
	for _, want := range []string{"resolve", "provision", "run"} {
		if !names[want] {
			t.Fatalf("missing %s event in %v", want, names)
		}
	}
	var st map[string]any
	call(t, &st, "status", "--json")
	if tail, _ := st["events"].([]any); len(tail) == 0 {
		t.Fatalf("status --json must carry the event tail: %v", st["events"])
	}
}

// `down --all` and `gc` are the two sweeps, and both need more than one machine to mean anything.
func TestDownAllAndGCSweeps(t *testing.T) {
	client, _ := vmtest.Install(t)
	packs := t.TempDir()
	t.Setenv("BOXER_PACKS", packs)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	// A second machine whose worktree is gone: what gc exists for.
	gone := filepath.Join(t.TempDir(), "removed")
	if err := client.Create(vm.CreateSpec{Name: "sb-orphan", Labels: map[string]string{"boxer.scope": "sb-orphan", "boxer.root": gone}}); err != nil {
		t.Fatal(err)
	}
	// And a pack nothing references, older than the idle timeout.
	pack := filepath.Join(packs, "stale.smolmachine")
	if err := os.WriteFile(pack, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(pack, old, old); err != nil {
		t.Fatal(err)
	}

	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 {
		t.Fatalf("gc: %d %s", code, out)
	}
	var deletedMachine, deletedPack bool
	for _, r := range rows {
		if r["deleted"] != true {
			t.Fatalf("gc row not deleted: %v", r)
		}
		if r["pack"] == pack {
			deletedPack = true
		}
		if r["scope"] == "sb-orphan" {
			deletedMachine = true
		}
	}
	if !deletedMachine || !deletedPack {
		t.Fatalf("gc must sweep the orphaned machine and the unreferenced pack: %v", rows)
	}
	if _, err := os.Stat(pack); !os.IsNotExist(err) {
		t.Fatalf("pack survived gc: %v", err)
	}

	var down []map[string]any
	if code, out := call(t, &down, "down", "--all", "--json"); code != 0 || len(down) != 1 {
		t.Fatalf("down --all: %d %v %s", code, down, out)
	}
	var ls []map[string]any
	if code, _ := call(t, &ls, "ls", "--json"); code != 0 || len(ls) != 0 {
		t.Fatalf("nothing should be left: %v", ls)
	}
	// A smolvm that refuses the delete must fail the command rather than report a clean sweep.
	if err := client.Create(vm.CreateSpec{Name: "sb-stuck", Labels: map[string]string{"boxer.scope": "sb-stuck"}}); err != nil {
		t.Fatal(err)
	}
	vmtest.FailVerb(t, "machine delete", "machine is busy")
	if code, out := call(t, nil, "down", "--all"); code != 1 || !strings.Contains(out, "busy") {
		t.Fatalf("down --all with a refusing smolvm: %d %s", code, out)
	}
}

// shim, package and inside are the three commands with no JSON output and no test, and each one
// writes something a user then depends on: programs on PATH, a published directory, a harness in
// the guest. Their usage and refusal paths are what a person hits first.
func TestShimPackageAndInsideUsage(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	dir := t.TempDir()

	if code, out := call(t, nil, "shim", "install", "--shell", dir); code != 0 || !strings.Contains(out, "boxer-bash") {
		t.Fatalf("shim install --shell: %d %s", code, out)
	}
	if st, err := os.Stat(filepath.Join(dir, "boxer-bash")); err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("boxer-bash must be executable: %v", err)
	}
	if code, out := call(t, nil, "shim", "install", "--harness", "claude,codex", dir); code != 0 || !strings.Contains(out, "2 harness shims") {
		t.Fatalf("shim install --harness: %d %s", code, out)
	}
	if code, out := call(t, nil, "shim", "install", "--harness", "nope", dir); code != 2 || !strings.Contains(out, "unknown harness") {
		t.Fatalf("an unknown harness must name the known ones: %d %s", code, out)
	}
	if code, out := call(t, nil, "shim", "install", dir); code != 0 || !strings.Contains(out, "shims to") {
		t.Fatalf("shim install from the intercept list: %d %s", code, out)
	}
	if code, out := call(t, nil, "shim"); code != 2 || !strings.Contains(out, "usage") {
		t.Fatalf("bare shim: %d %s", code, out)
	}

	// package renders the whole thing: one package plus one view per harness.
	out := filepath.Join(t.TempDir(), "dist")
	if code, o := call(t, nil, "package", "all", "--out", out); code != 0 {
		t.Fatalf("package all: %d %s", code, o)
	}
	if _, err := os.Stat(filepath.Join(out, "boxer", "skills", "boxer", "scripts", "task")); err != nil {
		t.Fatalf("the package must carry the skill's scripts: %v", err)
	}
	if code, o := call(t, nil, "package", "nosuchharness", "--out", out); code == 0 {
		t.Fatalf("an unknown harness must not render: %d %s", code, o)
	}

	// inside without a harness names the ones that exist rather than guessing.
	if code, o := call(t, nil, "shell"); code != 2 || !strings.Contains(o, "harnesses:") {
		t.Fatalf("bare shell: %d %s", code, o)
	}
}

// A `go install github.com/BarakChamo/boxer/cmd/boxer@v1.2.3` carries no ldflags, so the version
// has to come from the module the toolchain recorded. Reporting "dev" there makes doctor's drift
// warning meaningless and makes a bug report ambiguous.
func TestVersionFallsBackToTheModuleVersion(t *testing.T) {
	if got := versionOrBuildInfo("v1.2.3"); got != "v1.2.3" {
		t.Fatalf("a stamped build keeps its stamp: %q", got)
	}
	// In a test binary the build info says "(devel)", which is exactly the case that must stay
	// "dev" rather than leaking a placeholder into a bug report.
	if got := versionOrBuildInfo("dev"); got != "dev" {
		t.Fatalf("an unstamped build from a working tree: %q", got)
	}
}

// gc --all is the answer to "my disk is full and I do not care about the cache": it ignores
// idle_timeout, takes every stopped sandbox and every unreferenced pack, and says how much it
// freed. The default sweep must leave a fresh pack alone.
func TestGCAllReclaimsEveryPack(t *testing.T) {
	client, _ := vmtest.Install(t)
	packs := t.TempDir()
	t.Setenv("BOXER_PACKS", packs)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	pack := filepath.Join(packs, "fresh.smolmachine")
	if err := os.WriteFile(pack, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := client.Create(vm.CreateSpec{Name: "sb-stopped", Labels: map[string]string{"boxer.scope": "sb-stopped", "boxer.root": t.TempDir()}}); err != nil {
		t.Fatal(err)
	}

	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--dry-run", "--json"); code != 0 || len(rows) != 0 {
		t.Fatalf("a fresh pack and a live worktree are not stale: %d %v %s", code, rows, out)
	}
	if code, out := call(t, nil, "gc", "--all", "--dry-run"); code != 0 || !strings.Contains(out, "would reclaim") || !strings.Contains(out, "fresh.smolmachine") {
		t.Fatalf("gc --all: %d %s", code, out)
	}
	if _, err := os.Stat(pack); err != nil {
		t.Fatal("a dry run must delete nothing")
	}
	if code, out := call(t, nil, "gc", "--all"); code != 0 || !strings.Contains(out, "reclaimed") {
		t.Fatalf("gc --all: %d %s", code, out)
	}
	if _, err := os.Stat(pack); !os.IsNotExist(err) {
		t.Fatalf("the pack survived gc --all: %v", err)
	}
	ls, _ := client.List()
	for _, m := range ls {
		if m.Name == "sb-stopped" {
			t.Fatal("a stopped sandbox must not survive gc --all")
		}
	}
}

// doctor answers "what is boxer costing me", because that is the question a sandbox tool has to
// be able to answer about itself.
func TestDoctorReportsStorage(t *testing.T) {
	vmtest.Install(t)
	packs := t.TempDir()
	t.Setenv("BOXER_PACKS", packs)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if err := os.WriteFile(filepath.Join(packs, "p.smolmachine"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if code, out := call(t, &r, "doctor", "--json"); code != 0 {
		t.Fatalf("doctor: %d %s", code, out)
	}
	st, _ := r["storage"].(map[string]any)
	if st == nil || st["pack_count"].(float64) != 1 || st["pack_bytes"].(float64) != 1024 || st["free_bytes"].(float64) <= 0 {
		t.Fatalf("storage: %v", r["storage"])
	}
	if code, out := call(t, nil, "doctor"); code != 0 || !strings.Contains(out, "storage:") {
		t.Fatalf("human doctor must print the footprint: %d %s", code, out)
	}
}

// The release stamp is a linker flag, and `-X` refuses, silently, to rewrite a variable whose
// initializer is anything but a plain string constant. A computed initializer here would unstamp
// every published binary while every test still passed, so this builds one and looks.
func TestTheReleaseStampSurvivesTheLinker(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "boxer")
	build := exec.Command("go", "build", "-ldflags", "-X main.Version=v9.9.9-test", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "boxer v9.9.9-test" {
		t.Fatalf("the stamp did not reach the binary: %q", out)
	}
}
