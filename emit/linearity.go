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
//
// A BUFFER IS LINEAR WHEREVER IT IS BOUND, and the walk is seeded by TYPE
// (host-buffers.md §5). It used to be seeded by exactly two binders — `build`'s
// and a declared `(buffer V)` parameter's — which agreed with the type while no
// primitive could return a buffer. One can: a host call that writes into a buffer
// and hands it back, `B ⊸ B ⊗ R`, is how a write-borrow is declared. The buffer
// it returns was never checked, so handing it to a second call and then reading
// it was accepted and observed the second write through a dead name. Every
// binder is now asked whether the value it binds is a buffer — a `let`'s, a
// loop variable's, a host call's continuation parameter — and checked if so.
func CheckLinear(t *core.Term, tgt *Target, sig *core.Sig) error {
	c := &linChecker{tgt: tgt}
	taken := map[string]bool{}
	env := map[string]bool{}
	// ADR 0020: A DECLARED BUFFER PARAMETER IS LINEAR THROUGH THE BODY, and this
	// is the whole implementation of that half — the SAME walk, seeded from the
	// signature instead of from `build`'s binder.
	//
	// The two obligations sit at opposite ends and are different properties
	// (uniqueness.md 3, after Marshall/Vollmer/Orchard, ESOP 2022). UNIQUENESS
	// IN — no other reference exists — is a promise the CALLER makes; at an
	// export the caller is outside the program by construction, so it is
	// ASSUMED, which is refinements.md 6b's middle row. LINEARITY THROUGH —
	// moved exactly once, read freely before — is a promise the BODY makes, and
	// it is checkable from the body, which is what happens here.
	//
	// Neither alone suffices: uniqueness without linearity lets the body read
	// after a store, and linearity without uniqueness lets the caller observe
	// the write.
	//
	// Nothing is needed for an INTERNAL definition. Delta inlines every
	// non-exported call before any check runs, so such a parameter survives no
	// boundary and its occurrences are already in the residual, where `scan`
	// decides them against the `build` that made the buffer.
	if t != nil && t.Kind == core.KFn {
		body, raw, _ := openFresh(t, taken, asmIdent)
		bufs := make([]bool, len(raw))
		for i, name := range raw {
			if sig == nil || i >= len(sig.Params) || !core.IsBuffer(sig.Params[i].Type) {
				continue
			}
			bufs[i] = true
			if err := c.walk(body, name, &linState{declared: true}); err != nil {
				return err
			}
		}
		defer bindBufs(env, raw, bufs)()
		return c.scan(body, env, taken)
	}
	return c.scan(t, env, taken)
}

type linChecker struct{ tgt *Target }

// bindBufs records which of a binder's names hold buffers for the scope of its
// body, and returns what puts the enclosing entries back.
func bindBufs(env map[string]bool, names []string, bufs []bool) func() {
	type prev struct{ v, ok bool }
	old := make([]prev, len(names))
	for i, n := range names {
		v, ok := env[n]
		old[i] = prev{v, ok}
		env[n] = bufs[i]
	}
	return func() {
		for i := len(names) - 1; i >= 0; i-- {
			if old[i].ok {
				env[names[i]] = old[i].v
			} else {
				delete(env, names[i])
			}
		}
	}
}

