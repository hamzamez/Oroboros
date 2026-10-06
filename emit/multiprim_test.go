package emit

import (
	"testing"
)

// A HOST CALL MAY GIVE BACK SEVERAL RESULTS, and the language needed nothing.
//
// `Prim.Result` was one string, and that field refused 19.8% of Go's callable
// standard library — more than every language-level limitation put together,
// and compounding, because `(T, error)` is Go's CONSTRUCTOR idiom, so every
// name it blocked also blocked every method on the type that name would have
// returned (gostdlib-2026-09-06 §4a). It is why this language had no file I/O.
//
// The elimination form already existed: `((f x) (fn (a b) …))` is how the
// negative product is consumed (values.md), and `values-product.oro` has been a
// differential case since August. With a `def` producer, β performs that
// application and the product vanishes. With a `prim` producer β cannot — the
// operator of the outer application is itself an application — so the redex is
// stuck and the shape arrives at the backend, which emits the host's own form.
func TestAPrimMayGiveBackSeveralResults(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// The host's (bytes, error), now the raw call behind the fallible
	// declaration (retract.go): the host call with several results is it.
	p, ok := tg.Prims[RawName("go/os.ReadFile")]
	if !ok {
		t.Fatal("targets/go declares no raw go/os.ReadFile")
	}
	if len(p.Results) != 2 {
		t.Fatalf("ReadFile has %d declared results, want 2: %v", len(p.Results), p.Results)
	}
	// A single COMPOUND result must not be mistaken for several. `(array int)`
	// and `(int 0 255)` are both an application of names, and only `TypeName`
	// knows which constructors exist — which is why `resultList` consults it
	// first rather than counting kids.
	for _, n := range []string{"go/os.WriteFile", "go/os.Args"} {
		q := tg.Prims[n]
		if len(q.Results) != 0 {
			t.Errorf("%s: %q was read as %d results; a compound type is ONE result",
				n, q.Result, len(q.Results))
		}
	}
}
