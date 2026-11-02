package llvm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

func loadTarget(t *testing.T) *emit.Target {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows-llvm", []string{"../../targets"}, []string{"../../lib"})
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

// flagsIR has one operation of each kind the flags rule decides: an add of two
// non-negative operands (nuw nsw), one of an unbounded operand (nsw), a sub
// whose subtrahend is at most its minuend (nuw nsw), one that may go below zero
// (nsw), and an add fused under `mod 2ⁿ` (no flag: its residue is the answer).
const flagsIR = `(ir 1
  (target windows-llvm)
  (stage P)
  (ops add sub mod const yield)
  (func t-flags (params (%0 (int 0 10)) (%1 (int 0 5)) (%2 int) (%3 (int 5 9)))
    (results int int int int int)
    (region
      (val (%4 (int 0 15)) (add exact %0 %1))
      (val (%5 int) (add exact %2 %1))
      (val (%6 (int 0 9)) (sub exact %3 %1))
      (val (%7 (int -5 10)) (sub exact %0 %1))
      (val (%8 int) (add wrap %2 %2))
      (yield %4 %5 %6 %7 %8))))
`

// unjustified checks every flag printed on an add or sub against the IR's own
// facts (llvm.md §1): `nsw` only on an exact operation, `nuw` only where the
// operands' proven ranges keep it from wrapping as unsigned. It answers the
// first flag its proof does not support.
func unjustified(f *ir.Func, out string) string {
	def := map[ir.V]*ir.Stmt{}
	f.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	ends := func(v ir.V) (float64, float64) {
		lo, hi, ok := core.IntRange(f.Types[v])
		if !ok {
			return -1e300, 1e300
		}
		return float64(lo), float64(hi)
	}
	re := regexp.MustCompile(`%v(\d+) = (add|sub|mul)((?: nuw| nsw)*) i64`)
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		n, _ := strconv.Atoi(m[1])
		s := def[ir.V(n)]
		if s == nil {
			continue
		}
		fl := m[3]
		if strings.Contains(fl, "nsw") && s.Mode != ir.MExact {
			return fmt.Sprintf("%%v%d: nsw on a %s operation", n, s.Mode)
		}
		if strings.Contains(fl, "nuw") {
			alo, _ := ends(s.Args[0])
			blo, bhi := ends(s.Args[1])
			ok := alo >= 0 && blo >= 0
			if s.Op == ir.OSub {
				ok = blo >= 0 && alo >= bhi
			}
			if !ok {
				return fmt.Sprintf("%%v%d: nuw on %s %s, %s", n, s.Op, f.Types[s.Args[0]], f.Types[s.Args[1]])
			}
		}
	}
	return ""
}

func printFlags(t *testing.T) (*ir.Func, string) {
	t.Helper()
	tg := loadTarget(t)
	p, err := ir.Read(flagsIR)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Func(tg, p.Funcs[0])
	if err != nil {
		t.Fatal(err)
	}
	return p.Funcs[0], out
}

// A FLAG IS WRITTEN ONLY WHERE THE IR PROVED IT, and each one it can prove is
// written: the printer states the IR's proofs, and only them (ADR 0080 item 2).
func TestAFlagIsTheIRsProof(t *testing.T) {
	f, out := printFlags(t)
	for _, want := range []string{
		"%v4 = add nuw nsw i64 %v0, %v1",
		"%v5 = add nsw i64 %v2, %v1",
		"%v6 = sub nuw nsw i64 %v3, %v1",
		"%v7 = sub nsw i64 %v0, %v1",
		"%v8 = add i64 %v2, %v2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in\n%s", want, out)
		}
	}
	if why := unjustified(f, out); why != "" {
		t.Errorf("a flag without its proof: %s\n%s", why, out)
	}
}

// THE PLANTED FAULT: nuw written on every exact add and sub. The witness above
// must refuse it, or it proves nothing.
func TestAFlagWithoutItsProofIsCaught(t *testing.T) {
	plantNuwFault = true
	defer func() { plantNuwFault = false }()
	f, out := printFlags(t)
	if why := unjustified(f, out); why == "" {
		t.Errorf("the planted nuw passed the witness:\n%s", out)
	}
}

// THE PRINTER IS A FUNCTION OF ITS INPUT: printed twice, one text.
func TestThePrinterIsAFunctionOfItsInput(t *testing.T) {
	_, a := printFlags(t)
	_, b := printFlags(t)
	if a != b {
		t.Errorf("two printings differ:\n%s\n---\n%s", a, b)
	}
}
