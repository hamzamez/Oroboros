package emit_test

import (
	"testing"

	"oroboros/ir"
	"oroboros/ir/golang"
)

// TWO FAULTS examples/os/du.oro found, the first program to reach either.

// A LOOP'S RESULTS ARE TYPED WHERE THE LOOP IS, before their uses (ir/typing.go,
// program order). The loop sits in a scope, and its results meet strings in a
// variadic list of boxes. Typed outermost region first, the list fixed them as
// strings before the loop's region was visited, and the loop's ints clashed in
// the verifier (W5); typed in program order they are ints, boxed at the list.
func TestALoopsResultsAreTypedBeforeTheirUses(t *testing.T) {
	compiles(t, `use go/os
use go/fmt
fn walk(root) {
    var stk := array { root : k ∈ 0..3 }
    loop k = 0, a = 0, b = 0 {
        3 < k -> (a, b)
        else -> { stk[k] := root; again(k + 1, a + 1, b + 2) }
    }
}
export fn main() {
    av = os.Args()
    if len(av) < 1 { () } else { (a, b) = walk(av[0]); fmt.Println(a, "and", b) }
}
`)
}

// compiles runs the drivers' pipeline on src's first export, on Go, and prints it.
func compiles(t *testing.T, src string) {
	t.Helper()
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		t.Fatal(err)
	}
	name := prog.Exports[0]
	f, _, err := ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	// AND PRINTED, as gen prints it: the step to IR_P verifies the types (W5).
	if _, err := golang.FromFunc(tg, f); err != nil {
		t.Fatalf("not printed: %v", err)
	}
}
