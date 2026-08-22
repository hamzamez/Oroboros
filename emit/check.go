package emit

import (
	"fmt"
	"sort"

	"oroboros/core"
)

// The type checker, specified in docs/spec/types.md.
//
// It runs on the RESIDUAL, before emission. That is the cheap place: reduction
// has already specialised every generic definition and every backend refuses an
// escaping closure, so the term is monomorphic, first-order and closed. No type
// schemes, no generalisation, no unification beyond the trivial.
//
// It is not a backend. `targets/js.oro` declares argument types it never uses,
// and said why: "they document the primitive and a future checker could use
// them regardless of target". That is what lets one checker serve three targets
// — including the one that would otherwise catch nothing at all, and which today
// compiles `(f64.add "hello" 1.0)` into a program that prints `hello1`.

type checker struct {
	tgt   *Target
	types map[string]string // name -> the type demanded of it
}

// Check verifies a residual against the target's declared types. It reports the
// first conflict, naming both demands.
func Check(tgt *Target, what string, t *core.Term) error {
	c := &checker{tgt: tgt, types: map[string]string{}}
	// Two passes: the first learns parameter types from use, the second checks
	// with everything known. Without this a conflict is reported or missed
	// depending on the order the term happens to be written in.
	_, _ = c.walk(t, "")
	if _, err := c.walk(t, ""); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// walk checks a term and returns its type, or "" for unknown.
func (c *checker) walk(t *core.Term, want string) (string, error) {
	switch t.Kind {
	case core.KInt:
		return "int", c.agree("an integer literal", "int", want)
	case core.KFloat:
		return "f64", c.agree("a float literal", "f64", want)
	case core.KBool:
		return "bool", c.agree("a boolean literal", "bool", want)
	case core.KStr:
		return "string", c.agree("a string literal", "string", want)

	case core.KName:
		got := c.types[t.Name]
		if got == "" {
			if want != "" && want != "any" {
				c.types[t.Name] = want // the inference half
			}
			return want, nil
		}
		return got, c.agree(t.Name, got, want)

	case core.KFn:
		// Walk the body. The top-level term of every residual is an
		// abstraction, so skipping these checked nothing at all — which is
		// what the first run of this checker did.
		//
		// A λ in ARGUMENT position is a different thing: either a structural
		// primitive's step function, handled below, or an escaping closure the
		// emitter refuses for a better reason than a type error.
		return c.walk(t.Body(), want)
	}

	op := t.Op()
	if op.Kind != core.KName {
		return "", nil // the emitter reports this better than the checker can
	}
	p, ok := c.tgt.Prims[op.Name]
	if !ok {
		return "", nil // not a primitive, so no signature fixes the type
	}
	args := t.Args()

	switch p.Kind {
	case "loop":
		return c.loop(args, want)
	case "loop2":
		return c.loop2(args, want)
	case "cond":
		return c.cond(args, want)
	case "iterate":
		return c.iterate(args, want)
	case "let":
		return c.let(args, want)
	case "build":
		return c.build(args, want)
	}

	// An ordinary primitive: demand each declared argument type.
	for i, a := range args {
		d := ""
		if i < len(p.Args) {
			d = p.Args[i]
		}
		if _, err := c.walk(a, d); err != nil {
			// `=` is the LANGUAGE's equality and is deliberately narrow, so its
			// refusal has to explain itself — that explanation is the reason it
			// is called `=` rather than a narrow name like `tag=`. A name cannot
			// say why; an error can.
			if op.Name == "=" {
				return "", fmt.Errorf("in argument %d of `=`: %w.\n"+
					"`=` is the language's equality and it is integer equality only. "+
					"Floats are excluded because NaN is not an equivalence relation, "+
					"and strings because no two of the four targets agree on comparing "+
					"them (docs/spec/strings.md). For a host's own equality, name it: "+
					"`go.==`, `js.===`, `java.==`, `x64.sete` — that is target-native "+
					"and carries no portability claim", i+1, err)
			}
			return "", fmt.Errorf("in argument %d of %s: %w", i+1, op.Name, err)
		}
	}
	// A statement's value is argument 0 (target-files.md §3).
	res := p.Result
	if p.Kind == "stmt" && len(args) > 0 {
		if ty, _ := c.walk(args[0], ""); ty != "" {
			res = ty
		}
	}
	return res, c.agree(op.Name, res, want)
}

// agree is the whole of conflict detection: two different CONCRETE demands on
// the same thing. Unknown agrees with everything, and `any` demands nothing.
func (c *checker) agree(what, got, want string) error {
	if got == "" || want == "" || want == "any" || got == "any" || got == want {
		return nil
	}
	return fmt.Errorf("%s is %s, but %s is required here", what, got, want)
}

func (c *checker) loop(args []*core.Term, want string) (string, error) {
	if len(args) != 3 {
		return "", nil
	}
	acc, err := c.walk(args[0], "")
	if err != nil {
		return "", err
	}
	if _, err := c.walk(args[1], "int"); err != nil {
		return "", fmt.Errorf("in a loop count: %w", err)
	}
	step := args[2]
	if step.Kind == core.KFn && len(step.Params) == 2 {
		if acc != "" {
			c.types[step.Params[0]] = acc
		}
		c.types[step.Params[1]] = "int"
		if _, err := c.walk(step.Body(), acc); err != nil {
			return "", fmt.Errorf("in a loop body: %w", err)
		}
	}
	return acc, c.agree("a loop", acc, want)
}

func (c *checker) loop2(args []*core.Term, want string) (string, error) {
	if len(args) != 6 {
		return "", nil
	}
	for _, i := range []int{0, 1} {
		if _, err := c.walk(args[i], "f64"); err != nil {
			return "", fmt.Errorf("in a loop2 accumulator: %w", err)
		}
	}
	if _, err := c.walk(args[2], "int"); err != nil {
		return "", fmt.Errorf("in a loop2 count: %w", err)
	}
	for _, s := range []*core.Term{args[3], args[4]} {
		if s.Kind == core.KFn && len(s.Params) == 3 {
			c.types[s.Params[0]], c.types[s.Params[1]], c.types[s.Params[2]] = "f64", "f64", "int"
			if _, err := c.walk(s.Body(), "f64"); err != nil {
				return "", fmt.Errorf("in a loop2 step: %w", err)
			}
		}
	}
	fin := args[5]
	if fin.Kind == core.KFn && len(fin.Params) == 2 {
		c.types[fin.Params[0]], c.types[fin.Params[1]] = "f64", "f64"
		return c.walk(fin.Body(), want)
	}
	return "", nil
}

// iterate checks (loop (fn (x…) body) z…) — docs/spec/iteration.md.
//
// Each loop variable takes its initial value's type; every `again` argument
// must agree with the variable it feeds; every exit clause must agree with the
// loop's own type. `loop` had NO case here at all until targets/js/ was written
// and nothing complained about a mistyped one.
func (c *checker) iterate(args []*core.Term, want string) (string, error) {
	if len(args) < 2 || args[0].Kind != core.KFn {
		return "", nil
	}
	lam := args[0]
	inits := args[1:]
	if len(lam.Params) != len(inits) {
		return "", fmt.Errorf("loop has %d variable(s) and %d initial value(s)",
			len(lam.Params), len(inits))
	}
	tys := make([]string, len(inits))
	for i, z := range inits {
		ty, err := c.walk(z, "")
		if err != nil {
			return "", fmt.Errorf("in a loop's initial value: %w", err)
		}
		tys[i] = ty
		c.types[lam.Params[i]] = ty
	}
	return c.loopBody(lam.Body(), lam.Params, tys, want)
}

// loopBody walks the clause chain: `again` leaves check their arguments, other
// leaves are the loop's value.
func (c *checker) loopBody(t *core.Term, params, tys []string, want string) (string, error) {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if t.Op().Name == "again" {
			as := t.Args()
			if len(as) != len(params) {
				return "", fmt.Errorf("again takes %d argument(s), given %d", len(params), len(as))
			}
			for i, a := range as {
				if _, err := c.walk(a, tys[i]); err != nil {
					return "", fmt.Errorf("in again's argument %d: %w", i+1, err)
				}
			}
			return "", nil // not a value
		}
		if p, ok := c.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			if _, err := c.walk(t.Args()[0], "bool"); err != nil {
				return "", fmt.Errorf("in a loop's guard: %w", err)
			}
			a, err := c.loopBody(t.Args()[1], params, tys, want)
			if err != nil {
				return "", err
			}
			b, err := c.loopBody(t.Args()[2], params, tys, want)
			if err != nil {
				return "", err
			}
			if a != "" && b != "" && a != b {
				return "", fmt.Errorf("a loop's exits are %s and %s", a, b)
			}
			if a == "" {
				return b, nil
			}
			return a, nil
		}
	}
	return c.walk(t, want)
}

