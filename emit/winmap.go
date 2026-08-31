package emit

import (
	_ "embed"
	"fmt"
	"strings"

	"oroboros/core"
)

// LOWERING A MAP ONTO A TARGET THAT HAS NONE.
//
// Three hosts bring a hash map and we parasitize it. windows brings none, and a
// target does not get to decline a language construct — so `build-map`,
// `insert` and a map read are rewritten here into the ordinary buffer, loop and
// conditional that `winmap.oro` is written in.
//
// It is a SOURCE-LEVEL rewrite, done before reduction, and that is what makes
// it cost nothing downstream: the generated calls are inlined by δ and β like
// any others, the refinement layer sees ordinary indexing, the interval layer
// sees ordinary arithmetic, and the x86 backend sees a program it already knew
// how to emit. Nothing below this file learns that maps exist.
//
// The alternative — teaching `emit/asm.go` three new structural kinds — is the
// mistake CLAUDE.md names: a hash table is a LIBRARY, and libraries do not
// belong in the emitter.
//
//go:embed winmap.oro
var winMapSrc string

// mapImplPrefix is the module the embedded source declares. Every name the
// rewrite produces is qualified with it, so nothing can collide with a
// program's own definitions.
const mapImplPrefix = "win/map."

// NeedsMapImpl reports whether this target must be given one.
//
// DECLARED, not inferred, and the reason is that there is nothing sound to
// infer it from: an empty MapType means "this host has no map" on windows and
// "this host has no TYPES" on JavaScript, which has a perfectly good map and
// spells nothing at all. Guessing would have given JavaScript our hash table
// and thrown away a measured 3.67x.
func (tg *Target) NeedsMapImpl() bool { return tg.BuiltinMap }

// lowerMaps rewrites every map operation in the program into calls to the
// embedded implementation, and adds that implementation's definitions.
//
// It runs only when the program actually uses a map. A target with no host map
// otherwise carries no extra definitions at all — and since reduction inlines
// what it reaches and drops what it does not, an unused implementation would
// cost nothing anyway; not adding it keeps the residual honest.
func lowerMaps(tg *Target, p *core.Program) error {
	if !tg.NeedsMapImpl() || !usesMap(tg, p) {
		return nil
	}
	forms, err := core.Read(winMapSrc)
	if err != nil {
		return fmt.Errorf("the built-in map implementation does not parse: %w", err)
	}
	impl, _, err := core.Load(forms)
	if err != nil {
		return fmt.Errorf("the built-in map implementation does not load: %w", err)
	}
	// Only what the implementation's own MODULE declares. Loading it produces
	// the language's injected `option` too — every module gets `some` and
	// `none` — and those are already in the program under the same names, so
	// merging everything reports the injection colliding with itself.
	for n, d := range impl.Defs {
		if !strings.HasPrefix(n, mapImplPrefix) {
			continue
		}
		if _, dup := p.Defs[n]; dup {
			return fmt.Errorf("%s is defined twice — the %s namespace is reserved "+
				"for the built-in map implementation", n, mapImplPrefix)
		}
		p.Defs[n] = d
	}
	for n, d := range p.Defs {
		p.Defs[n] = rewriteMap(tg, p.Defs, newMapEnv(tg), d)
	}
	return nil
}

// usesMap reports whether any definition mentions a map construct.
func usesMap(tg *Target, p *core.Program) bool {
	for _, d := range p.Defs {
		if mentionsMap(tg, d) {
			return true
		}
	}
	return false
}

func mentionsMap(tg *Target, t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KApp {
		if op := t.Op(); op.Kind == core.KName {
			switch tg.Prims[op.Name].Kind {
			case "map-build", "map-insert", "map":
				return true
			}
		}
	}
	for _, k := range t.Kids {
		if mentionsMap(tg, k) {
			return true
		}
	}
	return false
}

