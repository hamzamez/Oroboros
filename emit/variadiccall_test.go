package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A VARIADIC CALL IS ITS DECLARATION APPLIED TO THE WORD ITS ARGUMENTS SPELL
// (spec/variadic.md §3). f : X × T* → B, and Go's spec builds the slice from
// the arguments, so `(fmt.Println a b c)` is Println of the list a b c, and
// `(spread xs)` is Println of a list one has, Go's `xs...`.
func TestAVariadicCallIsTheWordItsArgumentsSpell(t *testing.T) {
	const head = `(use go) (use go/fmt as fmt) (use go/strings as strings) (export f)
(sig f ((n (int 0 100)) (s go.bytestring) (bs (array (int 0 255)))) int)
`
	for _, c := range []struct {
		body, want string // want: in the emitted Go, or the refusal
		ok         bool
	}{
		// none, one, and more than the old wall of three; Go's own call
		{`(def f (n s bs) (seq (fmt.Println) 0))`, "fmt.Println()", true},
		{`(def f (n s bs) (seq (fmt.Println n) 0))`, "fmt.Println(v0)", true},
		{`(def f (n s bs) (seq (fmt.Println n s n s n) 0))`, "fmt.Println(v0, v1, v0, v1, v0)", true},
		// fixed parameters before the list
		{`(def f (n s bs) (seq (fmt.Printf "%d %s\n" n s) 0))`, `fmt.Printf("%d %s\n", v0, v1)`, true},
		{`(def f (n s bs) (seq (fmt.Printf "x\n") 0))`, `fmt.Printf("x\n")`, true},
		// a table is ONE value of ...any, as Go's fmt.Println(bs) prints it
		{`(def f (n s bs) (seq (fmt.Println bs) 0))`, "fmt.Println(v2)", true},
		// spread hands the table as the list: Go's slices are not covariant,
		// so a table of bytes is no list of boxes, as Go itself refuses
		{`(def f (n s bs) (let ws (array 1 2 3) (seq (fmt.Println (spread ws)) (len ws))))`, "go.Value", false},
		// a list of one type, handed with spread
		{`(def f (n s bs) (seq (strings.NewReplacer "a" s) 0))`, "", true},
		// misuse, each refused by the loader
		{`(def f (n s bs) (seq (fmt.Println (spread bs) n) 0))`, "stands alone", false},
		{`(def f (n s bs) (seq (fmt.Printf) 0))`, "takes 1 argument(s) before its list", false},
		{`(def f (n s bs) (seq (go.text (spread s)) 0))`, "stands only as that call's argument", false},
	} {
		out, err := entryGoLoad(t, head+c.body)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused: %v", c.body, err)
		case c.ok && !strings.Contains(out, c.want):
			t.Errorf("%s: want %q in:\n%s", c.body, c.want, out)
		case !c.ok && err == nil:
			t.Errorf("%s: accepted, and should be refused (%s)", c.body, c.want)
		case !c.ok && !strings.Contains(err.Error(), c.want):
			t.Errorf("%s: want %q, got %v", c.body, c.want, err)
		}
	}
}

// The word `spread` is the language's.
func TestSpreadIsNotADefinition(t *testing.T) {
	_, err := entryGoLoad(t, `(use go) (export f) (sig f () int) (def spread (x) x) (def f () 0)`)
	if err == nil || !strings.Contains(err.Error(), "which is the language's") {
		t.Errorf("a definition named spread: got %v", err)
	}
}

// A VARIADIC STATEMENT'S VALUE IS THE UNIT: the host's results are forgotten,
// and "its value is argument 0" would make it the list. A function whose value
// is a print was refused while its list was its value (the table, fixed once as
// a list of boxes and once as the function's result).
func TestAVariadicStatementsValueIsTheUnit(t *testing.T) {
	if _, err := entryGoLoad(t, `(use go) (use go/fmt as fmt) (export f) (sig f ((n (int 0 9))) int)
(def f (n) (seq (fmt.Println n) (fmt.Println (+ n 1))))`); err != nil {
		t.Errorf("a function ending in a print: %v", err)
	}
}

// LOADED WITHOUT THE TARGET, A VARIADIC IS REFUSED, NOT MISREAD. The rewrite
// is the loader's, from the target's forms; a program loaded without them
// never had its calls read as variadic, and `(fmt.Println xs)` would hand xs
// as the list. Every environment refuses such a program.
func TestAProgramLoadedWithoutItsTargetCannotCallAVariadic(t *testing.T) {
	tg := goNative(t)
	forms, err := core.Read(`(use go) (use go/fmt as fmt) (export f) (def f (x) (fmt.Println x))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms) // no target forms
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tg.Env(prog); err == nil || !strings.Contains(err.Error(), "loaded without the target's declarations") {
		t.Errorf("want the program refused, got %v", err)
	}
	// and with them, it is read
	prog, _, err = tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tg.Env(prog); err != nil {
		t.Errorf("loaded with the target: %v", err)
	}
}

// entryGoLoad is entryGo returning a load error instead of failing on it.
func entryGoLoad(t *testing.T, src string) (string, error) {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := tg.LoadProgram(forms); err != nil {
		return "", err
	}
	return entryGo(t, src)
}

var _ = emit.DeclaredName
