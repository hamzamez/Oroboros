//go:build ignore

// Depth is M-P1 of p8-research.md: what keeping a program's definitions buys in
// the reducer, the measurement P8's trigger named. For each program it
// reduces twice:
//   - K = ∅, every definition δ-unfolded (today);
//   - K = every dynamic definition the program defines (the bound of P2's K),
//     each never δ-unfolded at a call, and each body normalized once.
//
// It reports the residual's size (nodes) and nesting depth, the wall time, and
// whether reduction finished: depth is what core.MaxDepth bounds, and a
// sequence of inlined calls nests each continuation at the previous body's
// tails, so inlined depths add along a sequence where kept ones take a max.
//
//	go run gauntlet/factoring/measure.go gauntlet/factoring/depth.go -target windows FILE…
//
// (measure.go's helpers, load and dynamic, are shared; its main is replaced
// by setting DEPTH=1.)
package main

import (
	"fmt"
	"os"
	"time"

	"oroboros/core"
)

func depthOf(t *core.Term) int {
	if t == nil {
		return 0
	}
	d := 0
	for _, k := range t.Kids {
		if x := depthOf(k); x > d {
			d = x
		}
	}
	return d + 1
}

// depthMain runs when DEPTH is set, before measure.go's main.
func init() {
	if os.Getenv("DEPTH") == "" {
		return
	}
	target := "windows"
	var srcs []string
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-target" && i+1 < len(os.Args) {
			target = os.Args[i+1]
			i++
			continue
		}
		srcs = append(srcs, os.Args[i])
	}
	fmt.Println("program | K | finished | residual nodes | max depth | kept | time")
	for _, src := range srcs {
		for _, keepAll := range []bool{false, true} {
			l, err := load(src, target)
			if err != nil {
				fmt.Printf("%s | load: %v\n", src, err)
				break
			}
			keep := map[string]bool{}
			if keepAll {
				entry := map[string]bool{}
				for _, u := range l.units {
					entry[u.name] = true
				}
				for n, b := range l.prog.Defs {
					if !entry[n] && dynamic(l.env, n, b) {
						keep[n] = true
						l.env.Rec[n] = true
					}
				}
			}
			start := time.Now()
			nodes, depth, fin := 0, 0, "yes"
			bodies := []*core.Term{}
			for _, u := range l.units {
				bodies = append(bodies, u.term)
			}
			for n := range keep {
				bodies = append(bodies, l.prog.Defs[n])
			}
			for _, t := range bodies {
				nf, err := core.Normalize(t, l.env, core.DefaultFuel)
				if err != nil {
					fin = "no: " + firstLine(err)
					if len(fin) > 60 {
						fin = fin[:60]
					}
					break
				}
				nodes += size(nf)
				if d := depthOf(nf); d > depth {
					depth = d
				}
			}
			k := "∅"
			if keepAll {
				k = "all dynamic"
			}
			fmt.Printf("%s | %s | %s | %d | %d | %d | %s\n", src, k, fin, nodes, depth, len(keep), time.Since(start).Round(time.Millisecond))
		}
	}
	os.Exit(0)
}
