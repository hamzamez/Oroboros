package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

func TestLoadTargets(t *testing.T) {
	for _, name := range []string{"go", "js", "java", "blas"} {
		tg, err := LoadTarget("../targets/" + name + ".oro")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if tg.Name != name {
			t.Errorf("%s: name is %q", name, tg.Name)
		}
		if len(tg.Prims) == 0 {
			t.Errorf("%s: no primitives", name)
		}
		t.Logf("%-5s %2d primitives, %d types", name, len(tg.Prims), len(tg.Types))
	}
}

// JS declares no types at all, which is the point.
func TestJSTargetIsUntyped(t *testing.T) {
	tg, err := LoadTarget("../targets/portable-js.oro")
	if err != nil {
		t.Fatal(err)
	}
	if len(tg.Types) != 0 {
		t.Errorf("JS should declare no types, got %v", tg.Types)
	}
}

// The acceptance test for requirement 4: a host function is added by editing a
// target file and nothing else. `sqrt` exists in all three targets and in no Go
// source — if this fails, primitives have leaked back into the compiler.
//
// The name is qualified since arithmetic.md §5 moved it into num/f64, which is
// itself part of what this test asserts: a primitive moving between modules is
// a target-file edit and nothing more.
func TestHostFunctionIsDeclaredNotCompiledIn(t *testing.T) {
	for _, name := range []string{"portable-go", "portable-js", "portable-java"} {
		tg, err := LoadTarget("../targets/" + name + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		p, ok := tg.Prims["num/f64.sqrt"]
		if !ok {
			t.Errorf("%s does not declare sqrt", name)
			continue
		}
		if p.Kind != "expr" || p.Form == "" {
			t.Errorf("%s: sqrt should be an expression with a template, got %+v", name, p)
		}
	}
	// Go's declaration carries the import; the others need none.
	tg, _ := LoadTarget("../targets/portable-go.oro")
	if tg.Prims["num/f64.sqrt"].Import != "math" {
		t.Errorf("Go's sqrt should declare import math, got %q", tg.Prims["num/f64.sqrt"].Import)
	}
}

// A target may be a DIRECTORY of files, merged — docs/spec/target-native.md.
// targets/go/ is three files: the header, Go's builtins, and the whole of fmt.
func TestTargetDirectoryMerges(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Name != "go" {
		t.Errorf("merged target is named %q", tg.Name)
	}
	for _, n := range []string{"go.+", "go.%", "go.&^", "go.len", "go.delete",
		"go/fmt.Println", "go/fmt.Sprintf", "loop", "if", "let"} {
		if _, ok := tg.Prims[n]; !ok {
			t.Errorf("merged target is missing %s", n)
		}
	}
	// The build command comes from the header file, the primitives from the
	// others; merging is a union across all three.
	if tg.Build == "" {
		t.Error("the header's build command was lost in the merge")
	}
	// fold-range is deliberately absent: `loop` subsumes it (ADR 0015).
	if _, ok := tg.Prims["fold-range"]; ok {
		t.Error("the native target should not declare fold-range")
	}
}

// All three native targets are directories with the SAME structural set of
// three — let, if, loop — and each declares its host's own names.
func TestNativeTargetsAreThreeStructural(t *testing.T) {
	for _, c := range []struct {
		dir   string
		names []string
	}{
		{"go", []string{"go.+", "go.%", "go/fmt.Println"}},
		{"js", []string{"js.+", "js.===", "js.??", "js/Math.floor", "js/console.log"}},
		{"java", []string{"java.+", "java.>>>", "java/Math.abs", "java/Map.get"}},
	} {
		tg, err := LoadTarget("../targets/" + c.dir)
		if err != nil {
			t.Errorf("%s: %v", c.dir, err)
			continue
		}
		for _, n := range append(c.names, "let", "if", "loop") {
			if _, ok := tg.Prims[n]; !ok {
				t.Errorf("%s is missing %s", c.dir, n)
			}
		}
		for _, gone := range []string{"fold-range", "fold-range2", "make-vec"} {
			if _, ok := tg.Prims[gone]; ok {
				t.Errorf("%s should not declare %s", c.dir, gone)
			}
		}
		// The four reserved type names must be spelled by every target that
		// uses integer or float literals and conditionals.
		for _, ty := range []string{"int", "f64", "bool"} {
			if _, ok := tg.Types[ty]; !ok {
				t.Errorf("%s does not spell the reserved type %s", c.dir, ty)
			}
		}
	}
}

// The language's own constructs are INJECTED into every target, and a target
// may neither decline nor declare one.
//
// `if` was already like this (ADR 0017). `let` and `loop` were not: every one
// of eleven target files declared them identically, so a third-party author
// could forget one and make an ADR 0015 language construct silently
// unavailable — a construct in the core that a target can decline, which is a
// library with a portability claim rather than part of the language.
func TestEveryTargetHasTheLanguagesConstructs(t *testing.T) {
	for _, name := range []string{"go", "js", "java", "windows", "blas.oro",
		"portable-go.oro", "portable-js.oro", "portable-java.oro",
		"tutorial.oro", "tutorial-native.oro", "tutorial-sloppy.oro"} {
		tg, err := LoadTarget("../targets/" + name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []struct{ n, k string }{
			{"if", "cond"}, {"let", "let"}, {"loop", "iterate"},
		} {
			p, ok := tg.Prims[want.n]
			if !ok {
				t.Errorf("%s: %s is not available", name, want.n)
			} else if p.Kind != want.k {
				t.Errorf("%s: %s has kind %q, want %q", name, want.n, p.Kind, want.k)
			}
		}
	}
}

// And declaring one is an error, not a redundancy. A target author who writes
// the line has misunderstood where the boundary is, and the message says so.
func TestDeclaringALanguageConstructIsAnError(t *testing.T) {
	for _, line := range []string{
		`(structural let  let     pure)`,
		`(structural loop iterate pure)`,
		`(structural if   cond    pure)`,
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "t.oro")
		src := "(target t\n  (type int \"int\")\n  " + line + ")\n"
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTarget(path)
		if err == nil || !strings.Contains(err.Error(), "belongs to the language") {
			t.Errorf("%s must be refused, got %v", line, err)
		}
	}
}

// ELEMENT WIDTH FROM THE RANGE (ADR 0003, elemwidth-2026-08-27).
//
// A range is a type, the target declares what it can store a range in, and the
// narrowest declared representation that CONTAINS the range wins. No host fact
// lives in Go here: the JVM's signed `byte` excludes 0..255 by declaring
// -128..127, and `short` is selected because it is next, not because anything
// knows what a JVM is.
func TestRangeSelectsRepresentation(t *testing.T) {
	for _, c := range []struct{ target, ty, want string }{
		{"go", "array int 0 255", "[]byte"},
		{"go", "array int -128 127", "[]int8"},
		{"go", "array int 0 70000", "[]uint32"},
		{"go", "array int 0 9007199254740991", "[]int"}, // nothing narrower holds it
		{"go", "array int", "[]int"},                    // a plain int is not a range
		{"java", "array int 0 255", "short[]"},          // byte is SIGNED here
		{"java", "array int -128 127", "byte[]"},
		{"java", "array int 0 100000", "int[]"},
	} {
		tg, err := LoadTarget("../targets/" + c.target)
		if err != nil {
			t.Fatalf("%s: %v", c.target, err)
		}
		if got := tg.ty(c.ty); got != c.want {
			t.Errorf("%s: ty(%q) = %q, want %q", c.target, c.ty, got, c.want)
		}
	}
}

// A range says what a value IS; the width belongs to its storage alone. A local
// reading a byte array is an integer, or a counter over one would overflow at
// 255 while the language says integers do not overflow.
func TestRangeDoesNotNarrowAValue(t *testing.T) {
	if got := core.ValueType("int 0 255"); got != "int" {
		t.Errorf("ValueType(range) = %q, want int", got)
	}
	if got := core.ValueType("f64"); got != "f64" {
		t.Errorf("ValueType must pass non-ranges through, got %q", got)
	}
	if _, _, ok := core.IntRange("int"); ok {
		t.Error("a plain int must not read as a range: it is the portable window, not a claim")
	}
}

// A BUFFER'S ELEMENT RANGE IS INFERRED FROM EVERY STORE, not from the first.
//
// Taking the first was a silent wrong answer: tree.oro's node table stores a
// tag of 1..5 into one slot and a node index of up to 511 into another, so the
// first store said one byte and the rest truncated. The differential suite
// caught it as windows returning 4030140 where the others returned 4040171.
func TestBufferElemJoinsEveryStore(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	typeOf := func(*core.Term) string { return "" }
	for _, c := range []struct{ name, src, want string }{
		{"literals join", `(fn (b) (set (set b 0 93) 1 125))`, "int 0 125"},
		{"a big store widens", `(fn (b) (set (set b 0 5) 1 511))`, "int 0 511"},
		{"zero is always an element", `(fn (b) (set b 0 40))`, "int 0 40"},
		{"a negative store", `(fn (b) (set b 0 -7))`, "int -7 0"},
		{"a conditional joins its branches", `(fn (b) (set b 0 (if c 125 93)))`, "int 0 125"},
		{"one opaque store gives up", `(fn (b) (set (set b 0 5) 1 n))`, "int"},
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		lam := forms[0].Term
		got := bufferElem(lam.Closed(), lam.Params[0], typeOf)
		if got != c.want {
			t.Errorf("%s: bufferElem = %q, want %q", c.name, got, c.want)
		}
		if c.want == "int 0 125" && tg.ty("array "+got) != "[]byte" {
			t.Errorf("%s: %q should store in a byte, got %q", c.name, got, tg.ty("array "+got))
		}
	}
}

// A range too WIDE costs space; a range too NARROW is a silent wrong answer. So
// where two buffers cannot be told apart the stores merge, which can only
// widen.
func TestBufferRootDistinguishesTwoBuffers(t *testing.T) {
	forms, _ := core.Read(`(fn (a) (fn (b) (set (set a 0 5) 1 500)))`)
	inner := forms[0].Term.Closed()
	if got := BufferRoot(inner); got != "" {
		t.Errorf("an unopened lambda body has no nameable root, got %q", got)
	}
	forms, _ = core.Read(`(set (set nodes 0 5) 1 500)`)
	if got := BufferRoot(forms[0].Term); got != "nodes" {
		t.Errorf("BufferRoot through a threaded set = %q, want nodes", got)
	}
}

// TRYING TO BREAK INTERVAL-DERIVED NARROWING.
//
// This is the first place an analysis result decides how many BITS a value
// gets, so a wrong bound is a silent wrong answer rather than a slow program.
// The differential suite cannot catch it — every target narrows on the same
// decision, so they agree and are wrong together — which leaves these.
//
// The property is CONTAINMENT, not tightness: the claimed range must hold every
// value the program can actually store. Over-approximating is the safe
// direction and costs only space.
//
// Two of these were written expecting a refusal and got a claim, and the claims
// were right: `0 * 3` stays 0 forever, and `i*j` for i<10 and j stepping by 1e9
// really is under 9.9e10. The tests were wrong, which is the correct way round
// for this to go.
func TestBufferRangeContainsEveryStore(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, src string
		lo, hi    int64 // the true extremes, computed by hand
	}{
		{"a counter bounded by its own guard", `
			(fn (b) (loop ((b b) (i 0))
			  (go.>= i 512) b
			  else (again (set b i i) (go.+ i 1))))`, 0, 511},
		{"a multiplied accumulator that never leaves zero", `
			(fn (b) (loop ((b b) (i 0) (acc 0))
			  (go.>= i 10) b
			  else (again (set b i acc) (go.+ i 1) (go.* acc 3))))`, 0, 0},
		{"a product of two loop variables", `
			(fn (b) (loop ((b b) (i 0) (j 0))
			  (go.>= i 10) b
			  else (again (set b i (go.* i j)) (go.+ i 1) (go.+ j 1000000000))))`,
			0, 81000000000},
		{"a doubling accumulator", `
			(fn (b) (loop ((b b) (i 0) (acc 1))
			  (go.>= i 8) b
			  else (again (set b i acc) (go.+ i 1) (go.* acc 2))))`, 1, 128},
		{"a negative store", `
			(fn (b) (loop ((b b) (i 0))
			  (go.>= i 10) b
			  else (again (set b i (go.- 0 i)) (go.+ i 1))))`, -9, 0},
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, ok := BufferRange(tg, forms[0].Term)
		if !ok {
			continue // refusing is always safe
		}
		lo, hi, _ := core.IntRange(got)
		if lo > c.lo || hi < c.hi {
			t.Errorf("%s: claimed %q, which does not contain the true %d..%d — "+
				"a range too narrow truncates on store", c.name, got, c.lo, c.hi)
		}
		if lo > 0 {
			t.Errorf("%s: claimed %q, which excludes 0 — build zero-fills and an "+
				"unwritten slot reads 0", c.name, got)
		}
	}
}

