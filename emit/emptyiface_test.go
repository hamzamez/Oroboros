package emit_test

import (
	"strings"
	"testing"
)

// THE EMPTY INTERFACE (spec/types.md §3.3): Go's `any`, `go.Value`, is ∃X. X,
// and an interface with no methods is satisfied by every type vacuously. So a
// type held as ONE host value stands where a go.Value is wanted, and Go
// inserts the box; a tuple, a live buffer and a function do not, and no other
// interface is the top.
func TestEveryOneValueTypeEntersTheEmptyInterface(t *testing.T) {
	const head = "use go\nuse go/fmt as fmt\nuse go/os as gos\nuse go/io as gio\nexport "
	for _, c := range []struct {
		body string
		ok   bool
		why  string // the refusal that must fire, when it is the box's own
	}{
		// an integer, a string, a boolean, a host type, a frozen table
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    fmt.Println(n, s, true, gos.Stdout(), xs)
    0
}`, true, ""},
		// a scope's value is the frozen table, checked at the exit
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    fmt.Println({ var b := array { 0 : _ ∈ 0..1 }; b[0] := n; b[1] := n; b })
    0
}`, true, ""},
		// none, and one
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    fmt.Println()
    fmt.Println(n)
    0
}`, true, ""},
		// a tuple is several values at a boundary
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    fmt.Println((n, n))
    0
}`, false, ""},
		// a live buffer would be aliased by the host while the program writes it
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    len({ var b := array { 0 : _ ∈ 0..3 }; fmt.Println(b); b })
}`, false, "go.Value is required here"},
		// only the declared box is the top: an integer is no io.Writer
		{`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    gio.MultiWriter(7)
    0
}`, false, "`go/io`.Writer is required here"},
	} {
		_, err := entryGo(t, head+c.body)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused: %v", c.body, err)
		case !c.ok && err == nil:
			t.Errorf("%s: accepted, and the value cannot enter the box", c.body)
		case !c.ok && c.why != "" && !strings.Contains(err.Error(), c.why):
			t.Errorf("%s: refused, but not by the box's rule (%q): %v", c.body, c.why, err)
		}
	}
	// what Go is handed: the list written at the call is Go's own call, its
	// boxes inserted by Go (spec/variadic.md §4)
	out, err := entryGo(t, head+`fn f(n ∈ 0..100, s ∈ go.bytestring, xs ∈ Array(0..255)) (Int) {
    fmt.Println(n, s, true)
    0
}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fmt.Println(v0, v1, true)") || strings.Contains(out, "[]any") {
		t.Errorf("want Go's own variadic call:\n%s", out)
	}
}
