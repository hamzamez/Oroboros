//go:build ignore

// GO'S OUT-PARAMETERS, MEASURED (docs/outparams-research.md §3,
// gauntlet/results/outparams-2026-10-08.md).
//
//	go run gauntlet/stdlib/outparams.go
//
// Go's out-parameters are not in its types: an explicit pointer to a value is
// rare, and those that exist are shared or retained cells (sync/atomic, flag).
// The ones that are results are typed `any` and documented as pointers, so this
// reads the documentation: every exported function or method with an `any`
// parameter whose doc comment says the argument is stored through, must be a
// pointer, or is decoded into. The list is read one name at a time in the
// result; encoders that only READ their argument match the phrases too and are
// set aside there.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Exported functions and methods with a parameter of type any/interface{}
// whose doc comment says the argument must be (or is stored through) a pointer.
func main() {
	root := filepath.Join(goroot(), "src")
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	ptr := regexp.MustCompile(`(?i)(must be a (non-nil )?pointer|pointer to|stores? (the result|successive|the decoded)|into the value pointed to|decodes? .* into|pointed to by|storing|pointed at by|into successive|into the value|copies .* into|fills)`)
	var hits []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") ||
			strings.Contains(p, "internal") || strings.Contains(p, "testdata") || strings.Contains(p, "vendor") ||
			strings.Contains(p, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
			return nil
		}
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, p, nil, parser.ParseComments)
		if err != nil || strings.HasSuffix(f.Name.Name, "_test") || f.Name.Name == "main" {
			return nil
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !fd.Name.IsExported() || fd.Doc == nil {
				continue
			}
			anyParam := false
			for _, fl := range fd.Type.Params.List {
				t := fl.Type
				if e, ok := t.(*ast.Ellipsis); ok {
					t = e.Elt
				}
				switch x := t.(type) {
				case *ast.InterfaceType:
					if len(x.Methods.List) == 0 {
						anyParam = true
					}
				case *ast.Ident:
					if x.Name == "any" {
						anyParam = true
					}
				}
			}
			if !anyParam || !ptr.MatchString(fd.Doc.Text()) {
				continue
			}
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			name := fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				rt := fd.Recv.List[0].Type
				if s, ok := rt.(*ast.StarExpr); ok {
					rt = s.X
				}
				if id, ok := rt.(*ast.Ident); ok {
					if !id.IsExported() {
						continue
					}
					name = id.Name + "." + name
				}
			}
			first := ptr.FindString(fd.Doc.Text())
			hits = append(hits, filepath.ToSlash(rel)+"."+name+"   ["+first+"]")
		}
		return nil
	})
	sort.Strings(hits)
	for _, h := range hits {
		fmt.Println(h)
	}
	fmt.Println("total", len(hits))
}

func goroot() string {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
