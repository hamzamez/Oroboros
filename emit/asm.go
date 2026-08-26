package emit

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"oroboros/core"
)

// x86-64 backend, MASM syntax, Win64 calling convention.
//
// The fourth host, and the first one that is NOT AN EXPRESSION LANGUAGE. Go,
// JavaScript and Java all let a target file say `expr "%s + %s"` and mean it:
// the hole is filled with the operand's emitted expression and the host's own
// parser reassembles the tree. Assembly has no tree. `add` takes two operands,
// writes the first, and cannot be nested in anything.
//
// So this backend answers a question the other three could not ask: how much of
// the target-file format was a fact about targets, and how much was a fact
// about hosts that happen to have expressions? The answer was three additions
// and no subtractions — `%r`, `%u`, and `(jump …)` — recorded in
// docs/spec/windows-target.md. Everything else survived: `expr`, `stmt`,
// `(import …)`, purity, refinements, and the three structural kinds.
//
// WHAT A PLACE IS. The other backends return a string that is an expression.
// This one returns a PLACE: a register, an immediate, or a frame slot. That is
// the whole adaptation. `emit` still returns one value and pushes statements
// into a buffer, exactly as emit/golang.go does.
//
// REGISTERS. Values live in the callee-saved registers, which is what makes a
// Win32 call free: kernel32 preserves rbx, rsi, rdi and r12-r15, so nothing has
// to be saved around one. When the pool runs out a value goes to a frame slot
// and is loaded into scratch at each use — the classic fallback, and the only
// part of this backend that is not what a person would write.
//
// REGISTERS A TEMPLATE MAY USE. rax, rcx, rdx, r8, r9 and xmm0-xmm3 are free to
// clobber; that is the Win64 volatile set minus the scratch this backend
// reserves. A template must NOT touch r10, r11, xmm4 or xmm5, which carry
// spilled operands into it, nor anything in the value pools.
var (
	asmValueGP   = []string{"rbx", "rsi", "rdi", "r12", "r13", "r14", "r15"}
	asmValueX    = []string{"xmm6", "xmm7", "xmm8", "xmm9", "xmm10", "xmm11", "xmm12", "xmm13"}
	asmArgGP     = []string{"rcx", "rdx", "r8", "r9"}
	asmArgX      = []string{"xmm0", "xmm1", "xmm2", "xmm3"}
	asmScratchGP = []string{"r10", "r11"}
	asmScratchX  = []string{"xmm4"}
)

// asmShadow is the home space every Win64 caller owes its callee, plus room for
// a fifth and sixth stack argument. WriteFile takes five, and the two extra
// qwords are what let its template write [rsp+20h] without a frame of its own.
const asmShadow = 48

// AsmData and AsmExterns accumulate what emitted procedures need — the same
// package-level sink Imports is on Go. Data holds literals; Externs holds the
// DLL entry points a template named with (import …).
var (
	AsmData    []string
	AsmExterns = map[string]bool{}
	asmUniq    int
	asmLits    = map[string]string{} // literal text -> label, so equals are shared
)

// ResetAsm clears the accumulators. Emitting two programs in one process
// otherwise carries one's literals into the other.
func ResetAsm() {
	AsmData, AsmExterns, asmUniq = nil, map[string]bool{}, 0
	asmLits = map[string]string{}
}

// place is a value's location. At most one of imm / slot is set.
type place struct {
	text  string // "rbx", "42", "qword ptr [rsp+48]"
	xmm   bool
	imm   bool
	slot  int  // >0 when spilled to the frame
	owned bool // this emitter allocated it and may free it
}

func (p place) reg() bool { return !p.imm && p.slot == 0 && p.text != "" }

type asmEmitter struct {
	tgt    *Target
	buf    strings.Builder
	freeGP []string
	freeX  []string
	usedGP map[string]bool
	usedX  map[string]bool
	slots  int
	spare  []int
	bound  map[string]bool
	where  map[string]place

	// elem is how wide one element of the table a NAME holds is, in bytes.
	//
	// This host has no types, so it has to be carried. It is keyed by name
	// rather than by place because a table crosses binders — a `build` buffer
	// is threaded through loop variables and `let`s — and a name is what
	// survives that. Absent means eight, which is what `int` and `f64` need.
	elem map[string]int
}

func newAsmEmitter(tgt *Target) *asmEmitter {
	e := &asmEmitter{
		tgt:    tgt,
		elem:   map[string]int{},
		usedGP: map[string]bool{},
		usedX:  map[string]bool{},
		bound:  map[string]bool{},
		where:  map[string]place{},
	}
	// Reversed, so the first allocation takes rbx and the output reads the way
	// a person writes it.
	for i := len(asmValueGP) - 1; i >= 0; i-- {
		e.freeGP = append(e.freeGP, asmValueGP[i])
	}
	for i := len(asmValueX) - 1; i >= 0; i-- {
		e.freeX = append(e.freeX, asmValueX[i])
	}
	return e
}

func (e *asmEmitter) line(format string, args ...any) {
	e.buf.WriteString("        ")
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *asmEmitter) label(l string) { e.buf.WriteString(l + ":\n") }

func (e *asmEmitter) uniq() int { asmUniq++; return asmUniq }

// ---------------------------------------------------------------- places

func (e *asmEmitter) alloc(xmm bool) place {
	if xmm {
		if n := len(e.freeX); n > 0 {
			r := e.freeX[n-1]
			e.freeX = e.freeX[:n-1]
			e.usedX[r] = true
			return place{text: r, xmm: true, owned: true}
		}
	} else if n := len(e.freeGP); n > 0 {
		r := e.freeGP[n-1]
		e.freeGP = e.freeGP[:n-1]
		e.usedGP[r] = true
		return place{text: r, owned: true}
	}
	return e.allocSlot(xmm)
}

func (e *asmEmitter) allocSlot(xmm bool) place {
	var s int
	if n := len(e.spare); n > 0 {
		s, e.spare = e.spare[n-1], e.spare[:n-1]
	} else {
		e.slots++
		s = e.slots
	}
	// Addressed off rsp, not rbp. rsp does not move inside a procedure — every
	// push is in the prologue — so a slot's offset is known the moment it is
	// handed out, and no frame pointer is needed at all.
	return place{
		text:  fmt.Sprintf("qword ptr [rsp+%d]", asmShadow+8*(s-1)),
		xmm:   xmm,
		slot:  s,
		owned: true,
	}
}

func (e *asmEmitter) release(p place) {
	if !p.owned {
		return
	}
	switch {
	case p.slot > 0:
		e.spare = append(e.spare, p.slot)
	case p.imm || p.text == "":
	case p.xmm:
		e.freeX = append(e.freeX, p.text)
	default:
		e.freeGP = append(e.freeGP, p.text)
	}
}

// hold marks a place as somebody else's, so a consumer will not free it.
func hold(p place) place { p.owned = false; return p }

// move copies src into dst, inserting scratch when x86 forbids two memory
// operands in one instruction.
func (e *asmEmitter) move(dst, src place) {
	if dst.text == src.text || dst.text == "" {
		return
	}
	op := "mov"
	if dst.xmm || src.xmm {
		op = "movsd"
	}
	if dst.slot > 0 && src.slot > 0 {
		s := asmScratchGP[0]
		if dst.xmm {
			s = asmScratchX[0]
		}
		e.line("%s %s, %s", op, s, src.text)
		e.line("%s %s, %s", op, dst.text, s)
		return
	}
	e.line("%s %s, %s", op, dst.text, src.text)
}

// ---------------------------------------------------------------- literals

func asmLabel(stem string) string {
	asmUniq++
	return fmt.Sprintf("%s%d", stem, asmUniq)
}

// asmFloatLit puts a float in the data section AS BITS.
//
// ADR 0009 says staging must not change an answer, and `real8 0.1` hands the
// decision to the assembler's decimal parser. Emitting the IEEE-754 binary64
// bits the reducer computed with takes the assembler out of the question: the
// artifact holds exactly the double the compiler held.
func asmFloatLit(v float64) string {
	key := "f:" + strconv.FormatUint(math.Float64bits(v), 16)
	if l, ok := asmLits[key]; ok {
		return l
	}
	l := asmLabel("LF")
	AsmData = append(AsmData, fmt.Sprintf("%s qword 0%016Xh    ; %g", l, math.Float64bits(v), v))
	asmLits[key] = l
	return l
}

// asmStringLit puts a string in the data section as bytes, NUL-terminated, with
// a `_len` constant beside it.
//
// Byte by byte rather than as a quoted MASM string: MASM has no escapes, so a
// newline or a quote inside a literal cannot be written at all, and UTF-8 above
// ASCII cannot be trusted to the assembler's code page. strings.md §5 reached
// for \u escapes on the other three hosts for the same reason.
func asmStringLit(s string) string {
	key := "s:" + s
	if l, ok := asmLits[key]; ok {
		return l
	}
	l := asmLabel("LS")
	var b []string
	for _, c := range []byte(s) {
		b = append(b, fmt.Sprintf("0%02Xh", c))
	}
	b = append(b, "0")
	AsmData = append(AsmData,
		fmt.Sprintf("%s db %s", l, strings.Join(b, ",")),
		fmt.Sprintf("%s_len equ %d", l, len(s)))
	asmLits[key] = l
	return l
}

