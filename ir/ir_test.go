package ir

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// compile runs the front half of cmd/gen on a source file and lowers every
// export: exactly what gen hands a backend, minus the legality step, so modes
// stay undecided (IR_A).
func compile(t *testing.T, src, target string) (*emit.Target, *Program) {
	t.Helper()
	return compileOpt(t, src, target, Options{})
}

// compileOpt lowers with the given options: Decided, as gen does after the
// legality check, gives IR_A with every mode written, ready for Finalize.
func compileOpt(t *testing.T, src, target string, opt Options) (*emit.Target, *Program) {
	t.Helper()
	tg, p, err := compilePath(filepath.Join("..", src), target, opt)
	if err != nil {
		t.Fatal(err)
	}
	return tg, p
}

// compilePath is compileOpt on a path as given, reporting an error instead of
// failing: a generated program the front end refuses is skipped, and counted.
func compilePath(src, target string, opt Options) (*emit.Target, *Program, error) {
	layers, err := emit.SearchPath(src, filepath.Join("..", "targets"))
	if err != nil {
		return nil, nil, err
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "lib")}
	tg, err := emit.LoadTargetLayers(target, layers, dirs)
	if err != nil {
		return nil, nil, err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return nil, nil, err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return nil, nil, err
	}
	prog, _, err := core.LoadWithDefs(forms, resolver(dirs), tg.Defs)
	if err != nil {
		return nil, nil, err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return nil, nil, err
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	p := &Program{Target: target}
	for _, q := range exports {
		name := "t-" + q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			return nil, nil, err
		}
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf); err != nil {
			return nil, nil, err
		}
		sig := prog.Sigs[q]
		if nfl, fsig, k, err := emit.FlattenProducts(tg, sig, nf); err != nil {
			return nil, nil, err
		} else if k > 0 {
			nf, sig = nfl, fsig
		}
		if nf, _, err = emit.PromoteBig(tg, sig, nf); err != nil {
			return nil, nil, err
		}
		f, err := Lower(tg, name, sig, nf, opt)
		if err != nil {
			return nil, nil, err
		}
		p.Funcs = append(p.Funcs, f)
	}
	return tg, p, nil
}

func resolver(dirs []string) core.Resolver {
	return func(path string) (string, bool, error) {
		for _, d := range dirs {
			b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(path)+".oro"))
			if err == nil {
				return string(b), true, nil
			}
			if !os.IsNotExist(err) {
				return "", false, err
			}
		}
		return "", false, nil
	}
}

// corpus is a spread of shapes: loops with π, a join point, tables built and
// read, maps read and built, host calls with several results, big integers.
var corpus = []struct{ src, target string }{
	{"examples/native/dot-go.oro", "go"},
	{"examples/native/generic-go.oro", "go"},
	{"examples/native/smooth-go.oro", "go"},
	{"examples/json/tokenize.oro", "go"},
	{"examples/json/tree.oro", "go"},
	{"examples/io/wc.oro", "js"},
	{"examples/big/limbs.oro", "windows"},
	{"gauntlet/differential/cases/map-dynamic.oro", "java"},
	{"gauntlet/differential/cases/map-keys.oro", "go"},
	{"gauntlet/differential/cases/prod-loop.oro", "go"},
	{"gauntlet/differential/cases/build-zero.oro", "js"},
	// Three the first full sweep refused, each pinning a fix:
	{"examples/sum/parse.oro", "go"},       // a variant at a boundary, (fn (#x) (#x tag payload)): a product by structure
	{"examples/native/dot-js.oro", "js"},   // JavaScript's `+` over `any`: ℤ's add only on integer operands
	{"examples/big/render.oro", "windows"}, // `len` of a big integer, which the limb representation has
}

// TestOverloadedOperatorsAreDecidedByTheirOperands: on JavaScript `+` is
// declared over `any`, so the float accumulation in dot stays a call and the
// index increment is promoted to the language's add.
func TestOverloadedOperatorsAreDecidedByTheirOperands(t *testing.T) {
	_, p := compile(t, "examples/native/dot-js.oro", "js")
	var calls, adds int
	for _, f := range p.Funcs {
		f.Walk(func(r *Region) {
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch {
				case s.Op == OAdd:
					adds++
					for _, a := range s.Args {
						if f.Types[a] != "int" {
							t.Errorf("an add on %s", f.Types[a])
						}
					}
				case s.Op == OCall && emitArith(s.Name):
					calls++
				}
			}
		})
	}
	if adds == 0 || calls == 0 {
		t.Fatalf("want the index's add and the accumulator's call, got %d adds and %d calls\n%s", adds, calls, Print(p))
	}
}

