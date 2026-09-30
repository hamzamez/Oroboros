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
		{`(use go/math/bits) (export f) (sig f ((x int)) int) (def f (x) (bits.Len32 x))`,
			"go/math/bits.Len32's parameter x, declared (int 0 4294967295), requires"},
		{`(use go/math/bits) (export f) (sig f () int) (def f () (bits.Len32 -1))`,
			"go/math/bits.Len32's parameter x is declared (int 0 4294967295), and a call passes -1"},
		{`(use go) (use go/os) (export f) (sig f ((d (array (int 0 255)))) bool) (def f (d) (go.err-nil (os.WriteFile "p" d -1)))`,
			"go/os.WriteFile's parameter perm is declared (int 0 4294967295), and a call passes -1"},
	} {
		_, err := dischargeGo(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s\n  want %q, got %v", c.src, c.want, err)
		}
	}
	// …and a value the program's own facts keep inside the range passes.
	for _, src := range []string{
		`(use go/math/bits) (export f) (sig f ((x (int 0 5))) int) (def f (x) (bits.Len32 x))`,
		`(use go) (use go/os) (export f) (sig f ((d (array (int 0 255)))) bool) (def f (d) (go.err-nil (os.WriteFile "p" d 420)))`,
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
	head := "(use go) (export f) (sig f ((x (int 0 1000)) (n int)) int) "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"(def f (x n) (go.<< x 3))", true},
		{"(def f (x n) (if (>= n 0) (go.>> x n) 0))", true},
		{"(def f (x n) (go.<< x -1))", false},
		{"(def f (x n) (go.>> x n))", false},
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
	head := "(use go/os) (export f) (sig f ((c int)) int) "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"(def f (c) (os.Exit 0))", true},
		{"(def f (c) (os.Exit 255))", true},
		{"(def f (c) (os.Exit 256))", false},
		{"(def f (c) (os.Exit c))", false},
		{"(def f (c) (if (and (>= c 0) (<= c 255)) (os.Exit c) 0))", true},
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
	src := `(use go) (use go/os) (use go/os/File as File)
		(export f) (sig f () int)
		(def f () (let (tuple p n) (build b 16 ((File.Read (os.Stdin) b) (fn (q k e) (tuple q k)))) n))`
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
		{"(export f) (sig f ((d (int 0 9))) string) (def f (d) (string-of (+ 48 d)))", true},
		{"(export f) (sig f () string) (def f () (string-of 65))", true},
		{"(export f) (sig f ((c (int 57344 1114111))) string) (def f (c) (string-of c))", true},
		{`(export f) (sig f ((c (int 0 1114111))) string) (def f (c) (if (or (<= c 55295) (>= c 57344)) (string-of c) ""))`, true},
		{"(export f) (sig f () string) (def f () (string-of 55296))", false},
		{"(export f) (sig f ((c int)) string) (def f (c) (string-of c))", false},
		{"(export f) (sig f ((c (int 0 1114111))) string) (def f (c) (string-of c))", false},
	} {
		_, err := entryGo(t, c.src)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.src, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "string-of requires")) {
			t.Errorf("%s: want Scalar's precondition refused, got %v", c.src, err)
		}
	}
}
