package ir

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// THE INTERPRETATION AGREES WITH THE LANGUAGE (ADR 0089): for each operation
// windows realizes by a term, ⟦ι(f)⟧(x) = f(x) at every sample x of f's
// domain, and every operation of the term stays inside its own domain there:
// its result in the word, and a host operation's count, divisor and dividend
// where LLVM defines it. The samples are the domain's edges, where a term is
// most often wrong: the word's ends, 0, ±1, and the counts around 63.

func llvmTarget(t *testing.T) *emit.Target {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
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
		// udiv and urem read the bits unsigned: ℤ's `/` and `%` only on ℕ, a
		// positive divisor (instructions.oro)
		if (s.Via == "llvm.udiv-nat" || s.Via == "llvm.urem-nat") && (a.Sign() < 0 || b.Sign() <= 0) {
			panic(fmt.Errorf("%s of %s by %s: unsigned, not ℤ's, outside ℕ", s.Via, a, b))
		}
	}
}

// ON ℕ, `/` AND `%` ARE `udiv` AND `urem` (instructions.oro): chosen where both
// operands are proven in ℕ, a positive divisor, and nowhere else. A possibly
// negative dividend keeps `sdiv`, which truncates as ℤ does. THE PLANTED FAULT
// chooses a realization without asking its sets, and `udiv` meets −7.
func TestADivisionInNIsUnsignedAndOnlyThere(t *testing.T) {
	via := func(src string) (*Func, map[string]bool) {
		tg, f := lowerOn(t, src)
		Decide(tg, f, false)
		if _, err := RealizeOps(tg, f); err != nil {
			t.Fatal(err)
		}
		vs := map[string]bool{}
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == ODiv || s.Op == ORem {
					vs[s.Via] = true
				}
			}
		})
		return f, vs
	}
	const nat = "export fn f(x ∈ 0..1000, k ∈ 1..9) (Int) { x / k + x % k }\n"
	const signed = "export fn f(x ∈ -1000..1000, k ∈ 1..9) (Int) { x / k + x % k }\n"
	if _, vs := via(nat); !vs["llvm.udiv-nat"] || !vs["llvm.urem-nat"] {
		t.Errorf("on ℕ: want udiv and urem, got %v", vs)
	}
	check := func() error {
		f, _ := via(signed)
		for _, x := range []int64{-1000, -7, -1, 0, 7, 1000} {
			for _, k := range []int64{1, 2, 9} {
				got, err := runChecked(f, []cval{{i: big.NewInt(x)}, {i: big.NewInt(k)}})
				if err != nil {
					return err
				}
				want := new(big.Int).Add(new(big.Int).Quo(big.NewInt(x), big.NewInt(k)), new(big.Int).Rem(big.NewInt(x), big.NewInt(k)))
				if got.i.Cmp(want) != 0 {
					return fmt.Errorf("at (%d, %d): %s, ℤ gives %s", x, k, got.i, want)
				}
			}
		}
		return nil
	}
	if _, vs := via(signed); vs["llvm.udiv-nat"] || vs["llvm.urem-nat"] {
		t.Errorf("a possibly negative dividend: want sdiv and srem, got %v", vs)
	}
	if err := check(); err != nil {
		t.Errorf("a possibly negative dividend: %v", err)
	}
	PlantRealizeUnasked = true
	defer func() { PlantRealizeUnasked = false }()
	if err := check(); err == nil || !strings.Contains(err.Error(), "outside ℕ") {
		t.Errorf("with the sets unasked: want udiv caught outside ℕ, got %v: the witness cannot fail", err)
	}
}

// errTrap is a call into ∅ reached: the trap a checked form answers outside
// the word.
var errTrap = fmt.Errorf("trap")

// residue is x's signed residue mod 2⁶⁴, the word's two's complement.
func residue(x *big.Int) *big.Int {
	m := new(big.Int).Lsh(big.NewInt(1), 64)
	r := new(big.Int).Mod(new(big.Int).Sub(x, wordMin), m)
	return r.Add(r, wordMin)
}

