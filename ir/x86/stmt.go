package x86

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/plan"
)

// ---------------------------------------------------------------- statements

func (p *printer) stmt(s *ir.Stmt) {
	switch s.Op {
	case ir.OConst, ir.OAssume:
		// A literal is an operand where it is read; an assumption is a fact.
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg, ir.ODiv, ir.ORem,
		ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		form := p.formOf(s)
		if form == "" {
			p.fail("the target has no spelling for (%s %s)", s.Op, s.Mode)
			return
		}
		p.template(form, "expr", s.Res, s.Args)
	case ir.OCall:
		q, ok := p.tg.Prims[s.Name]
		if !ok || (q.Kind != "expr" && q.Kind != "stmt") {
			p.fail("(call %s) has no template on x86", s.Name)
			return
		}
		if len(s.Res) > 1 {
			p.fail("(call %s) has several results, which this printer does not place", s.Name)
			return
		}
		if q.Import != "" {
			emit.AsmExterns[q.Import] = true
			if q.Lib != "" {
				emit.AsmLibs[q.Lib] = true
			}
		}
		p.template(q.Form, q.Kind, s.Res, s.Args)
	case ir.OIndex:
		p.index(s)
	case ir.OLen:
		// A table's length is its header. A string here is a bare pointer to
		// NUL-terminated bytes (windows.oro), so its length is the target's
		// own template, the one lowering classified as `len` (`x64.strlen`).
		if q := p.stringLen(s.Args[0]); q != nil {
			p.template(q.Form, q.Kind, s.Res, s.Args)
			return
		}
		base := p.inReg(s.Args[0], scratchA)
		p.into(s.Res[0], func(dst string) { p.line("mov %s, qword ptr [%s]", dst, base) })
	case ir.OSet:
		p.store(s)
	case ir.OIf:
		p.ifStmt(s)
	case ir.OLoop:
		p.loop(s)
	case ir.OBuild:
		p.build(s)
	case ir.OArray:
		p.arrayLit(s)
	case ir.OTabulate:
		p.tabulate(s)
	default:
		p.fail("(%s …) is not printed on x86", s.Op)
	}
}

// template expands one target template. A statement's value is its first
// argument, on every target; an expression's result goes in its place, or in
// rax (xmm5) and then its slot.
func (p *printer) template(form, kind string, res, args []ir.V) {
	var dst, slot string
	if kind != "stmt" {
		dst = "rax"
		if len(res) == 1 && p.isFloat(res[0]) {
			dst = resultX
		}
		if len(res) == 1 && p.pl.Read(res[0]) {
			if t := p.text(p.loc[p.cls(res[0])]); t != "" {
				if isMem(t) {
					slot = t
				} else {
					dst = t
				}
			}
		}
	}
	ops, restore := p.operands(args, []string{scratchA, scratchB}, dst)
	if kind == "stmt" {
		if len(ops) == 0 {
			p.fail("a statement template with no arguments")
			return
		}
		dst = ops[0]
	}
	body, err := emit.FillAsm(form, dst, ops, emit.AsmUniq())
	if err != nil {
		p.fail("%v", err)
		return
	}
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || selfMove(l) {
			continue // `mov rbx, rbx`: the in-place exception's first line
		}
		if strings.Count(l, "[") > 1 {
			p.fail("a template names memory twice after placing its operands: %s", l)
		}
		p.line("%s", l)
	}
	restore()
	if slot != "" {
		if strings.HasPrefix(dst, "xmm") {
			p.line("movsd %s, %s", slot, dst)
		} else {
			p.line("mov %s, %s", slot, dst)
		}
	}
}

func selfMove(l string) bool {
	for _, op := range []string{"mov ", "movsd "} {
		if rest, ok := strings.CutPrefix(l, op); ok {
			a, b, ok := strings.Cut(rest, ",")
			return ok && strings.TrimSpace(a) == strings.TrimSpace(b)
		}
	}
	return false
}

// inReg is v in a register: its own, or the scratch it is loaded into.
func (p *printer) inReg(v ir.V, scratch string) string {
	if o := p.opnd(v); o != "" && !isMem(o) && !isImm(o) {
		return o
	}
	p.load(scratch, v)
	return scratch
}

func isImm(s string) bool {
	return s != "" && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9'))
}

