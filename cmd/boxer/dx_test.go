package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/internal/vmtest"
)

// ls says what a person needs in order to act: the backend, the branch, whether the worktree is
// dirty or gone, and where it serves — and says it as JSON for a program.
func TestLsReportsWorktreeState(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"network = { ports = [\"auto:3000\"] }\n")
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if code, out := call(t, &rows, "ls", "--json"); code != 0 || len(rows) != 1 {
		t.Fatalf("%d %s", code, out)
	}
	r := rows[0]
	g, _ := r["git"].(map[string]any)
	if r["backend"] != "smolvm" || g == nil || g["exists"] != true || g["dirty"] != true || g["branch"] == "" {
		t.Fatalf("ls row: %v", r)
	}
	if p, _ := r["ports"].(map[string]any); p["3000"] == nil {
		t.Fatalf("forwarded ports: %v", r)
	}
	// The text form is a table an agent can read, with no colour codes in it.
	_, out := call(t, nil, "ls")
	if !strings.Contains(out, "dirty(") || !strings.Contains(out, "smolvm") || strings.Contains(out, "\x1b[") {
		t.Fatalf("text ls:\n%s", out)
	}
	// BOXER_OUTPUT=json is --json for a caller that cannot add a flag.
	t.Setenv("BOXER_OUTPUT", "json")
	if code, _ := call(t, &rows, "ls"); code != 0 || len(rows) != 1 {
		t.Fatalf("BOXER_OUTPUT=json: %d", code)
	}
}

// stop and rm act on what they are told, by name, by filter, or on everything; an agent that
// names nothing is told how to rather than asked.
func TestStopAndRm(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var st map[string]any
	call(t, &st, "status", "--json")
	key, slug := st["scope"].(string), st["name"].(string)

	if code, out := call(t, nil, "rm"); code != 2 || !strings.Contains(out, "fix: boxer ls --json") {
		t.Fatalf("rm with nothing named, not a person: %d %s", code, out)
	}
	if code, out := call(t, nil, "rm", "no-such-sandbox"); code != 1 || !strings.Contains(out, "no boxer sandbox named") {
		t.Fatalf("unknown name: %d %s", code, out)
	}
	if code, out := call(t, nil, "stop", slug); code != 0 || !strings.Contains(out, "stopped") {
		t.Fatalf("stop by slug: %d %s", code, out)
	}
	if code, _ := call(t, &st, "status", "--json"); code != exitStopped {
		t.Fatalf("after stop: %d %v", code, st)
	}
	var rows []map[string]any
	if code, _ := call(t, &rows, "rm", "--stopped", "--json"); code != 0 || len(rows) != 1 || rows[0]["scope"] != key {
		t.Fatalf("rm --stopped: %d %v", code, rows)
	}
	if code, _ := call(t, &st, "status", "--json"); code != exitAbsent {
		t.Fatalf("after rm: %d", code)
	}
}

// --gone removes sandboxes whose worktree has been deleted, and nothing else.
func TestRmGone(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	var rows []map[string]any
	if code, _ := call(t, &rows, "rm", "--gone", "--json"); code != 0 || len(rows) != 0 {
		t.Fatalf("a live worktree is not gone: %v", rows)
	}
	t.Chdir(t.TempDir())
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, &rows, "rm", "--gone", "--json"); code != 0 || len(rows) != 1 {
		t.Fatalf("rm --gone: %d %v", code, rows)
	}
}

// The interactive picker: a person chooses by number, and choosing is the confirmation.
func TestRmInteractivePicker(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	if code, out := call(t, nil, "up"); code != 0 {
		t.Fatal(out)
	}
	t.Setenv("BOXER_OUTPUT", "human")
	var out, errb bytes.Buffer
	if code := run([]string{"rm", "-i"}, strings.NewReader("1\n"), &out, &errb); code != 0 {
		t.Fatalf("%d %s %s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "rm which?") || !strings.Contains(out.String(), "removed") {
		t.Fatalf("picker:\n%s", out.String())
	}
	t.Setenv("BOXER_OUTPUT", "json")
	var st map[string]any
	if code, _ := call(t, &st, "status"); code != exitAbsent {
		t.Fatalf("the chosen sandbox must be gone: %d", code)
	}
}

// url prints where the server is; with no sandbox or no forward, it says what to do.
func TestURL(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck+"network = { ports = [\"auto:3000\"] }\n")
	if code, out := call(t, nil, "url"); code != 4 || !strings.Contains(out, "fix: boxer up") {
		t.Fatalf("no sandbox: %d %s", code, out)
	}
	call(t, nil, "up")
	if code, out := call(t, nil, "url"); code != 0 || !strings.HasPrefix(out, "http://127.0.0.1:") {
		t.Fatalf("url: %d %s", code, out)
	}
	if code, out := call(t, nil, "url", "9999"); code != 1 || !strings.Contains(out, "forwarded: 3000") {
		t.Fatalf("an unforwarded port: %d %s", code, out)
	}
}

