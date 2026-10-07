package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
	// forwardPorts wants the port reachable, not a particular host port, so each worktree gets its own.
	if len(cfg.Network.Ports) != 2 || cfg.Network.Ports[0] != "auto:8000" || cfg.Network.Ports[1] != "auto:5432" {
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

// A devcontainer that builds its image becomes boxer's `build`, with its paths rewritten from the
// devcontainer file's directory to the worktree's.
func TestDevcontainerBuildBecomesBuild(t *testing.T) {
	for _, tc := range []struct{ name, body, build, context string }{
		{"build", `{"build":{"dockerfile":"Dockerfile","context":".."}}`, ".devcontainer/Dockerfile", "."},
		{"dockerFile", `{"dockerFile":"Dockerfile"}`, ".devcontainer/Dockerfile", ".devcontainer"},
		{"dockerFile and context", `{"dockerFile":"Dockerfile","context":".."}`, ".devcontainer/Dockerfile", "."},
		{"build, no context", `{"build":{"dockerfile":"docker/Dockerfile"}}`, ".devcontainer/docker/Dockerfile", ".devcontainer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDC(t, dir, tc.body)
			cfg, err := Load(dir, dir)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Build != tc.build || cfg.BuildContext != tc.context {
				t.Fatalf("build = %q, build_context = %q; want %q, %q", cfg.Build, cfg.BuildContext, tc.build, tc.context)
			}
			if cfg.Sources["build"] == "" {
				t.Fatal("doctor must be able to say where build came from")
			}
			if len(cfg.Warnings) != 0 {
				t.Fatalf("a build boxer does is not refused: %v", cfg.Warnings)
			}
		})
	}
	dir := t.TempDir()
	writeDC(t, dir, `{"build":{"dockerfile":"Dockerfile","args":{"V":"1"}}}`)
	cfg, _ := Load(dir, dir)
	if !strings.Contains(strings.Join(cfg.Warnings, "\n"), "build.args") {
		t.Fatalf("build args are not passed, and that must be said: %v", cfg.Warnings)
	}
}

// Silence would be worse than refusal: a repository whose tools come from `features` would get a
// sandbox that looks configured and is missing half of them.
func TestDevcontainerRefusesWhatNeedsABuild(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"features", `{"image":"x","features":{"ghcr.io/devcontainers/features/node:1":{}}}`, "features"},
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

// initializeCommand is the specification's only host-side hook, and it is what boxer's [prep] is.
// Reading it is the difference between honouring a devcontainer and honouring the half of it that
// happens to run in a container. updateContentCommand runs before postCreateCommand and both
// prepare the workspace, so both land in `setup`, in that order.
func TestTheHostSideAndContentHooksAreRead(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, `{
	  "image": "node:24-alpine",
	  "initializeCommand": "npm ci $BOXER_TARGET_FLAGS",
	  "onCreateCommand": "apk add git",
	  "updateContentCommand": "npm run codegen",
	  "postCreateCommand": ["npm", "run", "build"]
	}`)
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Prep.Commands) != 1 || cfg.Prep.Commands[0] != "npm ci $BOXER_TARGET_FLAGS" {
		t.Errorf("initializeCommand did not become prep: %v", cfg.Prep.Commands)
	}
	if len(cfg.ImageSetup) != 1 || cfg.ImageSetup[0] != "apk add git" {
		t.Errorf("onCreateCommand did not become image_setup: %v", cfg.ImageSetup)
	}
	// Specification order: updateContentCommand runs before postCreateCommand.
	if len(cfg.Setup) != 2 || cfg.Setup[0] != "npm run codegen" {
		t.Errorf("setup is not updateContent then postCreate: %v", cfg.Setup)
	}
}

// boxer.toml still wins: the devcontainer is the lowest layer, which is the only way to override
// a value you disagree with.
func TestBoxerTomlOverridesTheHostSideHook(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, `{"image":"node:24-alpine","initializeCommand":"npm ci"}`)
	write(t, dir, "boxer.toml", "[prep]\ncommands = [\"bun install\"]\n")
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Prep.Commands) != 1 || cfg.Prep.Commands[0] != "bun install" {
		t.Errorf("boxer.toml did not win: %v", cfg.Prep.Commands)
	}
}

