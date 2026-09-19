package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A LITERAL TABLE'S ELEMENTS DECIDE ITS ELEMENT TYPE (docs/literal-elements.md).
//
// Until this, `(array 104 105 33)` emitted `[]int` — the type of its FIRST
// element, which is the shape of the `BufferElemBytes` bug elemwidth-2026-08-27
// fixed for buffers, surviving in the safe direction. The consequence was not
// safe: a literal handed to a host call declaring `(array (int 0 255))` was
// refused BY GO — *"cannot use src (variable of type []int) as []byte value"* —
// naming a type nobody wrote.
//
// Two rules, tested separately because they answer different questions.
// SYNTHESIS gives a local literal the hull of its elements; CHECKING lets a
// declaration at a boundary decide, and refuses a literal that does not fit.

// litTarget declares one host call over a byte table, as `hex.Encode` does —
// on the host asked for, because the two hosts spell that byte table
// differently and that difference is the point.
func litTarget(t *testing.T, host string) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/" + host)
	if err != nil {
		t.Fatal(err)
	}
	form := "len(%s)"
	if host == "java" {
		form = "%s.length"
	}
	tg.Prims["x.take"] = Prim{Name: "x.take", Args: []string{"array int 0 255"},
		Result: "int", Kind: "expr", Form: form, Pure: true}
	return tg
}

// lit builds `(array e…)`.
func lit(vs ...int64) *core.Term {
	kids := []*core.Term{core.Name("array")}
	for _, v := range vs {
		kids = append(kids, core.Int(v))
	}
	return &core.Term{Kind: core.KApp, Kids: kids}
}

// SYNTHESIS: the hull of the elements, on each host that has types.
func TestALiteralTableTakesTheHullOfItsElements(t *testing.T) {
	for _, c := range []struct {
		elems            []int64
		wantGo, wantJava string
	}{
		// 33..105 is a byte on Go, and fits the JVM's SIGNED byte
		{[]int64{104, 105, 33}, "[]byte{", "new byte[]{"},
		// one element past a byte, so the whole table stays wide
		{[]int64{104, 300, 33}, "[]uint16{", "new short[]{"},
		// negative, which no unsigned representation holds
		{[]int64{1, -2, 3}, "[]int8{", "new byte[]{"},
	} {
		body := core.Fn(nil, lit(c.elems...))
		code, err := Func(litTarget(t, "go"), "t", nil, body)
		if err != nil {
			t.Fatalf("%v: %v", c.elems, err)
		}
		if !strings.Contains(code, c.wantGo) {
			t.Errorf("%v on Go: want %q, got:\n%s", c.elems, c.wantGo, code)
		}
		jcode, err := JavaMethod(litTarget(t, "java"), "t", nil, body)
		if err != nil {
			t.Fatalf("%v (java): %v", c.elems, err)
		}
		if !strings.Contains(jcode, c.wantJava) {
			t.Errorf("%v on the JVM: want %q, got:\n%s", c.elems, c.wantJava, jcode)
		}
	}
}

// takes builds the two shapes a literal reaches a declared parameter through:
// straight into the call, and bound by a `let` the body then hands over.
func takes(l *core.Term) map[string]*core.Term {
	return map[string]*core.Term{
		"argument": core.Fn(nil, core.App(core.Name("x.take"), l)),
		"let-bound": core.Fn(nil, core.App(core.Name("let"), l,
			core.Fn([]string{"d"}, core.App(core.Name("x.take"), core.Name("d"))))),
	}
}

// CHECKING: the declaration decides. The Go and JVM answers DIFFER for the same
// literal — `(int 0 255)` is `[]byte` on one and `short[]` on the other, whose
// `byte` is signed — which is exactly why synthesis alone cannot serve a
// boundary, and why this is bidirectional rather than one rule.
func TestADeclaredElementDecidesAtABoundary(t *testing.T) {
	for what, term := range takes(lit(104, 105, 33)) {
		code, err := Func(litTarget(t, "go"), "t", nil, term)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if !strings.Contains(code, "[]byte{") {
			t.Errorf("%s on Go: the declaration must decide, got:\n%s", what, code)
		}
		jcode, err := JavaMethod(litTarget(t, "java"), "t", nil, term)
		if err != nil {
			t.Fatalf("%s (java): %v", what, err)
		}
		if !strings.Contains(jcode, "new short[]{") {
			t.Errorf("%s on the JVM: want short[], got:\n%s", what, jcode)
		}
	}
}

// AND A LITERAL THAT DOES NOT FIT IS OURS TO REFUSE. Against HEAD before this
// both of these compiled, and the HOST refused them.
func TestALiteralOutsideTheDeclaredElementIsRefused(t *testing.T) {
	for what, term := range takes(lit(104, 300, 33)) {
		_, err := Func(litTarget(t, "go"), "t", nil, term)
		if err == nil {
			t.Fatalf("%s: a literal outside the declared element must be refused", what)
		}
		for _, want := range []string{"300", "int 0 255", "literal-elements.md"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal must name %q, got: %v", what, want, err)
			}
		}
		if _, err := JavaMethod(litTarget(t, "java"), "t", nil, term); err == nil {
			t.Errorf("%s: the JVM emitter must refuse it too", what)
		}
	}
}

// THE HULL IS A JOIN over every element — a control against reading only the
// first, which is the bug this replaces, and against reading only the last.
func TestTheHullIsTheJoinOfEveryElement(t *testing.T) {
	typeOf := func(*core.Term) string { return "int" }
	for _, c := range []struct {
		elems []int64
		want  string
	}{
		{[]int64{104, 105, 33}, "int 33 105"},
		{[]int64{5}, "int 5 5"},
		{[]int64{-3, 9}, "int -3 9"},
		{[]int64{9, -3}, "int -3 9"},
		{[]int64{0, 255}, "int 0 255"},
	} {
		if got := LiteralElem(lit(c.elems...), typeOf); got != c.want {
			t.Errorf("%v: want %q, got %q", c.elems, c.want, got)
		}
	}
}