// into writes an instruction whose destination is v's place: its register,
// or rax (xmm5) and then its slot.
func (p *printer) into(v ir.V, emitTo func(dst string)) {
	if !p.pl.Read(v) {
		return
	}
	t := p.text(p.loc[p.cls(v)])
	if t != "" && !isMem(t) {
		emitTo(t)
		return
	}
	r := "rax"
	if p.isFloat(v) {
		r = resultX
	}
	emitTo(r)
	if t != "" {
		if r == resultX {
			p.line("movsd %s, %s", t, r)
		} else {
			p.line("mov %s, %s", t, r)
		}
	}
}

// index is a scaled load past the eight-byte length header: movzx for a byte
// element, which zero-extends, and a qword load otherwise.
func (p *printer) index(s *ir.Stmt) {
	tab, i := s.Args[0], s.Args[1]
	base := p.inReg(tab, scratchA)
	idx := p.opnd(i)
	if idx == "" || isMem(idx) {
		idx = p.inReg(i, scratchB)
	}
	w := p.width(tab)
	addr := emit.AsmElemAddr(base, idx, w)
	p.into(s.Res[0], func(dst string) {
		switch {
		case w == 1:
			p.line("movzx %s, byte ptr %s", dword(dst), addr)
		case strings.HasPrefix(dst, "xmm"):
			p.line("movsd %s, qword ptr %s", dst, addr)
		default:
			p.line("mov %s, qword ptr %s", dst, addr)
		}
	})
}

// store writes one element. Its value is the buffer (an alias).
func (p *printer) store(s *ir.Stmt) {
	b, i, v := s.Args[0], s.Args[1], s.Args[2]
	base := p.inReg(b, scratchA)
	idx := p.opnd(i)
	if idx == "" || isMem(idx) {
		idx = p.inReg(i, scratchB)
	}
	w := p.width(b)
	addr := emit.AsmElemAddr(base, idx, w)
	val := p.opnd(v)
	switch {
	case p.isFloat(v):
		if val == "" || isMem(val) {
			p.load(scratchX, v)
			val = scratchX
		}
		p.line("movsd qword ptr %s, %s", addr, val)
		return
	case val == "" || isMem(val):
		p.load("rax", v)
		val = "rax"
	}
	if w == 1 {
		if !isImm(val) {
			val = emit.AsmByte(val)
		}
		p.line("mov byte ptr %s, %s", addr, val)
		return
	}
	p.line("mov qword ptr %s, %s", addr, val)
}

// tableOf allocates a table of n elements into dst's place and writes its
// length header. VirtualAlloc returns zeroed pages, so this is `build`'s zero
// fill (tables.md §14.3).
func (p *printer) tableOf(dst ir.V, n func(r string), nOpnd func() string, w int) {
	q, ok := p.tg.FindAlloc()
	if !ok {
		p.fail("this target declares no allocator, so `build` has no memory to use (docs/spec/tables.md §10)")
		return
	}
	if q.Import != "" {
		emit.AsmExterns[q.Import] = true
		if q.Lib != "" {
			emit.AsmLibs[q.Lib] = true
		}
	}
	n(scratchA)
	if w == 1 {
		p.line("add %s, %d", scratchA, emit.AsmTableHeader)
	} else {
		p.line("add %s, 1", scratchA)
		p.line("shl %s, 3", scratchA)
	}
	t := p.text(p.loc[p.cls(dst)])
	r := t
	if t == "" || isMem(t) {
		r = "rax"
	}
	body, err := emit.FillAsm(q.Form, r, []string{scratchA}, emit.AsmUniq())
	if err != nil {
		p.fail("alloc: %v", err)
		return
	}
	for _, l := range strings.Split(body, "\n") {
		if l = strings.TrimSpace(l); l != "" && !selfMove(l) {
			p.line("%s", l)
		}
	}
	if r == "rax" && t != "" {
		p.line("mov %s, rax", t)
	}
	// The call clobbered the scratch registers, so the length is read again.
	nv := nOpnd()
	if nv == "" || isMem(nv) {
		n(scratchB)
		nv = scratchB
	}
	p.line("mov qword ptr [%s], %s", r, nv)
}

