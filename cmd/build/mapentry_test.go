package main

import (
	"testing"

	"oroboros/ir/java"
)

// A MAP LITERAL'S ENTRIES ARE TAKEN INTO ITS CARRIER (keptnum-2026-11-22). A
// literal map is a finite function K → V, held on the JVM as Map<Box ρ(K), Box
// ρ(V)>; each entry is cast into ρ(K) and ρ(V) before it is boxed, since Java
// boxes an int only to Integer. Read at a key the decision does not know, the
// map is built, and `Map.entry(1, 10)` was a Map.Entry<Integer, Integer>, which
// javac refused as a member of a Map<Long, Long> (map-static, opaque).
// PlantMapEntryUncast writes the entries as their own types again.
func TestAMapLiteralsEntriesAreItsCarriers(t *testing.T) {
	const src = "use io\n\n" +
		"fn run(k ∈ 0..5) (Int) { case `map`(1(10), 2(20), 5(50))(k) { 'some(v) -> v; 'none -> 0 } }\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i <= 5 -> { io.print_int(run(i)); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const want = "0 10 20 0 0 50"
	if got, err := runOn(t, src, "java"); err != nil || got != want {
		t.Errorf("a map literal read at an unknown key on the JVM: want %s, got %q, %v", want, got, err)
	}
	java.PlantMapEntryUncast = true
	got, err := runOn(t, src, "java")
	java.PlantMapEntryUncast = false
	if err == nil && got == want {
		t.Error("with its entries uncast, the map literal still compiles: the witness cannot fail")
	}
}
