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
	const head = `(use go) (use go/fmt as fmt) (use go/os as gos) (use go/io as gio) (export f)
(sig f ((n (int 0 100)) (s go.bytestring) (xs (array (int 0 255)))) int)
`
	for _, c := range []struct {
		body string
		ok   bool
		why  string // the refusal that must fire, when it is the box's own
	}{
		// an integer, a string, a boolean, a host type, a frozen table
		{`(def f (n s xs) (seq (fmt.Println n s true (gos.Stdout) xs) 0))`, true, ""},
		// a scope's value is the frozen table, checked at the exit
		{`(def f (n s xs) (seq (fmt.Println (local b (table 2 0) (set (set b 0 n) 1 n))) 0))`, true, ""},
		// none, and one
		{`(def f (n s xs) (seq (fmt.Println) (fmt.Println n) 0))`, true, ""},
		// a tuple is several values at a boundary
		{`(def f (n s xs) (seq (fmt.Println (tuple n n)) 0))`, false, ""},
		// a live buffer would be aliased by the host while the program writes it
		{`(def f (n s xs) (len (local b (table 4 0) (seq (fmt.Println b) b))))`, false, "go.Value is required here"},
		// only the declared box is the top: an integer is no io.Writer
		{`(def f (n s xs) (seq (gio.MultiWriter 7) 0))`, false, "go/io.Writer is required here"},
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
	out, err := entryGo(t, head+`(def f (n s xs) (seq (fmt.Println n s true) 0))`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fmt.Println(v0, v1, true)") || strings.Contains(out, "[]any") {
		t.Errorf("want Go's own variadic call:\n%s", out)
	}
}
