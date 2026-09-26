package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/ir/golang"
)

// A RANGE RESULT IS RECEIVED INTO THE LANGUAGE'S INTEGER (target-files.md §3,
// "A result RANGE"). A host call gives back its own width — `utf8.DecodeRune`
// a `rune` — and until gostd-utf8-2026-09-13 the Go emitter bound it as that
// width, so `r, n := utf8.DecodeRuneInString(s)` followed by `r + n` was
// `int32 + int`, which Go refuses. Printing `r` alone compiled, which is why a
// program that only printed results could not have found it.

func TestSeveralRangeResultsAreReceivedAsInt(t *testing.T) {
	tg := receiveTarget(t)
	// (fn (s) ((x.decode s) (fn (r n) (go.+ r n))))
	term := core.Fn([]string{"s"},
		core.App(core.App(core.Name("x.decode"), core.Name("s")),
			core.Fn([]string{"r", "n"}, core.App(core.Name("go.+"), core.Name("r"), core.Name("n")))))
	got, err := golang.FromResidual(tg, "f", strSig("s"), term)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, ":= int(") {
		t.Errorf("the rune result was not received as int; `r + n` is int32 + int in Go:\n%s", got)
	}
	if strings.Contains(got, "n := int(") || strings.Contains(got, "nh") {
		t.Errorf("an `int` result was converted too; only a RANGE is received through a temporary:\n%s", got)
	}
}

func TestASingleRangeResultIsReceivedAsInt(t *testing.T) {
	tg := receiveTarget(t)
	// (fn (x) (go.+ (x.rev8 x) 1))
	term := core.Fn([]string{"x"},
		core.App(core.Name("go.+"), core.App(core.Name("x.rev8"), core.Name("x")), core.Int(1)))
	got, err := golang.FromResidual(tg, "f", &core.Sig{Params: []core.SigParam{{Name: "x", Type: "int 0 255"}}, Result: "int"}, term)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "int(bits.Reverse8(") {
		t.Errorf("a uint8 result was not received as int:\n%s", got)
	}
	// THE CONTROL: a result declared plain `int` is the host's integer already
	// and must be emitted exactly as before, or every existing program moves.
	plain := core.Fn([]string{"s"},
		core.App(core.Name("go.+"), core.App(core.Name("x.count"), core.Name("s")), core.Int(1)))
	got, err = golang.FromResidual(tg, "g", strSig("s"), plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "int(utf8.RuneCountInString(") {
		t.Errorf("a plain int result was wrapped:\n%s", got)
	}
}