func (c *checker) cond(args []*core.Term, want string) (string, error) {
	if len(args) != 3 {
		return "", nil
	}
	if _, err := c.walk(args[0], "bool"); err != nil {
		return "", fmt.Errorf("in a condition: %w", err)
	}
	a, err := c.walk(args[1], want)
	if err != nil {
		return "", err
	}
	b, err := c.walk(args[2], want)
	if err != nil {
		return "", err
	}
	if a != "" && b != "" && a != b {
		return "", fmt.Errorf("the branches of a conditional are %s and %s", a, b)
	}
	if a == "" {
		a = b
	}
	return a, c.agree("a conditional", a, want)
}

func (c *checker) let(args []*core.Term, want string) (string, error) {
	if len(args) != 2 {
		return "", nil
	}
	v, err := c.walk(args[0], "")
	if err != nil {
		return "", err
	}
	k := args[1]
	if k.Kind == core.KFn && len(k.Params) == 1 {
		if v != "" {
			c.types[k.Params[0]] = v
		}
		return c.walk(k.Body(), want)
	}
	return "", nil
}

func (c *checker) build(args []*core.Term, want string) (string, error) {
	if len(args) != 2 {
		return "", nil
	}
	if _, err := c.walk(args[0], "int"); err != nil {
		return "", fmt.Errorf("in a make-vec length: %w", err)
	}
	elem := args[1]
	if elem.Kind == core.KFn && len(elem.Params) == 1 {
		c.types[elem.Params[0]] = "int"
		if _, err := c.walk(elem.Body(), "f64"); err != nil {
			return "", fmt.Errorf("in a make-vec element: %w", err)
		}
	}
	return "vec-f64", c.agree("make-vec", "vec-f64", want)
}