// scan opens every binder, and checks each one that holds a buffer.
//
// Binders are opened against one shared `taken` set, so every name in the opened
// term is distinct and a check for one buffer can never count a sibling's or a
// shadowing binder's occurrences as its own.
func (c *linChecker) scan(t *core.Term, env, taken map[string]bool) error {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		body, raw, _ := openFresh(t, taken, asmIdent)
		defer bindBufs(env, raw, make([]bool, len(raw)))()
		return c.scan(body, env, taken)
	}
	// A host call with several results: a continuation parameter declared a
	// buffer is the buffer the host handed back.
	if p, as, k, ok := multiPrimCall(c.tgt, t); ok {
		for _, a := range as {
			if err := c.scan(a, env, taken); err != nil {
				return err
			}
		}
		if err := c.bufferArgs(p, as, env, taken); err != nil {
			return err
		}
		body, raw, _ := openFresh(k, taken, asmIdent)
		bufs := make([]bool, len(raw))
		for i, name := range raw {
			if i < len(p.Results) && core.IsBuffer(p.Results[i]) {
				bufs[i] = true
				if err := c.walk(body, name, &linState{origin: "a host call that hands it back"}); err != nil {
					return err
				}
			}
		}
		defer bindBufs(env, raw, bufs)()
		return c.scan(body, env, taken)
	}
	switch {
	// A MAP BUFFER IS A BUFFER (maps.md §3.3, tables.md §2.5). Its binder is
	// checked by the same walk as `build`'s: no check looked at one before, and a
	// map used twice leaked one `insert` into a map that never received it.
	case (c.isKind(t, "table-build") || c.isKind(t, "map-build")) && len(t.Kids) == 3 &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1:
		if err := c.scan(t.Kids[1], env, taken); err != nil {
			return err
		}
		body, raw, _ := openFresh(t.Kids[2], taken, asmIdent)
		if err := c.walk(body, raw[0], &linState{}); err != nil {
			return err
		}
		defer bindBufs(env, raw, []bool{true})()
		return c.scan(body, env, taken)

	case c.isKind(t, "let") && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn &&
		len(t.Kids[2].Params) == 1:
		if err := c.scan(t.Kids[1], env, taken); err != nil {
			return err
		}
		buf := c.isBuf(t.Kids[1], env, taken)
		body, raw, _ := openFresh(t.Kids[2], taken, asmIdent)
		if buf {
			if err := c.walk(body, raw[0], &linState{origin: "a `let`"}); err != nil {
				return err
			}
		}
		defer bindBufs(env, raw, []bool{buf})()
		return c.scan(body, env, taken)

	case c.isKind(t, "iterate") && len(t.Kids) >= 3 && t.Kids[1].Kind == core.KFn &&
		len(t.Kids[1].Params) == len(t.Kids)-2:
		inits := t.Kids[2:]
		bufs := make([]bool, len(inits))
		for i, z := range inits {
			if err := c.scan(z, env, taken); err != nil {
				return err
			}
			bufs[i] = c.isBuf(z, env, taken)
		}
		body, raw, _ := openFresh(t.Kids[1], taken, asmIdent)
		for i, name := range raw {
			if bufs[i] {
				if err := c.walk(body, name, &linState{origin: "a loop"}); err != nil {
					return err
				}
			}
		}
		defer bindBufs(env, raw, bufs)()
		return c.scan(body, env, taken)
	}
	for _, k := range t.Kids {
		if err := c.scan(k, env, taken); err != nil {
			return err
		}
	}
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if p, ok := c.tgt.Prims[t.Kids[0].Name]; ok {
			if err := c.storeTarget(p, t.Kids[1:], env, taken); err != nil {
				return err
			}
			return c.bufferArgs(p, t.Kids[1:], env, taken)
		}
	}
	return nil
}

// storeTarget refuses a store into a value that is not a live buffer — S in
// tables.md §2.5.
//
// `set` and `insert` are the language's own writes, and nothing asked what they
// wrote INTO. A frozen table is immutable by ADR 0018, and every name that shares
// it assumes so: `(let t (build b 4 (set b 0 n)) (let t2 (set t 1 n) (t 1)))` wrote
// through `t2` and read the write back through `t`, whose element 1 is 0. That is
// a wrong answer on every target (prodresult-2026-09-25). It is bufferArgs' rule
// for a host call that may write, applied to the language's own stores; and it is
// the hypothesis of ADR 0031's no-copy freeze, "no store can name a frozen value".
func (c *linChecker) storeTarget(p Prim, args []*core.Term, env, taken map[string]bool) error {
	if (p.Kind != "table-set" && p.Kind != "map-insert") || len(args) == 0 || c.isBuf(args[0], env, taken) {
		return nil
	}
	what, scope := "table", "`(build b n …)`"
	if p.Kind == "map-insert" {
		what, scope = "map", "`(build-map m cap …)`"
	}
	return fmt.Errorf("%s stores into %s, which is a frozen %s, not a buffer.\n"+
		"  A store writes a LIVE buffer, one a scope such as %s is still filling. A frozen\n"+
		"  %s is immutable (ADR 0018), and every other name that shares it would see the\n"+
		"  write (tables.md §2.5, S). For a changed copy, build one: copy the %s into a new\n"+
		"  buffer inside a scope and store there.", p.Name, args[0], what, scope, what, what)
}