// ---------------------------------------------------------------- templates

// fillAsm expands one emission template.
//
// Three holes the other hosts do not have:
//
//	%r     the destination the emitter allocated — assembly has no expression to
//	       BE the result, so a template must be told where to put it
//	%u     a unique number, so a template may carry its own labels and therefore
//	       its own control flow
//	%1…%9  operands by position, because an instruction sequence rarely uses
//	       them in order; %s still takes the next one for the simple cases
//
// and two spellings of a register — %b<n> for its 8-bit name, %e<n> for its
// 32-bit name — because x86 spells the widths of one register differently.
func fillAsm(form string, dst place, ops []place, u int) (string, error) {
	var b strings.Builder
	next := 0
	pick := func(c byte) (place, bool) {
		if c == 'r' {
			return dst, true
		}
		if c >= '1' && c <= '9' {
			if i := int(c - '1'); i < len(ops) {
				return ops[i], true
			}
		}
		return place{}, false
	}
	for i := 0; i < len(form); i++ {
		if form[i] != '%' || i+1 >= len(form) {
			b.WriteByte(form[i])
			continue
		}
		i++
		switch c := form[i]; {
		case c == '%':
			b.WriteByte('%')
		case c == 'r':
			b.WriteString(dst.text)
		case c == 'u':
			b.WriteString(strconv.Itoa(u))
		case c == 's':
			if next >= len(ops) {
				return "", fmt.Errorf("template %q wants more operands than the %d given",
					form, len(ops))
			}
			b.WriteString(ops[next].text)
			next++
		case c >= '1' && c <= '9':
			p, ok := pick(c)
			if !ok {
				return "", fmt.Errorf("template %q names operand %c of %d", form, c, len(ops))
			}
			b.WriteString(p.text)
		case (c == 'b' || c == 'e') && i+1 < len(form):
			i++
			p, ok := pick(form[i])
			if !ok {
				return "", fmt.Errorf("template %q names operand %c of %d", form, form[i], len(ops))
			}
			if c == 'b' {
				b.WriteString(asmByte(p.text))
			} else {
				b.WriteString(asmDword(p.text))
			}
		default:
			return "", fmt.Errorf("template %q has an unknown hole %%%c", form, c)
		}
	}
	return b.String(), nil
}

var asmByteName = map[string]string{
	"rax": "al", "rbx": "bl", "rcx": "cl", "rdx": "dl",
	"rsi": "sil", "rdi": "dil", "rbp": "bpl", "rsp": "spl",
}

var asmDwordName = map[string]string{
	"rax": "eax", "rbx": "ebx", "rcx": "ecx", "rdx": "edx",
	"rsi": "esi", "rdi": "edi", "rbp": "ebp", "rsp": "esp",
}

func asmByte(r string) string {
	if n, ok := asmByteName[r]; ok {
		return n
	}
	if strings.HasPrefix(r, "r") {
		return r + "b"
	}
	return r
}

func asmDword(r string) string {
	if n, ok := asmDwordName[r]; ok {
		return n
	}
	if strings.HasPrefix(r, "r") {
		return r + "d"
	}
	return r
}

// asmNegate is the condition code that jumps exactly when this one does not.
// Both the signed set (cmp) and the unsigned set (comisd) are here, because a
// float comparison on x86 sets the carry flag rather than the sign flag.
var asmNegate = map[string]string{
	"e": "ne", "ne": "e", "z": "nz", "nz": "z",
	"l": "ge", "ge": "l", "le": "g", "g": "le",
	"b": "ae", "ae": "b", "be": "a", "a": "be",
	"s": "ns", "ns": "s",
}

// ---------------------------------------------------------------- emit

func (e *asmEmitter) emit(t *core.Term) (place, error) {
	switch t.Kind {
	case core.KInt:
		// x86 takes a 32-bit immediate in an ALU instruction and no more, so a
		// wider literal has to be materialised. Our int is exact to 2^53-1
		// (ADR 0012), which is well past that.
		if t.Int > math.MaxInt32 || t.Int < math.MinInt32 {
			p := e.alloc(false)
			if p.slot > 0 {
				e.line("mov %s, %d", asmScratchGP[0], t.Int)
				e.line("mov %s, %s", p.text, asmScratchGP[0])
			} else {
				e.line("mov %s, %d", p.text, t.Int)
			}
			return p, nil
		}
		return place{text: strconv.FormatInt(t.Int, 10), imm: true}, nil

	case core.KFloat:
		l := asmFloatLit(t.Float)
		p := e.alloc(true)
		if p.slot > 0 {
			e.line("movsd %s, %s", asmScratchX[0], l)
			e.line("movsd %s, %s", p.text, asmScratchX[0])
		} else {
			e.line("movsd %s, %s", p.text, l)
		}
		return p, nil

	case core.KBool:
		// 1 and 0. No branch is needed to MAKE a boolean here — a conditional
		// that IS a connective never reaches this case, because emitIf lays it
		// out as branches and this literal is what the branches assign.
		if t.IsTrue() {
			return place{text: "1", imm: true}, nil
		}
		return place{text: "0", imm: true}, nil

	case core.KStr:
		// A string is its address. There is no string type on this host and
		// nothing to box: the value is a pointer to NUL-terminated bytes — the
		// C convention, because that is what every Win32 and CRT entry point
		// expects to be handed.
		l := asmStringLit(t.Str)
		p := e.alloc(false)
		if p.slot > 0 {
			e.line("lea %s, %s", asmScratchGP[0], l)
			e.line("mov %s, %s", p.text, asmScratchGP[0])
		} else {
			e.line("lea %s, %s", p.text, l)
		}
		return p, nil

	case core.KName:
		if p, ok := e.where[t.Name]; ok {
			return hold(p), nil
		}
		return place{}, fmt.Errorf("unbound name %q reached the assembler", t.Name)

	case core.KFn:
		return place{}, fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  This is an escaping closure; there is no environment to allocate it in.", t)

	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return place{}, fmt.Errorf("application of a non-name: %s", t)
		}
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			// INDEXING IS APPLICATION (tables.md §3), and x86 can do it: a
			// table is an address and `(a i)` is a scaled load. What x86 cannot
			// do yet is `len` — see the refusal below.
			if e.isTableName(op.Name) && len(t.Args()) == 1 {
				return e.asmIndex(op, t.Args()[0])
			}
			return place{}, IndexingErr("windows", op.Name)
		}
		switch p.Kind {
		case "len":
			// The length is the header. One load, no analysis, no convention
			// to agree with a caller about.
			if len(t.Args()) == 1 {
				a, err := e.emit(t.Args()[0])
				if err != nil {
					return place{}, err
				}
				ll, err := e.materialize([]place{a}, "len")
				if err != nil {
					return place{}, err
				}
				d := e.alloc(false)
				e.intoGP(d, ll, func(dst string) {
					e.line("mov %s, qword ptr [%s]", dst, ll[0].text)
				})
				e.release(a)
				return d, nil
			}
		case "table-alloc":
			return e.emitAlloc(t)
		case "table-build":
			return e.emitBuild(t)
		case "table-set":
			return e.emitSet(t)
		case "array":
			return e.emitArrayLit(t)
		case "table":
			return place{}, UnallocatedTableErr()
		}
		if false {
			return place{}, fmt.Errorf(
				"`len` is not available on the windows target yet.\n" +
					"  On the other three hosts an array carries its own length — `len(a)`,\n" +
					"  `a.length` — and x86 has only an address. Whether a table here is a\n" +
					"  fat pointer, a length at offset 0, or a separate argument is a\n" +
					"  REPRESENTATION decision, and it belongs with `(alloc …)`, which is what\n" +
					"  allocates. Deciding it here would settle it by implementation accident.\n" +
					"  Indexing works: `(a i)` is a scaled load. Pass the length explicitly\n" +
					"  until `alloc` lands (docs/spec/tables.md §10).")
		}
		if p.Kind == "table-build" || p.Kind == "table-alloc" || p.Kind == "table-set" {
			return place{}, fmt.Errorf(
				"`%s` is not available on the windows target yet. It needs the array\n"+
					"  representation `len` is waiting on, above, and it needs an ALLOCATOR:\n"+
					"  the other three hosts bring a collector and this one brings\n"+
					"  VirtualAlloc and nothing. ADR 0018 says reclamation here is a lexical\n"+
					"  arena or Perceus-style refcounting, and that is a decision to make\n"+
					"  deliberately rather than by writing whichever is easiest first.\n"+
					"  x64.buf and x64.mov-store are target-native and work today.", op.Name)
		}
		if p.Kind == "array" || p.Kind == "table" {
			return place{}, fmt.Errorf(
				"`%s` is not available on the windows target yet — it needs the array\n"+
					"  representation that `len` is waiting on, above.", op.Name)
		}
		if p.Import != "" {
			AsmExterns[p.Import] = true
		}
		switch p.Kind {
		case "let":
			return e.emitLet(t)
		case "cond":
			return e.emitIf(t)
		case "iterate":
			return e.emitLoop(t)
		case "expr", "stmt":
			return e.emitPrim(t, p)
		}
		return place{}, fmt.Errorf("%s has kind %q, which the windows backend does not implement",
			op.Name, p.Kind)
	}
	return place{}, fmt.Errorf("unhandled term: %s", t)
}

