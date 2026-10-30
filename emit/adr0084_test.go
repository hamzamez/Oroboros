package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// ADR 0084'S RULES, EACH WITH ITS PLANTED FAULT: a program the rule must refuse,
// refused with its reason, beside the program it must accept. examples/pe is
// the program that demanded them; gauntlet/differential/cases/codecs.oro runs
// them on every target.

// compileGo is the drivers' order with every stage's error returned, so a
// refusal by the reader, the loader, the claims or the reducer is a result,
// not a test failure.
func compileGo(t *testing.T, src string) (string, error) {
	t.Helper()
	// THE DRIVERS' TARGET, its library layers included (cmd/build): a host
	// declaration's `ensure` in lib/os/go.oro is a fact only there.
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("go", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		return "", err
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		return "", err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return "", err
	}
	if err := emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, allProgSigs(prog))); err != nil {
		return "", err
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		return "", err
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return "", err
	}
	f, _, err := ir.Entry(tg, q, q, prog.Sigs[q], nf, allProgSigs(prog), false, nil)
	if err != nil {
		return "", err
	}
	ir.SelectShifts(tg, f)
	return golang.FromFunc(tg, f)
}

func accepts(t *testing.T, what, src string) {
	t.Helper()
	if _, err := compileGo(t, src); err != nil {
		t.Errorf("%s: want it accepted, got %v", what, err)
	}
}

func refuses(t *testing.T, what, src, reason string) {
	t.Helper()
	_, err := compileGo(t, src)
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Errorf("%s: want it refused with %q, got %v", what, reason, err)
	}
}

// A SEPARATION IS ITS SET AND ITS FORMULA: a parameter's φ an obligation at
// each call, a result's a claim at the definition; a set definition unfolds.
func TestASeparationIsItsSetAndItsFormula(t *testing.T) {
	head := "set Small = set { x ∈ 0..100 : x <= 9 }\nfn f(y ∈ Small) (r ∈ set { z ∈ Int : z >= 1 }) { y + 1 }\n"
	accepts(t, "f(3)", head+"export fn g() (Int) { f(3) }")
	refuses(t, "f(12)", head+"export fn g() (Int) { f(12) }", "12 <= 9")
	refuses(t, "a result outside its separation",
		"fn h(y ∈ 0..5) (r ∈ set { z ∈ Int : z >= 10 }) { y }\nexport fn g() (Int) { h(3) }",
		"does not establish")
	refuses(t, "a separation inside a former",
		"fn h(t ∈ Array(set { z ∈ Int : z >= 10 })) (Int) { len(t) }\nexport fn g() (Int) { 1 }",
		"a binder's whole set")
	refuses(t, "a separation in a computation",
		"export fn g() (Int) { x = set { z ∈ Int : z >= 10 }; 1 }", "set { x ∈ S : φ }")
	refuses(t, "replacement", "set S = set { x * 2 : x ∈ 0..9 }\nexport fn g() (Int) { 1 }", "replacement")
}

// A RULE KEEPS ITS DOMAIN: ((table n f) i) is (f i) for i ∈ [0, n) only. The
// fault that shipped: a rule's index past its end read f outside the table.
func TestARuleKeepsItsDomain(t *testing.T) {
	head := "fn at(j ∈ 0..5) (Int) { t = array { i + 1 : i ∈ 0..4 }; t[j] }\n"
	refuses(t, "an index past a rule's end", head+"export fn g(j ∈ 0..5) (Int) { at(j) }",
		"an index into a table given by a rule")
	accepts(t, "an index inside it", "export fn g(j ∈ 0..4) (Int) { t = array { i + 1 : i ∈ 0..4 }; t[j] }")
}

// A SEGMENT IS ITS TABLE RESTRICTED: b[lo:hi] is k ↦ b[lo + k] on [0, hi − lo),
// formed where 0 ≤ lo ≤ hi ≤ len b, and it fuses.
func TestASegmentIsItsTableRestricted(t *testing.T) {
	head := "fn sum(s ∈ Array(0..255), m ∈ 0..9) (Int) { if m < len(s) { s[m] } else { 0 } }\n"
	accepts(t, "a read inside the segment", head+
		"export fn g(m ∈ 0..9) (Int) { b = array { 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12 }; sum(b[2:6], m) }")
	refuses(t, "a read past the segment's end, inside its table",
		"fn rd(s ∈ Array(0..255)) (Int) { s[4] }\n"+
			"export fn g() (Int) { b = array { 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12 }; rd(b[2:6]) }",
		"an index into a table given by a rule")
	refuses(t, "a segment whose ends cross",
		"fn rd(s ∈ Array(0..255)) (Int) { len(s) }\n"+
			"export fn g() (Int) { b = array { 1, 2, 3, 4, 5, 6 }; rd(b[4:2]) }",
		"a segment b[lo:hi]")
}

