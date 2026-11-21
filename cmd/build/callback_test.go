package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir"
	"oroboros/ir/java"
	"oroboros/ir/js"
	"oroboros/ir/llvm"
)

// A CALLBACK DURING A CALL IS DECIDED UNDER ITS CLAIM (ADR 0143, ADR 0053
// item 4). The claim is Π(x ∈ P). R: the host promises x ∈ P, and the λ owes
// its result in R. Under a host that promises no order, the λ is pure
// (Theorem D). Each rule has its witness refused, and its fault accepting it:
//   - the domain: the λ's parameters are the claim's sets, so r * r * r on an
//     int32 can reach 2¹⁵⁵, past the double word (r³'s 2⁹³ is E's since ADR
//     0149), and is refused; the fault leaves the parameter
//     unvisited, its fact [0, 0], the defect found while building this;
//   - the result: r + 1 can reach 2³¹, outside the claim's int32, which the
//     host would truncate;
//   - Theorem D: a print in IndexFunc's λ, whose order Go does not promise;
//   - 𝒰's clause: a relevant value used only inside a λ the host may call
//     never, under the default count 0.., is not used on every path.
func TestACallbackDuringACallIsDecidedUnderItsClaim(t *testing.T) {
	src, err := os.ReadFile("../../examples/callbacks/caesar.oro")
	if err != nil {
		t.Fatal(err)
	}
	got, err := runOnMayFail(t, string(src), "go")
	want := "Gzzgiq gz jgct, ygoj Igkygx. Attack at dawn, said Caesar. Attack at dawn, said Caesar. 14 14"
	if err != nil || strings.TrimSpace(got) != want {
		t.Errorf("caesar: want %q, got %q, %v", want, got, err)
	}
	const head = "use io\nuse go/strings\nexport fn main() {\n    s = \"abc,d\"\n"
	cases := []struct {
		name, body, want string
		plant            *bool
	}{
		{"domain", "    io.print_int(strings.IndexFunc(s, fn(r) { r * r * r * r * r > 8 }))\n}\n",
			"cannot be proven to stay inside the word", &ir.PlantCallbackParamUndeclared},
		{"result", "    io.print_line(strings.Map(fn(r) { r + 1 }, s))\n}\n",
			"which is not proven inside -2^31..2^31-1, the claim's result", &ir.PlantCallbackResultUnchecked},
		{"theorem D", "    io.print_int(strings.IndexFunc(s, fn(r) { io.print_int(r); r == 44 }))\n}\n",
			"its host does not promise the order or the number of its calls", &ir.PlantCallbackEffectUnchecked},
	}
	const relevant = `use io
use go/strings
set Token = 'key(Int) ∪ 'nokey relevant
fn make(n ∈ 0..9) (Token) { if n > 5 { 'key(n) } else { 'nokey } }
export fn main() {
    s = "abc,d"
    t = make(strings.Count(s, "c") % 10)
    io.print_int(strings.IndexFunc(s, fn(r) { case t { 'key(n) -> n == r; 'nokey -> false } }))
}
`
	if _, err := buildOnly(t, relevant, "go"); err == nil || !strings.Contains(err.Error(), "the binder t is not used on every path") {
		t.Errorf("𝒰: want t refused as unused on a path, got %v", err)
	}
	core.PlantCallbackUseCounted = true
	_, err = buildOnly(t, relevant, "go")
	core.PlantCallbackUseCounted = false
	if err != nil {
		t.Errorf("𝒰: with a λ's use counted, the program should build, so the witness can fail: %v", err)
	}
	for _, c := range cases {
		if _, err := buildOnly(t, head+c.body, "go"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a refusal saying %q, got %v", c.name, c.want, err)
		}
		*c.plant = true
		_, err := buildOnly(t, head+c.body, "go")
		*c.plant = false
		if err != nil {
			t.Errorf("%s: with its rule's fault planted the program should build, so the witness can fail: %v", c.name, err)
		}
	}
}

