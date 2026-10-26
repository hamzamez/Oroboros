// Command print is backend-2026-10-26's experiment: a prototype printer of IR_P
// into LLVM IR or C, beside ir/x86's MASM, so that the three candidate languages
// for a native target print the SAME input and can be measured. It reads the
// IR's text with the IR's own reader (ADR 0038: others may read it).
//
// It covers the operations the two benchmarks use (const, add, sub, mul, rem,
// the comparisons, index, set, len, build, if, loop, branch, yield, break,
// continue) and refuses anything else. The layout is the windows target's, so
// the three paths share it: a table is a pointer to an 8-byte length and then
// its elements, a byte for a boolean and a qword for an integer (as ir/x86
// prints), allocated by VirtualAlloc, as the windows target's `build` is.
//
// The mapping is the derivation's (backend-2026-10-26 §2): an `exact` operation
// is proven inside the word, which is LLVM's `nsw` (and `nuw` when both operands
// are proven non-negative), and C's signed arithmetic; a π is a renaming; a
// region's parameters and results are slots, which LLVM's mem2reg turns into φs
// (the front-end idiom clang itself uses) and C keeps as variables.
//
//	go run ./experiments/backend/print -lang llvm FILE.ir > out.ll
//	go run ./experiments/backend/print -lang c FILE.ir > out.c
package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"oroboros/core"
	"oroboros/ir"
)

func main() {
	lang := flag.String("lang", "llvm", "llvm or c")
	flag.Parse()
	src, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fail("%v", err)
	}
	p, err := ir.Read(string(src))
	if err != nil {
		fail("%v", err)
	}
	var out strings.Builder
	switch *lang {
	case "llvm":
		out.WriteString("target triple = \"x86_64-pc-windows-msvc\"\n\n")
		out.WriteString("declare dllimport ptr @VirtualAlloc(ptr, i64, i32, i32)\n\n")
		for _, f := range p.Funcs {
			(&llvm{f: f, out: &out}).fn()
		}
	case "c":
		out.WriteString("#include <stdint.h>\n#include <stddef.h>\n")
		out.WriteString("__declspec(dllimport) void *__stdcall VirtualAlloc(void *, size_t, unsigned long, unsigned long);\n\n")
		for _, f := range p.Funcs {
			(&cprint{f: f, out: &out}).fn()
		}
	default:
		fail("-lang is llvm or c")
	}
	fmt.Print(out.String())
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "print: "+f+"\n", a...)
	os.Exit(1)
}

func cname(n string) string { return strings.ReplaceAll(n, "-", "_") }

// elem is a table type's element: its LLVM type, C type and size.
func elem(ty string) (string, string, int) {
	e := strings.TrimPrefix(strings.TrimPrefix(ty, "array "), "buffer ")
	if e == "bool" {
		return "i8", "uint8_t", 1
	}
	return "i64", "int64_t", 8
}

func isTable(ty string) bool {
	return strings.HasPrefix(ty, "array ") || strings.HasPrefix(ty, "buffer ")
}

// nonNeg reports a value's proven range lies in [0, ∞).
func nonNeg(ty string) bool {
	lo, _, ok := core.IntRangeBig(ty)
	return ok && lo.Sign() >= 0
}

func literal(t *core.Term) string {
	switch t.Kind {
	case core.KBool:
		return fmt.Sprint(t.Int)
	case core.KInt:
		return fmt.Sprint(t.Int)
	}
	if v, ok := new(big.Int).SetString(t.String(), 10); ok {
		return v.String()
	}
	fail("literal %s", core.ShowTerm(t))
	return ""
}

// ───────────────────────────── LLVM IR ─────────────────────────────

type llvm struct {
	f       *ir.Func
	out     *strings.Builder
	n       int
	alias   map[ir.V]ir.V
	slots   map[ir.V]string // a region parameter's or a result's alloca
	allocas []string
	body    strings.Builder
}

func (g *llvm) fresh(p string) string { g.n++; return fmt.Sprintf("%s%d", p, g.n) }

func (g *llvm) ty(v ir.V) string {
	t := g.f.Types[v]
	switch {
	case t == "bool":
		return "i1"
	case isTable(t):
		return "ptr"
	}
	return "i64"
}

