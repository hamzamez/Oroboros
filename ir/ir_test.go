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
	forms, err := core.ReadEntry(string(text), src)
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
		nf = emit.EtaTails(tg, nf)
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf,
			func(x *core.Term) *core.Term { return DischargeRanges(tg, prog.Sigs[q], x) }); err != nil {
			return nil, nil, err
		}
		sig := prog.Sigs[q]
		if nfl, fsig, k, err := emit.FlattenProducts(tg, sig, nf); err != nil {
			return nil, nil, err
		} else if k > 0 {
			nf, sig = nfl, fsig
		}
		// THE RUNG ABOVE THE WORD, as the drivers take it (cmd/gen): fixed limbs
		// on the term, the host's bignum on the IR.
		plan, err := emit.PlanBig(tg, sig, nf)
		if err != nil {
			return nil, nil, err
		}
		if plan.Host || plan.Limbs {
			nf = emit.EraseWordAscriptions(tg.Word, nf)
		} else {
			nf = emit.EraseAscriptions(nf)
		}
		f, err := Lower(tg, name, sig, nf, opt)
		if err != nil {
			return nil, nil, err
		}
		// THE NUMBERS OF N, as Entry takes them (pipeline.go)
		if _, err := SelectNumbers(tg, f); err != nil {
			return nil, nil, err
		}
		if plan.Host || plan.Limbs {
			Decide(tg, f, false)
			if _, err := SelectRung(tg, f, sig, plan); err != nil {
				return nil, nil, err
			}
		}
		p.Funcs = append(p.Funcs, f)
	}
	return tg, p, nil
}

func resolver(dirs []string) core.Resolver { return core.SearchPath(dirs) }

// corpus is a spread of shapes: loops with π, a join point, tables built and
// read, maps read and built, host calls with several results, big integers.
var corpus = []struct{ src, target string }{
	{"examples/native/dot.oro", "go"},
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
	{"examples/sum/parse.oro", "go"},       // a variant at a boundary, (fn (#k) (#k tag payload)): a product by structure
	{"ir/testdata/js-any-dot.oro", "js"},   // JavaScript's `+` over `any`: ℤ's add only on integer operands
	{"examples/big/render.oro", "windows"}, // `len` of a big integer, which the limb representation has
}

