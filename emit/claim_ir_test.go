package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A BINDER'S CLAIM IS AN OBLIGATION (ADR 0069 item 4): `b ∈ S = e` is proven of e
// or the program is refused, for the reason the claim gives. Trusted, both of the
// false ones compiled (ownbuf-2026-10-21).
func TestABindersClaimIsProvenOrRefused(t *testing.T) {
	const head = "use go\nuse go/os as gos\nuse go/fmt\nexport fn main() {\n    av = gos.Args()\n"
	for what, c := range map[string]struct{ body, want string }{
		"false, a literal":    {"    x ∈ 0..5 = 10\n    fmt.Println(x)\n}\n", "the claim x ∈ 0..5 is false: its value is 10"},
		"not proven":          {"    x ∈ 0..5 = len(av) + 10\n    fmt.Println(x)\n}\n", "the claim x ∈ 0..5 is not proven"},
		"a constant's set":    {"    x ∈ 0..lim = len(av) + 10\n    fmt.Println(x)\n}\nlim = 5\n", "the claim x ∈ 0..5 is not proven"},
		"proven by its guard": {"    if len(av) < 4 { x ∈ 0..5 = len(av) + 1; fmt.Println(x) }\n}\n", ""},
	} {
		err := refusedGo(t, head+c.body)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", what, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", what, err, c.want)
		}
	}
}

// THE CORE'S ASCRIPTION IS NOT SURFACE (ADR 0062): written as a call it was a
// trusted claim.
func TestTheIsNotSurface(t *testing.T) {
	_, err := core.ReadEntry("use go\nuse go/fmt\nexport fn main() {\n    x = the(\"int 0 5\", 10)\n    fmt.Println(x)\n}\n", "t.oro")
	if err == nil || !strings.Contains(err.Error(), "`the` is the core's ascription, not J's") {
		t.Errorf("the(…) in J: %v", err)
	}
}

// A HOST CALL'S `ensure` NAMES A COMPONENT OF ITS TUPLE RESULT (ADR 0069 item 5):
// io.Reader's law, n ≤ len q, on os.File.Read, proves the last byte read is an
// index of the buffer. Without the law the index is unprovable.
func TestAReadsCountIsAtMostItsBuffer(t *testing.T) {
	out := printsGo(t, `use go
use go/os as gos
use go/os/File
use go/fmt
use result
fn done?(r) { case r { result.'ok(u) -> false; result.'err(e) -> { ignore(e); true } } }
export fn main() {
    var b := array { 0 : _ ∈ 0..4095 }
    (n, r) = File.Read(gos.Stdin(), b)
    ignore(done?(r))
    fmt.Println(if n > 0 { b[n - 1] } else { 0 })
}
`)
	if !strings.Contains(out, ".Read(") {
		t.Errorf("no read:\n%s", out)
	}
}

// claimCheck runs the drivers' program-level claim check (gen and build call
// it) on a program in J, on Go.
func claimCheck(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	return emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, allProgSigs(prog)))
}

// A DECLARED RESULT IS A CLAIM, decided at the definition (ADR 0072): the
// header is f ∈ Π(x ∈ P). R, so the obligation is ∀x ∈ P. ⟦B⟧(x) ∈ R, whether
// the definition is exported, called, or neither. Until then an internal
// definition's claim was dropped at inlining, and `bad(1)` printed 11.
func TestAResultClaimIsAnObligation(t *testing.T) {
	refused := func(name, src, why string) {
		t.Helper()
		err := claimCheck(t, src)
		if err == nil || !strings.Contains(err.Error(), why) || !strings.Contains(err.Error(), "ADR 0072") {
			t.Errorf("%s: want a refusal naming %q, got %v", name, why, err)
		}
	}
	accepted := func(name, src string) {
		t.Helper()
		if err := claimCheck(t, src); err != nil {
			t.Errorf("%s: a true claim is refused: %v", name, err)
		}
	}
	const main = "\nexport fn main() { 0 }\n"
	refused("called", "use go\nuse go/fmt\nfn bad(x ∈ 0..100) (0..5) { x + 10 }\n"+
		"export fn main() { fmt.Println(bad(1)) }\n", "does not establish 0 <= result && result <= 5")
	refused("never called", "fn bad(x ∈ 0..100) (0..5) { x + 10 }"+main, "its result is [10, 110]")
	refused("exported", "export fn bad(x ∈ 0..100) (0..5) { x + 10 }\n", "does not establish")
	// A parameter with no set ranges over its sort, ℤ: x % 6 may be negative.
	refused("no set", "fn f(x) (0..5) { x % 6 }"+main, "does not establish")
	accepted("saturating", "fn sat(x ∈ 0..2^62-1, y ∈ 0..2^62-1) (0..2^62-1) {\n"+
		"    s = x + y\n    if s <= (2^62-1) { s } else { 2^62-1 }\n}"+main)
	// THE CONTRACT IS P ⟹ Q: the `where` is assumed where the claim is decided.
	accepted("under its where", "fn half(x ∈ Int) (0..50) where 0 <= x && x <= 100 { x / 2 }"+main)
	refused("without it", "fn half(x ∈ Int) (0..50) { x / 2 }"+main, "does not establish")
	// A RESULT IN THE UNSIGNED WORD, which the reader makes no postcondition
	// of (its ends are past int64), is decided on a target that holds it in a
	// word: U, on Go.
	accepted("unsigned", "fn near(n ∈ 0..100) (0..2^64-1) { (2^64-1) - n }"+main)
	refused("unsigned, false", "fn near(n ∈ 0..100) (0..2^64-100) { (2^64-1) - n }"+main,
		"its result is in 0..18446744073709551516")
	// A TABLE'S READ IS APPLICATION: a claimed table parameter's elements are
	// in its set, and its read is no function's application.
	accepted("a table's read", "fn first(a ∈ Array(0..5)) (0..5) { if len(a) > 0 { a[0] } else { 0 } }"+main)
	// A DEFINITION OVER A FUNCTION: its contract quantifies over the function,
	// which has no set (ADR 0053); refused with that reason, not with a hint to
	// narrow a set. Applied to one value, the parameter may be a table instead,
	// so the refusal names both.
	refused("over a function", "fn app(f, x ∈ 0..10) (0..10) { f(fn(y) { y }) }"+main,
		"a function has no set to decide it over")
	refused("a table or a function", "fn app(f, x ∈ 0..10) (0..10) { f(x) }"+main,
		"if f is a function")
	// A TUPLE'S SET IS A PRODUCT: each component is decided in its factor.
	accepted("tuple", "fn split(x ∈ 0..100) (0..50 × 0..1) { (x / 2, x % 2) }"+main)
	refused("tuple, a false factor", "fn split(x ∈ 0..100) (0..50 × 0..0) { (x / 2, x % 2) }"+main,
		"component 2 of its result is in 0..0")
}