// The specification's variables were passed through literally, which made a bind mount of
// "${localWorkspaceFolder}/data" an empty directory named after the variable and a token of
// "${localEnv:TOKEN}" the literal text. Both looked configured.
func TestDevcontainerVariablesAreResolved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOXER_DC_TOKEN", "s3cret")
	writeDC(t, dir, `{
  "workspaceFolder": "/workspaces/${localWorkspaceFolderBasename}",
  "mounts": [
    "source=${localWorkspaceFolder}/data,target=${containerWorkspaceFolder}/data,type=bind",
    "source=${devcontainerId}-cache,target=/cache,type=bind"
  ],
  "containerEnv": { "HOME_DIR": "/home/dev", "ID": "${devcontainerId}" },
  "remoteEnv": {
    "TOKEN": "${localEnv:BOXER_DC_TOKEN}",
    "FALLBACK": "${localEnv:BOXER_DC_UNSET:dflt}",
    "FROM_CONTAINER": "${containerEnv:HOME_DIR}/bin",
    "UNKNOWN": "${containerEnv:PATH}:/x"
  },
  "postCreateCommand": "echo ${containerWorkspaceFolder}"
}`)
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(dir)
	if cfg.MountAt != "/workspaces/"+base {
		t.Errorf("workspaceFolder: %q", cfg.MountAt)
	}
	if len(cfg.Mounts) != 1 || cfg.Mounts[0] != dir+"/data:/workspaces/"+base+"/data" {
		t.Errorf("mounts must resolve, and one that cannot must be dropped rather than mounted wrong: %v", cfg.Mounts)
	}
	want := map[string]string{"HOME_DIR": "/home/dev", "TOKEN": "s3cret", "FALLBACK": "dflt", "FROM_CONTAINER": "/home/dev/bin"}
	for k, v := range want {
		if cfg.Env[k] != v {
			t.Errorf("env %s = %q, want %q", k, cfg.Env[k], v)
		}
	}
	for _, k := range []string{"UNKNOWN", "ID"} {
		if _, ok := cfg.Env[k]; ok {
			t.Errorf("env %s depends on a variable boxer cannot resolve and must be dropped, not passed literally: %q", k, cfg.Env[k])
		}
	}
	if len(cfg.Setup) != 1 || cfg.Setup[0] != "echo /workspaces/"+base {
		t.Errorf("setup: %v", cfg.Setup)
	}
	joined := strings.Join(cfg.Warnings, "\n")
	for _, name := range []string{"${containerEnv:PATH}", "${devcontainerId}"} {
		if !strings.Contains(joined, name) {
			t.Errorf("an unresolvable variable must be named: %s missing from %v", name, cfg.Warnings)
		}
	}
}

// The rest of the cheap half of the specification: who commands run as, what the host must
// provide, what a port is called, and which ports must keep their number.
func TestDevcontainerUserResourcesAndPortAttributes(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, `{
  "remoteUser": "node", "containerUser": "root",
  "hostRequirements": { "cpus": 8, "memory": "16gb" },
  "forwardPorts": [3000, 5432],
  "appPort": [9000],
  "portsAttributes": { "3000": { "label": "web" }, "5432": { "requireLocalPort": true } }
}`)
	cfg, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.User != "node" {
		t.Errorf("remoteUser wins over containerUser: %q", cfg.User)
	}
	if cfg.CPUs != 8 || cfg.Memory != "16384M" {
		t.Errorf("hostRequirements: cpus %d memory %q", cfg.CPUs, cfg.Memory)
	}
	if got := strings.Join(cfg.Network.Ports, " "); got != "auto:3000 5432:5432 9000:9000" {
		t.Errorf("ports: %s", got)
	}
	if cfg.URLs.Names["3000"] != "web" {
		t.Errorf("a port's label names its URL: %v", cfg.URLs.Names)
	}
	// Minimums never lower boxer's defaults.
	dir2 := t.TempDir()
	writeDC(t, dir2, `{"hostRequirements": { "cpus": 1, "memory": "512mb" }}`)
	cfg2, err := Load(dir2, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.CPUs != Defaults().CPUs || cfg2.Memory != Defaults().Memory {
		t.Errorf("a minimum below the default must not lower it: %d %q", cfg2.CPUs, cfg2.Memory)
	}
}

