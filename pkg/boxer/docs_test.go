package boxer_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/BarakChamo/boxer/pkg/boxer"
)

// The Go reference is written by hand. This test fails when an exported function, type, method or
// field of pkg/boxer is missing from it, including the fields of the internal types the package
// re-exports as aliases, which a reader of the reference cannot see any other way.
func TestGoReferenceCoversTheWholePackage(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "site", "content", "docs", "reference", "go.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)

	var names []string
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if !d.Name.IsExported() {
						continue
					}
					if d.Recv != nil {
						names = append(names, "Box."+d.Name.Name)
					} else {
						names = append(names, d.Name.Name)
					}
				case *ast.GenDecl:
					for _, s := range d.Specs {
						if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.IsExported() {
							names = append(names, ts.Name.Name)
						}
					}
				}
			}
		}
	}
	if len(names) < 15 {
		t.Fatalf("found only %d exported names; the parser has stopped seeing the package", len(names))
	}

	var missing []string
	for _, n := range names {
		if !strings.Contains(doc, n) {
			missing = append(missing, n)
		}
	}
	// Every exported field and method of every exported type, aliases included.
	for _, v := range []any{boxer.Options{}, boxer.RunOpts{}, boxer.Machine{}, boxer.Scope{}, boxer.Error{}} {
		typ := reflect.TypeOf(v)
		sec := section(doc, "## "+typ.Name())
		for i := 0; i < typ.NumField(); i++ {
			if f := typ.Field(i); f.IsExported() && !regexp.MustCompile(`\b`+f.Name+`\b`).MatchString(sec) {
				missing = append(missing, typ.Name()+"."+f.Name)
			}
		}
		for _, t := range []reflect.Type{typ, reflect.PointerTo(typ)} {
			for i := 0; i < t.NumMethod(); i++ {
				if m := t.Method(i).Name; m != "Error" && !regexp.MustCompile(`\b`+m+`\b`).MatchString(sec) {
					missing = append(missing, typ.Name()+"."+m+"()")
				}
			}
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("go.mdx does not document %s", m)
	}
}

// section is the text under a "## Heading" line, up to the next heading of that level.
func section(doc, heading string) string {
	i := strings.Index(doc, heading+"\n")
	if i < 0 {
		return ""
	}
	rest := doc[i+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
