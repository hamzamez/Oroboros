package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// AN ENTRY POINT'S PARAMETER IN U PAST THE WORD IS A PAIR (pairsel-2026-11-21's
// gap 1): on JavaScript the caller holds such a value as a BigInt, as a result
// in U is returned, and it enters the pair's digits once, by pair_bigint's
// inverse (⌊x/2³²⌋, x mod 2³²). Before, the pass split it as a word, and the
// realization rule refused `>>` of a value past 2⁵³. Four exports, called from
// node with BigInts and checked against BigInt's ℤ: `n + 1`, Bloch's
// `(lo + hi) / 2` over naturals, an FNV step mod 2⁶⁴, and `x / 3`, a parameter
// that also leaves the pair for ℤ (the rung's `big-of` read back on its
// digits). THE PLANTED FAULT splits the parameter as a word again, and the
// program is refused.
func TestAnEntryParameterInUIsAPair(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	const src = "export fn next(n ∈ 0..2^63-1) (0..2^63) { n + 1 }\n\n" +
		"export fn mid(lo ∈ 0..2^63-1, hi ∈ 0..2^63-1) (0..2^63-1) { (lo + hi) / 2 }\n\n" +
		"export fn step(h ∈ 0..2^64-1, b ∈ 0..255) (0..2^64-1) { ((h ⊕ b) * 1099511628211) mod 2^64 }\n\n" +
		"export fn third(x ∈ 0..2^63-1) (Int) { x / 3 }\n"
	code, err := surfaceGen(t, "js", map[string]string{"p.oro": src})
	if err != nil {
		t.Fatalf("an entry parameter in U on JavaScript: %v", err)
	}
	if !strings.Contains(code, ">> 32n") || !strings.Contains(code, "& 4294967295n") {
		t.Errorf("the parameters do not enter by pair_hi and pair_lo:\n%s", code)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.mjs"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	const driver = `import { genNext, genMid, genStep, genThird } from './p.mjs';
const T = 2n ** 63n - 1n, U = 2n ** 64n - 1n;
const xs = [0n, 1n, 2n ** 32n - 1n, 2n ** 32n, 2n ** 53n, 2n ** 53n + 1n, 2n ** 62n + 12345n, T - 1n, T];
let n = 0, bad = 0;
const eq = (got, want) => { n++; if (got !== want) { bad++; console.log('WRONG', String(got), String(want)); } };
for (const x of xs) { eq(genNext(x), x + 1n); eq(genThird(x), x / 3n); }
for (const a of xs) for (const b of xs) eq(genMid(a, b), (a + b) / 2n);
for (const h of [0n, 1n, 2n ** 32n, 2n ** 63n, U - 1n, U]) for (const b of [0, 97, 255]) eq(genStep(h, b), ((h ^ BigInt(b)) * 1099511628211n) % (2n ** 64n));
console.log(n + ' ' + bad);
`
	if err := os.WriteFile(filepath.Join(dir, "d.mjs"), []byte(driver), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, filepath.Join(dir, "d.mjs")).CombinedOutput()
	if err != nil || !strings.HasSuffix(strings.TrimSpace(string(out)), " 0") {
		t.Errorf("the four exports on JavaScript: %v\n%s", err, out)
	}
	ir.PlantPairEntryAsWord = true
	_, err = surfaceGen(t, "js", map[string]string{"p.oro": src})
	ir.PlantPairEntryAsWord = false
	if err == nil {
		t.Error("with the parameter split as a word, the exports still compile: the witness cannot fail")
	}
}
