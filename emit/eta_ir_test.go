package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// entryGo takes a program's first export through the drivers' order: reduce,
// η for products, the joins' check, then ir.Entry, and prints Go.
func entryGo(t *testing.T, src string) (string, error) {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return "", err
	}
	f, _, err := ir.Entry(tg, q, q, prog.Sigs[q], nf, allProgSigs(prog), false, nil)
	if err != nil {
		return "", err
	}
	ir.SelectShifts(tg, f)
	return golang.FromFunc(tg, f)
}

// etaHead opens f; each case binds (dst, k) and returns.
const etaHead = "use go/encoding/hex\nexport fn f(n ∈ 0..100) (Int) {\n    src = array { 104, 105, 33 }\n"

// η FOR PRODUCTS (tables.md §2.5): a host call with m declared results at a
// producer's exit is the tuple of its results, p = ⟨π₁ p, …, πₘ p⟩. In program
// order (ADR 0052) a borrowing call's value is its result alone, so J writes the
// tuple out, the buffer beside the call's result; the state translation gives
// back the call's continuation (fn (d c) (tuple d c)), which η takes apart: the
// call is made once, and no product is built.
func TestAHostCallsResultsAreAProductByEta(t *testing.T) {
	a, err := entryGo(t, etaHead+`    (dst, k) = {
        var buf := array { 0 : _ ∈ 0..2 * len(src)-1 }
        c = hex.Encode(buf, src)
        (buf, c)
    }
    if k > 2 { len(dst) } else { 0 }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(a, "hex.Encode("); n != 1 {
		t.Errorf("the call is made %d times; η evaluates it once:\n%s", n, a)
	}
	if strings.Contains(a, "struct") {
		t.Errorf("a product was built where the host has two results:\n%s", a)
	}
}

// THROUGH EVERY FORM A TAIL PASSES THROUGH: an `if`'s arms and a loop's exits,
// each giving the call's two results or a tuple.
func TestEtaReachesArmsAndLoopExits(t *testing.T) {
	for _, body := range []string{
		`{
        var buf := array { 0 : _ ∈ 0..2 * len(src)-1 }
        if n > 50 { c = hex.Encode(buf, src); (buf, c) } else { (buf, 0) }
    }`,
		`{
        var buf := array { 0 : _ ∈ 0..2 * len(src)-1 }
        if n > 50 { (buf, 0) } else { c = hex.Encode(buf, src); (buf, c) }
    }`,
		`{
        var buf := array { 0 : _ ∈ 0..2 * len(src)-1 }
        loop i = 0 { i >= 2 -> { c = hex.Encode(buf, src); (buf, c) }; else -> again(i + 1) }
    }`,
	} {
		src := etaHead + "    (dst, k) = " + body + "\n    if k > 2 { len(dst) } else { 0 }\n}"
		if _, err := entryGo(t, src); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}

// A PATTERN OF ANOTHER SIZE, OR AN EXIT OF ONE VALUE, IS REFUSED BY NAME: η
// applies to a call declaring exactly as many results as the pattern has names.
func TestAnExitThatIsNotAnMTupleIsRefused(t *testing.T) {
	for _, c := range []struct{ pattern, body string }{
		{"(dst, k, j)", `{ var buf := array { 0 : _ ∈ 0..5 }; c = hex.Encode(buf, src); (buf, c) }`},
		{"(dst, k)", `{ var buf := array { 0 : _ ∈ 0..5 }; buf }`},
	} {
		src := etaHead + "    " + c.pattern + " = " + c.body + "\n    len(dst)\n}"
		_, err := entryGo(t, src)
		if err == nil || !strings.Contains(err.Error(), "every exit must give") {
			t.Errorf("%s over %s: %v", c.pattern, c.body, err)
		}
	}
}
