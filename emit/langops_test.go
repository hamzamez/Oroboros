package emit

import (
	"strings"
	"testing"
)

// THE LANGUAGE OWNS INTEGER ARITHMETIC, on exactly `=`'s argument.
//
// Until this, `=` was the ONLY integer operator the language had: `(+ 1 2)` was
// "not bound", every gauntlet program lived in `examples/native/` writing
// `go.+` or `x64.add`, and the differential harness carried a macro table
// expanding `@add` into four spellings. Every portability claim in the
// repository was really a claim about `go.+`.
//
// integers.md asked the eleven questions on all four hosts and found them to
// AGREE on everything inside ADR 0012's window, which is what makes these
// promotable: division truncates toward zero, the remainder takes the
// dividend's sign, and `(a/b)*b + a%b == a`.
func TestEveryTargetProvidesTheLanguageOperators(t *testing.T) {
	for _, dir := range []string{"../targets/go", "../targets/js", "../targets/java", "../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range []string{"+", "-", "*", "/", "%", "<", "<=", ">", ">=", "="} {
			p, ok := tg.Prims[op]
			if !ok {
				t.Errorf("%s does not provide %q; a target may not decline a language "+
					"construct, and the compiler is what finds the implementation", dir, op)
				continue
			}
			if len(p.Args) != 2 {
				t.Errorf("%s: %q has %d argument(s), want 2", dir, op, len(p.Args))
			}
			if p.Form == "" {
				t.Errorf("%s: %q has no emission template", dir, op)
			}
		}
	}
}

// A COMPARISON YIELDS A BOOLEAN AND ARITHMETIC DOES NOT, and the search has to
// enforce it — otherwise a host that spells something `<` for another purpose
// could be picked, or `+` could resolve to a predicate.
func TestTheOperatorSearchRespectsTheResultKind(t *testing.T) {
	for _, dir := range []string{"../targets/go", "../targets/java", "../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range []string{"<", "<=", ">", ">=", "="} {
			if got := tg.Prims[op].Result; got != "bool" {
				t.Errorf("%s: %q returns %q, want bool", dir, op, got)
			}
		}
		for _, op := range []string{"+", "-", "*", "/", "%"} {
			if got := tg.Prims[op].Result; got == "bool" {
				t.Errorf("%s: %q returns bool", dir, op)
			}
		}
	}
}

// GO'S FLOAT ADDITION MUST NEVER BE PICKED FOR `+`. Go declares `+` for
// integers and `f+` for floats; the search compares the whole unqualified name,
// so `f+` cannot match `+`.
func TestFloatArithmeticIsNotPromoted(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// The TEMPLATES are identical — `%s + %s` for both — so the emission cannot
	// tell them apart, and neither can a test that compares it. What separates
	// them is the result type, which is exactly what the host declared.
	if got := tg.Prims["+"].Result; got != "int" {
		t.Errorf("`+` on Go returns %q, want int — Go's float addition is `f+` "+
			"and must not be picked, which the search gets right by comparing "+
			"the whole unqualified name", got)
	}
	if got := tg.Prims["go.f+"].Result; got != "f64" {
		t.Fatalf("go.f+ returns %q, so this test is no longer discriminating", got)
	}
}

