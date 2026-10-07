package main

import (
	"bytes"
	"encoding/json"
	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/inside"
	"github.com/BarakChamo/boxer/internal/vm"
	"github.com/BarakChamo/boxer/internal/vmtest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	if code != 0 || !strings.Contains(out, "sandbox:   stopped, image alpine") || !strings.Contains(out, "runtime:   smolvm 0.0.0-fake") {
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
	client, log := vmtest.Install(t)
	dir := t.TempDir() // no git repository here
	t.Chdir(dir)
	if err := client.Create(vm.CreateSpec{Name: "sb-deadbeef", Labels: map[string]string{"boxer.scope": "sb-deadbeef"}}); err != nil {
		t.Fatal(err)
	}
	// A machine boxer did not make is refused by name, and kept.
	if err := client.Create(vm.CreateSpec{Name: "mydev"}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"down", "--scope", "mydev"}, nil, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "only stops or deletes its own") {
		t.Fatalf("a foreign machine must be refused: %d %s", code, errOut.String())
	}
	if _, ok, _ := client.Status("mydev"); !ok {
		t.Fatal("a foreign machine was deleted")
	}
	out.Reset()
	errOut.Reset()
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
	// The brief names every task once, with its command line when it declares no description.
	code, out := call(t, nil, "brief")
	if code != 0 || !strings.Contains(out, "`boxer run --task build`") || !strings.Contains(out, "`boxer run --task test`") {
		t.Fatalf("prose brief: %d %s", code, out)
	}
	if strings.Count(out, "--task test") != 1 {
		t.Fatalf("a task is named once, not twice:\n%s", out)
	}
	if code, out := call(t, nil, "run", "--task", "test"); code != 0 || !strings.Contains(out, "ran-the-task") {
		t.Fatalf("run --task: %d %s", code, out)
	}
	// A task's env table reaches the command.
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks.greet]\ncmd = \"echo hello-$WHO\"\nenv = { WHO = \"task-env\" }\n")
	if code, out := call(t, nil, "run", "--task", "greet"); code != 0 || !strings.Contains(out, "hello-task-env") {
		t.Fatalf("task env: %d %s", code, out)
	}
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks]\ntest = \"echo ran-the-task\"\nbuild = \"make build\"\n")
	// An unknown name is a refusal an agent can act on: the fix line lists what does exist.
	code, out = call(t, nil, "run", "--task", "tset")
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

// A described task is what an agent chooses between, so the description has to reach both the
// listing and the brief. The JSON keeps `tasks` a name→command map, so a reader written against
// an older boxer still works, and puts the rest in task_details.
func TestTaskTableReachesTheListingAndTheBrief(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+
		"[tasks]\nbuild = \"make build\"\n\n[tasks.test]\ncmd = \"echo ran\"\ndescription = \"the unit suite\"\njunit = [\"junit.xml\"]\n")

	if code, out := call(t, nil, "tasks"); code != 0 || !strings.Contains(out, "the unit suite") || !strings.Contains(out, "make build") {
		t.Fatalf("tasks listing: %d %s", code, out)
	}
	var b map[string]any
	if code, _ := call(t, &b, "brief", "--json"); code != 0 {
		t.Fatalf("brief: %d", code)
	}
	if tasks, _ := b["tasks"].(map[string]any); tasks["test"] != "echo ran" {
		t.Fatalf("tasks stays a name→command map: %v", b["tasks"])
	}
	details, _ := b["task_details"].(map[string]any)
	row, _ := details["test"].(map[string]any)
	if row["description"] != "the unit suite" {
		t.Fatalf("task_details: %v", b["task_details"])
	}
	if _, ok := details["build"]; ok {
		t.Fatalf("a bare string task declares no details: %v", details)
	}
	if code, out := call(t, nil, "brief"); code != 0 || !strings.Contains(out, "the unit suite") {
		t.Fatalf("the brief must name what a task is for: %d %s", code, out)
	}
}

