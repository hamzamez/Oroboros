package main

import (
	"fmt"
	"os"
	"sort"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// reportRequires is -report-requires: every range obligation (ADR 0028) that
// reduction left for the analyses, with the interval route's verdict (the
// IR's facts, ir.MeasureRanges), one line per distinct (definition, parameter,
// argument):
//
//	requires DEF PARAM TYPE proven|unproven ARG  got INTERVAL
//
// A literal argument is decided while the call reduces and does not appear.
// An unproven line may still be proven by the refinement layer, which
// DischargeRequires asks next; what it cannot prove is refused.
var reportRequires bool

// reportResidual prints the verdict on each range mark in a unit's residual. It
// changes nothing: the term it reads is discharged as usual.
func reportResidual(reqs *emit.RequireSet, tg *emit.Target, sig *core.Sig, nf *core.Term) {
	rest, ascribed := reqs.DecideAscribed(tg.Word, nf)
	seen := map[string]bool{}
	var lines []string
	add := func(def, param, ty, arg, got string, proven bool) {
		class := "unproven"
		if proven {
			class = "proven"
		}
		line := fmt.Sprintf("requires %s %s %q %s %s  got %s", def, param, ty, class, arg, got)
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	for _, r := range ascribed {
		add(r.Def, r.Param, r.Type, r.Arg, r.Got, r.Proven)
	}
	for _, r := range ir.MeasureRanges(tg, sig, rest) {
		add(r.Def, r.Param, r.Type, r.Arg, r.Got, r.Proven)
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
}
