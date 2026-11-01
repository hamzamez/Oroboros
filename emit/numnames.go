package emit

import "oroboros/core"

// THE NAMES OF SORT N IN A RESIDUAL (ADR 0086), which the refinement layer's
// fragment, linear arithmetic over ℤ, must not reason about: over N, `x < 10`
// does not give x ≤ 9, 2x = 1 is no contradiction, and `x ≠ y` is not
// x ≤ y − 1 ∨ x ≥ y + 1. A fact about one is assumed as nothing and a goal
// about one is proven by nothing (linear.go), which is sound and costs only
// what the IR's numerators prove instead (ir/numbers.go).
//
// The set is an over-approximation, which is the safe side: the checker's own
// inference of every binder of sort N, joined with a syntactic closure, a name
// bound (by a λ applied, a let, or a loop's entry or back edge) to a term that
// is a numeral, a rounding onto a set outside ℤ, a real quotient, or N's
// arithmetic on a name already in the set.
func NumNames(tg *Target, sig *core.Sig, t *core.Term) map[string]bool {
	// A RESIDUAL WITH NO NUMBER OF N HAS NO NAME OF SORT N: a value of N is
	// made only by a numeral, a rounding, a quotient, or a set of N declared
	// on a parameter or written in an ascription or a claim.
	if !mentionsN(tg, sig, t) {
		return map[string]bool{}
	}
	c := &checker{tgt: tg, types: map[string]string{}, nums: map[string]bool{}}
	// A PARAMETER'S SET IS ITS HEADER'S, which the body need not repeat.
	if t != nil && t.Kind == core.KFn && sig != nil {
		var tys []string
		for i := range t.Params {
			ty := ""
			if i < len(sig.Params) {
				ty = sig.Params[i].Type
			}
			tys = append(tys, ty)
		}
		c.bind(t.Params, tys)
	}
	// The checker is written for a whole residual; on any other term (a
	// `where` alone) its half of the set is what it found before it stopped,
	// and the syntactic closure below still runs.
	func() {
		defer func() { _ = recover() }()
		_, _ = c.walk(t, "")
		c.maybe = nil
		_, _ = c.walk(t, "")
	}()
	s := c.nums
	for pass := 0; pass < 8; pass++ {
		before := len(s)
		closeNums(tg, t, s)
		if len(s) == before {
			break
		}
	}
	return s
}

// isNSorted reports a type whose values have sort N: N's, a decimal grid's, or
// a double's, an element of F64 ⊂ N (ADR 0086). Over any of them `x > 9` does
// not give x ≥ 10.
func isNSorted(tg *Target, ty string) bool {
	v := tg.ValueType(ty)
	return ty == core.NumType || v == core.NumType || v == core.F64Name
}

// Unscope is t with each rounding the scope wrote of an integer term replaced
// by that term, the identity on ℤ (numbers.md §3), so ℤ's reasoning reads ℤ's
// own operations: `i + 1` inside round(F64) { … } is the integer it was. It
// strips only an operand intTerm proves an integer; any other is left, and the
// IR's typing decides it (ir/numbers.go, unscope), so a wrong guess here can
// only leave a rounding in place.
func Unscope(tg *Target, sig *core.Sig, t *core.Term) *core.Term {
	if !hasScoped(t) {
		return t
	}
	nums := NumNames(tg, sig, t)
	var walk func(t *core.Term) *core.Term
	walk = func(t *core.Term) *core.Term {
		switch t.Kind {
		case core.KFn:
			return core.Fn(t.Params, walk(t.Body()))
		case core.KApp:
			if a := t.Args(); isScoped(t) && intTerm(tg, a[1], nums) {
				return walk(a[1])
			}
			n := *t
			n.Kids = make([]*core.Term, len(t.Kids))
			for i, k := range t.Kids {
				n.Kids[i] = walk(k)
			}
			return &n
		}
		return t
	}
	return walk(t)
}

func isScoped(t *core.Term) bool {
	return t.Kind == core.KApp && len(t.Kids) == 5 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == core.RoundName &&
		t.Kids[4].Kind == core.KStr && t.Kids[4].Str == core.RoundScoped
}

func hasScoped(t *core.Term) bool {
	if t == nil {
		return false
	}
	if isScoped(t) {
		return true
	}
	for _, k := range t.Kids {
		if hasScoped(k) {
			return true
		}
	}
	return false
}