// emitPrim expands one expr or stmt template.
//
// The destination is allocated BEFORE the operands are released, which is what
// makes the two-instruction shape `mov %r, %1 / add %r, %2` safe with no alias
// analysis at all: %r cannot be an operand that is still in use.
func (e *asmEmitter) emitPrim(t *core.Term, p Prim) (place, error) {
	args := t.Args()
	if p.Kind == "expr" && len(args) != len(p.Args) {
		return place{}, fmt.Errorf("%s takes %d argument(s), given %d",
			t.Op().Name, len(p.Args), len(args))
	}
	ops := make([]place, len(args))
	for i, a := range args {
		v, err := e.emit(a)
		if err != nil {
			return place{}, err
		}
		ops[i] = v
	}

	// A spilled operand is loaded into scratch, because an instruction may name
	// memory once. Two is the limit and it is checked: emitting
	// `add [rsp+48], [rsp+56]` would at least fail to assemble, but a silently
	// reused scratch register would not.
	live, err := e.materialize(ops, t.Op().Name)
	if err != nil {
		return place{}, err
	}

	var dst, real place
	if p.Kind == "stmt" {
		// A statement's value IS its first argument, on every target. Here that
		// also gives the template somewhere to write when it wants one.
		if len(live) == 0 {
			return place{}, fmt.Errorf("%s is a statement with no arguments", t.Op().Name)
		}
		dst, real = live[0], live[0]
	} else {
		dst = e.alloc(p.Result == "f64")
		real = dst
		if dst.slot > 0 {
			real = place{text: "rax"}
			if dst.xmm {
				real = place{text: "xmm5", xmm: true}
			}
		}
	}
	body, err2 := fillAsm(p.Form, real, live, e.uniq())
	if err = err2; err != nil {
		return place{}, fmt.Errorf("%s: %w", t.Op().Name, err)
	}
	for _, l := range strings.Split(body, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			e.line("%s", s)
		}
	}
	if p.Kind == "stmt" {
		for i := 1; i < len(ops); i++ {
			e.release(ops[i])
		}
		return ops[0], nil
	}
	if dst.slot > 0 {
		e.move(dst, real)
	}
	for _, o := range ops {
		e.release(o)
	}
	return dst, nil
}

// materialize loads any spilled operand into a scratch register, because an
// instruction may name memory once. Two general and one float is the limit and
// it is checked: emitting `add [rsp+48], [rsp+56]` would at least fail to
// assemble, but a silently reused scratch register would not.
func (e *asmEmitter) materialize(ops []place, who string) ([]place, error) {
	live := make([]place, len(ops))
	copy(live, ops)
	gp, xm := 0, 0
	for i := range live {
		if live[i].slot == 0 {
			continue
		}
		if live[i].xmm {
			if xm >= len(asmScratchX) {
				return nil, fmt.Errorf("%s has more spilled float operands than there are "+
					"scratch registers", who)
			}
			r := asmScratchX[xm]
			xm++
			e.line("movsd %s, %s", r, live[i].text)
			live[i] = place{text: r, xmm: true}
		} else {
			if gp >= len(asmScratchGP) {
				return nil, fmt.Errorf("%s has more spilled operands than there are "+
					"scratch registers", who)
			}
			r := asmScratchGP[gp]
			gp++
			e.line("mov %s, %s", r, live[i].text)
			live[i] = place{text: r}
		}
	}
	return live, nil
}

// ---------------------------------------------------------------- let

func (e *asmEmitter) emitLet(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return place{}, fmt.Errorf("let takes a value and (fn (x) body), got %s", t)
	}
	val, err := e.emit(args[0])
	if err != nil {
		return place{}, err
	}
	k := args[1]
	if !core.Occurs(k.Body(), k.Params[0]) {
		// `seq`: the value was emitted for its effect and is discarded.
		e.release(val)
		return e.emit(k.Body())
	}
	body, raw, _ := openFresh(k, e.bound, asmIdent)
	p := val
	if !val.owned && !val.imm {
		// The binding aliases somebody else's register. Copy it: that register
		// may be a loop variable, and `again` would rewrite it underneath.
		p = e.alloc(val.xmm)
		e.move(p, val)
	}
	prev, had := e.where[raw[0]]
	e.where[raw[0]] = hold(p)
	e.carryElem(raw[0], args[0])
	res, err := e.emit(body)
	if err != nil {
		return place{}, err
	}
	if had {
		e.where[raw[0]] = prev
	} else {
		delete(e.where, raw[0])
	}
	if res.text == p.text {
		// The body IS the binding — `(let v (fn (x) x))`, or a loop every exit
		// of which yields x. OWNERSHIP TRANSFERS: the caller must be the one to
		// free the register, or it is held for the rest of the procedure.
		// Getting this wrong leaks one register per nested let, which is how it
		// was found — the sieve ran the pool dry.
		return p, nil
	}
	e.release(p)
	return res, nil
}

// ---------------------------------------------------------------- if
//
// Both arms write one destination, so the arms are emitted as statements and
// the conditional becomes a value again at the join. That is the same shape
// emit/golang.go uses for a conditional in statement position; here there is no
// other position for one to be in.

func (e *asmEmitter) emitIf(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) != 3 {
		return place{}, fmt.Errorf("if takes a condition and two branches, got %s", t)
	}
	u := e.uniq()
	els, end := fmt.Sprintf("Lelse%d", u), fmt.Sprintf("Lend%d", u)
	if err := e.branchUnless(args[0], els); err != nil {
		return place{}, err
	}
	dst := e.alloc(e.isFloat(args[1]) || e.isFloat(args[2]))
	th, err := e.emit(args[1])
	if err != nil {
		return place{}, err
	}
	e.move(dst, th)
	e.release(th)
	e.line("jmp %s", end)
	e.label(els)
	el, err := e.emit(args[2])
	if err != nil {
		return place{}, err
	}
	e.move(dst, el)
	e.release(el)
	e.label(end)
	return dst, nil
}

// branchUnless jumps to `label` when the condition is FALSE.
//
// This is where (jump …) earns its place. Without it every test costs two
// compares — one to make a 0/1 boolean and one to test that boolean against
// zero — because assembly is the first host that cannot fold a comparison into
// a branch on its own. Go, JavaScript and Java all do it internally and none of
// them had to be told.

func (e *asmEmitter) branchUnless(c *core.Term, label string) error {
	// A connective in guard position is the dragon book's jumping code: both
	// failures leave for the SAME label, so the continuation the commuting
	// conversion would duplicate is one label and costs nothing (booleans.md
	// §2.7). This is where short-circuiting lives on this host, and it now comes
	// from the SHAPE OF THE TERM rather than from a claim in a target file —
	// which is what made `x64.andb` mean two different things.
	if cn, ok := connective(e.tgt, c); ok {
		switch cn.Op {
		case "and":
			if err := e.branchUnless(cn.Args[0], label); err != nil {
				return err
			}
			return e.branchUnless(cn.Args[1], label)
		case "or":
			u := e.uniq()
			taken := fmt.Sprintf("Lor%d", u)
			if err := e.branchIf(cn.Args[0], taken); err != nil {
				return err
			}
			if err := e.branchUnless(cn.Args[1], label); err != nil {
				return err
			}
			e.label(taken)
			return nil
		case "not":
			return e.branchIf(cn.Args[0], label)
		}
	}
	if c.Kind == core.KBool {
		if !c.IsTrue() {
			e.line("jmp %s", label)
		}
		return nil
	}
	if c.Kind == core.KApp && c.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[c.Op().Name]; ok && p.Jump != "" && len(c.Args()) == 2 {
			return e.compare(c, p, asmNegate[p.Jump], label)
		}
	}
	// A byte-table read used as a guard could fuse into the compare —
	//
	//	movzx r14d, byte ptr [c+i+8]     cmp byte ptr [c+i+8], 0
	//	cmp   r14, 0                ->   je  L
	//	je    L
	//
	// which is three instructions where x86 does one, and is exactly what
	// `x64.test-byte` exists for. It was BUILT AND REVERTED: measured on the
	// sieve, the only program with this shape, it is not distinguishable from
	// the unfused form — 90/90 and 74/78 ms across repeats. The loop is
	// memory-bound over 200,000 bytes and the extra instructions hide behind
	// the cache traffic, which is bce-2026-08-15's finding arriving again:
	// a saving on a compute-bound loop is nothing on a memory-bound one.
	//
	// Not kept, because nothing measured supports it (wintables-2026-08-25 §5).
	v, err := e.emit(c)
	if err != nil {
		return err
	}
	r := e.inRegister(v)
	e.line("cmp %s, 0", r.text)
	e.line("je %s", label)
	e.release(v)
	return nil
}

