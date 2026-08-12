package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Lists exported declarations with no doc comment, which is what a stranger
// reading pkg.go.dev would find missing.
func main() {
	root := os.Args[1]
	missing := map[string][]string{}
	total, documented := 0, 0

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		pkg := filepath.Dir(path)
		report := func(name string, doc *ast.CommentGroup) {
			// A method is only part of the surface if its own name is
			// exported: Session.send is private however public Session is.
			// A method counts only if both its own name and its receiver are
			// exported: writerObserver.Data is not part of any surface.
			for _, part := range strings.Split(name, ".") {
				if !ast.IsExported(part) {
					return
				}
			}
			total++
			if doc != nil && len(doc.List) > 0 {
				documented++
				return
			}
			missing[pkg] = append(missing[pkg], name)
		}

		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.FuncDecl:
				name := typed.Name.Name
				if typed.Recv != nil && len(typed.Recv.List) > 0 {
					name = receiver(typed.Recv.List[0].Type) + "." + name
				}
				report(name, typed.Doc)
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						doc := s.Doc
						if doc == nil {
							doc = typed.Doc
						}
						report(s.Name.Name, doc)
					case *ast.ValueSpec:
						doc := s.Doc
						if doc == nil {
							doc = typed.Doc
						}
						for _, name := range s.Names {
							report(name.Name, doc)
						}
					}
				}
			}
		}
		return nil
	})

	packages := make([]string, 0, len(missing))
	for pkg := range missing {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	for _, pkg := range packages {
		sort.Strings(missing[pkg])
		fmt.Printf("%-14s %d undocumented: %s\n", pkg, len(missing[pkg]), strings.Join(missing[pkg], ", "))
	}
	fmt.Printf("\n%d of %d exported declarations documented (%.0f%%)\n",
		documented, total, float64(documented)*100/float64(total))
}

func receiver(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiver(typed.X)
	case *ast.Ident:
		return typed.Name
	}
	return "?"
}
