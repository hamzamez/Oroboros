package main

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A CALL'S SORT CLAIM IS DECIDED AT THE CALL (ADR 0154). A definition's header
// says f : S → R, and f applied to a owes ⟦a⟧ ∈ S (ADR 0028) whoever reads the
// parameter. A range's claim is the argument's mark (#req) and a `where` the
// result's; a set that is no range, Int against N, String against an integer,
// had none, so an unread parameter's false claim compiled and a read one's was
// refused inside the body, in its library's names (the book's finding 110:
// `io.print_int(0.5 + 0.5)` "in argument 1 of io.print_line: in argument 1 of
// decimal: n is N"). Now β marks the call's result with the membership and the
// type checker decides it there. PlantSortClaimDropped marks nothing again:
// the unread claims build, and the read one is refused inside the body.
func TestACallsSortClaimIsDecidedAtTheCall(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go and windows programs")
	}
	unread := "use io\nfn f(s ∈ String) (Int) { 0 }\nexport fn main() { io.print_int(f(3)) }\n"
	dead := "use io\nfn f(s ∈ String) (Int) { 0 }\nexport fn main() { x = f(3); io.print_int(7) }\n"
	read := "use io\nexport fn main() { io.print_int(0.5 + 0.5) }\n"
	holds := "use io\nfn f(s ∈ String) (Int) { 7 }\nfn g(b ∈ Bool, n ∈ Int) (Int) { if b { n } else { 2 } }\n" +
		"export fn main() { x = 3 < 4; io.print_int(f(\"x\") + g(x, 5)) }\n"
	for _, target := range []string{"go", "windows"} {
		for _, c := range []struct{ name, src, at, why string }{
			{"unread", unread, "in argument 1 of f", "3 is Int, but String is required here"},
			{"dead", dead, "in argument 1 of f", "3 is Int, but String is required here"},
			{"read", read, "in argument 1 of io.print_int", "0.5 + 0.5 is N, but Int is required here"},
		} {
			_, err := runOnMayFail(t, c.src, target)
			if err == nil || !strings.Contains(err.Error(), c.at+": "+c.why) {
				t.Errorf("%s: a false claim at a call (%s) should be refused %s: %s; got %v", target, c.name, c.at, c.why, err)
			}
		}
		if got, err := runOnMayFail(t, holds, target); err != nil || got != "12" {
			t.Errorf("%s: true claims at calls should build and print 12, got %q, %v", target, got, err)
		}
		core.PlantSortClaimDropped = true
		for _, s := range []string{unread, dead} {
			if _, err := runOnMayFail(t, s, target); err != nil {
				t.Errorf("%s: with the claim dropped the unread false claim should build again, got %v", target, err)
			}
		}
		if _, err := runOnMayFail(t, read, target); err == nil || strings.Contains(err.Error(), "of io.print_int") {
			t.Errorf("%s: with the claim dropped the read claim should be refused inside the body, got %v", target, err)
		}
		core.PlantSortClaimDropped = false
	}
}

// THE DISCARD KEEPS ITS OBLIGATIONS (ADR 0154 item 3). ignore is !_A : A → 1,
// and an unread value's obligations are the program's statements (the book's
// 81): `ignore(g(30))` owes g's range as `x = g(30); ()` does, where the pure
// value was the unit with nothing kept. PlantDeadClaimDropped drops them again.
func TestADiscardedCallOwesItsContract(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go and windows programs")
	}
	src := "use io\nfn g(n ∈ 0..9) (Int) { n + 1 }\nfn h(n ∈ Int) (Int) where n < 5 { n + 1 }\n" +
		"export fn main() { ignore(g(30)); ignore(h(30)); io.print_int(7) }\n"
	for _, target := range []string{"go", "windows"} {
		_, err := runOnMayFail(t, src, target)
		if err == nil || !strings.Contains(err.Error(), "declared in 0..9") {
			t.Errorf("%s: a discarded call's range should be refused, got %v", target, err)
		}
		core.PlantDeadClaimDropped = true
		got, err := runOnMayFail(t, src, target)
		core.PlantDeadClaimDropped = false
		if err != nil || got != "7" {
			t.Errorf("%s: with the claims dropped the discarded calls should build and print 7, got %q, %v", target, got, err)
		}
	}
}

