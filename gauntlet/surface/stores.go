//go:build ignore

// WHERE A LOOP CARRIES A BUFFER (ADR 0052; storeorder-research.md §3).
//
//	go run gauntlet/surface/stores.go [-v]
//
// The state-passing translation S (spec/local.md §3) makes a state binder a
// loop's variable where the loop writes it, appended after the loop's own
// variables. Unthreading U, its converse on today's corpus, removes every loop
// variable that only carries a buffer. S ∘ U is the identity exactly when each
// such variable stands where S puts it: after the loop's own variables, in the
// order the buffers were bound. This counts, over the corpus's trees (μ of
// examples/, lib/, gauntlet/stdlib/acceptance/, gauntlet/differential/cases/):
//
//   - the loop variables whose initializer is a buffer in scope (a `local`
//     table, a `build-map`, or a variable of an enclosing loop that carries one);
//   - where each stands among its loop's variables;
//   - the loops whose carriers are a suffix in binding order, where S ∘ U is
//     the identity on the loop header, and the others, where it permutes it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oroboros/core"
)

type counter struct {
	c        map[string]int
	verbose  bool
	examples []string
	file     string
	moves    []string
}

func (k *counter) tail(t core.Tail, bufs []string) {
	switch t := t.(type) {
	case *core.Again:
		k.exprs(t.Args, bufs)
	case *core.TLet:
		for _, b := range t.Binds {
			k.expr(b.Value, bufs)
		}
		k.tail(t.Body, bufs)
	case *core.TSeq:
		k.exprs(t.Xs, bufs)
		k.tail(t.Last, bufs)
	case *core.TMonad:
		k.exprs(t.Items, bufs)
		k.tail(t.Body, bufs)
	case core.Expr:
		k.expr(t, bufs)
	}
}

func (k *counter) exprs(es []core.Expr, bufs []string) {
	for _, e := range es {
		k.expr(e, bufs)
	}
}

func index(xs []string, x string) int {
	for i, y := range xs {
		if y == x {
			return i
		}
	}
	return -1
}

func (k *counter) expr(e core.Expr, bufs []string) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *core.Lam:
		k.expr(e.Body, bufs)
	case *core.Apply:
		k.expr(e.Fn, bufs)
		k.exprs(e.Args, bufs)
	case *core.Index:
		k.expr(e.Table, bufs)
		k.expr(e.At, bufs)
	case *core.Let:
		for _, b := range e.Binds {
			k.expr(b.Value, bufs)
		}
		k.expr(e.Body, bufs)
	case *core.Case:
		k.expr(e.Scrut, bufs)
		for _, a := range e.Arms {
			k.expr(a.Body, bufs)
		}
	case *core.Local:
		inner := append([]string(nil), bufs...)
		for _, b := range e.Binds {
			k.expr(b.Init, inner)
			if b.Buffer {
				inner = append(inner, b.Name)
				k.c["buffer binders"]++
			}
		}
		k.expr(e.Body, inner)
	case *core.BuildMap:
		k.exprs(e.Caps, bufs)
		inner := append(append([]string(nil), bufs...), e.Names...)
		k.c["map buffer binders"] += len(e.Names)
		k.expr(e.Body, inner)
	case *core.Loop:
		k.exprs(e.Inits, bufs)
		k.c["loops"]++
		// The carriers, by the binding order of the buffer each carries.
		var carriers []int // positions
		var order []int    // the carried buffer's place in bufs
		inner := append([]string(nil), bufs...)
		for j, init := range e.Inits {
			v, ok := init.(*core.Var)
			if !ok {
				continue
			}
			if b := index(bufs, v.Name); b >= 0 {
				carriers = append(carriers, j)
				order = append(order, b)
				inner = append(inner, e.Vars[j])
			}
		}
		if len(carriers) > 0 {
			k.c["loops carrying a buffer"]++
			k.c["loop variables carrying a buffer"] += len(carriers)
			n := len(e.Vars)
			suffix := true
			for i, j := range carriers {
				if j != i {
					suffix = false
				}
				switch {
				case n == 1:
					k.c["  … the loop's only variable"]++
				case j == n-1:
					k.c["  … last"]++
				case j == 0:
					k.c["  … first"]++
				default:
					k.c["  … between"]++
				}
			}
			if suffix && sort.IntsAreSorted(order) {
				k.c["loops whose carriers are a prefix in binding order (S ∘ U the identity)"]++
			} else {
				k.c["loops whose carriers are not (S ∘ U permutes the header)"]++
				if len(k.examples) < 40 {
					k.examples = append(k.examples, fmt.Sprintf("%s: (%s), carriers at %v", k.file, strings.Join(e.Vars, " "), carriers))
				}
			}
		}
		for _, a := range e.Arms {
			if a.Guard != nil {
				k.expr(a.Guard, inner)
			}
			if a.When != nil {
				k.expr(a.When, inner)
			}
			k.tail(a.Body, inner)
			// What each back edge hands a carrier's place: a version of the same
			// buffer (a store chain on it, or a call that borrows it) is S of a
			// program-order write; another buffer's is a move between places.
			for _, ag := range agains(a.Body) {
				for _, j := range carriers {
					if j >= len(ag.Args) {
						continue
					}
					switch r := root(ag.Args[j], inner); {
					case r == e.Vars[j]:
						k.c["back-edge arguments: a version of the carrier's own buffer"]++
					case r == "":
						k.c["back-edge arguments: no buffer found (a fresh table?)"]++
						k.note(fmt.Sprintf("%s: (%s) position %d: no root", k.file, strings.Join(e.Vars, " "), j))
					default:
						k.c["back-edge arguments: ANOTHER buffer moved into the place"]++
						k.note(fmt.Sprintf("%s: (%s) position %d receives %s", k.file, strings.Join(e.Vars, " "), j, r))
					}
				}
			}
		}
	case *core.The:
		k.expr(e.E, bufs)
	case *core.If:
		k.expr(e.C, bufs)
		k.expr(e.A, bufs)
		k.expr(e.B, bufs)
	case *core.And:
		k.exprs(e.Xs, bufs)
	case *core.Or:
		k.exprs(e.Xs, bufs)
	case *core.Not:
		k.expr(e.X, bufs)
	case *core.Cond:
		for _, a := range e.Arms {
			k.expr(a.Guard, bufs)
			k.expr(a.Body, bufs)
		}
	case *core.Seq:
		k.exprs(e.Xs, bufs)
	case *core.Monad:
		k.exprs(e.Items, bufs)
		k.expr(e.Body, bufs)
	}
}