func (g *llvm) val(v ir.V) string {
	for {
		a, ok := g.alias[v]
		if !ok {
			break
		}
		v = a
	}
	return fmt.Sprintf("%%v%d", v)
}

func (g *llvm) emit(f string, a ...any) { fmt.Fprintf(&g.body, "  "+f+"\n", a...) }
func (g *llvm) label(l string)          { fmt.Fprintf(&g.body, "%s:\n", l) }

func (g *llvm) slot(v ir.V) string {
	if s, ok := g.slots[v]; ok {
		return s
	}
	s := g.fresh("%s")
	g.slots[v] = s
	g.allocas = append(g.allocas, fmt.Sprintf("  %s = alloca %s", s, g.ty(v)))
	return s
}

// target is where a region's yield, break or continue sends its arguments:
// the slots to store into and the label to jump to.
type target struct {
	slots []ir.V
	label string
}

type exits struct{ yield, brk, cont *target }

func (g *llvm) fn() {
	g.alias, g.slots = map[ir.V]ir.V{}, map[ir.V]string{}
	g.region(g.f.Body, exits{})
	// nounwind: no function of ours unwinds (a crash is ExitProcess), so no
	// unwind tables. minsize optsize: what a size-first target asks for; a .ll
	// input gets them only from its printer, never from -Os.
	attrs := "nounwind"
	if *minsize {
		attrs += " minsize optsize"
	}
	fmt.Fprintf(g.out, "define dso_local i64 @%s() %s {\nentry:\n", cname(g.f.Name), attrs)
	for _, a := range g.allocas {
		fmt.Fprintln(g.out, a)
	}
	g.out.WriteString(g.body.String())
	g.out.WriteString("}\n")
}

func (g *llvm) send(t *target, args []ir.V) {
	for i, a := range args {
		g.emit("store %s %s, ptr %s", g.ty(a), g.val(a), g.slot(t.slots[i]))
	}
	g.emit("br label %%%s", t.label)
}

func (g *llvm) region(r *ir.Region, x exits) {
	for _, pi := range r.Pis {
		g.alias[pi.V] = pi.Of
	}
	for i := range r.Stmts {
		g.stmt(&r.Stmts[i], x)
	}
	switch r.T {
	case ir.TYield:
		if x.yield == nil {
			g.emit("ret i64 %s", g.val(r.Args[0]))
			return
		}
		g.send(x.yield, r.Args)
	case ir.TBreak:
		g.send(x.brk, r.Args)
	case ir.TContinue:
		g.send(x.cont, r.Args)
	case ir.TBranch:
		if *selects && pureContinue(r.Then) && pureContinue(r.Else) && x.cont != nil {
			// A BRANCH BOTH OF WHOSE ARMS GO ROUND THE LOOP with pure arguments
			// is one back edge whose arguments are chosen: compute both arms,
			// then `select`. Sound under poison: select does not propagate it
			// from the operand it does not choose (LangRef, `select`).
			g.region(&ir.Region{Pis: r.Then.Pis, Stmts: r.Then.Stmts, T: ir.TYield}, exits{yield: &target{label: "-"}})
			g.trimLastBr()
			g.region(&ir.Region{Pis: r.Else.Pis, Stmts: r.Else.Stmts, T: ir.TYield}, exits{yield: &target{label: "-"}})
			g.trimLastBr()
			for i := range r.Then.Args {
				ty := g.ty(r.Then.Args[i])
				sel := g.fresh("%sel")
				g.emit("%s = select i1 %s, %s %s, %s %s", sel, g.val(r.Cond), ty,
					g.val(r.Then.Args[i]), ty, g.val(r.Else.Args[i]))
				g.emit("store %s %s, ptr %s", ty, sel, g.slot(x.cont.slots[i]))
			}
			g.emit("br label %%%s", x.cont.label)
			return
		}
		th, el := g.fresh("then"), g.fresh("else")
		g.emit("br i1 %s, label %%%s, label %%%s", g.val(r.Cond), th, el)
		g.label(th)
		g.region(r.Then, x)
		g.label(el)
		g.region(r.Else, x)
	default:
		fail("terminator %v", r.T)
	}
}

