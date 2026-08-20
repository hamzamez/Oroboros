package emit

import "testing"

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
		r := Intervals(tg, nil, nf, 0)
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
	bare := Intervals(tg, nil, nf, 0)
	declared := Intervals(tg, nil, nf, 1000)
	if declared.Proven <= bare.Proven {
		t.Errorf("a declared range proved nothing extra: %d then %d", bare.Proven, declared.Proven)
	}
}
