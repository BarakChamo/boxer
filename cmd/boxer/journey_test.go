package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

// fireHook feeds one hook event to the dialect and returns what the harness would see.
func fireHook(t *testing.T, harness string, in map[string]any) (string, int) {
	t.Helper()
	b, _ := json.Marshal(in)
	var out, errb bytes.Buffer
	code := run([]string{"hook", harness}, bytes.NewReader(b), &out, &errb)
	return out.String() + errb.String(), code
}

// A session, end to end, as the agent experiences it: the repository is prepared, a session
// begins, the agent is briefed, it runs a build command, it runs a declared task, it asks what
// the sandbox is doing, and the session ends. Each step is a separate command in real life and a
// separate mechanism in the code; what this test holds is that they agree with each other.
//
// Every assertion here is a behaviour a user or an agent can observe. None of them reach into a
// package to check a field.
func TestASessionFromInstallToReclaim(t *testing.T) {
	_, log := vmtest.Install(t)
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("BOXER_PACKS", t.TempDir())
	t.Setenv("BOXER_NO_RECLAIM", "1") // the sweep has its own test; here it would race the log
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[tasks]\ntest = \"echo ran-the-task\"\n")

	// 1. The repository is prepared for one harness. The agent-visible half is a skill with
	//    scripts it can run; the enforcement half is a hook entry in the harness's settings.
	if code, out := call(t, nil, "install", "claude-code"); code != 0 {
		t.Fatalf("install: %d %s", code, out)
	}
	for _, p := range []string{".claude/settings.json", ".claude/skills/boxer/SKILL.md", ".claude/skills/boxer/scripts/task", ".mcp.json"} {
		st, err := os.Stat(filepath.Join(dir, p))
		if err != nil {
			t.Fatalf("install did not write %s: %v", p, err)
		}
		if strings.Contains(p, "scripts/") && st.Mode()&0o111 == 0 {
			t.Fatalf("%s must be executable: the agent runs it", p)
		}
	}

	// 2. The session starts. The agent is told what this repository expects, without being asked.
	out, code := fireHook(t, "claude-code", map[string]any{"hook_event_name": "SessionStart", "cwd": dir, "session_id": "s1"})
	if code != 0 || !strings.Contains(out, "boxer sandbox") || !strings.Contains(out, "boxer run --task") {
		t.Fatalf("session start must brief the agent and name the tasks: %d %s", code, out)
	}

	// 3. The agent runs a build command. It is rewritten, silently, into a sandboxed one.
	out, code = fireHook(t, "claude-code", map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "npm test"}, "cwd": dir, "session_id": "s1"})
	if code != 0 || !strings.Contains(out, `boxer run -c 'npm test'`) {
		t.Fatalf("an intercepted command must be rewritten: %d %s", code, out)
	}

	// 4. A command on the passthrough list is left alone, because it needs the host's credentials.
	out, code = fireHook(t, "claude-code", map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "git status"}, "cwd": dir, "session_id": "s1"})
	if code != 0 || strings.Contains(out, "boxer run") {
		t.Fatalf("git must stay on the host: %d %s", code, out)
	}

	// 5. The rewritten command runs, and so does the declared task. Both reach the guest. (The
	// fake smolvm executes what it is given, so the command here is a real one that costs
	// nothing: what matters is which side of the boundary it ran on.)
	if code, out := call(t, nil, "run", "-c", "go version"); code != 0 {
		t.Fatalf("run: %d %s", code, out)
	}
	if code, out := call(t, nil, "run", "--task", "test"); code != 0 || !strings.Contains(out, "ran-the-task") {
		t.Fatalf("run --task: %d %s", code, out)
	}
	// An unknown task is refused with the real ones, because a refusal an agent cannot act on is
	// worse than no refusal at all.
	if code, out := call(t, nil, "run", "--task", "tset"); code != 1 || !strings.Contains(out, "fix:") || !strings.Contains(out, "test") {
		t.Fatalf("unknown task: %d %s", code, out)
	}

	// 6. The agent asks what is going on. Both channels answer, and they agree.
	var st map[string]any
	if code, out := call(t, &st, "status", "--json"); code != 0 || st["state"] != "running" {
		t.Fatalf("status: %d %s %v", code, out, st)
	}
	var brief map[string]any
	if code, _ := call(t, &brief, "brief", "--json"); code != 0 || brief["mount_at"] != st["mount_at"] {
		t.Fatalf("brief and status must agree on the mount: %v vs %v", brief["mount_at"], st["mount_at"])
	}
	if tasks, _ := brief["tasks"].(map[string]any); tasks["test"] != "echo ran-the-task" {
		t.Fatalf("the brief carries the tasks: %v", brief["tasks"])
	}

	// 7. The session ends and the sandbox is reclaimed. What the guest saw is in the log; what the
	//    host ran is not.
	if code, out := call(t, nil, "down"); code != 0 {
		t.Fatalf("down: %d %s", code, out)
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "go version") || !strings.Contains(string(b), "echo ran-the-task") {
		t.Fatalf("both commands should have reached the guest:\n%s", b)
	}
	if code, _ := call(t, &st, "status", "--json"); code == 0 && st["state"] == "running" {
		t.Fatal("the sandbox should be gone")
	}
}

