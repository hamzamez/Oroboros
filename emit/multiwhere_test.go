package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A PRIMITIVE WITH SEVERAL RESULTS IS ELIMINATED BY APPLYING IT TO A
// CONTINUATION — `((p a…) (fn (x y) body))` — so the operator of the outer
// application is itself an application (values.go, multiPrimCall). The
// refinement walk returned at any operator that was not a name, which skipped
// three things at once: the primitive's own `where`, its arguments, and the
// whole continuation body. hex-2026-09-14 found it by planting a short buffer
// under hex.Encode, which built and panicked inside the host.

func TestAWhereOnAPrimitiveWithSeveralResultsIsDischarged(t *testing.T) {
	tg := tempTarget(t, `(sig pair ((k int)) (tuple int int) (where (<= 0 k)) (host expr "pair(%s)"))`)
	// k is unconstrained, so the precondition does not follow.
	if _, err := refineWith(t, tg, `(use tgt) (fn (k) ((tgt.pair k) (fn (a b) a)))`); err == nil {
		t.Error("an unproven precondition on a primitive with two results was accepted")
	}
	// CONTROL: a literal satisfies it, and must still be accepted and PROVEN.
	notes, err := refineWith(t, tg, `(use tgt) ((tgt.pair 3) (fn (a b) a))`)
	if err != nil {
		t.Errorf("a proven precondition was refused: %v", err)
	}
	if propagated(notes) {
		t.Errorf("the precondition must be proven, not propagated: %s", notes)
	}
}

func TestTheContinuationOfAPrimitiveWithSeveralResultsIsWalked(t *testing.T) {
	tg := tempTarget(t, `(sig pair ((k int)) (tuple int int) (host expr "pair(%s)"))`)
	// `need` requires 0 <= k, and the continuation passes it an unconstrained name.
	if _, err := refineWith(t, tg, `(use tgt) (fn (k) ((tgt.pair 1) (fn (a b) (tgt.need k))))`); err == nil {
		t.Error("an obligation inside the continuation body was never discharged")
	}
	// CONTROL: the same body with a literal is accepted.
	if _, err := refineWith(t, tg, `(use tgt) ((tgt.pair 1) (fn (a b) (tgt.need 2)))`); err != nil {
		t.Errorf("a discharged obligation inside the continuation was refused: %v", err)
	}
}

// A LOOP VARIABLE ASSIGNED A SCANNER'S RESULT IS NON-DECREASING, and the
// refinement layer could not see it: it licensed `0 <= i` only when every
// `again` adds a literal. `(again ni)` with `ni` bound to a loop that starts at
// `i + 1` and only adds is at least `i` by the loop monotonicity corollary
// (monotone.go), so the relation `e ⊒ S` gains the rule it was missing — a loop
// is at least S when its lower bound is — and the refiner asks it. Every
// text-reading program has this shape; freq.oro and jsonfmt.oro were refused
// on it the moment their bodies were walked.
func TestAnIndexAssignedAScannersResultIsNonNegative(t *testing.T) {
	// IMPURE, so the call with an unused result survives reduction and there is
	// an obligation left to discharge (see the note on `half` below).
	tg := tempTarget(t, `(sig at ((k int)) int (where (<= 0 k)) (host expr "at(%s)"))`)
	// The scanner is a definition, as in freq.oro: reduction inlines it, so the
	// loop reaches the refiner as the value of a `let`.
	const scanner = `(use tgt)
	  (def scan (fn (n i) (loop ((j (+ i 1))) (>= j n) j else (again (+ j 1)))))
	  (fn (n)
	    (loop ((i 0))
	      (>= i n) 0
	      else (let ni (scan n i)
                 u  (tgt.at i)
              (again ni))))`
	if _, err := refineWith(t, tg, scanner); err != nil {
		t.Errorf("0 <= i follows from monotonicity and must be derived: %v", err)
	}
	// CONTROL: a scanner that may return LESS than it was given is not
	// non-decreasing, and the obligation must stay refused.
	const falls = `(use tgt)
	  (def scan (fn (n i) (loop ((j (+ i 1))) (>= j n) (- j 5) else (again (+ j 1)))))
	  (fn (n)
	    (loop ((i 0))
	      (>= i n) 0
	      else (let ni (scan n i)
                 u  (tgt.at i)
              (again ni))))`
	if _, err := refineWith(t, tg, falls); err == nil {
		t.Error("a scanner whose exit subtracts was taken as non-decreasing")
	}
}

