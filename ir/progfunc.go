package ir

import (
	"fmt"
	"strings"
	"sync"

	"oroboros/core"
	"oroboros/emit"
)

// A LIBRARY INSTANCE IS A FUNCTION OF THE PROGRAM (ADR 0101, ir.md §2.5).
//
// An instance of K(ℕ*)'s library is a closed λ checked once as a theory
// (ADR 0035). Calling it and β-reducing it into the site are the same
// application, so a use may be `call f ā`: what changes is where its
// arithmetic is decided. The callee is decided once, under its parameters'
// sets, a numeral's table `array int 0 4294967295` (its header is 2m + s,
// digitfacts-2026-11-10), and its result's element range is what its stores
// prove. Ownership across the call is the frame rule: the arguments borrowed,
// the result a birth (W13 checks the callee's side).
//
// An instance depends on the target alone (max_len is its one specialization),
// so its function is built once per target and named for the instance, and a
// program's text holds each it calls: every printer reprints from the text
// (ADR 0038), registering the functions it finds there first.

// NumeralType is a numeral's table as a program function's parameter and
// result: the codomain [0, B), B = 2³², of every entry.
const NumeralType = "array int 0 4294967295"

// natPrefix names K(ℕ*)'s program functions.
const natPrefix = "nat."

// natCalled are the instances called rather than inlined: those whose result
// is a numeral (or text) and whose code is large. The small ones, `of`, the
// comparisons, `rem-small`, `div-small` and the bound checks, stay inlined: a
// call's result is known only by its declared set, and theirs are bounded
// relationally (a remainder below its divisor, a short quotient's digit below B
// because its partial remainder is below the divisor), which no declared set
// can say. Inlined, the site's facts bound them, as before ADR 0101.
//
// A test may add an instance its contract does not prove (div-small): P4 then
// inlines it (ADR 0102).
var natCalled = map[string]bool{"add": true, "sub": true, "mul": true, "mul-small": true,
	"div": true, "rem": true, "render": true}

type calleeKey struct {
	tg     *emit.Target
	fn     string
	narrow bool // PlantNarrowNumeral's contract, cached apart
}

var (
	calleeMu sync.Mutex
	callees  = map[calleeKey]*Func{}
	calleeEr = map[calleeKey]error{}
)

// natCallee is K(ℕ*)'s instance fn as a program function on tg, built and
// registered once: its name, or an error the instance's own.
func natCallee(tg *emit.Target, fn string) (string, error) {
	name := natPrefix + fn
	k := calleeKey{tg, fn, PlantNarrowNumeral}
	calleeMu.Lock()
	defer calleeMu.Unlock()
	if _, ok := callees[k]; ok {
		return name, nil
	}
	if err := calleeEr[k]; err != nil {
		return "", err
	}
	f, err := buildNatCallee(tg, fn, name)
	if err != nil {
		calleeEr[k] = err
		return "", err
	}
	callees[k] = f
	registerProgramFunc(tg, f)
	return name, nil
}

