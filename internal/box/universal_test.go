package box_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The core knows nothing about any harness: harness facts are data in internal/hook (the dialect
// table), internal/inside (the harness table) and internal/bundle (the templates), never code in
// the packages that resolve scope, run commands, or talk to smolvm. This test fails when a harness
// name appears in core code, so the constraint is checked rather than remembered. Comments are
// allowed: explaining why a parameter exists is not a code path.
func TestCoreNamesNoHarness(t *testing.T) {
	harness := regexp.MustCompile(`(?i)\b(claude|codex|gemini|opencode|kimi|grok|dsh|deepseek|paperclip|openhands|multica)\b`)
	for _, pkg := range []string{"box", "vm", "scope", "config", "decide", "shim"} {
		dir := filepath.Join("..", pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0) // no comments
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				var text string
				switch x := n.(type) {
				case *ast.Ident:
					text = x.Name
				case *ast.BasicLit:
					text = x.Value
				default:
					return true
				}
				if harness.MatchString(text) {
					t.Errorf("%s:%d: core code names a harness (%q); harness facts belong in a table, not in the core",
						path, fset.Position(n.Pos()).Line, text)
				}
				return true
			})
		}
	}
}
