package emit

import (
	"strings"
	"testing"
)

// AN OBLIGATION IS DISCHARGED, OR THE PROGRAM IS REFUSED — refinements.md §3a.
//
// An index outside the linear fragment, and a primitive's `where` outside it,
// were emitted with a note: "propagated, not proven". Out of range, JavaScript
// returned `undefined` from an `int` function and x86 reads past the table.

// THE WITNESS: b's cells reach 9 and t has six. Refused, naming the index; the
// same read under a guard that states the bounds is a proof, and is accepted.
func TestAnIndexOutsideTheFragmentIsRefused(t *testing.T) {
	prog := func(read string) string {
		return `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (let b (build b 8
           (loop ((b b) (i 0))
             (>= i 8)  b
             else      (again (set b i (% (+ i n) 10)) (+ i 1))))
       t (build c 6 c)
    ` + read + `))`
	}
	err := refineGo(t, prog(`(t (b 3))`))
	if err == nil || !strings.Contains(err.Error(), "is an indexing") || !strings.Contains(err.Error(), "not proven") {
		t.Errorf("an index into six that may be 9 must be refused: %v", err)
	}
	guarded := prog(`(let v (b 3) (if (and (<= 0 v) (< v (len t))) (t v) 0))`)
	if err := refineGo(t, guarded); err != nil {
		t.Errorf("the same read under a guard stating its bounds was refused: %v", err)
	}
}

// A PRIMITIVE'S `where` OUTSIDE THE FRAGMENT: a closed comparison is decided by
// evaluation, true accepted and false refused; an open one is refused.
func TestAPreconditionOutsideTheFragmentIsDecidedOrRefused(t *testing.T) {
	tg := tempTarget(t, `(sig fdiv ((a any) (b any)) any pure (where (!= b 0)) (host expr "%s / %s"))`)
	for _, c := range []struct {
		prog, want string
	}{
		{`(use tgt) (fn (x) (tgt.fdiv x 3.0))`, ""},
		{`(use tgt) (fn (x) (tgt.fdiv x 0.0))`, "which is false"},
		{`(use tgt) (fn (x y) (tgt.fdiv x (tgt.fdiv y 2.0)))`, "outside the decided fragment"},
	} {
		notes, err := refineWith(t, tg, c.prog)
		switch {
		case c.want == "" && (err != nil || propagated(notes)):
			t.Errorf("%s: a true closed comparison was not decided: %v %q", c.prog, err, notes)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: want a refusal containing %q, got %v", c.prog, c.want, err)
		}
	}
}

// CLOSED COMPARISONS ARE EXACT: integer against integer as integers (2⁵³ < 2⁵³+1,
// which float64 cannot tell apart), a float
// against anything as float64. (NaN, equal to nothing, is handled and untested: no
// literal writes one.)
func TestAClosedComparisonIsEvaluatedExactly(t *testing.T) {
	for _, c := range []struct {
		src  string
		want bool
	}{
		{`(!= 3.0 0)`, true}, {`(= 3 3)`, true}, {`(< 9007199254740992 9007199254740993)`, true},
		{`(<= 2 2)`, true}, {`(> 0.5 1)`, false}, {`(>= -1 -2)`, true},
	} {
		v, closed := closedComparison(mustRead(t, c.src))
		if !closed || v != c.want {
			t.Errorf("%s: got %v (closed %v), want %v", c.src, v, closed, c.want)
		}
	}
	if _, closed := closedComparison(mustRead(t, `(< x 3)`)); closed {
		t.Error("a comparison with a name was taken as closed")
	}
}

// AN ATOM IS MATCHED BY ITS RELATION, NOT ITS SPELLING (opaqueKey). The
// unsigned-word pass spells `<` as `u64<` where both operands lie in U, and
// there they are one relation: math/bits' Div64, guarded by exactly its own
// `where` in the other spelling, was refused. `x*x` keeps the atom outside the
// fragment, so only the match can discharge it; a guard of a DIFFERENT relation
// must not.
func TestAnAtomIsMatchedByItsRelation(t *testing.T) {
	tg := tempTarget(t, `(sig u64< ((a int) (b int)) bool pure (host expr "%s < %s"))
    (sig u64<= ((a int) (b int)) bool pure (host expr "%s <= %s"))
    (sig small ((x int)) int pure (where (< (* x x) 100)) (host expr "%s"))`)
	if _, err := refineWith(t, tg, `(use tgt) (fn (a) (if (tgt.u64< (* a a) 100) (tgt.small a) 0))`); err != nil {
		t.Errorf("a guard of the same relation in another spelling did not discharge it: %v", err)
	}
	if _, err := refineWith(t, tg, `(use tgt) (fn (a) (if (tgt.u64<= (* a a) 100) (tgt.small a) 0))`); !refusedUnproven(err) {
		t.Errorf("a guard of a different relation discharged it: %v", err)
	}
}
