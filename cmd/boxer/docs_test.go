package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The CLI reference is written by hand, and a flag added to the code without a line in it is a
// flag nobody can find. This test reads every flag this package defines and every command the
// dispatcher accepts, and fails when the reference does not name it.
func TestCLIReferenceCoversEveryCommandAndFlag(t *testing.T) {
	ref, err := os.ReadFile(filepath.Join("..", "..", "site", "content", "docs", "reference", "cli.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(ref)

	fset := token.NewFileSet()
	files, _ := filepath.Glob("*.go")
	flags := map[string]bool{}
	commands := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				name := sel.Sel.Name
				arg := 0
				switch name {
				case "Bool", "String", "Int", "Duration", "Float64", "Uint":
				case "BoolVar", "StringVar", "IntVar", "DurationVar", "Var":
					arg = 1
				default:
					return true
				}
				if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "fs" {
					return true
				}
				if len(n.Args) > arg {
					if lit, ok := n.Args[arg].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v, _ := strconv.Unquote(lit.Value)
						flags[v] = true
					}
				}
			case *ast.FuncDecl:
				if n.Name.Name != "run" {
					return true
				}
				// The top-level dispatcher: every case string is a command.
				ast.Inspect(n.Body, func(m ast.Node) bool {
					sw, ok := m.(*ast.SwitchStmt)
					if !ok {
						return true
					}
					for _, st := range sw.Body.List {
						for _, e := range st.(*ast.CaseClause).List {
							if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								v, _ := strconv.Unquote(lit.Value)
								commands[v] = true
							}
						}
					}
					return false // only the outermost switch
				})
			}
			return true
		})
	}
	if len(flags) < 30 || len(commands) < 20 {
		t.Fatalf("found only %d flags and %d commands; the parser has stopped seeing the code", len(flags), len(commands))
	}

	var missing []string
	for f := range flags {
		// A flag is documented when the reference shows it as `-f` or `--flag`, alone or with an
		// argument, anywhere: a flag table row, a synopsis or a code block.
		re := regexp.MustCompile(`(^|[\s\[|(` + "`" + `])--?` + regexp.QuoteMeta(f) + `($|[\s\]|)=` + "`" + `])`)
		if !re.MatchString(doc) {
			missing = append(missing, "flag -"+f)
		}
	}
	for c := range commands {
		if strings.HasPrefix(c, "-") || c == "help" {
			continue
		}
		if !strings.Contains(doc, "boxer "+c) {
			missing = append(missing, "command "+c)
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("cli.mdx does not document %s", m)
	}
}
