package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir"
)

// EVERY CLAIM IS DECIDED WHERE IT IS WRITTEN (ADR 0049; the book's findings
// 80 and 81). A binder's claim `b ∈ S = e` is the formula e ∈ S, an
// obligation of the program's text, so it does not depend on whether S's ends
// are constants or on whether anything reads b:
//   - a range with a term at an end, lo..hi, is lo <= e && e <= hi, a
//     where-mark on the value; it was dropped, so `k ∈ 0..n = i` returned i;
//   - an unread binding's value goes, and its claims move onto the scope's
//     body, whose value a mark leaves alone; they went with it.
//
// False claims are refused saying so, true ones build, on Go and windows.
// Two planted faults restore each hole.
func TestEveryClaimIsDecidedWhereWritten(t *testing.T) {
	const io = "use io\n\n"
	false_ := map[string]struct{ src, says string }{
		"a variable end, at a call": {io + "fn f(n ∈ 0..10, i ∈ 0..100) (Int) {\n    k ∈ 0..n = i\n    k\n}\n\n" +
			"export fn main() { io.print_int(f(3, 50)) }\n", "the claim k ∈ 0..n is false"},
		"a variable end, unproven": {io + "fn f(n ∈ 0..10, i ∈ 0..100) (Int) {\n    k ∈ 0..n = i\n    k\n}\n\n" +
			"export fn main() {\n    loop r = 0 { r >= 3 -> (); else -> { io.print_int(f(3, r * 40)); again(r + 1) } }\n}\n",
			"the claim k ∈ 0..n is not proven"},
		"the midpoint at lo == hi": {io + "fn mid(lo ∈ 0..100, hi ∈ 0..100) (Int) where lo <= hi {\n" +
			"    m ∈ lo..hi-1 = lo + (hi - lo) / 2\n    m\n}\n\nexport fn main() { io.print_int(mid(5, 5)) }\n",
			"the claim m ∈ lo..hi - 1 is false"},
		"a claim nothing reads": {io + "fn f(n ∈ 0..100) (Int) {\n    k ∈ 0..5 = n\n    0\n}\n\n" +
			"export fn main() { io.print_int(f(50)) }\n", "the claim k ∈ 0..5 is false"},
	}
	true_ := map[string]struct{ src, out string }{
		"a variable end its where proves": {io + "fn f(n ∈ 0..10, i ∈ 0..100) (Int) where i <= n {\n    k ∈ 0..n = i\n    k\n}\n\n" +
			"export fn main() { io.print_int(f(9, 4)) }\n", "4"},
		"the midpoint, its upper end hi": {io + "fn mid(lo ∈ 0..100, hi ∈ 0..100) (Int) where lo <= hi {\n" +
			"    m ∈ lo..hi = lo + (hi - lo) / 2\n    m\n}\n\nexport fn main() { io.print_int(mid(5, 5)) }\n", "5"},
		"a true claim nothing reads": {io + "fn f(n ∈ 0..100) (Int) {\n    k ∈ 0..100 = n\n    0\n}\n\n" +
			"export fn main() { io.print_int(f(50)) }\n", "0"},
	}
	for _, target := range []string{"go", "windows"} {
		for what, c := range false_ {
			if _, err := buildAndRunNoExec(t, c.src, target); err == nil || !strings.Contains(err.Error(), c.says) {
				t.Errorf("%s, %s: want it refused saying %q, got %v", target, what, c.says, err)
			}
		}
		for what, c := range true_ {
			if out, err := buildAndRun(t, c.src, target, ""); err != nil || strings.TrimSpace(out) != c.out {
				t.Errorf("%s, %s: %q, %v; want %s", target, what, out, err, c.out)
			}
		}
	}
	for name, c := range map[string]struct {
		plant *bool
		src   string
	}{
		"a variable end dropped":         {&core.PlantClaimEndsDropped, false_["a variable end, at a call"].src},
		"a dead binding's claim dropped": {&core.PlantDeadClaimDropped, false_["a claim nothing reads"].src},
	} {
		*c.plant = true
		_, err := buildAndRunNoExec(t, c.src, "go")
		*c.plant = false
		if err != nil {
			t.Errorf("with %s the false claim should build again, so the test reaches the rule; got %v", name, err)
		}
	}
}

