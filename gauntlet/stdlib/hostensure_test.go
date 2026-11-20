package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A HOST'S QUANTIFIED PROMISE IS ITS HOST'S SENTENCE (ADR 0145's L5). A host
// declaration's `ensure` that is a formula of the logic is believed as `pure`
// is, an axiom of the host's theory that no compiler can verify, so its
// evidence is kept beside the declarations, as a callback's markers are
// (TestEveryCallbackMarkerIsItsHostsSentence). host-ensures-evidence.txt gives
// each such declaration the conjuncts of its `ensure`, each with the sentence
// that states it. Checked both ways:
//   - every conjunct of every host `ensure` holding a formula, on every target,
//     has a row;
//   - every row names a conjunct declared so.
//
// The faults: a conjunct with no row, and a row for a conjunct not declared.
func TestEveryQuantifiedHostPromiseIsItsHostsSentence(t *testing.T) {
	rows, err := ensureRows(filepath.Join(root, "gauntlet", "stdlib", "host-ensures-evidence.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var tgs []*emit.Target
	for _, d := range []string{"go", "js", "java", "windows", "windows32"} {
		tg, err := emit.LoadTarget(filepath.Join(root, "targets", d))
		if err != nil {
			t.Fatal(err)
		}
		tgs = append(tgs, tg)
	}
	declared := declaredPromises(tgs)
	if len(declared) == 0 {
		t.Fatal("no host declares a quantified `ensure`: the check reads nothing (GetWindowTextA's is the first)")
	}
	for _, e := range checkPromises(declared, rows) {
		t.Error(e)
	}
	// the faults
	if len(rows) < 2 || len(checkPromises(declared, rows[1:])) == 0 {
		t.Error("a conjunct with no row was accepted: the check cannot fail")
	}
	bad := append(append([]ensureRow(nil), rows...), ensureRow{rows[0].decl, "r == 7", rows[0].source, rows[0].sentence})
	if len(checkPromises(declared, bad)) == 0 {
		t.Error("a row for a conjunct no declaration states was accepted: the check cannot fail")
	}
}

type ensureRow struct{ decl, conjunct, source, sentence string }

func ensureRows(path string) ([]ensureRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []ensureRow
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("an evidence row has declaration, conjunct, source and sentence: %q", line)
		}
		if !strings.HasPrefix(f[2], "https://") {
			return nil, fmt.Errorf("%s: a source is a page's URL, not %q", f[0], f[2])
		}
		out = append(out, ensureRow{f[0], f[1], f[2], f[3]})
	}
	return out, nil
}

// declaredPromises are, of every host operation whose `ensure` holds a
// formula, the conjuncts of that `ensure` in J, keyed decl + NUL + conjunct.
// A conjunction is D's (if a b false); the result, which the reader renamed
// core.ResultName, is shown as r.
func declaredPromises(tgs []*emit.Target) map[string]bool {
	out := map[string]bool{}
	for _, tg := range tgs {
		for n, p := range tg.Prims {
			// a derived variant (`#seg`, ADR 0132) is the declaration's image
			// under the restriction, its evidence the declaration's
			if p.Ensures == nil || !core.HasFormula(p.Ensures) || strings.Contains(n, "#") {
				continue
			}
			// the result: `result`, or of a threaded call (a buffer written in
			// place, ADR 0070) its components, the one the host returns shown r
			sub := map[string]*core.Term{core.ResultName: core.Name("r")}
			for j := 0; j < 8; j++ {
				sub[core.ResultComponent(j)] = core.Name(fmt.Sprintf("r%d", j))
			}
			if len(p.Results) == len(p.InPlace)+1 {
				for j := range p.Results {
					if !inPlaceComponent(p, j) {
						sub[core.ResultComponent(j)] = core.Name("r")
					}
				}
			}
			e := core.Rename2(p.Ensures, sub)
			for _, c := range ensureConjuncts(e) {
				out[n+"\x00"+core.ShowTerm(c)] = true
			}
		}
	}
	return out
}

func ensureConjuncts(t *core.Term) []*core.Term {
	if t.Kind == core.KApp && len(t.Kids) == 4 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == "if" &&
		t.Kids[3].Kind == core.KBool && t.Kids[3].Int == 0 {
		return append(ensureConjuncts(t.Kids[1]), ensureConjuncts(t.Kids[2])...)
	}
	return []*core.Term{t}
}

func checkPromises(declared map[string]bool, rows []ensureRow) []string {
	var errs []string
	given := map[string]bool{}
	for _, r := range rows {
		k := r.decl + "\x00" + r.conjunct
		given[k] = true
		if !declared[k] {
			errs = append(errs, fmt.Sprintf("the evidence gives %s `%s`, and no host declares that conjunct in a quantified `ensure`", r.decl, r.conjunct))
		}
	}
	var keys []string
	for k := range declared {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !given[k] {
			d, c, _ := strings.Cut(k, "\x00")
			errs = append(errs, fmt.Sprintf("%s promises `%s`, and host-ensures-evidence.txt gives no sentence of its host stating it", d, c))
		}
	}
	return errs
}

func inPlaceComponent(p emit.Prim, j int) bool {
	for _, ip := range p.InPlace {
		if ip.Result == j {
			return true
		}
	}
	return false
}
