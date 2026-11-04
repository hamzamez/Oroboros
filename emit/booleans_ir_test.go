package emit_test

import (
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir/golang"
)

// `and`, `or` and `not` are reader sugar over `if` (ADR 0017), and each backend
// must put the HOST's operator back. Emitting the conditional would be lowering
// further than the target requires — the objection that kept them primitives in
// ADR 0012, and the reason the recogniser exists.
func TestConnectiveEmitsHostOperator(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"fn t(a, b) { go.`<`(a, b) && go.`<`(b, 9) }", "&&"},
		{"fn t(a, b) { go.`<`(a, b) || go.`<`(b, 9) }", "||"},
		{"fn t(a, b) { !go.`<`(a, b) }", "!"},
	} {
		nf := reduce(t, `use go
`+c.src, "go")
		tg, err := emit.LoadTarget("../targets/go")
		if err != nil {
			t.Fatal(err)
		}
		code, err := golang.FromResidual(tg, "t", nil, nf)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if !strings.Contains(code, c.want) {
			t.Errorf("%s did not emit %q:\n%s", c.src, c.want, code)
		}
		if strings.Contains(code, "if ") {
			t.Errorf("%s lowered to a conditional:\n%s", c.src, code)
		}
	}
}
