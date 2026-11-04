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
	tg, err := emit.LoadTargetLayers("windows", []string{"../../targets"}, []string{"../../lib"})
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
  (target windows)
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

// compile builds a J source for windows as the drivers do, and prints it.
func compile(t *testing.T, src string) (string, error) {
	t.Helper()
	tg := loadTarget(t)
	forms, err := core.Read(src)
	if err != nil {
		return "", err
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		return "", err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return "", err
	}
	var sigs []*core.Sig
	for _, s := range prog.Sigs {
		sigs = append(sigs, s)
	}
	if err := emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, sigs)); err != nil {
		return "", err
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		return "", err
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return "", err
	}
	f, _, err := ir.Entry(tg, q, q, prog.Sigs[q], nf, sigs, false, nil)
	if err != nil {
		return "", err
	}
	return FromFunc(tg, f)
}

// A TARGET STATES ITS REALIZATIONS (ADR 0088): each of the language's
// operators is the declaration windows.oro names, and nothing is found by
// spelling; LLVM's own `shl` keeps its own domain.
func TestATargetStatesItsRealizations(t *testing.T) {
	tg := loadTarget(t)
	if !tg.States() {
		t.Fatal("windows states no realization")
	}
	if why := unstated(tg); why != "" {
		t.Error(why)
	}
	if rs := tg.Realize["<<"]; len(rs) != 2 || rs[1] != "llvm.shl" {
		t.Errorf("<<'s realizations are %v, want lang.shl then llvm.shl", rs)
	}
	if c := tg.Prims["+"].Checked; c != "lang.add-checked" {
		t.Errorf("+'s checked form is %q", c)
	}
}

// `<<` IS LLVM's `shl` WHERE THE COUNT IS PROVEN BELOW 64, with the flags the
// IR proved, and the total form elsewhere; a program naming `llvm.shl` owes
// its domain.
func TestAShiftIsTheInstructionWhereItsCountIsProven(t *testing.T) {
	out, err := compile(t, "export fn g(a ∈ 0..1000, k ∈ 0..40) (Int) { a << k }")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "= shl nuw nsw i64") || strings.Contains(out, ", 63") {
		t.Errorf("a count proven below 64 should print the bare instruction with its flags:\n%s", out)
	}
	out, err = compile(t, "export fn g(k ∈ 0..100) (Int) { 0 << k }")
	if err != nil {
		t.Fatal(err)
	}
	// THE TERM, lang.shl: the count compared with 63, LLVM's shl on the arm that
	// proves it below 64, with that arm's flags, and 0 past it.
	if !strings.Contains(out, "icmp sle i64") || !strings.Contains(out, "= shl nuw nsw i64 0,") {
		t.Errorf("a count up to 100 should print lang.shl's term:\n%s", out)
	}
	if _, err := compile(t, "use llvm\nexport fn g(k ∈ 0..100) (Int) { llvm.shl(0, k) }"); err == nil {
		t.Errorf("llvm.shl with a count up to 100 was accepted: its domain is 0..63")
	}
	if _, err := compile(t, "use llvm\nexport fn g(k ∈ 0..63) (Int) { llvm.shl(0, k) }"); err != nil {
		t.Errorf("llvm.shl inside its domain was refused: %v", err)
	}
}

// THE PLANTED FAULT: names found by spelling on a stating target, so `<<` is
// LLVM's `shl`, partial past 63. TestATargetStatesItsRealizations must see it.
func TestASpelledRealizationIsCaught(t *testing.T) {
	emit.PlantSpellingFault = true
	defer func() { emit.PlantSpellingFault = false }()
	if unstated(loadTarget(t)) == "" {
		t.Errorf("with the spelling fault planted, every name is still the declaration stated: the check cannot fail")
	}
}

// unstated is the first of the language's names a target realizes otherwise
// than it states, or "".
func unstated(tg *emit.Target) string {
	for _, op := range []string{"+", "-", "*", "/", "%", "=", "<", "<=", ">", ">=", "&", "|", "⊕", "~", "<<", ">>", "mod", "string-of", "concat", "abandon", "trap-if"} {
		r, ok := tg.Realized[op]
		if !ok {
			return op + " is not stated"
		}
		first, _, _ := strings.Cut(r, "|")
		first, _, _ = strings.Cut(first, ",")
		// A TERM REALIZES IT (ADR 0089): no template; the IR inlines the term.
		if tg.IsDefinition(first) {
			if got := tg.Prims[op].Form; got != "" {
				return fmt.Sprintf("%s is %q, want no template: %s is a term", op, got, first)
			}
			continue
		}
		if got, want := tg.Prims[op].Form, tg.Prims[first].Form; got != want || got == "" {
			return fmt.Sprintf("%s is %q, want %s's %q", op, got, first, want)
		}
	}
	return ""
}

// A CALL INTO ∅ NEVER RETURNS (ADR 0090): the branch it ends is typed by its
// other arm, 0 → Y, and the region ends `unreachable`.
func TestACallIntoTheEmptySetNeverReturns(t *testing.T) {
	src := "use llvm\nexport fn g(c ∈ 0..1) (Int) { if c == 1 { llvm.trap() } else { 5 } }"
	out, err := compile(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "call void @llvm.trap()\n  unreachable") {
		t.Errorf("the trap should end its block:\n%s", out)
	}
	emit.PlantBottomFault = true
	defer func() { emit.PlantBottomFault = false }()
	if _, err := compile(t, src); err == nil {
		t.Errorf("with only abandon never returning, a branch ending in llvm.trap was accepted: the test does not reach the rule")
	}
}
