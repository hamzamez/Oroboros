// Package x86 prints IR_P as x86-64 assembly for MASM and the Win64 calling
// convention (ADR 0032 step 3, docs/spec/ir.md §9.6). It is the one host with
// no variables, so the printer decides where each value lives, and that
// decision is an interval colouring (alloc.go). What it shares with the term
// backend (emit/asm.go) is the target's templates, the table representation, the
// frame and the literal pool, through emit's exported helpers, so an IR-printed
// procedure and a term-printed one agree on every convention.
package x86

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/plan"
)

// Scratch registers. r10 and r11 carry slot operands into a template, which
// may not touch them (windows.oro); rax is free between templates, so the
// printer's own instructions (index, store, compare, moves) use it too.
const (
	scratchA = "r10"
	scratchB = "r11"
	scratchX = "xmm4"
	resultX  = "xmm5"
)

var (
	argGP = []string{"rcx", "rdx", "r8", "r9"}
	argX  = []string{"xmm0", "xmm1", "xmm2", "xmm3"}
	retGP = []string{"rax", "rdx", "r8", "r9"}
	retX  = []string{"xmm0", "xmm1", "xmm2", "xmm3"}
)

// FromResidual lowers a residual, finalises it to IR_P, verifies it and
// prints it: the whole of step 3 for one exported definition.
func FromResidual(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) (string, error) {
	f, err := ir.Lower(tg, name, sig, nf, ir.Options{Decided: true})
	if err != nil {
		return "", err
	}
	return FromFunc(tg, f)
}

// FromFunc prints a lowered and DECIDED function (ir.Decide): the step to
// IR_P, the verifier, and the printer. It is what the pipeline calls, so the
// modes the IR decided are the ones printed.
func FromFunc(tg *emit.Target, f *ir.Func) (string, error) {
	if err := ir.ToP(tg, f); err != nil {
		return "", err
	}
	return Func(tg, f)
}

// Func prints one IR_P function as a Win64 procedure.
func Func(tg *emit.Target, f *ir.Func) (string, error) {
	out, err := newPrinter(tg, f).function()
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name, err)
	}
	return out, nil
}

func newPrinter(tg *emit.Target, f *ir.Func) *printer {
	return &printer{tg: tg, f: f, pl: plan.New(tg, f),
		defs: map[ir.V]*ir.Stmt{}, fused: map[ir.V]bool{}, hints: map[ir.V][]ir.V{},
		inPlaceOf: map[ir.V]ir.V{}, spareOf: map[*ir.Stmt][]ir.V{}, spareFor: map[*ir.Stmt]ir.V{},
		spareLen: map[ir.V]int64{}, spareParam: map[ir.V]int{}, topOf: map[*ir.Stmt]string{},
		exitOf: map[*ir.Stmt]string{}, usedGP: map[string]bool{}, usedX: map[string]bool{}}
}

type printer struct {
	tg *emit.Target
	f  *ir.Func
	pl *plan.Plan
	b  strings.Builder

	defs      map[ir.V]*ir.Stmt // each value's defining statement
	fused     map[ir.V]bool     // booleans printed as jumps (§9.6)
	hints     map[ir.V][]ir.V   // classes a class would share a register with
	inPlaceOf map[ir.V]ir.V     // a result's in-place operand (§9.6's exception)
	// Rule R: a loop's spares, each the buffer parameter of the back-edge
	// build that writes into it, whose place therefore lives across the loop.
	spareOf    map[*ir.Stmt][]ir.V
	spareLen   map[ir.V]int64
	spareFor   map[*ir.Stmt]ir.V // a back-edge build, and its buffer (the spare)
	spareParam map[ir.V]int      // the loop parameter a spare alternates with

	loc           []loc
	lives         []*live // the live sets allocation coloured, for the tests
	slots         int
	usedGP, usedX map[string]bool
	shadow        int
	loops         []*ir.Stmt // the loops being printed, innermost last
	topOf, exitOf map[*ir.Stmt]string
	borrow        []int // frame slots for borrowed registers (operands)
	owner         []owner
	ret           string
	err           error
}

// owner is where a region's `yield` writes: a statement's results, or the
// function's return registers.
type owner struct {
	res      []ir.V
	done     string
	fn       bool
	store    bool // a tabulate's body: its yield is a store into tab at idx
	tab, idx ir.V
}

func (p *printer) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}

func (p *printer) line(format string, args ...any) {
	p.b.WriteString("        " + fmt.Sprintf(format, args...) + "\n")
}

