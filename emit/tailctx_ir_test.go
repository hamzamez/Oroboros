package emit_test

import (
	"strings"
	"testing"

	"oroboros/ir/golang"
)

// A SEVERAL-RESULTS RETURN IN EACH RESULT'S VALUE TYPE. `(int 0 1)` is stored
// as a byte but computed as an `int` (a `long` on the JVM), so the signature and
// the record field that receive it must say so: `byte` there was a type error on
// both hosts, which the return from inside a host continuation first reached.
func TestASeveralResultsReturnUsesTheValueType(t *testing.T) {
	const src = `(export f)
(sig f ((a (int -1000 1000))) (tuple (int 0 1) int))
(def f (a) (tuple (if (> a 0) 1 0) a))`
	tg, nf, sig := normGo(t, src)
	goSrc, err := golang.FromResidual(tg, "f", sig, nf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(goSrc, ") (int, int) {") {
		t.Errorf("the Go signature must return (int, int):\n%s", goSrc)
	}
}