// CheckSignatures compares every declared signature against the target's native
// implementation of the same name.
//
// **This is the job no host compiler can do.** A library defines num/vec.dot and
// a target may provide it natively; the two are compared against one statement,
// and they live on different targets, so no single host compiler ever sees both.
// It is modules.md T2's substitution soundness becoming machine-checked instead
// of asserted — and until now the only evidence for it was a conformance suite
// that runs the code.
func CheckSignatures(tgt *Target, prog *core.Program, env *core.Env) error {
	names := make([]string, 0, len(prog.Sigs))
	for n := range prog.Sigs {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		sig := prog.Sigs[n]
		p, native := tgt.Prims[n]
		if !native {
			// This target derives the name, so the claim is about the
			// DEFINITION. Normalising it is bounded by the number of
			// signatures, which is small, and it is the only way to check a
			// library export that the program never reaches directly.
			body, ok := prog.Defs[n]
			if !ok || env == nil {
				continue
			}
			nf, err := core.Normalize(body, env, core.DefaultFuel)
			if err != nil {
				continue // reduction already reports this better
			}
			if err := CheckAgainstSig(tgt, n, sig, nf); err != nil {
				return err
			}
			continue
		}
		if len(p.Args) != len(sig.Params) {
			return fmt.Errorf("%s: target %s provides it with %d argument(s), "+
				"but its signature declares %d", n, tgt.Name, len(p.Args), len(sig.Params))
		}
		for i, want := range sig.Params {
			if got := p.Args[i]; !compatible(got, want.Type) {
				return fmt.Errorf("%s: argument %d is %s in target %s, but %s in its signature",
					n, i+1, got, tgt.Name, want.Type)
			}
		}
		if !compatible(p.Result, sig.Result) {
			return fmt.Errorf("%s: target %s returns %s, but its signature declares %s",
				n, tgt.Name, p.Result, sig.Result)
		}
	}
	return nil
}

// compatible treats `any` and the unknown type as agreeing with anything, which
// is the same rule the term checker uses.
func compatible(a, b string) bool {
	return a == "" || b == "" || a == "any" || b == "any" || a == b
}

// CheckAgainstSig checks a residual against its declared signature: the
// parameters take their declared types and the body must produce the declared
// result. This is what makes a signature a claim about the DEFINITION rather
// than only about the target.
func CheckAgainstSig(tgt *Target, name string, sig *core.Sig, t *core.Term) error {
	if t.Kind != core.KFn {
		return nil
	}
	if len(t.Params) != len(sig.Params) {
		return fmt.Errorf("%s takes %d argument(s), but its signature declares %d",
			name, len(t.Params), len(sig.Params))
	}
	c := &checker{tgt: tgt, types: map[string]string{}}
	for i, p := range t.Params {
		if ty := sig.Params[i].Type; ty != "" && ty != "any" {
			c.types[p] = ty
		}
	}
	for pass := 0; pass < 2; pass++ {
		got, err := c.walk(t.Body(), sig.Result)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if pass == 1 && got != "" && sig.Result != "" && sig.Result != "any" && got != sig.Result {
			return fmt.Errorf("%s returns %s, but its signature declares %s",
				name, got, sig.Result)
		}
	}
	return nil
}
