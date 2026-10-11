//go:build ignore

// HOW THE CORPUS NESTS ITS SCOPES (docs/surface-design.md §2.13).
//
//	go run gauntlet/surface/scopes.go
//
// Over examples/, lib/, gauntlet/stdlib/acceptance/ and
// gauntlet/differential/cases/, read as S-expressions (not through the
// reader, which would erase `let`): every `let` and `local`, its binders by
// kind, and every CHAIN of scopes, a scope whose body is a scope. A chain's
// RUNS are its maximal blocks of one head; merging `let` and `local` writes
// a chain as one scope, so it removes runs − 1 nesting levels. A buffer is a
// `local` binder whose initializer is `table` or `array` (ADR 0047), a cell
// any other. A name bound twice in one group is counted, since today `let`
// shadows and `local` refuses.
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

// scope reports a `let` or `local` with an odd number of elements after the
// head (binder, value pairs, then the body), and returns its pairs and body.
func scope(n *node) (string, []*node, *node, bool) {
	h := head(n)
	if (h != "let" && h != "local") || len(n.kids) < 4 || (len(n.kids)-2)%2 != 0 {
		return "", nil, nil, false
	}
	return h, n.kids[1 : len(n.kids)-1], n.kids[len(n.kids)-1], true
}

func main() {
	c := map[string]int{}
	runs := map[int]int{}
	var examples []string
	isChild := map[*node]bool{}
	for _, dir := range []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			forms := parse(string(b))
			var all []*node
			var walk func(n *node)
			walk = func(n *node) {
				if !n.list {
					return
				}
				all = append(all, n)
				for _, k := range n.kids {
					walk(k)
				}
			}
			for _, f := range forms {
				walk(f)
			}
			for _, n := range all {
				h, pairs, body, ok := scope(n)
				if !ok {
					continue
				}
				c[h+" forms"]++
				seen := map[string]bool{}
				for i := 0; i+1 < len(pairs); i += 2 {
					name := pairs[i].atom
					if pairs[i].list {
						name = ""
					}
					switch {
					case h == "let":
						c["let binders"]++
					case head(pairs[i+1]) == "table" || head(pairs[i+1]) == "array":
						c["local binders: buffers"]++
					default:
						c["local binders: cells"]++
					}
					if name != "" && name != "_" && seen[name] {
						c[h+" groups binding a name twice"]++
					}
					seen[name] = true
				}
				if _, _, _, ok := scope(body); ok {
					isChild[body] = true
				}
			}
			// Chains: from each scope that is not another's body, follow bodies.
			for _, n := range all {
				h, _, _, ok := scope(n)
				if !ok || isChild[n] {
					continue
				}
				heads := []string{h}
				cur := n
				for {
					_, _, body, _ := scope(cur)
					hb, _, _, ok := scope(body)
					if !ok {
						break
					}
					heads = append(heads, hb)
					cur = body
				}
				r := 1
				for i := 1; i < len(heads); i++ {
					if heads[i] != heads[i-1] {
						r++
					}
				}
				if len(heads) > 1 {
					c["chains of two or more scopes"]++
					key := strings.Join(heads, " > ")
					c["  chain "+key]++
					if r > 1 && len(examples) < 12 {
						examples = append(examples, p+": "+key)
					}
				}
				runs[r]++
			}
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
	fmt.Println("\noutermost scopes by number of runs (merging removes runs − 1 levels):")
	var rs []int
	for r := range runs {
		rs = append(rs, r)
	}
	sort.Ints(rs)
	for _, r := range rs {
		fmt.Printf("  %d run(s): %d\n", r, runs[r])
	}
	fmt.Println("\nchains that alternate heads:")
	for _, e := range examples {
		fmt.Println("  " + e)
	}
}
