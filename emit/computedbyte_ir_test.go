package emit_test

import (
	"regexp"
	"strings"
	"testing"
)

// TWO BUGS examples/io/freq.oro FOUND, both pre-existing
// (freq-2026-09-08). One is about what the element inference can see, and one
// is about emission having a side effect.

// A PROGRAM THAT COMPUTES A BYTE MUST GET A BYTE BUFFER.
//
// Through the IR the table is consumed inside the function: a declared
// `(array int)` RESULT is a boundary host code is compiled against, and fixes
// the word (Theorem D′), which the term backend narrowed with the contents.
//
// The syntactic element inference was closed under nothing — a literal, an `if`
// over literals, a read from an already-narrowed table — so `(+ 48 (% x 10))`
// was refused although every operand is exact. jsonfmt only ever COPIED bytes,
// which is why nothing had found it: a text program that COMPUTES its output
// could not hand it to `os.text-of`, whose argument is `(array (int 0 255))`.
//
// The remainder rule is the one with no hypothesis on its operand: |a %% d| is
// under |d| whatever a is, on all four targets inside the portable window
// (integers.md §5), so a literal divisor bounds the result with nothing known
// about the dividend. That is exactly what makes a digit exact, since nothing
// bounds `x / p`.
func TestAComputedByteNarrowsItsBuffer(t *testing.T) {
	// THE BUFFER HOLDS ONE OF EACH, and that is what isolates the rule.
	//
	// NEITHER EXISTING PATH DECIDES THIS ALONE. The syntactic one knows the
	// COPY -- a read from an already-narrowed table -- and did not know the
	// arithmetic. The interval one knows the ARITHMETIC, because `remI` already
	// contracts to the divisor, and cannot see the copy: `BufferRange` runs on
	// the `build` lambda alone (frozen-2026-08-28), where `src` is free and
	// therefore ⊤. So a buffer holding both was refused by both, and that is
	// exactly the shape of a report line -- a word copied out of the file and a
	// count rendered into digits.
	got, err := genOn(t, "go", `
(use go)
(export f)
(sig f ((src (array (int 0 255)))) int (where (<= 8 (len src))))
(def f (fn (src)
  (let t (build 8 (fn (out)
           (loop ((out out) (k 0))
             (>= k 8)  out
             else      (again (set out k (if (< k 4) (src k) (+ 48 (% (src k) 10)))) (+ k 1)))))
    (+ (t 0) (t 7)))))`,
		"f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "make([]byte, 8)") {
		t.Errorf("a buffer holding a copied byte and a computed one is a byte buffer:\n%s", got)
	}
}

// A BUFFER NARROWS ON ITS OWN CONTENTS ONLY BY A PROOF.
//
// The circularity guard was written for a bare read of the buffer and then
// threaded through arithmetic, because freq's run-length counter is `(+ (b s)
// 1)`, an increment of a slot inside the `build` that fills it, and deciding
// the element range from that on the term side was reasoning in a circle. It
// was a policy test: with the guard removed no counterexample was found.
//
// The IR's interval domain now DERIVES the range, by induction rather than
// circularity: Theorem 3 of ir/trip.go (bounded increments) bounds every cell
// by its zero fill plus one per back edge, so eight trips give [0, 8] and the
// buffer is a []byte. That is the proof the guard was standing in for. What
// must hold is that the proof is the only way in: the same increments over
// 4000 trips reach 500, and that buffer must not be a []byte. The control
// stores a literal, which narrows with no induction at all.
func TestABufferNarrowsOnItsOwnContentsOnlyByAProof(t *testing.T) {
	prog := func(trips, v string) string {
		return `
(use go)
(export f)
(sig f ((n int)) int (where (and (<= 0 n) (<= n 4))))
(def f (fn (n)
  (let t (build 8 (fn (b)
           (loop ((b b) (k 0))
             (>= k ` + trips + `)  b
             else      (again (set b (% k 8) ` + v + `) (+ k 1)))))
    (+ (t 0) (t 7)))))`
	}
	proven, err := genOn(t, "go", prog("8", "(+ (b (% k 8)) 1)"), "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proven, "make([]byte, 8)") {
		t.Errorf("eight increments of one bound every cell by 8, so the buffer is a []byte:\n%s", proven)
	}
	wide, err := genOn(t, "go", prog("4000", "(+ (b (% k 8)) 1)"), "f")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(wide, "[]byte") {
		t.Errorf("4000 increments over 8 cells reach 500, which a byte does not hold:\n%s", wide)
	}
	control, err := genOn(t, "go", prog("8", "(+ 48 1)"), "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(control, "make([]byte, 8)") {
		t.Errorf("the control must narrow:\n%s", control)
	}
}

// A `let` INSIDE A LOOP GUARD MUST NOT LEAVE DEAD STATEMENTS.
//
// `e.emit` is not a query: a bound containing a `let` emits STATEMENTS into the
// enclosing block. The bounds-check pass emitted the bound and then decided
// whether it had anything to narrow, so when it declined, the statements stayed
// — after the loop's initialisers, dead, and Go refuses a declared and unused
// variable. Nine lines reproduce it, and no program had ever put a `let` in a
// loop guard: `(>= k (slen sp w))` is the first, `slen` binding because a span's
// length is a difference of two table reads that has to be clamped.
//
// The property is stated on the OUTPUT rather than on the pass, because that is
// what the host refuses: every variable the function declares is read somewhere.
func TestALetInALoopGuardLeavesNoDeadBinding(t *testing.T) {
	got, err := genOn(t, "go", `
(use go)
(export run)
(sig run ((n int)) int (where (and (<= 0 n) (<= n 100))))
(def lim (fn (n) (let d (- n 1)
                   (if (< d 0) 0 d))))
(def run (fn (n)
  (loop ((s 0) (k 0))
    (>= k (lim n))  s
    else            (again (+ s k) (+ k 1)))))`, "run")
	if err != nil {
		t.Fatal(err)
	}
	decl := regexp.MustCompile(`(?m)^\s*var (\w+) `)
	for _, m := range decl.FindAllStringSubmatch(got, -1) {
		name := m[1]
		// Every occurrence but the declaration and its assignments must be a
		// read. Counting is enough: a binding that is only ever written appears
		// exactly once per assignment plus once in the declaration.
		uses := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
		writes := regexp.MustCompile(`(?m)^\s*(var )?` + regexp.QuoteMeta(name) + ` = `)
		if len(uses.FindAllString(got, -1)) <=
			len(writes.FindAllString(got, -1))+1 {
			t.Errorf("%s is declared and never read — Go refuses this:\n%s", name, got)
		}
	}
}
