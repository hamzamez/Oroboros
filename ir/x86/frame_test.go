package x86

import (
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/plan"
)

// These two came from emit's term-backend tests (hole_test, borrow_test) with
// the backend they tested (irstep4a-2026-09-26): the same properties, of the
// IR's x86 printer.

func windows(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget(filepath.Join("..", "..", "targets", "windows"))
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

// THE OUTGOING-ARGUMENT AREA IS SIZED PER PROCEDURE.
//
// Reserving room for the widest possible call in EVERY frame pushes every value
// slot 64 bytes further from rsp, and x86 encodes a displacement up to 127 in
// one byte and everything above it in four, so a hot loop that spills would
// pay, in code size, for a call it does not make.
func TestTheArgumentAreaIsSizedPerProcedure(t *testing.T) {
	tg := windows(t)
	tg.Prims["wide7"] = emit.Prim{Name: "wide7", Args: make([]string, 7), Result: "int"}
	tg.Prims["wide14"] = emit.Prim{Name: "wide14", Args: make([]string, 14), Result: "int"}
	shadow := func(body *ir.Region) int {
		p := newPrinter(tg, &ir.Func{Body: body})
		return p.shadowFor()
	}
	call := func(name string) ir.Stmt { return ir.Stmt{Op: ir.OCall, Name: name} }
	if got := shadow(&ir.Region{}); got != emit.AsmShadowFloor {
		t.Errorf("a procedure calling nothing reserves %d, want the floor %d", got, emit.AsmShadowFloor)
	}
	// Seven arguments put three on the stack, at [rsp+32], [rsp+40], [rsp+48],
	// so 56 bytes are needed and the reservation rounds to 64.
	if got := shadow(&ir.Region{Stmts: []ir.Stmt{call("wide7")}}); got != 64 {
		t.Errorf("seven arguments reserve %d, want 64", got)
	}
	// Fourteen is the widest entry point in the Windows SDK.
	if got := shadow(&ir.Region{Stmts: []ir.Stmt{call("wide14")}}); got != 112 {
		t.Errorf("fourteen arguments reserve %d, want 112", got)
	}
	// A call inside a LOOP counts too: missing it would let the template write
	// past the frame, a silent corruption of whatever the caller had there.
	loop := ir.Stmt{Op: ir.OLoop, Sub: []*ir.Region{{Stmts: []ir.Stmt{call("wide14")}}}}
	if got := shadow(&ir.Region{Stmts: []ir.Stmt{loop}}); got != 112 {
		t.Errorf("a call inside a loop reserves %d, want 112", got)
	}
}

// borrowing builds a printer whose seven value registers are all in use, and
// values %0…%3: %0 in rbx, the rest in frame slots.
func borrowing(t *testing.T) (*printer, []ir.V) {
	t.Helper()
	tg := windows(t)
	f := &ir.Func{Body: &ir.Region{}, Types: []string{"int", "int", "int", "int"}}
	p := newPrinter(tg, f)
	p.pl = plan.New(tg, f)
	p.loc = make([]loc, 4)
	p.shadow = emit.AsmShadowFloor
	for _, r := range valueGP {
		p.usedGP[r] = true
	}
	p.loc[0] = loc{reg: "rbx"}
	for v := 1; v < 4; v++ {
		p.loc[v] = loc{slot: v}
	}
	p.slots = 3
	return p, []ir.V{0, 1, 2, 3}
}

// A TEMPLATE WITH MORE SLOT OPERANDS THAN SCRATCH BORROWS A VALUE REGISTER,
// saved to a frame slot and restored after, never pushed: rsp does not move
// inside a procedure, and a push would shift the 16-byte alignment a callee's
// own aligned spill depends on.
func TestATemplateBorrowsAValueRegisterWhenScratchRunsOut(t *testing.T) {
	p, vs := borrowing(t)
	out, restore := p.operands(vs[1:], []string{scratchA, scratchB}, "rax")
	for i, o := range out {
		if isMem(o) || o == "" {
			t.Errorf("operand %d is %q, not a register", i, o)
		}
	}
	restore()
	body := p.b.String()
	r := out[2]
	save := "mov " + p.text(loc{slot: p.borrow[0]}) + ", " + r
	load := "mov " + r + ", " + p.text(loc{slot: p.borrow[0]})
	if i, j := strings.Index(body, save), strings.LastIndex(body, load); i < 0 || j < 0 || i > j {
		t.Errorf("%s is not saved before and restored after:\n%s", r, body)
	}
	if strings.Contains(body, "push ") || strings.Contains(body, "pop ") {
		t.Errorf("the borrow must not move rsp:\n%s", body)
	}
}

// A REGISTER ALREADY HOLDING ONE OF THIS INSTRUCTION'S OPERANDS, OR ITS
// DESTINATION, MAY NOT BE BORROWED: loading into it would destroy the operand,
// a silent wrong answer rather than a refusal.
func TestABorrowNeverTakesARegisterThisCallIsUsing(t *testing.T) {
	p, vs := borrowing(t)
	out, restore := p.operands(vs, []string{scratchA, scratchB}, "rsi")
	restore()
	if out[0] != "rbx" {
		t.Errorf("operand 0 moved from rbx to %s", out[0])
	}
	for i, o := range out[1:] {
		if o == "rbx" || o == "rsi" {
			t.Errorf("operand %d borrowed %s, which this instruction is using", i+1, o)
		}
	}
}