// TestOverloadedOperatorsAreDecidedByTheirOperands: on JavaScript `+` is
// declared over `any`, so the float accumulation in dot stays a call and the
// index increment is promoted to the language's add.
func TestOverloadedOperatorsAreDecidedByTheirOperands(t *testing.T) {
	_, p := compile(t, "ir/testdata/js-any-dot.oro", "js")
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
// the loop's shape, its π-parameters, the classification of >= and + on the
// index as the language's operations, and each rounding realized as the call
// of the declaration that states it (numbers.md §6.2).
func TestDotIsTheSpecExample(t *testing.T) {
	_, p := compile(t, "examples/native/dot.oro", "go")
	text := Print(p)
	for _, want := range []string{
		"(do (assume %4))",        // the export's `where`, assumed at entry (ADR 0028)
		"(val (%5 int) (len %0))", // measured once, before the loop (ADR 0071)
		"(loop (init %6 %7)",
		"(val (%11 bool) (ge %10 %5))",
		"(pi %15 int (%10 lt %5))",
		"(pi %16 int (%5 gt %10))",
		"(pi %17 (array f64) ((len %0) gt %10))", // a guard on a measured length guards the table
		"(val (%18 f64) (index %17 %15))",
		"(val (%20 f64) (call go.fmul %18 %19))", // round(F64, a[i] · b[i]), Go's realization
		"(val (%21 f64) (call go.fadd %9 %20))",  // round(F64, s + …)
		"(add %15 %22)",
		"(continue %21 %23)",
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
	const dot = "examples/native/dot.oro"
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
	tg, _ := compile(t, "examples/native/dot.oro", "go")
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
	tg, _ := compile(t, "examples/native/dot.oro", "go")
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
	tg, _ := compile(t, "examples/native/dot.oro", "go")
	term, err := core.ParseTerm("(fn (x) (fn (y) y))")
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
	if n := count("examples/native/dot.oro"); n != 1 {
		t.Errorf("dot: want 1 restriction (its `where` assumes len p = len q), got %d", n)
	}
	if n := count("ir/testdata/count-zeros.oro"); n != 0 {
		t.Errorf("the §9.4 witness: want no restriction (len a ≤ len b is not assumed), got %d", n)
	}
}

// THE CRASH ENDS ITS REGION (spec/errors.md §8): `abandon` is crash : E → 0,
// so the region it ends has no exit, `unreachable`, and contributes nothing to
// its owner's results. A body with no exit at all takes its declared results,
// by the one arrow 0 → R. The text round-trips (the corpus test reads every
// program back); here each claim is pinned on the example, and the verifier's
// rule has a planted fault in each direction.
func TestTheCrashEndsItsRegion(t *testing.T) {
	const src = "examples/errors/expect.oro"
	tg, p := compile(t, src, "go")
	if err := Verify(tg, p); err != nil {
		t.Fatal(err)
	}
	text := Print(p)
	if n := strings.Count(text, "(unreachable)"); n != 3 {
		t.Errorf("want three unreachable regions, two in quarter and one in always-crashes, got %d in\n%s", n, text)
	}
	for _, f := range p.Funcs {
		if len(f.Results) != 1 || f.Results[0] != "int" {
			t.Errorf("%s: the declared result is int, got %v", f.Name, f.Results)
		}
	}
	t.Run("W3 unreachable without the crash", func(t *testing.T) {
		expect(t, plant(t, src, func(p *Program) {
			for _, f := range p.Funcs {
				f.Walk(func(r *Region) {
					if r.T == TUnreachable {
						r.Stmts = r.Stmts[:len(r.Stmts)-1]
					}
				})
			}
		}), "W3")
	})
	t.Run("W3 a yielding region marked unreachable", func(t *testing.T) {
		expect(t, plant(t, "examples/native/dot.oro", func(p *Program) {
			p.Funcs[0].Body.T = TUnreachable
		}), "W3")
	})
}

// pisOf is every π of f with the region holding it.
func pisOf(f *Func) (out []struct {
	r *Region
	i int
}) {
	f.Walk(func(r *Region) {
		for i := range r.Pis {
			out = append(out, struct {
				r *Region
				i int
			}{r, i})
		}
	})
	return out
}

// A π IS JUSTIFIED BY ITS BRANCH (Theorem E as W6, item 7 of the 10-24 plan):
// each fault the plan names, planted in dot's IR, which verifies before it.
// The first is the one that shipped (work-2026-10-22 §5): a measured length
// credited to the other table.
func TestAPiIsJustifiedByItsBranch(t *testing.T) {
	const dot = "examples/native/dot.oro"
	lengthPi := func(p *Program) *Pi {
		for _, x := range pisOf(p.Funcs[0]) {
			if x.r.Pis[x.i].Len {
				return &x.r.Pis[x.i]
			}
		}
		t.Fatal("dot has no length π")
		return nil
	}
	valuePi := func(p *Program, rel string) *Pi {
		for _, x := range pisOf(p.Funcs[0]) {
			if pi := &x.r.Pis[x.i]; !pi.Len && pi.Rel == rel {
				return pi
			}
		}
		t.Fatalf("dot has no %s π", rel)
		return nil
	}
	t.Run("the wrong table", func(t *testing.T) {
		got := plant(t, dot, func(p *Program) {
			pi := lengthPi(p)
			pi.Of = p.Funcs[0].Params[1] // b, whose length the guard did not measure
		})
		expect(t, got, "W6")
		if !strings.Contains(got, "does not imply") {
			t.Errorf("the refusal does not name Theorem E's premise: %s", got)
		}
	})
	t.Run("a flipped relation", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			pi := valuePi(p, "lt")
			pi.Rel = "gt"
		}), "W6")
	})
	t.Run("a foreign operand", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			pi := valuePi(p, "lt")
			pi.Other = p.Funcs[0].Params[0] // compared with a, not the measured length
		}), "W6")
	})
	t.Run("a missing negation", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			pi := valuePi(p, "lt") // the else arm of `i >= n`: i < n
			pi.Rel = "ge"          // the guard as written, not its negation
		}), "W6")
	})
	t.Run("a π no branch enters", func(t *testing.T) {
		expect(t, plant(t, dot, func(p *Program) {
			_, loop := find(p.Funcs[0], OLoop)
			body := loop.Sub[0]
			arm := body.Else
			if arm == nil || len(arm.Pis) == 0 {
				t.Fatal("the loop's continuing arm has no π")
			}
			body.Pis = append(body.Pis, arm.Pis[0])
			arm.Pis = arm.Pis[1:]
		}), "W6")
	})
}

// THE FALSE SIDE OF A CONJUNCTION IMPLIES NO CONJUNCT'S NEGATION: ¬(a ∧ b) is
// ¬a ∨ ¬b, so the meet over its two routes is empty. A π stating a ≥ n on the
// else arm of `a < n && b < n` is planted, and refused; its premise is computed:
// the program verifies before it, and the then arm's a < n is implied.
func TestTheFalseSideOfAConjunctionImpliesNoConjunct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "and.oro")
	src := "export fn f(a ∈ 0..9, b ∈ 0..9, n ∈ 0..9) (Int) { if a < n && b < n { a } else { b } }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, p, err := compilePath(path, "go", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(tg, p); err != nil {
		t.Fatalf("the program verifies before the fault is planted: %v", err)
	}
	f := p.Funcs[0]
	// the body ends in the branch on the conjunction: its else arm gets a ≥ n
	if f.Body.T != TBranch {
		t.Fatalf("the body does not end in the branch on the conjunction:\n%s", Print(p))
	}
	fresh := V(len(f.Types))
	f.Types = append(f.Types, f.Types[f.Params[0]])
	f.Body.Else.Pis = append(f.Body.Else.Pis, Pi{V: fresh, Of: f.Params[0], Other: f.Params[2], Rel: "ge"})
	err = Verify(tg, p)
	if err == nil || !strings.Contains(err.Error(), "W6:") {
		t.Errorf("a ≥ n on the false side of a < n ∧ b < n was not refused: %v", err)
	}
}