func TestCompletion(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		if code, out := call(t, nil, "completion", sh); code != 0 || !strings.Contains(out, "backends") || !strings.Contains(out, "integrations") {
			t.Fatalf("%s: %d %s", sh, code, out)
		}
	}
	if code, _ := call(t, nil, "completion", "tcsh"); code != 2 {
		t.Fatal("an unknown shell is a usage error")
	}
}

// backends reports the configured one as installed and answering; a probe of the fake cannot run
// a real guest, so it is exercised by the smoke suite instead.
func TestBackends(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	var rows []map[string]any
	if code, out := call(t, &rows, "backends", "smolvm", "--json"); code != 0 || len(rows) != 1 {
		t.Fatalf("%d %s", code, out)
	}
	if rows[0]["installed"] != true || rows[0]["reachable"] != true || rows[0]["configured"] != true {
		t.Fatalf("%v", rows[0])
	}
	if code, _ := call(t, nil, "backends", "nosuch"); code != 2 {
		t.Fatal("an unknown backend is a usage error")
	}
}

// integrations asks the installer which files it writes and checks them, so an install makes the
// row read installed and nothing else has to know the file list.
func TestIntegrationsReflectsInstall(t *testing.T) {
	vmtest.Install(t)
	dir := vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	row := func() map[string]any {
		var rows []map[string]any
		if code, out := call(t, &rows, "integrations", "--json"); code != 0 {
			t.Fatalf("%d %s", code, out)
		}
		for _, r := range rows {
			if r["name"] == "claude-code" {
				return r
			}
		}
		t.Fatal("no claude-code row")
		return nil
	}
	if r := row(); r["project"] != "no" {
		t.Fatalf("before install: %v", r)
	}
	if code, out := call(t, nil, "install", "claude-code"); code != 0 {
		t.Fatal(out)
	}
	if r := row(); r["project"] != "installed" || r["skill"] != "yes" {
		t.Fatalf("after install: %v", r)
	}
	_ = exec.Command("git", "-C", dir, "status").Run()
	if code, _ := call(t, nil, "integrations", "add", "nosuch"); code != 2 {
		t.Fatal("an unknown integration is a usage error")
	}
}

// The standard library stops at the first positional, which made `boxer backends docker --probe`
// ignore --probe. Flags must be honoured wherever they are.
func TestParseAnywhere(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	probe := fs.Bool("probe", false, "")
	yes := fs.Bool("y", false, "")
	pos, err := parseAnywhere(fs, []string{"docker", "--probe", "extra", "-y"})
	if err != nil || !*probe || !*yes || strings.Join(pos, ",") != "docker,extra" {
		t.Fatalf("pos %v probe %v yes %v err %v", pos, *probe, *yes, err)
	}
}

// fakeBin puts a script named name on PATH that records its arguments and prints out.
func fakeBin(t *testing.T, name, out string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, name+".args")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\nprintf '%s' '" + out + "'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// The skill and the plugin go through the ecosystem's own installers, with boxer's source.
func TestIntegrationsAddUsesNpx(t *testing.T) {
	log := fakeBin(t, "npx", "")
	if code, out := call(t, nil, "integrations", "add", "skill", "--agent", "claude-code", "--user"); code != 0 {
		t.Fatalf("%d %s", code, out)
	}
	if code, out := call(t, nil, "integrations", "add", "plugin", "--agent", "cursor"); code != 0 {
		t.Fatalf("%d %s", code, out)
	}
	b, _ := os.ReadFile(log)
	got := string(b)
	for _, want := range []string{"skills add BarakChamo/boxer --skill boxer -y -a claude-code -g", "plugins add BarakChamo/boxer -y -t cursor -s project"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// Branch, distance from upstream, detached HEAD and the pull request, read the way ls reads them.
func TestWorktreeGit(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if b, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
	}
	git("init", "-q", "-b", "main")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "a")
	g := worktreeGit(dir, false)
	if !g.Exists || g.Branch != "main" || g.Dirty || g.summary() != "clean" || g.branchLabel() != "main" {
		t.Fatalf("%+v", g)
	}
	fakeBin(t, "gh", `{"number":42,"state":"OPEN","url":"https://example/pull/42"}`)
	if g := worktreeGit(dir, true); g.PR == nil || g.PR.Number != 42 || g.branchLabel() != "main #42" {
		t.Fatalf("pr: %+v", g)
	}
	git("checkout", "-q", "--detach")
	if g := worktreeGit(dir, true); !g.Detached || g.PR != nil || g.branchLabel() != "(detached)" {
		t.Fatalf("detached: %+v", g)
	}
	if g := worktreeGit(filepath.Join(dir, "gone"), false); g.Exists || g.summary() != "gone" {
		t.Fatalf("gone: %+v", g)
	}
}