// rewriteMap is the rewrite itself, bottom-up so a nested map is lowered before
// the term that contains it.
func rewriteMap(tg *Target, defs map[string]*core.Term, env mapEnv, t *core.Term) *core.Term {
	if t == nil {
		return nil
	}
	// THE READ IS DETECTED ON THE ORIGINAL TERM, before the recursion below
	// lowers its operand. Bottom-up, `(map …)` has already become a `build` by
	// the time the enclosing read is looked at, and a `build` is an array
	// buffer — indistinguishable from a map's. Deciding first and lowering the
	// pieces afterwards keeps the two apart.
	if t.Kind == core.KApp {
		if out, done := rewriteRead(tg, defs, env, t); done {
			return out
		}
	}
	out := *t
	if len(t.Kids) > 0 {
		out.Kids = make([]*core.Term, len(t.Kids))
		for i, k := range t.Kids {
			out.Kids[i] = rewriteMap(tg, defs, env.at(t, i), k)
		}
	}
	if out.Kind != core.KApp {
		return &out
	}

	op := out.Op()
	if op.Kind != core.KName {
		return &out
	}
	switch tg.Prims[op.Name].Kind {
	case "map-build":
		// `(build-map cap (fn (m) …))` becomes `(build (* 3 slots) (fn (m) …))`.
		// The buffer IS the table, so the body needs no change: every operation
		// on `m` inside it has already been rewritten by the recursion above.
		if a := out.Args(); len(a) == 2 {
			return core.App(core.Name("build"),
				core.App(core.Name(tg.times()), core.Int(3), slotsOf(a[0])), a[1])
		}
	case "map-insert":
		if a := out.Args(); len(a) == 3 {
			return core.App(core.Name(mapImplPrefix+"wm-put"), a[0], a[1], a[2])
		}
	case "len":
		// `len m` IS `|dom m|`, and it is the one map operation that cannot be
		// recognised after lowering: a lowered map is an array buffer, so
		// `(len m)` and `(len a)` are the same term meaning different things.
		// Deciding it HERE, while `build-map` is still visible, is the only
		// place the type exists.
		//
		// Without this it returned the BUFFER's length — 3 words per slot, at
		// twice the capacity — so a five-entry map reported 48. It compiled,
		// ran and printed.
		if a := out.Args(); len(a) == 1 && env.isMapValue(t.Args()[0]) {
			return core.App(core.Name(mapImplPrefix+"wm-len"), a[0])
		}
	case "map":
		// A LITERAL becomes a `build-map` of its own rows, which the case above
		// then lowers. It reaches here only when it survived beta-tab, and it
		// survives exactly when the key is dynamic.
		//
		// AND THE STATIC CASE STILL LEAVES NOTHING, which is the part worth
		// stating: rewriting before reduction looks as though it would destroy
		// the folding maps.md §5.2 depends on, and it does not. A literal table
		// with a literal key makes the whole probe static — the hash, the
		// slots, the compares — so reduction runs the hash table AT COMPILE
		// TIME and the residual is the value. The two-level language does on
		// the host with no map exactly what beta-tab does on the three with
		// one, by a different route and to the same normal form.
		rows := out.Args()
		// FILLED BY A LOOP OVER TWO ARRAYS, not by nested `insert`s.
		//
		// The obvious lowering is `(wm-put (wm-put … k1 v1) k2 v2)`, one call
		// per row, and it is WRONG on this host beyond two rows — not by a map
		// bug but by an x86 limit the repository already documents: each
		// `wm-put` inlines to its own probe loop holding the buffer, an index
		// and a slot, and the scratch registers run out.
		// `gauntlet/differential/cases/len-bounded.oro` records the same wall
		// from the other direction, capping itself at three inputs because
		// "more of them exhausts x86's scratch registers".
		//
		// A loop has ONE call site and constant register pressure however many
		// rows there are, so it works at any size. The keys and values become
		// two ordinary array literals, which every backend already emits.
		keys := make([]*core.Term, len(rows))
		vals := make([]*core.Term, len(rows))
		for i, row := range rows {
			if row.Kind != core.KApp || len(row.Kids) != 2 {
				return &out
			}
			keys[i], vals[i] = row.Kids[0], row.Kids[1]
		}
		// BOUND BY `let`, not indexed in place. An array literal in operator
		// position — `((array 11 22) i)` — is a legal term and reaches the x86
		// backend as an application whose operator is not a name, which it
		// refuses. Naming them is what every other program does anyway.
		ks := core.App(core.Name("array"), keys...)
		vs := core.App(core.Name("array"), vals...)
		fill := core.App(core.Name(mapImplPrefix+"wm-fill"),
			core.Name("#m"), core.Name("#ks"), core.Name("#vs"))
		body := core.App(core.Name("let"), ks, core.Fn([]string{"#ks"},
			core.App(core.Name("let"), vs, core.Fn([]string{"#vs"}, fill))))
		return core.App(core.Name("build"),
			core.App(core.Name(tg.times()), core.Int(3),
				slotsOf(core.Int(int64(len(rows))))),
			core.Fn([]string{"#m"}, body))
	}
	return &out
}

