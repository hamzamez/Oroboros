package main

import (
	"strings"
	"testing"

	"oroboros/core"
)

// AN UNREAD BINDING'S TYPING CLAIM IS CHECKED (ADR 0137): `s ∈ String = n; 0`
// compiled, since β dropped the unread binding and its claim, whose set is no
// range, made no mark. It is kept as a typing mark and decided by the type
// checker, read or not:
//   - unread and false, in a kept definition, inlined into main, and a
//     top-level constant's: refused, naming the set required and the claim;
//   - unread and true (a range, a Boolean): builds and runs;
//   - read and false: refused as before.
//
// The fault PlantTypingClaimDropped drops the claim with its value, and the
// unread false claims build again.
func TestAnUnreadTypingClaimIsChecked(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go and windows programs")
	}
	falseKept := "use win/print\nfn f(n ∈ 0..9) (Int) { s ∈ String = n; 0 }\nexport fn main() { print.print_int(f(3)) }\n"
	falseMain := "use win/print\nexport fn main() { s ∈ String = 3; print.print_int(7) }\n"
	falseTop := "use win/print\ns ∈ String = 3\nexport fn main() { print.print_int(7) }\n"
	readFalse := "use win/print\nfn f(n ∈ 0..9) (Int) { s ∈ String = n; s }\nexport fn main() { print.print_int(f(3)) }\n"
	holds := "use win/print\nfn f(n ∈ 0..9) (Int) { s ∈ 0..9 = n; b ∈ Bool = n < 3; w ∈ String = \"x\"; 7 }\nexport fn main() { print.print_int(f(3)) }\n"
	for _, target := range []string{"windows", "go"} {
		src := func(s string) string {
			if target == "go" {
				return strings.NewReplacer("use win/print\n", "use go/fmt\n", "print.print_int(", "fmt.Println(").Replace(s)
			}
			return s
		}
		for _, c := range []struct{ name, src string }{{"unread in a kept definition", falseKept}, {"unread in main", falseMain}, {"a constant's", falseTop}, {"read", readFalse}} {
			_, err := runOnMayFail(t, src(c.src), target)
			if err == nil || !strings.Contains(err.Error(), "String is required") {
				t.Errorf("%s: a false claim %s should be refused, naming String; got %v", target, c.name, err)
			}
		}
		if got, err := runOnMayFail(t, src(holds), target); err != nil || got != "7" {
			t.Errorf("%s: true unread claims should build and print 7, got %q, %v", target, got, err)
		}
		core.PlantTypingClaimDropped = true
		for _, s := range []string{falseKept, falseMain, falseTop} {
			if _, err := runOnMayFail(t, src(s), target); err != nil {
				t.Errorf("%s: with the claim dropped the unread false claim should build again, got %v", target, err)
			}
		}
		core.PlantTypingClaimDropped = false
	}
}