// buildNatCallee lowers the instance with a numeral's set on each table
// parameter and result, decides it under them, and takes it through the
// stages an entry takes after its rung. Every operation must be proven: a
// callee has no site whose facts could prove more.
func buildNatCallee(tg *emit.Target, fn, name string) (*Func, error) {
	of := fn
	if PlantNatFault && fn == "sub" {
		of = "add" // the planted fault, where the difference is now built
	}
	inst, err := emit.NatFunc(tg, of)
	if err != nil {
		return nil, err
	}
	sig := *inst.Sig
	sig.Params = append([]core.SigParam(nil), inst.Sig.Params...)
	for i := range sig.Params {
		switch {
		case sig.Params[i].Type == "array int" && PlantNarrowNumeral:
			sig.Params[i].Type = "array int 0 1"
		case sig.Params[i].Type == "array int":
			sig.Params[i].Type = NumeralType
		// THE WORD OPERAND'S SET is the one the plan chose the one-pass form for
		// (limbs.go's widened: k ∈ [0, 2²⁸)), a divisor's without its zero
		case sig.Params[i].Type == "int" && (fn == "mul-small" || fn == "div-small"):
			lo := 0
			if fn == "div-small" {
				lo = 1
			}
			sig.Params[i].Type = fmt.Sprintf("int %d %d", lo, 1<<28-1)
		}
	}
	f, err := Lower(tg, name, &sig, inst.Term, Options{Decided: true})
	if err != nil {
		return nil, fmt.Errorf("the built-in K(ℕ*)'s %s as a function: %v", fn, err)
	}
	leg := Decide(tg, f, false)
	if leg, err = afterRung(tg, f, name, false, leg, func(Note) {}); err != nil {
		return nil, err
	}
	if err := leg.Refusal(name, tg); err != nil {
		return nil, fmt.Errorf("the built-in K(ℕ*)'s %s is not decided under its parameters' sets: %v", fn, err)
	}
	if err := leg.SizeRefusal(name, tg); err != nil {
		return nil, err
	}
	if err := CheckBornIn(tg, f); err != nil {
		return nil, fmt.Errorf("the built-in K(ℕ*)'s %s: %v", fn, err)
	}
	// THE RESULT'S SET IS A CLAIM ITS BODY PROVES (ADR 0072, on a table): every
	// returned table's elements in [0, B), by its stores, before the callers
	// may read it as a numeral's
	if len(f.Results) == 1 && f.Results[0] == "array int" {
		if err := proveResultElems(tg, f, 0, 1<<32-1); err != nil {
			return nil, fmt.Errorf("the built-in K(ℕ*)'s %s: %v", fn, err)
		}
		f.Results[0] = NumeralType
	}
	SelectShifts(tg, f)
	return f, nil
}

// PlantResultUnproven declares a program function's result a numeral's
// without asking its body.
var PlantResultUnproven bool

// proveResultElems requires every table f returns to have its elements in
// [lo, hi] by f's own facts.
func proveResultElems(tg *emit.Target, f *Func, lo, hi int64) error {
	if PlantResultUnproven {
		return nil
	}
	fs := analyse(tg, f)
	var err error
	f.Walk(func(r *Region) {
		if err != nil || r.T != TYield || !returns(f, r) {
			return
		}
		for _, a := range r.Args {
			if x := fs[a]; !x.hasEl || !x.el.within(ei(lo), ei(hi)) {
				err = fmt.Errorf("its result's elements are %s, not inside %d..%d, which its callers would read", termShow(x.el), lo, hi)
				return
			}
		}
	})
	return err
}

// registerProgramFunc names f among the target's primitives: its signature
// its own, pure, and a call's spelling on LLVM.
func registerProgramFunc(tg *emit.Target, f *Func) {
	if q, ok := tg.Prims[f.Name]; ok && q.Program {
		return
	}
	var args, holes []string
	for i, p := range f.Params {
		args = append(args, f.Types[p])
		holes = append(holes, fmt.Sprintf("%%%d", i+1))
	}
	res := ""
	if len(f.Results) == 1 {
		res = f.Results[0]
	}
	tg.Prims[f.Name] = emit.Prim{Name: f.Name, Args: args, Result: res, Kind: "expr", Pure: true, Program: true,
		Form: fmt.Sprintf("call %s %s(%s)", llvmType(res), llvmName(f.Name), strings.Join(holes, ", "))}
}

// llvmType is an IR type's LLVM type, as ir/llvm spells it for a program
// function's signature: a table, a string and a pointer are `ptr`.
func llvmType(t string) string {
	switch {
	case t == "bool":
		return "i1"
	case t == "int" || strings.HasPrefix(t, "int "):
		return "i64"
	}
	return "ptr"
}

// llvmName is ir/llvm.Name, here because ir/llvm imports ir.
func llvmName(n string) string { return "@" + strings.ReplaceAll(n, "-", "_") }

// IsProgramFunc says name is a program function's.
func IsProgramFunc(name string) bool { return strings.HasPrefix(name, natPrefix) }

// Callees are the program functions f calls, built for tg, in name order.
func Callees(tg *emit.Target, f *Func) []*Func {
	seen := map[string]bool{}
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall && IsProgramFunc(s.Name) {
				seen[s.Name] = true
			}
		}
	})
	calleeMu.Lock()
	defer calleeMu.Unlock()
	var out []*Func
	for k, c := range callees {
		if k.tg == tg && seen[c.Name] {
			out = append(out, c)
		}
	}
	sortFuncs(out)
	return out
}