// A test report answers "which tests failed" without scraping the log. It must never turn a
// non-zero exit into a different one, and a report older than the run is not this run's.
func TestJUnitSummaryAndFailOnTestFailures(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	red := `<testsuite name="cart"><testcase name="ok"/><testcase name="bad"><failure message="no"/></testcase></testsuite>`
	write := "printf '%s' '" + red + "' > junit.xml"

	code, out := call(t, nil, "run", "--junit", "junit.xml", "-c", write)
	if code != 0 || !strings.Contains(out, "1 passed, 1 failed") || !strings.Contains(out, "FAIL cart/bad") {
		t.Fatalf("summary: %d %s", code, out)
	}
	if code, out := call(t, nil, "run", "--junit", "junit.xml", "--fail-on-test-failures", "-c", write); code != 1 {
		t.Fatalf("a green command with a red report must exit 1: %d %s", code, out)
	}
	if code, _ := call(t, nil, "run", "--junit", "junit.xml", "--fail-on-test-failures", "-c", write+"; exit 3"); code != 3 {
		t.Fatalf("the command's own exit code must survive: %d", code)
	}
	// The report is now stale: a later run that writes nothing must not inherit its verdict.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "junit.xml"), old, old); err != nil {
		t.Fatal(err)
	}
	if code, out := call(t, nil, "run", "--junit", "junit.xml", "--fail-on-test-failures", "-c", "true"); code != 0 || strings.Contains(out, "FAIL") {
		t.Fatalf("a stale report must be ignored: %d %s", code, out)
	}
}

// A sandbox is addressed by a hash because the hash is what is unique, and read by a person who
// cannot hold one in their head. The slug is derived from the key, so it appears everywhere
// without being stored anywhere, and --scope takes either.
func TestSandboxesAreNamedAsWellAsHashed(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, _ := call(t, nil, "up"); code != 0 {
		t.Fatal("up")
	}
	var st map[string]any
	if code, _ := call(t, &st, "status", "--json"); code != 0 {
		t.Fatalf("status: %d", code)
	}
	name, _ := st["name"].(string)
	key, _ := st["scope"].(string)
	if name == "" || name == key || !strings.HasPrefix(key, "sb-") {
		t.Fatalf("status must carry both: %v", st)
	}
	if code, out := call(t, nil, "ls"); code != 0 || !strings.Contains(out, name) || !strings.Contains(out, key) {
		t.Fatalf("ls: %d %s", code, out)
	}
	// The readable name is accepted wherever the key is.
	if code, out := call(t, nil, "run", "--scope", name, "-c", "echo by-name"); code != 0 || !strings.Contains(out, "by-name") {
		t.Fatalf("run --scope <name>: %d %s", code, out)
	}
	if code, out := call(t, nil, "down", "--scope", name); code != 0 || !strings.Contains(out, "removed") {
		t.Fatalf("down --scope <name>: %d %s", code, out)
	}
}

