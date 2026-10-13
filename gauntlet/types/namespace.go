//go:build ignore

// WHERE A TYPE AND A VALUE SHARE A NAME (docs/spec/typevalues.md §2).
//
//	go run gauntlet/types/namespace.go
//
// Types as static values (ADR 0049) put a type and a value in one namespace: a
// name denotes one thing. Today a module has two namespaces, so a name may be
// both a type (`type`, `variant`) and a value (`sig`, `def`, `const`, a
// variant's constructor). This counts, per module, the names that are both, over
// the target files and the programs, and prints each with what it names.
//
// A module is a `(module PATH …)` (nested ones joined by `/`), a `(provides T
// PATH …)`, or, in a program, the file. A companion module, `go/strconv/NumError`
// beside the type `go/strconv.NumError`, is a third meaning of the name, counted
// separately: it is a namespace, not a value.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type node struct {
	atom string
	kids []*node
	list bool
}

func parse(src string) []*node {
	var toks []string
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '(' || c == ')':
			toks = append(toks, string(c))
			i++
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			toks = append(toks, src[i:j+1])
			i = j + 1
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\n\r();\"", rune(src[j])) {
				j++
			}
			toks = append(toks, src[i:j])
			i = j
		}
	}
	stack := [][]*node{{}}
	for _, t := range toks {
		switch t {
		case "(":
			stack = append(stack, nil)
		case ")":
			if len(stack) < 2 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = append(stack[len(stack)-1], &node{kids: top, list: true})
		default:
			stack[len(stack)-1] = append(stack[len(stack)-1], &node{atom: t})
		}
	}
	return stack[0]
}

func head(n *node) string {
	if n != nil && n.list && len(n.kids) > 0 && !n.kids[0].list {
		return n.kids[0].atom
	}
	return ""
}

// declName is the name a declaration binds: `(sig NAME …)`, `(sig (NAME T…) …)`,
// `(variant NAME …)`, `(variant (NAME T…) …)`.
func declName(n *node) string {
	if len(n.kids) < 2 {
		return ""
	}
	k := n.kids[1]
	if k.list {
		return head(k)
	}
	return k.atom
}

type space struct {
	types, values map[string][]string // name -> what declares it
	file          string
}

func main() {
	mods := map[string]*space{}
	children := map[string]bool{} // module paths, for companions
	get := func(m, file string) *space {
		if mods[m] == nil {
			mods[m] = &space{types: map[string][]string{}, values: map[string][]string{}, file: file}
		}
		return mods[m]
	}
	var walk func(n *node, mod, file string)
	walk = func(n *node, mod, file string) {
		switch h := head(n); h {
		case "module":
			if len(n.kids) > 1 && !n.kids[1].list {
				m := n.kids[1].atom
				if mod != "" && !strings.HasPrefix(mod, "file:") {
					m = mod + "/" + m // a child's path is its parent's and its own (target-files.md §1a)
				}
				children[m] = true
				for _, k := range n.kids[2:] {
					walk(k, m, file)
				}
			}
		case "provides":
			if len(n.kids) > 2 && !n.kids[2].list {
				m := n.kids[2].atom
				children[m] = true
				for _, k := range n.kids[3:] {
					walk(k, m, file)
				}
			}
		case "target":
			for _, k := range n.kids[2:] {
				walk(k, "", file)
			}
		case "type", "variant":
			s := get(mod, file)
			name := declName(n)
			if name != "" {
				s.types[name] = append(s.types[name], h)
			}
			if h == "variant" {
				for _, c := range n.kids[2:] {
					if c.list && head(c) != "success" && head(c) != "relevant" {
						s.values[head(c)] = append(s.values[head(c)], "constructor")
					}
				}
			}
		case "sig", "def", "const":
			name := declName(n)
			if name != "" {
				get(mod, file).values[name] = append(get(mod, file).values[name], h)
			}
		}
	}
	for _, dir := range []string{"targets", "lib", "examples", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			mod := ""
			if !strings.HasPrefix(p, "targets") {
				mod = "file:" + filepath.ToSlash(p)
			}
			for _, f := range parse(string(b)) {
				walk(f, mod, filepath.ToSlash(p))
			}
			return nil
		})
	}
	var both, companion []string
	nTypes := 0
	for m, s := range mods {
		for t := range s.types {
			nTypes++
			if vs := s.values[t]; len(vs) > 0 {
				both = append(both, fmt.Sprintf("%-28s %-14s type (%s) and value (%s)   %s", m, t,
					strings.Join(s.types[t], ","), strings.Join(dedup(vs), ","), s.file))
			}
			if children[m+"/"+t] || (m == "" && children[t]) {
				companion = append(companion, m+"."+t)
			}
		}
	}
	sort.Strings(both)
	sort.Strings(companion)
	fmt.Printf("%d type names declared; %d also name a value in their module:\n", nTypes, len(both))
	for _, l := range both {
		fmt.Println("  " + l)
	}
	fmt.Printf("%d also name a companion module (a namespace, not a value):\n  %s\n", len(companion),
		strings.Join(companion, "\n  "))
}

func dedup(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
