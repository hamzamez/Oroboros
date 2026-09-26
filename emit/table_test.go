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
	return CheckLinear(nf, tg, nil)
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
				(let c2 (set c 0 1)
      (seq (set c 1 (c 0)) c2))))))
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
			(let b (build n (fn (c)
  				(loop ((c c) (i 0))
  					(go.>= i n)  c
  					(c i)        (again c (go.+ i 1))
  					else         (again (set c i true) (go.+ i 1)))))
     (if (b 0) 1 0))))
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
			(let b (build n (fn (c)
  				(loop ((c c) (i 0))
  					(go.>= i n)  c
  					else         (again (inner c n) (go.+ i 1)))))
     (if (b 0) 1 0))))
	`); err != nil {
		t.Errorf("a shadowing loop variable is its own buffer: %v", err)
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