// A CELL WRITTEN IN A CALLBACK IS THREADED IN THE HOST'S ORDER (ADR 0143,
// Theorem T₁): under `ordered` the cell is one variable written in place,
// captured by reference, and a read after the call is the join of what the
// activations stored, a fixed point over the host's unknown count. Each rule
// has its witness and its fault:
//   - the translation: prefix.oro counts slices.EqualFunc's comparisons, 6 for
//     an lcp of 5; the fault refuses it as a closure, as before;
//   - Theorem D on a write: strings.Map promises no order, so a count there is
//     refused; the fault threads it anyway;
//   - the join: after the call k is in [0, 10⁶], so k · 2⁶² is refused; the
//     fault drops the stores from the join, k reads as its initial 0, and the
//     product is believed proven.
func TestACellWrittenInACallbackIsThreadedInTheHostsOrder(t *testing.T) {
	src, err := os.ReadFile("../../examples/callbacks/prefix.oro")
	if err != nil {
		t.Fatal(err)
	}
	got, err := runOnMayFail(t, string(src), "go")
	want := "3 3 1 1 4 4 1 1 5 5 9 8 different 6 6"
	if err != nil || strings.TrimSpace(got) != want {
		t.Errorf("prefix: want %q, got %q, %v", want, got, err)
	}
	core.PlantCallbackCellRefused = true
	_, err = buildOnly(t, string(src), "go")
	core.PlantCallbackCellRefused = false
	if err == nil || !strings.Contains(err.Error(), "a function that mentions a cell") {
		t.Errorf("with the translation off, prefix should be refused as a closure: %v", err)
	}
	const mapCount = `use io
use go/strings
export fn main() {
    var n := 0
    t = strings.Map(fn(r) { if n < 1000000 { n := n + 1 }; r }, "abc")
    io.print_int(n)
}
`
	if _, err := buildOnly(t, mapCount, "go"); err == nil || !strings.Contains(err.Error(), "writes the cell n") {
		t.Errorf("a count under strings.Map: want the Theorem D refusal, got %v", err)
	}
	core.PlantCallbackCellOrderUnchecked = true
	_, err = buildOnly(t, mapCount, "go")
	core.PlantCallbackCellOrderUnchecked = false
	if err != nil {
		t.Errorf("with Theorem D unchecked the count should build, so the witness can fail: %v", err)
	}
	const joined = `use io
use go/slices
export fn main() {
    var k := 0
    same = slices.EqualFunc(array { 1, 2 }, array { 1, 3 }, fn(x, y) { if k < 1000000 { k := k + 1 }; x == y })
    t = { var t := array { 0 }; t[0] := k * 4611686018427387904; t }
    io.print_int(t[0])
}
`
	if _, err := buildOnly(t, joined, "go"); err == nil || !strings.Contains(err.Error(), "cannot be proven to stay inside the word") {
		t.Errorf("k · 2⁶² after the call: want refused, k reaching 10⁶, got %v", err)
	}
	ir.PlantCellReadUnjoined = true
	_, err = buildOnly(t, joined, "go")
	ir.PlantCellReadUnjoined = false
	if err != nil {
		t.Errorf("with the stores dropped from the join, k reads as 0 and the product should build: %v", err)
	}
}

// A CALLBACK CAPTURES ITS ENVIRONMENT THROUGH THE CONTEXT WORD (ADR 0143 item
// 6): on windows the λ is closure-converted, E a record in the caller's frame,
// its address in EnumWindows' LPARAM, or, where the API passes none
// (EnumSystemGeoID), in a slot written around the call. The law ∀ distributes
// over ∧ is checked on both triples. The faults: E read from the λ's first
// parameter, the window's handle, instead of the context word; and the slot
// left unwritten, so the λ loads its captures through null and crashes.
func TestACallbackCapturesItsEnvironmentThroughTheContextWord(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs windows programs")
	}
	src, err := os.ReadFile("../../examples/callbacks/allwindows.oro")
	if err != nil {
		t.Fatal(err)
	}
	const law = "0 0 0 11 4"
	for _, tg := range []string{"windows", "windows32"} {
		if got, err := runOnMayFail(t, string(src), tg); err != nil || strings.TrimSpace(got) != law {
			t.Errorf("allwindows on %s: want %q, got %q, %v", tg, law, got, err)
		}
	}
	llvm.PlantCtxSwapped = true
	got, err := runOnMayFail(t, string(src), "windows")
	llvm.PlantCtxSwapped = false
	if err == nil && strings.TrimSpace(got) == law {
		t.Error("with E read from the window's handle the law should not hold, so the witness can fail")
	}
	const geo = `use win/print
use windows/winnls
use windows/errhandlingapi
export fn main() {
    k = errhandlingapi.GetLastError() % 2 + 7
    print.print_int(winnls.EnumSystemGeoID(16, 0, fn(g) { if g == k { 0 } else { 1 } }))
}
`
	for _, tg := range []string{"windows", "windows32"} {
		if got, err := runOnMayFail(t, geo, tg); err != nil || strings.TrimSpace(got) != "1" {
			t.Errorf("EnumSystemGeoID through the slot on %s: want 1, got %q, %v", tg, got, err)
		}
	}
	// THE SLOT'S WITNESS IS ITS TEXT: no context-free enumerator measured
	// (EnumSystemGeoID, EnumSystemLocalesA, EnumSystemCodePagesA,
	// EnumDateFormatsA) answers differently when its callback stops it, and a
	// load through null is undefined, which clang may fold away, so neither an
	// answer nor a crash shows a wrong E. The slot is written with E's address
	// before the call; the fault leaves it unwritten.
	slotWritten := func() bool {
		if _, err := buildOnly(t, geo, "windows"); err != nil {
			t.Fatal(err)
		}
		call := strings.Index(lastCode, "call i32 @EnumSystemGeoID(")
		store := strings.Index(lastCode, "store ptr %.env")
		return call > 0 && store > 0 && store < call
	}
	if !slotWritten() {
		t.Error("the slot should be written with E's address before EnumSystemGeoID is called")
	}
	llvm.PlantSlotUnwritten = true
	unwritten := slotWritten()
	llvm.PlantSlotUnwritten = false
	if unwritten {
		t.Error("with the slot unwritten the text should hold no store of E before the call, so the witness can fail")
	}
}

