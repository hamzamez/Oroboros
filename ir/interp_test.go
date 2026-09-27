package ir

import (
	"fmt"
	"math/big"

	"oroboros/core"
	"oroboros/emit"
)

// A CONCRETE INTERPRETER OF THE IR, for testing the analyses against
// executions. Integers are ℤ (big.Int), never a word: a fact claims to
// contain the mathematical value (spec §5), so a value past int64 is a true
// integer the fact must contain by an infinite end. It runs the fragment the
// loop rules are about (arithmetic, comparisons, tables and buffers, `if`,
// `loop`, `build`, `tabulate`) and reports errUnsupported for the rest.
//
// Every definition of a value is RECORDED, a buffer as a snapshot of its cells
// when it is defined, since `set` updates the one buffer in place (linearity:
// the old name is never read again).

type cval struct {
	i   *big.Int
	b   bool
	arr []*big.Int // a table or a buffer
	tab bool
}

type obs struct {
	ints  []*big.Int
	elems []*big.Int
	lens  []int
}

type interp struct {
	f     *Func
	env   []cval
	seen  map[V]*obs
	steps int
}

var errUnsupported = fmt.Errorf("unsupported")

type exit struct {
	t    Term
	args []cval
}

func runFunc(f *Func, args []cval) (map[V]*obs, error) {
	in := &interp{f: f, env: make([]cval, f.NV()), seen: map[V]*obs{}}
	for i, p := range f.Params {
		in.bind(p, args[i])
	}
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
					return
				}
				err = fmt.Errorf("%v", r)
			}
		}()
		in.region(f.Body)
	}()
	return in.seen, err
}

func (in *interp) bind(v V, x cval) {
	in.env[v] = x
	o := in.seen[v]
	if o == nil {
		o = &obs{}
		in.seen[v] = o
	}
	switch {
	case x.tab:
		o.lens = append(o.lens, len(x.arr))
		for _, e := range x.arr {
			o.elems = append(o.elems, new(big.Int).Set(e))
		}
	case x.i != nil:
		o.ints = append(o.ints, x.i)
	}
}

func (in *interp) region(r *Region) exit {
	if in.steps++; in.steps > 200000 {
		panic(fmt.Errorf("step budget"))
	}
	for _, pi := range r.Pis {
		in.bind(pi.V, in.env[pi.Of])
	}
	for i := range r.Stmts {
		in.stmt(&r.Stmts[i])
	}
	switch r.T {
	case TBranch:
		if in.env[r.Cond].b {
			return in.region(r.Then)
		}
		return in.region(r.Else)
	default:
		out := exit{t: r.T}
		for _, a := range r.Args {
			out.args = append(out.args, in.env[a])
		}
		return out
	}
}

func intv(x int64) cval { return cval{i: big.NewInt(x)} }

