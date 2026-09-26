package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir/golang"
	"oroboros/ir/java"
)

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
		// The literal is read inside the function, so its representation is
		// its own class's: a RETURNED table would take the export's declared
		// boundary type, which is the word (Theorem D′), whatever it holds.
		body := core.Fn([]string{"i"}, core.App(core.Name("let"), lit(c.elems...),
			core.Fn([]string{"t"}, core.App(core.Name("t"), core.Name("i")))))
		code, err := golang.FromResidual(litTarget(t, "go"), "t", litSig, body)
		if err != nil {
			t.Fatalf("%v: %v", c.elems, err)
		}
		if !strings.Contains(code, c.wantGo) {
			t.Errorf("%v on Go: want %q, got:\n%s", c.elems, c.wantGo, code)
		}
		jcode, err := java.FromResidual(litTarget(t, "java"), "t", litSig, body)
		if err != nil {
			t.Fatalf("%v (java): %v", c.elems, err)
		}
		if !strings.Contains(jcode, c.wantJava) {
			t.Errorf("%v on the JVM: want %q, got:\n%s", c.elems, c.wantJava, jcode)
		}
	}
}

// CHECKING: the declaration decides. The Go and JVM answers DIFFER for the same
// literal — `(int 0 255)` is `[]byte` on one and `short[]` on the other, whose
// `byte` is signed — which is exactly why synthesis alone cannot serve a
// boundary, and why this is bidirectional rather than one rule.
func TestADeclaredElementDecidesAtABoundary(t *testing.T) {
	for what, term := range takes(lit(104, 105, 33)) {
		code, err := golang.FromResidual(litTarget(t, "go"), "t", nil, term)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if !strings.Contains(code, "[]byte{") {
			t.Errorf("%s on Go: the declaration must decide, got:\n%s", what, code)
		}
		jcode, err := java.FromResidual(litTarget(t, "java"), "t", nil, term)
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
		_, err := golang.FromResidual(litTarget(t, "go"), "t", nil, term)
		if err == nil {
			t.Fatalf("%s: a literal outside the declared element must be refused", what)
		}
		for _, want := range []string{"300", "int 0 255", "literal-elements.md"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal must name %q, got: %v", what, want, err)
			}
		}
		if _, err := java.FromResidual(litTarget(t, "java"), "t", nil, term); err == nil {
			t.Errorf("%s: the JVM emitter must refuse it too", what)
		}
	}
}

// litSig types the index a literal is read at: an index below 3.
var litSig = &core.Sig{Params: []core.SigParam{{Name: "i", Type: "int 0 2"}}, Result: "int"}
