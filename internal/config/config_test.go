package config

import (
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
	if cfg.Mode != "off" || cfg.Sources["mode"] != "env" || cfg.CPUs != 1 {
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