func (p *printer) label(l string) { p.b.WriteString(l + ":\n") }

func (p *printer) newLabel(stem string) string { return fmt.Sprintf("%s%d", stem, emit.AsmUniq()) }

// ---------------------------------------------------------------- classes

// cls is a value's class representative.
func (p *printer) cls(v ir.V) ir.V { return p.pl.Res(v) }

func (p *printer) isConst(c ir.V) bool { return p.pl.Const[c] != nil }

func (p *printer) typeOf(v ir.V) string { return p.f.Types[p.cls(v)] }

func (p *printer) isFloat(v ir.V) bool { return p.tg.ValueType(p.typeOf(v)) == "f64" }

// width is a table's element width in bytes: 1 for a bool or a range the
// target holds in a byte, and the qword otherwise (emit.ElemBytes's rule, read
// off IR_P's final element type).
func (p *printer) width(tab ir.V) int {
	t := p.typeOf(tab)
	t = strings.TrimPrefix(t, "buffer ")
	t = strings.TrimPrefix(t, "array ")
	if e := ir.ElemOf(p.tg, p.typeOf(tab)); e != "" {
		t = e
	}
	if p.tg.ValueType(t) == "bool" || t == "bool" {
		return 1
	}
	if lo, hi, ok := core.IntRange(t); ok {
		if n := p.tg.ReprBytes(lo, hi); n == 1 {
			return 1
		}
	}
	return 8
}

// ---------------------------------------------------------------- decisions

// decide makes the decisions that change classes, before anything is
// numbered: a build's frozen buffer is its buffer, and a loop's reusable
// buffer alternates with a spare (Rule R, 2.5–2.7× measured). Loop results are
// NOT coalesced with parameters here: that keeps a counter live across the
// whole loop, which on this host costs a move on every iteration to save one
// at the exit.
func (p *printer) decide() {
	p.f.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				p.defs[v] = s
			}
			switch s.Op {
			case ir.OBuild:
				buf := s.Sub[0].Params[0]
				if len(s.Res) == 1 && p.pl.YieldsOnly(s.Sub[0], buf) {
					p.pl.Alias[s.Res[0]] = buf
				}
			case ir.OLoop:
				d := p.pl.DecideExits(s)
				var js []int
				for j := range d.Spare {
					js = append(js, j)
				}
				sort.Ints(js)
				for _, j := range js {
					sb := d.Spare[j]
					buf := sb.Build.Sub[0].Params[0]
					p.spareOf[s] = append(p.spareOf[s], buf)
					p.spareFor[sb.Build] = buf
					p.spareLen[p.cls(buf)] = sb.Len
					p.spareParam[p.cls(buf)] = j
				}
			}
		}
	})
	p.pl.CountUses()
}

// schedule orders a loop body's back-edge updates so each can be in place
// (windows-target.md §5 item 3). A `continue` is a parallel assignment
// p_j ← e_j(p), and x86's destructive `add` computes p_j + k in place only if
// nothing reads p_j afterwards. So an in-place candidate v = p_j ⊕ k whose value
// is the continue's argument j moves after the last statement that reads p_j,
// when none of those statements reads v. That is L6: the statement is pure
// arithmetic, which reads no memory, so it commutes with every statement it
// passes, stores included.
func (p *printer) schedule(r *ir.Region, loop *ir.Stmt) {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		for _, sub := range s.Sub {
			if s.Op == ir.OLoop {
				p.schedule(sub, s)
			} else {
				p.schedule(sub, loop)
			}
		}
	}
	if r.T == ir.TBranch {
		p.schedule(r.Then, loop)
		p.schedule(r.Else, loop)
		return
	}
	if r.T != ir.TContinue || loop == nil {
		return
	}
	params := loop.Sub[0].Params
	for j, a := range r.Args {
		if j >= len(params) {
			break
		}
		b := -1
		for i := range r.Stmts {
			if s := &r.Stmts[i]; len(s.Res) == 1 && s.Res[0] == a {
				b = i
			}
		}
		if b < 0 || !p.movable(&r.Stmts[b]) || p.pl.Res(r.Stmts[b].Args[0]) != p.pl.Res(params[j]) {
			continue
		}
		last := -1
		for i := b + 1; i < len(r.Stmts); i++ {
			if reads(&r.Stmts[i], p.pl, params[j]) {
				last = i
			}
		}
		if last < 0 {
			continue
		}
		clash := false
		for i := b + 1; i <= last; i++ {
			if reads(&r.Stmts[i], p.pl, a) {
				clash = true
			}
		}
		if clash {
			continue
		}
		moved := r.Stmts[b]
		copy(r.Stmts[b:last], r.Stmts[b+1:last+1])
		r.Stmts[last] = moved
	}
}