// A RANGE IS A STATIC VALUE: inf and sup read its ends at compile time, and one
// that reaches run time is refused (Law S).
func TestARangeHasNoRunTimeValue(t *testing.T) {
	accepts(t, "sup of a range", "export fn g() (Int) { sup(3..9) - inf(3..9) }")
	refuses(t, "a range carried by a loop",
		"export fn g() (Int) { r = loop i = 0, s = 1..2 { i < 2 -> again(i + 1, s); else -> s }; sup(r) }",
		"no value")
}

// A LINEAR RESULT IS OWNED: a definition moves out a region it made, never one
// it was given.
func TestALinearResultIsOwned(t *testing.T) {
	refuses(t, "a parameter moved out",
		"fn bad(s ∈ Array(0..255)) (r ∈ Array(0..255) linear) { s }\n"+
			"export fn g() (Int) { var b := bad(array { 1, 2 }); b[0] := 3; b[0] }",
		"moves out a region it owns")
	accepts(t, "a region it made",
		"fn fresh(n ∈ 1..9) (r ∈ Array(0..255) linear) { var r := array { 0 : _ ∈ 0..n-1 }; r }\n"+
			"export fn g() (Int) { var b := fresh(3); b[0] := 3; b[0] }")
}

// AN OUT TABLE IS S'S LONGEST ELEMENT, so S must bound its length.
func TestAnOutTableNeedsALongestElement(t *testing.T) {
	refuses(t, "an out set with no greatest length",
		"use os\nexport fn g() (Int) { (b, r) = os.ReadAt(\"x\", 0, out set { t ∈ Array(0..255) : len(t) >= 4 }); 0 }",
		"bounds no length")
}

// A PARTIAL CODEC IS CHECKED: get takes a total codec only.
func TestAPartialCodecIsChecked(t *testing.T) {
	head := "use bytes\nd9 = bytes.codec(0..9, bytes.'le)\nu16 = bytes.codec(0..2^16-1, bytes.'le)\n"
	refuses(t, "get on a partial codec",
		head+"export fn g() (Int) { b = array { 1, 2 }; bytes.get(b, 0, d9) }", "bytes.get")
	accepts(t, "get on a total codec", head+"export fn g() (Int) { b = array { 1, 2 }; bytes.get(b, 0, u16) }")
	accepts(t, "check on a partial codec",
		head+"export fn g() (Int) { b = array { 1, 2 }; case bytes.check(b, 0, d9) { 'some(v) -> v; 'none -> -1 } }")
}

// A BUFFER A HOST FILLS IS NOT ITS ZERO FILL: the elements a host stores in
// place are its parameter's declared set. The fault that shipped: os.ReadAt's
// buffer read as all zeros, so the claim below was proven, and JavaScript's
// 32-bit `|` was chosen for a value past 2³² (examples/pe printed 0x40000000
// for an image base of 0x140000000). With the rule removed this program is
// accepted (checked when the rule was written).
func TestABufferAHostFillsIsNotItsZeroFill(t *testing.T) {
	refuses(t, "a byte a host wrote, claimed zero",
		"use os\nuse result\nexport fn g() (Int) {\n"+
			"    (b, read) = os.ReadAt(\"x\", 0, out set { t ∈ Array(0..255) : len(t) <= 8 })\n"+
			"    (n, r) = read\n"+
			"    case r { result.'err(k) -> 0; result.'ok(u) -> { h = b[0:n]; if len(h) > 0 { x ∈ 0..0 = h[0]; x } else { 0 } } }\n}",
		"the claim x ∈ 0..0 is not proven")
}

// A UNION OF RANGES: string_of's domain, a byte read from a table holds it.
func TestAByteIsAScalar(t *testing.T) {
	accepts(t, "string_of of a byte", "export fn g(b ∈ Array(0..255)) (String) { if len(b) > 0 { string_of(b[0]) } else { \"\" } }")
}
