package core

import (
	"reflect"
	"strings"
	"testing"
)

func declsOf(t *testing.T, src string) []Decl {
	t.Helper()
	_, decls, _, err := readModule(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return carryClaims(decls)
}

// A `sig` IS ITS DEFINITION'S CLAIM (ADR 0051, D1–D2): a definition is a named
// λ, and the λ carries each parameter's set, the result's set, the `where` and
// the `ensures`. Today's `sig` binds its own names, which are α-equivalent to
// the λ's, and is written where it stands; the claim keeps both, for D.
func TestASigIsItsDefinitionsClaim(t *testing.T) {
	ds := declsOf(t, `(sig f ((n (int 0 9)) (m int)) int (where (> n 0)) (ensures (> result 0)))
(def f (x y) (+ x y))`)
	if len(ds) != 1 {
		t.Fatalf("one declaration, the definition carrying its claim; got %d", len(ds))
	}
	def := ds[0].(*DefDecl)
	l := def.Value.(*Lam)
	c := l.Claim
	switch {
	case c == nil:
		t.Fatal("the λ carries no claim")
	case len(c.Params) != 2 || c.Params[0].Name != "n" || c.Params[1].Name != "m":
		t.Errorf("the claim's binders: %+v", c.Params)
	case l.Params[0] != "x" || l.Params[1] != "y":
		t.Errorf("the λ's binders are its own: %v", l.Params)
	case c.Pre == nil || c.Post == nil || c.Result == nil:
		t.Errorf("pre, post and result: %+v", c)
	case c.At != 0 || def.At != 1:
		t.Errorf("written at %d and %d", c.At, def.At)
	}
	// A constant takes no claim; two claims, or two definitions, or a λ of
	// another arity, leave the claim where Load decides it.
	for _, src := range []string{
		`(sig f () int) (def f 3)`,
		`(sig f ((n int)) int) (sig f ((n int)) int) (def f (x) x)`,
		`(sig f ((n int)) int) (def f (x) x) (def f (x) x)`,
		`(sig f ((n int)) int) (def f (x y) x)`,
		`(sig g ((n int)) int) (def f (x) x)`,
	} {
		apart, carried := 0, false
		for _, d := range declsOf(t, src) {
			switch d := d.(type) {
			case *ClaimDecl:
				apart++
			case *DefDecl:
				if l, ok := d.Value.(*Lam); ok && l.Claim != nil {
					carried = true
				}
			}
		}
		if carried || apart != strings.Count(src, "(sig ") {
			t.Errorf("%s: every claim should stay apart; %d did, and a λ carries one: %v", src, apart, carried)
		}
	}
}

// THEOREM D3 ON THE SHAPES THE CORPUS LACKS: positional parameters, a claim
// after its definition, a variant whose markers come first, an alias, a target
// fragment, a top-level term, and every declaration form together.
func TestDeclarationTreesGiveTodaysFormsOnEveryShape(t *testing.T) {
	for _, src := range []string{
		`(sig f (int (array f64)) int) (def f (a b) a)`,
		`(sig f ((n int) (xs (array f64))) (tuple int int)) (def f (fn (n xs) (tuple n n)))`,
		`(def g (x) x) (sig g ((n (int 0 9))) (int 0 9) (ensures (>= result 0)) (where (> n 1)))`,
		`(module m) (use go/strings as s) (use go/fmt) (export f g) (def f 3) (def g (x) (s.f x))`,
		`(variant (result T E) (success ok) (relevant) (ok T) (err E))`,
		`(variant colour red green blue)`,
		`(provides go m (sig h () int (host expr "1"))) (def k () 1)`,
		`(f 1 2)`,
		`(sig main () int) (def main () (let x 1 (+ x 1)))`,
	} {
		r, decls, read, err := readModule(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got, err := r.d.forms(carryClaims(decls))
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if !reflect.DeepEqual(formsText(got), formsText(read)) {
			t.Errorf("%s: the trees give\n%v\nand today's reader made\n%v", src, formsText(got), formsText(read))
		}
	}
	ds := declsOf(t, `(sig f (int (array f64)) int) (def f (a b) a)`)
	c := ds[0].(*DefDecl).Value.(*Lam).Claim
	if c == nil || c.Params[0].Name != "" || c.Params[1].Name != "" {
		t.Errorf("positional parameters keep no name: %+v", c)
	}
}

// D GIVES THE FORMS BACK IN THE ORDER WRITTEN: a claim where its `sig` stood,
// whichever comes first.
func TestAClaimIsWrittenWhereItStood(t *testing.T) {
	forms, err := Read(`(def f (x) x) (export f) (sig f ((n int)) int)`)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, f := range forms {
		kinds = append(kinds, f.Kind)
	}
	if strings.Join(kinds, " ") != "def export sig" {
		t.Errorf("forms in the order %v", kinds)
	}
}

// formsText renders forms with every term printed, so two forms compare by
// what they say, not by which term pointers they share.
func formsText(fs []Form) []string {
	var out []string
	for _, f := range fs {
		s := f.Kind + " " + f.Name + " " + f.Alias + " " + strings.Join(f.Names, ",")
		if f.Term != nil {
			s += " " + f.Term.String()
		}
		if f.Sig != nil {
			s += " " + sigText(f.Sig)
		}
		if f.Sum != nil {
			s += " " + strings.Join(f.Sum.Params, ",") + " " + f.Sum.Success
			for _, v := range f.Sum.Variants {
				s += " " + v.Name + ":" + v.Payload
			}
			if f.Sum.Relevant {
				s += " relevant"
			}
		}
		out = append(out, s)
	}
	return out
}

func sigText(s *Sig) string {
	t := ""
	for _, p := range s.Params {
		t += p.Name + ":" + p.Type + " "
	}
	t += "-> " + s.Result + " " + strings.Join(s.Results, ",")
	for _, x := range []*Term{s.Where, s.Written, s.Ensures, s.ResultRange} {
		if x != nil {
			t += " " + x.String()
		} else {
			t += " -"
		}
	}
	return t
}