func (in *interp) stmt(s *Stmt) {
	a := func(k int) cval { return in.env[s.Args[k]] }
	one := func(x cval) { in.bind(s.Res[0], x) }
	switch s.Op {
	case OConst:
		switch s.Lit.Kind {
		case core.KBool:
			one(cval{b: s.Lit.Int == 1})
		case core.KInt:
			one(intv(s.Lit.Int))
		default:
			panic(errUnsupported)
		}
	case OAdd:
		one(cval{i: new(big.Int).Add(a(0).i, a(1).i)})
	case OSub:
		one(cval{i: new(big.Int).Sub(a(0).i, a(1).i)})
	case OMul:
		one(cval{i: new(big.Int).Mul(a(0).i, a(1).i)})
	case ONeg:
		one(cval{i: new(big.Int).Neg(a(0).i)})
	case ODiv, ORem:
		if a(1).i.Sign() == 0 {
			panic(fmt.Errorf("division by zero"))
		}
		if s.Op == ODiv {
			one(cval{i: new(big.Int).Quo(a(0).i, a(1).i)})
		} else {
			one(cval{i: new(big.Int).Rem(a(0).i, a(1).i)})
		}
	case OEq, ONe, OLt, OLe, OGt, OGe:
		var c int
		if a(0).i == nil {
			if a(0).b == a(1).b {
				c = 0
			} else {
				c = 1
			}
		} else {
			c = a(0).i.Cmp(a(1).i)
		}
		r := map[Op]bool{OEq: c == 0, ONe: c != 0, OLt: c < 0, OLe: c <= 0, OGt: c > 0, OGe: c >= 0}[s.Op]
		one(cval{b: r})
	case OIndex:
		t, k := a(0), a(1).i
		if !k.IsInt64() || k.Int64() < 0 || k.Int64() >= int64(len(t.arr)) {
			panic(fmt.Errorf("index %s out of [0, %d)", k, len(t.arr)))
		}
		one(cval{i: t.arr[k.Int64()]})
	case OLen:
		one(intv(int64(len(a(0).arr))))
	case OArray:
		out := cval{tab: true}
		for k := range s.Args {
			out.arr = append(out.arr, a(k).i)
		}
		one(out)
	case OSet:
		t, k := a(0), a(1).i
		if !k.IsInt64() || k.Int64() < 0 || k.Int64() >= int64(len(t.arr)) {
			panic(fmt.Errorf("store %s out of [0, %d)", k, len(t.arr)))
		}
		t.arr[k.Int64()] = a(2).i
		one(t)
	case OThe:
		one(a(0))
	case ORequire, OAssume:
	case ORestrict:
		t, n := a(0), a(1).i
		if n.IsInt64() && n.Int64() >= 0 && n.Int64() < int64(len(t.arr)) {
			t.arr = t.arr[:n.Int64()]
		}
		one(t)
	case OIf:
		arm := s.Sub[1]
		if a(0).b {
			arm = s.Sub[0]
		}
		ex := in.region(arm)
		for k, v := range s.Res {
			in.bind(v, ex.args[k])
		}
	case OLoop:
		body := s.Sub[0]
		cur := make([]cval, len(s.Args))
		for k := range s.Args {
			cur[k] = a(k)
		}
		for {
			for k, p := range body.Params {
				in.bind(p, cur[k])
			}
			ex := in.region(body)
			if ex.t == TBreak {
				for k, v := range s.Res {
					in.bind(v, ex.args[k])
				}
				return
			}
			cur = ex.args
		}
	case OBuild:
		n := a(0).i
		if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 1<<20 {
			panic(fmt.Errorf("build of %s", n))
		}
		buf := cval{tab: true, arr: make([]*big.Int, n.Int64())}
		for k := range buf.arr {
			buf.arr[k] = new(big.Int)
		}
		body := s.Sub[0]
		in.bind(body.Params[0], buf)
		ex := in.region(body)
		for k, v := range s.Res {
			in.bind(v, ex.args[k])
		}
	case OTabulate:
		n := a(0).i
		if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 1<<20 {
			panic(fmt.Errorf("tabulate of %s", n))
		}
		out := cval{tab: true}
		body := s.Sub[0]
		for k := int64(0); k < n.Int64(); k++ {
			in.bind(body.Params[0], intv(k))
			ex := in.region(body)
			out.arr = append(out.arr, ex.args[0].i)
		}
		one(out)
	case OCall:
		one(in.call(s, a))
	default:
		panic(errUnsupported)
	}
}

var (
	two64Big = new(big.Int).Lsh(big.NewInt(1), 64)
	two63Big = new(big.Int).Lsh(big.NewInt(1), 63)
)

