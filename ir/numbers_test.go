package ir

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// refRound is r_ℤ,μ(x) on a rational, the specification's (numbers.md §3),
// written from floor and ceiling and nothing the pass uses.
func refRound(x *big.Rat, mode string) *big.Int {
	fl := new(big.Int).Div(x.Num(), x.Denom()) // Euclidean: floor, since the denominator is positive
	if x.IsInt() {
		return fl
	}
	c := new(big.Int).Add(fl, big.NewInt(1))
	frac := new(big.Rat).Sub(x, new(big.Rat).SetInt(fl))
	half := big.NewRat(1, 2)
	nonneg := x.Sign() >= 0
	switch mode {
	case "down":
		return fl
	case "up":
		return c
	case "zero":
		if nonneg {
			return fl
		}
		return c
	case "away":
		if nonneg {
			return c
		}
		return fl
	}
	switch frac.Cmp(half) {
	case -1:
		return fl
	case 1:
		return c
	}
	if mode == "half-away" {
		if nonneg {
			return c
		}
		return fl
	}
	if fl.Bit(0) == 0 { // a tie, to the even one
		return fl
	}
	return c
}

// THE ROUNDINGS ARE THE SPECIFICATION'S, on every tie and every sign
// (numbers.md §3, §6.1): each mode's integer expansion, interpreted over ℤ,
// against refRound on the exact quotient, for every numerator pair on a grid
// that holds ties (a ÷ b with a, b ∈ D(1, …)) and for a rounding of a finer
// grid onto a coarser one.
func TestRoundingIsTheSpecifications(t *testing.T) {
	dir := t.TempDir()
	for _, mode := range []string{"nearest", "half_away", "down", "up", "zero", "away"} {
		m := strings.ReplaceAll(mode, "_", "-")
		// A divisor's sign is one interval's: b ≠ 0 has none, so the two signs
		// are two functions (numbers.go, decide).
		src := "export fn qp(a ∈ D(1, -6..6), b ∈ D(1, 1..3)) (Int) { round(Int, a ÷ b, '" + mode + ") }\n" +
			"export fn qn(a ∈ D(1, -6..6), b ∈ D(1, -3..-1)) (Int) { round(Int, a ÷ b, '" + mode + ") }\n" +
			"export fn g(x ∈ D(3, -2..2)) (D(1, -2..2)) { round(D(1, -2..2), x, '" + mode + ") }\n"
		path := filepath.Join(dir, mode+".oro")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		fs, err := lowerFuncs(path, "go", true)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if len(fs) != 3 {
			t.Fatalf("%s: %d functions", mode, len(fs))
		}
		fg, fqn, fqp := fs[0], fs[1], fs[2] // exports in sorted order: g, qn, qp
		for a := int64(-60); a <= 60; a++ {
			for b := int64(-30); b <= 30; b++ {
				if b == 0 {
					continue
				}
				fq := fqp
				if b < 0 {
					fq = fqn
				}
				got, err := runResult(fq, []cval{intv(a), intv(b)})
				if err != nil {
					t.Fatalf("%s: %d/10 ÷ %d/10: %v", mode, a, b, err)
				}
				want := refRound(big.NewRat(a, b), m)
				if got.i == nil || got.i.Cmp(want) != 0 {
					t.Errorf("%s: round(Int, %d/10 ÷ %d/10) = %v, want %s", mode, a, b, got.i, want)
				}
			}
		}
		for x := int64(-2000); x <= 2000; x++ {
			got, err := runResult(fg, []cval{intv(x)})
			if err != nil {
				t.Fatalf("%s: %d/1000: %v", mode, x, err)
			}
			// round onto D(1): the numerator at scale 1 is r(x/1000 · 10) = r(x/100)
			want := refRound(big.NewRat(x, 100), m)
			if got.i == nil || got.i.Cmp(want) != 0 {
				t.Errorf("%s: round(D(1), %d/1000) has numerator %v, want %s", mode, x, got.i, want)
			}
		}
	}
}

