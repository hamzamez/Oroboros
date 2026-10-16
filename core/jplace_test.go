package core

import (
	"strings"
	"testing"
)

// placeEnv is an environment for reducing the tests' definitions: the
// language's injected names, all pure, and the program's definitions.
func placeEnv(p *Program) *Env {
	e := &Env{Defs: p.Defs, Prim: map[string]bool{}, Pure: map[string]bool{}, Rec: map[string]bool{},
		Word: Word{Lo: -(1<<53 - 1), Hi: 1<<53 - 1}}
	for _, n := range []string{"if", "=", "+", "-", "*", "<", ">=", "loop", "set", "build", "len", "let", "alloc", "insert", "build-map"} {
		e.Prim[n] = true
		e.Pure[n] = true
	}
	e.MarkRecursive()
	return e
}

// loadJ loads a program written in J: the state translation S decides R0–R4.
func loadJ(t *testing.T, src string) (*Program, error) {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		t.Fatalf("J does not read: %v\n%s", err, src)
	}
	p, _, err := Load(forms)
	return p, err
}

// A BUFFER IN PROGRAM ORDER MEANS THE THREADED BUFFER (ADR 0052, spec/local.md
// §2.1): S of each program-order definition reduces to the normal form of the
// definition threaded by hand. The theorem of storeorder-research §2.3, on its
// cases: a loop's store, a borrow, a borrow with a result, a swap between
// places, a map's store, and a φ. The threaded definitions were today's
// notation, which J has no spelling of (ADR 0058); each case keeps the normal
// form they reduced to, the core term's text (ParseTerm), as captured at step
// 5b (jswitch-2026-10-16).
func TestABufferInProgramOrderMeansTheThreadedBuffer(t *testing.T) {
	for _, c := range []struct{ name, j, threaded string }{
		{"a loop's store",
			`fn f(n) {
				var c := array { false : _ ∈ 0..n-1 }
				loop j = 0 {
					j < n -> { c[j] := true; again(j + 1) }
					else  -> c
				}
			}`,
			`(fn (n) (build n (fn (c) (loop (fn (c1 j) (if (< j n) (again (set c1 j true) (+ j 1)) c1)) (the "buffer bool" c) 0))))`},
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
			`(fn (n) (build n (fn (b) (loop (fn (b1 k) (if (>= k n) b1 (again (set b1 k (* k 2)) (+ k 1)))) b 0))))`},
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
			`(fn (n) (build n (fn (b) (set (set b 0 1) 1 2))))`},
		{"a swap between places",
			`fn f(n) {
				var a := array { 0 : _ ∈ 0..3 }
				var b := array { 0 : _ ∈ 0..3 }
				loop w = 1 {
					w >= n -> a
					else   -> { a[0] := w; a, b := b, a; again(w * 2) }
				}
			}`,
			`(fn (n) (build 4 (fn (a) (build 4 (fn (b) (loop (fn (a1 b1 w) (if (>= w n) a1 (again b1 (set a1 0 w) (* w 2)))) a b 1))))))`},
		{"a map buffer's store (ADR 0058)",
			`fn f(n) {
				var m := map(8)
				loop i = 0 {
					i >= n -> m
					else   -> { m[i] := i * 10; again(i + 1) }
				}
			}`,
			`(fn (n) (build-map 8 (fn (m) (loop (fn (m1 i) (if (>= i n) m1 (again (insert m1 i (* i 10)) (+ i 1)))) m 0))))`},
		{"a write under a condition, a φ",
			`fn f(n) {
				var b := array { 0 : _ ∈ 0..3 }
				if n < 2 { b[0] := 1 }
				b
			}`,
			`(fn (n) (build 4 (fn (b) (if (< n 2) (set b 0 1) b))))`},
	} {
		t.Run(c.name, func(t *testing.T) {
			pj, err := loadJ(t, c.j)
			if err != nil {
				t.Fatal(err)
			}
			fj, err := Normalize(pj.Defs["f"], placeEnv(pj), DefaultFuel)
			if err != nil {
				t.Fatal(err)
			}
			ft, err := ParseTerm(c.threaded)
			if err != nil {
				t.Fatal(err)
			}
			if !fj.Equal(ft) {
				t.Errorf("program order and threading differ:\n  J        %s\n  threaded %s", fj, ft)
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
