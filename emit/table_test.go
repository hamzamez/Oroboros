package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// linearOn runs ADR 0018's linearity check on a residual.
func linearOn(t *testing.T, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs[prog.Exports[0]], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return CheckLinear(nf, tg)
}

// refineOn runs the REFINEMENT pass, which is where a bounds obligation is
// discharged. genOn only emits, and the hole below was invisible until this
// existed — the emitter is perfectly happy to write `a[i]` for any i.
func refineOn(t *testing.T, target, src, name string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
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
	_, err = Refine(tg, name, prog.Sigs[q], nf)
	return err
}

// docs/spec/tables.md, the emitters' half. INDEXING IS APPLICATION, so `(a i)`
// lowers to each host's own indexing and no target declares any of it.

// The construct reaches every backend that has arrays, and each one spells it
// its own way.
func TestIndexingIsApplicationOnEveryTarget(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"go", `(use go)
			(export f) (sig f ((a (array f64)) (i int)) f64
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, "return a[i]"},
		{"js", `(use js)
			(export f) (sig f ((a (array any)) (i any)) any
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, "return a[i];"},
		// The (int) CAST is Java's and it is not optional: our `int` maps to
		// `long` and a Java array index must be an `int`. Without it javac
		// refuses the file with "possible lossy conversion".
		{"java", `(use java)
			(export f) (sig f ((a (array f64)) (i int)) f64
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, "a[(int) i]"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s: wanted %q in:\n%s", c.target, c.want, code)
		}
	}
}

// THE HOLE THIS BUILD FOUND, and the reason it is the most important test here.
//
// The bounds obligation used to live in the primitive's `(where …)` —
// `at-float64` declared `(and (<= 0 i) (< i (len v)))`. Making indexing
// application DELETED the primitive, and the obligation went with it: `(a i)`
// with a completely unconstrained `i` was accepted, while `(go.at-float64 a i)`
// was correctly refused. A refactor that looks clean and silently removes a
// safety property.
//
// The obligation is generated from the FORM now, which means a target author
// cannot forget it and it applies on all four targets at once. tables.md §6
// already said the right thing and it reads differently after this: bounds are
// the DOMAIN — `0 <= i < len(a)` is the condition for the application to be
// defined, not a check bolted onto an operation.
func TestAnUnprovenIndexIsRefused(t *testing.T) {
	err := refineOn(t, "go", `
		(use go)
		(export f) (sig f ((a (array f64)) (i int)) f64)
		(def f (fn (a i) (a i)))
	`, "f")
	if err == nil {
		t.Fatal("an unconstrained index must be refused")
	}
	if !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("the message must say what the coder did, got: %v", err)
	}
}

// And a loop's own bound proves it, which is where almost every real index
// gets its facts.
func TestALoopBoundProvesItsIndex(t *testing.T) {
	src := `
		(use go)
		(export f) (sig f ((a (array f64))) f64)
		(def f (fn (a)
			(loop ((acc 0.0) (i 0))
				(go.>= i (len a))  acc
				else               (again (go.f+ acc (a i)) (go.+ i 1)))))
	`
	if err := refineOn(t, "go", src, "f"); err != nil {
		t.Fatalf("a loop bounded by len must prove its own index: %v", err)
	}
	code, err := genOn(t, "go", src, "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "a[i]") {
		t.Errorf("expected an indexing:\n%s", code)
	}
}

// `(array V)` resolves through ONE declaration per target instead of an entry
// per element type. That enumeration — 54 declarations across four targets —
// is the surface this construct deletes.
func TestArrayTypeIsOneDeclaration(t *testing.T) {
	for _, c := range []struct{ target, want string }{
		{"go", "[]float64"},
		{"java", "double[]"},
	} {
		tg, err := LoadTarget("../targets/" + c.target)
		if err != nil {
			t.Fatal(err)
		}
		if got := tg.ty("array f64"); got != c.want {
			t.Errorf("%s: (array f64) is %q, want %q", c.target, got, c.want)
		}
	}
}

// A rule-table that reaches a backend has no memory and nothing to emit. The
// refusal is the construct doing its job: the rule form exists to FUSE, and one
// that survives did not.
func TestAnUnallocatedTableIsRefused(t *testing.T) {
	_, err := genOn(t, "go", `
		(use go)
		(export f) (sig f ((n int)) any)
		(def f (fn (n) (table n (fn (i) i))))
	`, "f")
	if err == nil || !strings.Contains(err.Error(), "no memory") {
		t.Errorf("a surviving rule-table must be refused, got %v", err)
	}
}

// The language's `len` and a host's own `len` are DIFFERENT KEYS, because
// tg.Prims is keyed by the qualified name. `go.len` works on maps and channels
// and stays reachable; the language's works on tables.
func TestLanguageLenDoesNotShadowTheHosts(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tg.Prims["len"]; !ok {
		t.Error("the language's `len` must be injected")
	}
	if _, ok := tg.Prims["go.len"]; !ok {
		t.Error("go.len must still be reachable")
	}
}

// --- THE WRITE SIDE, ADR 0018 -----------------------------------------------

// `build` reaches every target that has arrays, and each one spells the
// allocation and the store its own way. No target declares any of it.
func TestBuildAndSetOnEveryTarget(t *testing.T) {
	src := func(u, ge, add string) string {
		return `(use ` + u + `)
			(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 100))))
			(def f (fn (n)
				(let (build n (fn (c)
					(loop ((c c) (i 0)) (` + ge + ` i n) c
						else (again (set c i true) (` + add + ` i 1)))))
					(fn (b) (if (b 0) 1 0)))))`
	}
	cases := []struct{ target, src, want string }{
		{"go", src("go", "go.>=", "go.+"), "c2[i] = true"},
		{"js", src("js", "js.>=", "js.+"), "c2[i] = true;"},
		// The array is FILLED rather than left sparse: a sparse array on V8 is
		// a dictionary, so every store into one is a map insert.
		{"js", src("js", "js.>=", "js.+"), ".fill(0)"},
		{"java", src("java", "java.>=", "java.+"), "new boolean[(int) n]"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s: wanted %q in:\n%s", c.target, c.want, code)
		}
	}
}

// LINEARITY, which is what lets `build` freeze its buffer without copying.
//
// `(set b i v)` CONSUMES b and returns it, so after a store the old name is
// dead. This is checked by walking the residual in EVALUATION ORDER — a `let`'s
// value before its body — because it is an ordering property, not a counting
// one: reads before the move are fine, anything after it is not.
func TestUsingABufferAfterItIsConsumed(t *testing.T) {
	err := linearOn(t, `
		(use go)
		(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 100))))
		(def f (fn (n)
			(build n (fn (c)
				(let (set c 0 1) (fn (c2)
					(seq (set c 1 (c 0)) c2)))))))
	`)
	if err == nil {
		t.Fatal("using a buffer after a store must be refused")
	}
	if !strings.Contains(err.Error(), "already been handed on") {
		t.Errorf("the message must say the old name is dead, got: %v", err)
	}
}

// READS DO NOT CONSUME, and the sieve is why that matters: it tests a cell and
// then keeps going with the same buffer. A checker that counted occurrences
// rather than ordering them would refuse the one program ADR 0018 exists for.
func TestReadingABufferIsFine(t *testing.T) {
	if err := linearOn(t, `
		(use go)
		(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 100))))
		(def f (fn (n)
			(let (build n (fn (c)
				(loop ((c c) (i 0))
					(go.>= i n)  c
					(c i)        (again c (go.+ i 1))
					else         (again (set c i true) (go.+ i 1)))))
				(fn (b) (if (b 0) 1 0)))))
	`); err != nil {
		t.Errorf("a read must not consume the buffer: %v", err)
	}
}

// A buffer threaded through a nested loop that REUSES ITS NAME must not be
// confused with the outer one. The first version walked `Body()`, which opens a
// lambda using its parameter-name hints — so the sieve's inner `(fn (c i) …)`
// turned its own occurrences into free `c`s and the check refused a correct
// program. `Closed()` leaves inner binders as indices.
func TestAShadowingLoopVariableIsNotTheOuterBuffer(t *testing.T) {
	if err := linearOn(t, `
		(use go)
		(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 100))))
		(def inner (fn (c n)
			(loop ((c c) (j 0)) (go.>= j n) c else (again (set c j true) (go.+ j 1)))))
		(def f (fn (n)
			(let (build n (fn (c)
				(loop ((c c) (i 0))
					(go.>= i n)  c
					else         (again (inner c n) (go.+ i 1)))))
				(fn (b) (if (b 0) 1 0)))))
	`); err != nil {
		t.Errorf("a shadowing loop variable is its own buffer: %v", err)
	}
}

// `alloc` is the GATHER — a rule put in memory, pure and parallel by
// construction, as against `build`'s sequential scatter.
func TestAllocEmitsAFillLoop(t *testing.T) {
	code, err := genOn(t, "go", `
		(use go)
		(export f) (sig f ((n int)) (array int) (where (and (<= 0 n) (< n 1000))))
		(def f (fn (n) (alloc (table n (fn (i) (go.* i i))))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"make([]int,", "for i := 0;", "= (i * i)", ") []int {"} {
		if !strings.Contains(code, want) {
			t.Errorf("wanted %q in:\n%s", want, code)
		}
	}
}

