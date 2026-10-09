package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A HOST WRITING A CELL (spec/local.md §5, §6), on Go's Scan family. Inside the
// call a cell is a reference to a copy, and after it the cell holds what the
// copy holds: the Go a programmer writes, a variable, its address, the call,
// and the read after it.
func TestAHostWritesACellThroughAReference(t *testing.T) {
	const head = `(use go) (use go/fmt as fmt) (use result) (export f) (sig f ((s go.bytestring)) int)
(def ok? (r) (case r (result.ok u) true (result.err e) false))
`
	call := regexp.MustCompile(`fmt\.Sscan\(v0, ([^)]*)\)`)
	for _, c := range []struct {
		body  string
		want  []string // in order, in the emitted Go
		types []string // the copies' declarations
		read  int      // how many of the references the program reads back
	}{
		// (out τ): a cell for the one call, holding τ's zero; its value, then
		// the result's components
		{`(def f (s) ((fmt.Sscan s (out int) (out go.bytestring)) (fn (age name n e) (if (and (ok? e) (> age 0)) n 0))))`,
			[]string{" = 0", ` = ""`, "fmt.Sscan(v0, "}, []string{"c int", "c string"}, 1},
		// cells in scope, passed where the host stores: a cell's own write
		// before the call is what the copy starts as, and a string cell's copy
		// is a string, whatever cell stands beside it in the list
		{`(def f (s) (local x 7 w "" (seq (set x 9) ((fmt.Sscan s x w) (fn (n e) (seq (ignore w) (if (ok? e) x n)))))))`,
			[]string{" = 9", "fmt.Sscan(v0, "}, []string{"c int", "c string"}, 1},
	} {
		got, err := entryGo(t, head+c.body)
		if err != nil {
			t.Errorf("%s: %v", c.body, err)
			continue
		}
		at := 0
		for _, w := range c.want {
			i := strings.Index(got[at:], w)
			if i < 0 {
				t.Errorf("%s: %q not found after offset %d in:\n%s", c.body, w, at, got)
				break
			}
			at += i + len(w)
		}
		for _, ty := range c.types {
			if !strings.Contains(got, ty) {
				t.Errorf("%s: no copy declared %q in:\n%s", c.body, ty, got)
			}
		}
		// THE COPY IS DECLARED ONCE, at the function's top, and assigned at the
		// call: declared where it is used, Go moved a fresh copy to the heap on
		// every iteration of a loop, one allocation per call more than
		// hand-written Go (outcells-2026-10-09).
		if regexp.MustCompile(`var v[0-9]+c (\S+ )?=`).MatchString(got) {
			t.Errorf("%s: a copy is declared where it is used:\n%s", c.body, got)
		}
		// THE REFERENCE PASSED IS THE ONE READ: a duplicated reference would
		// pass one copy and read another, which the host never wrote.
		m := call.FindStringSubmatch(got)
		if m == nil {
			t.Errorf("%s: no call in:\n%s", c.body, got)
			continue
		}
		after := got[strings.Index(got, m[0]):]
		for _, r := range strings.Split(m[1], ", ")[:c.read] {
			if !strings.Contains(after, "*"+r) {
				t.Errorf("%s: %s is passed and never read back after the call:\n%s", c.body, r, got)
			}
		}
	}
}

// A CELL PARAMETER TAKES A CELL. A value there is refused: the host would write
// through the address of a copy nobody reads again. So is a λ that does not
// take the cells' values and then the result's components.
func TestACellParameterTakesACell(t *testing.T) {
	const head = `(use go) (use go/fmt as fmt) (use result) (export f) (sig f ((s go.bytestring)) int)
`
	for _, c := range []struct{ body, refused string }{
		{`(def f (s) ((fmt.Sscan s 5) (fn (n e) (seq (ignore e) n))))`, "takes a cell in scope or (out τ)"},
		{`(def f (s) (let y 5 ((fmt.Sscan s y) (fn (n e) (seq (ignore e) n)))))`, "takes a cell in scope or (out τ)"},
		{`(def f (s) ((fmt.Sscan s (out int)) (fn (n e) (seq (ignore e) n))))`, "gives 1 cell value(s) and then its declared result's 2"},
	} {
		_, err := entryGoLoad(t, head+c.body)
		if err == nil {
			_, err = entryGo(t, head+c.body)
		}
		if err == nil || !strings.Contains(err.Error(), c.refused) {
			t.Errorf("%s\n  want a refusal naming %q, got %v", c.body, c.refused, err)
		}
	}
}

