// Command gen compiles an Oroboros program for a target and writes the result.
//
// The target is loaded from targets/NAME.oro — a data file, not Go source. A
// program declares no targets of its own; which names are primitive comes
// entirely from the target file, which is the whole of ADR 0002's parameter.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
	"oroboros/ir/java"
	"oroboros/ir/js"
	"oroboros/ir/x86"
)

func main() {
	dir := flag.String("targets", "targets", "search path for target declarations; the source's own directory is always the nearest layer")
	name := flag.String("name", "", "name for the emitted function (defaults to the source's stem)")
	path := flag.String("path", "lib", "search path for imported modules")
	bigRepr := flag.String("big-repr", "", "storage for a value above the target's word: `limbs` or `host`, overriding what the target declares. The BOUND is the declaration's either way, so this changes how a program is stored and not what it computes")
	checked := flag.Bool("checked", false,
		"rewrite integer operations the compiler cannot bound to the target's checked form")
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile of this compile to `FILE` (go tool pprof)")
	flag.StringVar(&irOut, "ir", "", "also lower what the backend receives to the IR (docs/spec/ir.md), verify it, and write its canonical text to `FILE`; a lowering or verification failure is written to FILE.err and changes nothing that is emitted")
	flag.BoolVar(&reportRequires, "report-requires", false,
		"print the interval analysis's verdict on every contract obligation reduction left (ADR 0028, requires.go)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gen [-targets DIR] [-name N] SRC.oro TARGET OUT\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gen:", err)
			os.Exit(2)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, "gen:", err)
			os.Exit(2)
		}
	}
	err := run(*dir, flag.Arg(0), flag.Arg(1), flag.Arg(2), *name, *path, *checked, *bigRepr)
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, name, path string, checked bool, bigRepr string) error {
	layers, err := emit.SearchPath(src, targetDir)
	if err != nil {
		return err
	}
	tg, err := emit.LoadTargetLayers(target, layers, libDirs(src, path))
	if err != nil {
		return err
	}
	// WHICH CODE GENERATOR COMPILES THIS TARGET, asked of the TARGET and not of
	// the flag, and asked HERE so a target that cannot answer is refused before
	// anything is reduced (target-system.md §1.1).
	backend, err := tg.ResolveBackend()
	if err != nil {
		return err
	}
	// AN OVERRIDE, NOT A DECISION. The target declares which representation it
	// prefers, because that declaration is a measurement somebody took. This
	// exists so the alternative can be measured on the same program, and so the
	// two can be checked against each other — a change of storage that changed
	// an answer would be ADR 0009's rule broken at the representation boundary.
	if bigRepr != "" {
		if bigRepr != "limbs" && bigRepr != "host" {
			return fmt.Errorf("-big-repr is `limbs` or `host`, got %q", bigRepr)
		}
		tg.BigRepr = bigRepr
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// A TARGET FRAGMENT IS NOT A PROGRAM. `(provides TARGET PATH …)` is read by
	// the target loader — it is `(target T (module PATH …))` written where the
	// library lives — so a file that says nothing else has nothing to emit, and
	// saying so beats reporting the first name inside it as unbound.
	if core.OnlyFragments(forms) {
		return fmt.Errorf("%s is a target fragment — it declares (provides …) and nothing else, "+
			"so there is no program here to build", src)
	}
	prog, terms, err := core.LoadWithDefs(forms, fileResolver(libDirs(src, path)), tg.Defs)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// Name resolution, over EVERY definition rather than only what reduction
	// reaches — a typo in unused code was previously invisible.
	if err := env.CheckProgram(terms); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	for _, n := range env.Shadowed() {
		fmt.Fprintf(os.Stderr, "note: %s is defined here and provided natively by target %q; "+
			"the target's is used\n", n, target)
	}

	// A signature is checked against the TARGET's native implementation as
	// well as against the definition — the one job no host compiler can do,
	// since the two live on different targets (docs/spec/types.md).
	if err := emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, allSigs(prog))); err != nil {
		return err
	}
	// A DEFINITION'S CONTRACT IS CHECKED AT ITS CALLS (ADR 0028), from here on:
	// the units below are the program's calls. CheckSignatures, above, reduced
	// every signed definition on its own with its parameters free, and those are
	// not calls the program makes.
	reqs := emit.InstallRequires(env, prog)

	// A program's entry points are its EXPORTS, and an emitted function is named
	// after the export it came from. Naming by position — GenGeneric0,
	// GenGeneric1 — was the last thing modules left unfinished (modules.md §1).
	//
	// A file with no `(export …)` still works: its anonymous top-level terms are
	// named from the source stem, as before. That is the whole of the fallback.
	type unit struct {
		name string
		qual string // the fully qualified export, so its signature can be found
		term *core.Term
	}
	var units []unit
	prefix := name
	if prefix == "" {
		prefix = "gen"
	}
	for _, q := range prog.Exports {
		local := q
		if i := strings.LastIndex(local, "."); i >= 0 {
			local = local[i+1:]
		}
		units = append(units, unit{name: prefix + "-" + local, qual: q, term: prog.Defs[q]})
	}
	if len(units) == 0 {
		stem := name
		if stem == "" {
			stem = "gen-" + strings.TrimSuffix(filepath.Base(src), ".oro")
		}
		for i, t := range terms {
			n := stem
			if len(terms) > 1 {
				n = fmt.Sprintf("%s-%d", stem, i)
			}
			units = append(units, unit{name: n, term: t})
		}
	}

	funcs := map[string]string{}
	irProg := &ir.Program{Target: target, Stage: ir.StageA}
	var irErrs []string
	for _, u := range units {
		nf, err := core.Normalize(u.term, env, core.DefaultFuel)
		if err != nil {
			return err
		}
		// A JOIN POINT IS NOT JUMPED OUT OF (tables.md §2.5): refused before any
		// pass walks a clause chain.
		// η FOR PRODUCTS (tables.md §2.5): a host call's several results at a
		// producer's exit become the tuple a tuple pattern takes apart.
		nf = emit.EtaTails(tg, nf)
		if err := emit.CheckJoins(tg, nf); err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
		if reportRequires {
			reportResidual(reqs, tg, prog.Sigs[u.qual], nf)
		}
		if nf, err = emit.DischargeRequires(reqs, tg, u.name, prog.Sigs[u.qual], nf,
			func(x *core.Term) *core.Term { return ir.DischargeRanges(tg, prog.Sigs[u.qual], x) }); err != nil {
			return err
		}
		fname := u.name
		// THE PIPELINE (ir.Entry): products, the rung above the word, the checks
		// on terms, the decision, the unsigned word and the postcondition, as
		// every driver takes them.
		fA, leg, err := ir.Entry(tg, fname, fname, prog.Sigs[u.qual], nf, allSigs(prog), checked,
			func(n ir.Note) {
				switch n.Kind {
				case ir.NoteFlattened:
					fmt.Fprintf(os.Stderr, "note: %s: %d product access(es) flattened\n", fname, n.N)
				case ir.NoteRefine:
					fmt.Fprintln(os.Stderr, "note:", n.Text)
				case ir.NoteBig:
					fmt.Fprintf(os.Stderr, "note: %s: %d operation(s) in arbitrary precision\n", fname, n.N)
				case ir.NoteWord:
					fmt.Fprintf(os.Stderr, "note: %s: %d operation(s) or conversion(s) in the unsigned word\n", fname, n.N)
				}
			})
		if err != nil {
			return err
		}
		if leg.Ops > 0 || leg.Loops > 0 {
			fmt.Fprintf(os.Stderr, "note: %s: %d of %d integer operations bounded; "+
				"%d of %d loop(s) proven terminating\n",
				fname, leg.Proven, leg.Ops, leg.Halts, leg.Loops)
		}
		// BOUNDED BY DEFAULT (ADR 0019). `-checked` is the second escape: the
		// IR has written `trap` where the proof failed.
		if !checked {
			if err := leg.Refusal(fname, tg); err != nil {
				return err
			}
		}
		if err := leg.SizeRefusal(fname, tg); err != nil {
			return err
		}
		// DIVISION BY A POWER OF TWO IS A SHIFT where the decision's facts put
		// the dividend in [0, 2^shift-width) (shiftdiv-2026-09-03, ir/shift.go).
		if shifts := ir.SelectShifts(tg, fA); shifts > 0 {
			fmt.Fprintf(os.Stderr, "note: %s: %d division(s) became a shift or a mask\n", fname, shifts)
		}
		// THE IR (ADR 0032), the decided IR_A the backend receives. It is
		// written beside the code; cmd/check's `ir` step reads it (docs/spec/
		// ir.md §11: lowering is total on the corpus).
		if irOut != "" {
			irProg.Funcs = append(irProg.Funcs, fA.Clone())
		}
		// The BACKEND, not the flag — see cmd/build and target-system.md §1.1.
		var code string
		switch backend {
		case "js":
			code, err = js.FromFunc(tg, fA)
		case "java":
			code, err = java.FromFunc(tg, fA)
		case "x86-64":
			code, err = x86.FromFunc(tg, fA)
		case "go":
			code, err = golang.FromFunc(tg, fA)
		default:
			return fmt.Errorf("no code generator for backend %q", backend)
		}
		if err != nil {
			return err
		}
		funcs[fname] = code
	}

	var text2 string
	switch backend {
	case "js":
		text2 = emit.JSFile(funcs)
	case "java":
		base := filepath.Base(out)
		text2 = emit.JavaFile(strings.TrimSuffix(base, ".java"), funcs)
	case "x86-64":
		text2 = emit.AsmFile(tg, funcs, "")
	case "go":
		text2 = emit.File("gauntlet", funcs)
	default:
		return fmt.Errorf("no code generator for backend %q", backend)
	}
	if err := os.WriteFile(out, []byte(text2), 0o644); err != nil {
		return err
	}
	if irOut != "" {
		writeIR(tg, irProg, irErrs)
	}
	fmt.Printf("wrote %s\n", out)
	return nil
}

// irOut is -ir's file.
var irOut string

// writeIR is ir.WriteFile on -ir's file.
func writeIR(tg *emit.Target, p *ir.Program, errs []string) {
	_ = ir.WriteFile(irOut, tg, p, errs)
}

// fileResolver finds a module on a search path: `(use num/vec)` looks for
// num/vec.oro under each directory in turn. A path with no file is not an
// error — it is a module the TARGET provides, like go/strings.
func fileResolver(dirs []string) core.Resolver {
	return func(path string) (string, bool, error) {
		for _, d := range dirs {
			p := filepath.Join(d, filepath.FromSlash(path)+".oro")
			b, err := os.ReadFile(p)
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

// libDirs is the search path: the entry file's own directory first, so a
// program can keep its modules beside it, then whatever -path adds.
func libDirs(entry, extra string) []string {
	dirs := []string{filepath.Dir(entry)}
	for _, d := range filepath.SplitList(extra) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// allSigs is every signature the program declares. The fixed-limb rung's width
// comes from the whole program rather than from one function, because reduction
// inlines every non-exported call and `main` has no signature at all — see
// emit/biglimb.go's LimbWidth.
func allSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}
