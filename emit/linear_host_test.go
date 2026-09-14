package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A BUFFER IS LINEAR WHEREVER IT IS BOUND (host-buffers.md §5). These are §2's
// witnesses B and E, which the checker ACCEPTED while it was seeded only by
// `build`'s binder and a declared parameter: built and run, E printed 195 — the
// second write seen through a dead name — and B printed an `x` that `y` had
// overwritten. Each comes with the control it must not disturb.

// hostBufferTarget declares a write-borrow and an append-shaped call on buffers,
// the two shapes host-buffers.md §1 names, exactly as a target file would.
func hostBufferTarget(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	buf := "buffer int 0 255"
	tg.Prims["x.encode"] = Prim{Name: "x.encode", Args: []string{buf, "int"},
		Results: []string{buf, "int"}, Kind: "expr",
		Form: "func(p []byte, r int32) ([]byte, int) { return p, utf8.EncodeRune(p, r) }(%s, int32(%s))"}
	tg.Prims["x.append"] = Prim{Name: "x.append", Args: []string{buf, "int"},
		Result: buf, Kind: "expr", Form: "utf8.AppendRune(%s, int32(%s))"}
	return tg
}

func nm(s string) *core.Term { return core.Name(s) }

// (fn (k) (build 4 (fn (b) ((x.encode b 26085) (fn (b2 m)
//     ((x.encode b2 233) (fn (b3 m2) (let (b2 0) (fn (v) b3)))))))))
func reborrowTerm(readDead bool) *core.Term {
	var tail *core.Term
	if readDead {
		tail = core.App(nm("let"), core.App(nm("b2"), core.Int(0)), core.Fn([]string{"v"}, nm("b3")))
	} else {
		tail = core.App(nm("let"), core.App(nm("b3"), core.Int(0)), core.Fn([]string{"v"}, nm("b3")))
	}
	inner := core.App(core.App(nm("x.encode"), nm("b2"), core.Int(233)), core.Fn([]string{"b3", "m2"}, tail))
	outer := core.App(core.App(nm("x.encode"), nm("b"), core.Int(26085)), core.Fn([]string{"b2", "m"}, inner))
	return core.Fn([]string{"k"}, core.App(nm("build"), core.Int(4), core.Fn([]string{"b"}, outer)))
}

func TestABufferAHostCallHandsBackIsLinear(t *testing.T) {
	tg := hostBufferTarget(t)
	err := CheckLinear(reborrowTerm(true), tg, nil)
	if err == nil {
		t.Fatal("reading b2 after handing it to a second write-borrow must be refused: " +
			"b3 is the same storage, and the read observes the second write")
	}
	if !strings.Contains(err.Error(), "b2") || !strings.Contains(err.Error(), "host call") {
		t.Errorf("the refusal must name the buffer and where it came from, got: %v", err)
	}
	// THE CONTROL: the same program reading the buffer it was handed back.
	if err := CheckLinear(reborrowTerm(false), tg, nil); err != nil {
		t.Fatalf("threading the handed-back buffer is the write-borrow working, and was refused: %v", err)
	}
}

// (fn (k) (build 1 (fn (h) (let (x.append h 233) (fn (base)
//     (let (x.append base 26085) (fn (x) (let (x.append <base|x> 128578) (fn (y) y)))))))))
func appendTerm(twice bool) *core.Term {
	second := nm("x")
	if twice {
		second = nm("base")
	}
	y := core.App(nm("let"), core.App(nm("x.append"), second, core.Int(128578)), core.Fn([]string{"y"}, nm("y")))
	x := core.App(nm("let"), core.App(nm("x.append"), nm("base"), core.Int(26085)), core.Fn([]string{"x"}, y))
	base := core.App(nm("let"), core.App(nm("x.append"), nm("h"), core.Int(233)), core.Fn([]string{"base"}, x))
	return core.Fn([]string{"k"}, core.App(nm("build"), core.Int(1), core.Fn([]string{"h"}, base)))
}

func TestABufferALetBindsIsLinear(t *testing.T) {
	tg := hostBufferTarget(t)
	err := CheckLinear(appendTerm(true), tg, nil)
	if err == nil {
		t.Fatal("appending to base twice must be refused: the two results can share base's capacity")
	}
	if !strings.Contains(err.Error(), "base") {
		t.Errorf("the refusal must name base, got: %v", err)
	}
	if err := CheckLinear(appendTerm(false), tg, nil); err != nil {
		t.Fatalf("appending to each result in turn is linear, and was refused: %v", err)
	}
}

