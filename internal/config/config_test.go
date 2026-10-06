package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLayering(t *testing.T) {
	d := t.TempDir()
	user := write(t, d, "user.toml", "image = \"debian:bookworm-slim\"\ncpus = 2\n")
	repo := write(t, d, "repo.toml", "isolation = \"session\"\nsetup = [\"bun install\"]\n[harness.gemini-cli]\nmode = \"tool\"\n")
	wt := write(t, d, "wt.toml", "cpus = 8\n")

	cfg, err := LoadFiles(user, repo, wt)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CPUs != 8 || cfg.Sources["cpus"] != wt {
		t.Fatalf("worktree should win cpus: %d from %s", cfg.CPUs, cfg.Sources["cpus"])
	}
	if cfg.Image != "debian:bookworm-slim" || cfg.Sources["image"] != user {
		t.Fatalf("user image should survive: %q", cfg.Image)
	}
	if cfg.Isolation != "session" || len(cfg.Setup) != 1 {
		t.Fatalf("repo values: %+v", cfg)
	}
	if cfg.Mode != "rewrite" || cfg.Sources["mode"] != "" {
		t.Fatalf("default mode: %q", cfg.Mode)
	}
	g := cfg.ForHarness("gemini-cli")
	if g.Mode != "tool" || g.Sources["mode"] != "harness.gemini-cli" || cfg.Mode != "rewrite" {
		t.Fatalf("harness override: %q / original %q", g.Mode, cfg.Mode)
	}
	if len(cfg.Files) != 3 {
		t.Fatalf("files read: %v", cfg.Files)
	}
}

func TestUnknownKeyIsError(t *testing.T) {
	d := t.TempDir()
	p := write(t, d, "b.toml", "enforcment = \"both\"\n")
	_, err := LoadFiles(p)
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("want unknown key error, got %v", err)
	}
}

// A [harness.<name>] override is held to the same enumerations as the top-level key it replaces:
// a typo in its enforcement or isolation must not quietly fall back to something weaker.
func TestHarnessOverrideEnumsAreChecked(t *testing.T) {
	d := t.TempDir()
	for _, c := range []struct{ body, want string }{
		{"[harness.claude-code]\nenforcement = \"hoook\"\n", "harness.claude-code.enforcement"},
		{"[harness.codex]\nisolation = \"sesion\"\n", "harness.codex.isolation"},
		{"[harness.kimi]\nmode = \"tools\"\n", "harness.kimi.mode"},
	} {
		p := write(t, d, "h.toml", c.body)
		if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "allowed:") {
			t.Errorf("%q: want %s error with the allowed values, got %v", c.body, c.want, err)
		}
	}
	p := write(t, d, "ok.toml", "[harness.codex]\nenforcement = \"hook\"\nisolation = \"session\"\nmode = \"tool\"\n")
	if _, err := LoadFiles(p); err != nil {
		t.Fatalf("valid override refused: %v", err)
	}
}

// 1.0 accepted these two keys, and neither did anything. They still load, so a boxer.toml 1.0 read
// keeps working, and each one says what to use instead.
func TestDeprecatedKeysLoadWithAWarning(t *testing.T) {
	d := t.TempDir()
	for _, c := range []struct{ line, want string }{
		{"require_linked_worktree = true", `require_worktree = "require"`},
		{"reuse_existing = false", "always reuses"},
	} {
		p := write(t, d, "r.toml", c.line+"\n")
		cfg, err := LoadFiles(p)
		if err != nil {
			t.Fatalf("%s: a key 1.0 accepted must still load: %v", c.line, err)
		}
		if !strings.Contains(strings.Join(cfg.Warnings, "\n"), c.want) {
			t.Errorf("%s: want a warning naming %q, got %v", c.line, c.want, cfg.Warnings)
		}
	}
}

