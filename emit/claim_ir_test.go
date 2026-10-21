package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A BINDER'S CLAIM IS AN OBLIGATION (ADR 0069 item 4): `b ∈ S = e` is proven of e
// or the program is refused, for the reason the claim gives. Trusted, both of the
// false ones compiled (ownbuf-2026-10-21).
func TestABindersClaimIsProvenOrRefused(t *testing.T) {
	const head = "use go\nuse go/os as gos\nuse go/fmt\nexport fn main() {\n    av = gos.Args()\n"
	for what, c := range map[string]struct{ body, want string }{
		"false, a literal":    {"    x ∈ 0..5 = 10\n    fmt.Println(x)\n}\n", "the claim x ∈ 0..5 is false: its value is 10"},
		"not proven":          {"    x ∈ 0..5 = len(av) + 10\n    fmt.Println(x)\n}\n", "the claim x ∈ 0..5 is not proven"},
		"a constant's set":    {"    x ∈ 0..lim = len(av) + 10\n    fmt.Println(x)\n}\nlim = 5\n", "the claim x ∈ 0..5 is not proven"},
		"proven by its guard": {"    if len(av) < 4 { x ∈ 0..5 = len(av) + 1; fmt.Println(x) }\n}\n", ""},
	} {
		err := refusedGo(t, head+c.body)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", what, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", what, err, c.want)
		}
	}
}

// THE CORE'S ASCRIPTION IS NOT SURFACE (ADR 0062): written as a call it was a
// trusted claim.
func TestTheIsNotSurface(t *testing.T) {
	_, err := core.ReadEntry("use go\nuse go/fmt\nexport fn main() {\n    x = the(\"int 0 5\", 10)\n    fmt.Println(x)\n}\n", "t.oro")
	if err == nil || !strings.Contains(err.Error(), "`the` is the core's ascription, not J's") {
		t.Errorf("the(…) in J: %v", err)
	}
}

// A HOST CALL'S `ensure` NAMES A COMPONENT OF ITS TUPLE RESULT (ADR 0069 item 5):
// io.Reader's law, n ≤ len q, on os.File.Read, proves the last byte read is an
// index of the buffer. Without the law the index is unprovable.
func TestAReadsCountIsAtMostItsBuffer(t *testing.T) {
	out := printsGo(t, `use go
use go/os as gos
use go/os/File
use go/fmt
use result
fn done?(r) { case r { result.'ok(u) -> false; result.'err(e) -> { ignore(e); true } } }
export fn main() {
    var b := array { 0 : _ ∈ 0..4095 }
    (n, r) = File.Read(gos.Stdin(), b)
    ignore(done?(r))
    fmt.Println(if n > 0 { b[n - 1] } else { 0 })
}
`)
	if !strings.Contains(out, ".Read(") {
		t.Errorf("no read:\n%s", out)
	}
}