func (k *counter) note(s string) {
	if len(k.moves) < 60 {
		k.moves = append(k.moves, s)
	}
}

// agains is a tail's back edges to its own loop.
func agains(t core.Tail) []*core.Again {
	switch t := t.(type) {
	case *core.Again:
		return []*core.Again{t}
	case *core.TLet:
		return agains(t.Body)
	case *core.TSeq:
		return agains(t.Last)
	case *core.TMonad:
		return agains(t.Body)
	}
	return nil
}

// root is the buffer a term's value is a version of: a name in bufs, followed
// through stores, lets, a loop's carried initializer and a call's arguments (a
// borrow hands back what it took); "" when no buffer is found.
func root(e core.Expr, bufs []string) string {
	switch e := e.(type) {
	case *core.Var:
		if index(bufs, e.Name) >= 0 {
			return e.Name
		}
	case *core.Apply:
		for _, a := range e.Args {
			if r := root(a, bufs); r != "" {
				return r
			}
		}
	case *core.Let:
		return root(e.Body, bufs)
	case *core.Seq:
		return root(e.Xs[len(e.Xs)-1], bufs)
	case *core.If:
		return root(e.A, bufs)
	case *core.Loop:
		for j, init := range e.Inits {
			if r := root(init, bufs); r != "" {
				_ = j
				return r
			}
		}
	case *core.The:
		return root(e.E, bufs)
	}
	return ""
}

// bufferParams is a λ's parameters that are stored into, directly or as the
// initializer of a loop variable that is.
func bufferParams(lam *core.Lam) []string {
	stored := map[string]bool{}
	from := map[string]string{} // loop variable ↦ the name it is initialized with
	var walk func(t core.Tail)
	walk = func(t core.Tail) {
		visitTail(t, func(e core.Expr) {
			switch e := e.(type) {
			case *core.Apply:
				if f, ok := e.Fn.(*core.Var); ok && f.Name == "set" && len(e.Args) == 3 {
					if v, ok := e.Args[0].(*core.Var); ok {
						stored[v.Name] = true
					}
				}
			case *core.Loop:
				for j, init := range e.Inits {
					if v, ok := init.(*core.Var); ok {
						from[e.Vars[j]] = v.Name
					}
				}
			}
		})
	}
	walk(lam.Body)
	for changed := true; changed; {
		changed = false
		for v, n := range from {
			if stored[v] && !stored[n] {
				stored[n], changed = true, true
			}
		}
	}
	var out []string
	for _, p := range lam.Params {
		if stored[p] {
			out = append(out, p)
		}
	}
	return out
}

// resultShape says where a definition's value puts its buffers: alone, first
// in a tuple, or elsewhere, judged at the value's tails.
func resultShape(e core.Expr, ps []string) string {
	var shapes []string
	var tails func(t core.Tail)
	tails = func(t core.Tail) {
		switch t := t.(type) {
		case *core.Let:
			tails(t.Body)
		case *core.TLet:
			tails(t.Body)
		case *core.Seq:
			tails(t.Xs[len(t.Xs)-1])
		case *core.TSeq:
			tails(t.Last)
		case *core.If:
			tails(t.A)
			tails(t.B)
		case *core.Cond:
			for _, a := range t.Arms {
				tails(a.Body)
			}
		case *core.Local:
			tails(t.Body)
		case *core.Loop:
			for _, a := range t.Arms {
				if _, ok := a.Body.(*core.Again); !ok {
					tails(a.Body)
				}
			}
		case *core.Apply:
			if f, ok := t.Fn.(*core.Var); ok && f.Name == "tuple" {
				for i, a := range t.Args {
					if root(a, append(ps, loopVars(e)...)) != "" {
						if i == 0 {
							shapes = append(shapes, "a tuple, the buffer first")
						} else {
							shapes = append(shapes, "a tuple, the buffer not first")
						}
						return
					}
				}
				shapes = append(shapes, "a tuple without the buffer")
				return
			}
			if root(t, append(ps, loopVars(e)...)) != "" {
				shapes = append(shapes, "the buffer alone")
			} else {
				shapes = append(shapes, "no buffer")
			}
		case *core.Var:
			if root(t, append(ps, loopVars(e)...)) != "" {
				shapes = append(shapes, "the buffer alone")
			} else {
				shapes = append(shapes, "no buffer")
			}
		default:
			shapes = append(shapes, "another value")
		}
	}
	tails(e)
	sort.Strings(shapes)
	var uniq []string
	for i, s := range shapes {
		if i == 0 || s != shapes[i-1] {
			uniq = append(uniq, s)
		}
	}
	return "result " + strings.Join(uniq, " | ")
}

