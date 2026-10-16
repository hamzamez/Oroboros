package core

import (
	"strings"
	"testing"
)

// RELEVANCE (spec/errors.md §7): weakening is denied for a value of a
// `(relevant)` type. The check is at β, where a binder the program wrote meets
// its value, and the binder must be used on every path of the λ as written.
//
// `write` is the shape of a fallible host call: an effect, then a constructor
// of the relevant `result`. `opt` is the same through a variant that is not
// relevant.
const relevanceDecl = "use go\nset Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant\nset Maybe(T) = 'just(T) ∪ 'nothing success 'just\nfn write(n) {\n    go.print(n)\n    if go.`>`(n, 0) { 'ok(n) } else { 'err(n) }\n}\nfn opt(n) {\n    go.print(n)\n    if go.`>`(n, 0) { 'just(n) } else { 'nothing }\n}\nfn drop(x) { 0 }\nfn check(r) { case r { 'ok(v) -> v; 'err(e) -> go.`-`(0, e) } }"

var relevancePrims = []string{"go.>", "go.-", "if", "=", "!go.print", "!abandon", "ignore", "loop"}

func relevanceReduce(t *testing.T, body string) (string, error) {
	t.Helper()
	p, err := loadSrc(t, relevanceDecl+"\nfn f(n, c) {\n"+body+"\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	nf, err := Normalize(p.Defs["f"], testEnv(p, relevancePrims...), DefaultFuel)
	if err != nil {
		return "", err
	}
	return nf.String(), nil
}

func TestARelevantValueMustBeUsedOnEveryPath(t *testing.T) {
	for _, c := range []struct {
		body string
		ok   bool
	}{
		// in statement position, and bound and never read
		{`{
write(n)
1
}`, false},
		{`{
r = write(n)
1
}`, false},
		{`{
(a, _) = (1, write(n))
a
}`, false},
		// handed to a definition that drops it: η keeps the mark on the tuple
		// that stands for r, so drop's β meets the rule
		{`{
r = write(n)
drop(r)
}`, false},
		{`drop(write(n))`, false},
		// a pure constructor, as written
		{`{
'ok(1)
1
}`, false},
		// used: eliminated, or handed to a definition that eliminates it
		{`{
r = write(n)
check(r)
}`, true},
		{`{ case write(n) { 'ok(v) -> v; 'err(e) -> e } }`, true},
		{`check(write(n))`, true},
		// discarded on purpose
		{`{
ignore(write(n))
1
}`, true},
		{`{
r = write(n)
ignore(r)
1
}`, true},
		// ADDITIVE: an `if` uses r when its condition does, or both arms do
		{`{
r = write(n)
if c { check(r) } else { 0 }
}`, false},
		{`{
r = write(n)
if c { check(r) } else { ignore(r); 0 }
}`, true},
		{"{\nr = write(n)\nif go.`>`(check(r), 0) { 1 } else { 2 }\n}", true},
		// `and` is an `if` whose second arm is false: short-circuit loses r
		{"{\nr = write(n)\nif c && go.`>`(check(r), 0) { 1 } else { 2 }\n}", false},
		// try's failure arm returns the second error and loses the first
		{`{
w = write(n)
try 'ok(u) = write(1) { 'ok(check(w)) }
}`, false},
		// a path that crashes uses it vacuously: 0 is initial
		{`{
r = write(n)
if c { check(r) } else { abandon("x") }
}`, true},
		// a loop uses it when its exits do; an `again` continues
		{"{\nr = write(n)\nloop i = 0 { go.`>`(n, i) -> again(go.`-`(i, 1)); else -> check(r) }\n}", true},
		{"{\nr = write(n)\nloop i = 0 { go.`>`(n, i) -> again(go.`-`(i, 1)); else -> 0 }\n}", false},
		// a relevant payload: the slot that binds it is the compiler's, and
		// is read by the one arm that selects it
		{`{ case 'ok('ok(n)) { 'ok(r) -> check(r); 'err(e) -> e } }`, true},
		// a variant that is not relevant may be dropped
		{`{
opt(n)
1
}`, true},
		{`{
m = opt(n)
drop(m)
}`, true},
	} {
		_, err := relevanceReduce(t, c.body)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused, and it uses its relevant value: %v", c.body, err)
		case !c.ok && err == nil:
			t.Errorf("%s: accepted, and it discards a relevant value", c.body)
		case !c.ok && !strings.Contains(err.Error(), "relevant type"):
			t.Errorf("%s: refused for another reason: %v", c.body, err)
		}
	}
}

