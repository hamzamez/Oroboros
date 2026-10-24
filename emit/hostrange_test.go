package emit_test

import (
	"strings"
	"testing"
)

// A HOST FUNCTION'S PARAMETER RANGE IS AN OBLIGATION AT ITS CALLS (ADR 0028,
// refinements.md §6b, gotarget-2026-09-30). `bits.Len32`'s argument is a
// uint32, [0, 2^32 − 1], and its template converts with uint32(%s): the range
// was read as a type only, so (bits.Len32 x) with x : int compiled to
// bits.Len32(uint32(x)) and answered 32 for −1.
func TestAHostParametersRangeIsAnObligation(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`use go/math/bits
export fn f(x ∈ Int) (Int) { bits.Len32(x) }`,
			"`go/math/bits`.Len32's parameter x, declared in 0..2^32-1, requires"},
		{`use go/math/bits
export fn f() (Int) { bits.Len32(-1) }`,
			"`go/math/bits`.Len32's parameter x is declared in 0..2^32-1, and a call passes -1"},
		{`use go
use go/os
use result
export fn f(d ∈ Array(0..255)) (Bool) {
    case os.WriteFile("p", d, -1) { result.'ok(u) -> true; result.'err(e) -> false }
}`,
			"`go/os`.WriteFile's parameter perm is declared in 0..2^32-1, and a call passes -1"},
	} {
		_, err := dischargeGo(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s\n  want %q, got %v", c.src, c.want, err)
		}
	}
	// …and a value the program's own facts keep inside the range passes.
	for _, src := range []string{
		`use go/math/bits
export fn f(x ∈ 0..5) (Int) { bits.Len32(x) }`,
		`use go
use go/os
use result
export fn f(d ∈ Array(0..255)) (Bool) {
    case os.WriteFile("p", d, 420) { result.'ok(u) -> true; result.'err(e) -> false }
}`,
	} {
		if _, err := dischargeGo(t, src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// A SHIFT'S DOMAIN IS b ≥ 0 (the Go specification: a negative count panics).
// Declared as a `where`, it is decided like `/`'s b ≠ 0: by a literal, by a
// guard, or refused.
func TestAGoShiftCountMustBeNonNegative(t *testing.T) {
	head := "use go\nexport "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"fn f(x ∈ 0..1000, n ∈ Int) (Int) { go.`<<`(x, 3) }", true},
		{"fn f(x ∈ 0..1000, n ∈ Int) (Int) { if n >= 0 { go.`>>`(x, n) } else { 0 } }", true},
		{"fn f(x ∈ 0..1000, n ∈ Int) (Int) { go.`<<`(x, -1) }", false},
		{"fn f(x ∈ 0..1000, n ∈ Int) (Int) { go.`>>`(x, n) }", false},
	} {
		_, err := entryGo(t, head+c.body)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.body, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "requires")) {
			t.Errorf("%s: want the shift's domain refused, got %v", c.body, err)
		}
	}
}

// os.Exit's PRECONDITION is 0 ≤ code ≤ 255 (targets/go/os.oro): on POSIX the
// status is the code mod 256, so Exit(256) would exit 0. Declared as a `where`
// beyond the host's `int`, and so an obligation at every call.
func TestExitsCodeIsAStatus(t *testing.T) {
	head := "use go/os\nexport "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"fn f(c ∈ Int) (Int) { os.Exit(0) }", true},
		{"fn f(c ∈ Int) (Int) { os.Exit(255) }", true},
		{"fn f(c ∈ Int) (Int) { os.Exit(256) }", false},
		{"fn f(c ∈ Int) (Int) { os.Exit(c) }", false},
		{"fn f(c ∈ Int) (Int) { if c >= 0 && c <= 255 { os.Exit(c) } else { 0 } }", true},
	} {
		_, err := entryGo(t, head+c.body)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.body, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: an exit status outside [0, 255] was accepted", c.body)
		}
	}
}

// THE STANDARD STREAMS ARE DECLARED, and a *File's Read is a write-borrow into
// a scope's buffer: what bufio needs from os (gotarget-2026-09-30 §5).
func TestStdinCanBeRead(t *testing.T) {
	src := `use go
use go/os
use go/os/File
export fn f() (Int) {
    (p, n) = {
        var b := array { 0 : _ ∈ 0..15 }
        (k, e) = File.Read(os.Stdin(), b)
        ignore(e)
        (b, k)
    }
    n
}`
	if _, err := entryGo(t, src); err != nil {
		t.Errorf("reading standard input: %v", err)
	}
}