// JAVASCRIPT'S `/` IS FLOAT DIVISION — `7 / 2` is 3.5 — where the other three
// divide integers and truncate toward zero. So the language's `/` must resolve
// to that host's declared integer division instead, which is the parasite model
// doing its job: the host that lacks the operation says how to get it, in its
// own file, and no other target learns that JavaScript is different.
//
// This is the one place the promotion is not just a rename, and without it the
// same source printed 7003 on three targets and 7003.5 on the fourth.
func TestJavaScriptDivisionIsIntegerDivision(t *testing.T) {
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	// Asserted as PROPERTIES rather than one exact string, because the template
	// has already had to grow once — for negative zero — and a test that pins
	// the spelling fails on every future fix rather than on a regression.
	for _, op := range []string{"/", "%"} {
		got := tg.Prims[op].Form
		if got == "%s / %s" {
			t.Errorf("the language's %q resolved to JavaScript's FLOAT division; "+
				"7 / 2 is 3.5 there and 3 everywhere else", op)
		}
		if strings.Contains(got, "| 0") || strings.Contains(got, "|0") {
			t.Errorf("%q on JavaScript uses `| 0`, which coerces to int32 and "+
				"wraps every value past 2^31 — well inside the window: %s", op, got)
		}
		// NEGATIVE ZERO: `Math.trunc(-1/2)` and `-2 % 2` are both `-0` on V8 and
		// `0` on the other three. It compares equal, `String(-0)` is "0", and
		// every arithmetic operation normalises it — so it hides everywhere
		// except a raw print, where `console.log` shows `-0`. `+ 0` maps it to
		// `0` and is the identity on every other value.
		if !strings.Contains(got, "+ 0") {
			t.Errorf("%q on JavaScript does not normalise negative zero: %s\n"+
				"the same program printed 0 on three targets and -0 on one", op, got)
		}
	}
	if got := tg.Prims["/"].Form; !strings.Contains(got, "Math.trunc") {
		t.Errorf("`/` on JavaScript is %q, want a truncating division", got)
	}
}

// BITWISE AND SHIFTS ARE DELIBERATELY NOT PROMOTED, and the reason is a
// measured divergence rather than caution: JavaScript coerces both operands to
// int32 for `& | ^ << >>` (and uint32 for `>>>`), so `(2^32) & -1` is 0 on V8
// and 4294967296 on Go, Java and x86 — an OBSERVABLE disagreement INSIDE ADR
// 0012's window, which is exactly what makes a construct Tier 2.
//
// They stay target-native, where a program using one has chosen its host.
func TestBitwiseIsNotPromoted(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"&", "|", "^", "<<", ">>", "~"} {
		if _, ok := tg.Prims[op]; ok {
			t.Errorf("%q was promoted to the language; JavaScript truncates it "+
				"to int32, which is observable inside the portable window", op)
		}
	}
}

// A LANGUAGE OPERATOR'S RESULT IS THE LANGUAGE'S, NOT THE HOST'S — and this is
// the test for a soundness hole, not for tidiness.
//
// `targets/js` declares everything `any` on purpose: JavaScript has one number
// type and untyped containers, so one declaration serves every element type.
// `addCore` copies the host's primitive when it promotes `+` to the language,
// and it used to copy the RESULT TYPE too — so the language's `+` returned
// `any` on that host.
//
// `emit/interval.go`'s transfer function ignores any operation whose result is
// not `int`. So the analysis counted ZERO integer operations on JavaScript, and
// `emit.Unbounded` returns success when there is nothing to prove:
//
//	fib without a declaration    go REFUSED   java REFUSED   windows REFUSED
//	                             js  compiled
//
// ADR 0019's bounded-by-default was enforced on three targets and VACUOUS on
// the fourth — and the fourth is the one whose failure mode is silent precision
// loss, which is the case the ADR's own headline example is about: fib(100)
// wraps on Go and the JVM and comes back 354224848179262000000 on V8. The
// refusal fired everywhere except where it mattered most.
//
// It is sound to say `int` because these names are the LANGUAGE's: their
// arguments are already declared `(int int)`, and integers.md measured all four
// hosts agreeing on their meaning inside the window. Only the result had been
// left to whatever the host happened to say.
func TestTheLanguagesArithmeticIsTypedByTheLanguage(t *testing.T) {
	for _, dir := range []string{"../targets/go", "../targets/js", "../targets/java", "../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range []string{"+", "-", "*", "/", "%"} {
			if got := tg.Prims[op].Result; got != "int" {
				t.Errorf("%s: the language's %q returns %q, not int — the interval "+
					"analysis skips it, so bounded-by-default is vacuous on this "+
					"target", dir, op, got)
			}
		}
	}
}