func emitArith(name string) bool {
	return emit.ArithOp(name, 2) == "add" || emit.ArithOp(name, 2) == "mul"
}

// TestCorpusLowersAndVerifies is spec §11's first two rows on a sample; the
// cmd/check `ir` step is the same on every program the sweep emits.
func TestCorpusLowersAndVerifies(t *testing.T) {
	for _, c := range corpus {
		t.Run(filepath.Base(c.src)+"/"+c.target, func(t *testing.T) {
			tg, p := compile(t, c.src, c.target)
			if err := Verify(tg, p); err != nil {
				t.Fatalf("verify:\n%v\n%s", err, Print(p))
			}
			// `any` is the faithful type where the host has no static types and
			// the program's signature says so (JavaScript, windows); on a typed
			// host it would be a value no equation reached.
			if c.target == "go" || c.target == "java" {
				for _, f := range p.Funcs {
					for v, ty := range f.Types {
						if ty == "any" {
							t.Errorf("%s: %%%d is left untyped", f.Name, v)
						}
					}
				}
			}
		})
	}
}

// TestRoundTrip: print ∘ read ∘ print = print, and what is read verifies.
func TestRoundTrip(t *testing.T) {
	for _, c := range corpus {
		t.Run(filepath.Base(c.src)+"/"+c.target, func(t *testing.T) {
			tg, p := compile(t, c.src, c.target)
			text := Print(p)
			q, err := Read(text)
			if err != nil {
				t.Fatalf("the reader refuses the printer's text: %v\n%s", err, text)
			}
			if again := Print(q); again != text {
				t.Fatalf("print ∘ read ∘ print ≠ print:\n--- first\n%s\n--- second\n%s", text, again)
			}
			if err := Verify(tg, q); err != nil {
				t.Fatalf("the program read back does not verify: %v", err)
			}
		})
	}
}

// TestLoweringIsAFunction: lowering and printing twice give one text (the
// emitter's rule, CLAUDE.md: "test it by running twice").
func TestLoweringIsAFunction(t *testing.T) {
	_, p := compile(t, "examples/json/tree.oro", "go")
	_, q := compile(t, "examples/json/tree.oro", "go")
	if Print(p) != Print(q) {
		t.Fatal("two lowerings of one residual print differently")
	}
}

func TestTypeSpellingsAreInverse(t *testing.T) {
	for _, ty := range []string{
		"int", "f64", "bool", "string", "go.bytestring", "slice-float64", "u64",
		"int 0 255", "int -9223372036854775808 9223372036854775807",
		"int 0 340282366920938463463374607431768211455",
		"array f64", "array int 0 1023", "buffer int -5 5",
		"map int int", "map int array f64", "map int map int string",
	} {
		ts, err := core.ReadRaw(TypeText(ty))
		if err != nil || len(ts) != 1 {
			t.Fatalf("%q: %s does not read: %v", ty, TypeText(ty), err)
		}
		back, err := typeOf(ts[0])
		if err != nil || back != ty {
			t.Errorf("%q → %s → %q (%v)", ty, TypeText(ty), back, err)
		}
	}
}

// TestDotIsTheSpecExample checks the parts of spec §10.3 that lowering decides:
// the loop's shape, its π-parameters, and the classification of go.>= and go.+
// as the language's operations and of go.f* as a call.
func TestDotIsTheSpecExample(t *testing.T) {
	_, p := compile(t, "examples/native/dot-go.oro", "go")
	text := Print(p)
	for _, want := range []string{
		"(do (assume %5))", // the export's `where`, assumed at entry (ADR 0028)
		"(loop (init %14 %15)",
		"(val (%20 bool) (ge %18 %19))",
		"(pi %23 int (%18 lt %19))",
		"(pi %24 slice-float64 ((len %0) gt %18))",
		"(val (%25 f64) (index %24 %23))",
		"(call go.f* %25 %26)",
		"(add %23 %29)",
		"(continue %28 %30)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in\n%s", want, text)
		}
	}
}

// ---------------------------------------------------------------- planted faults

// plant lowers dot, applies a fault, and reports what the verifier says. The
// unmutated program is checked first: a fault's error must come from the fault.
func plant(t *testing.T, src string, fault func(p *Program)) string {
	t.Helper()
	tg, p := compile(t, src, "go")
	if err := Verify(tg, p); err != nil {
		t.Fatalf("the program verifies before the fault is planted: %v", err)
	}
	fault(p)
	err := Verify(tg, p)
	if err == nil {
		t.Fatal("the planted fault was not found")
	}
	return err.Error()
}

