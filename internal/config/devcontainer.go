package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	Image                string `json:"image"`
	InitializeCommand    any    `json:"initializeCommand"`
	OnCreateCommand      any    `json:"onCreateCommand"`
	UpdateContentCommand any    `json:"updateContentCommand"`
	PostCreateCommand    any    `json:"postCreateCommand"`
	PostStartCommand     any    `json:"postStartCommand"`
	ForwardPorts         []any  `json:"forwardPorts"`
	AppPort              any    `json:"appPort"`
	PortsAttributes      map[string]struct {
		Label            string `json:"label"`
		RequireLocalPort bool   `json:"requireLocalPort"`
	} `json:"portsAttributes"`
	ContainerEnv     map[string]string `json:"containerEnv"`
	RemoteEnv        map[string]string `json:"remoteEnv"`
	Mounts           []any             `json:"mounts"`
	WorkspaceFolder  string            `json:"workspaceFolder"`
	RemoteUser       string            `json:"remoteUser"`
	ContainerUser    string            `json:"containerUser"`
	HostRequirements struct {
		CPUs   int    `json:"cpus"`
		Memory string `json:"memory"`
		GPU    any    `json:"gpu"`
	} `json:"hostRequirements"`

	// Refused, and named in the refusal.
	Features         map[string]any `json:"features"`
	Build            any            `json:"build"`
	DockerFile       string         `json:"dockerFile"`
	DockerComposeFil any            `json:"dockerComposeFile"`
	// Read only to be refused: each one widens or reshapes the boundary boxer draws, and honouring
	// it quietly would hand a devcontainer a way around the sandbox.
	RunArgs           []string `json:"runArgs"`
	Privileged        bool     `json:"privileged"`
	CapAdd            []string `json:"capAdd"`
	SecurityOpt       []string `json:"securityOpt"`
	Init              bool     `json:"init"`
	WorkspaceMount    string   `json:"workspaceMount"`
	OverrideCommand   *bool    `json:"overrideCommand"`
	PostAttachCommand any      `json:"postAttachCommand"`
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
func (c *Config) mergeDevcontainer(path, workspace string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d, unresolved, err := decodeDevcontainer(stripJSONComments(raw), workspace, c.MountAt)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, u := range unresolved {
		c.Warnings = append(c.Warnings, path+": "+u)
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
	// initializeCommand is the one hook the specification puts on the *host*, which is exactly
	// what boxer's [prep] is. Reading it is the difference between honouring a devcontainer and
	// honouring the half of it that happens to run in a container.
	set("prep", len(c.Prep.Commands) > 0, func() { c.Prep.Commands = commandList(d.InitializeCommand) })
	// updateContentCommand runs in the guest whenever the source changes, which for boxer's
	// one-sandbox-per-worktree shape is the same moment as postCreateCommand: the worktree is
	// prepared once and the sandbox is discarded with it. Both therefore land in `setup`, in
	// specification order, rather than one of them being dropped.
	set("image_setup", len(c.ImageSetup) > 0, func() { c.ImageSetup = commandList(d.OnCreateCommand) })
	set("setup", len(c.Setup) > 0, func() {
		c.Setup = append(commandList(d.UpdateContentCommand), commandList(d.PostCreateCommand)...)
	})
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
	set("network", len(c.Network.Ports) > 0, func() { c.Network.Ports = d.ports() })
	// A label is the name a person gave the port, which is exactly what its URL should be called.
	set("urls", len(c.URLs.Names) > 0, func() {
		for port, a := range d.PortsAttributes {
			if a.Label != "" && isPortNumber(port) {
				if c.URLs.Names == nil {
					c.URLs.Names = map[string]string{}
				}
				c.URLs.Names[port] = a.Label
			}
		}
	})
	// remoteUser is who the tools run as, which is every command boxer runs; containerUser is the
	// fallback the specification gives it.
	set("user", c.User != "", func() { c.User = firstNonEmptyString(d.RemoteUser, d.ContainerUser) })
	// hostRequirements are minimums, so they only ever raise boxer's defaults.
	set("cpus", c.CPUs != Defaults().CPUs, func() {
		if d.HostRequirements.CPUs > c.CPUs {
			c.CPUs = d.HostRequirements.CPUs
		}
	})
	set("memory", c.Memory != Defaults().Memory, func() {
		if m, ok := hostMemory(d.HostRequirements.Memory); ok {
			if have, err := MemoryMiB(c.Memory); err != nil || m > have {
				c.Memory = fmt.Sprintf("%dM", m)
			}
		}
	})
	for _, key := range []string{"network", "urls", "user", "cpus", "memory"} {
		if c.Sources[key] == path && !d.sets(key) {
			delete(c.Sources, key)
		}
	}

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
		out = append(out, "`features` needs an image build, which boxer does not do ("+strings.Join(names, ", ")+
			"). Install them in `image_setup`, or build an image yourself and set `image` to it.")
	}
	if d.Build != nil || d.DockerFile != "" {
		out = append(out, "`build`/`dockerFile` needs an image build, which boxer does not do. "+
			"Build it yourself and set `image` to it: the tag on docker or podman, or a `docker save` archive on smolvm.")
	}
	var widen []string
	if len(d.RunArgs) > 0 {
		widen = append(widen, "runArgs")
	}
	if d.Privileged {
		widen = append(widen, "privileged")
	}
	if len(d.CapAdd) > 0 {
		widen = append(widen, "capAdd")
	}
	if len(d.SecurityOpt) > 0 {
		widen = append(widen, "securityOpt")
	}
	if len(widen) > 0 {
		out = append(out, "`"+strings.Join(widen, "`, `")+"` change the sandbox's own boundary, and boxer does not let a "+
			"repository file do that. They are ignored; if the project truly needs one, it needs a sandbox boxer does not provide.")
	}
	if d.Init {
		out = append(out, "`init` is ignored: boxer's guest already reaps the processes it starts.")
	}
	if d.WorkspaceMount != "" {
		out = append(out, "`workspaceMount` is ignored: boxer always mounts the worktree, at `workspaceFolder` (boxer's `mount_at`).")
	}
	if d.OverrideCommand != nil && !*d.OverrideCommand {
		out = append(out, "`overrideCommand: false` is ignored: boxer never runs the image's own command. "+
			"Put what it runs in `postStartCommand` (boxer's `start`).")
	}
	if d.PostAttachCommand != nil {
		out = append(out, "`postAttachCommand` is ignored: nothing attaches to a boxer sandbox. Use `postStartCommand` if it must run every start.")
	}
	if g := d.HostRequirements.GPU; g != nil && g != false && g != "optional" {
		out = append(out, "`hostRequirements.gpu` cannot be met: no boxer backend passes a GPU through.")
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

// ports maps forwardPorts and appPort onto boxer's "host:guest" forms.
//
// forwardPorts asks for a port to be reachable, not for a particular host port: the
// specification's `requireLocalPort` defaults to false, meaning any free local port will do. That
// is boxer's "auto:", and it matters — mapping 3000 to 3000 made the second worktree of any
// repository with a devcontainer fail to start on a busy address. A port whose attributes demand
// the same local port keeps it, and appPort, which is Docker's -p, is fixed by definition.
func (d devcontainer) ports() []string {
	var out []string
	for _, p := range d.ForwardPorts {
		port := ""
		switch t := p.(type) {
		case float64:
			port = strconv.Itoa(int(t))
		case string:
			// "host:port" names a Compose service's port; boxer has one guest, so it is that port.
			if _, after, found := strings.Cut(t, ":"); found {
				port = after
			} else {
				port = t
			}
		}
		if port == "" {
			continue
		}
		if d.PortsAttributes[port].RequireLocalPort {
			out = append(out, port+":"+port)
		} else {
			out = append(out, "auto:"+port)
		}
	}
	var app []any
	switch t := d.AppPort.(type) {
	case []any:
		app = t
	case nil:
	default:
		app = []any{t}
	}
	for _, p := range app {
		switch t := p.(type) {
		case float64:
			n := strconv.Itoa(int(t))
			out = append(out, n+":"+n)
		case string:
			if strings.Contains(t, ":") {
				out = append(out, t)
			} else if t != "" {
				out = append(out, t+":"+t)
			}
		}
	}
	return out
}

// sets reports whether the file actually said anything for a boxer key, so doctor does not
// attribute a default to it.
func (d devcontainer) sets(key string) bool {
	switch key {
	case "network":
		return len(d.ForwardPorts) > 0 || d.AppPort != nil
	case "urls":
		for port, a := range d.PortsAttributes {
			if a.Label != "" && isPortNumber(port) {
				return true
			}
		}
		return false
	case "user":
		return d.RemoteUser != "" || d.ContainerUser != ""
	case "cpus":
		return d.HostRequirements.CPUs > 0
	case "memory":
		_, ok := hostMemory(d.HostRequirements.Memory)
		return ok
	}
	return true
}

func isPortNumber(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0 && n < 65536
}

// hostMemory reads hostRequirements.memory ("8gb", "512mb", "1tb") as MiB.
func hostMemory(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	mult := map[string]float64{"tb": 1 << 20, "gb": 1 << 10, "mb": 1, "kb": 1.0 / 1024}
	for suffix, m := range mult {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(s, suffix), 64); strings.HasSuffix(s, suffix) && err == nil && n > 0 {
			return int(n*m + 0.5), true
		}
	}
	return 0, false
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// decodeDevcontainer parses the file and resolves the specification's ${...} variables in every
// string value before any of it is interpreted.
//
// Leaving them literal was a silent bug, and the worst kind: a bind mount whose source is the
// string "${localWorkspaceFolder}/data" makes the runtime create a directory by that name and
// mount it empty, and "${localEnv:GITHUB_TOKEN}" hands the guest twenty-four characters of
// nonsense as a token. Both look configured.
//
// What boxer cannot know on the host — ${containerEnv:X} outside remoteEnv, ${devcontainerId} — is
// reported by name, and an env value or mount that still depends on one is dropped rather than
// passed on half-substituted: a missing variable is a clearer failure than a wrong one.
func decodeDevcontainer(raw []byte, workspace, defaultMountAt string) (devcontainer, []string, error) {
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return devcontainer{}, nil, err
	}
	local := map[string]string{
		"localWorkspaceFolder":         workspace,
		"localWorkspaceFolderBasename": filepath.Base(workspace),
	}
	// containerWorkspaceFolder is workspaceFolder, which may itself use the local variables.
	cwf := defaultMountAt
	if wf, ok := tree["workspaceFolder"].(string); ok && wf != "" {
		cwf, _ = substitute(wf, local, nil)
	}
	vars := map[string]string{"containerWorkspaceFolder": cwf, "containerWorkspaceFolderBasename": path.Base(cwf)}
	for k, v := range local {
		vars[k] = v
	}
	// containerEnv is resolved first, so remoteEnv can refer to it — the one place the
	// specification allows ${containerEnv:...}, and the only one boxer can answer from the file.
	containerEnv := map[string]string{}
	if ce, ok := tree["containerEnv"].(map[string]any); ok {
		for k, v := range ce {
			if s, ok := v.(string); ok {
				containerEnv[k], _ = substitute(s, vars, nil)
			}
		}
	}
	missing := map[string]map[string]bool{}
	for key, v := range tree {
		var inGuest map[string]string
		if key == "remoteEnv" {
			inGuest = containerEnv
		}
		tree[key] = walkStrings(v, func(s string) string {
			out, bad := substitute(s, vars, inGuest)
			for _, b := range bad {
				if missing[key] == nil {
					missing[key] = map[string]bool{}
				}
				missing[key][b] = true
			}
			return out
		})
	}
	// Drop what still depends on an unresolved variable, in the two places a literal does harm.
	for _, key := range []string{"containerEnv", "remoteEnv"} {
		if m, ok := tree[key].(map[string]any); ok {
			for k, v := range m {
				if s, ok := v.(string); ok && unresolvedVar.MatchString(s) {
					delete(m, k)
				}
			}
		}
	}
	if ms, ok := tree["mounts"].([]any); ok {
		kept := ms[:0]
		for _, m := range ms {
			if s, ok := m.(string); ok && unresolvedVar.MatchString(s) {
				continue
			}
			if o, ok := m.(map[string]any); ok && (unresolvedVar.MatchString(fmt.Sprint(o["source"])) || unresolvedVar.MatchString(fmt.Sprint(o["target"]))) {
				continue
			}
			kept = append(kept, m)
		}
		tree["mounts"] = kept
	}
	var notes []string
	keys := make([]string, 0, len(missing))
	for k := range missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		names := make([]string, 0, len(missing[k]))
		for n := range missing[k] {
			names = append(names, "${"+n+"}")
		}
		sort.Strings(names)
		what := "left as written"
		if k == "containerEnv" || k == "remoteEnv" || k == "mounts" {
			what = "the entries using it are dropped"
		}
		notes = append(notes, fmt.Sprintf("`%s` uses %s, which boxer cannot resolve on the host; %s. "+
			"Set the value in boxer.toml instead.", k, strings.Join(names, ", "), what))
	}
	b, err := json.Marshal(tree)
	if err != nil {
		return devcontainer{}, nil, err
	}
	var d devcontainer
	err = json.Unmarshal(b, &d)
	return d, notes, err
}

