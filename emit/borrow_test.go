package emit

import (
	"strings"
	"testing"
)

// A TEMPLATE WITH MORE SPILLED OPERANDS THAN SCRATCH BORROWS A VALUE REGISTER.
//
// The scratch pool is two registers and a table store has three operands, so
// the one thing the language's primary data structure needs did not fit its own
// backend. The STRUCTURAL store has a way out — form the address with `lea`
// first, freeing the index register before the value needs one — and a DECLARED
// prim does not, because its template is opaque data.
//
// Three programs were refused for it, and the third is the one that says what
// the defect really was: `merge-sort`'s failure was not in the sort but in
// `lib/win/fmt.oro`'s `print-int`, unable to make a call that was always legal
// because the program's own live values had exhausted the value pool.
//
// It is a unit test because the shape cannot be hand-written down small: it
// needs seven live values before the eighth operand spills, which is why no
// construct test had ever produced one.
func TestATemplateBorrowsAValueRegisterWhenScratchRunsOut(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	e := newAsmEmitter(tg)
	// Exhaust the value pool, which is what makes an operand spill at all.
	for range asmValueGP {
		e.alloc(false)
	}
	ops := []place{e.allocSlot(false), e.allocSlot(false), e.allocSlot(false)}

	live, left, borrowed := e.materializeBest(ops)
	if left != 0 {
		t.Fatalf("%d operand(s) left in memory; a three-operand template is then refused", left)
	}
	if len(borrowed) != 1 {
		t.Fatalf("borrowed %d registers, want exactly the one scratch could not cover", len(borrowed))
	}
	for i, p := range live {
		if !p.reg() {
			t.Errorf("operand %d is %q, not a register", i, p.text)
		}
	}

	// AND IT MUST BE PUT BACK. A value register holds something; the borrow is
	// only sound because the frame slot gives it back before anything reads it.
	e.restoreBorrows(borrowed)
	body := e.buf.String()
	r := borrowed[0].reg
	save := "mov " + borrowed[0].save.text + ", " + r
	load := "mov " + r + ", " + borrowed[0].save.text
	if !strings.Contains(body, save) {
		t.Errorf("no save of %s:\n%s", r, body)
	}
	if !strings.Contains(body, load) {
		t.Errorf("no restore of %s:\n%s", r, body)
	}
	if strings.Index(body, save) > strings.Index(body, load) {
		t.Errorf("%s is restored before it is saved:\n%s", r, body)
	}
	// SAVED TO A FRAME SLOT AND NOT PUSHED, because rsp does not move inside a
	// procedure — every push is in the prologue — and a push would shift the
	// 16-byte alignment a callee's own aligned spill depends on. That is a fault
	// inside kernel32 with nothing in the traceback pointing here.
	if strings.Contains(body, "push ") || strings.Contains(body, "pop ") {
		t.Errorf("the borrow must not move rsp:\n%s", body)
	}
}

// A REGISTER ALREADY HOLDING ONE OF THIS CALL'S OPERANDS MAY NOT BE BORROWED,
// or loading a second operand into it would destroy the first — which is a
// silent wrong answer rather than a refusal.
func TestABorrowNeverTakesARegisterThisCallIsUsing(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	e := newAsmEmitter(tg)
	var held []place
	for range asmValueGP {
		held = append(held, e.alloc(false))
	}
	// One operand is already in a value register; the other two spill.
	ops := []place{held[0], e.allocSlot(false), e.allocSlot(false), e.allocSlot(false)}
	live, left, borrowed := e.materializeBest(ops)
	if left != 0 {
		t.Fatalf("%d operand(s) left in memory", left)
	}
	for _, b := range borrowed {
		if b.reg == held[0].text {
			t.Fatalf("borrowed %s, which is already carrying operand 0", b.reg)
		}
	}
	if live[0].text != held[0].text {
		t.Errorf("operand 0 moved from %s to %s", held[0].text, live[0].text)
	}
}
