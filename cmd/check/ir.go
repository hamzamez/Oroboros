package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oroboros/ir"
)

// THE IR STEP (docs/spec/ir.md §11, ADR 0032).
//
// The emission sweep runs gen with -ir, so every program that emits is also
// lowered to the IR, verified (W1–W10) and printed canonically, into .check/ir.
// This step reads what the sweep wrote and checks three properties:
//
//   - TOTALITY: every program that emitted code has an IR. A lowering or a
//     verification that fails is a .err file, named here with its reason.
//   - ONLY WHAT EMITTED: no program that was refused has an IR, because L runs
//     on what a backend receives.
//   - THE PRINTING IS CANONICAL: read ∘ print is the identity on the text, so
//     print ∘ read ∘ print = print for every file.
//
// It needs the sweep and runs after it. It records no baseline of its own: the
// IR's text is not a gate until a printer reads it (ADR 0032, "Consequences").

func irDir(work string) string { return filepath.Join(work, "ir") }

func irName(source, target string) string { return emittedName(source, target) + ".ir" }

func (c *checker) ir() result {
	if !c.emissionRan {
		return result{status: skip, detail: "needs the emission sweep"}
	}
	dir := irDir(c.work)
	var missing, stray, failed, drift []string
	lowered, funcs, stmts, empty := 0, 0, 0, 0
	for _, o := range c.emitted {
		file := filepath.Join(dir, irName(o.Source, o.Target))
		_, errIR := os.Stat(file)
		errText, errErr := os.ReadFile(file + ".err")
		switch {
		case o.Emitted && errErr == nil:
			failed = append(failed, fmt.Sprintf("%s %s: %s", o.Source, o.Target, firstLines(string(errText), 3)))
		case o.Emitted && errIR != nil:
			missing = append(missing, o.Source+" "+o.Target)
		case !o.Emitted && (errIR == nil || errErr == nil):
			stray = append(stray, o.Source+" "+o.Target)
		case o.Emitted:
			text, err := os.ReadFile(file)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s %s: %v", o.Source, o.Target, err))
				continue
			}
			p, err := ir.Read(string(text))
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s %s: the reader refuses the printer's text: %v", o.Source, o.Target, err))
				continue
			}
			if again := ir.Print(p); again != string(text) {
				drift = append(drift, o.Source+" "+o.Target)
				continue
			}
			lowered++
			funcs += len(p.Funcs)
			if len(p.Funcs) == 0 {
				empty++
			}
			for _, f := range p.Funcs {
				f.Walk(func(r *ir.Region) { stmts += len(r.Stmts) })
			}
		}
	}
	// A program with no export and no top-level term emits no function at all:
	// the differential cases' `run` is compiled by their own runner. The sweep's
	// scope, named rather than counted as coverage (CLAUDE.md: "the emission
	// sweep compiles EXPORTS").
	summary := fmt.Sprintf("%d of %d emitted programs lowered and verified (%d of them emit no function): %d functions, %d operations; round trip exact",
		lowered, countEmittedN(c.emitted), empty, funcs, stmts)
	var bad []string
	for _, g := range []struct {
		what string
		list []string
	}{
		{"lowering or verification failed", failed},
		{"emitted but no IR was written", missing},
		{"refused but an IR was written", stray},
		{"print ∘ read ∘ print ≠ print", drift},
	} {
		if len(g.list) == 0 {
			continue
		}
		sort.Strings(g.list)
		bad = append(bad, fmt.Sprintf("%d %s:", len(g.list), g.what))
		for i, l := range g.list {
			if i == 10 {
				bad = append(bad, fmt.Sprintf("  … and %d more", len(g.list)-10))
				break
			}
			bad = append(bad, "  "+l)
		}
	}
	if len(bad) > 0 {
		for _, l := range bad {
			fmt.Println("   " + l)
		}
		return result{status: fail, detail: summary}
	}
	return result{status: pass, detail: summary}
}

func countEmittedN(outs []outcome) int {
	n := 0
	for _, o := range outs {
		if o.Emitted {
			n++
		}
	}
	return n
}

func firstLines(s string, n int) string {
	ls := strings.Split(strings.TrimSpace(s), "\n")
	if len(ls) > n {
		ls = append(ls[:n], "…")
	}
	return strings.Join(ls, " | ")
}
