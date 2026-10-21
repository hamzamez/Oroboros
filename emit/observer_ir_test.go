package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// AN OBSERVER AT A HOST PARAMETER (ADR 0069, emit/observer.go): a pure host call
// whose results are scalars reads a live buffer for the length of the call.

// THE OBSERVER IS DERIVED FROM THE DECLARATION: pure, a table parameter, scalar
// results. Each refusal is the hypothesis it lacks.
func TestAnObserverIsDerivedFromItsDeclaration(t *testing.T) {
	tg := goNative(t)
	for name, want := range map[string]bool{
		"go/unicode/utf8.Valid":          true,  // pure, Bool
		"go/unicode/utf8.RuneCount":      true,  // pure, an integer
		"go/encoding/hex.EncodeToString": false, // pure, but a string result: not a scalar
		"#raw:go/os/File:Write":          false, // impure, and an error among its results
		"go/strconv.AppendInt":           false, // a consume-and-return over a buffer (G7)
		"go/encoding/hex.Encode":         false, // a write-borrow
	} {
		p, ok := tg.Prims[name]
		if !ok {
			t.Fatalf("%s is not declared", name)
		}
		if got := tg.Observer(p); got != want {
			t.Errorf("%s: observer %v, want %v", name, got, want)
		}
	}
}

// A LIVE BUFFER IS OBSERVED BEFORE ITS NEXT STORE, AND THE OBSERVATION KEEPS ITS
// VALUE: ok and n are read before b[2] := 255 and printed after it. Withheld from
// the reducer, the observation moves past the store and linearity refuses the
// program; this pins the order the reducer keeps.
func TestAnObserverReadsALiveBufferBeforeItsNextStore(t *testing.T) {
	out := printsGo(t, `use go
use go/unicode/utf8
use go/fmt
export fn main() {
    var b := array { 0 : _ ∈ 0..3 }
    b[0] := 104
    b[1] := 105
    ok = utf8.Valid(b)
    n = utf8.RuneCount(b)
    b[2] := 255
    fmt.Println(ok, n, utf8.Valid(b), b[2])
}
`)
	valid, count, store := strings.Index(out, "utf8.Valid("), strings.Index(out, "utf8.RuneCount("), strings.Index(out, "= byte(255)")
	if valid < 0 || count < 0 || store < 0 || valid > store || count > store {
		t.Errorf("the observations are not before the store:\n%s", out)
	}
}

// WHAT IS NOT AN OBSERVER STILL TAKES A BUFFER AS IT DID: handed on, a MOVE, the
// freeze at its last use (ADR 0031), so a store after it is a use after the move
// and refused. A string result may share nothing on Go, but the rule claims only
// what a scalar cannot do; an impure call may write; a write-borrow's source
// parameter is read by a call that writes. The same program with an observer in
// that place compiles (TestAnObserverReadsALiveBufferBeforeItsNextStore).
func TestALiveBufferIsRefusedWhereNoObserverStands(t *testing.T) {
	for what, call := range map[string]string{
		"a string result":         `fmt.Println(hex.EncodeToString(b))`,
		"an impure call":          `{ (n, r) = File.Write(os.Stdout(), b); ignore(r); ignore(n) }`,
		"a write-borrow's source": `{ var d := array { 0 : _ ∈ 0..7 }; ignore(hex.Encode(d, b)) }`,
	} {
		err := refusedGo(t, `use go
use go/os
use go/os/File
use go/fmt
use go/encoding/hex
export fn main() {
    var b := array { 0 : _ ∈ 0..3 }
    b[0] := 104
    `+call+`
    b[1] := 105
}
`)
		if err == nil || !strings.Contains(err.Error(), "after it has already been handed on") {
			t.Errorf("%s: %v, want the buffer handed on and the store after it refused", what, err)
		}
	}
}

// GROWTH IS UNCHANGED (G7, ADR 0052): `strconv.AppendInt(b, k, 10)` is a borrow
// statement over a buffer, and the observer leaves it as it was.
func TestGrowthIsStillABorrowStatement(t *testing.T) {
	out := printsGo(t, `use go/strconv
use go/fmt
export fn main() {
    var b := array { 0 : _ ∈ 0..-1 }
    strconv.AppendInt(b, 42, 10)
    fmt.Println(len(b))
}
`)
	if !strings.Contains(out, "strconv.AppendInt(") {
		t.Errorf("growth is not printed as the host's append:\n%s", out)
	}
}

// LINEARITY KEEPS THE RULE SOUND: an observation of a version a store has
// already consumed is a read after a move, refused, whatever the reducer did.
func TestAnObserverAfterAStoreIsAReadAfterAMove(t *testing.T) {
	tg := goNative(t)
	term, err := core.ParseTerm(`(build 4 (fn (b) (let (set b 0 1) (fn (c) (go/unicode/utf8.Valid b)))))`)
	if err != nil {
		t.Fatal(err)
	}
	err = emit.CheckLinear(term, tg, nil)
	if err == nil || !strings.Contains(err.Error(), "is read after it has already been handed on") {
		t.Errorf("an observation of a consumed buffer: %v", err)
	}
}

// printsGo runs the drivers' pipeline on src's first export and returns the Go.
func printsGo(t *testing.T, src string) string {
	t.Helper()
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		t.Fatal(err)
	}
	name := prog.Exports[0]
	f, _, err := ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	out, err := golang.FromFunc(tg, f)
	if err != nil {
		t.Fatalf("not printed: %v", err)
	}
	return out
}

// refusedGo is the pipeline's refusal of src, or nil.
func refusedGo(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		return err
	}
	name := prog.Exports[0]
	_, _, err = ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	return err
}
