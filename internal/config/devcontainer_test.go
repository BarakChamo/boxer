package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jsonUnmarshalForTest keeps the comment stripper's test honest about what a decoder accepts.
func jsonUnmarshalForTest(b []byte, v any) error { return json.Unmarshal(b, v) }

func writeDC(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, ".devcontainer", "devcontainer.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A repository that already describes its environment should not have to describe it twice. This
// is the realistic file: comments, a trailing comma, every runtime property, and one that needs a
// build.
func TestDevcontainerRuntimePropertiesAreRead(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, `{
  // a real file has comments
  "name": "example",
  "image": "python:3.12",
  "onCreateCommand": "apt-get install -y libpq-dev",
  "postCreateCommand": "pip install -r requirements.txt",
  "postStartCommand": ["python", "-m", "http.server", "8000"],
  "forwardPorts": [8000, "127.0.0.1:5432"],
  "containerEnv": { "PYTHONUNBUFFERED": "1", "EDITOR": "nano" },
  "remoteEnv": { "EDITOR": "vi" },
  "workspaceFolder": "/workspaces/example",
  "mounts": [
    "source=/host/cache,target=/root/.cache,type=bind",
    { "type": "volume", "source": "vol", "target": "/data" },
  ],
}`)
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Image != "python:3.12" {
		t.Errorf("image: %q", cfg.Image)
	}
	if len(cfg.Setup) != 1 || cfg.Setup[0] != "pip install -r requirements.txt" {
		t.Errorf("setup: %v", cfg.Setup)
	}
	// onCreateCommand is the cacheable half, and boxer keeps it that way.
	if len(cfg.ImageSetup) != 1 || cfg.ImageSetup[0] != "apt-get install -y libpq-dev" {
		t.Errorf("image_setup: %v", cfg.ImageSetup)
	}
	if len(cfg.Start) != 1 || cfg.Start[0] != "python -m http.server 8000" {
		t.Errorf("start: %v", cfg.Start)
	}
	if cfg.MountAt != "/workspaces/example" {
		t.Errorf("mount_at: %q", cfg.MountAt)
	}
	// remoteEnv is the tool's own environment and wins where both name a variable.
	if cfg.Env["EDITOR"] != "vi" || cfg.Env["PYTHONUNBUFFERED"] != "1" {
		t.Errorf("env: %v", cfg.Env)
	}
	if len(cfg.Network.Ports) != 2 || cfg.Network.Ports[0] != "8000:8000" || cfg.Network.Ports[1] != "5432:5432" {
		t.Errorf("ports: %v", cfg.Network.Ports)
	}
	// A bind mount is a host directory; a volume is Docker's own storage and has no meaning here.
	if len(cfg.Mounts) != 1 || cfg.Mounts[0] != "/host/cache:/root/.cache" {
		t.Errorf("mounts: %v", cfg.Mounts)
	}
	if cfg.Sources["image"] == "" || !strings.HasSuffix(cfg.Sources["image"], "devcontainer.json") {
		t.Errorf("doctor must be able to say where each value came from: %v", cfg.Sources)
	}
}

// boxer.toml is how you disagree with a devcontainer, so it wins every time.
func TestBoxerTomlOverridesTheDevcontainer(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, `{"image":"python:3.12","postCreateCommand":"pip install -r requirements.txt"}`)
	if err := os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("image = \"alpine\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Image != "alpine" {
		t.Errorf("boxer.toml must win: %q", cfg.Image)
	}
	if len(cfg.Setup) != 1 {
		t.Errorf("what it does not override still applies: %v", cfg.Setup)
	}
}

// Silence would be worse than refusal: a repository whose tools come from `features` would get a
// sandbox that looks configured and is missing half of them.
func TestDevcontainerRefusesWhatNeedsABuild(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"features", `{"image":"x","features":{"ghcr.io/devcontainers/features/node:1":{}}}`, "features"},
		{"build", `{"build":{"dockerfile":"Dockerfile"}}`, "build"},
		{"dockerFile", `{"dockerFile":"Dockerfile"}`, "build"},
		{"compose", `{"dockerComposeFile":"docker-compose.yml","service":"app"}`, "dockerComposeFile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDC(t, dir, tc.body)
			cfg, err := Load(dir, dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Warnings) == 0 {
				t.Fatalf("%s must be refused by name", tc.name)
			}
			joined := strings.Join(cfg.Warnings, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("the refusal must name it: %s", joined)
			}
			if !strings.Contains(joined, "setup") && !strings.Contains(joined, "image") && !strings.Contains(joined, "orchestrator") {
				t.Fatalf("a refusal must say what to do instead: %s", joined)
			}
		})
	}
}

// The lifecycle commands take three shapes in the specification and all three appear in the wild.
func TestCommandListShapes(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want []string
	}{
		{"npm ci", []string{"npm ci"}},
		{[]any{"npm", "ci"}, []string{"npm ci"}},
		{map[string]any{"b": "second", "a": "first"}, []string{"first", "second"}},
		{"", nil},
		{nil, nil},
	} {
		got := commandList(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%v -> %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestStripJSONComments(t *testing.T) {
	in := []byte("{\n  // line\n  \"a\": \"http://x\", /* block */\n  \"b\": [1,2,],\n}")
	var out map[string]any
	if err := jsonUnmarshalForTest(stripJSONComments(in), &out); err != nil {
		t.Fatalf("%v\n%s", err, stripJSONComments(in))
	}
	if out["a"] != "http://x" {
		t.Errorf("a URL is not a comment: %v", out["a"])
	}
	if len(out["b"].([]any)) != 2 {
		t.Errorf("b: %v", out["b"])
	}
}
