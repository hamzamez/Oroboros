package x86

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/ir"
)

// PLACES (docs/spec/ir.md §9.6). Every value's class (its aliases share one
// place) gets its exact live set, a union of ranges of program points, and the
// classes are coloured greedily in order of definition. For strict SSA that
// greedy colouring in dominance order is optimal (Hack, Grund and Goos 2006):
// it uses exactly as many registers as the most values live at one point.

var (
	valueGP = []string{"rbx", "rsi", "rdi", "r12", "r13", "r14", "r15"}
	valueX  = []string{"xmm6", "xmm7", "xmm8", "xmm9", "xmm10", "xmm11", "xmm12", "xmm13"}
)

// loc is where a class lives: a register, or a frame slot numbered from 1.
type loc struct {
	reg  string
	slot int
	xmm  bool
}

// span is a closed range of program points; a class's live set is a union.
type span struct{ lo, hi int }

// live is one class's live set, and what allocation needs to know of it.
type live struct {
	cls     ir.V
	start   int // the definition, which orders the colouring
	spans   []span
	xmm     bool
	weight  float64
	hints   []ir.V // classes whose register this one would like to share
	inPlace ir.V   // the operand whose register it may take at its start, or -1
}

func (a *live) meets(b *live) bool {
	for _, x := range a.spans {
		for _, y := range b.spans {
			if x.lo <= y.hi && y.lo <= x.hi {
				return true
			}
		}
	}
	return false
}

// meetsOnlyAt: a and b meet at point q and nowhere else.
func (a *live) meetsOnlyAt(b *live, q int) bool {
	for _, x := range a.spans {
		for _, y := range b.spans {
			lo, hi := max(x.lo, y.lo), min(x.hi, y.hi)
			if lo <= hi && (lo != q || hi != q) {
				return false
			}
		}
	}
	return true
}

// numbering is the pre-order walk of §9.6: points, definitions and live sets.
type numbering struct {
	p       *printer
	pt      int
	def     map[ir.V]int
	spans   map[ir.V][]span
	weight  map[ir.V]float64
	open    []*container // the statements and branches the walk is inside
	pending []pend       // a whole loop, whose end is not yet numbered
	depth   int          // loop nesting
	loops   []*ir.Stmt   // the loops the walk is inside, innermost last
}

// container is a statement with sub-regions, or a branch. entry is the entry
// point of the sub-region the walk is in now.
type container struct {
	start, end, entry int
	loop              bool
}

type pend struct {
	v ir.V
	c *container
}

func (n *numbering) defAt(v ir.V, at int) {
	c := n.p.cls(v)
	if d, ok := n.def[c]; !ok || at < d {
		n.def[c] = at
	}
	n.spans[c] = append(n.spans[c], span{at, at})
	n.weight[c] += pow8(n.depth)
}

// useAt adds to v's live set the points on the paths from its definition to a
// read at q. Outside the containers the walk is in, that is [def, q]. Into a
// container not holding the definition, it is the container's own point and
// the path through the sub-region the read is in, not its siblings, except
// that a loop is covered whole, since its back edge returns to every point.
func (n *numbering) useAt(v ir.V, q int) {
	if v < 0 {
		return
	}
	c := n.p.cls(v)
	if n.p.isConst(c) {
		return
	}
	n.weight[c] += pow8(n.depth)
	d, ok := n.def[c]
	if !ok {
		return
	}
	from := d
	for _, k := range n.open {
		if k.start <= d {
			continue // the definition is inside this container
		}
		n.spans[c] = append(n.spans[c], span{from, k.start})
		if k.loop {
			n.pending = append(n.pending, pend{c, k})
			return
		}
		from = k.entry
	}
	n.spans[c] = append(n.spans[c], span{from, q})
}

func pow8(d int) float64 {
	if d > 6 {
		d = 6
	}
	w := 1.0
	for i := 0; i < d; i++ {
		w *= 8
	}
	return w
}

func (n *numbering) next() int { n.pt++; return n.pt }

// enter marks the next point as the entry of the sub-region about to be walked.
func (n *numbering) enter(k *container) { k.entry = n.pt + 1 }