func (p *printer) build(s *ir.Stmt) {
	buf := s.Sub[0].Params[0]
	w := p.width(buf)
	if p.spareFor[s] == buf {
		// A back-edge build writes into its loop's spare, which it clears:
		// the zero fill a fresh table has for free.
		p.clear(buf, p.spareLen[p.cls(buf)], w)
	} else {
		n := s.Args[0]
		p.tableOf(buf, func(r string) { p.load(r, n) }, func() string { return p.opnd(n) }, w)
	}
	done := p.newLabel("Lbuilt")
	p.owner = append(p.owner, owner{res: s.Res, done: done})
	p.region(s.Sub[0])
	p.owner = p.owner[:len(p.owner)-1]
	p.label(done)
}

// clear zeroes a reused table: a counted loop over its literal length.
func (p *printer) clear(buf ir.V, n int64, w int) {
	base := p.inReg(buf, scratchA)
	u := emit.AsmUniq()
	p.line("xor %s, %s", dword(scratchB), dword(scratchB))
	p.label(fmt.Sprintf("Lzc%d", u))
	p.line("cmp %s, %d", scratchB, n)
	p.line("jge Lzd%d", u)
	if w == 1 {
		p.line("mov byte ptr %s, 0", emit.AsmElemAddr(base, scratchB, 1))
	} else {
		p.line("mov qword ptr %s, 0", emit.AsmElemAddr(base, scratchB, 8))
	}
	p.line("inc %s", scratchB)
	p.line("jmp Lzc%d", u)
	p.label(fmt.Sprintf("Lzd%d", u))
}

// arrayLit builds a graph in memory: a table, then one store per element.
func (p *printer) arrayLit(s *ir.Stmt) {
	res := s.Res[0]
	n := int64(len(s.Args))
	w := p.width(res)
	p.tableOf(res, func(r string) { p.line("mov %s, %d", r, n) }, func() string { return fmt.Sprint(n) }, w)
	for i, a := range s.Args {
		base := p.inReg(res, scratchA)
		val := p.opnd(a)
		if val == "" || isMem(val) {
			p.load("rax", a)
			val = "rax"
		}
		if w == 1 {
			if !isImm(val) {
				val = emit.AsmByte(val)
			}
			p.line("mov byte ptr [%s+%d], %s", base, emit.AsmTableHeader+i, val)
		} else {
			p.line("mov qword ptr [%s+%d], %s", base, emit.AsmTableHeader+8*i, val)
		}
	}
}

// tabulate fills a fresh table by its rule: a counted loop whose body's
// yield is stored at the index (L11).
func (p *printer) tabulate(s *ir.Stmt) {
	res, n, i := s.Res[0], s.Args[0], s.Sub[0].Params[0]
	w := p.width(res)
	p.tableOf(res, func(r string) { p.load(r, n) }, func() string { return p.opnd(n) }, w)
	idx := p.text(p.loc[p.cls(i)])
	p.line("mov %s, 0", idx)
	top, next, done := p.newLabel("Ltab"), p.newLabel("Ltabn"), p.newLabel("Ltabd")
	p.label(top)
	ri := idx
	if isMem(ri) {
		p.line("mov rax, %s", idx)
		ri = "rax"
	}
	bound := p.opnd(n)
	if bound == "" || (isMem(bound) && isMem(ri)) {
		p.load(scratchB, n)
		bound = scratchB
	}
	p.line("cmp %s, %s", ri, bound)
	p.line("jge %s", done)
	p.owner = append(p.owner, owner{done: next, store: true, tab: res, idx: i})
	p.region(s.Sub[0])
	p.owner = p.owner[:len(p.owner)-1]
	p.label(next)
	p.line("add %s, 1", idx)
	p.line("jmp %s", top)
	p.label(done)
}

// storeAt writes v into element i of table t: store's addressing, for a
// tabulate's yield.
func (p *printer) storeAt(t, i, v ir.V) {
	p.store(&ir.Stmt{Op: ir.OSet, Args: []ir.V{t, i, v}})
}

// ---------------------------------------------------------------- control

func (p *printer) loop(s *ir.Stmt) {
	for _, sp := range p.spareOf[s] {
		n := p.spareLen[p.cls(sp)]
		p.tableOf(sp, func(r string) { p.line("mov %s, %d", r, n) }, func() string { return fmt.Sprint(n) }, p.width(sp))
	}
	p.moves(s.Sub[0].Params, s.Args)
	top, exit := p.newLabel("Ltop"), p.newLabel("Lexit")
	p.topOf[s], p.exitOf[s] = top, exit
	p.label(top)
	p.loops = append(p.loops, s)
	p.region(s.Sub[0])
	p.loops = p.loops[:len(p.loops)-1]
	p.label(exit)
}

