//go:build ignore

// Measure is M-S1 and M-S2 of specialization-research.md §9.
//
// M-S1, duplication, as Proposition 2's two normal forms of one program:
//   - R = |NF_∅|, the residual with every definition unfolded, today's;
//   - R_K = |NF_K| + Σ_{f ∈ K reached} |NF_K(f)|, with K the dynamic,
//     first-order definitions kept: never δ-unfolded, each body once.
//
// R − R_K is what keeping them saves. K is taken twice: K_hdr, the
// definitions with a header (a contract P3 can use), and K_all, every dynamic
// one (the bound). A definition found applied to a λ is higher-order, static
// by §3, and leaves K; π(f) counts δ on f in NF_∅.
//
// M-S2, proofs at a boundary: each dynamic definition with a header, decided
// once under its contract (as an entry is: its sets and `where` assumed at
// its boundary, ADR 0028), beside the program's own verdict, which inlines it.
//
//	go run gauntlet/factoring/measure.go [-target go] [-v] > out.txt
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

func size(t *core.Term) int {
	if t == nil {
		return 0
	}
	n := 1
	for _, k := range t.Kids {
		n += size(k)
	}
	return n
}

// names collects every name t mentions.
func names(t *core.Term, into map[string]bool) {
	if t == nil {
		return
	}
	if t.Kind == core.KName {
		into[t.Name] = true
	}
	for _, k := range t.Kids {
		names(k, into)
	}
}

// lambdaArgs collects the names applied to a λ somewhere in t.
func lambdaArgs(t *core.Term, into map[string]bool) {
	if t == nil {
		return
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		for _, a := range t.Args() {
			if a.Kind == core.KFn {
				into[t.Op().Name] = true
			}
		}
	}
	for _, k := range t.Kids {
		lambdaArgs(k, into)
	}
}

func dynamic(env *core.Env, name string, body *core.Term) bool {
	return body != nil && env.Static[name] == 0 && body.Kind == core.KFn
}

// bounded says every integer parameter of sig has both ends: a contract the
// word can be decided under.
func bounded(sig *core.Sig) bool {
	for _, p := range sig.Params {
		if p.Type == "int" || p.Type == "Int" || p.Type == "" || p.Type == "any" {
			return false
		}
		if strings.HasPrefix(p.Type, "int") {
			lo, hi, ok := core.IntRange(p.Type)
			_ = lo
			_ = hi
			if !ok {
				return false
			}
		}
	}
	return true
}

type unit struct {
	name string
	sig  *core.Sig
	term *core.Term
}

type row struct {
	src                 string
	R, RHdr, RAll       int
	kHdr, kAll, dup     int
	accepted            string
	defs                []defRow
	unitsN, failedNorms int
}

type defRow struct {
	name    string
	own, pi int
	verdict string
}

func main() {
	target := flag.String("target", "go", "the target to reduce and decide on")
	verbose := flag.Bool("v", false, "list every definition decided once")
	flag.Parse()
	var srcs []string
	for _, dir := range []string{"examples", "gauntlet/differential/cases", "gauntlet/stdlib/acceptance"} {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".oro") {
				srcs = append(srcs, filepath.ToSlash(p))
			}
			return nil
		})
	}
	sort.Strings(srcs)
	start := time.Now()
	var out []*row
	for _, src := range srcs {
		r, err := measure(src, *target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", src, firstLine(err))
			continue
		}
		out = append(out, r)
	}
	fmt.Printf("# M-S1 and M-S2 on %s: %d programs measured of %d, %s\n\n", *target, len(out), len(srcs),
		time.Since(start).Round(time.Second))
	fmt.Println("program | R = |NF_∅| | R_K hdr | R_K all | kept hdr | kept all | of them π ≥ 2 | R_K all / R | verdict")
	var tR, tH, tA int
	var big []*row
	for _, p := range out {
		tR += p.R
		tH += p.RHdr
		tA += p.RAll
		fmt.Printf("%s | %d | %d | %d | %d | %d | %d | %.2f | %s\n", p.src, p.R, p.RHdr, p.RAll, p.kHdr, p.kAll, p.dup,
			float64(p.RAll)/float64(max(p.R, 1)), p.accepted)
		big = append(big, p)
	}
	fmt.Printf("\ntotal: R %d, R_K hdr %d (%.3f), R_K all %d (%.3f)\n", tR, tH, float64(tH)/float64(max(tR, 1)),
		tA, float64(tA)/float64(max(tR, 1)))
	sort.Slice(big, func(i, j int) bool { return big[i].R-big[i].RAll > big[j].R-big[j].RAll })
	fmt.Println("\nlargest savings R − R_K all:")
	for _, p := range big[:min(12, len(big))] {
		fmt.Printf("  %s: %d → %d (%.2f)\n", p.src, p.R, p.RAll, float64(p.RAll)/float64(max(p.R, 1)))
	}
	fmt.Print("\n# M-S2: each dynamic definition with a header, decided once under its contract\n\n")
	counts := map[string]int{}
	for _, p := range out {
		for _, r := range p.defs {
			cat := strings.SplitN(r.verdict, ":", 2)[0]
			if strings.HasPrefix(r.verdict, "proven") || strings.HasPrefix(r.verdict, "unproven") {
				if strings.Contains(r.verdict, "unbounded contract") {
					cat += " (unbounded contract)"
				} else {
					cat += " (bounded contract)"
				}
			}
			counts[cat+" / program "+strings.SplitN(p.accepted, " ", 2)[0]]++
			if *verbose || !strings.HasPrefix(r.verdict, "proven") {
				fmt.Printf("%s | %s | |f| %d | π %d | %s | program %s\n", p.src, r.name, r.own, r.pi, r.verdict, p.accepted)
			}
		}
	}
	fmt.Println()
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("%-70s %d\n", k, counts[k])
	}
}

