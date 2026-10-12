//go:build ignore

// HOW THE CORPUS CHOOSES AND ITERATES (docs/surface-design.md §3.2).
//
//	go run gauntlet/surface/control.go
//
// Over examples/, lib/, gauntlet/stdlib/acceptance/ and
// gauntlet/differential/cases/, read as S-expressions (the reader would erase
// `cond`, `match` and `seq`): every choice and every loop.
//   - the forms: `if`, `cond`, `case`, `match`, `loop`, `try`, `expect`, and
//     their arms;
//   - `if`: one-armed (the else is `(tuple)`, a statement) and connective (a
//     branch is a boolean literal, booleans.md);
//   - `case`: by pattern kind, with or without `else`;
//   - nesting: a chain form (`if`, `cond`, `case`, `match`, `loop`) in an arm
//     of another, the shape notation-research's H2 is about;
//   - loops: variables, `again` sites, and each `again` argument that is its
//     own variable unchanged (the identity), with the variables that are the
//     identity at every `again` (never changed by the loop) and, of those, the
//     ones whose initializer names a scope's buffer (ADR 0052 removes them);
//   - `try` and `expect` chains: one in another's success body;
//   - a two-armed case on result or option whose failure arm handles it: a
//     STEP when the success arm is 18 nodes or more and larger than the
//     failure arm (the rest of a program after opening a file), else a small
//     choice; and whether a step stands in tail position (§3.14).
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