func (g *llvm) stmt(s *ir.Stmt, x exits) {
	res := func() string { return g.val(s.Res[0]) }
	arg := func(i int) string { return g.val(s.Args[i]) }
	flags := func() string {
		if s.Mode != ir.MExact {
			fail("an operation not proven exact: %s", s.Op)
		}
		f := " nsw"
		if nonNeg(g.f.Types[s.Args[0]]) && nonNeg(g.f.Types[s.Args[1]]) {
			f = " nuw nsw"
		}
		return f
	}
	switch s.Op {
	case ir.OConst:
		if g.ty(s.Res[0]) == "i1" {
			g.emit("%s = or i1 %s, false", res(), literal(s.Lit))
		} else {
			g.emit("%s = add i64 %s, 0", res(), literal(s.Lit))
		}
	case ir.OAdd, ir.OSub, ir.OMul:
		op := map[ir.Op]string{ir.OAdd: "add", ir.OSub: "sub", ir.OMul: "mul"}[s.Op]
		g.emit("%s = %s%s i64 %s, %s", res(), op, flags(), arg(0), arg(1))
	case ir.ORem, ir.ODiv:
		// The divisor is proven non-zero (integers.md §5), and the quotient in
		// the word: neither of sdiv's undefined cases is reachable.
		op := map[ir.Op]string{ir.ORem: "srem", ir.ODiv: "sdiv"}[s.Op]
		g.emit("%s = %s i64 %s, %s", res(), op, arg(0), arg(1))
	case ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		c := map[ir.Op]string{ir.OEq: "eq", ir.ONe: "ne", ir.OLt: "slt", ir.OLe: "sle", ir.OGt: "sgt", ir.OGe: "sge"}[s.Op]
		g.emit("%s = icmp %s %s %s, %s", res(), c, g.ty(s.Args[0]), arg(0), arg(1))
	case ir.OIndex:
		et, _, _ := elem(g.f.Types[s.Args[0]])
		p := g.fresh("%p")
		g.emit("%s = getelementptr inbounds %s, ptr %s, i64 %s", p, et, g.dataPtr(arg(0)), arg(1))
		if et == "i8" {
			b := g.fresh("%b")
			g.emit("%s = load i8, ptr %s", b, p)
			g.emit("%s = icmp ne i8 %s, 0", res(), b)
		} else {
			g.emit("%s = load i64, ptr %s", res(), p)
		}
	case ir.OSet:
		et, _, _ := elem(g.f.Types[s.Args[0]])
		p := g.fresh("%p")
		g.emit("%s = getelementptr inbounds %s, ptr %s, i64 %s", p, et, g.dataPtr(arg(0)), arg(1))
		v := arg(2)
		if et == "i8" {
			w := g.fresh("%z")
			g.emit("%s = zext i1 %s to i8", w, v)
			v = w
		}
		g.emit("store %s %s, ptr %s", et, v, p)
		g.alias[s.Res[0]] = s.Args[0]
	case ir.OLen:
		g.emit("%s = load i64, ptr %s", res(), arg(0))
	case ir.OBuild:
		_, _, size := elem(g.f.Types[s.Res[0]])
		bytes := g.fresh("%n")
		g.emit("%s = mul nuw nsw i64 %s, %d", bytes, arg(0), size)
		tot := g.fresh("%n")
		g.emit("%s = add nuw nsw i64 %s, 8", tot, bytes)
		p := g.fresh("%a")
		g.emit("%s = call ptr @VirtualAlloc(ptr null, i64 %s, i32 12288, i32 4)", p, tot)
		g.emit("store i64 %s, ptr %s", arg(0), p)
		body := s.Sub[0]
		g.alias[body.Params[0]] = 0 // replaced below
		delete(g.alias, body.Params[0])
		g.emit("%s = getelementptr i8, ptr %s, i64 0", g.val(body.Params[0]), p)
		join := g.fresh("built")
		g.region(body, exits{yield: &target{slots: s.Res, label: join}, brk: x.brk, cont: x.cont})
		g.label(join)
		g.load(s.Res)
	case ir.OIf:
		th, el, join := g.fresh("ifthen"), g.fresh("ifelse"), g.fresh("ifjoin")
		g.emit("br i1 %s, label %%%s, label %%%s", arg(0), th, el)
		y := &target{slots: s.Res, label: join}
		g.label(th)
		g.region(s.Sub[0], exits{yield: y, brk: x.brk, cont: x.cont})
		g.label(el)
		g.region(s.Sub[1], exits{yield: y, brk: x.brk, cont: x.cont})
		g.label(join)
		g.load(s.Res)
	case ir.OLoop:
		body := s.Sub[0]
		head, exit := g.fresh("head"), g.fresh("exit")
		for i, a := range s.Args {
			g.emit("store %s %s, ptr %s", g.ty(a), g.val(a), g.slot(body.Params[i]))
		}
		g.emit("br label %%%s", head)
		g.label(head)
		g.load(body.Params)
		g.region(body, exits{brk: &target{slots: s.Res, label: exit}, cont: &target{slots: body.Params, label: head}})
		g.label(exit)
		g.load(s.Res)
	default:
		fail("operation %s is outside the prototype", s.Op)
	}
}

