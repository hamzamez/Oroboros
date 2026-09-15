package core

import (
	"strings"
	"testing"
)

// A SUM'S NAME WAS A GLOBAL KEY, AND THE MODULE LOADED LAST WON.
//
// `Load` collected every module's sums into one table keyed by the bare name and
// checked only that no CONSTRUCTOR belonged to two differently-named sums. Two
// modules each declaring `result` passed that check, the later declaration
// overwrote the earlier, and a signature returning `result` took its payload
// type from whichever module happened to load last. Measured 2026-09-15 on Go:
//
//	module a before module b:  func GenPick(n int) (int, string)
//	module b before module a:  func GenPick(n int) (int, int)
//
// Our checker accepted both; Go refused the first, and JavaScript, which types
// nothing, emitted it without complaint. Inside a program the same confusion is
// invisible, because a payload's type is erased by reduction — which is why no
// program in the corpus ever showed it.
//
// The real fix is a type name resolved like any other name, so `a/result` and
// `b/result` are different types (spec/theories.md §3.4, spec/data.md §5.5).
// Until the loader resolves type names, two DIFFERENT sums with one name are
// refused, so that meaning cannot depend on load order.
func TestTwoDifferentSumsWithOneNameAreRefused(t *testing.T) {
	a := `(module a)
(sum result (ok int) (err int))
(export pick)
(sig pick ((n int)) result)
(def pick (fn (n) (if (< n 0) (err 0) (ok n))))
`
	b := `(module b)
(sum result (ok string) (err string))
(export label)
(def label (fn (s) s))
`
	for _, c := range []struct{ order, src string }{
		{"a before b", a + b},
		{"b before a", b + a},
	} {
		_, err := loadSrc(t, c.src)
		if err == nil {
			t.Errorf("%s: two different sums named result loaded, so a.pick's signature "+
				"depends on which module loads last", c.order)
			continue
		}
		for _, want := range []string{"result", "module a", "module b"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal must name %q (both declarations' modules); got: %v",
					c.order, want, err)
			}
		}
	}
}

// THE CONTROL: the SAME sum declared in two modules still loads.
//
// Every module already holds its own copy of the language's injected `option`
// (newModule), so a rule refusing any repeated name would refuse every program
// with two modules. What makes load order matter is two DIFFERENT declarations;
// two identical ones cannot disagree about anything.
func TestTheSameSumDeclaredInTwoModulesStillLoads(t *testing.T) {
	src := `(module a)
(sum result (ok int) (err int))
(export pick)
(def pick (fn (n) (ok n)))

(module b)
(sum result (ok int) (err int))
(export pick2)
(def pick2 (fn (n) (err n)))
`
	if _, err := loadSrc(t, src); err != nil {
		t.Errorf("two identical sums in two modules, and two copies of option, must load: %v", err)
	}
}