// branchIf is branchUnless's dual, needed only by ||.
func (e *asmEmitter) branchIf(c *core.Term, label string) error {
	if cn, ok := connective(e.tgt, c); ok {
		switch cn.Op {
		case "and":
			u := e.uniq()
			no := fmt.Sprintf("Land%d", u)
			if err := e.branchUnless(cn.Args[0], no); err != nil {
				return err
			}
			if err := e.branchIf(cn.Args[1], label); err != nil {
				return err
			}
			e.label(no)
			return nil
		case "or":
			if err := e.branchIf(cn.Args[0], label); err != nil {
				return err
			}
			return e.branchIf(cn.Args[1], label)
		case "not":
			return e.branchUnless(cn.Args[0], label)
		}
	}
	if c.Kind == core.KBool {
		if c.IsTrue() {
			e.line("jmp %s", label)
		}
		return nil
	}
	if c.Kind == core.KApp && c.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[c.Op().Name]; ok && p.Jump != "" && len(c.Args()) == 2 {
			return e.compare(c, p, p.Jump, label)
		}
	}
	v, err := e.emit(c)
	if err != nil {
		return err
	}
	r := e.inRegister(v)
	e.line("cmp %s, 0", r.text)
	e.line("jne %s", label)
	e.release(v)
	return nil
}

// compare emits the comparison and one conditional jump. `cmp` for integers,
// `comisd` for floats — which is why a target declares the UNSIGNED condition
// codes (b, be, a, ae) for its float predicates: comisd sets the carry flag,
// not the sign flag.
func (e *asmEmitter) compare(c *core.Term, p Prim, cc, label string) error {
	a, err := e.emit(c.Args()[0])
	if err != nil {
		return err
	}
	b, err := e.emit(c.Args()[1])
	if err != nil {
		return err
	}
	if p.JumpForm != "" {
		// The target declared its own flag-setting form, which is how a
		// predicate that is not a comparison of two values — `is this byte
		// nonzero` — becomes one instruction instead of three.
		ops, err := e.materialize([]place{a, b}, c.Op().Name)
		if err != nil {
			return err
		}
		body, err := fillAsm(p.JumpForm, place{}, ops, e.uniq())
		if err != nil {
			return fmt.Errorf("%s: %w", c.Op().Name, err)
		}
		for _, l := range strings.Split(body, "\n") {
			if s := strings.TrimSpace(l); s != "" {
				e.line("%s", s)
			}
		}
		e.line("j%s %s", cc, label)
		e.release(a)
		e.release(b)
		return nil
	}
	ar := e.inRegister(a)
	br := b
	if b.slot > 0 {
		br = e.scratchCopy(b, 1)
	}
	if ar.xmm {
		e.line("comisd %s, %s", ar.text, br.text)
	} else {
		e.line("cmp %s, %s", ar.text, br.text)
	}
	e.line("j%s %s", cc, label)
	e.release(a)
	e.release(b)
	return nil
}

// inRegister makes a place usable as the first operand of an instruction: an
// immediate or a frame slot moves into scratch.
func (e *asmEmitter) inRegister(p place) place {
	if p.reg() {
		return p
	}
	return e.scratchCopy(p, 0)
}

// intoGP emits a load whose destination may be SPILLED.
//
// x86 has no memory-to-memory `mov`, so a spilled destination has to be loaded
// through a general register and then stored — the same rule `move` already
// applies between two slots, arriving here from the other direction.
//
// The register has to be free AFTER the address is formed. A scratch register
// the address does not use is free by definition; failing that, one the address
// DOES use is free the instant the load reads it, because a single instruction
// computes its memory operand before writing its destination, and materialize
// put that value there for this instruction alone.
//
// Found by the JSON tokeniser — six loop variables and three nested loops, the
// first program in this repository with enough live values for the destination
// of a `len` to be spilled (json-2026-08-26).
func (e *asmEmitter) intoGP(d place, live []place, emit func(dst string)) {
	if d.slot == 0 {
		emit(d.text)
		return
	}
	r := ""
	for _, s := range asmScratchGP {
		used := false
		for _, p := range live {
			if p.text == s {
				used = true
			}
		}
		if !used {
			r = s
			break
		}
	}
	if r == "" {
		r = asmScratchGP[0]
	}
	emit(r)
	e.line("mov %s, %s", d.text, r)
}

func (e *asmEmitter) scratchCopy(p place, n int) place {
	if p.xmm {
		r := place{text: asmScratchX[0], xmm: true}
		e.line("movsd %s, %s", r.text, p.text)
		return r
	}
	r := place{text: asmScratchGP[n%len(asmScratchGP)]}
	e.line("mov %s, %s", r.text, p.text)
	return r
}

// isFloat decides which register file a value belongs in. There is no
// inference: a primitive's declared result type says so, a literal says so, and
// a name was recorded when it was bound. That is the whole of what the type
// language buys on this host — one bit, and it is not optional, because xmm and
// the general registers are different instructions rather than different
// declarations.
func (e *asmEmitter) isFloat(t *core.Term) bool {
	switch t.Kind {
	case core.KFloat:
		return true
	case core.KName:
		return e.where[t.Name].xmm
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := e.tgt.Prims[op.Name]; ok {
				switch p.Kind {
				case "let":
					if k := t.Args()[1]; k.Kind == core.KFn {
						return e.isFloat(k.Body())
					}
				case "cond":
					return e.isFloat(t.Args()[1]) || e.isFloat(t.Args()[2])
				case "iterate":
					if lam := t.Args()[0]; lam.Kind == core.KFn {
						return e.exitIsFloat(lam.Body())
					}
				case "stmt":
					if len(t.Args()) > 0 {
						return e.isFloat(t.Args()[0])
					}
				}
				return p.Result == "f64"
			}
		}
	}
	return false
}

// ---------------------------------------------------------------- loop

func (e *asmEmitter) emitLoop(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return place{}, fmt.Errorf("loop takes (fn (x…) body) and one initial value per variable")
	}
	lam, inits := args[0], args[1:]
	if len(lam.Params) != len(inits) {
		return place{}, fmt.Errorf("loop has %d variable(s) and %d initial value(s)",
			len(lam.Params), len(inits))
	}
	vals := make([]place, len(inits))
	for i, z := range inits {
		v, err := e.emit(z)
		if err != nil {
			return place{}, err
		}
		vals[i] = v
	}
	body, raw, _ := openFresh(lam, e.bound, asmIdent)
	vars := make([]place, len(inits))
	saved := map[string]place{}
	had := map[string]bool{}
	// A variable that never actually changes keeps the place its initial value
	// is already in — no register of its own, and no copy in or out. See
	// LoopInvariant: a threaded buffer looks like it changes and does not,
	// because `set` hands back what it was given, and on a host with a fixed
	// register file that difference cost the inner loop's INDEX its register.
	invariant := LoopInvariant(e.tgt, body, raw)
	aliased := map[int]bool{}
	for i := range inits {
		if invariant[i] {
			// The place may belong to an ENCLOSING binder — a nested loop
			// threading the same buffer is the common case — so it is aliased
			// rather than taken, and not released below.
			vars[i] = vals[i]
			aliased[i] = !vals[i].owned
		} else {
			// Otherwise it is assigned every iteration and needs a place of its
			// own, even when the initial value already had one.
			vars[i] = e.alloc(vals[i].xmm || e.isFloat(inits[i]))
			e.move(vars[i], vals[i])
			e.release(vals[i])
		}
		saved[raw[i]], had[raw[i]] = e.where[raw[i]], hasPlace(e.where, raw[i])
		e.where[raw[i]] = hold(vars[i])
		e.carryElem(raw[i], inits[i])
	}

	// If every exit yields the same variable, that variable IS the loop's value
	// and no result register is needed. The other three backends make the same
	// saving; here it is a whole register rather than a declaration.
	names := make([]string, len(raw))
	copy(names, raw)
	var result place
	if n := soleExit(e.tgt.Prims, body, raw, names, e.bound, asmIdent); n != "" {
		result = hold(e.where[n])
	} else {
		result = e.alloc(e.exitIsFloat(body))
	}

	u := e.uniq()
	top, end := fmt.Sprintf("Ltop%d", u), fmt.Sprintf("Ldone%d", u)
	e.label(top)
	if err := e.emitLoopBody(body, raw, vars, result, top, end); err != nil {
		return place{}, err
	}
	e.label(end)

	for i := range raw {
		if had[raw[i]] {
			e.where[raw[i]] = saved[raw[i]]
		} else {
			delete(e.where, raw[i])
		}
		if aliased[i] {
			continue // borrowed from an enclosing binder; not ours to free
		}
		if vars[i].text != result.text {
			e.release(vars[i])
			continue
		}
		// Every exit yielded this loop variable, so its register IS the loop's
		// value and its ownership passes to whoever consumes the loop. When the
		// sole exit is an OUTER name instead, nothing here matches and the
		// place stays unowned, which is also right.
		result = vars[i]
	}
	return result, nil
}