// A CLAIM'S COPY IS NO USE (ADR 0154, programOccurrences). β substitutes a
// pure value the program reads once, and a call's claim copies its argument,
// logic erased once decided: counted as a read, `primes` read once by
// `primes[5]` stayed a binding, and roundtrip's main on windows grew by a fifth.
// PlantClaimCounted counts the copy again: the refusal names the binding.
func TestAClaimsCopyIsNoUse(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a Go program")
	}
	src := "use io\n\nexport fn main() {\n    primes = array { 2, 3, 5, 7, 11 }\n    io.print_int(primes[5])\n}\n"
	_, err := runOnMayFail(t, src, "go")
	if err == nil || !strings.Contains(err.Error(), "`array`(2, 3, 5, 7, 11)[5] is an indexing") {
		t.Errorf("the table read once should be substituted, its index refused as the literal's; got %v", err)
	}
	core.PlantClaimCounted = true
	_, err = runOnMayFail(t, src, "go")
	core.PlantClaimCounted = false
	if err == nil || !strings.Contains(err.Error(), "primes[5] is an indexing") {
		t.Errorf("with the claim's copy counted the binding should stay, its index refused as primes[5]; got %v", err)
	}
}

// A WHERE-MARK LEAVES A λ ONLY IF ITS CONDITION READS NONE OF ITS VARIABLES
// (sortclaim-2026-11-22). A λ's body is closed, its variables de Bruijn
// indices; a mark hoisted out of a condition's λ with one of them read the next
// binder's. name's claim copies its argument, a segment of a segment, a table
// rule whose body holds the segments' marks, and reducing the claim again
// hoisted them past the rule and the host call's binders: n read as the pair
// (n, r) and the buffer as n. PlantWhereLeavesItsBinder hoists them again.
func TestAWhereMarkStaysUnderItsBinder(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a Go program")
	}
	src := "use io\nuse os\nuse result\n\nset Head = set { t ∈ Array(0..255) : len(t) <= 4096 }\n\n" +
		"fn name(s ∈ Array(0..255)) (String) {\n" +
		"    loop i = 0, t = \"\" {\n" +
		"        i < len(s) && s[i] != 0 -> again(i + 1, t ++ string_of(s[i]))\n" +
		"        else -> t\n" +
		"    }\n" +
		"}\n\n" +
		"fn report(h ∈ Head) {\n" +
		"    case {\n" +
		"        len(h) < 8 -> { io.print_line(\"short\"); 0 }\n" +
		"        else -> { io.print_line(name(h[0:8])); 0 }\n" +
		"    }\n" +
		"}\n\n" +
		"export fn main() {\n" +
		"    av = os.Args()\n" +
		"    if len(av) < 2 { io.print_line(\"usage\"); 0 } else {\n" +
		"        (buf, read) = os.ReadAt(av[1], 0, out Head)\n" +
		"        (n, r) = read\n" +
		"        case r {\n" +
		"            result.'err(k) -> { io.print_line(\"cannot read\"); 0 }\n" +
		"            result.'ok(u) -> report(buf[0:n])\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	if got, err := runOnMayFail(t, src, "go"); err != nil || got != "usage" {
		t.Errorf("a claim's copy of a segment of a segment should build and print usage, got %q, %v", got, err)
	}
	// the claim's condition reduced as a `where`'s reaches hoistWheres, and the
	// binder rule alone keeps the program right
	core.PlantSortClaimReducedAsWhere = true
	if got, err := runOnMayFail(t, src, "go"); err != nil || got != "usage" {
		t.Errorf("with the claim reduced as a where, the binder rule should keep it right: got %q, %v", got, err)
	}
	core.PlantWhereLeavesItsBinder = true
	_, err := runOnMayFail(t, src, "go")
	core.PlantWhereLeavesItsBinder, core.PlantSortClaimReducedAsWhere = false, false
	if err == nil || !strings.Contains(err.Error(), "a segment b[lo:hi]") {
		t.Errorf("with the marks hoisted out of their binders the segment's bound should read the pair and be refused, got %v", err)
	}
}
