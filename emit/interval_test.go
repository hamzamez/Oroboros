package emit

import (
	"strings"
	"testing"
)

// The interval experiment is only worth anything if it never claims a bound it
// does not have. These are the soundness cases: each one MUST come out
// unproven, and a change that makes the headline number go up by breaking one
// of them has measured nothing.
func TestIntervalsNeverOverclaim(t *testing.T) {
	for _, c := range []struct {
		name, src string
		proven    int
		ops       int
	}{
		{"literals fold", `(use go) (fn () (go.+ (go.* 3 4) 5))`, 2, 2},
		// Two operands that each fit comfortably, whose PRODUCT does not.
		{"product escapes the window",
			`(use go) (fn (a) (let (go.& a 1073741823) (fn (x) (go.* x x))))`, 0, 1},
		// A counter bounded by its own guard.
		{"guarded counter",
			`(use go) (fn () (loop ((i 0)) (go.>= i 100) i else (again (go.+ i 1))))`, 1, 1},
		// An accumulator over an UNBOUNDED trip count is genuinely unbounded and
		// must not be claimed. This is the residue class the experiment reports.
		{"accumulator over an unbounded loop",
			`(use go) (fn (n) (loop ((i 0) (acc 0)) (go.>= i n) acc else (again (go.+ i 1) (go.+ acc 1))))`,
			0, 2},
		// A parameter with nothing declared about it bounds nothing.
		{"undeclared parameter", `(use go) (fn (a b) (go.+ a b))`, 0, 1},
	} {
		nf := reduce(t, c.src, "go")
		tg, err := LoadTarget("../targets/go")
		if err != nil {
			t.Fatal(err)
		}
		r, _ := Intervals(tg, nil, nf, 0)
		if r.Ops != c.ops || r.Proven != c.proven {
			t.Errorf("%s: got %d/%d proven, want %d/%d\n  %v",
				c.name, r.Proven, r.Ops, c.proven, c.ops, r.Unproven)
		}
	}
}

// And the knob has to do something, or the two columns of the experiment are
// the same column.
func TestIntervalsUseDeclaredRange(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	nf := reduce(t, `(use go) (fn (n) (loop ((i 0) (s 0)) (go.>= i n) s else (again (go.+ i 1) (go.+ s i))))`, "go")
	bare, _ := Intervals(tg, nil, nf, 0)
	declared, _ := Intervals(tg, nil, nf, 1000)
	if declared.Proven <= bare.Proven {
		t.Errorf("a declared range proved nothing extra: %d then %d", bare.Proven, declared.Proven)
	}
}

// The rewrite must be the IDENTITY on a target that declares no checked forms.
//
// Everything about representation selection rests on this: the pass rebuilds the
// whole residual, and a target that opts out must get back exactly what it gave
// — same tree, same emitted code. `portable-go` declares none.
func TestIntervalRewriteIsIdentityWithoutCheckedForms(t *testing.T) {
	tg := goTarget(t) // portable-go
	for _, src := range []string{
		`(use num/int) (fn (a b) (int.add a b))`,
		`(use num/int) (fn (n) (loop ((i 0) (s 0)) (int.ge i n) s else (again (int.add i 1) (int.add s i))))`,
		`(use num/int) (fn (n) (let (int.mul n n) (fn (q) (if (int.lt q 10) q (int.sub q 1)))))`,
	} {
		nf := reduce(t, src, "portable-go")
		before := nf.String()
		_, out := Intervals(tg, nil, nf, 0)
		if out.String() != before {
			t.Errorf("rewrite changed the term on a target with no checked forms\n src  %s\n got  %s\n want %s",
				src, out, before)
		}
	}
}

// And where the target DOES declare them, the selection has to go both ways:
// an operation the compiler can bound keeps the host's own operator, and one it
// cannot gets the checked form. A pass that rewrote everything would be no more
// useful than one that rewrote nothing.
func TestIntervalSelectsBothWays(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// `i + 1` is bounded by a LITERAL guard, so it is provable whatever else is
	// undeclared. `s + n` is bounded by nothing, because n is a parameter with
	// no range. Using a literal here is the point: with `(go.>= i n)` instead,
	// neither is provable and the test would not be testing both ways.
	nf := reduce(t, `(use go) (fn (n) (loop ((i 0) (s 0)) (go.>= i 100) s else (again (go.+ i 1) (go.+ s n))))`, "go")
	rep, out := Intervals(tg, nil, nf, 0)
	got := out.String()
	if !strings.Contains(got, "(go.+ i 1)") {
		t.Errorf("a bounded operation should keep the host operator:\n%s", got)
	}
	if !strings.Contains(got, "(go.add-exact s n)") {
		t.Errorf("an unbounded operation should take the checked form:\n%s", got)
	}
	if rep.Selected != 1 {
		t.Errorf("want exactly one selection, got %d", rep.Selected)
	}

	// Declare the range and every check disappears. This is the whole trade.
	rep2, out2 := Intervals(tg, nil, nf, 1000)
	if rep2.Selected != 0 {
		t.Errorf("a declared range should remove every check, got %d:\n%s", rep2.Selected, out2)
	}
	if strings.Contains(out2.String(), "exact") {
		t.Errorf("no checked form should survive a declared range:\n%s", out2)
	}
}