func hasPlace(m map[string]place, k string) bool { _, ok := m[k]; return ok }

// exitIsFloat finds the register file of the first non-`again` leaf.
func (e *asmEmitter) exitIsFloat(t *core.Term) bool {
	if isAgain(t) {
		return false
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok {
			if p.Kind == "cond" && len(t.Args()) == 3 {
				return e.exitIsFloat(t.Args()[1]) || e.exitIsFloat(t.Args()[2])
			}
			if p.Kind == "let" && len(t.Args()) == 2 {
				if k := t.Args()[1]; k.Kind == core.KFn {
					return e.exitIsFloat(k.Body())
				}
			}
		}
	}
	return e.isFloat(t)
}

func (e *asmEmitter) emitLoopBody(t *core.Term, raw []string, vars []place,
	result place, top, end string) error {

	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			u := e.uniq()
			skip := fmt.Sprintf("Lnext%d", u)
			if err := e.branchUnless(t.Args()[0], skip); err != nil {
				return err
			}
			if err := e.emitLoopBody(t.Args()[1], raw, vars, result, top, end); err != nil {
				return err
			}
			e.label(skip)
			return e.emitLoopBody(t.Args()[2], raw, vars, result, top, end)
		}
		// `let binds, if branches` — a clause body may bind before it jumps.
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "let" && len(t.Args()) == 2 {
			args := t.Args()
			k := args[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					e.release(val)
					return e.emitLoopBody(k.Body(), raw, vars, result, top, end)
				}
				kbody, kraw, _ := openFresh(k, e.bound, asmIdent)
				e.carryElem(kraw[0], args[0])
				q := val
				if !val.owned && !val.imm {
					q = e.alloc(val.xmm)
					e.move(q, val)
				}
				e.where[kraw[0]] = hold(q)
				err = e.emitLoopBody(kbody, raw, vars, result, top, end)
				delete(e.where, kraw[0])
				e.release(q)
				return err
			}
		}
	}
	if isAgain(t) {
		return e.emitAgain(t, raw, vars, top)
	}
	v, err := e.emit(t)
	if err != nil {
		return err
	}
	e.move(result, v)
	e.release(v)
	e.line("jmp %s", end)
	return nil
}

// emitAgain is the back edge: assign the loop variables and jump.
//
// Two things it does, and the second one is new here.
//
// An argument that is syntactically the variable itself is skipped — hamza's
// optimisation from ADR 0015, and it matters more on this host than anywhere
// else, because a skipped assignment is a `mov` that is simply not there rather
// than one the host optimiser will later notice.
//
// And the assignments are ORDERED rather than staged through temporaries. Go
// has parallel assignment; JS and Java have neither, and needTemps asks the one
// question they can act on — is a temporary needed at all. x86 can do better,
// because the answer is usually "for one of them, and only if you do it first".
//
//	(again (x64.add i 1) (x64.add acc i))
//
// needTemps says yes: acc reads i, and i is changing. But assigning acc FIRST
// makes the question disappear, and both updates are then single instructions
// against their own registers. Only a genuine cycle — a swap — needs a copy,
// and then only one. This is parallel-copy sequentialisation, and it is worth
// the thirty lines exactly once, on the host where a redundant `mov` is a
// redundant `mov` rather than a hint.
func (e *asmEmitter) emitAgain(t *core.Term, raw []string, vars []place, top string) error {
	as := t.Args()
	if len(as) != len(vars) {
		return fmt.Errorf("again takes %d argument(s), given %d", len(vars), len(as))
	}
	pending := map[int]bool{}
	// nil: x86 has no `for` statement to hoist a post clause into, and no host
	// compiler to hide the loop shape from — the labels and the back jump ARE
	// the emitted code. PostVars exists to make Go, V8 and C2 recognise a
	// counted loop; here there is nobody to convince.
	for _, i := range changedArgs(as, raw, nil) {
		pending[i] = true
	}
	// A variable that has been copied aside is no longer a hazard: every later
	// read of its name finds the copy, which holds the value it had on entry.
	saved := map[string]bool{}
	var temps []place

	// hazard reports whether writing raw[i] now would be seen by an argument
	// that has not been evaluated yet.
	hazard := func(i int) bool {
		if saved[raw[i]] {
			return false
		}
		one := map[string]bool{raw[i]: true}
		for j := range pending {
			if j != i && readsAny(as[j], one) {
				return true
			}
		}
		return false
	}

	for len(pending) > 0 {
		// Deterministic order: the loop's own, so the output does not depend on
		// map iteration.
		next := -1
		for i := range vars {
			if pending[i] && !hazard(i) {
				next = i
				break
			}
		}
		if next < 0 {
			// Every remaining assignment is read by another: a cycle. Break it
			// by copying one variable aside, which costs exactly one `mov`.
			var pick = -1
			for i := range vars {
				if pending[i] {
					pick = i
					break
				}
			}
			c := e.alloc(vars[pick].xmm)
			e.move(c, vars[pick])
			e.where[raw[pick]] = hold(c)
			saved[raw[pick]] = true
			temps = append(temps, c)
			continue
		}
		if err := e.assign(vars[next], as[next]); err != nil {
			return err
		}
		delete(pending, next)
	}
	for i := range vars {
		if saved[raw[i]] {
			e.where[raw[i]] = hold(vars[i])
		}
	}
	for _, c := range temps {
		e.release(c)
	}
	e.line("jmp %s", top)
	return nil
}

// ---------------------------------------------------------------- procedures

// asmIdent is the identity: a local has no name in the output at all, it has a
// register. Mangling exists on the other three hosts only because their locals
// are spelled.
func asmIdent(s string) string { return s }

// AsmMangle makes a name a legal MASM identifier. Only PROCEDURES need one.
func AsmMangle(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "X" + out
	}
	return out
}

// AsmProc emits a top-level abstraction as one Win64 procedure.
// asmRetGP and asmRetX carry SEVERAL results. Win64 defines one return
// register; these mirror its argument convention for the rest, and are ours
// because both sides of the call are ours.
var asmRetGP = []string{"rax", "rdx", "r8", "r9"}
var asmRetX = []string{"xmm0", "xmm1", "xmm2", "xmm3"}

// emitMulti places each result in its convention register.
func (e *asmEmitter) emitMulti(name string, sig *core.Sig, body *core.Term) error {
	end := fmt.Sprintf("Lret%d", e.uniq())
	if err := e.multiTail(body, sig, name, end); err != nil {
		return err
	}
	e.label(end)
	return nil
}

// multiTail walks `if` and `let` to the leaves that actually produce a product,
// placing the results and jumping to a shared exit from each one.
//
// Sums are what demanded it: a function returning a sum returns from several
// places. It mirrors emitLoopBody, which walks the same two forms for the same
// reason -- they are exactly what beta can leave between a function and its
// value.
func (e *asmEmitter) multiTail(body *core.Term, sig *core.Sig, name, end string) error {
	if body.Kind == core.KApp && body.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[body.Op().Name]; ok && len(body.Args()) == 3 && p.Kind == "cond" {
			args := body.Args()
			els := fmt.Sprintf("Lelse%d", e.uniq())
			if err := e.branchUnless(args[0], els); err != nil {
				return err
			}
			if err := e.multiTail(args[1], sig, name, end); err != nil {
				return err
			}
			e.label(els)
			return e.multiTail(args[2], sig, name, end)
		}
		if p, ok := e.tgt.Prims[body.Op().Name]; ok && p.Kind == "let" && len(body.Args()) == 2 {
			args := body.Args()
			k := args[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					e.release(val)
					return e.multiTail(k.Body(), sig, name, end)
				}
				kbody, kraw, _ := openFresh(k, e.bound, asmIdent)
				e.carryElem(kraw[0], args[0])
				q := val
				if !val.owned && !val.imm {
					q = e.alloc(val.xmm)
					e.move(q, val)
				}
				e.where[kraw[0]] = hold(q)
				err = e.multiTail(kbody, sig, name, end)
				delete(e.where, kraw[0])
				e.release(q)
				return err
			}
		}
	}
	vs, ok := multiValue(body, len(sig.Results))
	if !ok {
		return multiResultErr(name, sig, body)
	}
	// TWO PASSES, and the first draft had one. Placing each result into its
	// return register as it is computed CLOBBERS the earlier ones: rax and rdx
	// are `idiv`'s own operands, so computing the second result of a `divmod`
	// overwrote the first. Emitted, read, and caught:
	//
	//	mov rdi, rax     ; quotient held
	//	mov rax, rdi     ; result 0 placed
	//	mov rax, rbx     ; ...and immediately destroyed
	//
	// So: compute everything into the places the allocator chose, then move
	// into the convention registers once nothing else will run.
	places := make([]place, len(vs))
	for i, v := range vs {
		r, err := e.emit(v)
		if err != nil {
			return err
		}
		places[i] = r
	}
	var gp, xm int
	for i, r := range places {
		if sig.Results[i] == "f64" {
			if xm >= len(asmRetX) {
				return fmt.Errorf("%s returns more float results than this convention "+
					"carries (%d)", name, len(asmRetX))
			}
			if r.text != asmRetX[xm] {
				e.line("movsd %s, %s", asmRetX[xm], r.text)
			}
			xm++
			continue
		}
		if gp >= len(asmRetGP) {
			return fmt.Errorf("%s returns more integer results than this convention "+
				"carries (%d)", name, len(asmRetGP))
		}
		if r.text != asmRetGP[gp] {
			e.line("mov %s, %s", asmRetGP[gp], r.text)
		}
		gp++
	}
	e.line("jmp %s", end)
	return nil
}