// WINDOWS HAS TABLES NOW, which is what makes "a construct promoted to the
// language works on every target" true again rather than aspirational.
//
// A table here is ONE REGISTER: a pointer whose first eight bytes hold the
// length, elements from offset 8. A fat pointer would need two registers and
// would stop a table being a value, since this convention passes one value per
// register. The header costs nothing to skip — `[rbx+rcx*8+8]` is the same
// instruction as `[rbx+rcx*8]`, because the displacement is part of x86's
// addressing mode.
func TestWindowsHasTables(t *testing.T) {
	code, err := genOn(t, "windows", `
		(use x64)
		(export f) (sig f ((n int)) int (where (and (< 0 n) (< n 1000))))
		(def f (fn (n)
			(let (alloc (table n (fn (i) (x64.imul i i)))) (fn (v)
				(loop ((acc 0) (i 0))
					(x64.setge i (len v))  acc
					else                   (again (x64.add acc (v i)) (x64.add i 1)))))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"call VirtualAlloc",     // the target's own allocator, found not declared anew
		"mov qword ptr [r12], ", // the length header
		"*8+8]",                 // elements past it, at no cost
	} {
		if !strings.Contains(code, want) {
			t.Errorf("wanted %q in:\n%s", want, code)
		}
	}
}

// The allocator is the TARGET's, and a target that declares none is told so —
// which is ADR 0002's division made concrete: `alloc` and `build` are the
// language's, and where the bytes come from is the host's.
func TestATargetWithNoAllocatorIsTold(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tg.findAlloc(); !ok {
		t.Error("windows declares VirtualAlloc and findAlloc must see it")
	}
	bare := &Target{Prims: map[string]Prim{}}
	if _, ok := bare.findAlloc(); ok {
		t.Error("a target with no allocator must not appear to have one")
	}
}

// ELEMENT SIZE IS PART OF THE TYPE on the one host with no types of its own.
//
// wintables-2026-08-25 measured eight bytes per element against one at **3x**
// on a boolean sieve. Go never showed it because Go has a `bool` and `[]bool`
// is one byte — three hosts were sizing our elements for us through their own
// type systems, and x86 is where the choice became ours.
func TestWindowsSizesABooleanTableByTheByte(t *testing.T) {
	code, err := genOn(t, "windows", `
		(use x64)
		(export f) (sig f ((n int)) int (where (and (< 0 n) (< n 1000))))
		(def f (fn (n)
			(let (build n (fn (c)
				(loop ((c c) (i 0))
					(x64.setge i n)  c
					else             (again (set c i true) (x64.add i 1)))))
				(fn (b) (if (b 0) 1 0)))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// A byte store, a byte-wide read, and a scale of ONE rather than eight.
	if !strings.Contains(code, "mov byte ptr [") {
		t.Errorf("a boolean table stores one byte:\n%s", code)
	}
	if !strings.Contains(code, "movzx") {
		t.Errorf("and reads it zero-extended, so a bool comes back as 0 or 1:\n%s", code)
	}
	if strings.Contains(code, "*8+8]") {
		t.Errorf("a byte table must not be indexed at a scale of eight:\n%s", code)
	}
}

// An INT table keeps the wide form, because that is what an int needs.
func TestWindowsKeepsEightBytesForInts(t *testing.T) {
	code, err := genOn(t, "windows", `
		(use x64)
		(export f) (sig f ((n int)) int (where (and (< 0 n) (< n 1000))))
		(def f (fn (n)
			(let (alloc (table n (fn (i) (x64.imul i i)))) (fn (v) (v 0)))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "*8+8]") {
		t.Errorf("an int table is eight bytes an element:\n%s", code)
	}
}

// The width must survive a BINDER. The sieve's buffer is created by `build`,
// threaded through two loops and re-bound by a `let`, and losing the width at
// any one of those reads a byte array as qwords — seven bytes of the following
// elements per access, which is a wrong answer rather than a slow one.
func TestTheElementWidthSurvivesABinder(t *testing.T) {
	code, err := genOn(t, "windows", `
		(use x64)
		(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 1000))))
		(def f (fn (n)
			(let (build n (fn (c)
				(loop ((c c) (i 0))
					(x64.setge i n)  c
					else             (again (set c i true) (x64.add i 1)))))
				(fn (b)
					(loop ((acc 0) (k 0))
						(x64.setge k (len b))  acc
						(b k)                  (again (x64.add acc 1) (x64.add k 1))
						else                   (again acc (x64.add k 1)))))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// The COUNTING loop is behind a `let` whose value is a `build` term, not a
	// name — which is the case the first version missed.
	if strings.Contains(code, "*8+8]") {
		t.Errorf("the width was lost crossing a binder:\n%s", code)
	}
}
