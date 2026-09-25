package cli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The CLI's presentation must not leak into the core: nothing under internal/ or pkg/ except this
// package may import it. How boxer looks to a person is cmd/boxer's business; the core is also a
// library (pkg/boxer), and a library that decides whether to print colour is wrong for everyone
// who embeds it.
func TestCoreDoesNotImportCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "pkg"} {
		err := filepath.Walk(filepath.Join(root, dir), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.Contains(p, string(filepath.Separator)+"cli"+string(filepath.Separator)) {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				if strings.Contains(imp.Path.Value, "boxer/internal/cli") || strings.Contains(imp.Path.Value, "boxer/cmd/") {
					t.Errorf("%s imports %s: the core must not depend on the CLI", p, imp.Path.Value)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