func AsmProc(tgt *Target, name string, sig *core.Sig, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := newAsmEmitter(tgt)
	body, raw, _ := openFresh(t, e.bound, asmIdent)
	if len(raw) > len(asmArgGP) {
		return "", fmt.Errorf("%s takes %d arguments; the Win64 convention passes four in "+
			"registers and this backend does not read the fifth off the stack", name, len(raw))
	}
	for i, p := range raw {
		xmm := sig != nil && i < len(sig.Params) && sig.Params[i].Type == "f64"
		d := e.alloc(xmm)
		if xmm {
			e.line("movsd %s, %s", d.text, asmArgX[i])
		} else {
			e.line("mov %s, %s", d.text, asmArgGP[i])
		}
		e.where[p] = hold(d)
	}
	if sig != nil && len(sig.Results) > 1 {
		// SEVERAL RESULTS — the negative product reaching a boundary.
		//
		// Win64 returns ONE value, in rax or xmm0; a 16-byte struct goes back
		// through a hidden pointer. So a second register is OUR convention
		// rather than the platform's — legitimate because these procedures are
		// ours on both sides and nothing outside the emitted program calls
		// them. The first attempt at this feature let windows DECLINE multiple
		// results and was reverted for it.
		//
		// The convention mirrors Win64's ARGUMENT convention, which is the
		// least surprising choice available and costs no memory traffic:
		// integer results in rax, rdx, r8, r9 and float results in xmm0..xmm3,
		// each in declaration order within its class.
		if err := e.emitMulti(name, sig, body); err != nil {
			return "", err
		}
	} else {
		res, err := e.emit(body)
		if err != nil {
			return "", err
		}
		if res.xmm {
			if res.text != "xmm0" {
				e.line("movsd xmm0, %s", res.text)
			}
		} else if res.text != "rax" && res.text != "" {
			e.line("mov rax, %s", res.text)
		}
	}

	// The prologue is written last: only now is it known which callee-saved
	// registers were used and how many slots were spilled. The other three
	// backends never had to know either fact.
	var saved []string
	for _, r := range asmValueGP {
		if e.usedGP[r] {
			saved = append(saved, r)
		}
	}
	var savedX []string
	for _, r := range asmValueX {
		if e.usedX[r] {
			savedX = append(savedX, r)
		}
	}
	frame := asmShadow + 8*(e.slots+len(savedX))
	// rsp is 8 mod 16 on entry and each push moves it by 8. The frame must
	// bring it back to 0 mod 16 or a callee's own aligned spill faults — which
	// is a crash inside kernel32 with nothing in the traceback pointing here.
	for ((8-8*len(saved)-frame)%16+16)%16 != 0 {
		frame += 8
	}
	xbase := asmShadow + 8*e.slots

	m := AsmMangle(name)
	var out strings.Builder
	fmt.Fprintf(&out, "%s proc\n", m)
	for _, r := range saved {
		fmt.Fprintf(&out, "        push %s\n", r)
	}
	fmt.Fprintf(&out, "        sub rsp, %d\n", frame)
	for i, r := range savedX {
		fmt.Fprintf(&out, "        movsd qword ptr [rsp+%d], %s\n", xbase+8*i, r)
	}
	out.WriteString(asmPeephole(e.buf.String()))
	for i, r := range savedX {
		fmt.Fprintf(&out, "        movsd %s, qword ptr [rsp+%d]\n", r, xbase+8*i)
	}
	fmt.Fprintf(&out, "        add rsp, %d\n", frame)
	for i := len(saved) - 1; i >= 0; i-- {
		fmt.Fprintf(&out, "        pop %s\n", saved[i])
	}
	out.WriteString("        ret\n")
	fmt.Fprintf(&out, "%s endp\n", m)
	return out.String(), nil
}

// AsmFile wraps emitted procedures in an assemblable translation unit.
func AsmFile(tgt *Target, procs map[string]string, entry string) string {
	names := make([]string, 0, len(procs))
	for n := range procs {
		names = append(names, n)
	}
	sort.Strings(names)

	if entry != "" {
		AsmExterns["ExitProcess"] = true
	}
	var code strings.Builder
	for _, n := range names {
		code.WriteString(procs[n])
		code.WriteString("\n")
	}
	body := code.String()

	var out strings.Builder
	out.WriteString("; Code generated by oroboros. DO NOT EDIT.\n\n")
	out.WriteString("option casemap:none\n\n")
	for _, x := range sortedSet(AsmExterns) {
		fmt.Fprintf(&out, "extern %s: proc\n", x)
	}
	// A target's storage is emitted only when something USES it. `__buf` is
	// 4096 bytes and every program was carrying it: hello-win.oro's artifact was
	// 6656 bytes against a hand-written 2560, and the whole difference was one
	// buffer it never touches. Imports were demand-driven from the very first
	// target file; data was not, because until this host no target had any.
	var used []string
	for _, d := range tgt.Data {
		label, _, _ := strings.Cut(strings.TrimSpace(d), " ")
		if label == "" || strings.Contains(body, label) {
			used = append(used, d)
		}
	}
	out.WriteString("\n.data\n")
	for _, d := range used {
		fmt.Fprintf(&out, "%s\n", d)
	}
	for _, d := range AsmData {
		fmt.Fprintf(&out, "%s\n", d)
	}
	out.WriteString("\n.code\n")
	out.WriteString(body)
	if entry != "" {
		// Win32 has no runtime to return to: a process ends when something
		// calls ExitProcess, so that is the one call this backend makes on the
		// program's behalf. The emitted procedure's value is discarded, exactly
		// as it is by Go's `func main` and Java's `static void main`.
		fmt.Fprintf(&out, "main proc\n        sub rsp, 40\n        call %s\n"+
			"        xor ecx, ecx\n        call ExitProcess\nmain endp\n", AsmMangle(entry))
	}
	out.WriteString("\nend\n")
	return out.String()
}

// ---------------------------------------------------------------- in place
//
// `again` assigns a loop variable, and the assignment is nearly always
// `x = x OP e`. Emitted the general way that is three instructions —
// `mov t, x` / `add t, e` / `mov x, t` — where x86 does it in one, because the
// destructive form IS `add x, e`. Closing that gap is the difference between
// this backend's inner loop and a hand-written one.
//
// The condition for folding a template into its own first operand is textual
// and checkable: the template must begin `mov %r, %1` and never mention %1
// again. Then with %r and %1 the same register the first instruction is a
// no-op, nothing later reads a value it has clobbered, and dropping it is
// exactly equivalent. That is a proof about the template rather than a special
// case for `add`, so it applies to every arithmetic primitive a target declares
// in that shape — and to none declared in any other.
func (e *asmEmitter) emitInto(t *core.Term, dst place) (bool, error) {
	if !dst.reg() || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return false, nil
	}
	p, ok := e.tgt.Prims[t.Op().Name]
	if !ok || p.Kind != "expr" {
		return false, nil
	}
	head, rest, split := strings.Cut(p.Form, "\n")
	if !split || (head != "mov %r, %1" && head != "movsd %r, %1") {
		return false, nil
	}
	if strings.Contains(rest, "%1") || strings.Contains(rest, "%s") {
		return false, nil
	}
	args := t.Args()
	if len(args) == 0 || len(args) != len(p.Args) {
		return false, nil
	}
	// Every operand must be readable WITHOUT emitting anything, so that failing
	// the test costs nothing and the general path is still available. A name
	// already in a register and a small literal are the whole of it, which is
	// also the whole of what an `again` argument turns out to be.
	ops := make([]place, len(args))
	for i, a := range args {
		switch {
		case a.Kind == core.KInt && a.Int <= math.MaxInt32 && a.Int >= math.MinInt32:
			ops[i] = place{text: strconv.FormatInt(a.Int, 10), imm: true}
		case a.Kind == core.KName:
			q, known := e.where[a.Name]
			if !known || !q.reg() {
				return false, nil
			}
			ops[i] = hold(q)
		default:
			return false, nil
		}
		// A later operand living in the destination would be clobbered by the
		// first write. `x = y - x` is the case, and it takes the general path.
		if i > 0 && ops[i].text == dst.text {
			return false, nil
		}
	}
	if ops[0].text != dst.text {
		return false, nil
	}
	body, err := fillAsm(rest, dst, ops, e.uniq())
	if err != nil {
		return false, fmt.Errorf("%s: %w", t.Op().Name, err)
	}
	for _, l := range strings.Split(body, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			e.line("%s", s)
		}
	}
	return true, nil
}

