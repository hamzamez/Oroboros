package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// releaseSrc carries a table round a loop, makes a fresh one from it each
// iteration, and joins two tables at an `if`: a birth, a move by `continue`, a
// move by `yield`, two borrows of one table, and deaths on both arms.
const releaseSrc = "export fn f(n ∈ 0..100) (Int) {\n" +
	"    loop k = 0, t = { var a := array { 0 : _ ∈ 0..9 }; a } {\n" +
	"        k >= n -> {\n" +
	"            u = { var c := array { 7 : _ ∈ 0..9 }; c }\n" +
	"            s = if n > 5 { t } else { u }\n" +
	"            len(s)\n" +
	"        }\n" +
	"        else -> {\n" +
	"            x = (if len(t) > 3 { t[2] } else { 0 }) % 7\n" +
	"            again(k + 1, { var b := array { 0 : _ ∈ 0..9 }; b[0] := x; b[1] := (x + len(t)) % 7; b })\n" +
	"        }\n" +
	"    }\n}\n"

// loopMoveSrc moves a table made before the loop into its carried table on one
// iteration: a move inside a loop it is defined outside of, which happens once
// an iteration, so the table is not owned and nothing of it is released.
const loopMoveSrc = "export fn f(n ∈ 0..100) (Int) {\n" +
	"    w = { var o := array { 5 : _ ∈ 0..9 }; o }\n" +
	"    loop k = 0, t = { var a := array { 0 : _ ∈ 0..9 }; a } {\n" +
	"        k >= n -> len(t)\n" +
	"        else -> again(k + 1, if k == 3 { w } else { { var b := array { 0 : _ ∈ 0..9 }; b[0] := len(t) % 7; b } })\n" +
	"    }\n}\n"

// releaseEntry runs the windows pipeline on src and counts the frees printed.
func releaseEntry(t *testing.T, src string) (int, error) {
	t.Helper()
	_, frees, err := releaseCounts(t, src)
	return frees, err
}

// releaseCounts runs the windows pipeline on src and counts the allocations
// and the frees printed.
func releaseCounts(t *testing.T, src string) (int, int, error) {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
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
	f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
	if err != nil {
		return 0, 0, err
	}
	if err := ToP(tg, f); err != nil {
		return 0, 0, err
	}
	allocs, frees := 0, 0
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall && strings.HasSuffix(s.Name, "HeapFree") {
				frees++
			}
			if s.Op == OCall && strings.HasSuffix(s.Name, "HeapAlloc") {
				allocs++
			}
		}
	})
	return allocs, frees, nil
}

// A TABLE IS RELEASED AT ITS DEATH (ADR 0096): the carried table on the arm
// that makes its successor, the fresh table nothing moves on, and whichever
// of the two the join did not take; W11 accepts each release.
func TestATableIsReleasedAtItsDeath(t *testing.T) {
	frees, err := releaseEntry(t, releaseSrc)
	if err != nil {
		t.Fatal(err)
	}
	// t's on the loop's continuing arm, and t, u and s on its exit: the join
	// takes one of t and u, and the other dies on that arm of the `if`.
	if frees < 4 {
		t.Errorf("want a release for each death, at least 4, got %d", frees)
	}
	// A MOVE INSIDE A LOOP the table is defined outside of is a leak, never a
	// refusal: w is moved once an iteration, so nothing it reaches is owned.
	if _, err := releaseEntry(t, loopMoveSrc); err != nil {
		t.Errorf("a move inside a loop: %v", err)
	}
}

// THE PLANTED FAULTS, each a way the pass could free storage a reference
// still reaches, and each refused by W11 before anything is printed.
func TestAReleaseW11RefusesIsAFault(t *testing.T) {
	for _, c := range []struct {
		name  string
		plant *bool
		src   string
	}{
		{"released after its first use", &PlantReleaseFirstUse, releaseSrc},
		{"released where it was moved", &PlantReleaseMoved, releaseSrc},
		{"moved inside a loop it is defined outside", &PlantReleaseInLoop, loopMoveSrc},
	} {
		*c.plant = true
		_, err := releaseEntry(t, c.src)
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), "W11") {
			t.Errorf("%s: want W11's refusal, got %v", c.name, err)
		}
	}
}

