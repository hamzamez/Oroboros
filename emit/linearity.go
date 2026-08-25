package emit

import (
	"fmt"

	"oroboros/core"
)

// CheckLinear enforces ADR 0018's linearity on a residual.
//
// **A buffer is linear and scoped.** `(set b i v)` CONSUMES `b` and returns it,
// so after a store the old name is dead and the returned one must be used. That
// is what makes the freeze at `build`'s boundary copy nothing: linearity
// guarantees nothing else holds the buffer.
//
// The check is `occurrences` on the residual, **not a type** — uniqueness never
// enters a signature, which is the whole reason ADR 0018 was affordable.
//
// Reads do NOT consume. `(b i)` inside the scope is exactly what the sieve does
// and is fine; what is forbidden is any use of `b` *after* it has been moved.
// So this is an ordering check, not a counting one, and it walks in EVALUATION
// ORDER: a `let`'s value before its body, an `if`'s condition before its
// branches, arguments before the operation that consumes them.
//
// Branches are independent — a buffer moved in one arm of an `if` is not moved
// in the other — so the state forks and rejoins conservatively.
func CheckLinear(t *core.Term, tgt *Target) error {
	return (&linChecker{tgt: tgt}).scan(t)
}

type linChecker struct{ tgt *Target }

// scan finds every `build` and checks its buffer.
func (c *linChecker) scan(t *core.Term) error {
	if t == nil {
		return nil
	}
	if c.isKind(t, "table-build") && len(t.Kids) == 3 {
		lam := t.Kids[2]
		if lam.Kind == core.KFn && len(lam.Params) == 1 {
			body, raw, _ := openFresh(lam, map[string]bool{}, asmIdent)
			st := &linState{}
			if err := c.walk(body, raw[0], st); err != nil {
				return err
			}
			if err := c.scan(body); err != nil {
				return err
			}
			return c.scan(t.Kids[1])
		}
	}
	for _, k := range t.Kids {
		if err := c.scan(k); err != nil {
			return err
		}
	}
	return nil
}

type linState struct{ moved bool }

func (c *linChecker) isKind(t *core.Term, kind string) bool {
	if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := c.tgt.Prims[t.Kids[0].Name]
	return ok && p.Kind == kind
}

// walk is the ordering check. `name` is the buffer; `st.moved` says whether it
// has already been handed on.
func (c *linChecker) walk(t *core.Term, name string, st *linState) error {
	if t == nil {
		return nil
	}
	switch {
	case t.Kind == core.KName && t.Name == name:
		// A bare occurrence is a MOVE: the buffer is being handed on, as an
		// `again` argument, a loop's initial value, or the body's own result.
		if st.moved {
			return c.dead(name, "used")
		}
		st.moved = true
		return nil

	case t.Kind == core.KApp && len(t.Kids) == 2 &&
		t.Kids[0].Kind == core.KName && t.Kids[0].Name == name:
		// A READ — `(b i)`. Reads do not consume, which is what lets the sieve
		// test a cell and then keep going. The index is evaluated first.
		if st.moved {
			return c.dead(name, "read")
		}
		return c.walk(t.Kids[1], name, st)

	case c.isKind(t, "table-set") && len(t.Kids) == 4 &&
		t.Kids[1].Kind == core.KName && t.Kids[1].Name == name:
		// A STORE consumes the buffer. Its index and value are evaluated
		// before the store happens, so they are walked first.
		if st.moved {
			return c.dead(name, "stored into")
		}
		if err := c.walk(t.Kids[2], name, st); err != nil {
			return err
		}
		if err := c.walk(t.Kids[3], name, st); err != nil {
			return err
		}
		st.moved = true
		return nil
	}

	// `if`: the condition, then the branches INDEPENDENTLY. A buffer moved in
	// one arm is not moved in the other, and the join is conservative.
	if c.isKind(t, "cond") && len(t.Kids) == 4 {
		if err := c.walk(t.Kids[1], name, st); err != nil {
			return err
		}
		a, b := &linState{moved: st.moved}, &linState{moved: st.moved}
		if err := c.walk(t.Kids[2], name, a); err != nil {
			return err
		}
		if err := c.walk(t.Kids[3], name, b); err != nil {
			return err
		}
		st.moved = a.moved || b.moved
		return nil
	}

	// A `loop` binds its own copy of everything, so the buffer reaching it does
	// so through the initial values; the body is checked under the loop's name
	// by the enclosing scan.
	if t.Kind == core.KFn {
		// `Closed()`, NOT `Body()`. `Body()` opens the body using the
		// parameter-name HINTS, so a nested binder that happens to reuse the
		// buffer's name — which the sieve does, threading `c` as a loop
		// variable — turns its own occurrences into KName("c") and every one
		// of them looks like a use of the outer buffer. `Closed()` leaves
		// inner binders as KBound, so only genuinely free occurrences of this
		// buffer match. The same shadowing hazard `match` hit from the other
		// direction.
		return c.walk(t.Closed(), name, st)
	}
	for _, k := range t.Kids {
		if err := c.walk(k, name, st); err != nil {
			return err
		}
	}
	return nil
}

func (c *linChecker) dead(name, how string) error {
	return fmt.Errorf("the buffer %s is %s after it has already been handed on.\n"+
		"  `(set b i v)` CONSUMES b and returns it, so the value to carry forward is\n"+
		"  the one `set` gave back — the old name is dead. A buffer is linear\n"+
		"  (ADR 0018), and that is what lets `build` freeze it on the way out\n"+
		"  without copying: nothing else can be holding it.\n"+
		"  Reading a buffer is fine and does not consume it; using it after a store\n"+
		"  or after passing it on is not.", name, how)
}