// A CALLBACK IS THE HOST'S OWN CLOSURE ON JAVASCRIPT AND THE JVM (ADR 0143):
// an arrow function bound to the λ's value, and a lambda written where its one
// use is, typed by the stream's LongPredicate. The programs check quantifier
// laws: De Morgan, the least witness and findIndex on JavaScript (11 of 11),
// and ∀ over ∧ and ∃ over ∨ on the JVM (100 of 100). The JVM's rule: a lambda
// reads only effectively final locals, so a captured loop variable is copied
// to a final one; the fault reads it as it stands, and javac refuses.
func TestACallbackIsTheHostsOwnClosureOnJavaScriptAndTheJVM(t *testing.T) {
	for _, c := range []struct{ file, target, want string }{
		{"quantifiers.oro", "js", "11"},
		{"semilattice.oro", "java", "100"},
	} {
		src, err := os.ReadFile("../../examples/callbacks/" + c.file)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := runOn(t, string(src), c.target); err != nil || strings.TrimSpace(got) != c.want {
			t.Errorf("%s on %s: want %s, got %q, %v", c.file, c.target, c.want, got, err)
		}
	}
	const loopVar = `use io
use java/Arrays
export fn main() {
    a = array { 3, 1, 4 }
    loop k = 0 {
        k < 3 -> { io.print_line(if Arrays.allMatch(a, fn(x) { x > k }) { "all" } else { "not all" }); again(k + 1) }
        else -> io.print_int(k)
    }
}
`
	if got, err := runOn(t, loopVar, "java"); err != nil || strings.TrimSpace(got) != "all not all not all 3" {
		t.Errorf("a loop variable captured on the JVM: want all not all not all 3, got %q, %v", got, err)
	}
	java.PlantJavaCaptureUnfinal = true
	_, err := buildOnly(t, loopVar, "java")
	java.PlantJavaCaptureUnfinal = false
	if err == nil {
		t.Error("with the loop variable captured as it stands javac should refuse it, so the witness can fail")
	}
}