func (n *numbering) region(r *ir.Region) {
	entry := n.next()
	for _, q := range r.Params {
		n.defAt(q, entry)
	}
	for i := range r.Stmts {
		s := &r.Stmts[i]
		if !n.p.pl.Live(s) {
			continue
		}
		n.stmt(s)
	}
	at := n.next()
	switch r.T {
	case ir.TYield, ir.TBreak, ir.TContinue:
		for _, a := range r.Args {
			n.readCond(a, at) // a fused yield (a boolean arm's) is read as its jumps
		}
		if r.T == ir.TContinue && len(n.loops) > 0 {
			// The spare swap reads the spare and the parameter it alternates
			// with, here.
			l := n.loops[len(n.loops)-1]
			for _, sp := range n.p.spareOf[l] {
				n.useAt(sp, at)
				n.useAt(l.Sub[0].Params[n.p.spareParam[n.p.cls(sp)]], at)
			}
		}
	case ir.TBranch:
		n.readCond(r.Cond, at)
		k := &container{start: at}
		n.open = append(n.open, k)
		n.enter(k)
		n.region(r.Then)
		n.enter(k)
		n.region(r.Else)
		n.open = n.open[:len(n.open)-1]
		k.end = n.next()
	}
}

// readCond reads a condition at point at: a materialised boolean is read
// there, and a fused one's operands are, since its jumps are emitted there.
func (n *numbering) readCond(c ir.V, at int) {
	if n.p.fused[n.p.pl.Res(c)] {
		d := n.p.defs[n.p.pl.Res(c)]
		switch d.Op {
		case ir.OIf:
			n.readCond(d.Args[0], at)
		default:
			for _, a := range d.Args {
				n.useAt(a, at)
			}
		}
		return
	}
	n.useAt(c, at)
}

func (n *numbering) stmt(s *ir.Stmt) {
	at := n.next()
	fusedHere := len(s.Res) == 1 && n.p.fused[n.p.pl.Res(s.Res[0])]
	switch {
	case fusedHere:
		// Its reads happen where it is consumed (readCond), immediately after;
		// a fused `if`'s arms are printed there, in this order.
		if s.Op == ir.OIf {
			k := &container{start: at}
			n.open = append(n.open, k)
			for _, sub := range s.Sub {
				n.enter(k)
				n.region(sub)
			}
			n.open = n.open[:len(n.open)-1]
			k.end = n.next()
		}
		return
	case s.Op == ir.OIf:
		n.readCond(s.Args[0], at)
	default:
		for _, a := range s.Args {
			n.useAt(a, at)
		}
	}
	if len(s.Sub) == 0 {
		for _, v := range s.Res {
			n.defAt(v, at)
		}
		return
	}
	// A tabulate is a loop too: its table, count and index are read on every
	// iteration, and the table exists from the start.
	tab := s.Op == ir.OTabulate
	k := &container{start: at, loop: s.Op == ir.OLoop || tab}
	n.open = append(n.open, k)
	if tab {
		n.defAt(s.Res[0], at)
		for _, x := range []ir.V{s.Res[0], s.Args[0], s.Sub[0].Params[0]} {
			if c := n.p.cls(x); !n.p.isConst(c) {
				n.pending = append(n.pending, pend{c, k})
			}
		}
		n.depth++
		defer func() { n.depth-- }()
	}
	if s.Op == ir.OLoop {
		// A spare is allocated before the loop and lives across it.
		n.loops = append(n.loops, s)
		for _, sp := range n.p.spareOf[s] {
			n.defAt(sp, at)
			n.pending = append(n.pending, pend{n.p.cls(sp), k})
		}
		n.depth++
	}
	for _, sub := range s.Sub {
		n.enter(k)
		if s.Op == ir.OBuild {
			// tableOf writes the length header after the allocator returns, so
			// the size is read where the buffer is defined: they interfere.
			n.useAt(s.Args[0], k.entry)
		}
		n.region(sub)
	}
	if s.Op == ir.OLoop {
		n.depth--
		n.loops = n.loops[:len(n.loops)-1]
	}
	n.open = n.open[:len(n.open)-1]
	k.end = n.next()
	for _, v := range s.Res {
		n.defAt(v, k.end)
	}
}

