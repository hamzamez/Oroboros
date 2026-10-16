package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
)

// reduceGo loads and reduces a program's export on the Go target, returning
// the reducer's refusal rather than failing on it.
func reduceGo(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.Normalize(prog.Defs[prog.Exports[0]], env, core.DefaultFuel)
	return err
}

// A PARTIAL SUCCESS'S ERROR IS RELEVANT (spec/errors.md §4.3): the error factor
// of ℕ × (1 + E) is `(result (tuple) error)`, so a program that drops it is
// refused as one that drops `os.WriteFile`'s result is (§7). Where the host's
// algebra makes the error ignorable, the declaration keeps `(option error)`:
// strings.Builder's writes, whose error is always nil, and bufio.Writer's,
// whose errors are sticky and reported by Flush.
func TestAPartialSuccesssErrorIsRelevant(t *testing.T) {
	const head = `use go
use result
use go/os/File as File
use go/strings/Builder as sb
use go/bufio/Writer as bw
use go/strconv as sc
export `
	for _, c := range []struct {
		body string
		ok   bool
	}{
		// a File write's error, dropped, read, and ignored
		{"fn f(h ∈ `go/os`.File, b ∈ Array(0..255)) (Int) {\n    (n, e) = File.Write(h, b)\n    n\n}", false},
		{"fn f(h ∈ `go/os`.File, b ∈ Array(0..255)) (Int) {\n    (n, e) = File.Write(h, b)\n    case e { result.'ok(u) -> n; result.'err(x) -> 0 }\n}", true},
		{"fn f(h ∈ `go/os`.File, b ∈ Array(0..255)) (Int) {\n    ignore(File.Write(h, b))\n    0\n}", true},
		// a parse: pure, and the error comes in the tuple pattern
		{`fn f(s ∈ go.bytestring) (Int) {
    (v, e) = sc.Atoi(s)
    v
}`, false},
		{`fn f(s ∈ go.bytestring) (Int) {
    (v, e) = sc.Atoi(s)
    case e { result.'ok(u) -> v; result.'err(x) -> 0 }
}`, true},
		// the ignorable ones
		{"fn f(w ∈ `go/strings`.Builder, b ∈ Array(0..255)) (Int) {\n    (n, e) = sb.Write(w, b)\n    n\n}", true},
		{"fn f(w ∈ `go/bufio`.Writer, b ∈ Array(0..255)) (Int) {\n    (n, e) = bw.Write(w, b)\n    n\n}", true},
	} {
		err := reduceGo(t, head+c.body)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused: %v", c.body, err)
		case !c.ok && err == nil:
			t.Errorf("%s: accepted, and it drops a partial success's error", c.body)
		case !c.ok && !strings.Contains(err.Error(), "relevant type"):
			t.Errorf("%s: refused for another reason: %v", c.body, err)
		}
	}
}