// The same journey with the sandbox turned off: boxer must get out of the way completely, because
// a repository that opts out has to behave as though boxer were not installed.
func TestModeOffChangesNothing(t *testing.T) {
	_, log := vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"mode = \"off\"\n")

	out, code := fireHook(t, "claude-code", map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "npm test"}, "cwd": dir, "session_id": "s1"})
	if code != 0 || strings.Contains(out, "boxer run") || strings.Contains(out, "deny") {
		t.Fatalf("mode off must neither rewrite nor deny: %d %s", code, out)
	}
	b, _ := os.ReadFile(log)
	if strings.Contains(string(b), "machine create") {
		t.Fatalf("mode off must not provision anything:\n%s", b)
	}
}

// Tool mode is for a harness that can refuse a command but not rewrite one. The refusal has to
// name the way in, or the agent has no move but to give up or work around the sandbox.
func TestToolModeRefusesWithTheWayIn(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"mode = \"tool\"\n")

	out, code := fireHook(t, "claude-code", map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "npm test"}, "cwd": dir, "session_id": "s1"})
	if code != 0 || !strings.Contains(out, "deny") || !strings.Contains(out, "boxer_run") {
		t.Fatalf("tool mode must deny and name boxer_run: %d %s", code, out)
	}
}

// Installing is the command a user runs once per repository, and its failure modes are the ones
// they meet first: the wrong place, an unknown harness, a harness with no project layer, and the
// two special targets that are not harnesses at all.
func TestInstallPathsAndRefusals(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)

	if code, out := call(t, nil, "install"); code != 2 || !strings.Contains(out, "usage:") {
		t.Fatalf("no harness named: %d %s", code, out)
	}
	if code, out := call(t, nil, "install", "nosuchharness"); code == 0 || !strings.Contains(out, "known:") || !strings.Contains(out, "claude-code") {
		t.Fatalf("an unknown harness must list the known ones: %d %s", code, out)
	}
	// Copilot's hooks load only from a trusted directory, which headless mode does not grant, so
	// the project layer refuses and names the flag that works.
	if code, out := call(t, nil, "install", "copilot"); code == 0 || !strings.Contains(out, "--user") {
		t.Fatalf("copilot's refusal must name --user: %d %s", code, out)
	}

	// `all` writes every harness this repository can take, and is idempotent: a user runs it again
	// after every upgrade, and the second run must add nothing. One install writes one hook entry
	// per lifecycle event, so the invariant is that the count does not grow.
	if code, out := call(t, nil, "install", "all"); code != 0 {
		t.Fatalf("install all: %d %s", code, out)
	}
	settings := filepath.Join(dir, ".claude", "settings.json")
	first, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if code, out := call(t, nil, "install", "all"); code != 0 {
		t.Fatalf("install all, again: %d %s", code, out)
	}
	second, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("a second install changed the settings:\n%s\n---\n%s", first, second)
	}
	if n := strings.Count(string(second), "boxer hook claude-code"); n == 0 {
		t.Fatalf("no hook entry at all: %s", second)
	}

	// git is not a harness: it is a post-checkout hook that warms a worktree the moment git makes
	// one, which is what an orchestrator needs.
	if code, out := call(t, nil, "install", "git"); code != 0 || !strings.Contains(out, "wrote") {
		t.Fatalf("install git: %d %s", code, out)
	}
	hookFile := filepath.Join(dir, ".git", "hooks", "post-checkout")
	b, err := os.ReadFile(hookFile)
	if err != nil || !strings.Contains(string(b), "boxer up --detach") {
		t.Fatalf("post-checkout must warm the new worktree: %v %s", err, b)
	}

	// Conductor is an orchestrator, not a harness: its file points at boxer's shims.
	if code, out := call(t, nil, "install", "conductor"); code != 0 || !strings.Contains(out, "settings.toml") {
		t.Fatalf("install conductor: %d %s", code, out)
	}
}

