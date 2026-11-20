package ir

import (
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A FAILED ASSUMPTION LEAVES THE FRAMES: an assumption lowering does not know
// is dropped with every trace of its attempt. An arm's guard renames its
// binder's frame entry to a π for the arm (guarded), and a failure inside the
// arm skips the undo: binary search's kept definition on windows, whose
// `where` is len(a) <= 1000000 && ∀…, then read `len(a)` in its body as the
// length of the π's value, and IR_P did not verify (logic3-2026-11-20). The
// fault leaves the frames as the failure did (PlantLowerFramesLeft).
func TestAFailedAssumptionLeavesTheFrames(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	// a guard on a's length, then a name lowering does not know
	term, err := core.ParseTerm("(if (<= (len a) 5) (frob a) false)")
	if err != nil {
		t.Fatal(err)
	}
	for _, plant := range []bool{false, true} {
		l := &lowerer{tg: tg, decl: map[V]string{}, marks: map[*core.Term][]V{}, lenOfV: map[V][2]int{}}
		r := &Region{}
		a := l.fresh()
		l.push([]V{a}, []string{"a"})
		PlantLowerFramesLeft = plant
		l.assume(bindNames(term, []string{"a"}, 0), r)
		PlantLowerFramesLeft = false
		same := len(l.frames) == 1 && l.frames[0][0] == a
		if !plant && (!same || len(r.Stmts) != 0 || l.nv != 1) {
			t.Errorf("a failed assumption left frames %v, %d statements, %d values; want [[%v]], 0, 1", l.frames, len(r.Stmts), l.nv, a)
		}
		if plant && same {
			t.Error("with the frames left, the guard's π should stand for a, so the witness can fail")
		}
	}
}

// A STRUCT CELL IN AN ARM KEEPS ITS LEAVES' SETS: a cell made in a branch's
// arm, as a `case` on an Option makes one (ADR 0146's CreateWindowExA in
// status-win), is found by every walk of the function, and its leaf read
// there is the struct's field set, CHARRANGE's LONG; with the walk that does
// not enter arms (PlantStructCellArmsUnread) the leaf has no set.
func TestAStructCellInAnArmKeepsItsLeaves(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	get := Stmt{Op: OCellGet, Args: []V{1}, Res: []V{2}, Lit: core.Int(1)}
	arm := &Region{Stmts: []Stmt{{Op: OCellFresh, Type: "windows/richedit.CHARRANGE", Res: []V{1}}, get}, T: TYield}
	f := &Func{Name: "arm", Body: &Region{Stmts: []Stmt{{Op: OConst, Lit: core.Bool(true), Res: []V{0}}}, T: TBranch, Cond: 0, Then: arm, Else: &Region{T: TYield}}}
	if got := structLeafType(tg, f, &get); got != "int -2147483648 2147483647" {
		t.Errorf("the leaf in an arm is %q; want LONG's set", got)
	}
	PlantStructCellArmsUnread = true
	got := structLeafType(tg, f, &get)
	PlantStructCellArmsUnread = false
	if got != "" {
		t.Errorf("with the arms unread the leaf should have no set, so the witness can fail: %q", got)
	}
}
