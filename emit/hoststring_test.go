package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// AT A HOST BOUNDARY A STRING IS THE HOST'S (ADR 0030). Ours is Σ*, the host's
// is B*, `string ≤ go.bytestring` by UTF-8 — the identity on the stored bytes —
// and the only way back is d, `go.text`. The run-time half, that `os.text-of`
// is d on three hosts, is the differential case `text-of`.

// checkGo reduces a program's first export (or its only term) on the Go target
// and type-checks the residual.
func checkGo(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	term := terms[0]
	if len(prog.Exports) > 0 {
		term = prog.Defs[prog.Exports[0]]
	}
	nf, err := core.Normalize(term, env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return Check(tg, "test", nf)
}

// THE HOST'S BYTES ARE NOT OURS. `Unquote("\xe6")` and `Unquote("\x97\xa5")`
// are each outside Σ*, and their concatenation is 日: a scalar neither holds
// (Theorem 2). So a host string may not reach `concat` — nor any parameter
// declared `string` — without the decode. Before, it did: the realizations are
// one Go `string`, and identity by realization let the bytes through.
func TestTheHostsStringIsNotOurs(t *testing.T) {
	err := checkGo(t, `(use go/strconv as sc)
(fn (q) ((sc.Unquote q) (fn (s e) (concat s "x"))))`)
	if err == nil || !strings.Contains(err.Error(), "bytestring") {
		t.Errorf("a host string was accepted as ours; got %v", err)
	}
}

// OURS IS THE HOST'S, FOR FREE. A value of Σ* is a Go string already, so it
// flows where the host's string is wanted: `valid` of a literal, and `Quote`,
// which reads any Go string.
func TestOursIsAHostString(t *testing.T) {
	for _, src := range []string{
		`(use go) (fn (q) (go.valid (concat "a" q)))`,
		`(use go/strconv as sc) (fn (q) (sc.Quote (concat "a" q)))`,
	} {
		if err := checkGo(t, src); err != nil {
			t.Errorf("our string where the host's is wanted: %v\n%s", err, src)
		}
	}
}

// THE WAY BACK IS THE DECODE, and after it the value is ours.
func TestTheWayBackIsTheDecode(t *testing.T) {
	if err := checkGo(t, `(use go) (use go/strconv as sc)
(fn (q) ((sc.Unquote q) (fn (s e) (concat (go.text s) "x"))))`); err != nil {
		t.Errorf("d gives a value of Σ*: %v", err)
	}
}

// A LANGUAGE TYPE IS NEVER A HOST TYPE BY REALIZATION, in either direction;
// an opaque host type named under two keys still is.
func TestALanguageTypeIsNotAHostTypeByRealization(t *testing.T) {
	tg := goNative(t)
	if tg.SameHostType("string", "go.bytestring") || tg.SameHostType("go.bytestring", "string") {
		t.Error("string and go.bytestring were identified by their realization")
	}
	if !tg.Subsumes("string", "go.bytestring") {
		t.Error("string ≤ go.bytestring is the declared edge")
	}
	if tg.Subsumes("go.bytestring", "string") {
		t.Error("go.bytestring ≤ string would admit host bytes as ours")
	}
}