// A CLAIMED CONSTANT IS DECIDED AT ITS DEFINITION (ADR 0072; the book's
// finding 45): `half ∈ 0..cap = cap * 2` is false whether or not anything
// reads half, and is refused under its own name; a true one builds. The fault
// decides a constant only where it is read: the unread false claim builds.
func TestAClaimedConstantIsDecidedWhereWritten(t *testing.T) {
	const head = "use io\n\ncap = 100\n"
	unread := head + "half ∈ 0..cap = cap * 2\n\nexport fn main() { io.print_int(cap) }\n"
	read := head + "half ∈ 0..cap = cap * 2\n\nexport fn main() { io.print_int(half) }\n"
	true_ := head + "half ∈ 0..cap = cap / 2\n\nexport fn main() { io.print_int(half) }\n"
	for _, target := range []string{"go", "windows"} {
		for what, src := range map[string]string{"unread": unread, "read": read} {
			if _, err := buildAndRunNoExec(t, src, target); err == nil ||
				!strings.Contains(err.Error(), "the claim half ∈ 0..100 is false") {
				t.Errorf("%s, a false claimed constant %s: %v, want it refused naming the claim", target, what, err)
			}
		}
		if out, err := buildAndRun(t, true_, target, ""); err != nil || strings.TrimSpace(out) != "50" {
			t.Errorf("%s, a true claimed constant: %q, %v; want 50", target, out, err)
		}
	}
	ir.PlantConstantClaimUndecided = true
	_, err := buildAndRunNoExec(t, unread, "go")
	ir.PlantConstantClaimUndecided = false
	if err != nil {
		t.Errorf("with a constant decided only where read, the unread false claim should build; got %v", err)
	}
}

// AN UNREAD CALL OWES ITS CONTRACT (the claims step's "dead call's
// contract"; ADR 0028): `a = f(-1)` with nothing reading a still owes f's
// parameter range, and `b = g(0)` g's `where`, as read calls do. A call
// whose contract holds builds. The fault drops an unread call's contract
// with its value: both false programs build.
func TestAnUnreadCallOwesItsContract(t *testing.T) {
	const head = "use io\n\nfn f(x ∈ 0..10) { x + 1 }\nfn g(x) where x != 0 { 100 / x }\n\n"
	range_ := head + "export fn main() {\n    a = f(-1)\n    io.print_int(0)\n}\n"
	where := head + "export fn main() {\n    b = g(0)\n    io.print_int(0)\n}\n"
	holds := head + "export fn main() {\n    a = f(3)\n    b = g(5)\n    io.print_int(7)\n}\n"
	for _, target := range []string{"go", "windows"} {
		for src, says := range map[string]string{range_: "f's parameter x", where: "g's `where` is false"} {
			if _, err := buildAndRunNoExec(t, src, target); err == nil || !strings.Contains(err.Error(), says) {
				t.Errorf("%s: an unread call with a false contract: %v, want a refusal saying %q", target, err, says)
			}
		}
		if out, err := buildAndRun(t, holds, target, ""); err != nil || strings.TrimSpace(out) != "7" {
			t.Errorf("%s: unread calls whose contracts hold: %q, %v; want 7", target, out, err)
		}
	}
	core.PlantDeadCallContractDropped = true
	for _, src := range []string{range_, where} {
		if _, err := buildAndRunNoExec(t, src, "go"); err != nil {
			t.Errorf("with an unread call's contract dropped the false program should build; got %v", err)
		}
	}
	core.PlantDeadCallContractDropped = false
}