// And what it must refuse: a store nothing in the lambda can bound. Failure is
// the safe direction and it has to be the default.
func TestBufferRangeRefusesWhatItCannotBound(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, src string }{
		{"a value read out of the buffer itself", `
			(fn (b) (loop ((b b) (i 1))
			  (go.>= i 10) b
			  else (again (set b i (go.* (b (go.- i 1)) 2)) (go.+ i 1))))`},
		{"a free variable", `(fn (b) (set b 0 n))`},
		{"a parameter of the enclosing function", `(fn (b) (set b 0 (go.* n n)))`},
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got, ok := BufferRange(tg, forms[0].Term); ok {
			t.Errorf("%s: claimed %q; an unbounded store must keep the machine word",
				c.name, got)
		}
	}
}

// And the bound it DOES claim has to hold. A guard is the fact that makes this
// work at all — examples/json/tree.oro's node table is bounded by `nn < 512`
// and by nothing a literal can show.
func TestBufferRangeUsesAGuard(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(`
		(fn (b) (loop ((b b) (i 0))
		  (go.>= i 512) b
		  else (again (set b i i) (go.+ i 1))))`)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := BufferRange(tg, forms[0].Term)
	if !ok {
		t.Fatal("a store bounded by the loop's own guard must be provable")
	}
	lo, hi, _ := core.IntRange(got)
	if lo > 0 || hi < 511 {
		t.Errorf("range %q must contain 0..511 — 0 because build zero-fills, "+
			"511 because that is the largest value the guard admits", got)
	}
	if tg.ty("array "+got) != "[]uint16" {
		t.Errorf("0..511 should store in a uint16 on Go, got %q", tg.ty("array "+got))
	}
}