// A ROUNDING'S OBLIGATIONS ARE DECIDED, and each refusal says which. Their
// premises are computed: the same program with the obligation true passes.
func TestARoundingsObligationsAreDecided(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, ok, bad, reason string
	}{
		{"its set's ends",
			"export fn f(x ∈ D(2, 0..10)) (D(1, 0..10)) { round(D(1, 0..10), x) }",
			"export fn f(x ∈ D(2, 0..10)) (D(1, 0..9)) { round(D(1, 0..9), x) }",
			"inside its set: not proven"},
		{"a divisor that may be zero",
			"export fn f(a ∈ D(1, 0..10), b ∈ D(1, 1..10)) (Int) { round(Int, a ÷ b) }",
			"export fn f(a ∈ D(1, 0..10), b ∈ D(1, 0..10)) (Int) { round(Int, a ÷ b) }",
			"may be 0: not proven"},
		{"a quotient not at a rounding's root",
			"export fn f(a ∈ D(1, 0..10), b ∈ D(1, 1..10)) (Int) { round(Int, a ÷ b) }",
			"export fn f(a ∈ D(1, 0..10), b ∈ D(1, 1..10)) (Int) { round(Int, (a ÷ b) * 2) }",
			"only at the root of a rounding"},
		{"a scale that grows around a loop",
			"export fn f(x ∈ D(1, 0..10), n ∈ 0..10) (Int) { loop i = 0, y = x { i < n -> again(i + 1, round(D(1, 0..10), y * 0.5, 'down)); else -> round(Int, y) } }",
			"export fn f(x ∈ D(1, 0..10), n ∈ 0..10) (Int) { loop i = 0, y = x { i < n -> again(i + 1, y * 0.5); else -> round(Int, y) } }",
			"grows past"},
		{"a finer value declared in a coarser grid",
			"export fn f(x ∈ D(1, 0..10)) (D(2, 0..10)) { x }",
			"export fn f(x ∈ D(2, 0..10)) (D(1, 0..10)) { x }",
			"a finer number is not in a coarser grid"},
	}
	for _, c := range cases {
		for _, which := range []string{"ok", "bad"} {
			src := c.ok
			if which == "bad" {
				src = c.bad
			}
			path := filepath.Join(dir, "o.oro")
			if err := os.WriteFile(path, []byte(src+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := lowerFuncs(path, "go", true)
			switch {
			case which == "ok" && err != nil:
				t.Errorf("%s: the true program was refused: %v", c.name, err)
			case which == "bad" && (err == nil || !strings.Contains(err.Error(), c.reason)):
				t.Errorf("%s: want a refusal saying %q, got %v", c.name, c.reason, err)
			}
		}
	}
}

// AN ALIGNMENT IS AN INTEGER OPERATION, AND THE DECISION PROVES IT (numbers.go):
// adding a value at scale 10⁻¹⁸ to one of magnitude 10³ multiplies the latter's
// numerator by 10¹⁸, past the word, so the program is refused by Decide like
// any other overflow, and never wraps. Its premise is computed: the same sum
// of a value of magnitude 1 fits, and passes.
func TestAnAlignmentPastTheWordIsRefused(t *testing.T) {
	dir := t.TempDir()
	run := func(hi string) error {
		src := "export fn f(a ∈ D(18, 0..1), b ∈ 0.." + hi + ") (D(18, 0..10000)) { a + b }\n"
		path := filepath.Join(dir, "w.oro")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return eachEntry(path, "go", func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
			_, leg, err := Entry(tg, name, name, sig, nf, nil, false, nil)
			if err != nil {
				return err
			}
			if len(leg.Unproven) > 0 {
				return fmt.Errorf("unproven: %v", leg.Unproven)
			}
			return nil
		})
	}
	if err := run("1"); err != nil {
		t.Fatalf("a sum that fits was refused: %v", err)
	}
	if err := run("1000"); err == nil {
		t.Errorf("an alignment past the word was accepted")
	}
}

// NO FACT ABOUT A NUMBER OF N IS ASSUMED BY ℤ'S FRAGMENT (emit/numnames.go):
// over N, `x > 9` does not give x ≥ 10 (9.5 > 9), so the refinement layer,
// whose fragment is linear arithmetic over ℤ, may assume nothing of x. Its
// refusal lists what it knew, and x is not in it; with the filter off it knew
// x ≥ 10 and x ≤ 9, false of 9.5 and, together, no state at all.
func TestNoFactAboutANumberIsAssumedInZ(t *testing.T) {
	dir := t.TempDir()
	src := "export fn f(x ∈ D(1, 0..20), t ∈ Array(Int)) (Int) where len(t) == 10 {\n" +
		"    if x > 9 && x < 10 { t[10] } else { 0 }\n}\n"
	path := filepath.Join(dir, "k.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	err := eachEntry(path, "go", func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
		_, _, err := Entry(tg, name, name, sig, nf, nil, false, nil)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "does not follow") {
		t.Fatalf("t[10] in a table of 10 was not refused: %v", err)
	}
	known := err.Error()[strings.Index(err.Error(), "known:"):]
	known = known[:strings.IndexByte(known, '\n')]
	if strings.Contains(known, "x") {
		t.Errorf("the refinement layer assumed a fact about a number of N: %s", known)
	}
}
