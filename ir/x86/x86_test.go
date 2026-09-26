package x86

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// lower runs gen's front half on a source and returns every export's IR_P,
// as `gen -printer x86` would print it.
func lower(t *testing.T, src string) (*emit.Target, []*ir.Func) {
	t.Helper()
	src = filepath.Join("..", "..", src)
	layers, err := emit.SearchPath(src, filepath.Join("..", "..", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "..", "lib")}
	tg, err := emit.LoadTargetLayers("windows", layers, dirs)
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(string(text))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.LoadWithDefs(forms, func(path string) (string, bool, error) {
		for _, d := range dirs {
			if b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(path)+".oro")); err == nil {
				return string(b), true, nil
			}
		}
		return "", false, nil
	}, tg.Defs)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	var fs []*ir.Func
	for _, q := range exports {
		name := q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf); err != nil {
			t.Fatal(err)
		}
		f, err := ir.Lower(tg, name, prog.Sigs[q], nf, ir.Options{Decided: true})
		if err != nil {
			t.Fatal(err)
		}
		p := &ir.Program{Target: tg.Name, Funcs: []*ir.Func{f}}
		if err := ir.Finalize(tg, p); err != nil {
			t.Fatal(err)
		}
		fs = append(fs, f)
	}
	return tg, fs
}

// printed prints every export of a source, and keeps each printer.
func printed(t *testing.T, src string) (string, []*printer) {
	t.Helper()
	tg, fs := lower(t, src)
	var out strings.Builder
	var ps []*printer
	for _, f := range fs {
		p := newPrinter(tg, f)
		code, err := p.function()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		out.WriteString(code)
		ps = append(ps, p)
	}
	return out.String(), ps
}

// TestTheSievesLoops: the measured shape of spec §9.6. A back edge's `i + 1`
// is `add` in place, as the term backend and hand-written assembly print it,
// and no instruction moves a register onto itself.
func TestTheSievesLoops(t *testing.T) {
	code, _ := printed(t, "examples/native/sieve-win.oro")
	three := regexp.MustCompile(`mov (\w+), (\w+)\n\s+add (\w+), 1\n\s+mov (\w+), (\w+)\n`)
	for _, m := range three.FindAllStringSubmatch(code, -1) {
		if m[1] == m[3] && m[4] == m[2] && m[5] == m[1] {
			t.Errorf("a back edge copies instead of adding in place:\n%s", m[0])
		}
	}
	if !regexp.MustCompile(`\n\s+add (rbx|rsi|rdi|r1[2-5]), 1\n\s+jmp Ltop`).MatchString(code) {
		t.Errorf("no in-place increment on a back edge:\n%s", code)
	}
	for _, l := range strings.Split(code, "\n") {
		if selfMove(strings.TrimSpace(l)) {
			t.Errorf("a self-move: %s", l)
		}
	}
}

// TestPrintingIsAFunction: print twice, compare (§9.5).
func TestPrintingIsAFunction(t *testing.T) {
	for _, src := range []string{"examples/native/sieve-win.oro", "ir/testdata/x86-shapes.oro"} {
		a, _ := printed(t, src)
		b, _ := printed(t, src)
		// Labels are numbered by a counter shared across procedures; strip it.
		num := regexp.MustCompile(`L([a-z]+)\d+`)
		if num.ReplaceAllString(a, "L$1") != num.ReplaceAllString(b, "L$1") {
			t.Errorf("%s prints differently twice", src)
		}
	}
}

// TestTheAllocationIsAColouring checks scan against the live sets it was
// given, independently of how it chose: no two classes whose live sets meet
// share a register, except an in-place result and its operand meeting only
// at the result's definition (§9.6's exception).
func TestTheAllocationIsAColouring(t *testing.T) {
	for _, src := range []string{"examples/native/sieve-win.oro", "ir/testdata/x86-shapes.oro"} {
		_, ps := printed(t, src)
		for _, p := range ps {
			if bad := colouringFault(p); bad != "" {
				t.Errorf("%s %s: %s", src, p.f.Name, bad)
			}
		}
	}
}

func colouringFault(p *printer) string {
	for i, a := range p.lives {
		for _, b := range p.lives[i+1:] {
			ra, rb := p.loc[a.cls].reg, p.loc[b.cls].reg
			if ra == "" || ra != rb || !a.meets(b) {
				continue
			}
			first, second := a, b
			if b.start < a.start {
				first, second = b, a
			}
			if second.inPlace == first.cls && first.meetsOnlyAt(second, second.start) {
				continue
			}
			return "classes " + itoa(int(a.cls)) + " and " + itoa(int(b.cls)) + " meet and share " + ra
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestThePrintersOwnReadsCount: the reads the printer adds are live. A build's
// size is read again after the allocator returns, so it never shares the
// buffer's register (map-len failed on exactly that), and a Rule R loop's
// parameter is read by the swap at its continue, so the new buffer never takes
// its register (big-limbs).
func TestThePrintersOwnReadsCount(t *testing.T) {
	_, ps := printed(t, "ir/testdata/x86-shapes.oro")
	builds, spares := 0, 0
	for _, p := range ps {
		p.f.Walk(func(r *ir.Region) {
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch s.Op {
				case ir.OBuild:
					n, buf := p.cls(s.Args[0]), p.cls(s.Sub[0].Params[0])
					if p.isConst(n) || p.spareFor[s] == s.Sub[0].Params[0] {
						continue
					}
					builds++
					if rn := p.loc[n].reg; rn != "" && rn == p.loc[buf].reg {
						t.Errorf("%s: a build's size and its buffer share %s", p.f.Name, rn)
					}
				case ir.OLoop:
					for _, sp := range p.spareOf[s] {
						spares++
						q := p.cls(s.Sub[0].Params[p.spareParam[p.cls(sp)]])
						for _, c := range exitsOf(s.Sub[0]) {
							j := p.spareParam[p.cls(sp)]
							if a := p.cls(c[j]); a != q && p.loc[a].reg != "" && p.loc[a].reg == p.loc[q].reg {
								t.Errorf("%s: the new buffer takes the parameter's register %s, which the swap reads", p.f.Name, p.loc[a].reg)
							}
						}
					}
				}
			}
		})
	}
	if builds == 0 || spares == 0 {
		t.Fatalf("anti-vacuity: %d computed-size builds and %d spares; the test checks nothing", builds, spares)
	}
}

func exitsOf(r *ir.Region) [][]ir.V {
	var out [][]ir.V
	var walk func(*ir.Region)
	walk = func(r *ir.Region) {
		switch r.T {
		case ir.TContinue:
			out = append(out, r.Args)
		case ir.TBranch:
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}