// INDEX NARROWING FROM THE INTERVAL ANALYSIS.
//
// Holding a value in 32 bits computes the same answer as 64 exactly when every
// intermediate stays inside 32 bits. MaxOp answers that for operations; it does
// NOT answer it for literals or for values read out of a table, so those are
// checked directly. Refusing keeps the host's widest integer, which is what
// every program emitted before this existed.
func TestFitsIndexSourceRefusesWhatMaxOpDidNotCount(t *testing.T) {
	tg, err := LoadTarget("../targets/java")
	if err != nil {
		t.Fatal(err)
	}
	raw := []string{"i", "a"}
	for _, c := range []struct {
		name string
		src  string
		want bool
	}{
		{"a small literal", `7`, true},
		{"a literal past int32", `4294967296`, false},
		{"a loop variable", `i`, true},
		{"a free name", `q`, false},
		{"addition", `(java.+ i 1)`, true},
		{"multiplication", `(java.* i i)`, true},
		// Division is BOUNDED by the analysis and not joined into MaxOp, so
		// trusting it would trust a number that was never checked.
		{"division", `(java./ i 2)`, false},
		// A table read's element range is not known to this pass.
		{"a table read", `(a i)`, false},
		{"a conditional over literals", `(if c 3 9)`, true},
		{"a conditional hiding a big literal", `(if c 3 4294967296)`, false},
		{"a conditional hiding a table read", `(if c 3 (a i))`, false},
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := fitsIndexSource(tg, forms[0].Term, raw); got != c.want {
			t.Errorf("%s: fitsIndexSource(%s) = %v, want %v", c.name, c.src, got, c.want)
		}
	}
}

// And the whole-function gate: one unbounded operation anywhere refuses every
// loop in the method. Coarse, and the safe coarseness.
func TestNarrowByIntervalNeedsTheWholeFunctionToFit(t *testing.T) {
	tg, err := LoadTarget("../targets/java")
	if err != nil {
		t.Fatal(err)
	}
	forms, _ := core.Read(`(again (java.+ i 1))`)
	body := forms[0].Term
	raw := []string{"i"}
	inits := []*core.Term{{Kind: core.KInt, Int: 0}}
	if nw := NarrowByInterval(tg, false, body, raw, inits); nw != nil {
		t.Error("a method with an unbounded operation must narrow nothing")
	}
	if nw := NarrowByInterval(tg, true, body, raw, inits); nw == nil || !nw["i"] {
		t.Error("a counter whose every value MaxOp bounded must narrow")
	}
}