func sortFuncs(fs []*Func) {
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fs[j].Name < fs[j-1].Name; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// RegisterProgram registers every program function a program's text holds,
// so a printer reprints a call from the text alone (ADR 0038).
func RegisterProgram(tg *emit.Target, p *Program) {
	for _, f := range p.Funcs {
		if IsProgramFunc(f.Name) {
			registerProgramFunc(tg, f)
		}
	}
}

// CheckBornIn is W13, a program function's side of the frame rule: every
// table it returns is born in it, never one of its parameters, so the caller
// may own the result while its arguments stay its own.
func CheckBornIn(tg *emit.Target, f *Func) error {
	c := newRelCtx(tg, f)
	params := map[V]bool{}
	for _, p := range f.Params {
		params[p] = true
	}
	var err error
	f.Walk(func(r *Region) {
		if err != nil || r.T != TYield || r != f.Body && !returns(f, r) {
			return
		}
		for _, a := range r.Args {
			if !c.table(a) {
				continue
			}
			if params[c.rootOf(a)] {
				err = fmt.Errorf("W13: a program function returns its parameter %%%d, which its caller still owns", c.rootOf(a))
				return
			}
		}
	})
	return err
}

// returns says region r's yield is f's result: a terminator tree's leaf of
// f's body, not an inner region's.
func returns(f *Func, r *Region) bool {
	var found bool
	var walk func(x *Region)
	walk = func(x *Region) {
		if x == r {
			found = true
			return
		}
		if x.T == TBranch {
			walk(x.Then)
			walk(x.Else)
		}
	}
	walk(f.Body)
	return found
}

// PlantNoCallSets admits every argument to a program function, its set unasked.
var PlantNoCallSets bool

// PlantNarrowNumeral declares a numeral parameter's elements 0..1, a contract
// stronger than the sites': every call's obligation then fails, which P4 must
// answer by inlining (ADR 0102).
var PlantNarrowNumeral bool

// CallSetError is a call whose argument is not in its parameter's set; Entry
// inlines that instance instead (ADR 0102, P4).
type CallSetError struct {
	Callee string // the program function's name, nat.f
	Msg    string
}

func (e *CallSetError) Error() string { return e.Msg }

// instance is the K(ℕ*) instance a program function realizes.
func (e *CallSetError) instance() string { return strings.TrimPrefix(e.Callee, natPrefix) }

// CheckCallSets is the obligation at each call of a program function: every
// table argument's element fact lies in its parameter's declared element set,
// which the callee was decided under (ir.md §2.5). A fact outside it is a
// refusal, never an assumption.
func CheckCallSets(tg *emit.Target, f *Func) error {
	if PlantNoCallSets {
		return nil
	}
	fs := analyse(tg, f)
	var err error
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if err != nil || s.Op != OCall || !tg.Prims[s.Name].Program {
				continue
			}
			p := tg.Prims[s.Name]
			for j, a := range s.Args {
				if j >= len(p.Args) {
					continue
				}
				x := fs[a]
				// A SCALAR's fact in its parameter's range
				if lo, hi, ok := core.IntRange(p.Args[j]); ok {
					if !x.v.within(ei(lo), ei(hi)) {
						err = &CallSetError{s.Name, fmt.Sprintf("argument %d of %s is %s, not inside its parameter's %d..%d "+
							"(ADR 0101: a program function is decided under its parameters' sets)", j+1, s.Name, termShow(x.v), lo, hi)}
						return
					}
					continue
				}
				// A TABLE's elements in its parameter's element range
				lo, hi, ok := core.IntRange(ElemOf(tg, unbuffer(p.Args[j])))
				if !ok {
					continue
				}
				if !x.hasEl || !x.el.within(ei(lo), ei(hi)) {
					err = &CallSetError{s.Name, fmt.Sprintf("argument %d of %s is a table whose elements are %s, not inside its parameter's %d..%d "+
						"(ADR 0101: a program function is decided under its parameters' sets)", j+1, s.Name, termShow(x.el), lo, hi)}
					return
				}
			}
		}
	})
	return err
}