// bufferArgs refuses a value that is not a buffer where a primitive declares a
// buffer parameter.
//
// That is host-buffers.md §4's second hypothesis: a host call that may write its
// argument is declared at `(buffer V)`, and only a buffer may reach it. The type
// checker could not say this, because it gives `build` no result type — a frozen
// array handed to a write-borrow type-checked, built and ran (witness G), and the
// host would have written into a value this language promises nobody writes.
func (c *linChecker) bufferArgs(p Prim, args []*core.Term, env, taken map[string]bool) error {
	for i, a := range args {
		if i >= len(p.Args) || !core.IsBuffer(p.Args[i]) || c.isBuf(a, env, taken) {
			continue
		}
		return fmt.Errorf("argument %d of %s, %s, is not a buffer, and %s declares it `(%s)`.\n"+
			"  A parameter declared a buffer is one the host may WRITE, so it takes a linear\n"+
			"  buffer and nothing else: an immutable array handed to it would be written\n"+
			"  under every other name that shares it (ADR 0018, docs/host-buffers.md §4).\n"+
			"  Build the bytes inside `build`, or copy the array into one there first.",
			i+1, p.Name, a, p.Name, p.Args[i])
	}
	return nil
}

// isBuf reports whether a term's value is a buffer. It is the buffer half of
// the type, read where the type checker cannot help: `build` is structural and
// the checker has no case for it. Every producer of a buffer is here — a store,
// which hands its buffer back; a primitive whose declared result is a buffer; a
// name bound to one; and the forms that pass a value through. Answering NO wrongly
// leaves a buffer unchecked, which is the hole this exists to close; answering YES
// wrongly can only refuse a program, never accept a wrong one.
func (c *linChecker) isBuf(t *core.Term, env, taken map[string]bool) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName {
		return env[t.Name]
	}
	if t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := c.tgt.Prims[t.Kids[0].Name]
	if !ok {
		return false
	}
	switch p.Kind {
	case "table-set", "map-insert":
		return true
	case "cond":
		return len(t.Kids) == 4 && (c.isBuf(t.Kids[2], env, taken) || c.isBuf(t.Kids[3], env, taken))
	case "let":
		if len(t.Kids) != 3 || t.Kids[2].Kind != core.KFn || len(t.Kids[2].Params) != 1 {
			return false
		}
		body, raw, _ := openFresh(t.Kids[2], taken, asmIdent)
		defer bindBufs(env, raw, []bool{c.isBuf(t.Kids[1], env, taken)})()
		return c.isBuf(body, env, taken)
	case "iterate":
		if len(t.Kids) < 3 || t.Kids[1].Kind != core.KFn || len(t.Kids[1].Params) != len(t.Kids)-2 {
			return false
		}
		bufs := make([]bool, len(t.Kids)-2)
		for i, z := range t.Kids[2:] {
			bufs[i] = c.isBuf(z, env, taken)
		}
		body, raw, _ := openFresh(t.Kids[1], taken, asmIdent)
		defer bindBufs(env, raw, bufs)()
		return c.exitIsBuf(body, env, taken)
	}
	if core.IsBuffer(p.Result) {
		return true
	}
	// A statement's value is its first argument (target-files.md §3).
	return p.Kind == "stmt" && len(t.Kids) > 1 && c.isBuf(t.Kids[1], env, taken)
}

// exitIsBuf walks a loop's clause chain: an `again` is a jump and yields no value,
// every other leaf is the loop's value.
func (c *linChecker) exitIsBuf(t *core.Term, env, taken map[string]bool) bool {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		if t.Kids[0].Name == "again" {
			return false
		}
		if c.isKind(t, "cond") && len(t.Kids) == 4 {
			return c.exitIsBuf(t.Kids[2], env, taken) || c.exitIsBuf(t.Kids[3], env, taken)
		}
	}
	return c.isBuf(t, env, taken)
}

