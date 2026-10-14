//go:build ignore

// HOW THE CORPUS DECLARES (docs/surface-design.md §6.2).
//
//	go run gauntlet/surface/decls.go
//
// Over the programs (examples/, lib/, gauntlet/stdlib/acceptance/,
// gauntlet/differential/cases/) and, separately, the target files (targets/),
// read as S-expressions: every top-level and module-level form by its head;
// `use` with and without an alias, and whether an alias is the path's last
// segment; `export` lists; `variant` declarations, their constructors, payloads
// and parameters; `sig` beside `def`; target declarations (`type`, `repr`,
// `implements`, …); host bodies and their clauses; module nesting; and whether
// an outermost module's path restates the file's own path (the path of a
// target file below targets/, of a library file below lib/).
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

var declHeads = map[string]bool{"module": true, "use": true, "export": true, "def": true, "sig": true,
	"variant": true, "type": true, "repr": true, "implements": true, "include": true, "const": true,
	"provides": true, "structural": true, "fact": true, "underlying": true, "backend": true, "link": true,
	"max-len": true, "big-repr": true, "int-repr": true, "target": true}

func main() {
	for _, part := range []struct {
		name string
		dirs []string
	}{
		{"programs", []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"}},
		{"targets", []string{"targets"}},
	} {
		c := map[string]int{}
		cur := "" // the file's path, as a module path would spell it
		var walk func(n *node, depth int, sigs, defs map[string]bool)
		walk = func(n *node, depth int, sigs, defs map[string]bool) {
			h := head(n)
			if !declHeads[h] {
				return
			}
			c["form "+h]++
			switch h {
			case "target":
				for _, k := range n.kids[2:] {
					walk(k, depth, sigs, defs)
				}
			case "module", "provides":
				c[fmt.Sprintf("  %s at nesting depth %d", h, depth)]++
				if h == "module" && depth == 0 {
					switch p := name(n); {
					case p == cur:
						c["  module at depth 0: its path is the file's path"]++
					case strings.HasPrefix(p, cur+"/"):
						c["  module at depth 0: a child of the file's path"]++
					default:
						c["  module at depth 0: another path than the file's"]++
					}
				}
				from := 2
				if h == "provides" {
					from = 3
				}
				inner, innerDefs := map[string]bool{}, map[string]bool{}
				for _, k := range n.kids[from:] {
					walk(k, depth+1, inner, innerDefs)
				}
				pair(c, inner, innerDefs)
			case "use":
				// A companion (theories.md §3.3) is a type's child module: its
				// path's last segment is the type's name, capitalized.
				if len(n.kids) >= 2 && !n.kids[1].list {
					path := n.kids[1].atom
					if last := path[strings.LastIndex(path, "/")+1:]; last != "" && last[0] >= 'A' && last[0] <= 'Z' {
						c["  use: of a companion module"]++
					}
				}
				switch {
				case len(n.kids) == 2:
					c["  use: no alias (the last segment)"]++
				case len(n.kids) == 4 && n.kids[2].atom == "as":
					path, alias := n.kids[1].atom, n.kids[3].atom
					last := path[strings.LastIndex(path, "/")+1:]
					if alias == last {
						c["  use: an alias equal to the last segment"]++
					} else {
						c["  use: an alias other than the last segment"]++
					}
				}
			case "export":
				c["  export: names exported"] += len(n.kids) - 1
			case "variant":
				if n.kids[1].list {
					c["  variant: with type parameters"]++
				}
				for _, k := range n.kids[2:] {
					switch {
					case head(k) == "success" || head(k) == "relevant" || k.atom == "relevant":
						c["  variant: marker "+head(k)+k.atom]++
					case !k.list:
						c["  variant: a constructor with no payload"]++
						c["  variant: constructors"]++
					default:
						c["  variant: constructors"]++
						c[fmt.Sprintf("  variant: a constructor with %d payload(s)", len(k.kids)-1)]++
					}
				}
			case "sig":
				sigs[name(n)] = true
				if n.kids[1].list {
					c["  sig: a head with static parameters"]++
				}
				for _, k := range n.kids[3:] {
					w := head(k)
					if w == "" {
						w = k.atom
					}
					switch w {
					case "host":
						if len(k.kids) > 1 {
							c["  sig: host "+k.kids[1].atom]++
						}
						for _, hc := range k.kids[3:] {
							hw := head(hc)
							c["  sig: host clause "+hw]++
						}
					case "where", "ensures", "pure", "variadic", "index":
						c["  sig: "+w]++
					}
				}
			case "def":
				defs[name(n)] = true
			case "type":
				if len(n.kids) == 3 && head(n.kids[2]) == "host" {
					c["  type: a host type"]++
				} else {
					c["  type: a manifest type (a definiens)"]++
				}
			}
		}
		for _, dir := range part.dirs {
			filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
					return nil
				}
				b, _ := os.ReadFile(p)
				c["files"]++
				cur = strings.TrimSuffix(filepath.ToSlash(p), ".oro")
				for _, root := range []string{"targets/", "lib/"} {
					cur = strings.TrimPrefix(cur, root)
				}
				sigs, defs := map[string]bool{}, map[string]bool{}
				for _, f := range parse(string(b)) {
					walk(f, 0, sigs, defs)
				}
				pair(c, sigs, defs)
				return nil
			})
		}
		var ks []string
		for k := range c {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Println("== " + part.name)
		for _, k := range ks {
			fmt.Printf("%6d  %s\n", c[k], k)
		}
	}
}

// pair counts, in one module, the definitions with a `sig` beside them and the
// sigs with no definition (a host declaration, in a target file).
func pair(c map[string]int, sigs, defs map[string]bool) {
	for n := range defs {
		if sigs[n] {
			c["  def with a sig beside it"]++
		} else {
			c["  def with no sig"]++
		}
	}
	for n := range sigs {
		if !defs[n] {
			c["  sig with no def (a host declaration or a claim)"]++
		}
	}
}