// allocate numbers the function and colours its classes.
func (p *printer) allocate() error {
	n := &numbering{p: p, def: map[ir.V]int{}, spans: map[ir.V][]span{}, weight: map[ir.V]float64{}}
	entry := n.next()
	for _, v := range p.f.Params {
		n.defAt(v, entry)
	}
	n.region(p.f.Body)
	for _, pd := range n.pending {
		n.spans[pd.v] = append(n.spans[pd.v], span{pd.c.start, pd.c.end})
	}
	var ls []*live
	for c, d := range n.def {
		if p.isConst(c) {
			continue
		}
		ls = append(ls, &live{cls: c, start: d, spans: n.spans[c], xmm: p.isFloat(c), weight: n.weight[c], inPlace: -1})
	}
	// Dominance order: by definition, then by class (a printer is a function
	// of its input).
	sort.Slice(ls, func(i, j int) bool {
		if ls[i].start != ls[j].start {
			return ls[i].start < ls[j].start
		}
		return ls[i].cls < ls[j].cls
	})
	byCls := map[ir.V]*live{}
	for _, l := range ls {
		byCls[l.cls] = l
	}
	for _, l := range ls {
		l.hints = p.hints[l.cls]
		// The in-place exception applies when the operand and the result meet
		// only at the statement: then no path from it reads the operand again
		// (scan's blocking test decides that on the live sets).
		if op, ok := p.inPlaceOf[l.cls]; ok {
			if o := byCls[p.cls(op)]; o != nil {
				l.inPlace = o.cls
			}
		}
	}
	p.scan(ls)
	p.lives = ls
	return nil
}

// scan colours in dominance order: each class takes a register no class
// already on it meets, preferring the in-place operand's, then a hint's. When
// a file is exhausted, the cheaper of spilling this class or the classes
// blocking one register goes to frame slots, whole (spill everywhere).
func (p *printer) scan(ls []*live) {
	on := map[string][]*live{} // register → the classes on it
	var slotted []*live
	newSlot := func(l *live) {
		used := map[int]bool{}
		for _, a := range slotted {
			if a.meets(l) {
				used[p.loc[a.cls].slot] = true
			}
		}
		k := 1
		for used[k] {
			k++
		}
		p.loc[l.cls] = loc{slot: k, xmm: l.xmm}
		if k > p.slots {
			p.slots = k
		}
		slotted = append(slotted, l)
	}
	blocking := func(l *live, r string) []*live {
		var out []*live
		for _, a := range on[r] {
			if a.meets(l) && !(l.inPlace == a.cls && a.meetsOnlyAt(l, l.start)) {
				out = append(out, a)
			}
		}
		return out
	}
	for _, l := range ls {
		regs := valueGP
		if l.xmm {
			regs = valueX
		}
		pick := ""
		var prefs []string
		if l.inPlace >= 0 {
			prefs = append(prefs, p.loc[l.inPlace].reg)
		}
		for _, h := range l.hints {
			prefs = append(prefs, p.loc[p.cls(h)].reg)
		}
		for _, r := range append(prefs, regs...) {
			if r != "" && inFile(r, regs) && len(blocking(l, r)) == 0 {
				pick = r
				break
			}
		}
		if pick == "" {
			// Free the register whose blockers weigh least, if they weigh
			// less than this class; otherwise this class takes a slot.
			best, bestCost := "", l.weight
			for _, r := range regs {
				cost := 0.0
				for _, a := range blocking(l, r) {
					cost += a.weight
				}
				if cost < bestCost {
					best, bestCost = r, cost
				}
			}
			if best == "" {
				newSlot(l)
				continue
			}
			for _, a := range blocking(l, best) {
				on[best] = removeLive(on[best], a)
				newSlot(a)
			}
			pick = best
		}
		p.loc[l.cls] = loc{reg: pick, xmm: l.xmm}
		on[pick] = append(on[pick], l)
		if l.xmm {
			p.usedX[pick] = true
		} else {
			p.usedGP[pick] = true
		}
	}
}

func inFile(r string, regs []string) bool {
	for _, x := range regs {
		if x == r {
			return true
		}
	}
	return false
}

func removeLive(as []*live, x *live) []*live {
	var out []*live
	for _, a := range as {
		if a != x {
			out = append(out, a)
		}
	}
	return out
}

// text is a class's place as an operand.
func (p *printer) text(l loc) string {
	switch {
	case l.reg != "":
		return l.reg
	case l.slot > 0:
		return fmt.Sprintf("qword ptr [rsp+%d]", p.shadow+8*(l.slot-1))
	}
	return ""
}

func isMem(s string) bool { return strings.Contains(s, "[") }
