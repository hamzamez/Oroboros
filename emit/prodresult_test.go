package emit

import (
	"strings"
	"testing"
)

// ADR 0031 — A BUILD'S RESULT IS A PRODUCT (tables.md §2.5). Its no-copy freeze
// rests on four refusals; the first two tests are its hypothesis, and both were
// holes that gave wrong answers before (prodresult-2026-09-25).

// S: A STORE NEEDS A LIVE BUFFER. Each program below was accepted, and each
// changed what a frozen value reads; each legal twin beside it must stay legal,
// or the refusal would be proving nothing (the anti-vacuity half).
func TestAStoreIntoAFrozenValueIsRefused(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(export f)
(def f (n)
  (let t (build b 4 (set b 0 n))
       t2 (set t 1 n)
    (+ (t2 0) (t 1))))`, "set stores into t, which is a frozen table"},
		{`(export f)
(def f (n)
  (let m (build-map m 8 (insert m 1 n))
       m2 (insert m 1 (+ n 1))
    (len m2)))`, "which is a frozen map"},
	} {
		err := linearOn(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want a refusal containing %q, got %v\n%s", c.want, err, c.src)
		}
	}
	for _, ok := range []string{
		`(export f)
(def f (n) (let t (build b 4 (set (set b 0 n) 1 n)) (t 1)))`,
		`(export f)
(def f (n) (len (build-map m 8 (insert (insert m 1 n) 2 n))))`,
	} {
		if err := linearOn(t, ok); err != nil {
			t.Errorf("a store into a live buffer was refused: %v\n%s", err, ok)
		}
	}
}

// A MAP BUFFER IS LINEAR. Used twice, the second `insert` wrote into the
// storage the first had already handed on, and a read of the second result saw
// the first key.
func TestAMapBufferIsLinear(t *testing.T) {
	src := `(export f)
(def f (n)
  (len (build-map m 8
         (let m2 (insert m 1 n)
              m3 (insert m 2 n)
           m3))))`
	if err := linearOn(t, src); err == nil || !strings.Contains(err.Error(), "handed on") {
		t.Errorf("a map buffer used twice: want the linearity refusal, got %v", err)
	}
	threaded := `(export f)
(def f (n)
  (len (build-map m 8
         (loop ((m m) (i 0))
           (>= i n)  m
           else      (again (insert m i 1) (+ i 1))))))`
	if err := linearOn(t, threaded); err != nil {
		t.Errorf("a threaded map buffer was refused: %v", err)
	}
}

// THE COMPONENT LAW, WITNESSED SOUND (ADR 0031 §3). The two components of one
// tuple have different ranges, and each name must get its OWN: `a` ≤ 11, so
// a·10¹⁵ is inside int64; `b` ≤ 1.1·10¹³, so b·10⁶ is not. Exactly that one
// operation is unproven (the `%` keeps it out of the sum). Binding every name to
// the first component's facts proves the overflow; binding none loses a·10¹⁵.
//
// The back edge is a `let` whose body jumps, which is what a clause chain looks
// like after reduction, and the join over the exits must see that it yields no
// value (prodfacts.go, noValue) or it loses the exit it does have.
func TestEachComponentHasItsOwnFacts(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (let (tuple a b) (loop ((i 0) (s 0))
                     (>= i n)  (tuple i (* i 1000000000000))
                     else      (let j (if (< i 5) (+ i 1) (+ i 2))
                                 (again j j)))
    (+ (* a 1000000000000000) (% (* b 1000000) 7))))`
	rep := reportGo(t, src)
	if rep.Proven != 5 || rep.Ops != 6 || len(rep.Unproven) != 1 ||
		!strings.Contains(rep.Unproven[0], "(* b 1000000)") {
		t.Errorf("%d of %d proven, unproven %v; want 5 of 6, only b·10⁶", rep.Proven, rep.Ops, rep.Unproven)
	}
}

