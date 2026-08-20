// Command intervals answers the question that gates the integer design:
// how often can the compiler prove an integer stays in a machine word?
//
// docs/spec/data-model.md §8. Run over the gauntlet and the sieves, with and
// without simulated range declarations, it produces the number that decides
// whether "exact by default, ranges choose the representation" is a design or a
// trap.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	targets := flag.String("targets", "targets", "directory holding target declarations")
	path := flag.String("path", "lib", "search path for imported modules")
	assume := flag.Int64("assume", 0, "simulate a declared range [0,N] on every parameter and length")
	verbose := flag.Bool("v", false, "list every operation that could not be proven")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: intervals [-assume N] [-v] SRC.oro TARGET\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*targets, flag.Arg(0), flag.Arg(1), *path, *assume, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "intervals:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, path string, assume int64, verbose bool) error {
	tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
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
	prog, terms, err := core.LoadWith(forms, resolver(dirs(src, path)))
	if err != nil {
		return err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return err
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

	total, proven, lv, lb := 0, 0, 0, 0
	loops, term, trips := 0, 0, 0
	byOp := map[string][2]int{}
	for _, u := range units {
		nf, err := core.Normalize(u.term, env, core.DefaultFuel)
		if err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
		r, _ := emit.Intervals(tg, u.sig, nf, assume)
		total += r.Ops
		proven += r.Proven
		lv += r.LoopVars
		lb += r.LoopBound
		loops += r.Loops
		term += r.Terminates
		trips += r.Trips
		if verbose {
			for _, d := range r.Diverging {
				fmt.Printf("    %s: no descent on the cycle %s\n", u.name, d)
			}
		}
		for k, v := range r.ByOp {
			e := byOp[k]
			byOp[k] = [2]int{e[0] + v[0], e[1] + v[1]}
		}
		if verbose {
			for _, n := range r.Unproven {
				fmt.Printf("    %s: %s\n", u.name, n)
			}
		}
	}
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(proven) / float64(total)
	}
	lpct := 0.0
	if lv > 0 {
		lpct = 100 * float64(lb) / float64(lv)
	}
	var ops []string
	for k := range byOp {
		ops = append(ops, k)
	}
	sort.Strings(ops)
	var parts []string
	for _, k := range ops {
		parts = append(parts, fmt.Sprintf("%s %d/%d", k, byOp[k][0], byOp[k][1]))
	}
	_ = lpct
	fmt.Printf("%-28s %3d/%-3d ops (%5.1f%%)  loopvar %d/%d  term %d/%d  trip %d/%d  %s\n",
		filepath.Base(src), proven, total, pct, lb, lv, term, loops, trips, loops,
		strings.Join(parts, "  "))
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

func dirs(entry, extra string) []string {
	out := []string{filepath.Dir(entry)}
	for _, d := range filepath.SplitList(extra) {
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}
