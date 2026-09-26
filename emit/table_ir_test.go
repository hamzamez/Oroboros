package emit_test

import (
	"regexp"
	"strings"
	"testing"
)

// The construct reaches every backend that has arrays, and each one spells it
// its own way.
func TestIndexingIsApplicationOnEveryTarget(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"go", `(use go)
			(export f) (sig f ((a (array f64)) (i int)) f64
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, `v\d+\[v\d+\]`},
		{"js", `(use js)
			(export f) (sig f ((a (array any)) (i any)) any
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, `return v\d+\[v\d+\];`},
		// The (int) CAST is Java's and it is not optional: our `int` maps to
		// `long` and a Java array index must be an `int`. Without it javac
		// refuses the file with "possible lossy conversion".
		{"java", `(use java)
			(export f) (sig f ((a (array f64)) (i int)) f64
			  (where (and (<= 0 i) (< i (len a)))))
			(def f (fn (a i) (a i)))`, `v\d+\[\(int\) v\d+\]`},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !regexp.MustCompile(c.want).MatchString(code) {
			t.Errorf("%s: wanted %s in:\n%s", c.target, c.want, code)
		}
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
	if !regexp.MustCompile(`v\d+\[v\d+\]`).MatchString(code) {
		t.Errorf("expected an indexing:\n%s", code)
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
	if err == nil || !strings.Contains(err.Error(), "never allocated") {
		t.Errorf("a surviving rule-table must be refused, got %v", err)
	}
}

// `build` reaches every target that has arrays, and each one spells the
// allocation and the store its own way. No target declares any of it.
func TestBuildAndSetOnEveryTarget(t *testing.T) {
	src := func(u, ge, add string) string {
		return `(use ` + u + `)
			(export f) (sig f ((n int)) int (where (and (< 2 n) (< n 100))))
			(def f (fn (n)
				(let b (build n (fn (c)
					(loop ((c c) (i 0)) (` + ge + ` i n) c
						else (again (set c i true) (` + add + ` i 1)))))
					(if (b 0) 1 0))))`
	}
	cases := []struct{ target, src, want string }{
		{"go", src("go", "go.>=", "go.+"), `v\d+\[v\d+\] = true`},
		{"js", src("js", "js.>=", "js.+"), `v\d+\[v\d+\] = true;`},
		// The array is FILLED rather than left sparse: a sparse array on V8 is
		// a dictionary, so every store into one is a map insert.
		{"js", src("js", "js.>=", "js.+"), `\.fill\(0\)`},
		{"java", src("java", "java.>=", "java.+"), `new boolean\[\(int\) v\d+\]`},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if !regexp.MustCompile(c.want).MatchString(code) {
			t.Errorf("%s: wanted %s in:\n%s", c.target, c.want, code)
		}
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
	for _, want := range []string{`make\(\[\]int,`, `for v\d+ := 0;`, `\((v\d+) \* v\d+\)`, `\) \[\]int \{`} {
		if !regexp.MustCompile(want).MatchString(code) {
			t.Errorf("wanted %s in:\n%s", want, code)
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
			(let v (alloc (table n (fn (i) (x64.imul i i))))
     (loop ((acc 0) (i 0))
 					(x64.setge i (len v))  acc
 					else                   (again (x64.add acc (v i)) (x64.add i 1))))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"call VirtualAlloc", // the target's own allocator, found not declared anew
		"mov qword ptr [",   // the length header, at whichever register holds the table
		"*8+8]",             // elements past it, at no cost
	} {
		if !strings.Contains(code, want) {
			t.Errorf("wanted %q in:\n%s", want, code)
		}
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
			(let b (build n (fn (c)
  				(loop ((c c) (i 0))
  					(x64.setge i n)  c
  					else             (again (set c i true) (x64.add i 1)))))
     (if (b 0) 1 0))))
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
			(let v (alloc (table n (fn (i) (x64.imul i i))))
     (v 0))))
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
			(let b (build n (fn (c)
  				(loop ((c c) (i 0))
  					(x64.setge i n)  c
  					else             (again (set c i true) (x64.add i 1)))))
     (loop ((acc 0) (k 0))
						(x64.setge k (len b))  acc
						(b k)                  (again (x64.add acc 1) (x64.add k 1))
						else                   (again acc (x64.add k 1))))))
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
