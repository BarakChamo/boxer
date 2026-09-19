package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A devcontainer.json is where a repository that cares about a reproducible environment already
// writes it down. boxer reads the part of it that needs no image build, so such a repository does
// not have to maintain the same facts twice.
//
// The split is not arbitrary. `image`, `postCreateCommand`, `postStartCommand`, `forwardPorts`,
// `containerEnv`, `remoteEnv`, bind `mounts` and `workspaceFolder` are runtime properties: they
// describe how to run a container that already exists. `features`, `build`/`dockerFile` and
// `dockerComposeFile` describe how to *build* one, or how to orchestrate several, which smolvm
// cannot do — so boxer refuses them by name rather than ignoring them silently.
type devcontainer struct {
	Image             string            `json:"image"`
	OnCreateCommand   any               `json:"onCreateCommand"`
	PostCreateCommand any               `json:"postCreateCommand"`
	PostStartCommand  any               `json:"postStartCommand"`
	ForwardPorts      []any             `json:"forwardPorts"`
	ContainerEnv      map[string]string `json:"containerEnv"`
	RemoteEnv         map[string]string `json:"remoteEnv"`
	Mounts            []any             `json:"mounts"`
	WorkspaceFolder   string            `json:"workspaceFolder"`

	// Refused, and named in the refusal.
	Features         map[string]any `json:"features"`
	Build            any            `json:"build"`
	DockerFile       string         `json:"dockerFile"`
	DockerComposeFil any            `json:"dockerComposeFile"`
}

// DevcontainerFiles are the two standard locations, in the order the specification gives them.
var DevcontainerFiles = []string{".devcontainer/devcontainer.json", ".devcontainer.json"}