// A FROZEN COMPONENT KEEPS ITS SCOPE'S LENGTH: `(b 7)` is in bounds because
// len b = 8 crossed the tuple, and `(b 8)` is refused, which is the anti-vacuity
// half — a layer that proved everything would pass the first.
func TestAFrozenComponentKeepsItsLength(t *testing.T) {
	prog := func(i string) string {
		return `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (let (tuple b k) (build b 8
                     (loop ((b b) (i 0))
                       (>= i 8)  (tuple b i)
                       else      (again (set b i n) (+ i 1))))
    (+ (b ` + i + `) k)))`
	}
	if err := refineGo(t, prog("7")); err != nil {
		t.Errorf("(b 7) through a tuple: %v", err)
	}
	if err := refineGo(t, prog("8")); err == nil {
		t.Errorf("(b 8) was accepted on a table of 8")
	}
}

// A JOIN POINT IS NOT JUMPED OUT OF, and a scope does not hand out a function
// term (R1); both are refused by name, and the legal twin of each is not.
func TestAJoinPointAndAScopeRefuseWhatIsNotBuilt(t *testing.T) {
	jump := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (loop ((s 0) (j 0))
    (>= j n)  s
    else      (let (tuple a b) (loop ((i 0)) (>= i 3) (tuple i j) else (again (+ i 1)))
                (again (+ s (+ a b)) (+ j 1)))))`
	tg, nf, _ := normGo(t, jump)
	if err := CheckJoins(tg, nf); err == nil || !strings.Contains(err.Error(), "JOIN POINT") {
		t.Errorf("an again inside a join point: got %v", err)
	}
	fine := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (let (tuple a b) (loop ((i 0)) (>= i 3) (tuple i n) else (again (+ i 1)))
    (+ a b)))`
	tg, nf, _ = normGo(t, fine)
	if err := CheckJoins(tg, nf); err != nil {
		t.Errorf("a join point with no jump was refused: %v", err)
	}
	variant := `(export f)
(sig f ((n (int 0 3))) int)
(def f (n)
  (let o (build b 4 (some (set b 0 n)))
    (case o (some t) 7 none 0)))`
	tg, nf, _ = normGo(t, variant)
	if err := CheckJoins(tg, nf); err == nil || !strings.Contains(err.Error(), "(R1)") {
		t.Errorf("a variant out of a scope: got %v", err)
	}
}

// R2: THE SAME BUFFER TWICE is two live references to what the freeze hands
// out once. Linearity refuses it, because forming the tuple moves the buffer.
func TestTheSameBufferTwiceIsRefused(t *testing.T) {
	src := `(export f)
(def f (n)
  (let (tuple p q) (build b 4 (loop ((b b) (i 0)) (>= i 4) (tuple b b) else (again (set b i n) (+ i 1))))
    (+ (p 0) (q 0))))`
	if err := linearOn(t, src); err == nil || !strings.Contains(err.Error(), "handed on") {
		t.Errorf("(tuple b b): want the linearity refusal, got %v", err)
	}
}

// A FROZEN COMPONENT KEEPS ITS CONTENT FACTS. The loop stores its own index
// while it is below G, so every cell is below G, and with G = 6 `(b 3)` indexes
// a table of six. The fact is a Houdini invariant of the scope's loop, which the
// refiner caches by the loop's back-edge skeleton (bodyKey): the loop it walked
// yields the tuple, and the projection it asks about yields b — the same loop
// with another exit. With G = 7 it must NOT be proven (the anti-vacuity half).
// Both answers are exactly the one-result program's.
//
// An index outside the linear fragment — a table read, as here — that is not
// proven is "propagated" with a note rather than refused (refine.go,
// indexObligation), so the notes are what this test reads.
func TestAFrozenComponentKeepsItsContent(t *testing.T) {
	prog := func(g string) string {
		return `(export f)
(sig f ((n (int 0 100))) int)
(def f (n)
  (let (tuple b k) (build b 8
                     (loop ((b b) (i 0))
                       (>= i ` + g + `)  (tuple b i)
                       else      (again (set b i i) (+ i 1))))
       t (build c 6 c)
    (+ (t (b 3)) k)))`
	}
	proven := func(src string) bool {
		tg, nf, sig := normGo(t, src)
		notes, err := Refine(tg, "test", sig, nf)
		if err != nil {
			return false
		}
		for _, n := range notes {
			if strings.Contains(n, "propagated") {
				return false
			}
		}
		return true
	}
	if !proven(prog("6")) {
		t.Errorf("cells below 6, read as an index into six, were not proven through the tuple")
	}
	if proven(prog("7")) {
		t.Errorf("a cell that may be 6 was proven an index into six")
	}
}
