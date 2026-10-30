package core

import "math"

// A SCOPED BUFFER IS A BINDER — docs/spec/tables.md §2.4, spec/local.md.
//
//	(build-map m cap body)              ≡  (build-map cap (fn (m) body))
//	(local b (table n 0) body)          ≡  (build n (fn (b) body))
//
// J's reader reads a block's state binders into trees (jparse.go), and D makes
// the core form (desugar.go). The λ of the core form is never a function
// value — it sits where a backend consumes it structurally (tables.md §14.1) —
// so it is a binding occurrence, and the source writes it as one, as `let`
// writes its redex as a binding (binding.md §1). The fold is on the right, so
// the scope is sequential: nᵢ sees b₁ … bᵢ₋₁.
//
// What this returns IS the core form, term for term: `Fn` over the body the
// reader already built, exactly what `(fn (b) body)` reads as. So nothing below
// the reader knows the binder form exists, and emission cannot change.

// LocalName is the scope of local state (spec/local.md).
const LocalName = "local"

// CellName heads a cell's scope until the loader translates it away: the core
// form `(#cell e (fn (x) body))`, a binder x holding e. `#` starts no
// identifier, so no program writes one.
const CellName = "#cell"

// PlaceName heads a buffer's scope in program order until the loader
// translates it away (ADR 0052, spec/local.md §2.1): `(#place b (fn (b) body))`
// inside build's region, a state binder b holding the region's table. It is a
// cell whose value is a table, written element by element.
const PlaceName = "#place"

// BorrowName heads a λ's body whose parameters are places, `(#borrow p…
// (fn (p…) body))`: the buffers a caller lends, written in program order, the
// λ's value the buffers' last versions first, then the body's (local.md §2.1).
const BorrowName = "#borrow"

// MapPlaceName heads a map buffer's scope in program order (ADR 0058,
// spec/local.md §2.1): a place whose value is a map, whose element write is
// `insert` where a table's is `set`.
const MapPlaceName = "#mapplace"

// localPlace is a program-order buffer's binder: build's region for the
// initializer, as localBinder makes it, with the region's binder a place.
func localPlace(x string, e, body *Term) *Term {
	inner := App(Name(PlaceName), Name(x), Fn([]string{x}, body))
	if headIsName(e, "table") || headIsName(e, "array") {
		return localBinder(x, e, inner)
	}
	// A BUFFER HOLDING ANOTHER TABLE'S VALUE: (adopt e (fn (x) …)), a fresh
	// region whose contents are e's, which the reducer moves in when e is a
	// fresh region and copies otherwise (reduce.go, adopt).
	return App(Name(AdoptName), e, Fn([]string{x}, inner))
}

func headIsName(t *Term, name string) bool {
	return t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == name
}

// AdoptName is the buffer former whose region is given, (adopt e (fn (b)
// body)): the state translation threads it as build's, and δ makes it one.
const AdoptName = "adopt"

// zeroSort reports a literal that is the zero of one sort, and the sort:
// `false` of bool, positive `0.0` of f64.
func zeroSort(f *Term) (string, bool) {
	switch {
	case f.Kind == KBool && f.Int == 0:
		return "bool", true
	case f.Kind == KFloat && f.Float == 0 && !math.Signbit(f.Float):
		return "f64", true
	}
	return "", false
}

// localBinder is D of one binder of a `local` around its body.
func localBinder(x string, e, body *Term) *Term {
	head := func(t *Term, name string) bool {
		return t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == name
	}
	rebind := func(fill *Term) *Term { // (let x fill body), inside the scope
		return App(Fn([]string{x}, body), fill)
	}
	switch {
	case head(e, "table") && len(e.Kids) == 3:
		n, f := e.Kids[1], e.Kids[2]
		// THE ZERO OF A TYPE is what a fresh buffer holds (tables.md §14.3), so
		// a table of it is build's own zero-filled form, with no fill loop.
		// `0` names the zero of every numeric and boolean element, as build's
		// buffers always took it, and leaves the element to the stores; `false`
		// and `0.0` name the zero of one sort, and that claim stays checked by
		// ascribing the buffer. -0.0 is not a fresh buffer's zero.
		if f.Kind == KInt && f.Int == 0 {
			return App(Name("build"), n, Fn([]string{x}, body)) // zero-filled: build's own form
		}
		if elem, ok := zeroSort(f); ok {
			return App(Name("build"), n, Fn([]string{x}, rebind(App(Name(AscribeName), Str("buffer "+elem), Name(x)))))
		}
		// A FILL LOOP: element i is (f i), or v for a literal v (the constant
		// table K v). The size is bound once, since the loop reads it again.
		var elem *Term
		switch f.Kind {
		case KInt, KFloat, KStr, KBool:
			elem = f
		default:
			elem = App(f, Name("#i"))
		}
		loop := App(Name("loop"),
			Fn([]string{"#i", x}, App(Name("if"), App(Name(">="), Name("#i"), Name("#n")),
				Name(x),
				App(Name("again"), App(Name("+"), Name("#i"), Int(1)),
					App(Name("set"), Name(x), Name("#i"), elem)))),
			Int(0), Name(x))
		return App(Fn([]string{"#n"}, App(Name("build"), Name("#n"), Fn([]string{x}, rebind(loop)))), n)
	case head(e, "array"):
		elems := e.Kids[1:]
		var fill *Term = Name(x)
		for i, el := range elems {
			fill = App(Name("set"), fill, Int(int64(i)), el)
		}
		inner := body
		if len(elems) > 0 {
			inner = rebind(fill)
		}
		return App(Name("build"), Int(int64(len(elems))), Fn([]string{x}, inner))
	}
	return App(Name(CellName), e, Fn([]string{x}, body))
}