// The misconfiguration that cost the most time here: a podman machine on libkrun cannot mount.
func TestKnownProblemsNamesLibkrun(t *testing.T) {
	fakeBin(t, "podman", "podman-machine-default true")
	p := knownProblems("podman")
	if len(p) != 1 || !strings.Contains(p[0], "libkrun") || !strings.Contains(p[0], "CONTAINERS_MACHINE_PROVIDER=applehv") {
		t.Fatalf("%v", p)
	}
}

// A person removing several sandboxes is asked once; no is no.
func TestRmConfirmsForAPerson(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	call(t, nil, "up")
	t.Setenv("BOXER_OUTPUT", "human")
	var out, errb bytes.Buffer
	if code := run([]string{"rm", "--all"}, strings.NewReader("n\n"), &out, &errb); code != 1 || !strings.Contains(out.String(), "[y/N]") {
		t.Fatalf("declined: %d %s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"rm", "--all", "-y"}, strings.NewReader(""), &out, &errb); code != 0 || !strings.Contains(out.String(), "removed") {
		t.Fatalf("-y: %d %s %s", code, out.String(), errb.String())
	}
}

// The text forms, for a person and for an agent.
func TestBackendsAndIntegrationsText(t *testing.T) {
	vmtest.Install(t)
	vmtest.RepoIn(t, vmtest.NoWorktreeCheck)
	t.Setenv("BOXER_OUTPUT", "human")
	t.Setenv("NO_COLOR", "1")
	if code, out := call(t, nil, "backends", "smolvm"); code != 0 || !strings.Contains(out, "smolvm *") || !strings.Contains(out, "boxer backends --probe") {
		t.Fatalf("backends: %d %s", code, out)
	}
	t.Setenv("BOXER_OUTPUT", "text")
	if code, out := call(t, nil, "integrations"); code != 0 || !strings.Contains(out, "INTEGRATION") || !strings.Contains(out, "portless") {
		t.Fatalf("integrations: %d %s", code, out)
	}
}

// A probe that cannot run says why, as a failed probe rather than a crash; the fake cannot run a
// guest, which makes it the failure case. The success case is the smoke suite's.
func TestProbeReportsFailure(t *testing.T) {
	vmtest.Install(t)
	// The fake runs guest commands in this process's directory, so the probe's write would land in
	// the package source tree — it did, once, as cmd/boxer/probe.txt.
	t.Chdir(t.TempDir())
	p := runProbe("smolvm")
	if p == nil || p.OK || p.Error == "" {
		t.Fatalf("%+v", p)
	}
}

func TestDXHelpers(t *testing.T) {
	if plural(1, "sandbox", "sandboxes") != "1 sandbox" || plural(3, "sandbox", "sandboxes") != "3 sandboxes" {
		t.Fatal("plural")
	}
	if v := versionNumber("container CLI version 1.4.1 (build: release, commit: 9a8917c)"); v != "1.4.1" {
		t.Fatalf("version %q", v)
	}
	if v := versionNumber("no digits here\nsecond"); v != "no digits here" {
		t.Fatalf("fallback %q", v)
	}
	home, _ := os.UserHomeDir()
	if got := shortPath(filepath.Join(home, "src", "x")); got != "~/src/x" {
		t.Fatalf("shortPath %q", got)
	}
	if shortPath("/elsewhere") != "/elsewhere" || firstNonBlank("", "b") != "b" || firstSorted(map[string]string{"b": "2", "a": "1"}) != "1" {
		t.Fatal("helpers")
	}
	if firstErr(nil, os.ErrNotExist) != os.ErrNotExist {
		t.Fatal("firstErr")
	}
	if got := lastLines("a\nb\nc\n", 2); got != "b / c" {
		t.Fatalf("lastLines %q", got)
	}
	if got := lastLines("only", 3); got != "only" {
		t.Fatalf("lastLines short %q", got)
	}
}
