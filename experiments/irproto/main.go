// P1 of docs/ir-research.md: what lowering to an IR costs, against what the
// pipeline costs today, on the same residual.
//
//	go run ./experiments/irproto [-target go] [-reps 20] [-memprofile FILE] SRC.oro
//
// It runs cmd/gen's pipeline stage by stage on every export, recording the
// time and the bytes allocated by each (runtime.MemStats.TotalAlloc), then
// lowers the residual the emitter would have received to C2 and to C1.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"oroboros/core"
	"oroboros/emit"
)

type stage struct {
	name  string
	d     time.Duration
	bytes uint64
}

var stages = map[string]*stage{}
var order []string

func measure(name string, f func()) {
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	t0 := time.Now()
	f()
	d := time.Since(t0)
	runtime.ReadMemStats(&b)
	s, ok := stages[name]
	if !ok {
		s = &stage{name: name}
		stages[name] = s
		order = append(order, name)
	}
	s.d += d
	s.bytes += b.TotalAlloc - a.TotalAlloc
}

func main() {
	target := flag.String("target", "go", "target")
	reps := flag.Int("reps", 20, "repetitions of the lowering, for a stable time")
	memprofile := flag.String("memprofile", "", "write an allocation profile of today's pipeline to FILE")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: irproto [-target T] SRC.oro")
		os.Exit(2)
	}
	src := flag.Arg(0)
	if *memprofile != "" {
		runtime.MemProfileRate = 64 * 1024
	}
	residuals, tg := pipeline(src, *target)
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err == nil {
			pprof.Lookup("allocs").WriteTo(f, 0)
			f.Close()
		}
	}

	var total time.Duration
	var totalB uint64
	fmt.Printf("%-34s %10s %12s\n", "today's pipeline, per stage", "time", "allocated")
	for _, n := range order {
		s := stages[n]
		total += s.d
		totalB += s.bytes
		fmt.Printf("%-34s %10s %10.1f MB\n", n, s.d.Round(time.Millisecond), float64(s.bytes)/1e6)
	}
	fmt.Printf("%-34s %10s %10.1f MB\n", "TOTAL", total.Round(time.Millisecond), float64(totalB)/1e6)

	// THE LOWERINGS, on the residuals the emitter received.
	var nodes, opaque int
	var c2, c1, dom time.Duration
	var c2B, c1B, domB uint64
	var desc []string
	var blocks int
	for _, rt := range residuals {
		var fn *Func
		var l *lowerer
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		t0 := time.Now()
		for i := 0; i < *reps; i++ {
			fn, l = Lower(tg, rt)
		}
		c2 += time.Since(t0) / time.Duration(*reps)
		runtime.ReadMemStats(&b)
		c2B += (b.TotalAlloc - a.TotalAlloc) / uint64(*reps)
		nodes += l.nodes
		opaque += l.opaque
		desc = append(desc, fn.String())

		var g *CFG
		runtime.ReadMemStats(&a)
		t0 = time.Now()
		for i := 0; i < *reps; i++ {
			g = Flatten(fn)
		}
		c1 += time.Since(t0) / time.Duration(*reps)
		runtime.ReadMemStats(&b)
		c1B += (b.TotalAlloc - a.TotalAlloc) / uint64(*reps)
		blocks += len(g.Blocks)

		runtime.ReadMemStats(&a)
		t0 = time.Now()
		for i := 0; i < *reps; i++ {
			g.RPOAndDominators()
		}
		dom += time.Since(t0) / time.Duration(*reps)
		runtime.ReadMemStats(&b)
		domB += (b.TotalAlloc - a.TotalAlloc) / uint64(*reps)
	}
	fmt.Println()
	fmt.Printf("residual: %d term nodes lowered, %d opaque; C2: %s\n", nodes, opaque, strings.Join(desc, "; "))
	fmt.Printf("%-34s %10s %10.3f MB\n", "residual -> C2 (structured)", c2, float64(c2B)/1e6)
	fmt.Printf("%-34s %10s %10.3f MB   (%d blocks)\n", "C2 -> C1 (flatten)", c1, float64(c1B)/1e6, blocks)
	fmt.Printf("%-34s %10s %10.3f MB\n", "C1: RPO + dominators (to print)", dom, float64(domB)/1e6)
}