// movable: pure arithmetic with an in-place template, which may be delayed.
func (p *printer) movable(s *ir.Stmt) bool {
	switch s.Op {
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg:
		return s.Mode != ir.MTrap && len(s.Args) > 0 && p.inPlace(s)
	}
	return false
}

// reads reports whether s reads v, in its operands or anywhere in its regions.
func reads(s *ir.Stmt, pl *plan.Plan, v ir.V) bool {
	v = pl.Res(v)
	for _, a := range s.Args {
		if pl.Res(a) == v {
			return true
		}
	}
	for _, sub := range s.Sub {
		if regionReads(sub, pl, v) {
			return true
		}
	}
	return false
}

func regionReads(r *ir.Region, pl *plan.Plan, v ir.V) bool {
	for _, pi := range r.Pis {
		if pl.Res(pi.Of) == v || pl.Res(pi.Other) == v {
			return true
		}
	}
	for i := range r.Stmts {
		if reads(&r.Stmts[i], pl, v) {
			return true
		}
	}
	for _, a := range r.Args {
		if pl.Res(a) == v {
			return true
		}
	}
	if r.T == ir.TBranch {
		return pl.Res(r.Cond) == v || regionReads(r.Then, pl, v) || regionReads(r.Else, pl, v)
	}
	return false
}

// fuse decides which booleans print as jumps: read once, as the condition of
// the branch or `if` immediately after the definition, and a comparison with a
// jump form or a pure `if` of booleans (§9.6).
func (p *printer) fuse(r *ir.Region) {
	var prev *ir.Stmt
	consider := func(c ir.V) {
		if prev == nil || len(prev.Res) != 1 || p.pl.Res(prev.Res[0]) != p.pl.Res(c) || p.pl.Uses[p.pl.Res(c)] != 1 {
			return
		}
		if p.jumpable(prev) {
			p.fused[p.pl.Res(c)] = true
		}
	}
	for i := range r.Stmts {
		s := &r.Stmts[i]
		if !p.pl.Live(s) {
			continue
		}
		if s.Op == ir.OIf {
			consider(s.Args[0])
		}
		for _, sub := range s.Sub {
			p.fuse(sub)
		}
		prev = s
	}
	if r.T == ir.TBranch {
		consider(r.Cond)
		p.fuse(r.Then)
		p.fuse(r.Else)
	}
}

