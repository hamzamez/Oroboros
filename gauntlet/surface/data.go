//go:build ignore

// HOW THE CORPUS WRITES ITS TABLES (docs/surface-design.md §3.10).
//
//	go run gauntlet/surface/data.go
//
// Over examples/, lib/, gauntlet/stdlib/acceptance/ and
// gauntlet/differential/cases/, read as S-expressions: every table written as
// its graph, `(array e…)`, by length, and every table written by a rule,
// `(table n f)`, by what f is: a literal (the constant function K v), a λ, or
// a name. A two-element graph and a rule with a literal are the pair one
// bracket would tell apart under `array { a, b }` against `array(n, v)`.
// Also counted (§4.2): maps by their graph, `(map K V)` as a type, map
// buffers and their stores, `keys`, tuples and the unit, integer ranges
// `(int lo hi)` (sets of integers today), and the two string operations; and
// where a variant's constructors are written, in constructors below.
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

// isType reports an element that names a set of values rather than one:
// a sort, a host type, or a range, table, buffer, map, tuple or option former.
func isType(n *node) bool {
	if !n.list {
		switch n.atom {
		case "int", "bool", "f64", "string", "bytestring", "byte":
			return true
		}
		return strings.HasPrefix(n.atom, "go.") || strings.HasPrefix(n.atom, "java.") ||
			strings.HasPrefix(n.atom, "js.") || strings.HasPrefix(n.atom, "win.")
	}
	switch head(n) {
	case "int", "array", "buffer", "map", "tuple", "option", "cell":
		return true
	}
	return false
}

func main() {
	c := map[string]int{}
	lengths := map[int]int{}
	for _, dir := range []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			var walk func(n *node, parent string)
			walk = func(n *node, parent string) {
				if !n.list {
					return
				}
				h := head(n)
				// `(array V)` as a type sits in a sig or a declaration; a graph
				// is an array with at least one element that is not a type.
				if h == "array" && parent != "sig" && parent != "type" && len(n.kids) >= 2 {
					if len(n.kids) == 2 && isType(n.kids[1]) {
						c["(array T), a type: the same spelling as a one-element graph"]++
					} else {
						c["graphs (array e…)"]++
						lengths[len(n.kids)-1]++
					}
				}
				// The other data forms installment 4 covers, by their head.
				switch {
				case h == "map" && len(n.kids) >= 2 && n.kids[1].list:
					c["maps by their graph (map (k v) …)"]++
				case h == "map" && parent != "sig" && parent != "type" && len(n.kids) == 3 && !n.kids[1].list:
					c["(map K V), a type: the same head as a map's graph"]++
				case h == "build-map":
					c["map buffers (build-map …)"]++
				case h == "insert":
					c["map stores (insert m k v)"]++
				case h == "keys":
					c["(keys m)"]++
				case h == "tuple" && len(n.kids) == 1:
					c["the unit (tuple)"]++
				case h == "tuple" && parent != "let":
					c["tuples (tuple a b …), built"]++
				case h == "int" && len(n.kids) == 3:
					c["ranges (int lo hi), a set of integers"]++
				case h == "concat":
					c["strings: concat"]++
				case h == "string-of":
					c["strings: string-of"]++
				}
				if h == "table" && len(n.kids) == 3 {
					c["rules (table n f)"]++
					f := n.kids[2]
					switch {
					case !f.list && (f.atom == "0" || f.atom == "false" || f.atom == "0.0"):
						c["  … f the zero, a zero-filled buffer"]++
					case !f.list && (strings.Trim(f.atom, "-0123456789.") == "" || f.atom == "true" || strings.HasPrefix(f.atom, `"`)):
						c["  … f another literal, a constant table"]++
					case head(f) == "fn":
						c["  … f a λ"]++
					case !f.list:
						c["  … f a name (a definition, or a frozen table: thaw)"]++
					default:
						c["  … f another expression"]++
					}
				}
				for _, k := range n.kids {
					walk(k, h)
				}
			}
			for _, f := range parse(string(b)) {
				walk(f, "")
			}
			return nil
		})
	}
	constructors(c)
	var ks []string
	for k := range c {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("%6d  %s\n", c[k], k)
	}
	var ls []int
	for l := range lengths {
		ls = append(ls, l)
	}
	sort.Ints(ls)
	fmt.Print("graphs by length:")
	for _, l := range ls {
		fmt.Printf(" %d:%d", l, lengths[l])
	}
	fmt.Println()
}

