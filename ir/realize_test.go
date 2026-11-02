package ir

import (
	"fmt"
	"math/big"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// THE INTERPRETATION AGREES WITH THE LANGUAGE (ADR 0089): for each operation
// windows-llvm realizes by a term, ⟦ι(f)⟧(x) = f(x) at every sample x of f's
// domain, and every operation of the term stays inside its own domain there:
// its result in the word, and a host operation's count, divisor and dividend
// where LLVM defines it. The samples are the domain's edges, where a term is
// most often wrong: the word's ends, 0, ±1, and the counts around 63.

func llvmTarget(t *testing.T) *emit.Target {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows-llvm", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

var (
	wordMin = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 63))
	wordMax = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(1))
)

func inWord(x *big.Int) bool { return x.Cmp(wordMin) >= 0 && x.Cmp(wordMax) <= 0 }

// domainCheck is the hook: an integer operation of the term outside the word,
// or a host operation outside LLVM's domain for it, is an error.
func domainCheck(s *Stmt, env []cval) {
	if !(s.Op.IsArith()) || len(s.Res) != 1 {
		return
	}
	if r := env[s.Res[0]].i; r != nil && !inWord(r) {
		panic(fmt.Errorf("%s leaves the word: %s", s.Op, r))
	}
	if s.Via == "" {
		return
	}
	a, b := env[s.Args[0]].i, big.NewInt(0)
	if len(s.Args) > 1 {
		b = env[s.Args[1]].i
	}
	switch s.Op {
	case OShl, OShr:
		if b.Sign() < 0 || b.Cmp(big.NewInt(63)) > 0 {
			panic(fmt.Errorf("%s by %s: poison in LLVM", s.Via, b))
		}
	case ODiv, ORem:
		if b.Sign() == 0 || a.Cmp(wordMin) == 0 && b.Cmp(big.NewInt(-1)) == 0 {
			panic(fmt.Errorf("%s of %s by %s: undefined in LLVM", s.Via, a, b))
		}
	}
}

func runChecked(f *Func, args []cval) (cval, error) {
	in := &interp{f: f, env: make([]cval, f.NV()), seen: map[V]*obs{}, after: domainCheck}
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
		if ex := in.region(f.Body); len(ex.args) > 0 {
			out = ex.args[0]
		}
	}()
	return out, err
}

// spec is a language operation: its domain on the word and its value in ℤ.
type spec struct {
	op     string
	arity  int
	domain func(a, b *big.Int) bool
	value  func(a, b *big.Int) *big.Int
}

var termSpecs = []spec{
	{"<<", 2, func(a, b *big.Int) bool {
		return b.Sign() >= 0 && (a.Sign() == 0 || b.Cmp(big.NewInt(64)) < 0 && inWord(new(big.Int).Lsh(a, uint(b.Int64()))))
	}, func(a, b *big.Int) *big.Int {
		if a.Sign() == 0 {
			return big.NewInt(0)
		}
		return new(big.Int).Lsh(a, uint(b.Int64()))
	}},
	{">>", 2, func(a, b *big.Int) bool { return b.Sign() >= 0 }, func(a, b *big.Int) *big.Int {
		if b.Cmp(big.NewInt(1000)) > 0 {
			b = big.NewInt(1000)
		}
		return new(big.Int).Rsh(a, uint(b.Int64()))
	}},
	{"/", 2, func(a, b *big.Int) bool { return b.Sign() != 0 && inWord(new(big.Int).Quo(a, b)) },
		func(a, b *big.Int) *big.Int { return new(big.Int).Quo(a, b) }},
	{"%", 2, func(a, b *big.Int) bool { return b.Sign() != 0 },
		func(a, b *big.Int) *big.Int { return new(big.Int).Rem(a, b) }},
	{"mod", 2, func(a, b *big.Int) bool { return b.Sign() > 0 },
		func(a, b *big.Int) *big.Int { return new(big.Int).Mod(a, b) }},
	{"~", 1, func(a, b *big.Int) bool { return true },
		func(a, b *big.Int) *big.Int { return new(big.Int).Not(a) }},
}

func edgeSamples() []*big.Int {
	out := []*big.Int{wordMin, new(big.Int).Add(wordMin, big.NewInt(1)), wordMax, new(big.Int).Sub(wordMax, big.NewInt(1))}
	for _, x := range []int64{-1000, -65, -64, -63, -7, -2, -1, 0, 1, 2, 3, 7, 10, 62, 63, 64, 65, 100, 1000, 1 << 32} {
		out = append(out, big.NewInt(x))
	}
	return out
}

// agrees checks one term against its operation, answering the first
// disagreement or domain error.
func agrees(c *Func, sp spec) error {
	for _, a := range edgeSamples() {
		bs := edgeSamples()
		if sp.arity == 1 {
			bs = bs[:1]
		}
		for _, b := range bs {
			if !sp.domain(a, b) {
				continue
			}
			args := []cval{{i: a}}
			if sp.arity == 2 {
				args = append(args, cval{i: b})
			}
			got, err := runChecked(c, args)
			if err != nil {
				return fmt.Errorf("%s at (%s, %s): %v", sp.op, a, b, err)
			}
			if want := sp.value(a, b); got.i == nil || got.i.Cmp(want) != 0 {
				return fmt.Errorf("%s at (%s, %s): the term gives %v, ℤ gives %s", sp.op, a, b, got.i, want)
			}
		}
	}
	return nil
}

func TestTheInterpretationAgreesWithTheLanguage(t *testing.T) {
	tg := llvmTarget(t)
	for _, sp := range termSpecs {
		names, _, ok := tg.RealizedNames(sp.op)
		if !ok || !tg.IsDefinition(names[0]) {
			t.Errorf("%s is not realized by a term on windows-llvm", sp.op)
			continue
		}
		c, err := Instance(tg, names[0])
		if err != nil {
			t.Errorf("%s: %v", sp.op, err)
			continue
		}
		if err := agrees(c, sp); err != nil {
			t.Error(err)
		}
	}
}

// THE PLANTED FAULT: `<<`'s guard at 64, so LLVM's shl is used one count past
// its domain. The check must refuse it.
func TestATermPastItsHostDomainIsCaught(t *testing.T) {
	tg := llvmTarget(t)
	names, _, _ := tg.RealizedNames("<<")
	c, err := Instance(tg, names[0])
	if err != nil {
		t.Fatal(err)
	}
	bad := c.Clone()
	planted := false
	bad.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OConst && s.Lit.Kind == core.KInt && s.Lit.Int == 63 {
				s.Lit = core.Int(64)
				planted = true
			}
		}
	})
	if !planted {
		t.Fatal("the term has no 63 to plant the fault in")
	}
	if err := agrees(bad, termSpecs[0]); err == nil {
		t.Error("a guard at 64 passed: the check cannot see a term past its host's domain")
	}
}