// fuseArms fuses the booleans a fused `if`'s arms yield: each arm is printed
// as jumps (regionJump), so a yielded comparison defined just before its yield
// is its jumps too. Only under a fused `if`: an `if` printed as a value needs
// its arms to yield values.
func (p *printer) fuseArms(r *ir.Region) {
	var walk func(r *ir.Region)
	walk = func(r *ir.Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == ir.OIf && len(s.Res) == 1 && p.fused[p.pl.Res(s.Res[0])] {
				p.fuseLeaves(s.Sub[0])
				p.fuseLeaves(s.Sub[1])
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		if r.T == ir.TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
}

func (p *printer) fuseLeaves(r *ir.Region) {
	switch r.T {
	case ir.TBranch:
		p.fuseLeaves(r.Then)
		p.fuseLeaves(r.Else)
	case ir.TYield:
		if len(r.Args) != 1 {
			return
		}
		c := p.pl.Res(r.Args[0])
		var last *ir.Stmt
		for i := range r.Stmts {
			if p.pl.Live(&r.Stmts[i]) {
				last = &r.Stmts[i]
			}
		}
		if last == nil || len(last.Res) != 1 || p.pl.Res(last.Res[0]) != c || p.pl.Uses[c] != 1 || !p.jumpable(last) {
			return
		}
		p.fused[c] = true
		if last.Op == ir.OIf {
			p.fuseLeaves(last.Sub[0])
			p.fuseLeaves(last.Sub[1])
		}
	}
}

func (p *printer) jumpable(s *ir.Stmt) bool {
	switch {
	case s.Op.IsCmp():
		return p.cmpPrim(s.Op) != nil
	case s.Op == ir.OCall:
		q := p.tg.Prims[s.Name]
		return q.Jump != "" && q.Pure && len(s.Args) == 2
	case s.Op == ir.OIf:
		return len(s.Res) == 1 && p.typeOf(s.Res[0]) == "bool" &&
			p.pl.PureRegion(s.Sub[0]) && p.pl.PureRegion(s.Sub[1]) &&
			boolLeaves(s.Sub[0]) && boolLeaves(s.Sub[1])
	}
	return false
}

// boolLeaves: every exit of r is a yield (the arms of a boolean `if`).
func boolLeaves(r *ir.Region) bool {
	switch r.T {
	case ir.TYield:
		return len(r.Args) == 1
	case ir.TBranch:
		return boolLeaves(r.Then) && boolLeaves(r.Else)
	}
	return false
}

// cmpPrim is the target's primitive for a comparison, with its jump form: the
// one the spelling table chose.
func (p *printer) cmpPrim(o ir.Op) *emit.Prim {
	form := p.pl.Spell[o][0]
	var names []string
	for n := range p.tg.Prims {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		q := p.tg.Prims[n]
		if q.Form == form && q.Jump != "" {
			return &q
		}
	}
	return nil
}

// hint records the register preferences that save moves: a continue's
// argument would like its parameter's register, a parameter its initial
// value's, and an in-place template's result its first operand's.
func (p *printer) hint(r *ir.Region) {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		if len(s.Res) == 1 && len(s.Args) > 0 && p.inPlace(s) {
			p.inPlaceOf[p.cls(s.Res[0])] = s.Args[0]
		}
		if s.Op == ir.OLoop {
			body := s.Sub[0]
			for j, q := range body.Params {
				p.hints[p.cls(q)] = append(p.hints[p.cls(q)], s.Args[j])
				for _, c := range plan.Exits(body, ir.TContinue) {
					if j < len(c) {
						p.hints[p.cls(c[j])] = append(p.hints[p.cls(c[j])], q)
					}
				}
			}
		}
		for _, sub := range s.Sub {
			p.hint(sub)
		}
	}
	if r.T == ir.TBranch {
		p.hint(r.Then)
		p.hint(r.Else)
	}
}

// inPlace: the statement's template begins `mov %r, %1` and names %1 nowhere
// else, so its result may take %1's register when %1 dies there.
func (p *printer) inPlace(s *ir.Stmt) bool {
	form := p.formOf(s)
	rest, ok := strings.CutPrefix(form, "mov %r, %1\n")
	return ok && !strings.Contains(rest, "%1") && !strings.Contains(rest, "%s")
}

// formOf is a statement's template, or "".
func (p *printer) formOf(s *ir.Stmt) string {
	switch {
	case s.Op.IsArith() || s.Op.IsCmp():
		f := p.pl.Spell[s.Op]
		if s.Mode == ir.MTrap {
			return f[1]
		}
		return f[0]
	case s.Op == ir.OCall:
		q := p.tg.Prims[s.Name]
		if q.Kind == "expr" {
			return q.Form
		}
	}
	return ""
}

// ---------------------------------------------------------------- operands

// opnd is a value as an instruction operand: its place, or an immediate.
// A literal the instruction cannot take directly (a wide integer, a string, a
// float) is "", and materialise loads it.
func (p *printer) opnd(v ir.V) string {
	c := p.cls(v)
	if d := p.pl.Const[c]; d != nil && int(c) < p.f.NV() {
		switch d.Kind {
		case core.KInt:
			if d.Int >= math.MinInt32 && d.Int <= math.MaxInt32 {
				return strconv.FormatInt(d.Int, 10)
			}
		case core.KBool:
			if d.IsTrue() {
				return "1"
			}
			return "0"
		}
		return ""
	}
	return p.text(p.loc[c])
}

// load writes an instruction that puts v into register r.
func (p *printer) load(r string, v ir.V) {
	c := p.cls(v)
	if d := p.pl.Const[c]; d != nil && int(c) < p.f.NV() {
		switch d.Kind {
		case core.KInt:
			p.line("mov %s, %d", r, d.Int)
		case core.KBool:
			if d.IsTrue() {
				p.line("mov %s, 1", r)
			} else {
				p.line("xor %s, %s", dword(r), dword(r))
			}
		case core.KStr:
			p.line("lea %s, %s", r, emit.AsmStringLit(d.Str))
		case core.KFloat:
			if strings.HasPrefix(r, "xmm") {
				p.line("movsd %s, %s", r, emit.AsmFloatLit(d.Float))
			} else {
				p.line("mov %s, qword ptr %s", r, emit.AsmFloatLit(d.Float))
			}
		default:
			p.fail("literal %s has no x86 spelling", d)
		}
		return
	}
	src := p.text(p.loc[c])
	if src == r {
		return
	}
	if strings.HasPrefix(r, "xmm") || strings.HasPrefix(src, "xmm") {
		p.line("movsd %s, %s", r, src)
		return
	}
	p.line("mov %s, %s", r, src)
}

func dword(r string) string {
	switch r {
	case "rax", "rbx", "rcx", "rdx", "rsi", "rdi":
		return "e" + r[1:]
	}
	return r + "d"
}

// operands spells a template's operands: registers and small immediates as
// they are, and slot operands and literals the instruction cannot take loaded
// into scratch. Beyond the two scratch registers an operand BORROWS a value
// register (emit/asm.go's borrowValueReg): one holding none of this
// instruction's operands and not its destination, saved to a frame slot for
// the one template and restored after it. A value register is callee-saved, so
// a template that makes a call gets it back, and a slot rather than a push
// keeps rsp's alignment for any callee. restore puts the borrowed ones back.
func (p *printer) operands(args []ir.V, gp []string, dst string) (out []string, restore func()) {
	out = make([]string, len(args))
	taken := map[string]bool{dst: true}
	for _, a := range args {
		if o := p.opnd(a); o != "" && !isMem(o) {
			taken[o] = true
		}
	}
	var borrowed [][2]string // register, slot
	k := 0
	for i, a := range args {
		o := p.opnd(a)
		if o != "" && !isMem(o) {
			out[i] = o
			continue
		}
		if p.isFloat(a) {
			p.load(scratchX, a)
			out[i] = scratchX
			continue
		}
		if k < len(gp) {
			p.load(gp[k], a)
			out[i] = gp[k]
			k++
			continue
		}
		r := ""
		for _, v := range valueGP {
			if p.usedGP[v] && !taken[v] {
				r = v
				break
			}
		}
		if r == "" {
			p.fail("a template has more slot operands than scratch registers and no value register to borrow")
			out[i] = o
			continue
		}
		slot := p.text(loc{slot: p.borrowSlot(len(borrowed))})
		p.line("mov %s, %s", slot, r)
		p.load(r, a)
		out[i] = r
		taken[r] = true
		borrowed = append(borrowed, [2]string{r, slot})
	}
	return out, func() {
		for j := len(borrowed) - 1; j >= 0; j-- {
			p.line("mov %s, %s", borrowed[j][0], borrowed[j][1])
		}
	}
}

// borrowSlot is the frame slot the i-th borrowed register of one instruction
// is saved in, allocated past the allocation's own slots.
func (p *printer) borrowSlot(i int) int {
	for len(p.borrow) <= i {
		p.slots++
		p.borrow = append(p.borrow, p.slots)
	}
	return p.borrow[i]
}

// ---------------------------------------------------------------- function

func (p *printer) function() (string, error) {
	if len(p.f.Params) > len(argGP) {
		return "", fmt.Errorf("takes %d arguments; the Win64 convention passes four in registers and this printer does not read the fifth off the stack", len(p.f.Params))
	}
	p.schedule(p.f.Body, nil)
	p.decide()
	p.fuse(p.f.Body)
	p.fuseArms(p.f.Body)
	p.hint(p.f.Body)
	p.loc = make([]loc, p.f.NV())
	p.shadow = p.shadowFor()
	if err := p.allocate(); err != nil {
		return "", err
	}
	// Parameters arrive in rcx, rdx, r8, r9 (xmm0–xmm3 for a float), which no
	// value register is, so the entry moves are ordered freely.
	for i, v := range p.f.Params {
		if !p.pl.Read(v) {
			continue
		}
		dst := p.text(p.loc[p.cls(v)])
		if p.isFloat(v) {
			p.line("movsd %s, %s", dst, argX[i])
		} else {
			p.line("mov %s, %s", dst, argGP[i])
		}
	}
	p.ret = p.newLabel("Lret")
	p.owner = []owner{{fn: true, done: p.ret}}
	p.region(p.f.Body)
	p.label(p.ret)
	if p.err != nil {
		return "", p.err
	}
	return p.frame(), nil
}

// shadowFor is the outgoing-argument area: the floor every hand-written
// template was authored against, or room for the widest call (emit/asm.go's
// asmShadowFor, read off the IR).
func (p *printer) shadowFor() int {
	widest := 0
	p.f.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == ir.OCall {
				if n := len(p.tg.Prims[s.Name].Args); n > widest {
					widest = n
				}
			}
		}
	})
	n := emit.AsmShadowFloor
	if widest > 4 {
		if want := 32 + 8*(widest-4); want > n {
			n = want
		}
	}
	return (n + 15) / 16 * 16
}