// WHAT THE HOST WROTE IS UNKNOWN: the value read back has its type's whole
// range, so arithmetic on a scanned integer is refused until a guard bounds it.
func TestWhatTheHostWroteIsUnbounded(t *testing.T) {
	src := `(use go) (use go/fmt as fmt) (use result) (export f) (sig f ((s go.bytestring)) int)
(def f (s) ((fmt.Sscan s (out int)) (fn (v n e) (seq (ignore e) (+ v 1)))))`
	if err := entryLegal(t, src); err == nil || !strings.Contains(err.Error(), "cannot be proven") {
		t.Errorf("(+ v 1) on a scanned v was accepted: %v", err)
	}
	// the control: a guard bounds it
	guarded := strings.Replace(src, "(+ v 1)", "(if (and (>= v 0) (<= v 100)) (+ v 1) 0)", 1)
	if err := entryLegal(t, guarded); err != nil {
		t.Errorf("a guarded (+ v 1) was refused: %v", err)
	}
}

// entryLegal is entryGo's pipeline with the drivers' refusal of an operation
// not proven inside the word, which entryGo leaves to its caller.
func entryLegal(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		return err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return err
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		return err
	}
	nf = emit.EtaTails(tg, nf)
	_, leg, err := ir.Entry(tg, q, q, prog.Sigs[q], nf, allProgSigs(prog), false, nil)
	if err != nil {
		return err
	}
	return leg.Refusal(q, tg)
}

// A CELL THE HOST WRITES IN A LOOP is carried by the loop like any other, and a
// loop variable's type is the join of its initial value and its back edges'
// values: a cell started with "" and written a host string is a host string.
// A variadic call with a cell among its arguments, inside a scope, is still
// read as the list.
func TestACellTheHostWritesIsCarriedByALoop(t *testing.T) {
	src := `(use go) (use go/fmt as fmt) (use result) (export f)
(sig f ((xs (array go.bytestring))) (int 0 100))
(def f (xs)
  (local last 0 word ""
    (loop ((i 0))
      (>= i (len xs)) (seq (fmt.Println word last) (if (and (>= last 0) (<= last 100)) last 0))
      (>= i 100)      0
      else (let (tuple w n e) (fmt.Sscan (xs i) (out go.bytestring) last)
             (seq (ignore e) (set word w) (again (+ i 1)))))))`
	got, err := entryGo(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "fmt.Sscan(") || !strings.Contains(got, "fmt.Println(") || !strings.Contains(got, "for ") {
		t.Errorf("the loop or a call is missing:\n%s", got)
	}
}

// A PROGRAM LOADED WITHOUT THE TARGET'S cells FORMS IS REFUSED: there a cell
// passed to Sscan was read as its value, and Go's write went into a copy.
func TestAProgramLoadedWithoutCellFormsIsRefused(t *testing.T) {
	tg := goNative(t)
	forms, err := core.Read(`(use go) (use go/fmt as fmt) (export f) (sig f ((s go.bytestring)) int)
(def f (s) ((fmt.Sscan s 1) (fn (n e) n)))`)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := core.Load(forms)
	if err != nil {
		t.Fatalf("the program does not load without its target: %v", err)
	}
	if _, err := tg.Env(p); err == nil || !strings.Contains(err.Error(), "takes cells the host writes") {
		t.Errorf("a program loaded without the cells forms was accepted: %v", err)
	}
}