// EUCLIDEAN DIVISION HAS TWO HALVES. For x >= 0 and k > 0 the quotient q = x/k
// satisfies k·q <= x <= k·q + (k−1), and only the first half was assumed. So a
// buffer sized ⌊x/2⌋ could not be shown to hold the decoding of x bytes, which
// needs x <= 2·len + 1 — hex.Decode's exact precondition. The guard that makes
// both halves sound is the same one: x is a LENGTH, so x >= 0.
func TestBothHalvesOfDivisionByALiteralAreKnown(t *testing.T) {
	tg := tempTarget(t, `(sig / ((a int) (b int)) int pure (host expr "%s / %s"))
    (sig half ((dst (array int)) (src (array int))) int
      (where (<= (len src) (+ (* 2 (len dst)) 1))) (host expr "half(%s, %s)"))`)
	// IMPURE on purpose: a pure call whose result is unused is dropped by
	// reduction (weakening is allowed for it, ADR 0010), and then there is no
	// call left to check and both halves of this test pass vacuously.
	// The call sits inside the build's binder, which is where `len b` is known —
	// hex.Decode's own shape in encoding-hex.oro.
	notes, err := refineWith(t, tg, `(use tgt) (fn (a) (build (/ (len a) 2) (fn (b) (let u (tgt.half b a)
                                                 b))))`)
	if err != nil {
		t.Errorf("x <= 2·(x/2) + 1 must follow for a length x: %v", err)
	}
	if propagated(notes) {
		t.Errorf("the precondition must be proven, not propagated: %s", notes)
	}
	// CONTROL: one element fewer is genuinely too small, and must be refused.
	if _, err := refineWith(t, tg, `(use tgt) (fn (a) (build (- (/ (len a) 2) 1) (fn (b) (let u (tgt.half b a)
                                                       b))))`); err == nil {
		t.Error("a buffer one element short of ⌊x/2⌋ was accepted")
	}
}

// A FACT ASSUMED BEFORE AN EQUATION MUST STILL MEET A GOAL ASKED AFTER IT.
// Facts were rewritten by the equalities known when they were ASSUMED and goals
// by the equalities known when they were ASKED, so the division axioms — seeded
// at the root — kept `len(enc)` while a goal under `let enc = build (2·len a)`
// became `2·len(a)`, and the two stopped matching. Equality is a congruence:
// rewriting both sides by the same equations at query time changes no meaning.
func TestAFactAndAGoalAreRewrittenByTheSameEquations(t *testing.T) {
	tg := tempTarget(t, `(sig / ((a int) (b int)) int pure (host expr "%s / %s"))
    (sig half ((dst (array int)) (src (array int))) int
      (where (<= (len src) (+ (* 2 (len dst)) 1))) (host expr "half(%s, %s)"))`)
	const ok = `(use tgt)
	  (fn (a) (let enc (build (* 2 (len a)) (fn (c) c))
             (build (/ (len enc) 2) (fn (b) (let u (tgt.half b enc)
                                              b)))))`
	if _, err := refineWith(t, tg, ok); err != nil {
		t.Errorf("the axiom and the goal must agree after the let's equation: %v", err)
	}
	// CONTROL: one short is still refused.
	const short = `(use tgt)
	  (fn (a) (let enc (build (* 2 (len a)) (fn (c) c))
             (build (- (/ (len enc) 2) 1) (fn (b) (let u (tgt.half b enc)
                                                    b)))))`
	if _, err := refineWith(t, tg, short); err == nil {
		t.Error("a buffer one short was accepted once the equation was known")
	}
}

