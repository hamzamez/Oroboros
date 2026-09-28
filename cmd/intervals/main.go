// Command intervals reports what the IR's interval domain proves about a
// program on one target: how many integer operations it proves inside their
// word, and how many loops it proves terminate, per exported definition
// (ir.Decide, docs/spec/ir.md §7). With -v it lists each operation it could not
// prove, by its source application and its interval.
//
// It runs the pipeline the drivers run up to the decision: reduction, the
// contracts, the product flattening, the rung above the word and the unsigned
// word. A main-only program reports its top-level terms.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

func main() {
	targets := flag.String("targets", "targets", "directory holding target declarations")
	path := flag.String("path", "lib", "search path for imported modules")
	verbose := flag.Bool("v", false, "list every operation that could not be proven")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: intervals [-v] SRC.oro TARGET\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*targets, flag.Arg(0), flag.Arg(1), *path, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "intervals:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, path string, verbose bool) error {
	layers, err := emit.SearchPath(src, targetDir)
	if err != nil {
		return err
	}
	ds := dirs(src, path)
	tg, err := emit.LoadTargetLayers(target, layers, ds)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return err
	}
	prog, terms, err := core.LoadWithDefs(forms, resolver(ds), tg.Defs)
	if err != nil {
		return err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return err
	}
	reqs := emit.InstallRequires(env, prog)
	var all []*core.Sig
	for _, s := range prog.Sigs {
		all = append(all, s)
	}
	type unit struct {
		name string
		sig  *core.Sig
		term *core.Term
	}
	var units []unit
	for _, q := range prog.Exports {
		units = append(units, unit{q, prog.Sigs[q], prog.Defs[q]})
	}
	if len(units) == 0 {
		for i, t := range terms {
			units = append(units, unit{fmt.Sprintf("%s#%d", filepath.Base(src), i), nil, t})
		}
	}
	total, proven, loops, halts := 0, 0, 0, 0
	for _, u := range units {
		nf, err := core.Normalize(u.term, env, core.DefaultFuel)
		if err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
		sig := u.sig
		nf = emit.EtaTails(tg, nf)
		if nf, err = emit.DischargeRequires(reqs, tg, u.name, sig, nf,
			func(x *core.Term) *core.Term { return ir.DischargeRanges(tg, sig, x) }); err != nil {
			return err
		}
		_, leg, err := ir.Entry(tg, u.name, u.name, sig, nf, all, false, nil)
		if err != nil {
			return err
		}
		total += leg.Ops
		proven += leg.Proven
		loops += leg.Loops
		halts += leg.Halts
		if verbose {
			fmt.Printf("  %-24s %3d/%-3d ops  %d/%d loops\n", u.name, leg.Proven, leg.Ops, leg.Halts, leg.Loops)
			for _, m := range leg.Unproven {
				fmt.Printf("      %s\n", m)
			}
		}
	}
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(proven) / float64(total)
	}
	fmt.Printf("%-24s %3d/%-3d ops (%5.1f%%)  loops %d/%d terminate\n",
		filepath.Base(src), proven, total, pct, halts, loops)
	return nil
}

func resolver(ds []string) core.Resolver {
	return func(p string) (string, bool, error) {
		for _, d := range ds {
			b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(p)+".oro"))
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

func dirs(src, path string) []string {
	out := []string{filepath.Dir(src)}
	for _, d := range filepath.SplitList(path) {
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}
