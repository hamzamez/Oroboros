package plan

import (
	"path/filepath"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// A BIGNUM READ IS NOT MOVED PAST A WRITE INTO IT. On Go a destination form
// writes into its receiver (ir/bigreuse.go), so t = a·b, read at u after
// s = a.Add(b, a), would read the NEW a if it were written at its use. The
// shape needs t read by a statement after the write, which no source program
// makes today (the IR computes a continue's arguments in order); the premise
// is witnessed on IR built by hand, and the same read with no write between
// is still inlined.
func TestABignumReadIsNotMovedPastAWriteIntoIt(t *testing.T) {
	src := `(ir 1
  (target go)
  (stage A)
  (ops call yield)
  (func t (params (%0 big) (%1 big)) (results big)
    (region
      (val (%2 big) (call big* %0 %1))
      (val (%3 big) (call big+! %0 %1 %0))
      (val (%4 big) (call big- %3 %2))
      (val (%5 big) (call big* %0 %1))
      (val (%6 big) (call big- %4 %5))
      (yield %6))))
`
	p, err := ir.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	fact := filepath.Join("..", "..", "examples", "big", "fact.oro")
	layers, err := emit.SearchPath(fact, filepath.Join("..", "..", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTargetLayers("go", layers, []string{filepath.Dir(fact), filepath.Join("..", "..", "lib")})
	if err != nil {
		t.Fatal(err)
	}
	in := New(tg, p.Funcs[0]).Inlinable()
	if in[2] {
		t.Errorf("a·b read before a.Add(b, a) is written after it")
	}
	if !in[5] {
		t.Errorf("a·b with no write between it and its use is not inlined")
	}
}