func TestEnumIsError(t *testing.T) {
	d := t.TempDir()
	p := write(t, d, "b.toml", "mode = \"rewrtie\"\n")
	if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("want enum error, got %v", err)
	}
	p = write(t, d, "c.toml", "create_on = [\"worktree_create\"]\n")
	if _, err := LoadFiles(p); err == nil {
		t.Fatal("unknown create_on event must fail")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("BOXER_MODE", "off")
	t.Setenv("BOXER_CPUS", "1")
	cfg, err := LoadFiles()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "off" || cfg.Sources["mode"] != "BOXER_MODE" || cfg.CPUs != 1 {
		t.Fatalf("env: %+v", cfg)
	}
	t.Setenv("BOXER_MODE", "bogus")
	if _, err := LoadFiles(); err == nil {
		t.Fatal("bogus env must fail validation")
	}
}

func TestMemory(t *testing.T) {
	for in, want := range map[string]int{"4G": 4096, "512M": 512, "2048": 2048, "1GiB": 1024} {
		got, err := MemoryMiB(in)
		if err != nil || got != want {
			t.Fatalf("%s: %d %v", in, got, err)
		}
	}
	if _, err := MemoryMiB("lots"); err == nil {
		t.Fatal("bad memory must fail")
	}
}

func TestMissingFilesAreFine(t *testing.T) {
	cfg, err := LoadFiles(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || cfg.Mode != "rewrite" {
		t.Fatal(err)
	}
}

func TestTimingKeys(t *testing.T) {
	d := t.TempDir()
	p := write(t, d, "a.toml", "warm_on_session_start = true\n[worktree]\nmanage = \"detect\"\n")
	cfg, err := LoadFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.WarmOnSessionStart || cfg.Worktree.Manage != "detect" {
		t.Fatalf("keys not read: %+v", cfg)
	}
	p = write(t, d, "b.toml", "[worktree]\nmanage = \"auto\"\n")
	if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), "worktree.manage") {
		t.Fatalf("want enum error, got %v", err)
	}
	if Defaults().WarmOnSessionStart || Defaults().Worktree.Manage != "off" {
		t.Fatal("defaults: warm off, manage off")
	}
}

// Load is the real entry point: user file, then repository, then worktree, then the environment,
// each overriding the last, with provenance recorded so `doctor` can explain every value.
func TestLoadLayersUserRepoWorktreeAndEnv(t *testing.T) {
	userDir := t.TempDir()
	os.MkdirAll(filepath.Join(userDir, "boxer"), 0o755)
	os.WriteFile(filepath.Join(userDir, "boxer", "boxer.toml"),
		[]byte("memory = \"1G\"\ncpus = 1\nisolation = \"repo\"\n"), 0o644)
	t.Setenv("XDG_CONFIG_HOME", userDir)

	repoRoot := t.TempDir()
	os.WriteFile(filepath.Join(repoRoot, "boxer.toml"), []byte("cpus = 2\nmode = \"tool\"\n"), 0o644)

	worktree := t.TempDir()
	os.WriteFile(filepath.Join(worktree, "boxer.toml"), []byte("mode = \"off\"\n"), 0o644)

	t.Setenv("BOXER_MEMORY", "8G")

	cfg, err := Load(worktree, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ got, want, why string }{
		{cfg.Isolation, "repo", "user file wins where nothing else sets the key"},
		{fmt.Sprint(cfg.CPUs), "2", "repository overrides the user file"},
		{cfg.Mode, "off", "worktree overrides the repository"},
		{cfg.Memory, "8G", "the environment overrides every file"},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.why, c.got, c.want)
		}
	}
	if len(cfg.Files) != 3 {
		t.Errorf("every layer that existed should be recorded: %v", cfg.Files)
	}
	if src := cfg.Sources["mode"]; !strings.Contains(src, worktree) {
		t.Errorf("provenance for mode should name the worktree file, got %q", src)
	}
	if src := cfg.Sources["memory"]; src != "BOXER_MEMORY" {
		t.Errorf("provenance for memory should name the variable, got %q", src)
	}
}

// A repository with no boxer.toml anywhere is the common case and must not error.
func TestLoadWithNoFilesIsDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load(t.TempDir(), t.TempDir())
	if err != nil || cfg.Isolation != Defaults().Isolation || len(cfg.Files) != 0 {
		t.Fatalf("%v %+v", err, cfg.Files)
	}
}

