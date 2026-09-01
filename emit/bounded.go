package emit

import (
	"fmt"
	"strings"
)

// BOUNDED BY DEFAULT — ADR 0019's decision, made real.
//
// `int` is exact within ±(2^53−1) (ADR 0012). An integer operation the compiler
// cannot prove stays inside that window is a COMPILE ERROR, and the error is
// cleared by saying one of three things: narrow the range, ask for the trap, or
// — when it exists — declare a range above the window, which promotes the value
// to arbitrary precision.
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
		"inside the portable window, ±(2^53−1)", what, n, rep.Ops)
	for i, u := range rep.Unproven {
		if i == 3 {
			fmt.Fprintf(&b, "\n  … and %d more", len(rep.Unproven)-3)
			break
		}
		fmt.Fprintf(&b, "\n  %s", u)
	}
	b.WriteString("\n" +
		"  Outside that window the four targets disagree silently — Go and the JVM\n" +
		"  wrap, JavaScript loses precision — so this is refused rather than noted\n" +
		"  (ADR 0012, ADR 0019).\n" +
		"  Clear it by saying one of:\n" +
		"    · NARROW THE RANGE — `(sig f ((n (int 0 1000))) …)`, or a `(where …)`,\n" +
		"      so the operation is provably in range;\n" +
		"    · ASK FOR THE TRAP — build with `-checked`, which emits the target's\n" +
		"      own checked arithmetic and fails at run time instead.")
	return fmt.Errorf("%s", b.String())
}