func (p *printer) ifStmt(s *ir.Stmt) {
	els, done := p.newLabel("Lelse"), p.newLabel("Ldone")
	p.jumpFalse(s.Args[0], els)
	p.owner = append(p.owner, owner{res: s.Res, done: done})
	p.region(s.Sub[0])
	p.label(els)
	p.region(s.Sub[1])
	p.owner = p.owner[:len(p.owner)-1]
	p.label(done)
}

// jumpFalse jumps to l when c is false, and jumpTrue when it is true. A fused
// boolean is its jumps (§9.6): a comparison is the flag-setting instruction and
// one conditional jump, and a boolean `if` jumps to the two continuations.
func (p *printer) jumpFalse(c ir.V, l string) { p.jump(c, l, false) }
func (p *printer) jumpTrue(c ir.V, l string)  { p.jump(c, l, true) }

func (p *printer) jump(c ir.V, l string, when bool) {
	r := p.cls(c)
	if d := p.pl.Const[r]; d != nil && d.Kind == core.KBool {
		if d.IsTrue() == when {
			p.line("jmp %s", l)
		}
		return
	}
	if p.fused[r] {
		d := p.defs[r]
		switch {
		case d.Op == ir.OIf:
			p.jumpIf(d, l, when)
		case d.Op.IsCmp():
			p.compare(p.cmpPrim(d.Op), d.Args, l, when)
		default:
			q := p.tg.Prims[d.Name]
			p.compare(&q, d.Args, l, when)
		}
		return
	}
	x := p.opnd(c)
	if x == "" || isImm(x) {
		p.load("rax", c)
		x = "rax"
	}
	p.line("cmp %s, 0", x)
	if when {
		p.line("jne %s", l)
	} else {
		p.line("je %s", l)
	}
}

// compare is the flag-setting instruction of a jump form, then j<cc>: cmp
// for integers and comisd for floats, unless the target wrote its own.
func (p *printer) compare(q *emit.Prim, args []ir.V, l string, when bool) {
	cc := q.Jump
	if !when {
		cc = emit.AsmNegate(cc)
	}
	if q.JumpForm != "" {
		ops, restore := p.operands(args, []string{scratchA, scratchB}, "")
		body, err := emit.FillAsm(q.JumpForm, "", ops, emit.AsmUniq())
		if err != nil {
			p.fail("%v", err)
			return
		}
		for _, x := range strings.Split(body, "\n") {
			if x = strings.TrimSpace(x); x != "" {
				p.line("%s", x)
			}
		}
		restore() // a mov leaves the flags alone
		p.line("j%s %s", cc, l)
		return
	}
	a, b := args[0], args[1]
	if p.isFloat(a) {
		ra := p.opnd(a)
		if ra == "" || isMem(ra) {
			p.load(scratchX, a)
			ra = scratchX
		}
		rb := p.opnd(b)
		if rb == "" {
			p.load(resultX, b)
			rb = resultX
		}
		p.line("comisd %s, %s", ra, rb)
	} else {
		ra := p.opnd(a)
		if ra == "" || isMem(ra) || isImm(ra) {
			p.load("rax", a)
			ra = "rax"
		}
		rb := p.opnd(b)
		if rb == "" {
			p.load(scratchB, b)
			rb = scratchB
		}
		p.line("cmp %s, %s", ra, rb)
	}
	p.line("j%s %s", cc, l)
}

// jumpIf is a fused boolean `if` as jumping code: its condition chooses an
// arm, and the arm's yielded boolean jumps to l or falls through to done.
func (p *printer) jumpIf(d *ir.Stmt, l string, when bool) {
	els, done := p.newLabel("Lje"), p.newLabel("Ljd")
	p.jumpFalse(d.Args[0], els)
	p.regionJump(d.Sub[0], l, when, done)
	p.label(els)
	p.regionJump(d.Sub[1], l, when, done)
	p.label(done)
}

