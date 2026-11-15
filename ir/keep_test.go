package ir

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A LENT BUFFER IS HANDED BACK AS ITS OWN STORAGE (ADR 0113, Proposition C):
// written in place, B ⊸ B ⊗ R's handed-back B is the identity on the argument,
// so a kept definition's first results must be, at every return, its lent
// parameter's own storage, the greatest fixed point of a π, a store, a loop's
// parameters and a join's arms over it (sameStorage). No J program can hand
// back another table where its buffer belongs (the buffer is a binder its body
// only writes), so the witness is IR: a loop that stores into the buffer is
// its storage; a join of the buffer with a second buffer is not, and the caller
// would read the first while the callee wrote the second. THE PLANTED FAULT
// accepts the join.
func TestALentBufferIsHandedBackAsItsOwnStorage(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	read := func(body string) *Func {
		p, err := Read(`(ir 1
  (target windows)
  (stage A)
  (ops const add lt len set if loop branch yield break continue)
  (func g (params (%0 (buffer (int 0 255))) (%1 (buffer (int 0 255))) (%2 bool)) (results (buffer (int 0 255)) int)
` + body + `))
`)
		if err != nil {
			t.Fatal(err)
		}
		return p.Funcs[0]
	}
	// the buffer through a loop of stores: its own storage
	own := read(`    (region
      (val (%3 int) (const 0))
      (val (%4 (buffer (int 0 255))) (%5 int)
        (loop (init %0 %3)
          (region
            (params (%6 (buffer (int 0 255))) (%7 int))
            (val (%8 int) (len %6))
            (val (%9 bool) (lt %7 %8))
            (branch %9
              (region
                (val (%10 int) (const 7))
                (val (%11 (buffer (int 0 255))) (set %6 %7 %10))
                (val (%12 int) (const 1))
                (val (%13 int) (add exact %7 %12))
                (continue %11 %13))
              (region
                (break %6 %7))))))
      (yield %4 %5))`)
	if err := inPlace(tg, own, []int{0}); err != nil {
		t.Fatalf("the buffer through a loop of stores: %v; want its own storage", err)
	}
	if len(own.Results) != 1 || len(own.Body.Args) != 1 {
		t.Errorf("the handed-back buffer is not dropped: results %v, yield %v", own.Results, own.Body.Args)
	}
	// a join of the buffer and another: not its own
	join := `    (region
      (val (%3 (buffer (int 0 255)))
        (if %2
          (region
            (yield %0))
          (region
            (yield %1))))
      (val (%4 int) (const 0))
      (yield %3 %4))`
	if err := inPlace(tg, read(join), []int{0}); err == nil || !strings.Contains(err.Error(), "storage other than its own") {
		t.Fatalf("a join of the buffer and a second buffer: %v; want it refused as another storage", err)
	}
	PlantNotInPlace = true
	defer func() { PlantNotInPlace = false }()
	if err := inPlace(tg, read(join), []int{0}); err != nil {
		t.Errorf("with the check planted away: %v; want the join accepted", err)
	}
}

// A WRITE IN A BRANCH'S ARM IS SEEN (p8-2026-11-15): hostWrites walks the
// region tree, terminators included, so a buffer an impure call fills inside a
// `branch` arm of a build is no longer its zero fill. Found where windows'
// repair writes through a kept `put` in every arm of its automaton. THE
// PLANTED FAULT walks statements' regions only: the elements are then [0, 0].
func TestABranchArmsWriteIsSeen(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	tg.Prims["test.fill"] = emit.Prim{Name: "test.fill", Args: []string{"buffer int 0 255"}, Result: "int", Kind: "expr"}
	defer delete(tg.Prims, "test.fill")
	p, err := Read(`(ir 1
  (target windows)
  (stage A)
  (ops const call build branch yield)
  (func g (params (%0 bool)) (results (array int))
    (region
      (val (%1 int) (const 4))
      (val (%2 (array int))
        (build %1
          (region
            (params (%3 (buffer int)))
            (branch %0
              (region
                (val (%4 int) (call test.fill %3))
                (yield %3))
              (region
                (yield %3))))))
      (yield %2))))
`)
	if err != nil {
		t.Fatal(err)
	}
	f := p.Funcs[0]
	if el := analyse(tg, f)[2].el; el != rangeIV(0, 255) {
		t.Fatalf("a build whose arm calls test.fill: elements %s, want [0, 255]", show(el))
	}
	PlantWritesNoBranch = true
	defer func() { PlantWritesNoBranch = false }()
	if el := analyse(tg, f)[2].el; el != rangeIV(0, 0) {
		t.Errorf("with the arms unwalked: elements %s, want the zero fill [0, 0]", show(el))
	}
}