// frame wraps the body in the prologue and epilogue, written last because only
// now is it known which callee-saved registers were used and how many slots.
func (p *printer) frame() string {
	var saved, savedX []string
	for _, r := range valueGP {
		if p.usedGP[r] {
			saved = append(saved, r)
		}
	}
	for _, r := range valueX {
		if p.usedX[r] {
			savedX = append(savedX, r)
		}
	}
	frame := p.shadow + 8*(p.slots+len(savedX))
	// rsp is 8 mod 16 on entry; the frame brings it back to 0 mod 16, or a
	// callee's aligned spill faults inside kernel32.
	for ((8-8*len(saved)-frame)%16+16)%16 != 0 {
		frame += 8
	}
	xbase := p.shadow + 8*p.slots
	m := emit.AsmMangle(p.f.Name)
	var out strings.Builder
	fmt.Fprintf(&out, "%s proc\n", m)
	for _, r := range saved {
		fmt.Fprintf(&out, "        push %s\n", r)
	}
	fmt.Fprintf(&out, "        sub rsp, %d\n", frame)
	for i, r := range savedX {
		fmt.Fprintf(&out, "        movsd qword ptr [rsp+%d], %s\n", xbase+8*i, r)
	}
	out.WriteString(threadJumps(emit.AsmPeephole(threadJumps(p.b.String()))))
	for i, r := range savedX {
		fmt.Fprintf(&out, "        movsd %s, qword ptr [rsp+%d]\n", r, xbase+8*i)
	}
	fmt.Fprintf(&out, "        add rsp, %d\n", frame)
	for i := len(saved) - 1; i >= 0; i-- {
		fmt.Fprintf(&out, "        pop %s\n", saved[i])
	}
	out.WriteString("        ret\n")
	fmt.Fprintf(&out, "%s endp\n", m)
	return out.String()
}