// Running outside a git repository is the other thing every command has to survive, because an
// agent's shell starts wherever the user left it.
func TestCommandsOutsideARepository(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	vmtest.Repo(t, vmtest.NoWorktreeCheck) // sets the isolation env, then we leave it
	t.Chdir(t.TempDir())

	for _, args := range [][]string{{"up"}, {"status"}, {"brief"}, {"tasks"}, {"run", "-c", "true"}} {
		code, out := call(t, nil, args...)
		if code == 0 {
			t.Fatalf("%v outside a repository should fail: %s", args, out)
		}
		if !strings.Contains(out, "boxer:") || !strings.Contains(out, "fix:") {
			t.Fatalf("%v must fail with a reason and a fix: %s", args, out)
		}
	}
	// ls and gc are host-wide: they work anywhere, because they are how you find what is left.
	if code, out := call(t, nil, "ls"); code != 0 {
		t.Fatalf("ls is host-wide: %d %s", code, out)
	}
	if code, out := call(t, nil, "gc", "--dry-run"); code != 0 {
		t.Fatalf("gc is host-wide: %d %s", code, out)
	}
}

// doctor answers "will this worktree have to run setup again", which before environment packs
// could only be learned by watching a VM boot.
func TestDoctorReportsTheEnvironmentCache(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"image = \"alpine\"\nsetup = [\"echo hi\"]\n")

	var r map[string]any
	if code, out := call(t, &r, "doctor", "--json"); code != 0 {
		t.Fatalf("doctor: %d %s", code, out)
	}
	env, _ := r["environment"].(map[string]any)
	if env == nil || env["cached"] != false || env["key"] == "" {
		t.Fatalf("an uncached environment must say so: %v", r["environment"])
	}
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	if code, out := call(t, &r, "doctor", "--json"); code != 0 {
		t.Fatalf("doctor: %d %s", code, out)
	}
	if env, _ = r["environment"].(map[string]any); env["cached"] != true {
		t.Fatalf("after one sandbox the environment is cached: %v", r["environment"])
	}
	// The human form says it in words, because that is the form someone reads when a worktree took
	// longer than they expected.
	if code, out := call(t, nil, "doctor"); code != 0 || !strings.Contains(out, "environment:") || !strings.Contains(out, "skips setup") {
		t.Fatalf("doctor must say the environment is cached: %d %s", code, out)
	}
	// A repository with no setup has no environment to report. (A fresh map: unmarshalling into
	// one that already has keys merges rather than replaces, which would silently keep the old
	// answer.)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"image = \"alpine\"\n")
	var plain map[string]any
	if code, _ := call(t, &plain, "doctor", "--json"); code != 0 || plain["environment"] != nil {
		t.Fatalf("no setup, no environment row: %v", plain["environment"])
	}
}

// A listing that says "sb-4f2a" is a list of hashes; one that says which harness and session it
// belongs to is something a person can act on. The identity is in the scope key already, but a
// hash cannot be read back, so it is recorded as labels when the harness says.
func TestListingNamesWhoASandboxBelongsTo(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)

	if code, out := call(t, nil, "up", "--harness", "claude-code", "--session", "4f2a9c11"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	var rows []map[string]any
	if code, out := call(t, &rows, "ls", "--json"); code != 0 || len(rows) != 1 {
		t.Fatalf("ls: %d %v %s", code, rows, out)
	}
	if rows[0]["harness"] != "claude-code" || rows[0]["session"] != "4f2a9c11" {
		t.Fatalf("the listing must name the harness and session: %v", rows[0])
	}
	if code, out := call(t, nil, "ls", "--resources"); code != 0 || !strings.Contains(out, "claude-code/4f2a9c") {
		t.Fatalf("the human listing shows the attachment: %d %s", code, out)
	}

	// Resources are measured only when asked for, because measuring costs a process call and a
	// directory walk per machine.
	if code, _ := call(t, &rows, "ls", "--json"); code != 0 || rows[0]["resources"] != nil {
		t.Fatalf("a plain listing must not measure: %v", rows[0]["resources"])
	}
	var measured []map[string]any
	if code, out := call(t, &measured, "ls", "--resources", "--json"); code != 0 || measured[0]["resources"] == nil {
		t.Fatalf("--resources must measure: %d %s", code, out)
	}
	res := measured[0]["resources"].(map[string]any)
	if res["cpus"].(float64) <= 0 || res["memory_mib"].(float64) <= 0 {
		t.Fatalf("allocation comes from smolvm: %v", res)
	}
}