// findDevcontainer returns the first devcontainer file under root, or "".
func findDevcontainer(root string) string {
	for _, rel := range DevcontainerFiles {
		p := filepath.Join(root, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// mergeDevcontainer reads path and fills in anything boxer.toml has not already set. It is the
// lowest layer on purpose: a repository that says both means the boxer file, which is the only
// way to override a devcontainer value you disagree with.
func (c *Config) mergeDevcontainer(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var d devcontainer
	if err := json.Unmarshal(stripJSONComments(raw), &d); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	set := func(key string, already bool, apply func()) {
		if already {
			return
		}
		apply()
		c.Sources[key] = path
	}
	set("image", c.Image != "", func() { c.Image = d.Image })
	if d.Image == "" {
		delete(c.Sources, "image")
	}
	// onCreateCommand runs once when the container is built and is cacheable, which is exactly
	// boxer's image_setup; postCreateCommand runs for the workspace, which is boxer's setup. The
	// specification draws the line for the same reason boxer does.
	set("image_setup", len(c.ImageSetup) > 0, func() { c.ImageSetup = commandList(d.OnCreateCommand) })
	set("setup", len(c.Setup) > 0, func() { c.Setup = commandList(d.PostCreateCommand) })
	set("start", len(c.Start) > 0, func() { c.Start = commandList(d.PostStartCommand) })
	// A value still at its default was never set by anyone, so the devcontainer's answer is the
	// only one there is. Comparing against Defaults() is the honest test for that; an empty string
	// is not, because these keys have non-empty defaults.
	set("mount_at", c.MountAt != Defaults().MountAt, func() {
		if d.WorkspaceFolder != "" {
			c.MountAt = d.WorkspaceFolder
		}
	})
	set("mounts", len(c.Mounts) > 0, func() { c.Mounts = bindMounts(d.Mounts) })
	set("env", len(c.Env) > 0, func() {
		env := map[string]string{}
		for k, v := range d.ContainerEnv {
			env[k] = v
		}
		// remoteEnv is the tool's own environment and wins where both say a name.
		for k, v := range d.RemoteEnv {
			env[k] = v
		}
		if len(env) > 0 {
			c.Env = env
		}
	})
	set("network", len(c.Network.Ports) > 0, func() { c.Network.Ports = forwardPorts(d.ForwardPorts) })

	for _, w := range d.unsupported() {
		c.Warnings = append(c.Warnings, path+": "+w)
	}
	c.Files = append(c.Files, path)
	return nil
}

// unsupported names what boxer will not do and what to do instead. Silence here would be worse
// than a refusal: a repository whose environment is built by `features` would get a sandbox that
// looks configured and is missing half its tools.
func (d devcontainer) unsupported() []string {
	var out []string
	if len(d.Features) > 0 {
		names := make([]string, 0, len(d.Features))
		for f := range d.Features {
			names = append(names, f)
		}
		sort.Strings(names)
		out = append(out, "`features` needs an image build, which smolvm cannot do ("+strings.Join(names, ", ")+
			"). Install them in `setup`, or build an image yourself and set `image` to the archive.")
	}
	if d.Build != nil || d.DockerFile != "" {
		out = append(out, "`build`/`dockerFile` needs an image build, which smolvm cannot do. "+
			"Build it with your own tooling, `docker save` it, and set `image` to the archive.")
	}
	if d.DockerComposeFil != nil {
		out = append(out, "`dockerComposeFile` orchestrates several containers, which boxer does not. "+
			"Run the services in one guest with `setup` and `start`, or use an orchestrator.")
	}
	return out
}

// commandList flattens the three shapes a devcontainer lifecycle command takes: a string, an argv
// array, or an object of named commands the specification runs in parallel.
func commandList(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		parts := make([]string, 0, len(t))
		for _, p := range t {
			parts = append(parts, fmt.Sprint(p))
		}
		if len(parts) == 0 {
			return nil
		}
		return []string{strings.Join(parts, " ")}
	case map[string]any:
		names := make([]string, 0, len(t))
		for k := range t {
			names = append(names, k)
		}
		sort.Strings(names) // parallel in the spec; ordered here, because a VM is one machine
		out := make([]string, 0, len(names))
		for _, k := range names {
			out = append(out, commandList(t[k])...)
		}
		return out
	}
	return nil
}

// bindMounts keeps the bind mounts and drops the rest: a volume is Docker's own storage, which
// has no meaning here.
func bindMounts(in []any) []string {
	var out []string
	for _, m := range in {
		switch t := m.(type) {
		case string:
			if src, dst, ok := parseMountString(t); ok {
				out = append(out, src+":"+dst)
			}
		case map[string]any:
			if fmt.Sprint(t["type"]) != "bind" {
				continue
			}
			src, dst := fmt.Sprint(t["source"]), fmt.Sprint(t["target"])
			if src != "" && dst != "" {
				out = append(out, src+":"+dst)
			}
		}
	}
	return out
}

// parseMountString reads the "source=…,target=…,type=bind" form.
func parseMountString(s string) (src, dst string, ok bool) {
	kind := "bind"
	for _, part := range strings.Split(s, ",") {
		k, v, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		switch k {
		case "source", "src":
			src = v
		case "target", "destination", "dst":
			dst = v
		case "type":
			kind = v
		}
	}
	return src, dst, src != "" && dst != "" && kind == "bind"
}

// forwardPorts maps the devcontainer form (a number, or "host:port") onto smolvm's "host:guest".
func forwardPorts(in []any) []string {
	var out []string
	for _, p := range in {
		switch t := p.(type) {
		case float64:
			n := int(t)
			out = append(out, fmt.Sprintf("%d:%d", n, n))
		case string:
			if _, port, found := strings.Cut(t, ":"); found {
				out = append(out, port+":"+port)
			} else if t != "" {
				out = append(out, t+":"+t)
			}
		}
	}
	return out
}

// stripJSONComments removes // and /* */ comments and trailing commas. devcontainer.json is JSONC
// in practice — the specification says so and every real file uses it — and Go's decoder is not.
var (
	lineComment  = regexp.MustCompile(`(?m)(^|[^:"])//.*$`)
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	trailingComa = regexp.MustCompile(`,(\s*[}\]])`)
)

func stripJSONComments(b []byte) []byte {
	b = blockComment.ReplaceAll(b, nil)
	b = lineComment.ReplaceAll(b, []byte("$1"))
	return trailingComa.ReplaceAll(b, []byte("$1"))
}