// hostMeaning is the stated meaning of the host operations the terms call
// that are calls on the IR: the overflow intrinsics, W × W → W × 𝔹, (a, b) ↦
// (the residue of ℤ's result, whether ℤ's result left the word); and the trap,
// into ∅.
func hostMeaning(name string, args []cval) ([]cval, bool) {
	op := map[string]func(a, b *big.Int) *big.Int{
		"llvm.sadd-with-overflow": func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) },
		"llvm.ssub-with-overflow": func(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) },
		"llvm.smul-with-overflow": func(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) },
	}
	if f, ok := op[name]; ok {
		r := f(args[0].i, args[1].i)
		return []cval{{i: residue(r)}, {b: !inWord(r)}}, true
	}
	if name == "llvm.trap" {
		panic(errTrap)
	}
	// Σ_OS's exit is a call into ∅ too (ADR 0090, 0093), the crash a term ends
	// with; on windows its view is ExitProcess, inlined. What the crash writes
	// first, the reason to standard error, is an effect this interpreter does
	// not observe: the encoding, the handle and the write are stated as values,
	// and only the non-return is checked.
	switch name {
	case "windows/processthreadsapi.ExitProcess":
		panic(errTrap)
	case "windows/fileapi.WriteFile":
		// the whole table s written (ADR 0132): len(s) through the
		// out-result's cell (ADR 0131), and a nonzero BOOL
		args[2].arr[0] = big.NewInt(int64(len(args[1].arr)))
		return []cval{{i: big.NewInt(1)}}, true
	case "windows/fileapi.WriteFile#seg":
		// a synchronous write of the segment b[lo:hi] that succeeds (ADR
		// 0132): hi − lo through the out-result's cell (ADR 0131), and a
		// nonzero BOOL
		args[4].arr[0] = new(big.Int).Sub(args[3].i, args[2].i)
		return []cval{{i: big.NewInt(1)}}, true
	case "windows/processenv.GetStdHandle", "llvm/str.utf8":
		return []cval{{}}, true
	}
	return nil, false
}

