// Package bundle renders one plugin per harness from a single set of templates (R-PKG-1, R-PKG-5).
// Every bundle carries the same four components: instruction, lifecycle hooks, run tool, gap closer.
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

// Harnesses lists every bundle that can be rendered.
func Harnesses() []string {
	entries, _ := fs.ReadDir(templates, "templates")
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Render writes the bundle for harness into dir and returns the files written.
func Render(harness string, cfg config.Config, version, dir string) ([]string, error) {
	root := "templates/" + harness
	if _, err := fs.Stat(templates, root); err != nil {
		return nil, fmt.Errorf("no bundle for harness %q; known: %s", harness, strings.Join(Harnesses(), ", "))
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
	var written []string
	err := fs.WalkDir(templates, root, func(path string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, root+"/")
		src, err := templates.ReadFile(path)
		if err != nil {
			return err
		}
		t, err := template.New(rel).Delims("[[", "]]").Parse(string(src))
		if err != nil {
			return fmt.Errorf("%s/%s: %w", harness, rel, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, d); err != nil {
			return fmt.Errorf("%s/%s: %w", harness, rel, err)
		}
		if strings.HasSuffix(rel, ".json") {
			var v any
			if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
				return fmt.Errorf("%s/%s renders invalid JSON: %w\n%s", harness, rel, err, buf.String())
			}
		}
		out := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(out, buf.Bytes(), mode); err != nil {
			return err
		}
		written = append(written, out)
		return nil
	})
	if err != nil {
		return written, err
	}
	// Gap closer for harnesses that put a bundle directory on the shell's PATH.
	if d.Shims && harness == "claude-code" {
		paths, err := shim.Install(filepath.Join(dir, "bin"), cfg.Intercept)
		if err != nil {
			return written, err
		}
		written = append(written, paths...)
	}
	return written, nil
}