// find returns the first statement with op o in f, and the region holding it.
func find(f *Func, o Op) (*Region, *Stmt) {
	var rr *Region
	var ss *Stmt
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if ss == nil && r.Stmts[i].Op == o {
				rr, ss = r, &r.Stmts[i]
			}
		}
	})
	return rr, ss
}

func expect(t *testing.T, got, rule string) {
	t.Helper()
	if !strings.Contains(got, rule+":") {
		t.Errorf("expected %s, got:\n%s", rule, got)
	}
}

func TestPlantedFaults(t *testing.T) {
	const dot = "examples/native/dot-go.oro"
	t.Run("W1 a value defined twice", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			_, s := find(p.Funcs[0], OIndex)
			s.Res[0] = p.Funcs[0].Params[0]
		}), "W1")
	})
	t.Run("W2 a value used outside its scope", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			f := p.Funcs[0]
			_, idx := find(f, OIndex)
			_, loop := find(f, OLoop)
			// the loop's result is the index read inside its body
			f.Body.Args = []V{idx.Res[0]}
			_ = loop
		}), "W2")
	})
	t.Run("W3 a break outside a loop", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			p.Funcs[0].Body.T = TBreak
		}), "W3")
	})
	t.Run("W4 an operation with the wrong arity", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			_, s := find(p.Funcs[0], OIndex)
			s.Args = s.Args[:1]
		}), "W4")
	})
	t.Run("W4 a continue with the wrong arity", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			f := p.Funcs[0]
			f.Walk(func(r *Region) {
				if r.T == TContinue {
					r.Args = r.Args[:1]
				}
			})
		}), "W4")
	})
	t.Run("W5 a branch on a float", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			f := p.Funcs[0]
			_, loop := find(f, OLoop)
			loop.Sub[0].Cond = loop.Sub[0].Params[0] // the f64 accumulator
		}), "W5")
	})
	t.Run("W5 an index into a float", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			f := p.Funcs[0]
			_, loop := find(f, OLoop)
			_, idx := find(f, OIndex)
			idx.Args[1] = loop.Sub[0].Params[0]
		}), "W5")
	})
	t.Run("W6 a length π on an integer", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			f := p.Funcs[0]
			f.Walk(func(r *Region) {
				for i := range r.Pis {
					if r.Pis[i].Len {
						_, loop := find(f, OLoop)
						r.Pis[i].Of = loop.Sub[0].Params[1] // the int index
					}
				}
			})
		}), "W6")
	})
	t.Run("W9 an undecided operation in IR_P", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) { p.Stage = StageP }), "W9")
	})
	t.Run("W10 a call to no primitive", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			_, s := find(p.Funcs[0], OCall)
			s.Name = "go.no-such-primitive"
		}), "W10")
	})
}

// TestPlantedLinearityFaults: W7 on hand-written IR, which also exercises the
// reader. Each program is first shown to verify without its fault.
func TestPlantedLinearityFaults(t *testing.T) {
	tg, _ := compile(t, "examples/native/dot-go.oro", "go")
	const good = `(ir 1 (target go) (stage A)
  (ops const set build loop yield break continue branch ge add)
  (func f (params (%0 int)) (results (array int))
    (region
      (val (%1 int) (const 0))
      (val (%2 (array int))
        (build %0
          (region (params (%3 (buffer int)))
            (val (%4 (buffer int))
              (loop (init %3 %1)
                (region (params (%5 (buffer int)) (%6 int))
                  (val (%7 bool) (ge %6 %0))
                  (branch %7
                    (region (break %5))
                    (region
                      (val (%8 (buffer int)) (set %5 %6 %6))
                      (val (%9 int) (const 1))
                      (val (%10 int) (add %6 %9))
                      (continue %8 %10))))))
            (yield %4))))
      (yield %2))))`
	check := func(src string) error {
		p, err := Read(src)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return Verify(tg, p)
	}
	if err := check(good); err != nil {
		t.Fatalf("the unplanted program does not verify: %v", err)
	}
	for _, c := range []struct{ name, from, to, rule string }{
		{"a store into a buffer already consumed", "(set %5 %6 %6)", "(set %3 %6 %6)", "W7"},
		{"a buffer consumed twice on one path", "(continue %8 %10)", "(continue %5 %10)", "W7"},
		{"a use outside its scope: the build's result inside its own scope", "(val (%8 (buffer int)) (set %5 %6 %6))", "(val (%8 (buffer int)) (set %2 %6 %6))", "W2"},
		{"a store into a frozen table (ADR 0031)", "      (yield %2))))", "      (val (%11 (array int)) (set %2 %1 %1))\n      (yield %2))))", "W7"},
		{"break from an if arm", "(region (break %5))", "(region (val (%11 int) (if %7 (region (break %6)) (region (yield %6)))) (break %5))", "W3"},
		{"an operation missing from the header", "(ops const set", "(ops set", "W10"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(good, c.from, c.to, 1)
			if src == good {
				t.Fatalf("the fault %q was not planted", c.from)
			}
			err := check(src)
			if err == nil {
				t.Fatal("the planted fault was not found")
			}
			expect(t, err.Error(), c.rule)
		})
	}
}

