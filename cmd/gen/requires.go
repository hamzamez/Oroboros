package main

import (
	"fmt"
	"math/big"
	"os"
	"sort"

	"oroboros/core"
	"oroboros/emit"
)

// reportRequires is -report-requires: a MEASUREMENT, which changes nothing the
// compiler emits. For every direct call of a definition whose signature
// declares a range on a parameter, it reports whether the argument is provably
// in that range where the call is inlined — the question enforcing parameter
// ranges at inlined calls would ask (types.md §7, refinements.md §6b). One line
// per distinct (definition, parameter, argument):
//
//	requires DEF PARAM TYPE CLASS ARG
//
// CLASS is `in` or `OUT` when the argument reduced to a literal (decided by the
// reducer); `proven` or `unproven` when it did not, decided by the interval
// analysis on the residual, which prints the interval it found. The marks that
// carry the question are erased before anything else runs (emit.MeasureRequires),
// so what is emitted is what is emitted without the flag.
var reportRequires bool

// installRequires fills env's measurement table from the program's signatures
// and prints what the hook reports when the compile is done.
func installRequires(env *core.Env, prog *core.Program) func() {
	env.Requires = map[string][]string{}
	env.Prim[core.RequireName] = true
	env.Pure[core.RequireName] = true
	for name, sig := range prog.Sigs {
		if sig == nil {
			continue
		}
		if _, isDef := prog.Defs[name]; !isDef {
			continue
		}
		tys := make([]string, len(sig.Params))
		ranged := false
		for i, p := range sig.Params {
			if _, _, ok := core.IntRangeBig(p.Type); ok {
				tys[i], ranged = p.Type, true
			}
		}
		if ranged {
			env.Requires[name] = tys
		}
	}
	env.OnRequire = func(def, param, ty string, arg *core.Term) {
		requireLine(requireSeen, def, param, ty, classify(ty, arg), arg.String())
	}
	return func() {
		sort.Strings(lines)
		for _, l := range lines {
			fmt.Fprintln(os.Stderr, l)
		}
	}
}

var lines []string

func requireLine(seen map[string]bool, def, param, ty, class, arg string) {
	line := fmt.Sprintf("requires %s %s %q %s %s", def, param, ty, class, arg)
	if !seen[line] {
		seen[line] = true
		lines = append(lines, line)
	}
}

// requireSeen dedupes the residual's decisions with the reducer's.
var requireSeen = map[string]bool{}

// measureResidual decides the marks left in one unit's residual and returns it
// with the marks erased.
func measureResidual(tg *emit.Target, sig *core.Sig, nf *core.Term) *core.Term {
	res, stripped := emit.MeasureRequires(tg, sig, nf)
	for _, r := range res {
		class := "unproven"
		if r.Proven {
			class = "proven"
		}
		requireLine(requireSeen, r.Def, r.Param, r.Type, class, fmt.Sprintf("%s  got %v", r.Arg, r.Got))
	}
	return stripped
}

func classify(ty string, arg *core.Term) string {
	lo, hi, ok := core.IntRangeBig(ty)
	if !ok {
		return "fail"
	}
	v, closed := evalClosed(arg)
	if !closed {
		return "open"
	}
	if (lo != nil && v.Cmp(lo) < 0) || (hi != nil && v.Cmp(hi) > 0) {
		return "OUT"
	}
	return "in"
}

// evalClosed evaluates an integer term built from literals and the language's
// arithmetic, at arbitrary precision — how a literal past the word arrives
// (a Horner spine) and how a closed argument would fold. Anything else is open.
func evalClosed(t *core.Term) (*big.Int, bool) {
	switch t.Kind {
	case core.KInt:
		return big.NewInt(t.Int), true
	case core.KApp:
		op := t.Op()
		args := t.Args()
		if op.Kind != core.KName || len(args) != 2 {
			return nil, false
		}
		a, ok1 := evalClosed(args[0])
		b, ok2 := evalClosed(args[1])
		if !ok1 || !ok2 {
			return nil, false
		}
		r := new(big.Int)
		switch op.Name {
		case "+":
			return r.Add(a, b), true
		case "-":
			return r.Sub(a, b), true
		case "*":
			return r.Mul(a, b), true
		case "/":
			if b.Sign() == 0 {
				return nil, false
			}
			return r.Quo(a, b), true
		case "%":
			if b.Sign() == 0 {
				return nil, false
			}
			return r.Rem(a, b), true
		}
	}
	return nil, false
}
