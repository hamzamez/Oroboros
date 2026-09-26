package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/x86"
)

func winEmit(t *testing.T, src string) string {
	t.Helper()
	emit.ResetAsm()
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
	if err := emit.Check(tg, "t", nf); err != nil {
		t.Fatalf("check: %v", err)
	}
	code, err := x86.FromResidual(tg, "t", nil, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return code
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
	// The back edge: from the continue's arm to the jump to the head.
	i, j := strings.Index(code, "\nLelse"), strings.Index(code, "jmp Ltop")
	if i < 0 || j < i {
		t.Fatalf("no loop emitted:\n%s", code)
	}
	body := code[i:j]
	// Not one move on the back edge. acc reads i and i is changing, and
	// ordering the two updates makes the question disappear: the IR printer
	// schedules acc + i before i + 1 (L6), so both are in place.
	if strings.Contains(body, "mov ") {
		t.Errorf("the back edge staged a value it did not need to:\n%s", body)
	}
	if strings.Count(body, "add ") != 2 {
		t.Errorf("expected both updates as one add each:\n%s", body)
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
	body := between(code, "Ltop", "Lexit")
	if n := strings.Count(body, "mov "); n != 3 {
		t.Errorf("a swap should cost one copy and two moves, got %d:\n%s", n, body)
	}
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
		(fn (x) (let a (kernel32.GetTickCount64)
            (x64.add a x)))
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
	for _, d := range emit.AsmData {
		if strings.Contains(d, "3FB999999999999A") {
			found = true
		}
	}
	if !found {
		t.Errorf("0.1 was not emitted as its binary64 bits: %v", emit.AsmData)
	}
}