// gc --all ignores idle_timeout; it does not treat every sandbox as idle. It once did, so a running
// sandbox that had ever run a command was deleted from under the agent using it.
func TestGCAllKeepsRunningSandboxes(t *testing.T) {
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if err := client.Create(vm.CreateSpec{Name: "sb-busy", Labels: map[string]string{"boxer.scope": "sb-busy", "boxer.root": t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	if err := client.Start("sb-busy"); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(box.LastUsedDir(), "sb-busy")
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(stamp, old, old)
	if code, out := call(t, nil, "gc", "--all"); code != 0 {
		t.Fatalf("gc --all: %d %s", code, out)
	}
	if _, ok, _ := client.Status("sb-busy"); !ok {
		t.Fatal("gc --all deleted a running sandbox")
	}
}

// The usage line for `boxer shell` is written by hand; it once left out two harnesses that worked.
func TestUsageNamesEveryInsideHarness(t *testing.T) {
	line := usage[strings.Index(usage, "harnesses: claude"):]
	listed := strings.Split(strings.TrimPrefix(line[:strings.Index(line, "\n")], "harnesses: "), ", ")
	for _, h := range inside.Names() {
		if !slices.Contains(listed, h) {
			t.Errorf("boxer help does not list %q under boxer shell: %v", h, listed)
		}
	}
}

// Idle reclaim stops a sandbox whose worktree is still there rather than deleting it, and a named
// volume outlives the sandbox until its worktree goes or rm --volumes asks.
// gc never takes a sandbox a command is provisioning: the command holds the scope's lock, and gc
// skips what it cannot lock rather than stopping it from under the command.
func TestGCSkipsASandboxInUse(t *testing.T) {
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"1m\"\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	if code, out := call(t, &ls, "ls", "--json"); code != 0 || len(ls) != 1 {
		t.Fatalf("ls: %d %s", code, out)
	}
	key := ls[0]["scope"].(string)
	ageStamp(t, key)
	unlock, ok := box.TryLockScope(key)
	if !ok {
		t.Fatal("nothing else holds the lock")
	}
	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 0 {
		t.Fatalf("a locked sandbox must be left alone: %d %v %s", code, rows, out)
	}
	unlock()
	if m, ok, _ := client.Status(key); !ok || !m.Running() {
		t.Fatalf("it must still be running: %v %v", ok, m.State)
	}
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 1 {
		t.Fatalf("once free, an idle sandbox is reclaimed: %d %v %s", code, rows, out)
	}
}

func TestIdleReclaimStopsAndVolumesSurvive(t *testing.T) {
	old := box.VolumeGrace
	box.VolumeGrace = 0 // the week a missing worktree is given is tested on its own
	t.Cleanup(func() { box.VolumeGrace = old })
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"1m\"\nvolumes = [\"data:/data\"]\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	if code, out := call(t, &ls, "ls", "--json"); code != 0 || len(ls) != 1 {
		t.Fatalf("ls: %d %s", code, out)
	}
	key := ls[0]["scope"].(string)
	vol := filepath.Join(box.VolumeDir(), key, "data")
	if err := os.WriteFile(filepath.Join(vol, "row"), []byte("1"), 0o644); err != nil {
		t.Fatalf("the volume must exist on the host once the sandbox does: %v", err)
	}
	ageStamp(t, key)
	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 1 || rows[0]["stopped"] != true {
		t.Fatalf("an idle sandbox with a worktree must be stopped: %d %v %s", code, rows, out)
	}
	if m, ok, _ := client.Status(key); !ok || m.Running() {
		t.Fatalf("it must still exist, stopped: %v %v", ok, m.State)
	}
	if code, _ := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 0 {
		t.Fatalf("a stopped idle sandbox is left alone: %v", rows)
	}

	// rm deletes the sandbox and keeps the volume; the next sandbox sees the same data.
	if code, out := call(t, nil, "rm", key); code != 0 {
		t.Fatal(out)
	}
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if b, err := os.ReadFile(filepath.Join(vol, "row")); err != nil || string(b) != "1" {
		t.Fatalf("the volume must survive rm and recreate: %v", err)
	}
	if code, out := call(t, nil, "rm", "--volumes", key); code != 0 {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(box.VolumeDir(), key)); !os.IsNotExist(err) {
		t.Fatalf("rm --volumes must delete them: %v", err)
	}

	// A volume whose worktree is gone goes at the next gc, sandbox or not.
	orphan := filepath.Join(box.VolumeDir(), "sb-orphan")
	_ = os.MkdirAll(filepath.Join(orphan, "data"), 0o755)
	_ = os.WriteFile(filepath.Join(orphan, ".worktree"), []byte(filepath.Join(t.TempDir(), "removed")), 0o644)
	unknown := filepath.Join(box.VolumeDir(), "sb-unknown")
	_ = os.MkdirAll(unknown, 0o755)
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 1 || rows[0]["volumes"] != "sb-orphan" {
		t.Fatalf("gc must delete the orphaned volumes, and only them: %d %v %s", code, rows, out)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphaned volumes survived gc")
	}
	if _, err := os.Stat(unknown); err != nil {
		t.Fatal("volumes boxer cannot place must be left alone")
	}
}

// idle_action = "delete" is the 1.1 behaviour.
func TestIdleActionDelete(t *testing.T) {
	client, _ := vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"1m\"\nidle_action = \"delete\"\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	call(t, &ls, "ls", "--json")
	key := ls[0]["scope"].(string)
	ageStamp(t, key)
	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 1 || rows[0]["deleted"] != true {
		t.Fatalf("gc: %d %v %s", code, rows, out)
	}
	if _, ok, _ := client.Status(key); ok {
		t.Fatal(`idle_action = "delete" must delete`)
	}
}

// ageStamp records the scope as last used an hour ago.
func ageStamp(t *testing.T, key string) {
	t.Helper()
	stamp := filepath.Join(box.LastUsedDir(), key)
	_ = os.MkdirAll(filepath.Dir(stamp), 0o755)
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stamp, old, old); err != nil {
		t.Fatal(err)
	}
}

// A backend that does not answer is not a host with no sandboxes: ls, stop and rm say so with
// exit 1 rather than print an empty list and exit 0.
func TestListingFailureIsNotAnEmptyList(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	vmtest.FailVerb(t, "machine ls", "daemon not reachable")
	var rows []map[string]any
	if code, out := call(t, &rows, "ls", "--json"); code != 1 || len(rows) != 0 {
		t.Fatalf("ls --json: %d %v %s", code, rows, out)
	}
	for _, verb := range []string{"stop", "rm"} {
		if code, out := call(t, nil, verb, "--all", "-y", "--json"); code != 1 {
			t.Fatalf("%s --all must fail when the backend does: %d %s", verb, code, out)
		}
	}
}

// down --all goes on past a sandbox it cannot delete, and reports every one it acted on.
func TestDownAllContinuesPastAFailure(t *testing.T) {
	vmtest.Install(t)
	for range 2 {
		vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
		if code, out := call(t, nil, "up"); code != 0 {
			t.Fatal(out)
		}
	}
	vmtest.FailVerb(t, "machine delete", "disk on fire")
	var rows []map[string]any
	code, out := call(t, &rows, "down", "--all", "--json")
	if code != 1 || len(rows) != 2 || rows[0]["error"] == nil || rows[1]["error"] == nil {
		t.Fatalf("each failure must be a row, and the exit 1: %d %v %s", code, rows, out)
	}
}