// ---------------------------------------------------------------- regions

func (p *printer) region(r *ir.Region) {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		if !p.pl.Live(s) || (len(s.Res) == 1 && p.fused[p.pl.Res(s.Res[0])]) {
			continue
		}
		p.stmt(s)
	}
	switch r.T {
	case ir.TYield:
		o := p.owner[len(p.owner)-1]
		switch {
		case o.fn:
			p.returns(r.Args)
		case o.store:
			p.storeAt(o.tab, o.idx, r.Args[0])
		default:
			p.moves(o.res, r.Args)
		}
		p.line("jmp %s", o.done)
	case ir.TBreak:
		l := p.loops[len(p.loops)-1]
		p.moves(l.Res, r.Args)
		p.line("jmp %s", p.exitOf[l])
	case ir.TContinue:
		l := p.loops[len(p.loops)-1]
		dst := append([]ir.V(nil), l.Sub[0].Params...)
		src := append([]ir.V(nil), r.Args...)
		// The spare swap (Rule R): parameter j takes the new buffer, which the
		// back-edge build wrote into the spare, and the spare takes the old
		// one, which nothing else carries (Rule R's condition 3).
		for _, sp := range p.spareOf[l] {
			dst = append(dst, sp)
			src = append(src, l.Sub[0].Params[p.spareParam[p.cls(sp)]])
		}
		p.moves(dst, src)
		p.line("jmp %s", p.topOf[l])
	case ir.TBranch:
		els := p.newLabel("Lelse")
		p.jumpFalse(r.Cond, els)
		p.region(r.Then)
		p.label(els)
		p.region(r.Else)
	}
}

// returns places a function's results in the convention registers: rax (xmm0)
// for one, and rax, rdx, r8, r9 for several, each class in declaration order.
// Two passes are unnecessary here: every result is already in its own place.
func (p *printer) returns(args []ir.V) {
	var dst []string
	var src []ir.V
	gp, xm := 0, 0
	for _, a := range args {
		if p.isFloat(a) {
			if xm >= len(retX) {
				p.fail("returns more float results than the convention carries")
				return
			}
			dst = append(dst, retX[xm])
			xm++
		} else {
			if gp >= len(retGP) {
				p.fail("returns more integer results than the convention carries")
				return
			}
			dst = append(dst, retGP[gp])
			gp++
		}
		src = append(src, a)
	}
	p.moveTo(dst, src)
}
