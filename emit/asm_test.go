package emit

import (
	"strings"
	"testing"
)

func winTarget(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatalf("load windows target: %v", err)
	}
	return tg
}

// The point of the fourth target: the structural set did not grow. Three
// primitives are implemented in code and everything else is a template, on a
// host with no expressions at all.
func TestWindowsIsThreeStructural(t *testing.T) {
	tg := winTarget(t)
	for _, n := range []string{"let", "if", "loop",
		"x64.add", "x64.imul", "x64.setl", "windows/kernel32.WriteFile", "windows/msvcrt.printf1"} {
		if _, ok := tg.Prims[n]; !ok {
			t.Errorf("windows is missing %s", n)
		}
	}
	for _, gone := range []string{"fold-range", "fold-range2", "make-vec"} {
		if _, ok := tg.Prims[gone]; ok {
			t.Errorf("windows should not declare %s", gone)
		}
	}
	for _, ty := range []string{"int", "f64", "bool"} {
		if _, ok := tg.Types[ty]; !ok {
			t.Errorf("windows does not spell the reserved type %s", ty)
		}
	}
	if len(tg.Data) == 0 {
		t.Error("windows declares no (data …); WriteFile's out-parameter needs one")
	}
}

// %r and %u are the two holes assembly forced. %b and %e spell one register at
// three widths, which is x86's problem and not the language's.
func TestFillAsmHoles(t *testing.T) {
	dst := place{text: "rbx"}
	ops := []place{{text: "rsi"}, {text: "7", imm: true}}
	for _, c := range []struct{ form, want string }{
		{"mov %r, %1", "mov rbx, rsi"},
		{"add %r, %2", "add rbx, 7"},
		{"mov %s, %s", "mov rsi, 7"},
		{"setl %br", "setl bl"},
		{"xor %er, %er", "xor ebx, ebx"},
		{"L%u:", "L3:"},
		{"100%% done", "100% done"},
	} {
		got, err := fillAsm(c.form, dst, ops, 3)
		if err != nil {
			t.Errorf("%q: %v", c.form, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q gave %q, want %q", c.form, got, c.want)
		}
	}
	if _, err := fillAsm("mov %r, %9", dst, ops, 1); err == nil {
		t.Error("naming a ninth operand of two should be an error")
	}
	if _, err := fillAsm("mov %r, %z", dst, ops, 1); err == nil {
		t.Error("an unknown hole should be an error")
	}
}

func TestAsmPeephole(t *testing.T) {
	// A jump to the next instruction, past intervening labels, is not a jump.
	got := asmPeephole("        jmp Ldone\nLnext:\nLdone:\n        ret\n")
	if strings.Contains(got, "jmp") {
		t.Errorf("jump to the following label survived:\n%s", got)
	}
	// A conditional around a lone unconditional is one inverted branch.
	got = asmPeephole("        jl Lnext\n        jmp Ldone\nLnext:\n        ret\n")
	if !strings.Contains(got, "jge Ldone") || strings.Contains(got, "jmp") {
		t.Errorf("branch was not inverted:\n%s", got)
	}
	// And a jump that really does go somewhere else stays put.
	got = asmPeephole("        jmp Ltop\nLdone:\n        ret\n")
	if !strings.Contains(got, "jmp Ltop") {
		t.Errorf("a real jump was deleted:\n%s", got)
	}
}

// A target's storage is emitted only when a template that uses it is reached.
func TestAsmDataIsDemandDriven(t *testing.T) {
	tg := winTarget(t)
	ResetAsm()
	file := AsmFile(tg, map[string]string{"t": "t proc\n        ret\nt endp\n"}, "")
	if strings.Contains(file, "__buf") {
		t.Errorf("unused target storage was emitted:\n%s", file)
	}
	ResetAsm()
	file = AsmFile(tg, map[string]string{"t": "t proc\n        lea rbx, __buf\n        ret\nt endp\n"}, "")
	if !strings.Contains(file, "__buf") {
		t.Errorf("used target storage was dropped:\n%s", file)
	}
}
