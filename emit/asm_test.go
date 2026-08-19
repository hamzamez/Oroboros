package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

func winTarget(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatalf("load windows target: %v", err)
	}
	return tg
}

func winEmit(t *testing.T, src string) string {
	t.Helper()
	ResetAsm()
	tg := winTarget(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	nf, err := core.Normalize(terms[0], env, core.DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if err := Check(tg, "t", nf); err != nil {
		t.Fatalf("check: %v", err)
	}
	code, err := AsmProc(tg, "t", nil, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return code
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

// The loop is the host's own: a label, a compare that branches straight out,
// and a back edge. And the counter is incremented IN PLACE — `add rsi, 1`,
// not `mov t, rsi` / `add t, 1` / `mov rsi, t`, which is what separates this
// from hand-written assembly.
func TestAsmLoopIsInPlace(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (n) (loop ((i 0) (acc 0))
			(x64.setge i n)  acc
			else             (again (x64.add i 1) (x64.add acc i))))
	`)
	body := between(code, "Ltop", "Ldone")
	if body == "" {
		t.Fatalf("no loop emitted:\n%s", code)
	}
	// Not one move in the body. needTemps says this update needs staging —
	// acc reads i and i is changing — and ordering the two assignments makes
	// the question disappear.
	if strings.Contains(body, "mov ") {
		t.Errorf("the back edge staged a value it did not need to:\n%s", body)
	}
	for _, want := range []string{"cmp ", "jge ", "add "} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in the loop body:\n%s", want, body)
		}
	}
}

// A genuine cycle still needs one copy, and exactly one.
func TestAsmSwapNeedsOneCopy(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (n) (loop ((a 1) (b 2) (k 0))
			(x64.setge k n)  a
			else             (again b a (x64.add k 1))))
	`)
	body := between(code, "Ltop", "Ldone")
	if n := strings.Count(body, "mov "); n != 3 {
		t.Errorf("a swap should cost one copy and two moves, got %d:\n%s", n, body)
	}
}

// between returns the emitted lines from the first label starting with `from`
// to the first starting with `to`.
func between(code, from, to string) string {
	lines := strings.Split(code, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if start < 0 && strings.HasPrefix(t, from) && strings.HasSuffix(t, ":") {
			start = i + 1
		} else if start >= 0 && strings.HasPrefix(t, to) && strings.HasSuffix(t, ":") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return ""
}

// (jump …) is the whole of teaching a host to fold a comparison into a branch.
// Without it every guard costs a setcc and a second compare.
func TestAsmGuardIsOneCompare(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (a b) (if (x64.setl a b) 1 0))
	`)
	if strings.Contains(code, "setl") {
		t.Errorf("a comparison in guard position materialised a boolean:\n%s", code)
	}
	if n := strings.Count(code, "cmp "); n != 1 {
		t.Errorf("want one compare, got %d:\n%s", n, code)
	}
}

// The same predicate in VALUE position does need the boolean, and that is the
// declaration's other half rather than a failure.
func TestAsmComparisonAsValue(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (a b) (x64.setl a b))
	`)
	if !strings.Contains(code, "setl") {
		t.Errorf("a comparison used as a value should materialise it:\n%s", code)
	}
}

// Every value the procedure holds lives in a callee-saved register, which is
// what makes a Win32 call cost nothing: kernel32 preserves all of them, so
// nothing is saved around one.
func TestAsmCallNeedsNoSpill(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(use windows/kernel32)
		(fn (x) (let (kernel32.GetTickCount64) (fn (a) (x64.add a x))))
	`)
	if !strings.Contains(code, "call GetTickCount64") {
		t.Fatalf("no call emitted:\n%s", code)
	}
	if strings.Contains(code, "[rsp+") {
		t.Errorf("a value was spilled across a call that preserves everything:\n%s", code)
	}
}

// A float literal goes into the artifact as BITS, so the assembler's decimal
// parser never gets a vote (ADR 0009).
func TestAsmFloatLiteralIsBits(t *testing.T) {
	winEmit(t, `(use x64) (fn (x) (x64.addsd x 0.1))`)
	var found bool
	for _, d := range AsmData {
		if strings.Contains(d, "3FB999999999999A") {
			found = true
		}
	}
	if !found {
		t.Errorf("0.1 was not emitted as its binary64 bits: %v", AsmData)
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
