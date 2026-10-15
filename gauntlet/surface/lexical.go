//go:build ignore

// THE CORPUS AT THE LEXICAL LEVEL (docs/surface-design.md §8.2).
//
//	go run gauntlet/surface/lexical.go
//
// Over the programs (examples/, lib/, gauntlet/stdlib/acceptance/,
// gauntlet/differential/cases/) and, separately, the target files (targets/),
// strings and comments set aside: the names a program BINDS (definitions,
// parameters, `let`/`local`/loop binders, import aliases) and their shapes —
// with `-`, ending in `?` or `!`, with a capital, non-ASCII — and the binders
// that equal a reserved word of J; qualified names whose member is an operator
// (`go.%`); integer literals by magnitude and by the forms 2ᵏ, 2ᵏ−1 and −2ᵏ;
// float literals; `(pow 2 k)`; and comments, a line of their own or trailing.
package main

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// jReserved are J's words that open a form (installments 1–7); a binder equal
// to one must be renamed by the translator.
var jReserved = map[string]bool{"fn": true, "spec": true, "lemma": true, "set": true, "array": true,
	"map": true, "var": true, "let": true, "loop": true, "again": true, "case": true, "if": true,
	"else": true, "try": true, "expect": true, "assert": true, "use": true, "module": true,
	"provides": true, "private": true, "export": true, "axiom": true, "host": true, "true": true,
	"false": true}

// jContextual are J's words that only a clause position reads (a header's
// clauses, a binder's mode, a host body's clauses, a set's markers).
var jContextual = map[string]bool{"where": true, "ensure": true, "invariant": true, "pure": true,
	"inout": true, "out": true, "linear": true, "as": true, "import": true, "expr": true, "stmt": true,
	"comma_ok": true, "success": true, "relevant": true}

type tok struct {
	text string
	str  bool
}

func tokens(src string) ([]tok, int, int) {
	var ts []tok
	own, trailing := 0, 0
	lineHasCode := false
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			lineHasCode = false
			i++
		case c == ';':
			if lineHasCode {
				trailing++
			} else {
				own++
			}
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			ts = append(ts, tok{src[i : j+1], true})
			lineHasCode = true
			i = j + 1
		case c == '(' || c == ')':
			ts = append(ts, tok{string(c), false})
			lineHasCode = true
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\r\n();\"", rune(src[j])) {
				j++
			}
			ts = append(ts, tok{src[i:j], false})
			lineHasCode = true
			i = j
		}
	}
	return ts, own, trailing
}

var integer = regexp.MustCompile(`^-?[0-9]+$`)
var float = regexp.MustCompile(`^-?[0-9]+\.[0-9]+([eE][-+]?[0-9]+)?$|^-?[0-9]+[eE][-+]?[0-9]+$`)

func main() {
	for _, part := range []struct {
		name string
		dirs []string
	}{
		{"programs", []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"}},
		{"targets", []string{"targets"}},
	} {
		c := map[string]int{}
		bound := map[string]bool{}
		for _, dir := range part.dirs {
			filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
					return nil
				}
				b, _ := os.ReadFile(p)
				ts, own, trailing := tokens(string(b))
				c["comments: a line of their own"] += own
				c["comments: trailing code"] += trailing
				for i, t := range ts {
					if t.str || t.text == "(" || t.text == ")" {
						continue
					}
					x := t.text
					switch {
					case integer.MatchString(x):
						c["integer literals"]++
						v, _ := new(big.Int).SetString(x, 10)
						a := new(big.Int).Abs(v)
						if a.Cmp(big.NewInt(1<<31)) >= 0 {
							c["… of magnitude ≥ 2^31"]++
						}
						if a.BitLen() >= 9 {
							one := big.NewInt(1)
							p := new(big.Int).Lsh(one, uint(a.BitLen()-1))
							q := new(big.Int).Sub(new(big.Int).Lsh(one, uint(a.BitLen())), one)
							switch {
							case a.Cmp(p) == 0 && v.Sign() > 0:
								c["… of the form 2^k, k ≥ 8"]++
							case a.Cmp(p) == 0:
								c["… of the form -2^k, k ≥ 8"]++
							case a.Cmp(q) == 0:
								c["… of the form 2^k - 1, k ≥ 9"]++
							}
						}
						if len(strings.TrimPrefix(x, "-")) >= 10 {
							c["… of 10 digits or more"]++
						}
						continue
					case float.MatchString(x):
						c["float literals"]++
						if strings.ContainsAny(x, "eE") {
							c["… with an exponent"]++
						}
						continue
					}
					if x == "pow" && i > 0 && ts[i-1].text == "(" {
						c["(pow 2 k)"]++
					}
					if k := strings.LastIndex(x, "."); k > 0 && k < len(x)-1 {
						m := x[k+1:]
						if strings.IndexFunc(m, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) < 0 {
							c["qualified names whose member is an operator (go.%)"]++
						}
					}
					// Binders: (def NAME …), (let NAME e …), loop and local binders,
					// parameter lists, `as NAME`.
					if i >= 2 && ts[i-2].text == "(" && (ts[i-1].text == "def" || ts[i-1].text == "variant") {
						bound[x] = true
					}
					if i >= 1 && ts[i-1].text == "as" {
						bound[x] = true
					}
					if i >= 3 && ts[i-1].text == "(" && ts[i-2].text == "(" && (ts[i-3].text == "loop" || ts[i-3].text == "def" ||
						ts[i-3].text == "fn" || ts[i-3].text == "local" || ts[i-3].text == "sig") {
						bound[x] = true
					}
					if i >= 3 && ts[i-1].text == "(" && (ts[i-2].text == "(" || ts[i-2].text == ")") && i >= 4 && ts[i-4].text == "loop" {
						bound[x] = true
					}
				}
				return nil
			})
		}
		under := map[string][]string{}
		for n := range bound {
			c["bound names (distinct)"]++
			if strings.Contains(n, "-") && n != "-" {
				c["… containing -"]++
			}
			if strings.HasSuffix(n, "?") {
				c["… ending in ?"]++
			}
			if strings.HasSuffix(n, "!") {
				c["… ending in !"]++
			}
			if strings.IndexFunc(n, unicode.IsUpper) >= 0 {
				c["… with a capital"]++
			}
			if strings.IndexFunc(n, func(r rune) bool { return r > 127 }) >= 0 {
				c["… non-ASCII"]++
			}
			if jReserved[n] {
				c["… equal to a word J reserves: "+n]++
			}
			if jContextual[n] {
				c["… equal to a word J reads in a clause: "+n]++
			}
			t := strings.ReplaceAll(strings.TrimSuffix(n, "?"), "-", "_")
			under[t] = append(under[t], n)
		}
		for t, ns := range under {
			if len(ns) > 1 {
				sort.Strings(ns)
				c["… colliding once - becomes _ and ? is dropped: "+t+" ← "+strings.Join(ns, ", ")]++
			}
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
