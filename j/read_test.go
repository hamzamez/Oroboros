package j

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
)

// key is a term's canonical spelling up to α: a λ's parameter names are hints
// and its body is closed over indices (core/term.go, Fn), so two terms with one
// key are one term. A power of literals, `(pow a b)`, is keyed as its value, the
// literal it denotes in a range (core/read.go reads it so).
func key(t *core.Term) string {
	switch t.Kind {
	case core.KName:
		return t.Name
	case core.KInt:
		return fmt.Sprint(t.Int)
	case core.KFloat:
		return fmt.Sprintf("f%x", math.Float64bits(t.Float))
	case core.KStr:
		return fmt.Sprintf("%q", t.Str)
	case core.KBool:
		if t.IsTrue() {
			return "true"
		}
		return "false"
	case core.KBound:
		return fmt.Sprintf("#%d.%d", t.Depth, t.Index)
	case core.KFn:
		return fmt.Sprintf("(fn/%d %s)", len(t.Params), key(t.Kids[0]))
	}
	if len(t.Kids) == 3 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == "pow" &&
		t.Kids[1].Kind == core.KInt && t.Kids[2].Kind == core.KInt {
		v := new(big.Int).Exp(big.NewInt(t.Kids[1].Int), big.NewInt(t.Kids[2].Int), nil)
		return key(core.IntLiteral(v.String()))
	}
	parts := make([]string, len(t.Kids))
	for i, k := range t.Kids {
		parts[i] = key(k)
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// sections splits declaration terms at each `(module P)` and keys each section
// as a multiset: within a module the order of declarations carries nothing
// (theories.md §3: a module is the glue of its forms), and the export list is
// a set.
//
// Two more laws are applied as decided: an import's alias is always present,
// filled with the path's last segment (ADR 0064 item 3); and a `sig` binds its
// own names, so they are keyed by position (α for the sig's binder).
func sections(ts []*core.Term) []string {
	var out []string
	var cur, exports []string
	flush := func() {
		if len(exports) > 0 {
			sort.Strings(exports)
			cur = append(cur, "(export "+strings.Join(exports, " ")+")")
		}
		sort.Strings(cur)
		out = append(out, strings.Join(cur, "\n"))
		cur, exports = nil, nil
	}
	for _, t := range ts {
		if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
			switch t.Kids[0].Name {
			case "module":
				flush()
			case "export":
				for _, k := range t.Kids[1:] {
					exports = append(exports, k.Name)
				}
				continue
			case "use":
				path := t.Kids[1].Name
				alias := path[strings.LastIndex(path, "/")+1:]
				if len(t.Kids) == 4 {
					alias = t.Kids[3].Name
				}
				cur = append(cur, "(use "+path+" as "+alias+")")
				continue
			case "sig":
				cur = append(cur, key(positional(t)))
				continue
			}
		}
		cur = append(cur, key(t))
	}
	flush()
	return out
}

// EVERY PROGRAM READS BACK TO ITS MEANING (abstract.md §9.1, R2; spec/j.md §7):
// for each program, J's reading of its translation desugars to the declaration
// terms today's reading does, up to α and the order of a module's forms. Since
// the compiler reads those terms and nothing else, every program compiled from J
// is compiled from what it was.
func TestEveryProgramReadsBackToItsMeaning(t *testing.T) {
	agree, total := 0, 0
	var failures []string
	for _, path := range corpus(t) {
		b, _ := os.ReadFile(path)
		src := string(b)
		want, err := core.ReadDeclTerms(src)
		if err != nil {
			continue
		}
		r, err := Translate(src, filepath.Dir(path), "..")
		if err != nil {
			continue // a fragment, step 2d's
		}
		total++
		decls, err := ReadProgram(r.J, ModuleOf(src), filepath.Dir(path), "..")
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: J does not read: %v", path, err))
			continue
		}
		got := core.DeclTerms(decls)
		ws, gs := sections(want), sections(got)
		if strings.Join(ws, "\n--\n") == strings.Join(gs, "\n--\n") {
			agree++
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: the D-images differ:\n%s", path, firstDiff(ws, gs)))
	}
	for i, f := range failures {
		if i < 8 || os.Getenv("J_ALL") != "" {
			t.Error(f)
		}
	}
	if total < 150 || agree != total {
		t.Fatalf("%d of %d programs read back to their meaning; every one must, over the corpus's 150 and more", agree, total)
	}
	t.Logf("%d of %d programs read back to their meaning", agree, total)
}

func firstDiff(ws, gs []string) string {
	if len(ws) != len(gs) {
		return fmt.Sprintf("  %d module sections, against %d", len(gs), len(ws))
	}
	for i := range ws {
		w, g := strings.Split(ws[i], "\n"), strings.Split(gs[i], "\n")
		for j := 0; j < len(w) || j < len(g); j++ {
			var a, b string
			if j < len(w) {
				a = w[j]
			}
			if j < len(g) {
				b = g[j]
			}
			if a != b {
				return fmt.Sprintf("  today: %.400s\n  J:     %.400s", a, b)
			}
		}
	}
	return "  (equal)"
}

// positional renames a sig's parameter names to their positions, $1, $2, …,
// wherever they occur in it.
func positional(sig *core.Term) *core.Term {
	m := map[string]string{}
	if len(sig.Kids) > 2 {
		for i, prm := range sig.Kids[2].Kids {
			if prm.Kind == core.KApp && len(prm.Kids) == 2 && prm.Kids[0].Kind == core.KName {
				m[prm.Kids[0].Name] = fmt.Sprintf("$%d", i+1)
			}
		}
	}
	var re func(t *core.Term) *core.Term
	re = func(t *core.Term) *core.Term {
		if t.Kind == core.KName {
			if n, ok := m[t.Name]; ok {
				return core.Name(n)
			}
			return t
		}
		if len(t.Kids) == 0 {
			return t
		}
		c := *t
		c.Kids = make([]*core.Term, len(t.Kids))
		for i, k := range t.Kids {
			c.Kids[i] = re(k)
		}
		return &c
	}
	return re(sig)
}