// rewriteRead lowers a map read under its eliminator.
//
//	((m k) (fn (#t #p) body))
//	  ⟶  (let (wm-find m k) (fn (#f)
//	        (body[#t := (if (< #f 0) 1 0), #p := (if (< #f 0) 0 (m (wm-c m #f)))])))
//
// The `let` matters: `wm-find` is a loop, and naming its result is what stops
// the probe running twice, once for the tag and once for the payload. Beta
// would not duplicate it — the reducer let-binds an argument used more than
// once — but relying on that would be relying on an optimisation for a
// correctness-shaped property.
func rewriteRead(tg *Target, defs map[string]*core.Term, env mapEnv, t *core.Term) (*core.Term, bool) {
	op := t.Op()
	if op.Kind != core.KApp || len(t.Args()) != 1 || len(op.Args()) != 1 {
		return nil, false
	}
	cont := t.Args()[0]
	if cont.Kind != core.KFn || len(cont.Params) != 2 {
		return nil, false
	}
	if !isMapTerm(tg, defs, env, op.Op()) {
		return nil, false
	}
	m := rewriteMap(tg, defs, env, op.Op())
	key := rewriteMap(tg, defs, env, op.Args()[0])
	k := rewriteMap(tg, defs, env, cont)

	// THE MAP IS BOUND ONCE. It is mentioned three times below — the probe, the
	// clamp and the read — and when it is a lowered LITERAL that term is a
	// whole `build` with an insert per row. Duplicating it would not merely be
	// large: it would be three separate tables, which is a wrong answer rather
	// than a slow one.
	mv := core.Name("#mv")
	f := core.Name("#f")
	miss := core.App(core.Name(tg.less()), f, core.Int(0))
	tag := core.App(core.Name("if"), miss, core.Int(1), core.Int(0))
	// CLAMPED, through the implementation's own clamp. `wm-find` returns an
	// index it already clamped, or -1, so the fact is true — but it travelled
	// through `x64.and`, which is not linear arithmetic, and the refinement
	// layer is right to refuse what it cannot derive. Re-clamping turns a
	// refusal into a note and makes the program say what happens if the
	// impossible occurs, which is tree.oro's rule unchanged.
	hit := core.App(core.Name(mapImplPrefix+"wm-c"), mv, f)
	val := core.App(core.Name("if"), miss, core.Int(0), core.App(mv, hit))
	inner := core.App(core.Name("let"),
		core.App(core.Name(mapImplPrefix+"wm-find"), mv, key),
		core.Fn([]string{"#f"}, core.App(k, tag, val)))
	return core.App(core.Name("let"), m, core.Fn([]string{"#mv"}, inner)), true
}

// mapEnv tracks WHICH BINDERS HOLD A MAP, which is the one thing the lowering
// cannot recover after it has run.
//
// `(m k)` is distinguishable by shape — its eliminator is a two-parameter
// continuation, and nothing else produces that. `(len m)` is NOT: after
// lowering, a map IS an array buffer, so `(len m)` and `(len a)` are the same
// term and mean different things. The difference is the TYPE, and the only
// place the type is still visible is before the rewrite, while `build-map` and
// `insert` are still in the term.
//
// So the identification happens on the way down, and it is the same
// conservative tracking the JavaScript emitter does for the same reason: a
// binder it does not recognise is left alone and keeps whatever meaning it had.
// It can under-fire; it cannot invent a map.
//
// Keyed by ABSOLUTE binder level and parameter index. A `KBound` says how many
// binders OUT it points (0 = nearest), so at walk-depth d it refers to level
// d-1-Depth.
type mapEnv struct {
	tg    *Target
	depth int
	is    map[[2]int]bool

	// pending is which parameters of the NEXT binder hold maps, decided by that
	// binder's parent and consumed on the way in.
	pending map[int]bool
}

