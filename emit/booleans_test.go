package emit

import (
	"strings"
	"testing"
)

// `and`, `or` and `not` are reader sugar over `if` (ADR 0017), and each backend
// must put the HOST's operator back. Emitting the conditional would be lowering
// further than the target requires — the objection that kept them primitives in
// ADR 0012, and the reason the recogniser exists.
func TestConnectiveEmitsHostOperator(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(fn (a b) (and (go.< a b) (go.< b 9)))`, "&&"},
		{`(fn (a b) (or (go.< a b) (go.< b 9)))`, "||"},
		{`(fn (a b) (not (go.< a b)))`, "!"},
	} {
		nf := reduce(t, "(use go)\n"+c.src, "go")
		tg, err := LoadTarget("../targets/go")
		if err != nil {
			t.Fatal(err)
		}
		code, err := Func(tg, "t", nil, nf)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s did not emit %q:\n%s", c.src, c.want, code)
		}
		if strings.Contains(code, "if ") {
			t.Errorf("%s lowered to a conditional:\n%s", c.src, code)
		}
	}
}

// On the host with no operator, `and` in GUARD position is the dragon book's
// jumping code: two compares that leave for the same label, and no boolean.
func TestAsmAndIsJumpingCode(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (a b n) (if (and (x64.setl a n) (x64.setl b n)) 1 0))
	`)
	if n := strings.Count(code, "cmp "); n != 2 {
		t.Errorf("want two compares, got %d:\n%s", n, code)
	}
	if strings.Contains(code, "setl") {
		t.Errorf("a guard materialised a boolean:\n%s", code)
	}
	if strings.Contains(code, "and ") {
		t.Errorf("the strict instruction was used where a branch is required:\n%s", code)
	}
	// Both failures leave for ONE label, which is why the commuting conversion
	// costs nothing here (booleans.md §2.7).
	if strings.Count(code, "jge Lelse") != 2 {
		t.Errorf("the two failures do not share a label:\n%s", code)
	}
}

// And in VALUE position it still short-circuits, because the generic
// conditional already lays the branches out that way. `x64.andb` — the strict
// branchless instruction — is a different name and is not reached from `and`.
func TestAsmAndInValuePositionShortCircuits(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (a b n) (and (x64.setl a n) (x64.setl b n)))
	`)
	if strings.Contains(code, "and ") {
		t.Errorf("value position used the strict instruction:\n%s", code)
	}
	if !strings.Contains(code, "cmp ") || !strings.Contains(code, "jmp ") {
		t.Errorf("value position did not branch:\n%s", code)
	}
}

// The strict instruction is still reachable under its own host name, with no
// portability claim — Ada's `and` beside `and then` (booleans.md §2.9).
func TestStrictAndIsStillAvailable(t *testing.T) {
	code := winEmit(t, `
		(use x64)
		(fn (a b n) (x64.andb (x64.setl a n) (x64.setl b n)))
	`)
	if !strings.Contains(code, "and ") {
		t.Errorf("x64.andb should emit the instruction:\n%s", code)
	}
}

// The conditional belongs to the language now, so a target declaring one is an
// error rather than a redundancy.
func TestTargetCannotDeclareTheConditional(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := tg.Prims["if"]; !ok || p.Kind != "cond" {
		t.Errorf("every target should be given `if`, got %+v", p)
	}
	for _, n := range []string{"and", "or", "not", "cond", "if"} {
		if !coreNames[n] {
			t.Errorf("%s should belong to the language", n)
		}
	}
	// And no target file declares booleans any more.
	for _, dir := range []string{"go", "js", "java", "windows"} {
		tg, err := LoadTarget("../targets/" + dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		for _, gone := range []string{"true", "false", "logic.and"} {
			if _, ok := tg.Prims[gone]; ok {
				t.Errorf("%s still declares %s", dir, gone)
			}
		}
	}
}
