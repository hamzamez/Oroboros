package ir

import (
	"fmt"
	"strconv"
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
// sets, a numeral's table `array int 0 18446744073709551615` (its header is 2m + s,
// digitfacts-2026-11-10), and its result's element range is what its stores
// prove. Ownership across the call is the frame rule: the arguments borrowed,
// the result a birth (W13 checks the callee's side).
//
// An instance depends on the target alone (max_len is its one specialization),
// so its function is built once per target and named for the instance, and a
// program's text holds each it calls: every printer reprints from the text
// (ADR 0038), registering the functions it finds there first.

// NumeralType is a numeral's table as a program function's parameter and
// result on tg: its digits in [0, B), B = 2^dbits the target's digit (ADR
// 0124), so [0, 2⁶⁴) on x86_64 and [0, 2³²) on i686, inside the signed word.
func NumeralType(tg *emit.Target) string {
	if bits, err := tg.DigitBits(); err == nil && bits < 64 {
		return fmt.Sprintf("array int 0 %d", uint64(1)<<bits-1)
	}
	return "array int 0 18446744073709551615"
}

// digitMax is the largest digit on tg, B − 1.
func digitMax(tg *emit.Target) ep {
	if bits, err := tg.DigitBits(); err == nil && bits < 64 {
		return ei(1<<bits - 1)
	}
	return u64Max
}

// natPrefix names K(ℕ*)'s program functions.
const natPrefix = "nat."

// TokSuffix names an instance's token variant (ADR 0125): `nat.mul-small~tok`.
const TokSuffix = "~tok"

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
var natCalled = map[string]bool{"add": true, "sub": true, "mul": true, "sqr": true, "mul-small": true,
	"div": true, "rem": true, "render": true,
	// ADR 0082's operations (natnum.oro's bits), each a numeral built by
	// loops over its operands, rem's Algorithm D inside bmod
	"band": true, "bor": true, "bxor": true, "bnot": true, "bshl": true, "bshr": true, "bmod": true}

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
	// A TOKEN VARIANT, f~tok (ADR 0125): f's parameters, then a dead region and
	// its element count, which f's result's storage takes by fit
	fn, variant := strings.CutSuffix(fn, TokSuffix)
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
			sig.Params[i].Type = NumeralType(tg)
		// THE WORD OPERAND'S SET is the one the plan chose the one-pass form for
		// (limbs.go): a product's any digit, below B, a divisor's [1, 2²⁸)
		case sig.Params[i].Type == "int" && fn == "mul-small":
			sig.Params[i].Type = strings.Replace(NumeralType(tg), "array int", "int", 1)
		case sig.Params[i].Type == "int" && fn == "div-small":
			sig.Params[i].Type = fmt.Sprintf("int 1 %d", 1<<28-1)
		}
	}
	term := inst.Term
	if variant {
		sig.Params = append(sig.Params, core.SigParam{Name: "#tok", Type: "ptr"},
			core.SigParam{Name: "#ktok", Type: fmt.Sprintf("int 0 %d", tg.MaxLenOf())})
		term = core.FnClosed(append(append([]string(nil), inst.Term.Params...), "#tok", "#ktok"), inst.Term.Closed())
	}
	f, err := Lower(tg, name, &sig, term, Options{Decided: true})
	if err != nil {
		return nil, fmt.Errorf("the built-in K(ℕ*)'s %s as a function: %v", fn, err)
	}
	if variant {
		f.ResultTok = append([]V(nil), f.Params[len(f.Params)-2:]...)
	}
	leg := Decide(tg, f, false)
	if leg, err = afterRung(tg, f, name, &sig, false, leg, func(Note) {}); err != nil {
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
		if err := proveResultElems(tg, f, ei(0), u64Max); err != nil {
			return nil, fmt.Errorf("the built-in K(ℕ*)'s %s: %v", fn, err)
		}
		f.Results[0] = NumeralType(tg)
	}
	SelectShifts(tg, f)
	return f, nil
}

// PlantResultUnproven declares a program function's result a numeral's
// without asking its body.
var PlantResultUnproven bool

// proveResultElems requires every table f returns to have its elements in
// [lo, hi] by f's own facts.
func proveResultElems(tg *emit.Target, f *Func, lo, hi ep) error {
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
			if x := fs[a]; !x.hasEl || !x.el.within(lo, hi) {
				err = fmt.Errorf("its result's elements are %s, not inside %v..%v, which its callers would read", termShow(x.el), lo, hi)
				return
			}
		}
	})
	return err
}