// A LOOP VARIABLE THAT HOLDS A BUFFER is bound by the loop, and a store to it
// followed by a read of the old name in the same `again` is the same violation
// one binder further in. Before, the walk entered the loop's λ with the name
// still bound, so it never saw this.
func TestALoopVariableHoldingABufferIsLinear(t *testing.T) {
	tg := hostBufferTarget(t)
	loop := func(bad bool) *core.Term {
		var second *core.Term = core.Int(0)
		if bad {
			second = core.App(nm("c"), core.Int(0))
		}
		body := core.App(nm("if"), core.App(nm(">="), nm("i"), core.Int(4)), nm("c"),
			core.App(nm("again"), core.App(nm("set"), nm("c"), nm("i"), core.Int(7)), core.App(nm("+"), nm("i"), core.Int(1)), second))
		lp := core.App(nm("loop"), core.Fn([]string{"c", "i", "z"}, body), nm("b"), core.Int(0), core.Int(0))
		return core.Fn([]string{"k"}, core.App(nm("build"), core.Int(4), core.Fn([]string{"b"}, lp)))
	}
	if err := CheckLinear(loop(true), tg, nil); err == nil {
		t.Fatal("reading c after storing into it in the same again must be refused")
	}
	if err := CheckLinear(loop(false), tg, nil); err != nil {
		t.Fatalf("the ordinary fill loop was refused: %v", err)
	}
}

// AN IMMUTABLE ARRAY MAY NOT REACH A PARAMETER DECLARED A BUFFER. That is the
// theorem's second hypothesis (host-buffers.md §4), and it was not checked: the
// type checker gives `build` no result type, so a frozen array handed to a
// write-borrow type-checked, built and ran (witness G) — and the host would have
// written into a value this language promises nobody writes.
//
// (fn (k) (let (build 1 (fn (b) (set b 0 104))) (fn (h) (x.append h 233))))
func TestAnArrayIsNotABuffer(t *testing.T) {
	tg := hostBufferTarget(t)
	frozen := core.Fn([]string{"k"}, core.App(nm("let"),
		core.App(nm("build"), core.Int(1), core.Fn([]string{"b"}, core.App(nm("set"), nm("b"), core.Int(0), core.Int(104)))),
		core.Fn([]string{"h"}, core.App(nm("x.append"), nm("h"), core.Int(233)))))
	err := CheckLinear(frozen, tg, nil)
	if err == nil {
		t.Fatal("a frozen array handed to a parameter declared a buffer must be refused")
	}
	if !strings.Contains(err.Error(), "h") || !strings.Contains(err.Error(), "buffer") {
		t.Errorf("the refusal must name the value and say a buffer was required, got: %v", err)
	}
	// THE CONTROL: the same call inside the build, on the buffer itself.
	inside := core.Fn([]string{"k"}, core.App(nm("build"), core.Int(1), core.Fn([]string{"b"},
		core.App(nm("x.append"), core.App(nm("set"), nm("b"), core.Int(0), core.Int(104)), core.Int(233)))))
	if err := CheckLinear(inside, tg, nil); err != nil {
		t.Fatalf("appending to the buffer being built was refused: %v", err)
	}
}

// TWO THINGS ARE NOT MOVES, and walking loop variables for the first time found
// both on programs that were already correct. Each was a refusal of a legal
// program before it was a rule, so each is pinned by the program's shape.
//
// `again`'s arguments are simultaneous: the tokeniser passes its stack through
// unchanged in one argument and reads its top in another (examples/json/
// tokenize.oro). And `(len b)` observes: the limb library's `guard` reads a
// buffer's length and then a limb of it (emit/bignum.oro).
func TestAPassThroughAndALengthAreNotMoves(t *testing.T) {
	tg := hostBufferTarget(t)
	wrap := func(body *core.Term) *core.Term {
		lp := core.App(nm("loop"), core.Fn([]string{"c", "i", "z"}, body), nm("b"), core.Int(0), core.Int(0))
		return core.Fn([]string{"k"}, core.App(nm("build"), core.Int(4), core.Fn([]string{"b"}, lp)))
	}
	// (if (>= i 4) c (again c (+ i 1) (c 0)))
	passThrough := wrap(core.App(nm("if"), core.App(nm(">="), nm("i"), core.Int(4)), nm("c"),
		core.App(nm("again"), nm("c"), core.App(nm("+"), nm("i"), core.Int(1)), core.App(nm("c"), core.Int(0)))))
	if err := CheckLinear(passThrough, tg, nil); err != nil {
		t.Errorf("passing the buffer through unchanged beside a read of it was refused: %v", err)
	}
	// (if (>= i (len c)) (if (< (c 0) 1) c c) (again (set c i 7) (+ i 1) z))
	length := wrap(core.App(nm("if"), core.App(nm(">="), nm("i"), core.App(nm("len"), nm("c"))),
		core.App(nm("if"), core.App(nm("<"), core.App(nm("c"), core.Int(0)), core.Int(1)), nm("c"), nm("c")),
		core.App(nm("again"), core.App(nm("set"), nm("c"), nm("i"), core.Int(7)), core.App(nm("+"), nm("i"), core.Int(1)), nm("z"))))
	if err := CheckLinear(length, tg, nil); err != nil {
		t.Errorf("reading a buffer's length and then a slot of it was refused: %v", err)
	}
	// THE CONTROL for both: a pass-through beside a STORE is still two moves.
	twoMoves := wrap(core.App(nm("if"), core.App(nm(">="), nm("i"), core.Int(4)), nm("c"),
		core.App(nm("again"), nm("c"), core.App(nm("+"), nm("i"), core.Int(1)), core.App(nm("len"), core.App(nm("set"), nm("c"), core.Int(0), core.Int(1))))))
	if err := CheckLinear(twoMoves, tg, nil); err == nil {
		t.Error("passing c through and storing into it in the same again must be refused")
	}
}
