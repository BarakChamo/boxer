// Package bundle renders boxer's Agent Plugins 1.0.0 package from one template tree (R-LVL-6,
// R-PKG-1, R-PKG-5). The portable core is plugin.json, skills/boxer/SKILL.md and mcp.json; each
// client's hooks live in its reverse-domain extension directory, next to the native manifests the
// client's loader reads today. A per-harness bundle is a view: the subset of that tree one client
// reads, rendered with that harness's configuration overrides.
package bundle

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/BarakChamo/boxer/internal/box"
	"github.com/BarakChamo/boxer/internal/config"
	"github.com/BarakChamo/boxer/internal/shim"
)

//go:embed all:templates
var templates embed.FS

// Package is the Render target that produces the whole spec-shaped directory.
const Package = "plugin"

// Data is what every template sees.
type Data struct {
	Harness      string
	Version      string
	Mode         string
	Enforcement  string
	Intercept    []string
	Passthrough  []string
	MountAt      string
	Instructions string
	// ToolMode is true when the shell path should be removed or denied for this harness.
	ToolMode bool
	// Shims is true when the bundle should carry PATH shims.
	Shims         bool
	InterceptJSON string
}

// view is what one client reads out of the package.
type view struct {
	// Namespace is the client's extension directory (spec §8.2); boxer's choice until the client
	// publishes one.
	Namespace string
	// Native are the compatibility manifests and files the client's loader reads at fixed paths.
	Native []string
	// Alias copies a package file to the fixed path the client insists on (dst → src).
	Alias map[string]string
}

var shared = []string{"plugin.json", "mcp.json", "AGENTS.md", "skills/"}

var views = map[string]view{
	"claude-code": {Namespace: "com.anthropic.claude-code", Native: []string{".claude-plugin/", "agents/"}},
	"codex":       {Namespace: "com.openai.codex", Native: []string{".codex-plugin/", ".agents/plugins/"}},
	"grok":        {Namespace: "ai.x.grok", Native: []string{".grok-plugin/"}},
	// Gemini reads hooks/hooks.json at the extension root and nowhere else; Claude Code also
	// auto-loads that path and rejects Gemini's BeforeTool key, so the package keeps Gemini's
	// hooks in its namespace and only the Gemini view copies them to the fixed path.
	"gemini-cli": {Namespace: "com.google.gemini-cli", Native: []string{"gemini-extension.json"},
		Alias: map[string]string{"hooks/hooks.json": "com.google.gemini-cli/hooks/hooks.json"}},
	"kimi":     {Namespace: "ai.moonshot.kimi-code"},
	"copilot":  {Namespace: "com.github.copilot"},
	"dsh":      {Namespace: "com.deepseek.dsh"},
	"opencode": {Namespace: "ai.opencode"},
	"pi":       {Namespace: "works.earendil.pi"},
}

// Harnesses lists every view that can be rendered.
func Harnesses() []string {
	out := make([]string, 0, len(views))
	for h := range views {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// Namespace returns the extension directory for harness inside the package.
func Namespace(harness string) string { return views[harness].Namespace }

// Render writes the package (harness == Package) or one harness's view into dir and returns the
// files written.
func Render(harness string, cfg config.Config, version, dir string) ([]string, error) {
	v, ok := views[harness]
	if !ok && harness != Package {
		return nil, fmt.Errorf("no bundle for harness %q; known: %s, %s", harness, Package, strings.Join(Harnesses(), ", "))
	}
	ij, _ := json.Marshal(cfg.Intercept)
	d := Data{
		Harness: harness, Version: version, Mode: cfg.Mode, Enforcement: cfg.Enforcement,
		Intercept: cfg.Intercept, Passthrough: cfg.Passthrough, MountAt: cfg.MountAt,
		Instructions:  box.Instructions(cfg),
		ToolMode:      cfg.Mode == "tool",
		Shims:         cfg.Enforcement == "shim" || cfg.Enforcement == "both",
		InterceptJSON: string(ij),
	}
	partials, err := template.New("").Delims("[[", "]]").ParseFS(templates, "templates/_partials/*")
	if err != nil {
		return nil, err
	}
	wants := func(rel string) bool {
		if harness == Package {
			return true
		}
		for _, p := range append(append([]string{v.Namespace + "/"}, shared...), v.Native...) {
			if rel == p || strings.HasPrefix(rel, p) {
				return true
			}
		}
		return false
	}
	rendered := map[string][]byte{}
	const root = "templates/" + Package
	err = fs.WalkDir(templates, root, func(path string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, root+"/")
		if !wants(rel) {
			return nil
		}
		src, err := templates.ReadFile(path)
		if err != nil {
			return err
		}
		t, err := partials.Clone()
		if err != nil {
			return err
		}
		if _, err := t.New(rel).Parse(string(src)); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		var buf bytes.Buffer
		if err := t.ExecuteTemplate(&buf, rel, d); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if strings.HasSuffix(rel, ".json") {
			var x any
			if err := json.Unmarshal(buf.Bytes(), &x); err != nil {
				return fmt.Errorf("%s renders invalid JSON: %w\n%s", rel, err, buf.String())
			}
		}
		rendered[rel] = buf.Bytes()
		return nil
	})
	if err != nil {
		return nil, err
	}
	for dst, src := range v.Alias {
		rendered[dst] = rendered[src]
	}
	var written []string
	for rel, body := range rendered {
		out := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(out, body, 0o644); err != nil {
			return written, err
		}
		written = append(written, out)
	}
	sort.Strings(written)
	// Gap closer for the client that puts a bundle directory on the shell's PATH.
	if d.Shims && (harness == "claude-code" || harness == Package) {
		paths, err := shim.Install(filepath.Join(dir, "bin"), cfg.Intercept)
		if err != nil {
			return written, err
		}
		written = append(written, paths...)
	}
	return written, nil
}
