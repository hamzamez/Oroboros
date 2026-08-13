package legibility

// SROA written as a hand-written compiler pass — the conventional way.
//
// Written idiomatically and not strawmanned: this is roughly what the pass would
// look like in a real compiler, including the parallel-assignment care that
// g2 §6 requires.

// SROAPass scalarizes struct-typed locals into one local per field.
func SROAPass(f *Fn) *Fn {
	out := &Fn{}

	// Pass 1: find struct-typed locals. A local is a candidate if it is
	// declared with a struct initialiser.
	arity := map[string]int{}
	for _, n := range f.Nodes {
		if n.Kind == KVar && f.Nodes[n.Kids[0]].Kind == KStruct {
			arity[n.Str] = len(f.Nodes[n.Kids[0]].Kids)
		}
	}

	// Pass 2: rewrite, bottom-up, carrying a mapping from old node index to new.
	memo := map[int]int{}
	var walk func(i int) int
	walk = func(i int) int {
		if m, ok := memo[i]; ok {
			return m
		}
		n := f.Nodes[i]
		var res int
		switch n.Kind {

		case KVar:
			if k, ok := arity[n.Str]; ok {
				init := f.Nodes[n.Kids[0]]
				kids := make([]int, k)
				for j := 0; j < k; j++ {
					kids[j] = out.Var(fieldName(n.Str, j), walk(init.Kids[j]))
				}
				res = out.Seq(kids...)
			} else {
				res = out.Var(n.Str, walk(n.Kids[0]))
			}

		case KSet:
			k, ok := arity[n.Str]
			if !ok {
				res = out.Set(n.Str, walk(n.Kids[0]))
				break
			}
			val := f.Nodes[n.Kids[0]]
			if val.Kind != KStruct {
				// Whole-struct assignment from something other than a literal
				// constructor: copy field by field.
				kids := make([]int, k)
				for j := 0; j < k; j++ {
					kids[j] = out.Set(fieldName(n.Str, j), walk(f.Field(j, n.Kids[0])))
				}
				res = out.Par(kids...)
				break
			}
			// g2 §6: if any field on the right reads the variable being
			// assigned, the assignment is simultaneous and sequencing it is
			// wrong. Emit KPar so the backend introduces temporaries.
			simultaneous := false
			for _, kid := range val.Kids {
				if readsVar(f, kid, n.Str) {
					simultaneous = true
					break
				}
			}
			kids := make([]int, k)
			for j := 0; j < k; j++ {
				kids[j] = out.Set(fieldName(n.Str, j), walk(val.Kids[j]))
			}
			if simultaneous {
				res = out.Par(kids...)
			} else {
				res = out.Seq(kids...)
			}

		case KField:
			inner := f.Nodes[n.Kids[0]]
			switch {
			case inner.Kind == KStruct:
				// field-of-constructor folds away with no analysis at all
				res = walk(inner.Kids[n.I])
			case inner.Kind == KRef:
				if _, ok := arity[inner.Str]; ok {
					res = out.Ref(fieldName(inner.Str, n.I))
				} else {
					res = out.Field(n.I, walk(n.Kids[0]))
				}
			default:
				res = out.Field(n.I, walk(n.Kids[0]))
			}

		default:
			kids := make([]int, len(n.Kids))
			for j, k := range n.Kids {
				kids[j] = walk(k)
			}
			res = out.add(Node{Kind: n.Kind, Str: n.Str, F64: n.F64, I: n.I, Kids: kids})
		}
		memo[i] = res
		return res
	}

	out.Root = walk(f.Root)
	return out
}

func readsVar(f *Fn, i int, name string) bool {
	n := f.Nodes[i]
	if n.Kind == KRef && n.Str == name {
		return true
	}
	for _, k := range n.Kids {
		if readsVar(f, k, name) {
			return true
		}
	}
	return false
}

func fieldName(v string, i int) string {
	return v + "#" + string(rune('0'+i))
}