var minsize = flag.Bool("minsize", false, "llvm: ask for the smallest code (minsize optsize)")

var selects = flag.Bool("select", false, "llvm: print a branch whose arms both continue with pure arguments as one select")

// pureContinue reports a region that goes round its loop computing only pure
// arithmetic: no memory, no control.
func pureContinue(r *ir.Region) bool {
	if r.T != ir.TContinue {
		return false
	}
	for _, s := range r.Stmts {
		switch s.Op {
		case ir.OConst, ir.OAdd, ir.OSub, ir.OMul, ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		default:
			return false
		}
	}
	return true
}

// trimLastBr removes the jump a yield to the dummy target printed.
func (g *llvm) trimLastBr() {
	s := g.body.String()
	if i := strings.LastIndex(s, "  br label %-\n"); i >= 0 && i+len("  br label %-\n") == len(s) {
		g.body.Reset()
		g.body.WriteString(s[:i])
	}
}

func (g *llvm) dataPtr(p string) string {
	d := g.fresh("%d")
	g.emit("%s = getelementptr inbounds i8, ptr %s, i64 8", d, p)
	return d
}

func (g *llvm) load(vs []ir.V) {
	for _, v := range vs {
		g.emit("%s = load %s, ptr %s", g.val(v), g.ty(v), g.slot(v))
	}
}

// ───────────────────────────── C ─────────────────────────────

type cprint struct {
	f     *ir.Func
	out   *strings.Builder
	n     int
	alias map[ir.V]ir.V
	decls map[ir.V]bool
	body  strings.Builder
}

func (g *cprint) fresh(p string) string { g.n++; return fmt.Sprintf("%s%d", p, g.n) }

func (g *cprint) ty(v ir.V) string {
	t := g.f.Types[v]
	switch {
	case t == "bool":
		return "int"
	case isTable(t):
		return "char *"
	}
	return "int64_t"
}

func (g *cprint) val(v ir.V) string {
	for {
		a, ok := g.alias[v]
		if !ok {
			break
		}
		v = a
	}
	g.decls[v] = true
	return fmt.Sprintf("v%d", v)
}

func (g *cprint) emit(f string, a ...any) { fmt.Fprintf(&g.body, "  "+f+"\n", a...) }

type ctarget struct {
	vars  []ir.V
	label string
}
type cexits struct{ yield, brk, cont *ctarget }

func (g *cprint) fn() {
	g.alias, g.decls = map[ir.V]ir.V{}, map[ir.V]bool{}
	g.region(g.f.Body, cexits{})
	fmt.Fprintf(g.out, "int64_t %s(void) {\n", cname(g.f.Name))
	for v := range g.f.Types {
		if g.decls[ir.V(v)] {
			fmt.Fprintf(g.out, "  %s v%d;\n", g.ty(ir.V(v)), v)
		}
	}
	g.out.WriteString(g.body.String())
	g.out.WriteString("}\n")
}

// send is a parallel assignment, then a jump: every argument is read before any
// target is written.
func (g *cprint) send(t *ctarget, args []ir.V) {
	var tmps []string
	for i, a := range args {
		tmp := g.fresh("t")
		g.emit("%s %s = %s;", g.ty(t.vars[i]), tmp, g.val(a))
		tmps = append(tmps, tmp)
	}
	for i := range args {
		g.emit("%s = %s;", g.val(t.vars[i]), tmps[i])
	}
	g.emit("goto %s;", t.label)
}

