//go:build ignore

// HOW THE CORPUS WRITES ITS CLAIMS (docs/surface-design.md §5.2).
//
//	go run gauntlet/surface/claims.go
//
// Over the programs (examples/, lib/, gauntlet/stdlib/acceptance/,
// gauntlet/differential/cases/) and, separately, the target files (targets/),
// read as S-expressions: every `sig`, its parameters with and without a set,
// its result, and its `where` and `ensures`; every `the`. A claim's formula is
// classified by what it contains, which decides what the logic must read:
// comparisons, the connectives, arithmetic (linear when every product has a
// literal factor), a call (len, or a definition), and the result's name.
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

var cmp = map[string]bool{"=": true, "<": true, "<=": true, ">": true, ">=": true, "/=": true,
	"!=": true, "===": true, "eq": true}

// base drops a host qualifier, so `go.<` and `java.>=` read as `<` and `>=`.
func base(h string) string {
	if i := strings.LastIndex(h, "."); i > 0 && i < len(h)-1 {
		return h[i+1:]
	}
	return h
}

func isLit(n *node) bool {
	return !n.list && strings.Trim(n.atom, "-0123456789") == ""
}

// shape names what a formula contains, cumulatively.
func shape(n *node, seen map[string]bool) {
	if !n.list {
		if n.atom == "result" {
			seen["the result's name"] = true
		}
		return
	}
	h := base(head(n))
	switch {
	case cmp[h]:
		seen["a comparison"] = true
	case h == "and" || h == "or" || h == "not" || h == "&&" || h == "||":
		seen["a connective ("+h+")"] = true
	case h == "+" || h == "-":
		seen["linear arithmetic"] = true
	case h == "*":
		if len(n.kids) == 3 && (isLit(n.kids[1]) || isLit(n.kids[2])) {
			seen["linear arithmetic"] = true
		} else {
			seen["a product of two variables (nonlinear)"] = true
		}
	case h == "/" || h == "%":
		seen["division or remainder"] = true
	case h == "len" || h == "alen":
		seen["len"] = true
	case h != "" && !cmp[h]:
		seen["another call ("+h+")"] = true
	}
	for _, k := range n.kids[1:] {
		shape(k, seen)
	}
}

func main() {
	for _, part := range []struct {
		name string
		dirs []string
	}{
		{"programs", []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"}},
		{"targets", []string{"targets"}},
	} {
		c := map[string]int{}
		for _, dir := range part.dirs {
			filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
					return nil
				}
				b, _ := os.ReadFile(p)
				var walk func(n *node)
				walk = func(n *node) {
					if !n.list {
						return
					}
					h := head(n)
					switch h {
					case "sig":
						c["sig"]++
						if len(n.kids) > 2 && n.kids[2].list {
							for _, prm := range n.kids[2].kids {
								// In a program's sig every parameter is a name
								// and its set; in a target file a bare atom is
								// a set without a name (target-files.md), and
								// `any` is the set of every value.
								switch {
								case prm.list && len(prm.kids) >= 2 && !prm.kids[1].list && prm.kids[1].atom == "any":
									c["  parameters: a name and the set any"]++
								case prm.list && len(prm.kids) >= 2:
									c["  parameters: a name and its set"]++
								case !prm.list && prm.atom == "any":
									c["  parameters: the set any, without a name"]++
								default:
									c["  parameters: a set without a name"]++
								}
							}
						}
						if len(n.kids) > 3 {
							r := n.kids[3]
							switch {
							case head(r) == "tuple":
								c["  results: a tuple"]++
							case head(r) == "host" || head(r) == "where" || head(r) == "ensures":
								c["  results: none written"]++
							default:
								c["  results: a set"]++
							}
						}
						for _, k := range n.kids[3:] {
							switch head(k) {
							case "where", "ensures":
								c["  "+head(k)]++
								seen := map[string]bool{}
								for _, f := range k.kids[1:] {
									shape(f, seen)
								}
								for s := range seen {
									c["    "+head(k)+" contains "+s]++
								}
								// A where whose every conjunct compares one and the
								// same parameter with terms not mentioning it is a
								// bound on that parameter: its set (§5.4, Finding 2).
								if head(k) == "where" && len(k.kids) == 2 {
									params := map[string]bool{}
									if len(n.kids) > 2 && n.kids[2].list {
										for _, prm := range n.kids[2].kids {
											if prm.list && len(prm.kids) >= 1 && !prm.kids[0].list {
												params[prm.kids[0].atom] = true
											}
										}
									}
									c["    where: "+classify(k.kids[1], params)]++
								}
							case "pure", "variadic", "index":
								c["  marker "+head(k)]++
							}
						}
					case "the":
						c["the"]++
						if len(n.kids) > 1 {
							switch t := n.kids[1]; {
							case head(t) == "int":
								c["  the: an integer range"]++
							case head(t) == "array" || head(t) == "map" || head(t) == "tuple":
								c["  the: a table, map or tuple"]++
							default:
								c["  the: another set"]++
							}
						}
					}
					for _, k := range n.kids {
						walk(k)
					}
				}
				for _, f := range parse(string(b)) {
					walk(f)
				}
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

// classify sorts a where by its conjuncts. A conjunct is a BOUND when it
// compares one parameter, bare or under len, with a term mentioning no other
// parameter; it is a RELATION when it mentions two parameters. A where of
// bounds alone is each parameter's set; one with a relation needs the where,
// or a dependent set on the later parameter (§5.4, Finding 2).
func classify(f *node, params map[string]bool) string {
	var atoms []*node
	var flat func(n *node)
	flat = func(n *node) {
		if b := base(head(n)); b == "and" || b == "&&" {
			for _, k := range n.kids[1:] {
				flat(k)
			}
			return
		}
		atoms = append(atoms, n)
	}
	flat(f)
	mentioned := func(n *node) map[string]bool {
		m := map[string]bool{}
		var walk func(n *node)
		walk = func(n *node) {
			if !n.list {
				if params[n.atom] {
					m[n.atom] = true
				}
				return
			}
			for _, k := range n.kids {
				walk(k)
			}
		}
		walk(n)
		return m
	}
	lengths, relation, other := false, false, false
	for _, a := range atoms {
		if !cmp[base(head(a))] || len(a.kids) != 3 {
			other = true
			continue
		}
		switch len(mentioned(a)) {
		case 0:
			other = true
		case 1:
			for _, side := range a.kids[1:] {
				if b := base(head(side)); b == "len" || b == "alen" {
					lengths = true
				}
			}
		default:
			relation = true
		}
	}
	switch {
	case other:
		return "another formula"
	case relation:
		return "a relation between parameters (len p = len q)"
	case lengths:
		return "bounds, one or more a length: each parameter's set"
	}
	return "bounds on parameters: each parameter's set"
}