// constructors counts where a variant's constructors are written (§4.3): as
// expressions, where `c(x)` is an application of the injection and looks
// like any call, and as patterns, where it is matched; how many patterns are
// nullary, a bare name a reader cannot tell from a binder without the
// declaration; and how many definitions share a constructor's name. The
// constructors are every variant declared in the corpus, the targets and
// lib, and `option`'s, which the reducer declares (`some`, `none`).
func constructors(c map[string]int) {
	ctors := map[string]bool{"some": true, "none": true}
	defs := map[string]bool{}
	dirs := []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases", "targets"}
	files := map[string][]*node{}
	for _, dir := range dirs {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			files[p] = parse(string(b))
			return nil
		})
	}
	var collect func(n *node)
	collect = func(n *node) {
		if !n.list {
			return
		}
		switch head(n) {
		case "variant":
			for _, k := range n.kids[2:] {
				switch {
				case !k.list:
					ctors[k.atom] = true
				case head(k) != "success" && head(k) != "relevant":
					ctors[head(k)] = true
				}
			}
		case "def":
			if len(n.kids) > 1 && !n.kids[1].list {
				defs[n.kids[1].atom] = true
			}
		}
		for _, k := range n.kids {
			collect(k)
		}
	}
	for _, forms := range files {
		for _, f := range forms {
			collect(f)
		}
	}
	// Which constructors a file sees: those it declares, `option`'s, and any
	// written qualified (`result.ok`, `os.not-found`). A bare name declared as
	// a constructor only in another module is that file's own name.
	var local map[string]bool
	isCtor := func(name string) bool {
		if i := strings.LastIndex(name, "."); i >= 0 {
			return ctors[name[i+1:]]
		}
		return local[name] || name == "some" || name == "none"
	}
	declared := func(forms []*node) map[string]bool {
		m := map[string]bool{}
		var d func(n *node)
		d = func(n *node) {
			if !n.list {
				return
			}
			if head(n) == "variant" {
				for _, k := range n.kids[2:] {
					switch {
					case !k.list:
						m[k.atom] = true
					case head(k) != "success" && head(k) != "relevant":
						m[head(k)] = true
					}
				}
			}
			for _, k := range n.kids {
				d(k)
			}
		}
		for _, f := range forms {
			d(f)
		}
		return m
	}
	pattern := func(p *node, where string) {
		switch {
		case p.list && isCtor(head(p)):
			c["constructors as patterns, with a payload: c(x)"]++
			c["  … in "+where]++
		case !p.list && isCtor(p.atom):
			c["constructors as patterns, nullary: a bare name"]++
			c["  … in "+where]++
		}
	}
	var walk func(n *node, inPattern bool)
	walk = func(n *node, inPattern bool) {
		if !n.list {
			if !inPattern && isCtor(n.atom) {
				c["constructors as expressions, nullary"]++
			}
			return
		}
		h, k := head(n), n.kids
		pats := map[*node]bool{}
		switch h {
		case "case":
			for i := 2; i+1 < len(k); i += 2 {
				pattern(k[i], "a case arm")
				pats[k[i]] = true
			}
		case "try", "expect":
			if len(k) > 1 {
				pattern(k[1], "a try or expect binder")
				pats[k[1]] = true
			}
		case "variant", "sig", "def":
			if h == "variant" {
				return
			}
		}
		if !inPattern && isCtor(h) {
			c["constructors as expressions, applied: c(x)"]++
		}
		for i, x := range k {
			if i == 0 && !x.list {
				continue
			}
			walk(x, inPattern || pats[x])
		}
	}
	for p, forms := range files {
		if strings.HasPrefix(filepath.ToSlash(p), "targets/") {
			continue
		}
		local = declared(forms)
		for _, f := range forms {
			walk(f, false)
		}
	}
	shared := 0
	for d := range defs {
		if ctors[d] {
			shared++
		}
	}
	c["constructors declared"] = len(ctors)
	c["names a constructor in one module and a definition in another"] = shared
}