// string-of's DOMAIN IS Scalar = [0, D7FF] ∪ [E000, 10FFFF] (string-operations.md
// §7), the same on every target, because it is the language's operation and the
// hosts disagree outside it. A disjunction, decided by the sequent calculus's
// right-hand rules (refinements.md §3b): one side proven, or an assumption that
// is the same term.
func TestStringOfsDomainIsScalar(t *testing.T) {
	for _, c := range []struct {
		src string
		ok  bool
	}{
		{"export fn f(d ∈ 0..9) (String) { string_of(48 + d) }", true},
		{"export fn f() (String) { string_of(65) }", true},
		{"export fn f(c ∈ 57344..1114111) (String) { string_of(c) }", true},
		{`export fn f(c ∈ 0..1114111) (String) { if c <= 55295 || c >= 57344 { string_of(c) } else { "" } }`, true},
		{"export fn f() (String) { string_of(55296) }", false},
		{"export fn f(c ∈ Int) (String) { string_of(c) }", false},
		{"export fn f(c ∈ 0..1114111) (String) { string_of(c) }", false},
	} {
		_, err := entryGo(t, c.src)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.src, err)
		}
		// Refused by the domain's bounds, a range obligation since a `where`'s
		// constant bounds are the parameter's (wherebounds.go), or by its
		// disjunction, the `where` that stays.
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "string_of") || !strings.Contains(err.Error(), "requires")) {
			t.Errorf("%s: want Scalar's precondition refused, got %v", c.src, err)
		}
	}
}

// A *File IS AN io.Reader, and EOF is a value a program compares with: the
// subsumption edge os.oro declares, read by io.ReadAll, and io's package
// variables (gotarget-2026-09-30 §11).
func TestAFileIsAReaderAndEOFIsAValue(t *testing.T) {
	// ReadAll's error is the relevant 1 + E (ADR 0044), so the error compared
	// with EOF is the one inside it. Compared whole, the tag was compared and
	// the emitted Go did not compile; nothing compiled it (variadic-2026-10-08).
	src := "use go\nuse go/os\nuse go/io\nuse result\nexport fn f() (Int) {\n    (b, err) = io.ReadAll(os.Stdin())\n    case err {\n        result.'ok(u) -> len(b)\n        result.'err(e) -> if go.`==`(e, io.EOF()) { -1 } else { len(b) }\n    }\n}"
	if _, err := entryGo(t, src); err != nil {
		t.Errorf("io.ReadAll over os.Stdin: %v", err)
	}
}

// A PROGRAM'S OWN `go.u64+` IS ℤ/2⁶⁴'S SUM, NOT ℤ'S (u64.oro). The compiler
// writes the unqualified `u64+` only where ir.Decide proves ℤ's sum inside U,
// so there the two agree; a program that names the host's operation gets the
// host's, total on U × U, and its result's fact is the set U the declaration
// states. Both operands are at least 2⁶³, so ℤ's sum is at least 2⁶⁴ and the
// first arm is unreachable in ℤ; Go's sum wraps into it (2⁶³ + 2⁶³ is 0). A
// fact taken from ℤ reads [2⁶⁴, 2⁶⁵ − 2], outside every realization, and the
// program is refused or the arm lost; the host's fact keeps both.
func TestAProgramsU64SumIsTheHostsModularSum(t *testing.T) {
	src := "use go\nexport fn f(a ∈ 2^63..2^64-1, b ∈ 2^63..2^64-1) (0..100) {\n    if go.`u64+`(a, b) < (2^63) { 1 } else { 2 }\n}"
	out, err := entryGo(t, src)
	if err != nil {
		t.Fatalf("the host's sum is in U, and the comparison holds both operands: %v", err)
	}
	if !strings.Contains(out, "return 1") || !strings.Contains(out, "return 2") {
		t.Errorf("an arm the wrapped sum reaches was pruned:\n%s", out)
	}
}

// math/big's Quo and Rem panic on a zero divisor, and so does big%-small's
// Rem: each declares b ≠ 0 (bigint.oro), decided at a program's own call as
// `/`'s is. big%-small's divisor is a word, so the fragment decides it. A
// `big` operand is outside the fragment, so a direct big/ or big% is refused
// even at a nonzero literal (gotarget-2026-09-30 §14): sound, and no program
// in the corpus names one, since the language's `/` carries its own
// obligation.
func TestABignumDivisorIsNonZero(t *testing.T) {
	head := "use go\nexport "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"fn f(n ∈ Int) (Int) { go.`big%-small`(go.big_of(7), 3) }", true},
		{"fn f(n ∈ Int) (Int) { go.`big%-small`(go.big_of(7), 0) }", false},
		{"fn f(n ∈ Int) (Int) { go.`big%-small`(go.big_of(7), n) }", false},
		{"fn f(n ∈ Int) (Int) { if n == 0 { 0 } else { go.`big%-small`(go.big_of(7), n) } }", true},
		{"fn f(n ∈ Int) (Int) { go.`big%-small`(go.`big/`(go.big_of(7), go.big_of(0)), 3) }", false},
		{"fn f(n ∈ Int) (Int) { go.`big%-small`(go.`big%`(go.big_of(7), go.big_of(n)), 3) }", false},
	} {
		_, err := entryGo(t, head+c.body)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.body, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "requires")) {
			t.Errorf("%s: want the divisor refused, got %v", c.body, err)
		}
	}
}
