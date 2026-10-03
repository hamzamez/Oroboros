package emit_test

import (
	"strings"
	"testing"
)

// A CONJUNCTION OF EQUATIONS IS ONE FACT, WHATEVER ITS ORDER AND ORIENTATION
// (emit/linear.go, assumeEQ; lengtheq-2026-10-03). The equations are a solved
// form, Gaussian elimination, and an equation is added to it. It was assigned:
// `len a = len out` and then `len b = len out`, both keyed under len out, and
// the second overwrote the first, so `(b i)` under `i < len a` was refused in
// one orientation of a true precondition and proven in the other.
func TestEquationsAreAddedNotOverwritten(t *testing.T) {
	prog := func(where string) string {
		return `(use go) (export add)
(sig add ((out (buffer f64)) (a (array f64)) (b (array f64))) (buffer f64) (where ` + where + `))
(def add (out a b)
  (loop ((o out) (i 0))
    (>= i (len a))  o
    else            (again (set o i (go.f+ (a i) (b i))) (+ i 1))))`
	}
	for _, c := range []struct {
		where string
		ok    bool
	}{
		// len b = len a, in every orientation and order, through a third length
		{"(and (= (len a) (len out)) (= (len b) (len out)))", true},
		{"(and (= (len out) (len a)) (= (len b) (len a)))", true},
		{"(and (= (len a) (len out)) (= (len out) (len b)))", true},
		{"(and (= (len out) (len a)) (= (len out) (len b)))", true},
		{"(and (= (len b) (len out)) (= (len a) (len out)))", true},
		// a redundant equation changes nothing
		{"(and (= (len a) (len out)) (= (len b) (len out)) (= (len out) (len a)))", true},
		// b longer than a is in range; b shorter is not
		{"(and (= (len a) (len out)) (= (len b) (+ (len out) 1)))", true},
		{"(and (= (len a) (len out)) (= (len b) (- (len out) 1)))", false},
		// no unit coefficient: kept as two inequalities, which still decide
		{"(and (= (len a) (len out)) (= (* 2 (len b)) (* 2 (len out))))", true},
		{"(and (= (len a) (len out)) (= (* 2 (len b)) (len out)))", false},
	} {
		_, err := entryGo(t, prog(c.where))
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.where, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "(b i) is an indexing")) {
			t.Errorf("%s: want (b i) refused, got %v", c.where, err)
		}
	}
}