// A CELL WRITTEN IN A CALLBACK, ON EVERY TARGET (ADR 0143, Theorem T₁). Each
// program writes cells from a λ given to a host whose documentation fixes the
// order of its calls, and checks an answer that depends on that order against
// a loop of its own:
//   - JavaScript, `Array.prototype.forEach` (ECMA-262: "once for each element
//     present in the array, in ascending order"): the Dyck words of length 8,
//     the path's end and its minimum two cells, 14 balanced (Catalan's C₄) and
//     256 of 256 agreeing with the loop. The arrow closes over the cell's
//     variable; the fault writes a variable of the arrow's own, and the count
//     changes.
//   - the JVM, `LongStream.forEachOrdered` over a long[] (the JDK: "encounter
//     order", the array's spliterator ORDERED): Kadane's maximum subarray, two
//     cells, Bentley's example 6 and 100 of 100 tables agreeing with the
//     quadratic search. Each cell a final holder of one element the lambda
//     shares; the fault gives the lambda a clone, and the answers change.
//   - windows, both triples, `InitOnceExecuteOnce` (its page: "only one thread
//     will execute the callback", so `calls 0..1`, one activation one order):
//     one-time initialization is idempotent on success and leaves the block
//     fresh on failure, so f runs once over two successful calls and twice
//     over two failing ones, 1 1 1 2 0 0. The cell an alloca of the word,
//     reached through the context word; the faults drop the λ's store, and
//     widen the λ's held cell with no thresholds (the count then unbounded
//     after the call, and the program refused).
func TestACellWrittenInACallbackOnEveryTarget(t *testing.T) {
	src, err := os.ReadFile("../../examples/callbacks/dyck.oro")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := runOn(t, string(src), "js"); err != nil || strings.TrimSpace(got) != "14 256" {
		t.Errorf("dyck on js: want 14 256, got %q, %v", got, err)
	}
	js.PlantCellWriteLocal = true
	got, err := runOn(t, string(src), "js")
	js.PlantCellWriteLocal = false
	if err == nil && strings.TrimSpace(got) == "14 256" {
		t.Error("with the arrow writing a copy of its own the answer should change, so the witness can fail")
	}
	src, err = os.ReadFile("../../examples/callbacks/kadane.oro")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := runOn(t, string(src), "java"); err != nil || strings.TrimSpace(got) != "6 100" {
		t.Errorf("kadane on java: want 6 100, got %q, %v", got, err)
	}
	java.PlantJavaHolderCopied = true
	got, err = runOn(t, string(src), "java")
	java.PlantJavaHolderCopied = false
	if err == nil && strings.TrimSpace(got) == "6 100" {
		t.Error("with the lambda given a clone of the holder the answer should change, so the witness can fail")
	}
	src, err = os.ReadFile("../../examples/callbacks/once.oro")
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range []string{"windows", "windows32"} {
		if got, err := runOnMayFail(t, string(src), tg); err != nil || strings.TrimSpace(got) != "1 1 1 2 0 0" {
			t.Errorf("once on %s: want 1 1 1 2 0 0, got %q, %v", tg, got, err)
		}
	}
	llvm.PlantCellSetDropped = true
	got, err = runOnMayFail(t, string(src), "windows")
	llvm.PlantCellSetDropped = false
	if err == nil && strings.TrimSpace(got) == "1 1 1 2 0 0" {
		t.Error("with the λ's store dropped the count should change, so the witness can fail")
	}
	ir.PlantCallbackWidenedPlainly = true
	_, err = buildOnly(t, string(src), "windows")
	ir.PlantCallbackWidenedPlainly = false
	if err == nil || !strings.Contains(err.Error(), "[0, +inf]") {
		t.Errorf("with the λ widened plainly the count is [0, +inf] and the program should be refused: %v", err)
	}
}

// A DEFINITION APPLIED TO A λ THAT WRITES A CELL IS EXPANDED (ADR 0144): the
// program's own definition is the strongest host, its applications of the λ
// the sequence its body fixes, so the λ's cell is threaded through them as a
// host's under `ordered`, and on no host at all: the cell becomes a variable
// of the definition's own loop. Horner's rule through `each`, checked against
// the sum of powers at t = 0 … 9, and `twice` counting its applications, on
// every target: 89 10 7 2. The fault leaves the definition unexpanded, and the
// λ is refused as a closure, as before.
func TestADefinitionAppliedToALambdaThatWritesACellIsExpanded(t *testing.T) {
	src, err := os.ReadFile("../../examples/callbacks/horner.oro")
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range []string{"go", "js", "java", "windows", "windows32"} {
		run := runOnMayFail
		if tg == "js" || tg == "java" {
			run = runOn
		}
		if got, err := run(t, string(src), tg); err != nil || strings.Join(strings.Fields(got), " ") != "89 10 7 2" {
			t.Errorf("horner on %s: want 89 10 7 2, got %q, %v", tg, got, err)
		}
	}
	core.PlantHigherNotExpanded = true
	_, err = buildOnly(t, string(src), "go")
	core.PlantHigherNotExpanded = false
	if err == nil || !strings.Contains(err.Error(), "a function that mentions a cell") {
		t.Errorf("with the definition unexpanded the λ should be refused as a closure: %v", err)
	}
}
