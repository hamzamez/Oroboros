package emit

import (
	"fmt"
	"strings"
)

// BOUNDED BY DEFAULT — ADR 0019's decision, made real.
//
// `int` is ℤ, and each target computes it natively inside its WORD (ADR 0026).
// An integer operation the compiler cannot prove stays inside the target's word
// is a COMPILE ERROR on that target, and the error is cleared by saying one of
// three things: narrow the range, ask for the trap, or declare a range above
// the word, which promotes the value to arbitrary precision.
//
// WHY A REFUSAL AND NOT A NOTE. Outside the window the four hosts disagree, and
// they disagree SILENTLY:
//
//	fib(100)  Go   3736710778780434371     wrapped at int64
//	          Java 3736710778780434371     wrapped at int64
//	          JS   354224848179262000000   binary64, precision lost
//	          true 354224848179261915075
//
// One source, three answers, none of them right, compiling with only a note.
// That is the thing ADR 0012's window exists to exclude, and a note does not
// exclude it.
//
// WHY NOT TRAP BY DEFAULT, which is Swift's and Zig's answer: our proof rate
// makes a compile-time refusal affordable, and a refusal names the OPERATION
// where a trap names a stack frame. Trapping stays available and is the second
// escape.
func Unbounded(what string, rep *IntervalReport) error {
	if rep == nil || rep.Proven == rep.Ops {
		return nil
	}
	n := rep.Ops - rep.Proven
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d of %d integer operation(s) cannot be proven to stay "+
		"inside the word of target %s, %s", what, n, rep.Ops, rep.Target, rep.Word)
	for i, u := range rep.Unproven {
		if i == 3 {
			fmt.Fprintf(&b, "\n  … and %d more", len(rep.Unproven)-3)
			break
		}
		fmt.Fprintf(&b, "\n  %s", u)
	}
	// BOUNDED, BUT NOT BY THIS TARGET'S WORD: the program is not wrong, it is
	// not portable to this target as written (ADR 0026). Declaring the range
	// keeps it a machine word where one holds it and makes it arbitrary
	// precision here — ADR 0019's third escape, and the one that fits.
	if rep.Outside {
		b.WriteString("\n  An operation above is BOUNDED, only not by this target's word: targets whose\n" +
			"  word holds that interval accept the program, and this one refuses it. To compile it\n" +
			"  here too, DECLARE THE RANGE — e.g. a result of `(int 0 N)` — which is arbitrary\n" +
			"  precision on this target and a machine word where one holds it. `go run ./cmd/portable`\n" +
			"  reports which targets accept the program.")
	}
	b.WriteString("\n" +
		"  Outside its word a target's arithmetic does not compute the integer the\n" +
		"  program means — Go, the JVM and x86 wrap, JavaScript loses precision — so\n" +
		"  this is refused rather than noted (ADR 0019, ADR 0026).\n" +
		"  Clear it by saying one of:\n" +
		"    · NARROW THE RANGE — `(sig f ((n (int 0 1000))) …)`, or a `(where …)`,\n" +
		"      so the operation is provably in range;\n" +
		"    · ASK FOR THE TRAP — build with `-checked`, which emits the target's\n" +
		"      own checked arithmetic and fails at run time instead.")
	return fmt.Errorf("%s", b.String())
}