// assign puts a term's value in a place, in place where that is possible.
func (e *asmEmitter) assign(dst place, t *core.Term) error {
	if done, err := e.emitInto(t, dst); err != nil || done {
		return err
	}
	v, err := e.emit(t)
	if err != nil {
		return err
	}
	e.move(dst, v)
	e.release(v)
	return nil
}

// ---------------------------------------------------------------- peephole
//
// Two rewrites over this backend's own output. Neither is a general optimiser
// and neither looks at anything but the shape of the jumps, which is what makes
// them safe to state:
//
//	jmp L … L:        the jump is to the next instruction. Delete it.
//	jcc A / jmp B / A:  branch on the negated condition straight to B.
//
// Both exist because a clause chain emits a branch AROUND a clause body, and a
// clause body that only leaves the loop is one `jmp`. Structured control flow
// generated structurally always produces this, and a person writing the same
// loop never does.
func asmPeephole(src string) string {
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	for pass := 0; pass < 8; pass++ {
		var out []string
		changed := false
		for i := 0; i < len(lines); i++ {
			l := strings.TrimSpace(lines[i])
			if tgt, ok := strings.CutPrefix(l, "jmp "); ok {
				j := i + 1
				for j < len(lines) && asmIsLabel(lines[j]) && asmLabelOf(lines[j]) != tgt {
					j++
				}
				if j < len(lines) && asmIsLabel(lines[j]) && asmLabelOf(lines[j]) == tgt {
					changed = true
					continue
				}
			}
			if cc, tgt, ok := asmCondJump(l); ok && i+2 < len(lines) {
				if b, isJmp := strings.CutPrefix(strings.TrimSpace(lines[i+1]), "jmp "); isJmp &&
					asmIsLabel(lines[i+2]) && asmLabelOf(lines[i+2]) == tgt {
					if neg, negatable := asmNegate[cc]; negatable {
						out = append(out, "        j"+neg+" "+b)
						i++
						changed = true
						continue
					}
				}
			}
			out = append(out, lines[i])
		}
		lines = out
		if !changed {
			break
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func asmIsLabel(l string) bool {
	t := strings.TrimSpace(l)
	return strings.HasSuffix(t, ":") && !strings.ContainsAny(t, " \t")
}

func asmLabelOf(l string) string { return strings.TrimSuffix(strings.TrimSpace(l), ":") }

func asmCondJump(l string) (cc, target string, ok bool) {
	if !strings.HasPrefix(l, "j") || strings.HasPrefix(l, "jmp ") {
		return "", "", false
	}
	m, rest, split := strings.Cut(l, " ")
	if !split {
		return "", "", false
	}
	cc = m[1:]
	if _, known := asmNegate[cc]; !known {
		return "", "", false
	}
	return cc, strings.TrimSpace(rest), true
}

// isTableName reports whether an unknown name in operator position is a local,
// which in a residual can only be a table (tables.md §3.2).
func (e *asmEmitter) isTableName(n string) bool {
	_, ok := e.where[n]
	return ok
}

// asmIndex emits a scaled load, past the LENGTH HEADER.
//
// A table on this target is one register: a pointer whose first eight bytes
// hold the length, with element 0 at offset 8. That representation was chosen
// over a fat pointer because our calling convention passes one value per
// register, and a two-register table would stop a table being a value at all.
//
// The header costs NOTHING to skip. `[rbx+rcx*8+8]` is the same instruction as
// `[rbx+rcx*8]` — the displacement is part of x86's addressing mode — so the
// only price of carrying a length is the eight bytes themselves.
//
// Eight bytes per element, because that is what every element type the language
// has on this target occupies: `int` and `f64` are both 64-bit. A byte table is
// `x64.movzx`, which stays target-native until the element size is part of the
// type — and a `(array bool)` here therefore costs 8x what it does on Go.
// MATERIALIZE first. A spilled operand lives in a stack slot, and x86 cannot
// use memory as a base or an index register — emitting one directly
// produced `mov qword ptr [r15+qword ptr [rsp+48]*8+8], 1`, which MASM
// rejects with "constant expected". emitPrim has always done this for
// templates; the table operations are the first code in this backend that
// builds an addressing mode itself.
func (e *asmEmitter) asmIndex(tab, idx *core.Term) (place, error) {
	a, err := e.emit(tab)
	if err != nil {
		return place{}, err
	}
	i, err := e.emit(idx)
	if err != nil {
		return place{}, err
	}
	live, err := e.materialize([]place{a, i}, "index")
	if err != nil {
		return place{}, err
	}
	d := e.alloc(false)
	// A BYTE table is read with movzx, which zero-extends — so a bool comes back
	// as 0 or 1, which is what `if` wants. A qword load against a byte array
	// would read seven bytes of the following elements, so getting the width
	// wrong here is a wrong answer rather than a slow one.
	if w := e.elemOf(tab); w == 1 {
		e.intoGP(d, live, func(dst string) {
			e.line("movzx %s, byte ptr %s", asmDword(dst),
				asmElemAddr(live[0].text, live[1].text, 1))
		})
	} else {
		e.intoGP(d, live, func(dst string) {
			e.line("mov %s, qword ptr %s", dst,
				asmElemAddr(live[0].text, live[1].text, 8))
		})
	}
	e.release(a)
	e.release(i)
	return d, nil
}

// asmTableHeader is where a table's elements start. The first eight bytes hold
// the length; see asmIndex for why the representation is a single pointer.
const asmTableHeader = 8

// rawAlloc asks the TARGET for n bytes.
//
// ADR 0002's division, made concrete: `alloc` and `build` are the language's,
// and where memory comes from on this host is the target's. Go, JavaScript and
// Java have allocation as syntax and their backends emit it; x86 has a call,
// and the target says which — found by findAlloc, so `targets/windows/` needed
// no new declaration to get tables. It already declared VirtualAlloc.
//
// ADR 0018 says reclamation here is a lexical arena or Perceus-style
// refcounting. This is NEITHER: it is one allocation per `alloc`, never freed.
// That is the crude answer, it is the target's to change without touching the
// compiler, and its cost is recorded rather than hidden — a syscall per
// allocation, which a program allocating in a loop will feel.
func (e *asmEmitter) rawAlloc(bytes place) (place, error) {
	p, ok := e.tgt.findAlloc()
	if !ok {
		return place{}, fmt.Errorf(
			"this target declares no allocator, so `alloc` and `build` have no memory to " +
				"use.\n  Declare one — VirtualAlloc, malloc or HeapAlloc — taking a byte " +
				"count and\n  returning a pointer. `alloc` and `build` are the language's; " +
				"where the bytes\n  come from is the target's (docs/spec/tables.md §10).")
	}
	if p.Import != "" {
		AsmExterns[p.Import] = true
	}
	// The operand must be in a register: an allocator template moves it into
	// rdx or rcx, and an instruction may name memory once.
	live, err := e.materialize([]place{bytes}, "alloc")
	if err != nil {
		return place{}, err
	}
	d := e.alloc(false)
	real := d
	if d.slot > 0 {
		real = place{text: "rax"}
	}
	body, err := fillAsm(p.Form, real, live, e.uniq())
	if err != nil {
		return place{}, fmt.Errorf("alloc: %w", err)
	}
	for _, l := range strings.Split(body, "\n") {
		if x := strings.TrimSpace(l); x != "" {
			e.line("%s", x)
		}
	}
	if d.slot > 0 {
		e.move(d, real)
	}
	e.release(bytes)
	return d, nil
}

// tableOf allocates a table of `n` elements and writes the length header.
// The returned place points at the header, which is what every other operation
// expects.
func (e *asmEmitter) tableOf(n place, width int) (place, error) {
	// The header is eight bytes whatever the elements are, so the length is
	// always one aligned load. After it, `width` bytes each.
	bytes := e.alloc(false)
	e.move(bytes, n)
	if width == 1 {
		e.line("add %s, %d", bytes.text, asmTableHeader)
	} else {
		e.line("add %s, 1", bytes.text)
		e.line("shl %s, 3", bytes.text)
	}
	ptr, err := e.rawAlloc(bytes)
	if err != nil {
		return place{}, err
	}
	hl, err := e.materialize([]place{ptr, n}, "alloc")
	if err != nil {
		return place{}, err
	}
	e.line("mov qword ptr [%s], %s", hl[0].text, hl[1].text)
	return ptr, nil
}

// emitAlloc puts a rule in memory — the gather.
func (e *asmEmitter) emitAlloc(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) != 1 {
		return place{}, fmt.Errorf("alloc takes one table, got %s", t)
	}
	tab := args[0]
	if !isTableRule(e.tgt, tab) {
		return e.emit(tab) // a graph is already memory; a parameter is not ours to copy
	}
	rule := tab.Args()[1]
	if rule.Kind != core.KFn || len(rule.Params) != 1 {
		return place{}, fmt.Errorf("alloc's table needs an (fn (i) …) rule, got %s", rule)
	}
	n, err := e.emit(tab.Args()[0])
	if err != nil {
		return place{}, err
	}
	nHold := e.alloc(false)
	e.move(nHold, n)
	e.release(n)
	body, raw, _ := openFresh(rule, e.bound, asmIdent)
	width := ElemBytes(e.tgt, body)
	dst, err := e.tableOf(nHold, width)
	if err != nil {
		return place{}, err
	}
	idx := e.alloc(false)
	e.line("mov %s, 0", idx.text)
	e.where[raw[0]] = hold(idx)
	u := e.uniq()
	top, done := fmt.Sprintf("Lfill%d", u), fmt.Sprintf("Lfilled%d", u)
	e.label(top)
	cl, err := e.materialize([]place{idx, nHold}, "alloc")
	if err != nil {
		return place{}, err
	}
	e.line("cmp %s, %s", cl[0].text, cl[1].text)
	e.line("jge %s", done)
	v, err := e.emit(body)
	if err != nil {
		return place{}, err
	}
	fl, err := e.materialize([]place{dst, idx, v}, "alloc")
	if err != nil {
		return place{}, err
	}
	if width == 1 {
		e.line("mov byte ptr %s, %s",
			asmElemAddr(fl[0].text, fl[1].text, 1), asmByte(fl[2].text))
	} else {
		e.line("mov qword ptr %s, %s",
			asmElemAddr(fl[0].text, fl[1].text, 8), fl[2].text)
	}
	e.release(v)
	e.line("add %s, 1", idx.text)
	e.line("jmp %s", top)
	e.label(done)
	delete(e.where, raw[0])
	e.release(idx)
	e.release(nHold)
	return dst, nil
}

// emitBuild is ADR 0018's scoped mutable buffer. The buffer IS a table — the
// same pointer-with-a-header — because linearity is what makes the freeze on
// the way out free, and nothing has to change representation.
func (e *asmEmitter) emitBuild(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return place{}, fmt.Errorf("build takes a length and (fn (b) …), got %s", t)
	}
	n, err := e.emit(args[0])
	if err != nil {
		return place{}, err
	}
	nHold := e.alloc(false)
	e.move(nHold, n)
	e.release(n)
	// The body is opened BEFORE the allocation, because what it stores is what
	// decides how wide an element is — and that decides how many bytes to ask
	// for. wintables-2026-08-25 measured 3x for getting this wrong on a
	// boolean sieve.
	body, raw, _ := openFresh(args[1], e.bound, asmIdent)
	width := BufferElemBytes(e.tgt, body, raw[0])
	buf, err := e.tableOf(nHold, width)
	if err != nil {
		return place{}, err
	}
	e.release(nHold)
	e.where[raw[0]] = hold(buf)
	if width != 8 {
		e.elem[raw[0]] = width
	}
	out, err := e.emit(body)
	delete(e.where, raw[0])
	return out, err
}

// emitSet is a store. It consumes the buffer and returns it, which is the
// linear threading spelled in x86's own addressing mode.
func (e *asmEmitter) emitSet(t *core.Term) (place, error) {
	args := t.Args()
	if len(args) != 3 {
		return place{}, fmt.Errorf("set takes a buffer, an index and a value, got %s", t)
	}
	b, err := e.emit(args[0])
	if err != nil {
		return place{}, err
	}
	i, err := e.emit(args[1])
	if err != nil {
		return place{}, err
	}
	v, err := e.emit(args[2])
	if err != nil {
		return place{}, err
	}
	live, err := e.materialize([]place{b, i, v}, "set")
	if err != nil {
		return place{}, err
	}
	if w := e.elemOf(args[0]); w == 1 {
		e.line("mov byte ptr %s, %s",
			asmElemAddr(live[0].text, live[1].text, 1), asmByte(live[2].text))
	} else {
		e.line("mov qword ptr %s, %s",
			asmElemAddr(live[0].text, live[1].text, 8), live[2].text)
	}
	e.release(i)
	e.release(v)
	return b, nil
}

// emitArrayLit builds a graph in memory. There is no static-data form for one:
// staticdata-2026-08-20 measured compile-time materialisation as free of code
// on x86 and never a measurable win, so a literal table is built the same way
// any other is.
func (e *asmEmitter) emitArrayLit(t *core.Term) (place, error) {
	elems := t.Args()
	n := e.alloc(false)
	e.line("mov %s, %d", n.text, len(elems))
	// A literal's elements may differ in kind, so it takes the wide form. A
	// homogeneous bool literal paying eight bytes is a case nobody has written.
	dst, err := e.tableOf(n, 8)
	if err != nil {
		return place{}, err
	}
	e.release(n)
	for i, x := range elems {
		v, err := e.emit(x)
		if err != nil {
			return place{}, err
		}
		al, err := e.materialize([]place{dst, v}, "array")
		if err != nil {
			return place{}, err
		}
		e.line("mov qword ptr [%s+%d], %s", al[0].text, asmTableHeader+8*i, al[1].text)
		e.release(v)
	}
	return dst, nil
}

// elemOf is the element width of the table a term denotes, in bytes.
func (e *asmEmitter) elemOf(t *core.Term) int {
	if t == nil {
		return 8
	}
	if t.Kind == core.KName {
		if n, ok := e.elem[t.Name]; ok {
			return n
		}
		return 8
	}
	// A width has to be readable off the TERM as well as off a name, because a
	// table crosses a binder as an expression: `(let (build n …) (fn (c) …))`
	// binds `c` to a build, not to a name. Missing this read the sieve's
	// counting loop as qwords over a byte array — seven bytes of the following
	// elements per access, so a wrong answer rather than a slow one.
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		p, ok := e.tgt.Prims[t.Kids[0].Name]
		if !ok {
			return 8
		}
		switch p.Kind {
		case "table-build":
			if len(t.Args()) == 2 && t.Args()[1].Kind == core.KFn &&
				len(t.Args()[1].Params) == 1 {
				lam := t.Args()[1]
				return BufferElemBytes(e.tgt, lam.Closed(), lam.Params[0])
			}
		case "table-alloc":
			if len(t.Args()) == 1 && isTableRule(e.tgt, t.Args()[0]) {
				if rule := t.Args()[0].Args()[1]; rule.Kind == core.KFn {
					return ElemBytes(e.tgt, rule.Closed())
				}
			}
		case "table-set":
			// A store hands the buffer back, so the width is unchanged.
			if len(t.Args()) == 3 {
				return e.elemOf(t.Args()[0])
			}
		case "iterate":
			// A loop's value is one of its variables, and every one that can be
			// the value was seeded from an initial value. Reading the first
			// init is enough because a loop threading a buffer threads ONE.
			if len(t.Args()) >= 2 {
				for _, z := range t.Args()[1:] {
					if w := e.elemOf(z); w != 8 {
						return w
					}
				}
			}
		}
	}
	return 8
}

// carryElem propagates a table's element width across a binder. A `let` or a
// loop variable bound to a table holds the same table, so it holds the same
// element width — and losing it at one binder is enough to emit a qword load
// against a byte array, which reads seven bytes of the next elements.
func (e *asmEmitter) carryElem(name string, from *core.Term) {
	if n := e.elemOf(from); n != 8 {
		e.elem[name] = n
	}
}

// asmElemAddr spells the addressing mode for element `idx` of a table, at this
// element width. The eight-byte header is skipped in both, and the scale is
// what changes.
func asmElemAddr(base, idx string, width int) string {
	if width == 1 {
		return fmt.Sprintf("[%s+%s+%d]", base, idx, asmTableHeader)
	}
	return fmt.Sprintf("[%s+%s*8+%d]", base, idx, asmTableHeader)
}