// inputsFor is the harness's: a `// inputs:` line, or its defaults.
func inputsFor(src string) []int {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "// inputs:") {
			continue
		}
		var out []int
		for _, f := range strings.Fields(strings.TrimPrefix(l, "// inputs:")) {
			var n int
			if _, err := fmt.Sscanf(f, "%d", &n); err == nil {
				out = append(out, n)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []int{0, 1, 2, 3, 5, 8, 13, 21, 34}
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

type loaded struct {
	tg    *emit.Target
	prog  *core.Program
	env   *core.Env
	reqs  *emit.RequireSet
	all   []*core.Sig
	units []unit
}

func load(src, target string) (*loaded, error) {
	emit.ResetImports()
	layers, err := emit.SearchPath(src, "targets")
	if err != nil {
		return nil, err
	}
	ds := []string{filepath.Dir(src), "lib"}
	tg, err := emit.LoadTargetLayers(target, layers, ds)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}
	forms, err := core.ReadModule(string(text), src, core.ModulePath(src, []string{"lib"}))
	if err != nil {
		return nil, err
	}
	prog, terms, err := core.LoadWithDefs(forms, core.SearchPath(ds), tg.Defs)
	if err != nil {
		return nil, err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return nil, err
	}
	l := &loaded{tg: tg, prog: prog, env: env, reqs: emit.InstallRequires(env, prog)}
	for _, s := range prog.Sigs {
		l.all = append(l.all, s)
	}
	for _, q := range prog.Exports {
		l.units = append(l.units, unit{q, prog.Sigs[q], prog.Defs[q]})
	}
	// A DIFFERENTIAL CASE's entry is the harness's: `run` applied to each of
	// its inputs (gauntlet/differential/run.go), so each call is specialized at
	// its constant as there.
	if len(l.units) == 0 && prog.Defs["run"] != nil {
		for i, n := range inputsFor(string(text)) {
			t := &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name("run"), core.Int(int64(n))}}
			l.units = append(l.units, unit{fmt.Sprintf("main#%d", i), nil, t})
		}
	}
	if len(l.units) == 0 {
		for i, t := range terms {
			l.units = append(l.units, unit{fmt.Sprintf("main#%d", i), nil, t})
		}
	}
	if len(l.units) == 0 {
		return nil, fmt.Errorf("no entry")
	}
	return l, nil
}

// knf is |NF_K| of the program's units plus each kept definition reached,
// once, and the kept definitions found applied to a λ.
func knf(l *loaded, keep map[string]bool) (int, map[string]bool, error) {
	saved := map[string]bool{}
	for n := range keep {
		saved[n] = l.env.Rec[n]
		l.env.Rec[n] = true
	}
	defer func() {
		for n, v := range saved {
			if v {
				l.env.Rec[n] = true
			} else {
				delete(l.env.Rec, n)
			}
		}
	}()
	total := 0
	ho := map[string]bool{}
	reached := map[string]bool{}
	var work []string
	visit := func(t *core.Term) {
		total += size(t)
		lambdaArgs(t, ho)
		ns := map[string]bool{}
		names(t, ns)
		for n := range ns {
			if keep[n] && !reached[n] {
				reached[n] = true
				work = append(work, n)
			}
		}
	}
	for _, u := range l.units {
		nf, err := core.Normalize(u.term, l.env, core.DefaultFuel)
		if err != nil {
			return 0, nil, err
		}
		visit(nf)
	}
	for len(work) > 0 {
		n := work[0]
		work = work[1:]
		nf, err := core.Normalize(l.prog.Defs[n], l.env, core.DefaultFuel)
		if err != nil {
			return 0, nil, err
		}
		visit(nf)
	}
	for n := range ho {
		if !keep[n] {
			delete(ho, n)
		}
	}
	return total, ho, nil
}

