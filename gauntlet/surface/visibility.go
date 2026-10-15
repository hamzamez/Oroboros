//go:build ignore

// HOW MUCH OF A GO PACKAGE IS PUBLIC (docs/surface-design.md §6.8, item 3).
//
//	go run gauntlet/surface/visibility.go
//
// Counts top-level declarations exported against unexported in GOROOT/src,
// leaving out tests, testdata, vendor, internal and cmd; a package's majority is
// counted without constants, which generated syscall tables dominate.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	root := filepath.Join(runtime.GOROOT(), "src")
	c := map[string][2]int{}
	pkgs := map[string][2]int{}
	add := func(kind, pkg string, exp bool) {
		i := 1
		if exp {
			i = 0
		}
		v := c[kind]
		v[i]++
		c[kind] = v
		if kind == "const" {
			return
		}
		p := pkgs[pkg]
		p[i]++
		pkgs[pkg] = p
	}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			b := d.Name()
			if b == "testdata" || b == "vendor" || b == "internal" || rel == "cmd" || strings.HasPrefix(b, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil || f.Name.Name == "main" {
			return nil
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				k := "func"
				if d.Recv != nil {
					k = "method"
				}
				add(k, pkg, d.Name.IsExported())
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						add("type", pkg, s.Name.IsExported())
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.Name != "_" {
								add(strings.ToLower(d.Tok.String()), pkg, n.IsExported())
							}
						}
					}
				}
			}
		}
		return nil
	})
	te, tu := 0, 0
	for _, k := range []string{"func", "method", "type", "const", "var"} {
		v := c[k]
		te += v[0]
		tu += v[1]
		fmt.Printf("%-7s exported %6d  unexported %6d  (%.0f%% exported)\n", k, v[0], v[1], 100*float64(v[0])/float64(v[0]+v[1]))
	}
	fmt.Printf("%-7s exported %6d  unexported %6d  (%.0f%% exported)\n", "all", te, tu, 100*float64(te)/float64(te+tu))
	maj := 0
	for _, p := range pkgs {
		if p[0] > p[1] {
			maj++
		}
	}
	fmt.Printf("packages %d, of which %d export more than they hide\n", len(pkgs), maj)
	for _, p := range []string{"strings", "strconv", "bufio", "os", "io", "encoding/hex", "path/filepath", "errors", "sort", "bytes"} {
		v := pkgs[p]
		fmt.Printf("  %-14s %4d exported %4d unexported\n", p, v[0], v[1])
	}
}
