package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Every example in examples/ is loaded the way boxer loads a repository: it must parse, validate,
// and produce no warning. An example that drifts from the configuration format is worse than none,
// because it is copied.
func TestExamplesLoad(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		_, tomlErr := os.Stat(filepath.Join(dir, "boxer.toml"))
		if tomlErr != nil && findDevcontainer(dir) == "" {
			continue
		}
		n++
		t.Run(e.Name(), func(t *testing.T) {
			cfg, err := Load(dir, dir)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if len(cfg.Warnings) > 0 {
				t.Fatalf("an example must not warn: %v", cfg.Warnings)
			}
			if len(cfg.Network.Ports) > 0 && cfg.Network.Ports[0][:5] != "auto:" {
				t.Errorf("an example must forward auto: ports, not %v", cfg.Network.Ports)
			}
		})
	}
	if n < 8 {
		t.Fatalf("found %d examples, expected at least 8", n)
	}
}