// TestLinearityAcrossIterations: a buffer from outside a loop may be consumed
// on a path that BREAKS (once), and not on one that CONTINUES (every
// iteration). The two programs differ only in which arm stores.
func TestLinearityAcrossIterations(t *testing.T) {
	tg, _ := compile(t, "examples/native/dot-go.oro", "go")
	prog := func(breakArm, contArm string) string {
		return `(ir 1 (target go) (stage A)
  (ops const set build loop yield break continue branch ge add)
  (func g (params (%0 int)) (results (array int))
    (region
      (val (%1 int) (const 0))
      (val (%2 (array int))
        (build %0
          (region (params (%3 (buffer int)))
            (val (%4 (buffer int))
              (loop (init %1)
                (region (params (%5 int))
                  (val (%6 bool) (ge %5 %0))
                  (branch %6
                    (region ` + breakArm + `)
                    (region ` + contArm + `)))))
            (yield %4))))
      (yield %2))))`
	}
	onBreak := prog(`(val (%7 (buffer int)) (set %3 %5 %5)) (break %7)`,
		`(val (%8 int) (const 1)) (val (%9 int) (add %5 %8)) (continue %9)`)
	onContinue := prog(`(break %3)`,
		`(val (%7 (buffer int)) (set %3 %5 %5)) (val (%8 int) (const 1)) (val (%9 int) (add %5 %8)) (continue %9)`)
	p, err := Read(onBreak)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(tg, p); err != nil {
		t.Fatalf("a store on the breaking path runs once and is linear: %v", err)
	}
	q, err := Read(onContinue)
	if err != nil {
		t.Fatal(err)
	}
	err = Verify(tg, q)
	if err == nil || !strings.Contains(err.Error(), "W7:") || !strings.Contains(err.Error(), "every iteration") {
		t.Fatalf("a store into an outer buffer on the continuing path must be W7, got %v", err)
	}
}

// TestLoweringRefusesWhatItDoesNotKnow: the IR has no opaque operation
// (spec §8.1), so a closure is a lowering error that names it.
func TestLoweringRefusesWhatItDoesNotKnow(t *testing.T) {
	tg, _ := compile(t, "examples/native/dot-go.oro", "go")
	term, err := core.ReadTerm("(fn (x) (fn (y) y))")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Lower(tg, "closure", nil, term, Options{})
	if err == nil || !strings.Contains(err.Error(), "closure") {
		t.Fatalf("want a refusal naming the closure, got %v", err)
	}
}

// TestRestrictNeedsItsPremise: L13's rewrite fires where an assumption
// discharges len X ≤ len Y (dot's `where`), and not on the spec §9.4 witness,
// where every read is proven and nothing relates the two lengths.
func TestRestrictNeedsItsPremise(t *testing.T) {
	count := func(src string) int {
		tg, p := compileOpt(t, src, "go", Options{Decided: true})
		if err := Finalize(tg, p); err != nil {
			t.Fatal(err)
		}
		if err := Verify(tg, p); err != nil {
			t.Fatalf("%s: IR_P does not verify: %v", src, err)
		}
		n := 0
		for _, f := range p.Funcs {
			f.Walk(func(r *Region) {
				for i := range r.Stmts {
					if r.Stmts[i].Op == ORestrict {
						n++
					}
				}
			})
		}
		return n
	}
	if n := count("examples/native/dot-go.oro"); n != 1 {
		t.Errorf("dot: want 1 restriction (its `where` assumes len p = len q), got %d", n)
	}
	if n := count("ir/testdata/count-zeros.oro"); n != 0 {
		t.Errorf("the §9.4 witness: want no restriction (len a ≤ len b is not assumed), got %d", n)
	}
}