// walkStrings applies f to every string in a decoded JSON value.
func walkStrings(v any, f func(string) string) any {
	switch t := v.(type) {
	case string:
		return f(t)
	case []any:
		for i := range t {
			t[i] = walkStrings(t[i], f)
		}
	case map[string]any:
		for k := range t {
			t[k] = walkStrings(t[k], f)
		}
	}
	return v
}

var (
	varRef        = regexp.MustCompile(`\$\{([A-Za-z]+)(?::([^}:]*))?(?::([^}]*))?\}`)
	unresolvedVar = regexp.MustCompile(`\$\{(containerEnv:[^}]*|devcontainerId)\}`)
)

// substitute resolves one string. localEnv (and its older spelling env) reads the host
// environment, with the specification's optional default; containerEnv reads guestEnv when there
// is one. Anything else it does not know is returned in bad and left in place.
func substitute(s string, vars, guestEnv map[string]string) (string, []string) {
	var bad []string
	out := varRef.ReplaceAllStringFunc(s, func(ref string) string {
		m := varRef.FindStringSubmatch(ref)
		kind, name, def := m[1], m[2], m[3]
		switch kind {
		case "localEnv", "env":
			if v, ok := os.LookupEnv(name); ok {
				return v
			}
			return def
		case "containerEnv":
			if guestEnv != nil {
				if v, ok := guestEnv[name]; ok {
					return v
				}
			}
			bad = append(bad, "containerEnv:"+name)
			return ref
		}
		if name == "" {
			if v, ok := vars[kind]; ok {
				return v
			}
		}
		if kind == "devcontainerId" {
			bad = append(bad, kind)
		}
		return ref
	})
	return out, bad
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