// loopVars is every loop variable in e: a carrier's version is a buffer too.
func loopVars(e core.Expr) []string {
	var out []string
	visitTail(e, func(x core.Expr) {
		if l, ok := x.(*core.Loop); ok {
			out = append(out, l.Vars...)
		}
	})
	return out
}

// visitTail calls f on every expression of a tree.
func visitTail(t core.Tail, f func(core.Expr)) {
	k := &walker{f: f}
	k.tail(t)
}

type walker struct{ f func(core.Expr) }

func (w *walker) tail(t core.Tail) {
	switch t := t.(type) {
	case *core.Again:
		w.exprs(t.Args)
	case *core.TLet:
		for _, b := range t.Binds {
			w.expr(b.Value)
		}
		w.tail(t.Body)
	case *core.TSeq:
		w.exprs(t.Xs)
		w.tail(t.Last)
	case *core.TMonad:
		w.exprs(t.Items)
		w.tail(t.Body)
	case core.Expr:
		w.expr(t)
	}
}

func (w *walker) exprs(es []core.Expr) {
	for _, e := range es {
		w.expr(e)
	}
}

func (w *walker) expr(e core.Expr) {
	if e == nil {
		return
	}
	w.f(e)
	switch e := e.(type) {
	case *core.Lam:
		w.expr(e.Body)
	case *core.Apply:
		w.expr(e.Fn)
		w.exprs(e.Args)
	case *core.Index:
		w.expr(e.Table)
		w.expr(e.At)
	case *core.Let:
		for _, b := range e.Binds {
			w.expr(b.Value)
		}
		w.expr(e.Body)
	case *core.Case:
		w.expr(e.Scrut)
		for _, a := range e.Arms {
			w.expr(a.Body)
		}
	case *core.Loop:
		w.exprs(e.Inits)
		for _, a := range e.Arms {
			w.expr(a.Guard)
			w.expr(a.When)
			w.tail(a.Body)
		}
	case *core.Local:
		for _, b := range e.Binds {
			w.expr(b.Init)
		}
		w.expr(e.Body)
	case *core.BuildMap:
		w.exprs(e.Caps)
		w.expr(e.Body)
	case *core.The:
		w.expr(e.E)
	case *core.If:
		w.expr(e.C)
		w.expr(e.A)
		w.expr(e.B)
	case *core.And:
		w.exprs(e.Xs)
	case *core.Or:
		w.exprs(e.Xs)
	case *core.Not:
		w.expr(e.X)
	case *core.Cond:
		for _, a := range e.Arms {
			w.expr(a.Guard)
			w.expr(a.Body)
		}
	case *core.Seq:
		w.exprs(e.Xs)
	case *core.Monad:
		w.exprs(e.Items)
		w.expr(e.Body)
	}
}

func main() {
	k := &counter{c: map[string]int{}, verbose: len(os.Args) > 1 && os.Args[1] == "-v"}
	for _, dir := range []string{"examples", "lib", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			ds, err := core.ReadTrees(string(b))
			if err != nil {
				return nil
			}
			k.file = p
			for _, d := range ds {
				switch d := d.(type) {
				case *core.DefDecl:
					if lam, ok := d.Value.(*core.Lam); ok {
						if ps := bufferParams(lam); len(ps) > 0 {
							k.c["definitions taking a buffer it stores into"]++
							k.c["  … with "+resultShape(lam.Body, ps)]++
							k.note(fmt.Sprintf("borrower %s: %s %v", k.file, d.Name, ps))
							k.expr(lam.Body, ps)
							continue
						}
					}
					k.expr(d.Value, nil)
				case *core.TermDecl:
					k.expr(d.E, nil)
				}
			}
			return nil
		})
	}
	var ks []string
	for key := range k.c {
		ks = append(ks, key)
	}
	sort.Strings(ks)
	for _, key := range ks {
		fmt.Printf("%6d  %s\n", k.c[key], key)
	}
	if k.verbose {
		fmt.Println("\nloops whose carriers S would move:")
		for _, e := range append(k.examples, append([]string{"", "back edges that move between places, or find no buffer:"}, k.moves...)...) {
			fmt.Println("  " + e)
		}
	}
}
