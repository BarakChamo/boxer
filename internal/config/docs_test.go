package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The configuration reference is written by hand. A key added to Config without a section there
// is a key nobody can find, so this test walks every toml tag and every BOXER_* override and fails
// when the reference does not name it.
func TestConfigurationReferenceCoversEveryKey(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "site", "content", "docs", "reference", "configuration.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)

	var keys []string
	var walk func(prefix string, typ reflect.Type)
	walk = func(prefix string, typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := strings.Split(f.Tag.Get("toml"), ",")[0]
			if tag == "" || tag == "-" {
				continue
			}
			key := prefix + tag
			switch {
			case f.Type.Kind() == reflect.Struct:
				walk(key+".", f.Type)
			case f.Type.Kind() == reflect.Map && f.Type.Elem().Kind() == reflect.Struct:
				// [tasks.<name>] and [harness.<name>]: one section for the table, and each of its
				// keys named in it.
				keys = append(keys, key+".<name>")
				for j := 0; j < f.Type.Elem().NumField(); j++ {
					if sub := strings.Split(f.Type.Elem().Field(j).Tag.Get("toml"), ",")[0]; sub != "" && sub != "-" {
						keys = append(keys, key+".<name>:"+sub)
					}
				}
			default:
				keys = append(keys, key)
			}
		}
	}
	walk("", reflect.TypeOf(Config{}))
	if len(keys) < 40 {
		t.Fatalf("found only %d keys; the walk has stopped seeing Config", len(keys))
	}

	var missing []string
	for _, k := range keys {
		if head, sub, ok := strings.Cut(k, ":"); ok {
			// A table's own keys are listed inside its section.
			sec := section(doc, "### `"+head+"`")
			if !strings.Contains(sec, "`"+sub+"`") {
				missing = append(missing, head+" key "+sub)
			}
			continue
		}
		if !strings.Contains(doc, "### `"+k+"`") {
			missing = append(missing, k)
		}
	}
	for name := range envKeys {
		if !strings.Contains(doc, "`"+name+"`") {
			missing = append(missing, "variable "+name)
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("configuration.mdx does not document %s", m)
	}
}

func section(doc, heading string) string {
	i := strings.Index(doc, heading)
	if i < 0 {
		return ""
	}
	rest := doc[i+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	if j := strings.Index(rest, "\n### "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// The preset table is written out by hand; every preset and every host in it must be there.
func TestConfigurationReferenceListsEveryPresetHost(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "site", "content", "docs", "reference", "configuration.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	sec := section(string(b), "### `network.allow_presets`")
	for _, name := range PresetNames() {
		row := ""
		for _, line := range strings.Split(sec, "\n") {
			if strings.HasPrefix(line, "| `"+name+"` |") {
				row = line
			}
		}
		if row == "" {
			t.Errorf("preset %s has no row", name)
			continue
		}
		for _, h := range Presets[name] {
			if !strings.Contains(row, "`"+h+"`") {
				t.Errorf("preset %s: row does not list %s", name, h)
			}
		}
	}
}
