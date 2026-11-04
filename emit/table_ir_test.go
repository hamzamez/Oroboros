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
		{"go", `use go
export fn f(a ∈ Array(F64), i ∈ Int) (F64) where 0 <= i && i < len(a) { a[i] }`, `v\d+\[v\d+\]`},
		{"js", `use js
export fn f(a ∈ Array(Any), i ∈ Any) (Any) where 0 <= i && i < len(a) { a[i] }`, `return v\d+\[v\d+\];`},
		// The (int) CAST is Java's and it is not optional: our `int` maps to
		// `long` and a Java array index must be an `int`. Without it javac
		// refuses the file with "possible lossy conversion".
		{"java", `use java
export fn f(a ∈ Array(F64), i ∈ Int) (F64) where 0 <= i && i < len(a) { a[i] }`, `v\d+\[\(int\) v\d+\]`},
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
	src := "use go\nexport fn f(a ∈ Array(F64)) (F64) {\n    loop acc = 0.0, i = 0 {\n        go.`>=`(i, len(a)) -> acc\n        else -> again(go.`f+`(acc, a[i]), go.`+`(i, 1))\n    }\n}"
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
	_, err := genOn(t, "go", `use go
export fn f(n ∈ Int) (Any) { array { i : i ∈ 0..n-1 } }`, "f")
	if err == nil || !strings.Contains(err.Error(), "never allocated") {
		t.Errorf("a surviving rule-table must be refused, got %v", err)
	}
}

// `build` reaches every target that has arrays, and each one spells the
// allocation and the store its own way. No target declares any of it.
func TestBuildAndSetOnEveryTarget(t *testing.T) {
	// The host's own comparison and addition, h.`>=` and h.`+`.
	src := func(h string) string {
		return "use " + h + "\nexport fn f(n ∈ Int) (Int) where 2 < n && n < 100 {\n" +
			"    b = build(n, fn(c) { loop c = c, i = 0 { " + h + ".`>=`(i, n) -> c; else -> again(store(c, i, true), " + h + ".`+`(i, 1)) } })\n" +
			"    if b[0] { 1 } else { 0 }\n}"
	}
	cases := []struct{ target, src, want string }{
		{"go", src("go"), `v\d+\[v\d+\] = true`},
		{"js", src("js"), `v\d+\[v\d+\] = true;`},
		// The array is FILLED rather than left sparse: a sparse array on V8 is
		// a dictionary, so every store into one is a map insert.
		{"js", src("js"), `\.fill\(0\)`},
		{"java", src("java"), `new boolean\[\(int\) v\d+\]`},
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
	code, err := genOn(t, "go", "use go\nexport fn f(n ∈ Int) (Array(Int)) where 0 <= n && n < 1000 {\n    alloc(array { go.`*`(i, i) : i ∈ 0..n-1 })\n}", "f")
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
// A table here is ONE POINTER whose first eight bytes hold the length, elements
// from offset 8 (docs/spec/llvm.md §4): one value, so a table is a value. The
// header is a constant offset, folded into the address.
func TestWindowsHasTables(t *testing.T) {
	code, err := genOn(t, "windows", `export fn f(n ∈ Int) (Int) where 0 < n && n < 1000 {
    v = alloc(array { i * i : i ∈ 0..n-1 })
    loop acc = 0, i = 0 {
        i >= len(v) -> acc
        else -> again(acc + v[i], i + 1)
    }
}`, "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"call ptr @VirtualAlloc(",           // the allocator's term (ADR 0092), inlined
		"call ptr @HeapAlloc(",              // both of kernel32's, chosen by size
		"mul nsw i64 %v0, 8",                // n·w, proven inside the word by the term's guard
		"getelementptr inbounds i8, ptr %",  // the elements, past the length header
		"getelementptr inbounds i64, ptr %", // each eight bytes
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
// type systems, and a native target is where the choice is ours.
func TestWindowsSizesABooleanTableByTheByte(t *testing.T) {
	code, err := genOn(t, "windows", `export fn f(n ∈ Int) (Int) where 0 < n && n < 1000 {
    b = build(n,
        fn(c) {
            loop c = c, i = 0 {
                i >= n -> c
                else -> again(store(c, i, true), i + 1)
            }
        })
    if b(0) { 1 } else { 0 }
}`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// A byte store, a byte read, and an element of ONE byte rather than eight.
	if !strings.Contains(code, "store i8 ") {
		t.Errorf("a boolean table stores one byte:\n%s", code)
	}
	if !strings.Contains(code, "load i8, ptr ") {
		t.Errorf("and reads one byte back:\n%s", code)
	}
	if strings.Contains(code, "getelementptr inbounds i64") {
		t.Errorf("a byte table must not be indexed by eight-byte elements:\n%s", code)
	}
}

// An INT table keeps the wide form, because that is what an int needs.
func TestWindowsKeepsEightBytesForInts(t *testing.T) {
	code, err := genOn(t, "windows", `export fn f(n ∈ Int) (Int) where 0 < n && n < 1000 {
    v = alloc(array { i * i : i ∈ 0..n-1 })
    if 0 < len(v) { v(0) } else { 0 }
}`, "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "getelementptr inbounds i64") {
		t.Errorf("an int table is eight bytes an element:\n%s", code)
	}
}

// The width must survive a BINDER. The sieve's buffer is created by `build`,
// threaded through two loops and re-bound by a `let`, and losing the width at
// any one of those reads a byte array as qwords — seven bytes of the following
// elements per access, which is a wrong answer rather than a slow one.
func TestTheElementWidthSurvivesABinder(t *testing.T) {
	code, err := genOn(t, "windows", `export fn f(n ∈ Int) (Int) where 2 < n && n < 1000 {
    b = build(n,
        fn(c) {
            loop c = c, i = 0 {
                i >= n -> c
                else -> again(store(c, i, true), i + 1)
            }
        })
    loop acc = 0, k = 0 {
        k >= len(b) -> acc
        b[k] -> again(acc + 1, k + 1)
        else -> again(acc, k + 1)
    }
}`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// The COUNTING loop is behind a `let` whose value is a `build` term, not a
	// name — which is the case the first version missed.
	if strings.Contains(code, "getelementptr inbounds i64") {
		t.Errorf("the width was lost crossing a binder:\n%s", code)
	}
}
