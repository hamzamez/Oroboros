package llvm

import (
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// callbackIR is a lifted window procedure's text on a triple, its parameters
// the claim's sets there: `(callback "Q")` names the claim it is checked
// under, and nothing else of its C signature is written.
func callbackIR(target, wparam, lparam string) string {
	return `(ir 1
  (target ` + target + `)
  (stage P)
  (ops call yield)
  (func wp~cb (params (%0 ptr) (%1 (int 0 4294967295)) (%2 (int ` + wparam + `)) (%3 (int ` + lparam + `))) (results int) (callback "windows/winuser.WNDPROC")
    (region
      (val (%4 (int ` + lparam + `)) (call windows/winuser.DefWindowProcA %0 %1 %2 %3))
      (yield %4))))
`
}

// A CALLBACK IS PRINTED FROM ITS TEXT (ADR 0038, 0133, llvm.md §6c): the
// claim the text names, read off the target, gives the wrapper's convention
// and C types, and the address primitive is registered from the text, so a
// program reprints from IR_P alone. print ∘ read is the identity on the text.
func TestACallbackIsPrintedFromItsText(t *testing.T) {
	for _, c := range []struct {
		target, wparam, lparam string
		load                   func(*testing.T) *emit.Target
		want                   []string
		not                    string
	}{
		{"windows", "0 18446744073709551615", "-9223372036854775808 9223372036854775807", loadTarget,
			[]string{`define internal i64 @"wp~cb~c"(ptr %a0, i32 %a1, i64 %a2, i64 %a3)`, "%b1 = zext i32 %a1 to i64", "ret i64 %r"}, "x86_stdcallcc"},
		{"windows32", "0 4294967295", "-2147483648 2147483647", loadTarget32,
			[]string{`define internal x86_stdcallcc i32 @"wp~cb~c"(ptr %a0, i32 %a1, i32 %a2, i32 %a3)`, "%b2 = zext i32 %a2", "%b3 = sext i32 %a3", "%c = trunc i64 %r to i32"}, ""},
	} {
		text := callbackIR(c.target, c.wparam, c.lparam)
		p, err := ir.Read(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := ir.Print(p); got != text {
			t.Fatalf("%s: print ∘ read ≠ id:\n%s", c.target, got)
		}
		if p.Funcs[0].Callback != "windows/winuser.WNDPROC" {
			t.Fatalf("%s: the claim was not read: %q", c.target, p.Funcs[0].Callback)
		}
		tg := c.load(t)
		ir.RegisterProgram(tg, p)
		if _, ok := tg.Prims["wp~cb~addr"]; !ok {
			t.Fatalf("%s: the address primitive is not registered from the text", c.target)
		}
		out, err := Func(tg, p.Funcs[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: want %q in:\n%s", c.target, w, out)
			}
		}
		if c.not != "" && strings.Contains(out, c.not) {
			t.Errorf("%s: %q is a convention only on i686:\n%s", c.target, c.not, out)
		}
	}
}