// intTerm reports a term proven an integer: an integer literal, a name not of
// sort N, ℤ's operations on such terms, a call of a primitive whose result is
// an integer, or a rounding the scope wrote of one. A read of a table is not
// one, since its element may be a double.
func intTerm(tg *Target, t *core.Term, nums map[string]bool) bool {
	switch t.Kind {
	case core.KInt:
		return true
	case core.KName:
		return !nums[t.Name]
	case core.KApp:
		if isScoped(t) {
			return intTerm(tg, t.Args()[1], nums)
		}
		op := t.Op()
		if op.Kind != core.KName {
			return false
		}
		p, prim := tg.Prims[op.Name]
		switch {
		case op.Name == "+" || op.Name == "-" || op.Name == "*" || op.Name == "/" || op.Name == "%":
		case prim && p.Pure && (p.Result == "int" || tg.ValueType(p.Result) == "int"):
		default:
			return false
		}
		for _, a := range t.Args() {
			if !intTerm(tg, a, nums) {
				return false
			}
		}
		return true
	}
	return false
}

// numTerm reports a term whose value has sort N, as far as the set s says.
func numTerm(tg *Target, t *core.Term, s map[string]bool) bool {
	switch t.Kind {
	case core.KDec:
		return true
	case core.KName:
		return s[t.Name]
	case core.KFn:
		return numTerm(tg, t.Body(), s)
	case core.KApp:
		if len(t.Kids) == 0 {
			return false
		}
		op := t.Op()
		if op.Kind == core.KFn {
			return numTerm(tg, op.Body(), s)
		}
		if op.Kind != core.KName {
			return false
		}
		args := t.Args()
		switch op.Name {
		case core.QuoName:
			return true
		case core.RoundName:
			if isScoped(t) {
				return !intTerm(tg, args[1], s)
			}
			if len(args) == 3 && args[0].Kind == core.KStr {
				set := args[0].Str
				return !(set == "int" || tg.ValueType(set) == "int" || core.UnboundedRange(set))
			}
			return true
		case "+", "-", "*", "if", "let":
			for _, a := range args {
				if numTerm(tg, a, s) {
					return true
				}
			}
		case core.AscribeName, core.RequireName:
			for _, a := range args {
				if a.Kind == core.KStr && tg.ValueType(a.Str) == core.NumType {
					return true
				}
			}
			return numTerm(tg, args[len(args)-1], s)
		}
	}
	return false
}

// closeNums adds to s every name the bindings in t give a term of sort N.
func closeNums(tg *Target, t *core.Term, s map[string]bool) {
	if t == nil {
		return
	}
	bindAll := func(params []string, vals []*core.Term) {
		for i, p := range params {
			if i < len(vals) && numTerm(tg, vals[i], s) {
				s[p] = true
			}
		}
	}
	if t.Kind == core.KApp && len(t.Kids) > 0 {
		op, args := t.Op(), t.Args()
		switch {
		case op.Kind == core.KFn:
			bindAll(op.Params, args)
		case op.Kind == core.KName && op.Name == "let" && len(args) == 2 && args[1].Kind == core.KFn:
			bindAll(args[1].Params, args[:1])
		case op.Kind == core.KName && (op.Name == "loop" || op.Name == "iterate") && len(args) >= 2 && args[0].Kind == core.KFn:
			lam := args[0]
			bindAll(lam.Params, args[1:])
			for _, a := range agains(lam.Body()) {
				bindAll(lam.Params, a)
			}
		}
	}
	if t.Kind == core.KFn {
		closeNums(tg, t.Body(), s)
		return
	}
	for _, k := range t.Kids {
		closeNums(tg, k, s)
	}
}

// agains are the argument lists of the `again`s of one loop body, not of the
// loops inside it.
func agains(t *core.Term) [][]*core.Term {
	var out [][]*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KApp && len(x.Kids) > 0 && x.Op().Kind == core.KName {
			switch x.Op().Name {
			case "again":
				out = append(out, x.Args())
			case "loop", "iterate":
				return
			}
		}
		if x.Kind == core.KFn {
			walk(x.Body())
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return out
}

// mentionsN reports whether a residual or its header can make a value of N.
func mentionsN(tg *Target, sig *core.Sig, t *core.Term) bool {
	if sig != nil {
		for _, p := range sig.Params {
			if isNSorted(tg, p.Type) {
				return true
			}
		}
	}
	found := false
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if found || x == nil {
			return
		}
		switch x.Kind {
		case core.KDec:
			found = true
			return
		case core.KName:
			found = x.Name == core.RoundName || x.Name == core.QuoName
			return
		case core.KStr:
			found = isNSorted(tg, x.Str)
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	return found
}