// pipeline is cmd/gen's run, stage by stage, returning each export's residual
// as the emitter receives it.
func pipeline(src, target string) ([]*core.Term, *emit.Target) {
	die := func(err error) {
		fmt.Fprintln(os.Stderr, "irproto:", err)
		os.Exit(1)
	}
	var tg *emit.Target
	var prog *core.Program
	var env *core.Env
	var reqs *emit.RequireSet
	measure("load (targets, program, checks)", func() {
		layers, err := emit.SearchPath(src, "targets")
		if err != nil {
			die(err)
		}
		dirs := []string{filepath.Dir(src), "lib"}
		tg, err = emit.LoadTargetLayers(target, layers, dirs)
		if err != nil {
			die(err)
		}
		text, err := os.ReadFile(src)
		if err != nil {
			die(err)
		}
		forms, err := core.Read(string(text))
		if err != nil {
			die(err)
		}
		var terms []*core.Term
		prog, terms, err = core.LoadWithDefs(forms, fileResolver(dirs), tg.Defs)
		if err != nil {
			die(err)
		}
		env, err = tg.Env(prog)
		if err != nil {
			die(err)
		}
		if err := env.CheckProgram(terms); err != nil {
			die(err)
		}
		if err := emit.CheckSignatures(tg, prog, env); err != nil {
			die(err)
		}
		reqs = emit.InstallRequires(env, prog)
	})
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	var out []*core.Term
	for _, q := range exports {
		name := "gen-" + q[strings.LastIndex(q, ".")+1:]
		var nf *core.Term
		var err error
		measure("reduce (staging)", func() { nf, err = core.Normalize(prog.Defs[q], env, core.DefaultFuel) })
		if err != nil {
			die(err)
		}
		measure("contracts (DischargeRequires)", func() { nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf) })
		if err != nil {
			die(err)
		}
		sig := prog.Sigs[q]
		measure("term rewrites (products, big, words)", func() {
			if nfl, fsig, k, e := emit.FlattenProducts(tg, sig, nf); e == nil && k > 0 {
				nf, sig = nfl, fsig
			}
			nb, _, e := emit.PromoteBig(tg, sig, nf, allSigs(prog)...)
			if e != nil {
				die(e)
			}
			nf = nb
			if emit.DeclaresWord(tg, sig, nf) {
				if nw, k := emit.SelectWords(tg, sig, nf); k > 0 {
					nf = nw
				}
			}
		})
		measure("type check", func() { err = emit.Check(tg, name, nf) })
		if err != nil {
			die(err)
		}
		measure("linearity", func() { err = emit.CheckLinear(nf, tg, sig) })
		if err != nil {
			die(err)
		}
		measure("refinement", func() { _, err = emit.Refine(tg, name, sig, nf) })
		if err != nil {
			die(err)
		}
		measure("intervals (legality)", func() { emit.Intervals(tg, sig, nf, 0) })
		measure("shifts", func() {
			if sh, k := emit.SelectShifts(tg, sig, nf); k > 0 {
				nf = sh
			}
		})
		out = append(out, nf)
		measure("emission ("+target+")", func() {
			switch target {
			case "go":
				_, err = emit.Func(tg, name, sig, nf)
			case "js":
				_, err = emit.JSFunc(tg, name, sig, nf)
			case "java":
				_, err = emit.JavaMethod(tg, name, sig, nf)
			default:
				_, err = emit.AsmProc(tg, name, sig, nf)
			}
		})
		if err != nil {
			die(err)
		}
	}
	return out, tg
}

func fileResolver(dirs []string) core.Resolver {
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

func allSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}
