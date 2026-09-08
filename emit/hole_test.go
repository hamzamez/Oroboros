package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A TEMPLATE NAMES ITS TENTH OPERAND WITH A BRACED HOLE.
//
// `%1`…`%9` cannot be extended: `%12` would have to mean operand 12 in a
// template with twelve operands and operand 1 followed by the character `2` in
// one with fewer, so a template's meaning would depend on its arity. That is
// the kind of context-dependent parse this project refuses elsewhere, so the
// wide form is spelled differently rather than parsed cleverly.
//
// It exists because the widest entry point in the Windows SDK takes fourteen
// arguments and 1.0% of the declarable API is past nine (win32-2026-09-06).
func TestABracedHoleNamesAWideOperand(t *testing.T) {
	ops := make([]place, 12)
	for i := range ops {
		ops[i] = place{text: string(rune('a' + i))}
	}
	got, err := fillAsm("mov %{12}, %{10}\nadd %1, %{11}", place{text: "rax"}, ops, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := "mov l, j\nadd a, k"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// THE BARE FORM STILL MEANS WHAT IT ALWAYS MEANT, which is the property that
// makes the braced one safe to add: `%12` is operand 1 followed by `2`, whatever
// the arity, so no existing template changed meaning.
func TestABareHoleIsStillOneDigit(t *testing.T) {
	ops := make([]place, 12)
	for i := range ops {
		ops[i] = place{text: string(rune('a' + i))}
	}
	got, err := fillAsm("%12", place{text: "rax"}, ops, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a2" {
		t.Errorf("got %q, want %q — a bare hole is one digit even when operand 12 exists", got, "a2")
	}
}

// An operand that does not exist is refused rather than silently empty, and an
// unclosed brace is refused rather than copied through.
func TestABracedHoleIsCheckedLikeAnyOther(t *testing.T) {
	ops := []place{{text: "a"}, {text: "b"}}
	if _, err := fillAsm("%{7}", place{text: "rax"}, ops, 0); err == nil {
		t.Error("naming operand 7 of 2 must be refused")
	}
	if _, err := fillAsm("%{7", place{text: "rax"}, ops, 0); err == nil {
		t.Error("an unclosed %{ must be refused")
	}
}

// AND THE BYTE AND DWORD FORMS REACH IT TOO. `%b3` and `%er` were the shapes
// that shared the hole parser, so extending it had to leave both working — the
// sieve's `test-byte` and every `movzx` in the target file are written with
// them.
func TestTheByteFormStillWorksAndReachesAWideOperand(t *testing.T) {
	ops := make([]place, 11)
	for i := range ops {
		ops[i] = place{text: "rbx"}
	}
	got, err := fillAsm("mov %b3, %b{11}\nxor %er, %er", place{text: "rax"}, ops, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "bl, bl") || !strings.Contains(got, "eax, eax") {
		t.Errorf("byte and dword forms did not both fill: %q", got)
	}
}

// THE OUTGOING-ARGUMENT AREA IS SIZED PER PROCEDURE.
//
// Reserving room for the widest possible call in EVERY frame pushes every value
// slot 64 bytes further from rsp, and x86 encodes a displacement up to 127 in
// one byte and everything above it in four — so a hot loop that spills would
// pay, in code size, for a call it does not make. Measured: the flat 112-byte
// version changed all nine emitted windows programs and this one changes none.
func TestTheArgumentAreaIsSizedPerProcedure(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	tg.Prims["wide7"] = Prim{Name: "wide7", Args: make([]string, 7), Result: "int"}
	tg.Prims["wide14"] = Prim{Name: "wide14", Args: make([]string, 14), Result: "int"}

	call := func(name string, n int) *core.Term {
		var args []*core.Term
		for i := 0; i < n; i++ {
			args = append(args, core.Int(int64(i)))
		}
		return core.App(core.Name(name), args...)
	}
	if got := asmShadowFor(tg, core.Int(1)); got != asmShadow {
		t.Errorf("a procedure calling nothing reserves %d, want the floor %d", got, asmShadow)
	}
	// Seven arguments put three on the stack, at [rsp+32], [rsp+40], [rsp+48],
	// so 56 bytes are needed and the reservation rounds to 64.
	if got := asmShadowFor(tg, call("wide7", 7)); got != 64 {
		t.Errorf("seven arguments reserve %d, want 64", got)
	}
	// Fourteen is the widest entry point in the Windows SDK: ten on the stack,
	// the last at [rsp+104].
	if got := asmShadowFor(tg, call("wide14", 14)); got != 112 {
		t.Errorf("fourteen arguments reserve %d, want 112", got)
	}
	// A call UNDER A BINDER counts too — a wide call in a loop body is the
	// normal case, and missing it would let the template write past the frame,
	// which is a silent corruption of whatever the caller had there.
	inner := core.Fn([]string{"x"}, call("wide14", 14))
	if got := asmShadowFor(tg, inner); got != 112 {
		t.Errorf("a call under a lambda reserves %d, want 112", got)
	}
}