// THE THEOREM GIVES v >= z FOR ANY INITIAL VALUE z, so 0 <= v follows whenever
// the facts at the loop's entry prove 0 <= z — not only when z is a literal.
// jsonfmt.oro copies a token with a loop starting at the index `i` it already
// knows is non-negative, and was refused on it.
func TestALoopStartingAtAKnownNonNegativeValueStaysNonNegative(t *testing.T) {
	tg := tempTarget(t, `(sig at ((k int)) int (where (<= 0 k)) (host expr "at(%s)"))`)
	const guarded = `(use tgt)
	  (fn (n m) (if (< m 0) 0
	    (loop ((k m)) (>= k n) 0 else (let u (tgt.at k)
                                     (again (+ k 1))))))`
	if _, err := refineWith(t, tg, guarded); err != nil {
		t.Errorf("0 <= m on entry and k only grows, so 0 <= k: %v", err)
	}
	// CONTROL: with nothing known about m, k may start negative.
	const bare = `(use tgt)
	  (fn (n m) (loop ((k m)) (>= k n) 0 else (let u (tgt.at k)
                                             (again (+ k 1)))))`
	if _, err := refineWith(t, tg, bare); err == nil {
		t.Error("a loop starting at an unconstrained value was taken as non-negative")
	}
}

// A CLAMPED VALUE USED TWICE KEEPS ITS BOUNDS. Call-by-need let-binds a
// conditional that is used more than once, and the refinement layer learned
// nothing about a let-bound conditional — a limitation CLAUDE.md recorded with
// the product. The join over the branches (refine.go, joinConditional) gives
// the name every inequality all its leaves satisfy on their paths.
func TestAClampUsedTwiceKeepsItsBounds(t *testing.T) {
	tg := tempTarget(t, `(sig at ((k int)) int (where (and (<= 0 k) (< k 10))) (host expr "at(%s)"))`)
	const clamp = `(use tgt)
	  (fn (i) (let c (if (< i 0) 0 (if (>= i 10) 0 i))
                u (tgt.at c)
             (tgt.at c)))`
	if _, err := refineWith(t, tg, clamp); err != nil {
		t.Errorf("every branch of the clamp is in [0, 10), so c is: %v", err)
	}
	// CONTROL: one branch outside the range, and the join must not claim it.
	const leaky = `(use tgt)
	  (fn (i) (let c (if (< i 0) 0 (if (>= i 10) 11 i))
                u (tgt.at c)
             (tgt.at c)))`
	if _, err := refineWith(t, tg, leaky); err == nil {
		t.Error("a conditional with a branch at 11 was taken to be below 10")
	}
}

// AND A CLAMP OF A VALUE THE PROGRAM CANNOT SEE, which is freq.oro's `pat`: the
// read is bound to a name first, because the clamp mentions it three times, so
// the join has to look through that `let` — and must not keep a fact about the
// inner name, which means nothing outside.
func TestAClampOfAnOpaqueValueKeepsItsBounds(t *testing.T) {
	tg := tempTarget(t, `(sig rd ((k int)) int (host expr "rd(%s)"))
    (sig at ((k int)) int (where (and (<= 0 k) (< k 10))) (host expr "at(%s)"))`)
	const clamp = `(use tgt)
	  (def cl (fn (i) (if (< i 0) 0 (if (>= i 10) 0 i))))
	  (fn (j) (let c (cl (tgt.rd j))
                u (tgt.at c)
             (tgt.at c)))`
	if _, err := refineWith(t, tg, clamp); err != nil {
		t.Errorf("a clamp of an opaque host result is in [0, 10): %v", err)
	}
	// CONTROL: no clamp, and nothing is known about the host's result.
	const bare = `(use tgt)
	  (fn (j) (let c (tgt.rd j)
                u (tgt.at c)
             (tgt.at c)))`
	if _, err := refineWith(t, tg, bare); err == nil {
		t.Error("an unclamped host result was taken to be in range")
	}
}

// A CLAUSE THAT IS GIVEN TWICE MUST NOT SILENTLY LOSE ONE COPY. `(where …)`
// assigned a field, so a second one replaced the first and a precondition the
// author wrote vanished without a word — the conjunction is spelled `(and …)`.
func TestARepeatedWhereOrEnsuresIsRefused(t *testing.T) {
	for _, clause := range []string{"where", "ensures"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "t.oro")
		src := `(target tgt (type int (host "int")) (module tgt
  (sig f ((n int)) int pure (` + clause + ` (<= 0 n)) (` + clause + ` (<= n 9)) (host expr "f(%s)"))))`
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTarget(path)
		if err == nil {
			t.Errorf("a second (%s …) was accepted and one of the two was dropped", clause)
		} else if !strings.Contains(err.Error(), clause) {
			t.Errorf("the refusal should name the clause: %v", err)
		}
	}
}