// Settings that would widen the sandbox, or that boxer cannot honour, are named — never silently
// dropped and never silently obeyed.
func TestDevcontainerRefusesBoundaryAndAttachSettings(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"runArgs", `{"runArgs":["--network=host"]}`, "runArgs"},
		{"privileged", `{"privileged":true}`, "privileged"},
		{"capAdd", `{"capAdd":["SYS_PTRACE"]}`, "capAdd"},
		{"securityOpt", `{"securityOpt":["seccomp=unconfined"]}`, "securityOpt"},
		{"init", `{"init":true}`, "init"},
		{"workspaceMount", `{"workspaceMount":"source=/x,target=/y,type=bind"}`, "workspaceMount"},
		{"overrideCommand", `{"overrideCommand":false}`, "overrideCommand"},
		{"postAttachCommand", `{"postAttachCommand":"echo hi"}`, "postAttachCommand"},
		{"gpu", `{"hostRequirements":{"gpu":true}}`, "gpu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDC(t, dir, tc.body)
			cfg, err := Load(dir, dir)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(cfg.Warnings, "\n"), tc.want) {
				t.Errorf("%s must be named: %v", tc.name, cfg.Warnings)
			}
		})
	}
	dir := t.TempDir()
	writeDC(t, dir, `{"hostRequirements":{"gpu":"optional"},"overrideCommand":true}`)
	cfg, _ := Load(dir, dir)
	if len(cfg.Warnings) != 0 {
		t.Errorf("an optional GPU and overrideCommand: true ask for nothing boxer lacks: %v", cfg.Warnings)
	}
}

// What the 1.4 review found in the devcontainer mapping.
func TestDevcontainerReviewFindings(t *testing.T) {
	// A boxer.toml image wins over a devcontainer build.
	dir := t.TempDir()
	writeDC(t, dir, `{"build":{"dockerfile":"docker/Dockerfile"}}`)
	os.WriteFile(filepath.Join(dir, "boxer.toml"), []byte("image = \"python:3.12\"\n"), 0o644)
	c, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Image != "python:3.12" || c.Build != "" {
		t.Fatalf("boxer.toml's image must win: image %q build %q", c.Image, c.Build)
	}
	// An argv array runs as the words it names.
	dir = t.TempDir()
	writeDC(t, dir, `{"image":"x","postCreateCommand":["bash","-c","npm ci && npm test"]}`)
	c, _ = Load(dir, dir)
	if len(c.Setup) != 1 || c.Setup[0] != `bash -c 'npm ci && npm test'` {
		t.Fatalf("setup: %q", c.Setup)
	}
	// Comments are stripped, strings are not touched.
	dir = t.TempDir()
	writeDC(t, dir, "{\n  // a comment\n  \"image\": \"x\", /* another */\n  \"postCreateCommand\": \"ls src/**/*.go\",\n  \"containerEnv\": {\"A\": \"a,]\", \"B\": \"http://x\"},\n}")
	c, err = Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Setup[0] != "ls src/**/*.go" || c.Env["A"] != "a,]" || c.Env["B"] != "http://x" {
		t.Fatalf("strings changed: %q %q", c.Setup, c.Env)
	}
	// A worktree's own devcontainer wins over the repository's.
	repo, wt := t.TempDir(), t.TempDir()
	writeDC(t, repo, `{"image":"repo-image"}`)
	writeDC(t, wt, `{"image":"worktree-image"}`)
	if c, _ := Load(wt, repo); c.Image != "worktree-image" {
		t.Fatalf("the worktree's devcontainer must win: %q", c.Image)
	}
}

// A byte-order mark, which Windows editors write and VS Code accepts, is not a broken file.
func TestDevcontainerWithAByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	writeDC(t, dir, "\xef\xbb\xbf{\"image\": \"python:3.12\"}")
	c, err := Load(dir, dir)
	if err != nil || c.Image != "python:3.12" {
		t.Fatalf("%v %q", err, c.Image)
	}
}

// A devcontainer passes a host secret as "NAME": "${localEnv:NAME}". As env it was baked into the
// environment pack and passed on the command line; it becomes a secret, read by name at run time.
func TestDevcontainerHostSecretsBecomeSecrets(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_secret")
	dir := t.TempDir()
	writeDC(t, dir, `{"image": "alpine", "containerEnv": {"GITHUB_TOKEN": "${localEnv:GITHUB_TOKEN}", "MODE": "dev"},
		"remoteEnv": {"AUTH": "Bearer ${localEnv:GITHUB_TOKEN}"}}`)
	c, err := Load(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Env["GITHUB_TOKEN"]; ok || !slices.Contains(c.Secrets, "GITHUB_TOKEN") || c.Env["MODE"] != "dev" {
		t.Fatalf("env %v secrets %v", c.Env, c.Secrets)
	}
	if !strings.Contains(strings.Join(c.Warnings, "\n"), "remoteEnv.AUTH") {
		t.Fatalf("a host value it cannot move is named: %v", c.Warnings)
	}
}
