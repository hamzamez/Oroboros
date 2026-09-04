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
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	dir := flag.String("targets", "targets", "directory holding target declarations")
	name := flag.String("name", "", "name for the emitted function (defaults to the source's stem)")
	path := flag.String("path", "lib", "search path for imported modules")
	bigRepr := flag.String("big-repr", "", "storage for a value above the portable window: `limbs` or `host`, overriding what the target declares. The BOUND is the declaration's either way, so this changes how a program is stored and not what it computes")
	checked := flag.Bool("checked", false,
		"rewrite integer operations the compiler cannot bound to the target's checked form")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gen [-targets DIR] [-name N] SRC.oro TARGET OUT\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), flag.Arg(1), flag.Arg(2), *name, *path, *checked, *bigRepr); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, name, path string, checked bool, bigRepr string) error {
	tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
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
	prog, terms, err := core.LoadWith(forms, fileResolver(libDirs(src, path)))
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
	if err := emit.CheckSignatures(tg, prog, env); err != nil {
		return err
	}

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
	for _, u := range units {
		nf, err := core.Normalize(u.term, env, core.DefaultFuel)
		if err != nil {
			return err
		}
		fname := u.name
		// ARBITRARY PRECISION, ADR 0019's THIRD ESCAPE (emit/bigrep.go). Before
		// the checker, because the promotion is part of what the program MEANS:
		// `(* acc i)` types as `int` and would be refused against a result the
		// program has declared bigger than a machine word.
		nb, n, err := emit.PromoteBig(tg, prog.Sigs[u.qual], nf, allSigs(prog)...)
		if err != nil {
			return fmt.Errorf("%s: %w", fname, err)
		}
		if n > 0 {
			nf = nb
			fmt.Fprintf(os.Stderr, "note: %s: %d operation(s) in arbitrary precision\n", fname, n)
		}
		// Check the residual before emitting it (docs/spec/types.md). On Go and
		// Java the host would catch most of this; on JavaScript nothing would.
		if err := emit.Check(tg, fname, nf); err != nil {
			return err
		}
		// Refinements: the bounds obligation primitives.md §2 recorded and
		// nothing checked (docs/spec/refinements.md).
		sig := prog.Sigs[u.qual]
		// ADR 0018's linearity, checked on the residual rather than by a type.
		if err := emit.CheckLinear(nf, tg); err != nil {
			return fmt.Errorf("%s: %w", fname, err)
		}
		if notes, err := emit.Refine(tg, fname, sig, nf); err != nil {
			return err
		} else {
			for _, n := range notes {
				fmt.Fprintln(os.Stderr, "note:", n)
			}
		}
		// REPRESENTATION SELECTION. Every integer operation whose result is
		// not provably inside the portable window is rewritten to the checked
		// primitive the target declares — and one that IS provable keeps the
		// host's own operator, so a program the compiler can bound costs
		// nothing (sct-2026-08-19, data-model.md §1.5).
		//
		// A target declaring no checked form gets its term back unchanged.
		// A POSTCONDITION on an exported definition is an OBLIGATION, not an
		// assumption: the caller is outside the program, so the body is the
		// only evidence there is (postconditions.md §2).
		if ok, note := emit.CheckEnsures(tg, sig, nf); !ok {
			return fmt.Errorf("%s: %s", fname, note)
		} else if note != "" {
			fmt.Fprintln(os.Stderr, "note:", fname+": "+note)
		}
		rep, sel := emit.Intervals(tg, sig, nf, 0)
		if rep.Ops > 0 || rep.Loops > 0 {
			fmt.Fprintf(os.Stderr, "note: %s: %d of %d integer operations bounded; "+
				"%d of %d loop(s) proven terminating\n",
				fname, rep.Proven, rep.Ops, rep.Terminates, rep.Loops)
		}
		// BOUNDED BY DEFAULT (ADR 0019). `-checked` is the second escape: it
		// takes the trap instead of the refusal.
		if checked {
			nf = sel
		} else if err := emit.Unbounded(fname, rep); err != nil {
			return err
		}
		// DIVISION BY A POWER OF TWO IS A SHIFT where the analysis can prove the
		// dividend non-negative and inside the target's declared shift width
		// (shiftdiv-2026-09-03). LAST, because the fixed-limb library's own
		// carry splits are spliced in by the promotion above and are what this
		// is most for; and its own pass, because `Intervals` below has the
		// checked selection ON and using its rebuilt term by default would
		// reverse ADR 0012 without an ADR.
		if sh, k := emit.SelectShifts(tg, sig, nf); k > 0 {
			nf = sh
			fmt.Fprintf(os.Stderr, "note: %s: %d division(s) became a shift or a mask\n", fname, k)
		}
		var code string
		switch target {
		case "js":
			code, err = emit.JSFunc(tg, fname, sig, nf)
		case "java":
			code, err = emit.JavaMethod(tg, fname, sig, nf)
		case "windows":
			code, err = emit.AsmProc(tg, fname, sig, nf)
		default:
			code, err = emit.Func(tg, fname, sig, nf)
		}
		if err != nil {
			return err
		}
		funcs[fname] = code
	}

	var text2 string
	switch target {
	case "js":
		text2 = emit.JSFile(funcs)
	case "java":
		base := filepath.Base(out)
		text2 = emit.JavaFile(strings.TrimSuffix(base, ".java"), funcs)
	case "windows":
		text2 = emit.AsmFile(tg, funcs, "")
	default:
		text2 = emit.File("gauntlet", funcs)
	}
	if err := os.WriteFile(out, []byte(text2), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", out)
	return nil
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
