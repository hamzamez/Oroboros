package main

import (
	"fmt"
	"os"
	"sort"

	"oroboros/core"
	"oroboros/emit"
)

// reportRequires is -report-requires: every contract obligation (ADR 0028) that
// reduction left for the analyses, with the interval analysis's verdict, one
// line per distinct (definition, parameter, argument):
//
//	requires DEF PARAM TYPE proven|unproven ARG  got INTERVAL
//
// A literal argument is decided while the call reduces and does not appear.
// An unproven line may still be proven by the refinement layer, which
// DischargeRequires asks next; what it cannot prove is refused.
var reportRequires bool

// reportResidual prints the verdict on each range mark in a unit's residual. It
// changes nothing: the term it reads is discharged as usual.
func reportResidual(tg *emit.Target, sig *core.Sig, nf *core.Term) {
	res, _ := emit.MeasureRequires(tg, sig, nf)
	seen := map[string]bool{}
	var lines []string
	for _, r := range res {
		class := "unproven"
		if r.Proven {
			class = "proven"
		}
		line := fmt.Sprintf("requires %s %s %q %s %s  got %v", r.Def, r.Param, r.Type, class, r.Arg, r.Got)
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
}