func (p *printer) regionJump(r *ir.Region, l string, when bool, done string) {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		if !p.pl.Live(s) || (len(s.Res) == 1 && p.fused[p.pl.Res(s.Res[0])]) {
			continue
		}
		p.stmt(s)
	}
	switch r.T {
	case ir.TYield:
		p.jump(r.Args[0], l, when)
		p.line("jmp %s", done)
	case ir.TBranch:
		els := p.newLabel("Lje")
		p.jumpFalse(r.Cond, els)
		p.regionJump(r.Then, l, when, done)
		p.label(els)
		p.regionJump(r.Else, l, when, done)
	default:
		p.fail("a boolean `if` arm ends in %s", r.T)
	}
}

// ---------------------------------------------------------------- moves

// moves is the parallel copy dst ← src (§9.3) into values' places. A place
// nothing reads is not written.
func (p *printer) moves(dst, src []ir.V) {
	var d []string
	var s []ir.V
	for j, v := range dst {
		if j >= len(src) || !p.pl.Read(v) {
			continue
		}
		t := p.text(p.loc[p.cls(v)])
		if t == "" {
			continue
		}
		d = append(d, t)
		s = append(s, src[j])
	}
	p.moveTo(d, s)
}

// moveTo is the parallel copy into places spelled as text (a register, a slot,
// or a convention register), sequentialised by plan.Moves over tokens, so the
// read-set logic never sees operand syntax. One temporary per cycle: r10 then
// r11, or xmm4 then xmm5; a memory-to-memory copy goes through rax.
func (p *printer) moveTo(dst []string, src []ir.V) {
	for _, float := range []bool{false, true} {
		tok := map[string]string{} // place text → token
		back := map[string]string{}
		lit := map[string]ir.V{} // token → a literal source
		name := func(t string) string {
			if k, ok := tok[t]; ok {
				return k
			}
			k := fmt.Sprintf("P%d", len(tok))
			tok[t], back[k] = k, t
			return k
		}
		var ds, ss []string
		for j := range dst {
			if p.isFloat(src[j]) != float {
				continue
			}
			o := p.opnd(src[j])
			var sk string
			if o == "" || isImm(o) {
				sk = fmt.Sprintf("K%d", len(lit))
				lit[sk] = src[j]
			} else {
				sk = name(o)
			}
			dk := name(dst[j])
			if dk == sk {
				continue
			}
			ds, ss = append(ds, dk), append(ss, sk)
		}
		temps := []string{scratchA, scratchB}
		if float {
			temps = []string{scratchX, resultX}
		}
		nt := 0
		fresh := func() string {
			k := fmt.Sprintf("T%d", nt)
			if nt >= len(temps) {
				p.fail("a parallel move needs more than %d temporaries", len(temps))
				back[k] = temps[0]
			} else {
				back[k] = temps[nt]
			}
			nt++
			return k
		}
		for _, m := range plan.Moves(ds, ss, fresh) {
			to := back[m[0]]
			if v, ok := lit[m[1]]; ok {
				p.moveLit(to, v)
				continue
			}
			p.moveOne(to, back[m[1]], float)
		}
	}
}

func (p *printer) moveOne(to, from string, float bool) {
	if to == from {
		return
	}
	if isMem(to) && isMem(from) {
		p.line("mov rax, %s", from)
		p.line("mov %s, rax", to)
		return
	}
	if float {
		p.line("movsd %s, %s", to, from)
		return
	}
	p.line("mov %s, %s", to, from)
}

func (p *printer) moveLit(to string, v ir.V) {
	o := p.opnd(v)
	if isImm(o) && !strings.HasPrefix(to, "xmm") {
		p.line("mov %s, %s", to, o)
		return
	}
	if isMem(to) {
		p.load("rax", v)
		p.line("mov %s, rax", to)
		return
	}
	p.load(to, v)
}

// stringLen is the target's length primitive over a string, when v is one:
// the first, by name, that lowering would classify as `len` (a printer is a
// function of its input).
func (p *printer) stringLen(v ir.V) *emit.Prim {
	if t := p.typeOf(v); t != "string" && p.tg.ValueType(t) != "string" {
		return nil
	}
	var names []string
	for n, q := range p.tg.Prims {
		if len(q.Args) == 1 && q.Kind == "expr" && p.tg.IsLengthName(n) &&
			(q.Args[0] == "string" || p.tg.ValueType(q.Args[0]) == "string") {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		p.fail("the target declares no length for a string")
		return nil
	}
	sort.Strings(names)
	q := p.tg.Prims[names[0]]
	return &q
}