// watch is the live channel an interface needs: one process to tail rather than a poll loop. It
// is line-delimited JSON on purpose — a stream has no closing bracket, and a consumer must be able
// to read each line as it arrives.
func TestWatchStreamsTheLifecycle(t *testing.T) {
	client, _ := vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("BOXER_PACKS", t.TempDir())
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)

	r, w := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- run([]string{"watch", "--json", "--interval", "50ms"}, strings.NewReader(""), w, io.Discard)
	}()
	t.Cleanup(func() { _ = r.Close() })

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()

	if code, out := call(t, nil, "up", "--harness", "claude-code", "--session", "s1"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	changes := map[string]string{}
	deadline := time.After(10 * time.Second)
	for len(changes) < 1 {
		select {
		case line := <-lines:
			var e struct {
				Change  string `json:"change"`
				Sandbox struct {
					Scope   string `json:"scope"`
					Harness string `json:"harness"`
				} `json:"sandbox"`
			}
			if json.Unmarshal([]byte(line), &e) != nil {
				t.Fatalf("every line must be its own JSON document: %q", line)
			}
			if e.Sandbox.Harness != "claude-code" {
				t.Fatalf("the stream must name who the sandbox belongs to: %q", line)
			}
			changes[e.Change] = e.Sandbox.Scope
		case <-deadline:
			t.Fatalf("no change seen; got %v", changes)
		}
	}
	if changes["created"] == "" && changes["present"] == "" && changes["running"] == "" {
		t.Fatalf("a new sandbox must appear in the stream: %v", changes)
	}
	// Deleting it ends in "gone", which is what tells an interface to drop the row.
	scope := changes["created"]
	if scope == "" {
		scope = changes["running"]
	}
	if err := client.Delete(scope); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case line := <-lines:
			if strings.Contains(line, `"change":"gone"`) {
				return
			}
		case <-time.After(10 * time.Second):
			t.Fatal("a deleted sandbox must be reported gone")
		}
	}
}

// --rebuild drops the cached environment, because a cache you cannot drop is a liability: an
// install that depends on something outside the setup list needs a way to start again.
func TestUpRebuildDropsTheCachedEnvironment(t *testing.T) {
	vmtest.Install(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	packs := t.TempDir()
	t.Setenv("BOXER_PACKS", packs)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"image = \"alpine\"\nsetup = [\"echo installing\"]\n")

	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatalf("up: %d %s", code, out)
	}
	var r map[string]any
	call(t, &r, "doctor", "--json")
	if env, _ := r["environment"].(map[string]any); env["cached"] != true {
		t.Fatalf("the first sandbox caches its environment: %v", r["environment"])
	}
	if code, out := call(t, nil, "up", "--rebuild"); code != 0 || !strings.Contains(out, "dropped the cached environment") {
		t.Fatalf("--rebuild must drop it and say so: %d %s", code, out)
	}
	// It is cached again afterwards, because the rebuild ran setup and packed the result.
	var after map[string]any
	call(t, &after, "doctor", "--json")
	if env, _ := after["environment"].(map[string]any); env["cached"] != true {
		t.Fatalf("a rebuild leaves a fresh cache: %v", after["environment"])
	}
}

// One stream, two sources: a state change says a VM started, an event says what it was asked to
// do. A consumer should read one thing rather than correlating two.
func TestWatchCarriesEventsAsWellAsStateChanges(t *testing.T) {
	vmtest.Install(t)
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("BOXER_PACKS", t.TempDir())
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"[telemetry]\nenabled = true\n")

	r, w := io.Pipe()
	go func() { run([]string{"watch", "--json", "--interval", "50ms"}, strings.NewReader(""), w, io.Discard) }()
	t.Cleanup(func() { _ = r.Close() })
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()

	if code, out := call(t, nil, "run", "-c", "true"); code != 0 {
		t.Fatalf("run: %d %s", code, out)
	}
	deadline := time.After(15 * time.Second)
	for {
		select {
		case line := <-lines:
			if strings.Contains(line, `"change":"event"`) && strings.Contains(line, `"event"`) {
				return
			}
		case <-deadline:
			t.Fatal("the stream must carry events, not only state changes")
		}
	}
}