func runChecked(f *Func, args []cval) (cval, error) {
	in := &interp{f: f, env: make([]cval, f.NV()), seen: map[V]*obs{}, after: domainCheck, host: hostMeaning}
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
			t.Errorf("%s is not realized by a term on windows", sp.op)
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

// A CHECKED FORM AGREES WITH THE LANGUAGE WHERE f(x) IS IN THE WORD AND
// TRAPS EVERYWHERE ELSE (ADR 0019, 0090): the overflow intrinsic's flag, and the
// trap a call into ∅. trap-if is 0 at 0 and a trap elsewhere.
func TestACheckedFormTrapsExactlyOutsideTheWord(t *testing.T) {
	tg := llvmTarget(t)
	checked := []struct {
		op    string
		value func(a, b *big.Int) *big.Int
		pre   func(a, b *big.Int) bool
	}{
		{"+", func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) }, func(a, b *big.Int) bool { return true }},
		{"-", func(a, b *big.Int) *big.Int { return new(big.Int).Sub(a, b) }, func(a, b *big.Int) bool { return true }},
		{"*", func(a, b *big.Int) *big.Int { return new(big.Int).Mul(a, b) }, func(a, b *big.Int) bool { return true }},
		{"<<", func(a, b *big.Int) *big.Int {
			if a.Sign() == 0 {
				return big.NewInt(0)
			}
			if b.Cmp(big.NewInt(200)) > 0 {
				b = big.NewInt(200)
			}
			return new(big.Int).Lsh(a, uint(b.Int64()))
		}, func(a, b *big.Int) bool { return b.Sign() >= 0 }},
	}
	for _, c := range checked {
		_, def, ok := tg.RealizedNames(c.op)
		if !ok || !tg.IsDefinition(def) {
			t.Errorf("%s's checked form is not a term", c.op)
			continue
		}
		inst, err := Instance(tg, def)
		if err != nil {
			t.Errorf("%s: %v", c.op, err)
			continue
		}
		if err := trapsOutside(inst, c.op, c.value, c.pre); err != nil {
			t.Error(err)
		}
	}
	names, _, _ := tg.RealizedNames("trap-if")
	inst, err := Instance(tg, names[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range edgeSamples() {
		got, err := runChecked(inst, []cval{{i: a}})
		if a.Sign() == 0 && (err != nil || got.i == nil || got.i.Sign() != 0) || a.Sign() != 0 && err != errTrap {
			t.Errorf("trap-if at %s: %v, %v", a, got.i, err)
		}
	}
}

// trapsOutside checks a checked form: f(x) where it is in the word, a trap
// elsewhere, at every sample satisfying pre.
func trapsOutside(inst *Func, op string, value func(a, b *big.Int) *big.Int, pre func(a, b *big.Int) bool) error {
	for _, a := range edgeSamples() {
		for _, b := range edgeSamples() {
			if !pre(a, b) {
				continue
			}
			got, err := runChecked(inst, []cval{{i: a}, {i: b}})
			want := value(a, b)
			switch {
			case !inWord(want) && err != errTrap:
				return fmt.Errorf("%s at (%s, %s): ℤ gives %s, outside the word, and the term gave %v, %v", op, a, b, want, got.i, err)
			case inWord(want) && (err != nil || got.i == nil || got.i.Cmp(want) != 0):
				return fmt.Errorf("%s at (%s, %s): ℤ gives %s, the term %v, %v", op, a, b, want, got.i, err)
			}
		}
	}
	return nil
}

// THE PLANTED FAULT: the checked `+` with its arms swapped, a trap where the
// sum is in the word and the residue where it is not.
func TestACheckedFormTrappingTheWrongWayIsCaught(t *testing.T) {
	tg := llvmTarget(t)
	_, def, _ := tg.RealizedNames("+")
	inst, err := Instance(tg, def)
	if err != nil {
		t.Fatal(err)
	}
	bad := inst.Clone()
	swapped := false
	bad.Walk(func(r *Region) {
		if r.T == TBranch && !swapped {
			r.Then, r.Else = r.Else, r.Then
			swapped = true
		}
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OIf && !swapped {
				s.Sub[0], s.Sub[1] = s.Sub[1], s.Sub[0]
				swapped = true
			}
		}
	})
	if !swapped {
		t.Fatal("the checked + has no branch to plant the fault in")
	}
	add := func(a, b *big.Int) *big.Int { return new(big.Int).Add(a, b) }
	if trapsOutside(bad, "+", add, func(a, b *big.Int) bool { return true }) == nil {
		t.Error("a checked + trapping the wrong way passed: the check cannot see it")
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

// A TERM'S HOST `where` IS DECIDED, and V's SUBMONOID DECIDES `++`
// (strings-2026-11-16). lang.cat realizes the free monoid's operation as the
// rule x·y read back by ρ⁻¹ = llvm/str.text, whose `where` is utf8_valid: the
// refinement layer must decide it on the instance, and does, by V's being a
// submonoid of B*. THE PLANTED FAULTS: without the submonoid's case the
// instance is refused, its concatenation unproven; and without the refinement
// layer on the instance, the hole as it was, the same unproven term is
// inlined.
func TestATermsHostWhereIsDecided(t *testing.T) {
	load := func() *emit.Target {
		emit.ResetImports()
		tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	if _, err := Instance(load(), "lang.cat"); err != nil {
		t.Fatalf("lang.cat: %v", err)
	}
	emit.PlantCatValid = true
	defer func() { emit.PlantCatValid = false }()
	if _, err := Instance(load(), "lang.cat"); err == nil || !strings.Contains(err.Error(), "utf8_valid") {
		t.Errorf("without the submonoid: %v; want ρ⁻¹'s utf8_valid refused", err)
	}
	PlantTermWhereUnchecked = true
	defer func() { PlantTermWhereUnchecked = false }()
	if _, err := Instance(load(), "lang.cat"); err != nil {
		t.Errorf("with the term's where unchecked the instance should be accepted, the hole: %v", err)
	}
}

// A GOAL ≠ IS ITS ATOM UP TO ≠'S SYMMETRY (strings-2026-11-16): lang.div's
// guard `b == -1` gives ne(b, −1) on its other arm, and sdiv's `where` asks it
// inside a disjunction, ne(a, −2⁶³) ∨ ne(b, −1). Once a term's host `where`
// is decided, the instance is proven so. THE PLANTED FAULT leaves a goal ≠
// to its two strict sides, which no fact gives: the instance is refused.
func TestAGoalDisequalityIsItsAtom(t *testing.T) {
	load := func() *emit.Target {
		emit.ResetImports()
		tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	if _, err := Instance(load(), "lang.div"); err != nil {
		t.Fatalf("lang.div: %v", err)
	}
	emit.PlantNeGoalUnmatched = true
	defer func() { emit.PlantNeGoalUnmatched = false }()
	if _, err := Instance(load(), "lang.div"); err == nil || !strings.Contains(err.Error(), "sdiv") {
		t.Errorf("with a goal ≠ left to its strict sides: %v; want sdiv's where refused", err)
	}
}

// A REMAINDER BY A CONSTANT OF A PROVEN NATURAL IS THE SIGNED INSTRUCTION (ADR
// 0138): on ℕ ∩ S₆₄, `x % 7` and `x / 7` take `srem_natk` and `sdiv_natk`, the
// same function there as `urem` and `udiv`, which LLVM folds to C's
// divisibility test; a variable divisor and a power of two keep `urem_nat`
// and `udiv_nat` (ADR 0088). The fault leaves point(b) undecided, and the constant takes the
// unsigned instructions again.
func TestARemainderByAConstantIsSigned(t *testing.T) {
	vias := func(src string) map[string]int {
		tg, f := lowerOn(t, src)
		Decide(tg, f, false)
		if _, err := RealizeOps(tg, f); err != nil {
			t.Fatal(err)
		}
		vs := map[string]int{}
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == ODiv || s.Op == ORem {
					vs[s.Via]++
				}
			}
		})
		return vs
	}
	const k = "export fn f(x ∈ 0..1000000) (Int) { x / 7 + x % 7 }\n"
	const v = "export fn f(x ∈ 0..1000000, y ∈ 1..1000) (Int) { x / y + x % y }\n"
	if vs := vias(k); vs["llvm.sdiv-natk"] != 1 || vs["llvm.srem-natk"] != 1 {
		t.Errorf("a constant divisor on ℕ: want sdiv-natk and srem-natk, got %v", vs)
	}
	if vs := vias(v); vs["llvm.udiv-nat"] != 1 || vs["llvm.urem-nat"] != 1 {
		t.Errorf("a variable divisor on ℕ: want udiv-nat and urem-nat, got %v", vs)
	}
	// by 2^k the unsigned pair is one `lshr` and one `and` (1.13× on the limbs'
	// factorial with the signed pair, srem-2026-11-19)
	if vs := vias("export fn f(x ∈ 0..1000000) (Int) { x / 8 + x % 8 }"); vs["llvm.udiv-nat"] != 1 || vs["llvm.urem-nat"] != 1 {
		t.Errorf("a power of two on ℕ: want udiv-nat and urem-nat, got %v", vs)
	}
	PlantPointUndecided = true
	vs := vias(k)
	PlantPointUndecided = false
	if vs["llvm.sdiv-natk"] != 0 || vs["llvm.urem-nat"] != 1 {
		t.Errorf("with point undecided the constant should take the unsigned instructions, got %v", vs)
	}
}