func (g *cprint) region(r *ir.Region, x cexits) {
	for _, pi := range r.Pis {
		g.alias[pi.V] = pi.Of
	}
	for i := range r.Stmts {
		g.stmt(&r.Stmts[i], x)
	}
	switch r.T {
	case ir.TYield:
		if x.yield == nil {
			g.emit("return %s;", g.val(r.Args[0]))
			return
		}
		g.send(x.yield, r.Args)
	case ir.TBreak:
		g.send(x.brk, r.Args)
	case ir.TContinue:
		g.send(x.cont, r.Args)
	case ir.TBranch:
		g.emit("if (%s) {", g.val(r.Cond))
		g.region(r.Then, x)
		g.emit("} else {")
		g.region(r.Else, x)
		g.emit("}")
	}
}

func (g *cprint) stmt(s *ir.Stmt, x cexits) {
	res := func() string { return g.val(s.Res[0]) }
	arg := func(i int) string { return g.val(s.Args[i]) }
	switch s.Op {
	case ir.OConst:
		g.emit("%s = %s;", res(), literal(s.Lit))
	case ir.OAdd, ir.OSub, ir.OMul, ir.ORem, ir.ODiv:
		op := map[ir.Op]string{ir.OAdd: "+", ir.OSub: "-", ir.OMul: "*", ir.ORem: "%", ir.ODiv: "/"}[s.Op]
		g.emit("%s = %s %s %s;", res(), arg(0), op, arg(1))
	case ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		c := map[ir.Op]string{ir.OEq: "==", ir.ONe: "!=", ir.OLt: "<", ir.OLe: "<=", ir.OGt: ">", ir.OGe: ">="}[s.Op]
		g.emit("%s = %s %s %s;", res(), arg(0), c, arg(1))
	case ir.OIndex:
		_, ct, _ := elem(g.f.Types[s.Args[0]])
		g.emit("%s = ((%s *)(%s + 8))[%s];", res(), ct, arg(0), arg(1))
	case ir.OSet:
		_, ct, _ := elem(g.f.Types[s.Args[0]])
		g.emit("((%s *)(%s + 8))[%s] = (%s)%s;", ct, arg(0), arg(1), ct, arg(2))
		g.alias[s.Res[0]] = s.Args[0]
	case ir.OLen:
		g.emit("%s = *(int64_t *)%s;", res(), arg(0))
	case ir.OBuild:
		_, _, size := elem(g.f.Types[s.Res[0]])
		body := s.Sub[0]
		g.emit("%s = (char *)VirtualAlloc(0, (size_t)(%s * %d + 8), 0x3000, 4);", g.val(body.Params[0]), arg(0), size)
		g.emit("*(int64_t *)%s = %s;", g.val(body.Params[0]), arg(0))
		join := g.fresh("built")
		g.region(body, cexits{yield: &ctarget{vars: s.Res, label: join}, brk: x.brk, cont: x.cont})
		g.emit("%s:;", join)
	case ir.OIf:
		join := g.fresh("ifjoin")
		y := &ctarget{vars: s.Res, label: join}
		g.emit("if (%s) {", arg(0))
		g.region(s.Sub[0], cexits{yield: y, brk: x.brk, cont: x.cont})
		g.emit("} else {")
		g.region(s.Sub[1], cexits{yield: y, brk: x.brk, cont: x.cont})
		g.emit("}")
		g.emit("%s:;", join)
	case ir.OLoop:
		body := s.Sub[0]
		head, exit := g.fresh("head"), g.fresh("exit")
		for i, a := range s.Args {
			g.emit("%s = %s;", g.val(body.Params[i]), g.val(a))
		}
		g.emit("%s:;", head)
		g.region(body, cexits{brk: &ctarget{vars: s.Res, label: exit}, cont: &ctarget{vars: body.Params, label: head}})
		g.emit("%s:;", exit)
	default:
		fail("operation %s is outside the prototype", s.Op)
	}
}