// `ignore` IS !_A : A → 1, and erases (spec/errors.md §7.3): on a relevant
// value it is the eliminator with a constant body, whose arms agree, so the
// test goes by idempotence and only the effect is left. η for the terminal
// object then takes the unit's binding apart, so `(seq (ignore e) B)` is what
// `(seq e B)` was before relevance: the effect, then B.
func TestIgnoreLeavesTheEffectAndNothingElse(t *testing.T) {
	got, err := relevanceReduce(t, `{
ignore(write(n))
1
}`)
	if err != nil {
		t.Fatal(err)
	}
	want, err := relevanceReduce(t, `{
opt(n)
1
}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || strings.Contains(got, "ignore") || strings.Contains(got, "#k") {
		t.Errorf("ignore must leave the effect alone:\n got  %s\n want %s", got, want)
	}
	// on a value of any other type: pure, the unit; impure, its effect
	for body, want := range map[string]string{
		"{\nignore(go.`-`(n, 1))\n1\n}": `(fn (n c) 1)`,
		`{
ignore(go.print(n))
1
}`: `(fn (n c) (let (go.print n) (fn (`,
	} {
		got, err := relevanceReduce(t, body)
		if err != nil || !strings.HasPrefix(got, want) {
			t.Errorf("%s: got %s, %v; want %s…", body, got, err, want)
		}
	}
}

// The refusal says what was discarded, and how.
func TestARelevanceRefusalNamesTheValueAndTheBinder(t *testing.T) {
	for body, want := range map[string]string{
		`{
write(n)
1
}`: "the result of write is a value of a relevant type (spec/errors.md §7), and it is discarded",
		`{
r = write(n)
if c { check(r) } else { 0 }
}`: "the binder r is not used on every path",
	} {
		_, err := relevanceReduce(t, body)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", body, want, err)
		}
	}
}

// `again` IS VACUOUS ONLY FOR A VARIABLE THAT OUTLIVES THE JUMP (spec/errors.md
// §7.1, abstract.md §8.1). A value bound inside an iteration ends at the jump,
// so dropping it there is dropping it; one bound outside the loop goes round, and
// the loop uses it when each of its exits does (abstract2-2026-10-11: every
// `again` was vacuous, and the first two were accepted).
func TestAgainUsesOnlyWhatOutlivesTheJump(t *testing.T) {
	loop := func(clause string) string {
		return "loop i = n { go.`>`(i, 3) -> 0; else -> " + clause + " }"
	}
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{loop("{\nr = write(i)\nagain(go.`-`(i, 1))\n}"), false},
		{loop("{\nwrite(i)\nagain(go.`-`(i, 1))\n}"), false},
		{loop("{\nr = write(i)\nagain(go.`-`(check(r), 1))\n}"), true},
		{loop("{\nr = write(i)\nignore(r)\nagain(go.`-`(i, 1))\n}"), true},
		{loop("{\nr = write(i)\ncheck(r)\nagain(go.`-`(i, 1))\n}"), true},
		// bound outside the loop: the jump goes round, and the exits decide
		{"{\nr = write(n)\nloop i = n { go.`>`(i, 3) -> check(r); else -> again(go.`-`(i, 1)) }\n}", true},
		{"{\nr = write(n)\nloop i = n { go.`>`(i, 3) -> 0; else -> again(go.`-`(i, 1)) }\n}", false},
	} {
		_, err := relevanceReduce(t, c.body)
		if c.ok && err != nil {
			t.Errorf("%s: should be accepted: %v", c.body, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "relevant")) {
			t.Errorf("%s: should be refused as a dropped relevant value, got %v", c.body, err)
		}
	}
}