// show writes a node back as an S-expression, to compare two by shape.
func show(n *node) string {
	if !n.list {
		return n.atom
	}
	parts := make([]string, len(n.kids))
	for i, k := range n.kids {
		parts[i] = show(k)
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// size counts a node and everything under it.
func size(n *node) int {
	s := 1
	for _, k := range n.kids {
		s += size(k)
	}
	return s
}

func head(n *node) string {
	if n != nil && n.list && len(n.kids) > 0 && !n.kids[0].list {
		return n.kids[0].atom
	}
	return ""
}

var chain = map[string]bool{"if": true, "cond": true, "case": true, "match": true, "loop": true}

// arms returns the arm bodies of a chain form, where nesting is counted;
// the scrutinee, the guards and a loop's initializers are not arms.
func arms(n *node) []*node {
	k := n.kids
	switch head(n) {
	case "if":
		if len(k) == 4 {
			return k[2:]
		}
	case "cond":
		var out []*node
		for i := 1; i+1 < len(k); i += 2 {
			out = append(out, k[i+1])
		}
		return out
	case "case":
		var out []*node
		for i := 2; i+1 < len(k); i += 2 {
			out = append(out, k[i+1])
		}
		return out
	case "loop":
		var out []*node
		for i := 2; i+1 < len(k); i += 2 {
			out = append(out, k[i+1])
		}
		return out
	case "match":
		// (match (s…) p… [(when c)] body … else body): a body ends each arm;
		// it is every element after a pattern run, which this approximates
		// as the last element and every element followed by a pattern start.
		return k[len(k)-1:]
	}
	return nil
}

func main() {
	c := map[string]int{}
	var nestEx []string
	for _, dir := range []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			// buffers: names a `local` binds to a table or an array, file-wide
			buffers := map[string]bool{}
			tail := map[*node]bool{}
			var walk func(n *node, inArm bool)
			walk = func(n *node, inArm bool) {
				if !n.list {
					return
				}
				h := head(n)
				k := n.kids
				if h == "local" {
					for i := 1; i+1 < len(k)-1; i += 2 {
						if hh := head(k[i+1]); hh == "table" || hh == "array" {
							buffers[k[i].atom] = true
						}
					}
				}
				// Where a sequence meets a chain, the precedence question of §3.3:
				// an arm whose body is a sequence, and a chain that something
				// follows (a non-last element of a `seq`, or a binder's value).
				if chain[h] || h == "try" || h == "expect" {
					c["chains, if, try and expect"]++
					bodies := arms(n)
					if h == "try" || h == "expect" {
						bodies = k[len(k)-1:]
					}
					for _, a := range bodies {
						c["  arm bodies"]++
						if hb := head(a); hb == "seq" || hb == "let" || hb == "local" {
							c["  arm bodies that are a sequence (seq, let, local)"]++
						}
					}
				}
				isChain := func(x *node) bool {
					hx := head(x)
					return chain[hx] || hx == "try" || hx == "expect"
				}
				if h == "seq" {
					for _, x := range k[1 : len(k)-1] {
						if isChain(x) {
							c["  a chain followed by more: a non-last element of a seq"]++
						}
					}
				}
				if (h == "let" || h == "local") && len(k) >= 4 && (len(k)-2)%2 == 0 {
					for i := 2; i < len(k)-1; i += 2 {
						if isChain(k[i]) {
							c["  a chain followed by more: a binder's value"]++
						}
					}
				}
				switch h {
				case "if":
					c["if"]++
					if len(k) == 4 {
						if head(k[3]) == "tuple" && len(k[3].kids) == 1 {
							c["if: one-armed, the else (tuple)"]++
						}
						if !k[2].list && (k[2].atom == "true" || k[2].atom == "false") ||
							!k[3].list && (k[3].atom == "true" || k[3].atom == "false") {
							c["if: a branch a boolean literal (a connective)"]++
						}
					}
				case "cond":
					c["cond"]++
					c["cond: arms"] += (len(k) - 1) / 2
				case "case":
					c["case"]++
					hasElse := false
					for i := 2; i+1 < len(k); i += 2 {
						c["case: arms"]++
						pat := k[i]
						switch {
						case !pat.list && pat.atom == "else":
							hasElse = true
						case pat.list:
							c["case: a constructor pattern with a payload"]++
						case pat.atom == "_":
							c["case: a wildcard pattern"]++
						default:
							c["case: a nullary constructor or literal"]++
						}
					}
					if hasElse {
						c["case: with else"]++
					}
					// A two-armed case on result or option: what its failure arm
					// does. Re-building the failure is `try`, abandoning is
					// `expect`, and anything else is a handled bind (§3.4).
					if len(k) == 6 {
						ctor := func(p *node) string {
							if p.list && len(p.kids) > 0 {
								return p.kids[0].atom
							}
							return p.atom
						}
						fail, ok := -1, -1
						for _, i := range []int{2, 4} {
							switch ctor(k[i]) {
							case "result.err", "err", "none", "option.none":
								fail = i
							case "result.ok", "ok", "some", "option.some":
								ok = i
							}
						}
						if fail > 0 && ok > 0 {
							c["case: two arms on result or option"]++
							switch body := k[fail+1]; {
							case show(body) == show(k[fail]):
								c["case:   … the failure arm re-builds the failure (try)"]++
							case head(body) == "abandon":
								c["case:   … the failure arm abandons (expect)"]++
							default:
								c["case:   … the failure arm handles it (a handled bind)"]++
								// What layout fits it: the sizes of its two arms, in
								// nodes, and whether it stands in tail position.
								s, f := size(k[ok+1]), size(k[fail+1])
								switch {
								case s >= 18 && s > f:
									c["case:     … a step: the success arm 18 nodes or more and larger"]++
									if tail[n] {
										c["case:       … in tail position"]++
									}
								default:
									c["case:     … a small choice: the success arm under 18 nodes, or not larger"]++
								}
							}
						}
					}
				case "match":
					c["match"]++
					for _, x := range k {
						if head(x) == "when" {
							c["match: guards (when)"]++
						}
					}
				case "loop":
					c["loop"]++
					if len(k) >= 2 && k[1].list {
						vars := k[1].kids
						c["loop: variables"] += len(vars)
						c[fmt.Sprintf("loop: loops with %d variable(s)", len(vars))]++
						for _, v := range vars {
							if v.list && len(v.kids) == 2 && !v.kids[1].list && buffers[v.kids[1].atom] {
								c["loop: variables carrying a scope's buffer"]++
							}
						}
						c["loop: clauses"] += (len(k) - 2) / 2
						names := make([]string, len(vars))
						for i, v := range vars {
							if v.list && len(v.kids) == 2 {
								names[i] = v.kids[0].atom
							}
						}
						// every again of this loop: not inside a nested loop
						var agains [][]*node
						var find func(x *node)
						find = func(x *node) {
							if !x.list {
								return
							}
							if head(x) == "loop" && x != n {
								return
							}
							if head(x) == "again" {
								agains = append(agains, x.kids[1:])
							}
							for _, y := range x.kids {
								find(y)
							}
						}
						find(n)
						c["loop: again sites"] += len(agains)
						for i, nm := range names {
							same := len(agains) > 0
							for _, a := range agains {
								if i < len(a) {
									c["loop: again arguments"]++
									carrier := false
									if v := vars[i]; v.list && len(v.kids) == 2 && !v.kids[1].list && buffers[v.kids[1].atom] {
										carrier = true
										c["loop: again arguments for a variable carrying a scope's buffer"]++
									}
									if !a[i].list && a[i].atom == nm {
										c["loop: again arguments that are the variable unchanged"]++
										if !carrier {
											c["loop:   … for a variable not carrying a buffer"]++
										}
									} else {
										same = false
									}
								}
							}
							if same {
								c["loop: variables unchanged at every again"]++
								if v := vars[i]; v.list && len(v.kids) == 2 && !v.kids[1].list && buffers[v.kids[1].atom] {
									c["loop:   … of which carry a scope's buffer"]++
								}
							}
						}
					}
				case "try", "expect":
					c[h]++
					if len(k) > 0 {
						if hb := head(k[len(k)-1]); hb == "try" || hb == "expect" {
							c["try/expect: in another's success body"]++
						}
					}
				}
				if chain[h] && inArm && h != "if" {
					c["clause chains (no if) nested in an arm of any chain"]++
				}
				if chain[h] && inArm {
					c["chain forms nested in an arm of another"]++
					if len(nestEx) < 10 {
						nestEx = append(nestEx, p+": "+h)
					}
				}
				if chain[h] {
					c["chain forms"]++
					if h != "if" {
						c["clause chains (cond, case, match, loop)"]++
					}
				}
				armSet := map[*node]bool{}
				if chain[h] {
					for _, a := range arms(n) {
						armSet[a] = true
					}
				}
				for _, x := range k {
					// a lambda resets the context: its body is not the arm's
					walk(x, armSet[x] || (inArm && !chain[h] && h != "fn"))
				}
			}
			forms := parse(string(b))
			// TAIL POSITION: the last element of a sequence, a binding's body, a
			// λ's or a definition's body, a loop's clause body, and inherited by
			// a choice's arms. A handled bind can be written with the rest of a
			// block as its success arm only where it is in tail position.
			var markTail func(n *node, t bool)
			markTail = func(n *node, t bool) {
				if !n.list {
					return
				}
				tail[n] = t
				h, k := head(n), n.kids
				for i, x := range k {
					xt := false
					switch {
					case (h == "seq" || h == "let" || h == "local" || h == "fn" || h == "def") && i == len(k)-1:
						xt = true
					case h == "if" && i >= 2, (h == "case" || h == "cond") && i >= 2 && i%2 == 1:
						xt = t
					case h == "loop" && i >= 2 && i%2 == 1:
						xt = true
					}
					markTail(x, xt)
				}
			}
			for _, f := range forms {
				markTail(f, true)
			}
			for _, f := range forms {
				walk(f, false)
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
	fmt.Println("\nnested chains, the first ten:")
	for _, e := range nestEx {
		fmt.Println("  " + e)
	}
}