// registerProgramFunc names f among the target's primitives: its signature
// its own, pure, and a call's spelling on LLVM.
func registerProgramFunc(tg *emit.Target, f *Func) {
	// K(ℕ*)'s instances are the target's, one per name; a kept definition's
	// name is its program's, so a registration replaces another program's
	// (cmd/check reprints every program's text on one target)
	if q, ok := tg.Prims[f.Name]; ok && q.Program && strings.HasPrefix(f.Name, natPrefix) {
		return
	}
	var args, holes []string
	for i, p := range f.Params {
		args = append(args, f.Types[p])
		holes = append(holes, fmt.Sprintf("%%%d", i+1))
	}
	res := ""
	var results []string
	ret := "void"
	switch len(f.Results) {
	case 0:
	case 1:
		res = f.Results[0]
		ret = llvmType(res)
	default:
		// SEVERAL RESULTS are the product's factors (ADR 0113, Proposition C): an
		// aggregate the call's projections take apart (ir/llvm's call)
		results = append(results, f.Results...)
		var ts []string
		for _, r := range f.Results {
			ts = append(ts, llvmType(r))
		}
		ret = "{ " + strings.Join(ts, ", ") + " }"
	}
	// A TOKEN VARIANT RELEASES OR REUSES MEMORY (ADR 0125 item 5), so its call
	// keeps its place among its caller's effects
	pure := (strings.HasPrefix(f.Name, natPrefix) || funcPure(tg, f)) && len(f.ResultTok) == 0
	tg.Prims[f.Name] = emit.Prim{Name: f.Name, Args: args, Result: res, Results: results, Kind: "expr",
		Pure: pure, Program: true,
		Form: fmt.Sprintf("call %s %s(%s)", ret, llvmName(f.Name), strings.Join(holes, ", "))}
	programFuncs[f.Name] = true
}

// funcPure says f's body calls nothing impure: a call of it is then pure, and
// otherwise it keeps its place among its caller's effects (ADR 0010, 0113).
func funcPure(tg *emit.Target, f *Func) bool {
	if PlantLentPure {
		goto calls
	}
	// A LENT BUFFER IS WRITTEN: B ⊸ B ⊗ R is an effect on it, so a caller's
	// facts of its contents must take the callee's writes (hostWrites joins a
	// buffer argument's declared elements at an impure call), and a call that
	// lends one is never pure, whatever else its body does (ADR 0113).
	for _, p := range f.Params {
		if strings.HasPrefix(f.Types[p], "buffer") {
			return false
		}
	}
calls:
	pure := true
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall {
				if q, ok := tg.Prims[s.Name]; !ok || !q.Pure {
					pure = false
				}
			}
		}
	})
	return pure
}

// PlantLentPure lets a call that lends a buffer be pure when its body calls
// nothing impure: TestALentBufferIsWrittenByItsCall must then see the caller
// keep its buffer's element facts across the write and accept an index the
// callee's write has put out of range.
var PlantLentPure bool

// programFuncs are the program functions registered on any target: K(ℕ*)'s
// instances and the definitions a program keeps (ADR 0113).
var programFuncs = map[string]bool{}

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

// llvmName is ir/llvm.Name, here because ir/llvm imports ir: `-` made `_`,
// and quoted where the name holds anything else (a qualified definition's
// `os/host.ReadFile`, ADR 0113).
func llvmName(n string) string {
	n = strings.ReplaceAll(n, "-", "_")
	for _, c := range n {
		if !(c == '_' || c == '.' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return "@" + strconv.Quote(n)
		}
	}
	return "@" + n
}

// IsProgramFunc says name is a program function's: K(ℕ*)'s, or a kept
// definition's (ADR 0113).
func IsProgramFunc(name string) bool { return strings.HasPrefix(name, natPrefix) || programFuncs[name] }

