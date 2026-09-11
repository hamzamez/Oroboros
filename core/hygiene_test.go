package core

import "testing"

// faithful reports the first place where opening a term by its hints — which
// is what every consumer of a residual does — would make a name mean something
// its index does not: a reference whose binder is shadowed, by hint, by a
// nearer binder, or a free name equal to an enclosing binder's hint.
//
// It resolves each variable TWICE and compares: once by index (the truth) and
// once by name (what `Body()` gives the checker and the backends). That is an
// independent statement of the property hygiene.go establishes, not a copy of
// its algorithm.
func faithful(t *Term) (string, bool) {
	var scope [][]string
	var walk func(t *Term) (string, bool)
	walk = func(t *Term) (string, bool) {
		switch t.Kind {
		case KBound:
			if t.Depth >= len(scope) {
				return "", true
			}
			target := len(scope) - 1 - t.Depth
			name := scope[target][t.Index]
			for lvl := len(scope) - 1; lvl > target; lvl-- {
				for _, p := range scope[lvl] {
					if p == name {
						return "the reference to " + name + " is captured by a nearer binder of the same hint", false
					}
				}
			}
		case KName:
			for _, ps := range scope {
				for _, p := range ps {
					if p == t.Name {
						return "the free name " + t.Name + " is captured by a binder of the same hint", false
					}
				}
			}
		case KFn:
			scope = append(scope, t.Params)
			why, ok := walk(t.Kids[0])
			scope = scope[:len(scope)-1]
			return why, ok
		case KApp:
			for _, k := range t.Kids {
				if why, ok := walk(k); !ok {
					return why, ok
				}
			}
		}
		return "", true
	}
	return walk(t)
}

// THE SHAPE β MAKES. A comparator `(fn (a b) (cmp a b))` applied to two table
// reads inside a scope whose own table is also hinted `a`: effects.md §7c
// let-binds both arguments, and the second value, `(cs (a 1))`, sits inside the
// first binder — correct by index, and captured the moment anything opens it
// by name. tally.oro's checker reported "b is int, but string is required".
func TestAResidualCanBeOpenedByItsHints(t *testing.T) {
	src, prims := splitPrims("(prim !cmp)\n(fn (cs a) ((fn (a b) (cmp a b)) (cs (a 0)) (cs (a 1))))", "")
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := Normalize(terms[0], testEnv(prog, prims...), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if why, ok := faithful(nf); !ok {
		t.Errorf("%s:\n%s", why, nf)
	}
}

// AND IT RENAMES NOTHING THAT DOES NOT NEED IT. A shadowing binder whose body
// never mentions the outer variable is harmless by name as well as by index, and
// renaming it would change emitted code for no reason — which is what keeps
// every program without a latent capture byte-identical.
func TestHygieneLeavesHarmlessShadowingAlone(t *testing.T) {
	in := Fn([]string{"i"}, App(Name("let"), App(Name("f"), Name("i")),
		Fn([]string{"i"}, App(Name("g"), Name("i")))))
	if out := hygienic(in); out.String() != in.String() {
		t.Errorf("renamed a binder that captured nothing:\n  in:  %s\n  out: %s", in, out)
	}
	if _, ok := faithful(in); !ok {
		t.Errorf("the control is itself unfaithful, so it controls nothing")
	}
}