func newMapEnv(tg *Target) mapEnv { return mapEnv{tg: tg, is: map[[2]int]bool{}} }

// kindOf is the structural kind of a name on this target, or "".
func (e mapEnv) kindOf(name string) string { return e.tg.Prims[name].Kind }

// under enters a binder, marking the parameters at `which` as holding maps.
func (e mapEnv) under(which map[int]bool) mapEnv {
	out := mapEnv{tg: e.tg, depth: e.depth + 1, is: map[[2]int]bool{}}
	for k, v := range e.is {
		out.is[k] = v
	}
	for i := range which {
		out.is[[2]int{e.depth, i}] = true
	}
	return out
}

// holdsMap reports whether a bound variable refers to a binder marked as a map.
func (e mapEnv) holdsMap(t *core.Term) bool {
	if t == nil || t.Kind != core.KBound {
		return false
	}
	return e.is[[2]int{e.depth - 1 - t.Depth, t.Index}]
}

// at is the environment for kid i of term t.
//
// TWO THINGS HAPPEN AT DIFFERENT PLACES and conflating them was the first
// version's bug. Depth increases when we ENTER a binder — a `KFn` binds its
// body, whatever its parent was. Which of that binder's parameters hold maps is
// decided by the binder's PARENT, one level up, so it is carried down as
// `pending` and consumed on the way in.
//
// Three parents can introduce one, and they are exactly the three the
// JavaScript emitter tracks — the construct that MAKES a map, the one that
// NAMES it, and the one that THREADS it:
//
//	(build-map cap (fn (m) …))        m is the buffer
//	(let VALUE (fn (x) …))            x is a map when VALUE is
//	(loop (fn (v…) …) init…)          vᵢ is a map when initᵢ is
//
// Anything else carries nothing, so an unrecognised parent can only lose the
// identification, never invent one.
func (e mapEnv) at(t *core.Term, i int) mapEnv {
	if t.Kind == core.KFn {
		return e.under(e.pending)
	}
	out := e
	out.pending = nil
	if t.Kind != core.KApp || t.Kids[i].Kind != core.KFn {
		return out
	}
	op := t.Op()
	// A PLAIN BETA-REDEX, which is what a source `let` actually is. The reader
	// does NOT keep `(let V K)`: it produces `((fn (x) body) V)`, so the binder
	// is the OPERATOR and the value is an operand. Matching on the name `let`
	// found nothing, which is why the first version of this tracked no binder
	// at all and every `(len m)` fell through to the buffer's length.
	if op.Kind == core.KFn && i == 0 {
		which := map[int]bool{}
		for j, a := range t.Args() {
			if j < len(op.Params) && e.isMapValue(a) {
				which[j] = true
			}
		}
		if len(which) > 0 {
			out.pending = which
		}
		return out
	}
	if op.Kind != core.KName {
		return out
	}
	args := t.Args()
	which := map[int]bool{}
	switch e.kindOf(op.Name) {
	case "map-build":
		if i == 2 && len(args) == 2 {
			which[0] = true
		}
	case "let":
		if i == 2 && len(args) == 2 && e.isMapValue(args[0]) {
			which[0] = true
		}
	case "iterate":
		// `(loop (fn (v…) …) z…)`: the lambda is argument 1 and the inits
		// follow it positionally.
		if i == 1 {
			for j, z := range args[1:] {
				if e.isMapValue(z) {
					which[j] = true
				}
			}
		}
	}
	if len(which) > 0 {
		out.pending = which
	}
	return out
}