// keptSize is knf with K shrunk until no kept definition is applied to a λ.
func keptSize(l *loaded, keep map[string]bool) (int, int, error) {
	for round := 0; round < 8; round++ {
		r, ho, err := knf(l, keep)
		if err != nil {
			return 0, 0, err
		}
		if len(ho) == 0 {
			return r, len(keep), nil
		}
		for n := range ho {
			delete(keep, n)
		}
	}
	return 0, 0, fmt.Errorf("K reaches no first-order fixed point")
}

func measure(src, target string) (*row, error) {
	l, err := load(src, target)
	if err != nil {
		return nil, err
	}
	r := &row{src: src, accepted: "accepted"}
	// NF_∅, with π counted, and the program's verdict
	l.env.Unfolds = map[string]int{}
	for _, u := range l.units {
		nf, err := core.Normalize(u.term, l.env, core.DefaultFuel)
		if err != nil {
			return nil, err
		}
		r.R += size(nf)
		if ok, why := verdict(l, u.name, u.sig, nf); !ok && r.accepted == "accepted" {
			r.accepted = "refused (" + why + ")"
		}
	}
	pi := l.env.Unfolds
	l.env.Unfolds = nil
	if os.Getenv("MS_TOP") == src {
		type d struct {
			n    string
			c, o int
		}
		var ds []d
		for n, c := range pi {
			if b := l.prog.Defs[n]; dynamic(l.env, n, b) {
				ds = append(ds, d{n, c, size(b)})
			}
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i].o*(ds[i].c-1) > ds[j].o*(ds[j].c-1) })
		for _, x := range ds[:min(15, len(ds))] {
			_, hdr := l.prog.Sigs[x.n]
			fmt.Fprintf(os.Stderr, "TOP %s |f| %d π %d header %v\n", x.n, x.o, x.c, hdr)
		}
	}
	entry := map[string]bool{}
	for _, u := range l.units {
		entry[u.name] = true
	}
	hdr, all := map[string]bool{}, map[string]bool{}
	for n, c := range pi {
		body := l.prog.Defs[n]
		if entry[n] || c == 0 || !dynamic(l.env, n, body) {
			continue
		}
		all[n] = true
		if c >= 2 {
			r.dup++
		}
		if l.prog.Sigs[n] != nil {
			hdr[n] = true
		}
	}
	if r.RHdr, r.kHdr, err = keptSize(l, hdr); err != nil {
		return nil, err
	}
	if r.RAll, r.kAll, err = keptSize(l, all); err != nil {
		return nil, err
	}
	// M-S2
	var ns []string
	for n := range hdr {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	for _, n := range ns {
		r.defs = append(r.defs, defRow{n, size(l.prog.Defs[n]), pi[n], decideOnce(l, n)})
	}
	return r, nil
}

// verdict is the program's: its unit through the pipeline, accepted or not.
func verdict(l *loaded, name string, sig *core.Sig, nf *core.Term) (bool, string) {
	nf = emit.EtaTails(l.tg, nf)
	nf, err := emit.DischargeRequires(l.reqs, l.tg, name, sig, nf,
		func(x *core.Term) *core.Term { return ir.DischargeRanges(l.tg, sig, x) })
	if err != nil {
		return false, trim(firstLine(err))
	}
	_, leg, err := ir.Entry(l.tg, name, name, sig, nf, l.all, false, nil)
	if err != nil {
		return false, trim(firstLine(err))
	}
	if err := leg.Refusal(name, l.tg); err != nil {
		return false, trim(firstLine(err))
	}
	return true, ""
}

func trim(s string) string {
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}

// decideOnce decides definition n as a function: once, under its contract,
// as an entry is (its sets and `where` assumed at its boundary, ADR 0028).
func decideOnce(l *loaded, n string) string {
	sig := l.prog.Sigs[n]
	nf, err := core.Normalize(l.prog.Defs[n], l.env, core.DefaultFuel)
	if err != nil {
		return "not an entry: " + firstLine(err)
	}
	nf = emit.EtaTails(l.tg, nf)
	nf, err = emit.DischargeRequires(l.reqs, l.tg, n, sig, nf,
		func(x *core.Term) *core.Term { return ir.DischargeRanges(l.tg, sig, x) })
	if err != nil {
		return "not an entry: " + firstLine(err)
	}
	_, leg, err := ir.Entry(l.tg, n, n, sig, nf, l.all, false, nil)
	if err != nil {
		return "not an entry: " + firstLine(err)
	}
	b := "bounded"
	if !bounded(sig) {
		b = "unbounded"
	}
	if leg.Refusal(n, l.tg) == nil && leg.SizeRefusal(n, l.tg) == nil {
		return fmt.Sprintf("proven: %s contract, %d ops, %d loops", b, leg.Ops, leg.Loops)
	}
	return fmt.Sprintf("unproven: %s contract, %d of %d ops, %d loops; %s", b, leg.Ops-leg.Proven, leg.Ops,
		leg.Loops, strings.Join(leg.Unproven, "; "))
}