// A repository's boxer.toml outlives the binary that reads it, so a table this release does not
// know is a warning rather than a refusal: an older boxer must still run a newer checkout. A
// misspelled scalar key stays fatal, because silently ignoring `mod = "off"` would silently
// change what is enforced.
func TestUnknownTablesWarnAndUnknownKeysFail(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "boxer.toml")
	if err := os.WriteFile(p, []byte("[tasks]\ntest = \"make test\"\n\n[fromafuturerelease]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["test"].Cmd != "make test" {
		t.Fatalf("tasks: %v", cfg.Tasks)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "[fromafuturerelease]") {
		t.Fatalf("unknown table must warn: %v", cfg.Warnings)
	}
	if err := os.WriteFile(p, []byte("mod = \"off\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("a misspelled key must fail: %v", err)
	}
	if err := os.WriteFile(p, []byte("[tasks]\ntest = \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFiles(p); err == nil {
		t.Fatal("an empty task command must fail")
	}
}

// [telemetry] is off by default, parsed from the file, and overridable from the environment for a
// harness that offers nothing else. An unknown sink is a configuration error, not a silent "none".
func TestTelemetryTable(t *testing.T) {
	cfg := Defaults()
	if cfg.Telemetry.Enabled || cfg.Telemetry.Sink != "none" {
		t.Fatalf("telemetry must be off by default: %+v", cfg.Telemetry)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "boxer.toml")
	os.WriteFile(p, []byte("[telemetry]\nenabled = true\nsink = \"file\"\npath = \"/tmp/e.jsonl\"\nrecord_commands = true\n"), 0o644)
	cfg, err := LoadFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Telemetry.Enabled || cfg.Telemetry.Sink != "file" || cfg.Telemetry.Path != "/tmp/e.jsonl" || !cfg.Telemetry.RecordCommands {
		t.Fatalf("parsed: %+v", cfg.Telemetry)
	}
	bad := filepath.Join(dir, "bad.toml")
	os.WriteFile(bad, []byte("[telemetry]\nsink = \"carrier pigeon\"\n"), 0o644)
	if _, err := LoadFiles(bad); err == nil {
		t.Fatal("an unknown sink must be rejected")
	}
	t.Setenv("BOXER_TELEMETRY_SINK", "stderr")
	cfg, err = LoadFiles(p)
	if err != nil || cfg.Telemetry.Sink != "stderr" || cfg.Sources["telemetry"] != "BOXER_TELEMETRY_SINK" {
		t.Fatalf("environment override: %+v %v", cfg.Telemetry, cfg.Sources)
	}
}

// A task was a command string before it was a table, and a repository that wrote the string form
// must keep working. The table form is how a task says what it is for, where its JUnit XML lands,
// and how long it may take.
func TestTasksAcceptBothSpellings(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "boxer.toml")
	body := `
[tasks]
test = "go test ./..."

[tasks.e2e]
cmd         = "npm run e2e"
description = "the browser suite"
junit       = ["reports/junit.xml"]
timeout     = "10m"
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["test"].Cmd != "go test ./..." || cfg.Tasks["test"].Description != "" {
		t.Fatalf("string form: %+v", cfg.Tasks["test"])
	}
	e2e := cfg.Tasks["e2e"]
	if e2e.Cmd != "npm run e2e" || e2e.Description != "the browser suite" || e2e.Timeout != "10m" {
		t.Fatalf("table form: %+v", e2e)
	}
	if len(e2e.JUnit) != 1 || e2e.JUnit[0] != "reports/junit.xml" {
		t.Fatalf("junit: %v", e2e.JUnit)
	}
}

// A type that decodes its own subtree is invisible to Undecoded(), so the strictness promise has
// to be kept by hand here or it quietly stops at the [tasks] boundary.
func TestTaskTableRejectsUnknownKeyAndBadTimeout(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "boxer.toml")
	if err := os.WriteFile(p, []byte("[tasks.test]\ncmd = \"x\"\ndescriptoin = \"typo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("a misspelled task key must fail: %v", err)
	}
	if err := os.WriteFile(p, []byte("[tasks.test]\ncmd = \"x\"\ntimeout = \"soon\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFiles(p); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("an unparseable timeout must fail: %v", err)
	}
}