// deadBuildSrc is a buffer whose scope's value is an integer, a read of it,
// and nestedBuildSrc the JSON tree's shape: a stack inside the build of a node
// table, its scope's value the node table frozen, the stack frozen into
// nothing. Each allocation has its release.
const deadBuildSrc = "export fn f(n ∈ 0..100) (Int) {\n" +
	"    var stk := array { 0 : _ ∈ 0..31 }\n" +
	"    stk[n % 32] := n\n" +
	"    stk[(n + 1) % 32]\n" +
	"}\n"

const nestedBuildSrc = "export fn f(n ∈ 0..100) (Int) {\n" +
	"    t = {\n" +
	"        var nodes := array { 0 : _ ∈ 0..9 }\n" +
	"        var stk := array { 0 : _ ∈ 0..3 }\n" +
	"        stk[n % 4] := n % 10\n" +
	"        nodes[n % 10] := stk[n % 4]\n" +
	"        nodes\n" +
	"    }\n" +
	"    t[n % 10]\n" +
	"}\n"

// A BUILD'S BUFFER NOT FROZEN OUT DIES AT THE BUILD'S END (gauntlet-2026-11-16):
// the windows JSON tree leaked its stack and its worklist, 18 KiB a call,
// because no table value carried their storage out of their builds. THE
// PLANTED FAULT releases every build's storage, the node table's too, which is
// frozen into the outer build's value and read after: W11 refuses it.
func TestABuildsBufferNotFrozenOutIsReleased(t *testing.T) {
	for _, c := range []struct{ name, src string }{{"an integer scope", deadBuildSrc}, {"a stack inside a node table's build", nestedBuildSrc}} {
		allocs, frees, err := releaseCounts(t, c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if allocs == 0 || frees != allocs {
			t.Errorf("%s: %d allocation(s) and %d release(s), want one release each", c.name, allocs, frees)
		}
	}
	PlantReleaseFrozenBuild = true
	defer func() { PlantReleaseFrozenBuild = false }()
	if _, _, err := releaseCounts(t, nestedBuildSrc); err == nil || !strings.Contains(err.Error(), "W11") {
		t.Errorf("with a frozen build's storage released: %v; want W11's refusal", err)
	}
}

// coerceSrc computes a table, makes it a string by ρ⁻¹ (ASCII's, which needs no
// obligation), and uses the string after: the table is the string's storage.
const coerceSrc = "use io\n" +
	"use llvm/str as str\n" +
	"export fn f(n ∈ 0..100) (Int) {\n" +
	"    b = alloc(array { 104 + k % 2 : k ∈ 0..n })\n" +
	"    s = str.text_ascii(b)\n" +
	"    io.print_line(s ++ s)\n" +
	"    n\n" +
	"}\n"

// AN IDENTITY COERCION MOVES ITS TABLE (strings-2026-11-16): llvm/str.text is
// the identity on the representation, so the string is the table's storage
// and the table must not be released while the string lives. THE PLANTED
// FAULT reads the coercion as a borrow: the table is then released, one free
// more, under its string.
func TestAnIdentityCoercionMovesItsTable(t *testing.T) {
	_, moved, err := releaseCounts(t, coerceSrc)
	if err != nil {
		t.Fatal(err)
	}
	PlantCoercionBorrows = true
	defer func() { PlantCoercionBorrows = false }()
	_, borrowed, err := releaseCounts(t, coerceSrc)
	if err != nil {
		t.Fatal(err)
	}
	if borrowed != moved+1 {
		t.Errorf("frees with the coercion a move %d, a borrow %d; want the borrow to free the string's table, one more", moved, borrowed)
	}
}
