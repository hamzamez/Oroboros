package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// genChecked is surfaceGen with the build's -checked.
func genChecked(t *testing.T, target, src string) (string, error) {
	t.Helper()
	dir, err := os.MkdirTemp(".", "surfacetest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "p.oro"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "p.out")
	if err := run("../../targets", filepath.Join(dir, "p.oro"), target, out, "", "../../lib", true, ""); err != nil {
		return "", err
	}
	b, err := os.ReadFile(out)
	return string(b), err
}

// AN INTERMEDIATE PAST THE WORDS AND E IS THE INTEGER'S OWN (ADR 0150,
// sets-are-claims-research §3, ir/exact.go). Each rule against its planted
// fault, which must fail first:
//
//  1. D_A: a product of three words, past E, is held exactly on every target,
//     the program asking nothing, and reported; PlantExactDemandOff and
//     `-words` refuse it, as before A;
//  2. a remainder is exact only where its divisor is, so a·b·c % 7 returned as
//     a word is the rung's (an exact dividend's remainder by a word is a
//     word); PlantTaintThroughRem taints the dividend and refuses it;
//  3. a value that flows into a sink stays a word: a product stored into a
//     table is refused for its size, and under -checked trapped, where it
//     leaves the word; PlantTaintIgnored holds it exactly and the store
//     refuses it instead.
func TestAnIntermediatePastEIsTheIntegersOwn(t *testing.T) {
	const s = "-2^63..2^63-1"
	volume := "export fn volume(a ∈ " + s + ", b ∈ " + s + ", c ∈ " + s + ") (-6..6) { a * b * c % 7 }\n"
	for _, tg := range []string{"go", "java", "js", "windows", "windows32"} {
		if _, err := surfaceGen(t, tg, map[string]string{"p.oro": volume}); err != nil {
			t.Errorf("a·b·c %% 7 on %s: want the product held exactly, got %v", tg, err)
		}
	}
	if code, err := onGo(t, volume); err != nil || !strings.Contains(code, "big.Int") {
		t.Errorf("on Go, want the product a *big.Int: %v\n%s", err, code)
	}
	for name, plant := range map[string]*bool{
		"PlantExactDemandOff":  &ir.PlantExactDemandOff,
		"WordsOnly":            &ir.WordsOnly,
		"PlantTaintThroughRem": &ir.PlantTaintThroughRem,
	} {
		*plant = true
		_, err := onGo(t, volume)
		*plant = false
		if err == nil || !strings.Contains(err.Error(), "cannot be proven") {
			t.Errorf("with %s, a·b·c should be refused for its size, so the witness can fail: %v", name, err)
		}
	}

	store := "export fn store(a ∈ " + s + ", b ∈ " + s + ", c ∈ " + s + ") (Int) {\n" +
		"    var t := array { 0 }\n    t[0] := a * b * c\n    t[0]\n}\n"
	_, err := onGo(t, store)
	if err == nil || !strings.Contains(err.Error(), "cannot be proven") {
		t.Errorf("a product stored into a table: want it refused for its size, got %v", err)
	}
	if _, err := genChecked(t, "go", store); err != nil {
		t.Errorf("a product stored into a table, -checked: want it trapped where it leaves the word, got %v", err)
	}
	ir.PlantTaintIgnored = true
	_, err = genChecked(t, "go", store)
	ir.PlantTaintIgnored = false
	if err == nil || !strings.Contains(err.Error(), "held in arbitrary precision") {
		t.Errorf("with the sink ignored, the product should be held exactly and refused at the store: %v", err)
	}
}