// isMapValue reports whether a term, in the environment it appears in, produces
// a map. Deliberately syntactic: a `build-map`, an `insert`, a literal, or a
// bound variable already known to hold one.
func (e mapEnv) isMapValue(t *core.Term) bool {
	if t == nil {
		return false
	}
	if e.holdsMap(t) {
		return true
	}
	if t.Kind != core.KApp {
		return false
	}
	if op := t.Op(); op.Kind == core.KName {
		switch e.kindOf(op.Name) {
		case "map-build", "map-insert", "map":
			return true
		case "iterate":
			// A loop THREADING a map yields one. Its exits are its clause
			// bodies, and the buffer is what a linear loop hands back, so it is
			// enough that an INIT was a map.
			for _, z := range t.Args()[1:] {
				if e.isMapValue(z) {
					return true
				}
			}
		}
		if op.Name == mapImplPrefix+"wm-put" || op.Name == mapImplPrefix+"wm-fill" {
			return true
		}
	}
	return false
}

// slotsOf is the slot count for a capacity: the least power of two that is at
// least twice it, so the load factor never exceeds 1/2 and linear probing stays
// short.
//
// FOLDED HERE when the capacity is a literal, which it almost always is — a
// capacity is a declaration somebody wrote. That removes a runtime loop from
// every program that builds a map, and it is the two-level language applied to
// the compiler's own generated code rather than only to the user's.
//
// `wm-slots` remains for a capacity computed at run time.
func slotsOf(cap *core.Term) *core.Term {
	if cap.Kind == core.KInt {
		n := int64(1)
		for n < 2*cap.Int {
			n *= 2
		}
		return core.Int(n)
	}
	return core.App(core.Name(mapImplPrefix+"wm-slots"), cap)
}

// isMapTerm reports whether a term in OPERATOR position of a read is a map.
//
// A KBOUND, not a KName, and the distinction is load-bearing rather than
// incidental. Inside `build-map`'s body the buffer is a lambda PARAMETER, and
// by the time this runs the reader has already turned every binder into de
// Bruijn indices — so `m` prints as a name and is not one.
//
// Accepting a bare KName here would be a WRONG ANSWER, not a missed case: a
// user sum eliminated on a CALL is `((f x) (fn (#t #p) …))` with `f` a KName,
// which is the same shape, and rewriting it into a hash-table probe would
// silently compile a different program. That shape is exactly what
// `examples/sum/parse.oro` produces.
//
// A `case` on a sum-typed VARIABLE is safe either way — it expands to
// `(s (fn (#t #p) …))`, whose operator is the variable itself and not an
// application, so the guard above never reaches here.
func isMapTerm(tg *Target, defs map[string]*core.Term, env mapEnv, t *core.Term) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case core.KBound:
		// Marked by an enclosing `build-map`, `let` or `loop` — not "any bound
		// variable", which would call an ARRAY read a map read.
		return env.holdsMap(t)
	case core.KName:
		// A map held by a top-level definition, which reduction would inline
		// but has not yet.
		return isMapProducer(tg, defs, defs[t.Name])
	case core.KApp:
		return isMapProducer(tg, defs, t)
	}
	return false
}

// isMapProducer reports whether a term evaluates to a map. After the rewrite
// has run on it a lowered `build-map` is a `build`, so this recognises the
// unlowered forms and the implementation's own threading call.
func isMapProducer(tg *Target, defs map[string]*core.Term, t *core.Term) bool {
	if t == nil || t.Kind != core.KApp {
		return false
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	switch tg.Prims[op.Name].Kind {
	case "map", "map-build", "map-insert":
		return true
	}
	return op.Name == mapImplPrefix+"wm-put"
}

// less and times are the target's own comparison and multiplication, found the
// way `addCore` finds equality rather than hardcoded.
//
// The rewrite has to emit arithmetic, and arithmetic is TARGET-NATIVE: there is
// no language-level `<` or `*`. Looking them up keeps this file free of any
// host's spelling, so a second target with no map needs no change here.
func (tg *Target) less() string  { return tg.findOp([]string{"setl", "<"}) }
func (tg *Target) times() string { return tg.findOp([]string{"imul", "*"}) }

func (tg *Target) findOp(spellings []string) string {
	for _, s := range spellings {
		for _, n := range tg.Names {
			if suffix(n) == s {
				return n
			}
		}
	}
	return ""
}

func suffix(qualified string) string {
	for i := len(qualified) - 1; i >= 0; i-- {
		if qualified[i] == '.' {
			return qualified[i+1:]
		}
	}
	return qualified
}
