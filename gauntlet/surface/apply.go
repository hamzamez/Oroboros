//go:build ignore

// HOW THE CORPUS APPLIES (docs/surface-design.md §7.2).
//
//	go run gauntlet/surface/apply.go
//
// Over the programs (examples/, lib/, gauntlet/stdlib/acceptance/,
// gauntlet/differential/cases/), read as S-expressions: where a λ, `(fn …)`,
// is written (a definition's body, a binding's value, a table's rule, an
// argument of a call, applied where it stands); calls through an import, and
// among them calls of a companion's operation (`(de.Name ent)`, the
// receiver-style candidates); a call whose first argument is itself a call
// through an import (a chain a receiver form would read left to right); and
// a host call applied again, `((errors.AsType T) e)`: a static application.
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

func name(n *node) string {
	if len(n.kids) < 2 {
		return ""
	}
	if k := n.kids[1]; k.list {
		return head(k)
	}
	return n.kids[1].atom
}

func main() {
	c := map[string]int{}
	for _, dir := range []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			forms := parse(string(b))
			// The file's imports: alias -> path.
			uses := map[string]string{}
			for _, f := range forms {
				if head(f) == "use" && len(f.kids) >= 2 && !f.kids[1].list {
					path := f.kids[1].atom
					alias := path[strings.LastIndex(path, "/")+1:]
					if len(f.kids) == 4 && f.kids[2].atom == "as" {
						alias = f.kids[3].atom
					}
					uses[alias] = path
				}
			}
			qualified := func(n *node) (string, bool) {
				h := head(n)
				i := strings.Index(h, ".")
				if i <= 0 {
					return "", false
				}
				path, ok := uses[h[:i]]
				return path, ok
			}
			var walk func(n, parent *node, pos int)
			walk = func(n, parent *node, pos int) {
				if !n.list {
					return
				}
				if head(n) == "fn" {
					switch ph := head(parent); {
					case parent == nil:
					case pos == 0:
						c["λ: applied where it stands, ((fn …) a)"]++
					case ph == "def":
						c["λ: a definition's value, (def f (fn …))"]++
					case ph == "let" || ph == "local":
						c["λ: a binding's value"]++
					case ph == "table" || ph == "build" || ph == "build-map":
						c["λ: a table's rule or a region, (table n (fn (k) …))"]++
					case len(parent.kids) > 0 && parent.kids[0].list:
						c["λ: a continuation of a call's results, ((f a) (fn (q r) …))"]++
					default:
						if _, ok := qualified(parent); ok {
							c["λ: an argument of a host or library call"]++
						} else {
							c["λ: an argument of the program's own call (static, higher-order)"]++
						}
					}
				}
				if len(n.kids) > 1 && n.kids[0].list && head(n.kids[1]) != "fn" {
					if _, ok := qualified(n.kids[0]); ok {
						c["a host call applied again, ((errors.AsType T) e)"]++
					}
				}
				if path, ok := qualified(n); ok {
					c["calls through an import"]++
					last := path[strings.LastIndex(path, "/")+1:]
					if last != "" && last[0] >= 'A' && last[0] <= 'Z' {
						c["… of a companion's operation, (de.Name ent)"]++
					}
					if len(n.kids) > 1 {
						if _, inner := qualified(n.kids[1]); inner {
							c["… whose first argument is a call through an import"]++
						}
					}
				}
				for i, k := range n.kids {
					walk(k, n, i)
				}
			}
			for _, f := range forms {
				walk(f, nil, 0)
			}
			c["files"]++
			return nil
		})
	}
	var ks []string
	for k := range c {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("%6d  %s\n", c[k], k)
	}
}
