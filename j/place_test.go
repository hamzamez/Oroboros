package j

import (
	"strings"
	"testing"

	"oroboros/core"
)

// placeEnv is an environment for reducing the tests' definitions: the
// language's injected names, all pure, and the program's definitions.
func placeEnv(p *core.Program) *core.Env {
	e := &core.Env{Defs: p.Defs, Prim: map[string]bool{}, Pure: map[string]bool{}, Rec: map[string]bool{},
		Word: core.Word{Lo: -(1<<53 - 1), Hi: 1<<53 - 1}}
	for _, n := range []string{"if", "=", "+", "-", "*", "<", ">=", "loop", "set", "build", "len", "let", "alloc", "insert", "build-map"} {
		e.Prim[n] = true
		e.Pure[n] = true
	}
	e.MarkRecursive()
	return e
}

// loadJ loads a program written in J, and loadToday one in today's notation.
func loadJ(t *testing.T, src string) (*core.Program, error) {
	t.Helper()
	decls, err := core.ReadProgram(src, "", ".", "..")
	if err != nil {
		t.Fatalf("J does not read: %v\n%s", err, src)
	}
	forms, err := core.FormsOf(decls)
	if err != nil {
		return nil, err
	}
	p, _, err := core.Load(forms)
	return p, err
}

func loadToday(t *testing.T, src string) *core.Program {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A BUFFER IN PROGRAM ORDER MEANS THE THREADED BUFFER (ADR 0052, spec/local.md
// §2.1): S of each program-order definition reduces to the normal form of the
// definition today's notation writes by hand. The theorem of storeorder-research
// §2.3, on its cases: a loop's store, a borrow, a borrow with a result, a swap
// between places, and a φ. (A loop that only reads a buffer written before it
// reduces to a let of the store where the threaded form substitutes it into the
// loop: β decides the binding before the scope's selection reaches the loop.
// storeorder-2026-10-15 §3 records it.)
func TestABufferInProgramOrderMeansTheThreadedBuffer(t *testing.T) {
	for _, c := range []struct{ name, j, today string }{
		{"a loop's store",
			`fn f(n) {
				var c := array { false : _ ∈ 0..n-1 }
				loop j = 0 {
					j < n -> { c[j] := true; again(j + 1) }
					else  -> c
				}
			}`,
			`(def f (n) (local c (table n false) (loop ((c c) (j 0)) (< j n) (again (set c j true) (+ j 1)) else c)))`},
		{"a borrow",
			`fn fill(b linear, n) {
				loop k = 0 {
					k >= n -> ()
					else   -> { b[k] := k * 2; again(k + 1) }
				}
			}
			fn f(n) {
				var b := array { 0 : _ ∈ 0..n-1 }
				fill(b, n)
				b
			}`,
			`(def fill (b n) (loop ((b b) (k 0)) (>= k n) b else (again (set b k (* k 2)) (+ k 1))))
			 (def f (n) (local b (table n 0) (fill b n)))`},
		{"a borrow with a result",
			`fn put(b linear, k) {
				b[k] := 1
				k + 1
			}
			fn f(n) {
				var b := array { 0 : _ ∈ 0..n-1 }
				m = put(b, 0)
				b[m] := 2
				b
			}`,
			`(def put (b k) (tuple (set b k 1) (+ k 1)))
			 (def f (n) (local b (table n 0) (let (tuple b m) (put b 0) (set b m 2))))`},
		{"a swap between places",
			`fn f(n) {
				var a := array { 0 : _ ∈ 0..3 }
				var b := array { 0 : _ ∈ 0..3 }
				loop w = 1 {
					w >= n -> a
					else   -> { a[0] := w; a, b := b, a; again(w * 2) }
				}
			}`,
			`(def f (n) (local a (table 4 0) b (table 4 0)
			   (loop ((a a) (b b) (w 1)) (>= w n) a else (again b (set a 0 w) (* w 2)))))`},
		{"a map buffer's store (ADR 0058)",
			`fn f(n) {
				var m := map(8)
				loop i = 0 {
					i >= n -> m
					else   -> { m[i] := i * 10; again(i + 1) }
				}
			}`,
			`(def f (n) (build-map m 8 (loop ((m m) (i 0)) (>= i n) m else (again (insert m i (* i 10)) (+ i 1)))))`},
		{"a write under a condition, a φ",
			`fn f(n) {
				var b := array { 0 : _ ∈ 0..3 }
				if n < 2 { b[0] := 1 }
				b
			}`,
			`(def f (n) (local b (table 4 0) (if (< n 2) (set b 0 1) b)))`},
	} {
		t.Run(c.name, func(t *testing.T) {
			pj, err := loadJ(t, c.j)
			if err != nil {
				t.Fatal(err)
			}
			pt := loadToday(t, c.today)
			fj, err := core.Normalize(pj.Defs["f"], placeEnv(pj), core.DefaultFuel)
			if err != nil {
				t.Fatal(err)
			}
			ft, err := core.Normalize(pt.Defs["f"], placeEnv(pt), core.DefaultFuel)
			if err != nil {
				t.Fatal(err)
			}
			if !fj.Equal(ft) {
				t.Errorf("program order and threading differ:\n  J     %s\n  today %s", fj, ft)
			}
		})
	}
}

// EACH RULE REFUSES WHAT IT FORBIDS (ADR 0052, R0–R4; spec/local.md §2.1). A
// rule's planted fault, the check removed, makes its case accept
// (storeorder-2026-10-15 §4).
func TestEachRuleOfAPlaceRefusesWhatItForbids(t *testing.T) {
	for _, c := range []struct{ rule, j string }{
		{"R0", `fn f(xs) { var b := array { 0 : _ ∈ 0..3 }; b := xs; b }`},
		{"R0", `fn f(n) {
			var a := array { 0 : _ ∈ 0..3 }
			var b := array { 0 : _ ∈ 0..3 }
			var x := 0
			a, x := b, 1
			a
		}`},
		{"R1", `fn f(n) { var b := array { 0 : _ ∈ 0..3 }; d = b; d }`},
		{"R1", `fn f(n) {
			var b := array { 0 : _ ∈ 0..3 }
			var x := 0
			x := b
			x
		}`},
		{"R1", `fn g(t) { t }
			fn f(n) { var b := array { 0 : _ ∈ 0..3 }; m = (b, 1); b }`},
		{"R1", `fn f(n) { var m := map(8); d = m; d }`},
		{"R2", `fn g(p, q) { p[0] + q[0] }
			fn f(n) { var b := array { 0 : _ ∈ 0..3 }; s = g(b, b); b }`},
		{"R3", `fn f(n) { var b := array { 0 : _ ∈ 0..3 }; h = fn(i) { b[i] }; b }`},
		{"R4", `fn g(b linear) { b[0] := 1; b }`},
	} {
		t.Run(c.rule, func(t *testing.T) {
			_, err := loadJ(t, c.j)
			if err == nil {
				t.Fatalf("%s accepted:\n%s", c.rule, c.j)
			}
			if !strings.Contains(err.Error(), c.rule+":") {
				t.Errorf("%s refused, but not by %s: %v", c.j, c.rule, err)
			}
		})
	}
}