// A flag a command does not read is refused, not ignored: `restart --scope <child>` exited 0 having
// restarted the current worktree's services.
func TestScopedCommandsRefuseFlagsTheyIgnore(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	for _, args := range [][]string{{"restart", "--scope", "sb-000000000000"}, {"doctor", "--all"}, {"up", "--scope", "x"}, {"restart", "--json"}} {
		if code, out := call(t, nil, args...); code != 2 || !strings.Contains(out, "does not take --") {
			t.Fatalf("%v: %d %s", args, code, out)
		}
	}
	if code, out := call(t, nil, "doctor", "--json", "--session", "s"); code != 0 {
		t.Fatalf("flags a command reads still work: %d %s", code, out)
	}
}

// git and conductor have no user layer; --user used to act on paths relative to the current
// directory when run outside a repository.
func TestGitAndConductorRefuseUser(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, verb := range []string{"install", "uninstall"} {
		for _, target := range []string{"git", "conductor"} {
			if code, out := call(t, nil, verb, target, "--user"); code != 2 || !strings.Contains(out, "no user layer") {
				t.Fatalf("%s %s --user: %d %s", verb, target, code, out)
			}
		}
	}
}

// Each sandbox is reclaimed by its own repository's rules. The sweep a command starts used to run
// with that command's configuration, so one repository's idle_action = "delete" deleted the
// sandboxes of every other.
func TestGCJudgesEachSandboxByItsOwnRepository(t *testing.T) {
	client, _ := vmtest.Install(t)
	keep := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"never\"\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"1m\"\nidle_action = \"delete\"\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	call(t, &ls, "ls", "--json")
	var kept string
	for _, r := range ls {
		ageStamp(t, r["scope"].(string))
		if r["worktree"] == keep || strings.HasSuffix(keep, r["worktree"].(string)) || strings.HasSuffix(r["worktree"].(string), filepath.Base(keep)) {
			kept = r["scope"].(string)
		}
	}
	if kept == "" || len(ls) != 2 {
		t.Fatalf("could not tell the two sandboxes apart: %v", ls)
	}
	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 1 || rows[0]["scope"] == kept {
		t.Fatalf("only the repository that asked for deletion loses its sandbox: %d %v %s", code, rows, out)
	}
	if _, ok, _ := client.Status(kept); !ok {
		t.Fatal("a sandbox whose repository says never must survive another's sweep")
	}
}

// down --scope forgets what every other delete forgets.
func TestDownScopeForgetsTheSandbox(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "run", "--", "true"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	call(t, &ls, "ls", "--json")
	key := ls[0]["scope"].(string)
	if box.LastUsed(key).IsZero() {
		t.Fatal("a run marks the sandbox used")
	}
	if code, out := call(t, nil, "down", "--scope", key); code != 0 {
		t.Fatal(out)
	}
	if !box.LastUsed(key).IsZero() {
		t.Fatal("down --scope left the last-used stamp behind")
	}
	if _, ok := box.ReadRunRecord(key); ok {
		t.Fatal("down --scope left the run record behind")
	}
}

// A recreated sandbox is marked used: recreating deleted the stamp, and one with no stamp was never
// found idle.
func TestRecreateKeepsTheSandboxReclaimable(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if code, out := call(t, nil, "up", "--recreate"); code != 0 {
		t.Fatal(out)
	}
	var ls []map[string]any
	call(t, &ls, "ls", "--json")
	if box.LastUsed(ls[0]["scope"].(string)).IsZero() {
		t.Fatal("a recreated sandbox has no last-used stamp")
	}
}

// An idle parent is kept while a fork child is in use: deleting it takes the children with it.
func TestGCKeepsAParentWhoseChildIsInUse(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"idle_timeout = \"1m\"\nidle_action = \"delete\"\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var names []string
	if code, out := call(t, &names, "fork", "--prepare", "--count", "1", "--json"); code != 0 || len(names) != 1 {
		t.Fatalf("fork: %d %s", code, out)
	}
	parent := box.ParentOf(names[0])
	ageStamp(t, parent)
	if code, out := call(t, nil, "run", "--scope", names[0], "--", "true"); code != 0 {
		t.Fatal(out)
	}
	var rows []map[string]any
	if code, out := call(t, &rows, "gc", "--json"); code != 0 || len(rows) != 0 {
		t.Fatalf("a parent whose child is in use must be kept: %d %v %s", code, rows, out)
	}
}