// Callees are the program functions f calls, built for tg, in name order.
func Callees(tg *emit.Target, f *Func) []*Func {
	// TRANSITIVELY: a kept definition may call another (ADR 0113), each defined
	// once in the module
	calleeMu.Lock()
	defer calleeMu.Unlock()
	byName := map[string]*Func{}
	for k, c := range callees {
		if k.tg == tg {
			byName[c.Name] = c
		}
	}
	seen := map[string]bool{}
	var out []*Func
	var visit func(g *Func)
	visit = func(g *Func) {
		g.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == OCall && IsProgramFunc(s.Name) && !seen[s.Name] {
					if c, ok := byName[s.Name]; ok {
						seen[s.Name] = true
						out = append(out, c)
						visit(c)
					}
				}
				// A CALLBACK'S ADDRESS defines the function (ADR 0133): the
				// host calls it, so no call of ours names it
				if t, ok := addrOf[s.Name]; s.Op == OCall && ok && !seen[t] {
					if c, ok := byName[t]; ok {
						seen[t] = true
						out = append(out, c)
						visit(c)
					}
				}
			}
		})
	}
	visit(f)
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
	// A FUNCTION ANOTHER FUNCTION OF THE TEXT CALLS is a program function: K(ℕ*)'s
	// instances by their prefix, a kept definition by being called (ADR 0113).
	called := map[string]bool{}
	for _, f := range p.Funcs {
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == OCall {
					called[s.Name] = true
				}
			}
		})
	}
	for _, f := range p.Funcs {
		if IsProgramFunc(f.Name) || called[f.Name] || f.Callback != "" {
			registerProgramFunc(tg, f)
		}
		if f.Callback != "" {
			registerCallbackAddr(tg, f)
		}
	}
	// A CALLBACK'S STATE, ITS SLOT'S LEAVES (ADR 0133): each load and store the
	// text calls, its set the call's own type
	for _, f := range p.Funcs {
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op != OCall {
					continue
				}
				if _, ok := tg.Prims[s.Name]; ok {
					continue
				}
				n, k, op, ok := slotLeafOf(s.Name)
				if !ok {
					continue
				}
				ty := ""
				switch {
				case op == "get" && len(s.Res) == 1:
					ty = f.Types[s.Res[0]]
				case op == "set" && len(s.Args) == 1:
					ty = f.Types[s.Args[0]]
				}
				if ty == "u64" {
					ty = "int 0 18446744073709551615"
				}
				_ = tg.RegisterSlotLeaf(n, k, ty)
			}
		})
	}
}

// slotLeafOf reads a slot leaf's primitive name, `n~k~get` or `n~k~set`.
func slotLeafOf(name string) (string, int, string, bool) {
	parts := strings.Split(name, "~")
	if len(parts) < 3 || !strings.HasPrefix(parts[len(parts)-3], "slot") {
		return "", 0, "", false
	}
	op := parts[len(parts)-1]
	k, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil || op != "get" && op != "set" {
		return "", 0, "", false
	}
	return strings.Join(parts[:len(parts)-2], "~"), k, op, true
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

// setEndsExact is a set's ends as the decision's, exact past int64: a range of
// any size, or the unsigned word, [0, 2⁶⁴) (a base-2⁶⁴ digit's set, ADR 0105).
func setEndsExact(tg *emit.Target, ty string) (ep, ep, bool) {
	if ty == "u64" || tg.ValueType(ty) == core.U64Type {
		return ei(0), u64Max, true
	}
	l, h, ok := core.IntRangeBig(ty)
	if !ok || l == nil || h == nil {
		return ep{}, ep{}, false
	}
	lo, ok1 := fromBig(l)
	hi, ok2 := fromBig(h)
	return lo, hi, ok1 && ok2
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
// instance is the instance a failing call names, a token variant's being its
// base's (ADR 0125): P4 inlines the base, and the variant goes with it
func (e *CallSetError) instance() string {
	return strings.TrimSuffix(strings.TrimPrefix(e.Callee, natPrefix), TokSuffix)
}

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
				if lo, hi, ok := setEndsExact(tg, p.Args[j]); ok {
					if !x.v.within(lo, hi) {
						err = &CallSetError{s.Name, fmt.Sprintf("argument %d of %s is %s, not inside its parameter's %v..%v "+
							"(ADR 0101: a program function is decided under its parameters' sets)", j+1, s.Name, termShow(x.v), lo, hi)}
						return
					}
					continue
				}
				// A TABLE's elements in its parameter's element range
				lo, hi, ok := setEndsExact(tg, ElemOf(tg, unbuffer(p.Args[j])))
				if !ok {
					continue
				}
				if !x.hasEl || !x.el.within(lo, hi) {
					err = &CallSetError{s.Name, fmt.Sprintf("argument %d of %s is a table whose elements are %s, not inside its parameter's %v..%v "+
						"(ADR 0101: a program function is decided under its parameters' sets)", j+1, s.Name, termShow(x.el), lo, hi)}
					return
				}
			}
		}
	})
	return err
}

// PlantNumeralResultInt declares a K(ℕ*) result held exactly `array int`, as
// before gauntlet-2026-11-16's harness: a second fixed representation in its
// class, so W5 refuses examples/big's fib on windows again.
var PlantNumeralResultInt bool
