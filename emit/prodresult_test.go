package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// ADR 0031 — A BUILD'S RESULT IS A PRODUCT (tables.md §2.5). Its no-copy freeze
// rests on four refusals; the first two tests are its hypothesis, and both were
// holes that gave wrong answers before (prodresult-2026-09-25).

// S: A STORE NEEDS A LIVE BUFFER. Each program below was accepted, and each
// changed what a frozen value reads; each legal twin beside it must stay legal,
// or the refusal would be proving nothing (the anti-vacuity half).
func TestAStoreIntoAFrozenValueIsRefused(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`export fn f(n) {
    t = { var b := array { 0 : _ ∈ 0..3 }; b[0] := n; b }
    t2 = store(t, 1, n)
    t2[0] + t[1]
}`, "set stores into t, which is a frozen table"},
		{`export fn f(n) {
    m = { var m := map(8); m[1] := n; m }
    m2 = insert(m, 1, n + 1)
    len(m2)
}`, "which is a frozen map"},
	} {
		err := linearOn(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want a refusal containing %q, got %v\n%s", c.want, err, c.src)
		}
	}
	for _, ok := range []string{
		`export fn f(n) {
    t = { var b := array { 0 : _ ∈ 0..3 }; b[0] := n; b[1] := n; b }
    t[1]
}`,
		`export fn f(n) { len({ var m := map(8); m[1] := n; m[2] := n; m }) }`,
	} {
		if err := linearOn(t, ok); err != nil {
			t.Errorf("a store into a live buffer was refused: %v\n%s", err, ok)
		}
	}
}

// A MAP BUFFER IS LINEAR. Used twice, the second `insert` wrote into the
// storage the first had already handed on, and a read of the second result saw
// the first key. In program order (ADR 0052, 0058) a stale version cannot be
// written, and a second name for the map is refused by R1.
func TestAMapBufferIsLinear(t *testing.T) {
	src := `export fn f(n) { len({ var m := map(8); m2 = m; m[1] := n; m2 }) }`
	if err := placeRefusal(t, src); err == nil || !strings.Contains(err.Error(), "R1") {
		t.Errorf("a map buffer named twice: want R1's refusal, got %v", err)
	}
	threaded := `export fn f(n) {
    len({ var m := map(8); loop i = 0 { i >= n -> m; else -> { m[i] := 1; again(i + 1) } } })
}`
	if err := linearOn(t, threaded); err != nil {
		t.Errorf("a threaded map buffer was refused: %v", err)
	}
}

// A FROZEN COMPONENT KEEPS ITS SCOPE'S LENGTH: `(b 7)` is in bounds because
// len b = 8 crossed the tuple, and `(b 8)` is refused, which is the anti-vacuity
// half — a layer that proved everything would pass the first.
func TestAFrozenComponentKeepsItsLength(t *testing.T) {
	prog := func(i string) string {
		return `export fn f(n ∈ 0..100) (Int) {
    (b, k) = {
        var b := array { 0 : _ ∈ 0..7 }
        loop i = 0 { i >= 8 -> (b, i); else -> { b[i] := n; again(i + 1) } }
    }
    b[` + i + `] + k
}`
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
	jump := `export fn f(n ∈ 0..10) (Int) {
    loop s = 0, j = 0 {
        j >= n -> s
        else -> {
            (a, b) = loop i = 0 { i >= 3 -> (i, j); else -> again(i + 1) }
            again(s + (a + b), j + 1)
        }
    }
}`
	tg, nf, _ := normGo(t, jump)
	if err := CheckJoins(tg, nf); err == nil || !strings.Contains(err.Error(), "JOIN POINT") {
		t.Errorf("an again inside a join point: got %v", err)
	}
	fine := `export fn f(n ∈ 0..10) (Int) {
    (a, b) = loop i = 0 { i >= 3 -> (i, n); else -> again(i + 1) }
    a + b
}`
	tg, nf, _ = normGo(t, fine)
	if err := CheckJoins(tg, nf); err != nil {
		t.Errorf("a join point with no jump was refused: %v", err)
	}
	variant := `export fn f(n ∈ 0..3) (Int) {
    o = { var b := array { 0 : _ ∈ 0..3 }; b[0] := n; 'some(b) }
    case o { 'some(t) -> 7; 'none -> 0 }
}`
	// A VARIANT OUT OF A SCOPE IS A PRODUCT OUT OF A SCOPE since a constructor
	// is the tuple (tag, payload) (ADR 0041): the payload buffer is a component,
	// frozen by ADR 0031's law, and η for products binds it as a tuple pattern.
	// R1 is a buffer INSIDE another value; this one is at the top.
	tg, nf, _ = normGo(t, variant)
	if err := CheckJoins(tg, nf); err != nil {
		t.Errorf("a variant out of a scope is ADR 0031's product: %v", err)
	}
	// A closure out of a scope is still refused: its body may hold the buffer.
	// In program order the state translation refuses it by R3, no closure over
	// a buffer, before CheckJoins's R1.
	closure := `export fn f(n ∈ 0..3) (Int) {
    g = { var b := array { 0 : _ ∈ 0..3 }; fn(i) { b[i] := n; b } }
    g[0]
}`
	if err := placeRefusal(t, closure); err == nil || !strings.Contains(err.Error(), "R3") {
		t.Errorf("a closure out of a scope: got %v", err)
	}
}

// placeRefusal is the loader's refusal of src: the state translation decides
// R0–R4 on each place's scope (spec/local.md §2.1).
func placeRefusal(t *testing.T, src string) error {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = core.Load(forms)
	return err
}

// R2: THE SAME BUFFER TWICE is two live references to what the freeze hands
// out once. Linearity refuses it, because forming the tuple moves the buffer.
func TestTheSameBufferTwiceIsRefused(t *testing.T) {
	src := `export fn f(n) {
    (p, q) = {
        var b := array { 0 : _ ∈ 0..3 }
        loop i = 0 { i >= 4 -> (b, b); else -> { b[i] := n; again(i + 1) } }
    }
    p[0] + q[0]
}`
	if err := linearOn(t, src); err == nil || !strings.Contains(err.Error(), "handed on") {
		t.Errorf("(b, b): want the linearity refusal, got %v", err)
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
// proven is refused (refinements.md §3a); before that it was a note, which is
// why `proven` also reads the notes.
func TestAFrozenComponentKeepsItsContent(t *testing.T) {
	prog := func(g string) string {
		return `export fn f(n ∈ 0..100) (Int) {
    (b, k) = {
        var b := array { 0 : _ ∈ 0..7 }
        loop i = 0 { i >= ` + g + ` -> (b, i); else -> { b[i] := i; again(i + 1) } }
    }
    t = { var c := array { 0 : _ ∈ 0..5 }; c }
    t[b[3]] + k
}`
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
