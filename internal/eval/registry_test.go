package eval

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Drivers() is a hand-written list, and the failure mode of a hand-written list is that someone
// adds a driver file and forgets the line: the cell then silently never runs, and a matrix that
// silently omits a row is worse than one that fails. So the list is checked against the source
// rather than against another list — the same shape as TestCoreNamesNoHarness.
//
// A driver is any type in this package with a `Cells(tier string) []Cell` method. Unexported
// types (the adherence wrapper, the checklist, a test fake) are the composition machinery, not
// rows, and are skipped.
func TestEveryDriverInTheSourceIsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, d := range Drivers() {
		registered[strings.TrimPrefix(typeName(d), "*")] = true
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(".", e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Cells" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			name := receiverType(fn.Recv.List[0].Type)
			if name == "" || !ast.IsExported(name) {
				continue
			}
			if !registered[name] {
				t.Errorf("%s declares driver %s, which Drivers() does not list: its cells never run",
					e.Name(), name)
			}
		}
	}
}

func receiverType(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// typeName is the driver's Go type name without its package or pointer marker.
func typeName(d Driver) string {
	s := fmt.Sprintf("%T", d)
	s = strings.TrimPrefix(s, "*")
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}
