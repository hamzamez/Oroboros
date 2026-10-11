package core

import "fmt"

// THE BINDING SURFACE — docs/spec/binding.md.
//
//	(let LHS VALUE  LHS VALUE  …  BODY)
//
// with LHS a name, `_`, or a tuple pattern `(tuple a b …)`. Nothing here
// survives the reader: a name binds by an application of a λ, which is β's own
// redex, and a tuple pattern binds by applying the VALUE to a continuation,
// which is the product's eliminator (data.md §3.2). So the flat form costs the
// reducer, the checker and every backend exactly nothing.
//
// It is n-ary because nesting bindings is ASSOCIATIVE, and `(let b)` is legal
// because the body is nesting's IDENTITY — the same law that forces `(and)` to
// be `true` rather than choosing it (binding.md §4).
//
// μ reads the form into a Let (mu.go, muLet and letPat), and D folds it back
// into those redexes (desugar.go, let).

// discards respells a binder written `_` as an unwritable one, `#_`, the name
// `seq` binds (binding.md §6): `_` binds nothing, so no `_` written later, a
// pattern in a `case` or `try` that Load expands, may be captured by it. Two
// discards in one tuple pattern get two names.
func discards(ps []string) []string {
	out := ps
	n := 0
	for i, p := range ps {
		if p != "_" {
			continue
		}
		if &out[0] == &ps[0] {
			out = append([]string(nil), ps...)
		}
		out[i] = "#_"
		if n > 0 {
			out[i] = fmt.Sprintf("#_%d", n)
		}
		n++
	}
	return out
}

// patternNames recognises a tuple pattern on its image: μ checks a let's
// left-hand side on the term today's reader made of it, so `(tuple a b)`
// arrives as `(fn (#k) (#k a b))`.
//
// `#k` is unwritable — `#` is not isIdentStart — so no source term can be
// mistaken for a pattern, and a λ the programmer wrote has a different binder
// name and is refused as a left-hand side.
func patternNames(t *Term) ([]*Term, bool) {
	if t.Kind != KFn || len(t.Params) != 1 || t.Params[0] != "#k" {
		return nil, false
	}
	b := t.Closed()
	if b.Kind != KApp || len(b.Kids) < 3 || b.Kids[0].Kind != KBound {
		return nil, false
	}
	return b.Kids[1:], true
}
