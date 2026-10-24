package emit

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
)

// withWord gives a test fixture the word the Go, JVM and windows target files
// declare, when the fixture says none. ADR 0026 makes the word required of
// every target — the compiler holds no default window — and a fixture about
// something else should not have to repeat it. A fixture that declares its own
// word, or tests its absence, is left alone.
func withWord(src string) string {
	if regexp.MustCompile(`(?m)^\s*repr .* word\s*$`).MatchString(src) || strings.Contains(src, "no-word") {
		return src
	}
	// The target is its file, and its word a line of it.
	return "repr -2^63..2^63-1 word\n" + src
}

// A TUPLE COMPONENT'S RANGE IS A FACT, as a single result's is (mathbits-2026-09-23):
// Add64's carry-out is declared (int 0 1), so it satisfies the next Add64's
// `carry ≤ 1`. Before, the refinement layer bound a host call's several results
// with nothing known of them, and the most basic use of Add64 — a carry chain —
// was refused.
func TestATupleComponentsRangeIsAFact(t *testing.T) {
	err := refineGo(t, `use go/math/bits as b
export fn f(x ∈ 0..2^64-1, y ∈ 0..2^64-1) (0..1) {
    (s, c) = b.Add64(x, y, 0)
    (s2, c2) = b.Add64(x, y, c)
    c2
}`)
	if err != nil {
		t.Errorf("a carry chain must be proven: the first carry is (int 0 1): %v", err)
	}
	// ANTI-VACUITY: a carry-in the program cannot bound is still refused.
	err = refineGo(t, `use go/math/bits as b
export fn f(x ∈ 0..2^64-1, c ∈ 0..2^64-1) (0..1) {
    (s, o) = b.Add64(x, x, c)
    o
}`)
	if err == nil {
		t.Error("an unbounded carry-in must be refused")
	}
}

// A PRIMITIVE IS APPLIED TO EXACTLY WHAT IT DECLARES. `(fmt.Println a b)` against
// a one-argument Println emitted `fmt.Println(a)`: b was dropped, and the program
// printed less than it said (mathbits-2026-09-23). Println is variadic now (ADR
// 0046), its trailing arguments its list, so the witness is a fixed arity.
func TestAnExtraArgumentIsRefusedNotDropped(t *testing.T) {
	tg := goNative(t)
	nf := reduce(t, `use go/strconv
fn t() { strconv.Itoa(1, 2) }`, "go")
	if err := Check(tg, "t", nf); err == nil || !strings.Contains(err.Error(), "takes 1 argument") {
		t.Errorf("an over-applied primitive must be refused by name; got %v", err)
	}
	ok := reduce(t, `use go/strconv
fn t() { strconv.Itoa(1) }`, "go")
	if err := Check(tg, "t", ok); err != nil {
		t.Errorf("an exactly-applied primitive must be accepted: %v", err)
	}
}

// A REMAINDER BY A POSITIVE LITERAL IS AN ATOM of the fragment, as a quotient is,
// so the language's remainder facts (lang-facts.oro, F6–F8) reach a precondition.
// They were dead in the refiner: the fact matched, its conclusion was read, found
// outside the fragment, and nothing was assumed. So `hi < y` for
// `(b.Div64 (% h k) l k)` — the high word reduced mod the divisor, the step of
// every short division — was "propagated, not proven" and emitted anyway
// (u128-2026-09-23). Both spellings: h in the signed word, and h in U, where the
// goal says `u64%` and the fact's instance `%`, and each divisor is `(u64-of k)`.
func TestARemainderBoundsAPrecondition(t *testing.T) {
	for _, hi := range []string{"1000000", "18446744073709551615"} {
		notes, err := refineWorded(t, `use go/math/bits as b
export fn f(h ∈ 0..`+hi+`, l ∈ 0..2^64-1) (Any) {
    (q, r) = b.Div64(h % 1000000000000000000, l, 1000000000000000000)
    q
}`)
		if err != nil || propagated(notes) {
			t.Errorf("h ≤ %s: h mod k < k must be PROVEN: err %v, notes %s", hi, err, notes)
		}
	}
	// ANTI-VACUITY: a remainder by a LARGER divisor bounds nothing useful, and
	// the precondition is not proven.
	notes, err := refineWorded(t, `use go/math/bits as b
export fn f(h ∈ 0..2^64-1, l ∈ 0..2^64-1) (Any) {
    (q, r) = b.Div64(h % 1000000000000000001, l, 1000000000000000000)
    q
}`)
	if err == nil && !propagated(notes) {
		t.Errorf("h mod (k+1) < k does not follow and must not be proven: %s", notes)
	}
}

// refineWorded is refineGo with the type checker run first, as cmd/gen does.
// The unsigned word is no longer selected on the term (ADR 0033): the checker
// and the refinement layer see the integers, and the IR chooses their
// representation.
func refineWorded(t *testing.T, src string) (string, error) {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	sig := prog.Sigs[q]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(tg, "test", nf); err != nil {
		return "", err
	}
	// THIS HELPER TESTS THE REFINEMENT LAYER ALONE, which requires its residual's
	// contract marks decided (Undecided). Their decision is the pipeline's
	// (ir.DecideMarks, which this package cannot import) and is tested through
	// it; here they are erased, on purpose and in sight.
	nf = core.StripRequires(nf)
	notes, err := Refine(tg, "test", sig, nf)
	return strings.Join(notes, "; "), err
}