type linState struct {
	moved bool
	// declared distinguishes ADR 0020's parameter from ADR 0018's `build`
	// binder, so the diagnostic can name the right thing. The RULE is identical;
	// only where the buffer came from differs.
	declared bool
	// origin names any other binder: a `let`, a loop, or a host call's
	// continuation parameter.
	origin string
}

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
			return c.dead(name, "used", st)
		}
		st.moved = true
		return nil

	case t.Kind == core.KApp && len(t.Kids) == 2 &&
		t.Kids[0].Kind == core.KName && t.Kids[0].Name == name:
		// A READ — `(b i)`. Reads do not consume, which is what lets the sieve
		// test a cell and then keep going. The index is evaluated first.
		if st.moved {
			return c.dead(name, "read", st)
		}
		return c.walk(t.Kids[1], name, st)

	case (c.isKind(t, "table-set") || c.isKind(t, "map-insert")) && len(t.Kids) == 4 &&
		t.Kids[1].Kind == core.KName && t.Kids[1].Name == name:
		// A STORE consumes the buffer. Its index and value are evaluated
		// before the store happens, so they are walked first.
		if st.moved {
			return c.dead(name, "stored into", st)
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

	// `(len b)` and `(alloc b)` OBSERVE the buffer and do not hand it on, so a
	// bare name under them is not a move — which nothing needed to say while only
	// `build`'s binder was walked, because no program had taken the length of a
	// buffer and then read it again; the limb library's loops do.
	//
	// And `len` is allowed even AFTER a move, because a length cannot observe a
	// write: `set` passes its buffer's length through (LengthOf 1), and a call
	// that grows a buffer hands on a new one, leaving the old name's extent as it
	// was. What linearity forbids is seeing a store through a dead name, and a
	// length sees none. The Windows map library needs exactly this: it stores and
	// then indexes by `(len m)` in one expression (emit/winmap.oro). `alloc`
	// copies the contents, so it is an ordinary read and must come first.
	if c.isKind(t, "len") && len(t.Kids) == 2 && t.Kids[1].Kind == core.KName &&
		t.Kids[1].Name == name {
		return nil
	}
	if c.isKind(t, "table-alloc") && len(t.Kids) == 2 && t.Kids[1].Kind == core.KName &&
		t.Kids[1].Name == name {
		if st.moved {
			return c.dead(name, "read", st)
		}
		return nil
	}

	// `again`'s arguments are SIMULTANEOUS (ADR 0015): every one is evaluated
	// against the iteration's state and only then does the jump rebind the
	// variables. So a buffer passed through unchanged — `(again stk … (stk sp))`,
	// the tokeniser's shape — is handed on at the JUMP, after the read beside it,
	// not before. A store among the arguments is still evaluated where it stands,
	// so `(again (set stk sp x) … (stk sp))` is still a read after a move.
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName &&
		t.Kids[0].Name == "again" {
		passes := 0
		for _, a := range t.Kids[1:] {
			if a.Kind == core.KName && a.Name == name {
				passes++
				continue
			}
			if err := c.walk(a, name, st); err != nil {
				return err
			}
		}
		for i := 0; i < passes; i++ {
			if st.moved {
				return c.dead(name, "used", st)
			}
			st.moved = true
		}
		return nil
	}

	// `if`: the condition, then the branches INDEPENDENTLY. A buffer moved in
	// one arm is not moved in the other, and the join is conservative.
	if c.isKind(t, "cond") && len(t.Kids) == 4 {
		if err := c.walk(t.Kids[1], name, st); err != nil {
			return err
		}
		a := &linState{moved: st.moved, declared: st.declared, origin: st.origin}
		b := &linState{moved: st.moved, declared: st.declared, origin: st.origin}
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

func (c *linChecker) dead(name, how string, st *linState) error {
	where := "A buffer is linear (ADR 0018), and that is what lets `build`\n" +
		"  freeze it on the way out without copying: nothing else can be holding it."
	if st.origin != "" {
		where = "A buffer is linear wherever it is bound — this one by " + st.origin + " —\n" +
			"  and the storage it names may already have been written through the name\n" +
			"  it was handed to (docs/host-buffers.md)."
	}
	if st.declared {
		where = "A parameter declared `(buffer V)` is linear through the body\n" +
			"  (ADR 0020): the caller guarantees nobody else holds it, and in exchange\n" +
			"  the body must thread it and hand it back."
	}
	return fmt.Errorf("the buffer %s is %s after it has already been handed on.\n"+
		"  `(set b i v)` CONSUMES b and returns it, so the value to carry forward is\n"+
		"  the one `set` gave back — the old name is dead. %s\n"+
		"  Reading a buffer is fine and does not consume it; using it after a store\n"+
		"  or after passing it on is not.", name, how, where)
}