// call runs the primitives whose meaning is the language's: the unsigned
// word's, as the residue map ℤ → ℤ/2⁶⁴ with representatives in U or S
// (targets/go/u64.oro), and the mask and the shift SelectShifts writes, as
// two's complement AND and the arithmetic right shift. Anything else is a
// host's, which the interpreter does not run.
func (in *interp) call(s *Stmt, a func(int) cval) cval {
	rU := func(x *big.Int) *big.Int { return new(big.Int).Mod(x, two64Big) }
	rS := func(x *big.Int) *big.Int {
		y := rU(x)
		if y.Cmp(two63Big) >= 0 {
			y.Sub(y, two64Big)
		}
		return y
	}
	// ARBITRARY PRECISION is ℤ, and an object: a value is a *big.Int that
	// π-renaming, binding and `big-fit` share, and a destination form writes
	// INTO its receiver, as math/big does (d.Mul(a, b)). So a destination whose
	// receiver is read later gives a different answer here, the way it would
	// on Go.
	bigOp := map[string]func(z, x, y *big.Int) *big.Int{
		"+": (*big.Int).Add, "-": (*big.Int).Sub, "*": (*big.Int).Mul,
		"/": (*big.Int).Quo, "%": (*big.Int).Rem,
	}
	if len(s.Name) > 3 && s.Name[:3] == "big" {
		name := s.Name[3:]
		if op, ok := bigOp[name]; ok && len(s.Args) == 2 {
			if (name == "/" || name == "%") && a(1).i.Sign() == 0 {
				panic(fmt.Errorf("division by zero"))
			}
			return cval{i: op(new(big.Int), a(0).i, a(1).i)}
		}
		if n := len(name); n > 1 && name[n-1] == '!' && len(s.Args) == 3 {
			if op, ok := bigOp[name[:n-1]]; ok {
				if (name[:n-1] == "/" || name[:n-1] == "%") && a(2).i.Sign() == 0 {
					panic(fmt.Errorf("division by zero"))
				}
				d := a(0).i
				op(d, a(1).i, a(2).i)
				return cval{i: d}
			}
		}
		switch name {
		case "-of":
			return cval{i: new(big.Int).Set(a(0).i)}
		case "%-small":
			if a(1).i.Sign() == 0 {
				panic(fmt.Errorf("division by zero"))
			}
			return cval{i: new(big.Int).Rem(a(0).i, a(1).i)}
		case "<", "<=", ">", ">=", "=":
			c := a(0).i.Cmp(a(1).i)
			return cval{b: map[string]bool{"<": c < 0, "<=": c <= 0, ">": c > 0, ">=": c >= 0, "=": c == 0}[name]}
		case "-fit", "-fit-signed":
			x, k := a(0).i, uint(a(1).i.Int64())
			if x.BitLen() > int(k) || (name == "-fit" && x.Sign() < 0) {
				panic(fmt.Errorf("bignum overflow: the declared range is too small"))
			}
			return cval{i: x}
		}
	}
	// THE TRAP a limb instance's guard raises when a value leaves the set.
	if s.Name == "trap-if" && len(s.Args) == 1 {
		if a(0).i.Sign() != 0 {
			panic(fmt.Errorf("trap: a value left the declared set"))
		}
		return cval{i: new(big.Int)}
	}
	switch s.Name {
	case "u64-of":
		return cval{i: rU(a(0).i)}
	case "int-of-u64":
		return cval{i: rS(a(0).i)}
	case "u64+":
		return cval{i: rU(new(big.Int).Add(a(0).i, a(1).i))}
	case "u64-":
		return cval{i: rU(new(big.Int).Sub(a(0).i, a(1).i))}
	case "u64*":
		return cval{i: rU(new(big.Int).Mul(a(0).i, a(1).i))}
	case "u64/", "u64%":
		if a(1).i.Sign() == 0 {
			panic(fmt.Errorf("division by zero"))
		}
		if s.Name == "u64/" {
			return cval{i: new(big.Int).Quo(a(0).i, a(1).i)}
		}
		return cval{i: new(big.Int).Rem(a(0).i, a(1).i)}
	case "u64<", "u64<=", "u64>", "u64>=", "u64=":
		c := a(0).i.Cmp(a(1).i)
		return cval{b: map[string]bool{"u64<": c < 0, "u64<=": c <= 0, "u64>": c > 0, "u64>=": c >= 0, "u64=": c == 0}[s.Name]}
	}
	if len(s.Args) == 2 {
		switch emit.ArithOp(s.Name, 2) {
		case "and":
			return cval{i: new(big.Int).And(a(0).i, a(1).i)}
		case "shr":
			return cval{i: new(big.Int).Rsh(a(0).i, uint(a(1).i.Int64()))}
		}
	}
	panic(errUnsupported)
}

// contained checks every recorded value of f against its fact, and returns
// the first value outside it.
func contained(f *Func, fs []fact, seen map[V]*obs) error {
	for v, o := range seen {
		fv := fs[v]
		for _, x := range o.ints {
			if !member(x, fv.v) {
				return fmt.Errorf("%%%d = %s, outside its fact %s", v, x, show(fv.v))
			}
		}
		if fv.hasEl {
			for _, x := range o.elems {
				if !member(x, fv.el) {
					return fmt.Errorf("%%%d holds %s, outside its element fact %s", v, x, show(fv.el))
				}
			}
		}
		if fv.hasLn {
			for _, n := range o.lens {
				if !member(big.NewInt(int64(n)), fv.ln) {
					return fmt.Errorf("%%%d has length %d, outside %s", v, n, show(fv.ln))
				}
			}
		}
	}
	return nil
}

// runResult runs f and returns its (first) result, whatever region shape it
// ends in.
func runResult(f *Func, args []cval) (cval, error) {
	in := &interp{f: f, env: make([]cval, f.NV()), seen: map[V]*obs{}}
	for i, p := range f.Params {
		in.bind(p, args[i])
	}
	var out cval
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = e
					return
				}
				err = fmt.Errorf("%v", r)
			}
		}()
		ex := in.region(f.Body)
		if len(ex.args) > 0 {
			out = ex.args[0]
		}
	}()
	return out, err
}
