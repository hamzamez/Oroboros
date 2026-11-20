package llvm

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A λ THE HOST APPLIES DURING A CALL IS CLOSURE-CONVERTED (ADR 0143 item 6;
// Minamide, Morrisett and Harper 1996): the closure ∃E. (E × A → R) × E, its
// code a function of ours and E the values its region reads from the caller,
// a record in the caller's frame (the λ is second-class, so the frame outlives
// every activation). E's address travels in the API's context word where the
// claim's C signature names `ctx` (291 of 349 Win32 APIs), and through a slot
// otherwise, written before the call and restored after it, which nesting
// makes sound on one thread (shallow binding, Baker 1978).
//
// The lifted function takes the λ's parameters at the IR's types, then E's
// address, and loads each captured value at entry under the name the region
// already reads, so the region prints as it stands. The C wrapper the host
// calls is the window procedure's (CallbackWrapper), at the claim's
// convention and C's types.

// fnOutline prints the `fn` statement s: E's allocation and its stores here,
// the lifted function and its wrapper among the module's definitions. Its
// value is the wrapper's address, and E's address is what ctx(f) passes.
func (p *printer) fnOutline(s *ir.Stmt) {
	body := s.Sub[0]
	q := strings.TrimPrefix(s.Type, core.CallbackPrefix)
	info, err := ir.CallbackSignature(p.tg, q)
	if err != nil {
		p.fail("%v", err)
		return
	}
	d, _, _ := p.tg.CallbackDecl(q)
	caps := p.captures(body)
	p.fns++
	name := fmt.Sprintf("%s~fn%d", p.f.Name, p.fns)
	env := "null"
	var envTy string
	if len(caps) > 0 {
		var tys []string
		for _, v := range caps {
			tys = append(tys, p.ty(v))
		}
		envTy = "{ " + strings.Join(tys, ", ") + " }"
		env = "%" + p.fresh("env")
		p.allocas = append(p.allocas, fmt.Sprintf("  %s = alloca %s", env, envTy))
		for k, v := range caps {
			g := "%" + p.fresh("e")
			p.line("%s = getelementptr inbounds %s, ptr %s, i32 0, i32 %d", g, envTy, env, k)
			p.line("store %s %s, ptr %s", p.ty(v), p.val(v), g)
		}
	}
	p.names[s.Res[0]] = Name(name + ir.WrapperSuffix)
	if p.envOf == nil {
		p.envOf, p.slotOf = map[ir.V]string{}, map[ir.V]string{}
	}
	p.envOf[s.Res[0]] = env
	if !info.Ctx {
		g := fmt.Sprintf("@%q", name+"~env")
		emit.LLVMImports[g+" = internal global ptr null"] = true
		p.slotOf[s.Res[0]] = g
	}
	// the lifted function, printed by a printer of its own over the same
	// function's values
	fc := *p.f
	fc.Results = []string{d.Result}
	sub := &printer{tg: p.tg, f: &fc, pl: p.pl, slots: map[ir.V]string{}, names: map[ir.V]string{},
		dbl: p.dbl, wideNow: map[ir.V]bool{}, adder: p.adder, absorbed: p.absorbed}
	envp := "%ctx"
	if !info.Ctx {
		envp = "%envp"
		sub.line("%%envp = load ptr, ptr %s", p.slotOf[s.Res[0]])
	}
	if PlantCtxSwapped && info.Ctx && len(body.Params) > 0 {
		envp = fmt.Sprintf("%%v%d", body.Params[0])
	}
	// A PARAMETER IN S₃₂ is the word at the boundary and an i32 inside it,
	// narrowed once at entry, as a function's is (ADR 0127)
	for _, v := range body.Params {
		if sub.s32(v) {
			n := fmt.Sprintf("%%v%d.n", v)
			sub.line("%s = trunc i64 %%v%d to i32", n, v)
			sub.names[v] = n
		}
	}
	for k, v := range caps {
		sub.line("%%c%d.p = getelementptr inbounds %s, ptr %s, i32 0, i32 %d", k, envTy, envp, k)
		sub.line("%%c%d = load %s, ptr %%c%d.p", k, p.ty(v), k)
		sub.names[p.pl.Res(v)] = fmt.Sprintf("%%c%d", k)
	}
	sub.region(body, exits{ret: true})
	if sub.err != nil {
		p.fail("%v", sub.err)
		return
	}
	var params []string
	for _, v := range body.Params {
		params = append(params, fmt.Sprintf("%s %%v%d", p.llty(p.f.Types[v]), v))
	}
	if info.Ctx {
		params = append(params, "ptr %ctx")
	}
	var def strings.Builder
	fmt.Fprintf(&def, "define internal %s %s(%s) #0 {\nentry:\n", sub.retType(), Name(name), strings.Join(params, ", "))
	for _, a := range sub.allocas {
		def.WriteString(a + "\n")
	}
	def.WriteString(sub.b.String())
	def.WriteString("}\n")
	w, err := CallbackWrapper(p.tg, def.String(), name, info)
	if err != nil {
		p.fail("%v", err)
		return
	}
	emit.LLVMImports[def.String()+"\n"+w] = true
}

// captures are the values region r reads and does not define: what E holds.
// A constant or a zero is spelled where it is read, so it is no capture.
func (p *printer) captures(r *ir.Region) []ir.V {
	defined := map[ir.V]bool{}
	used := map[ir.V]bool{}
	var walk func(r *ir.Region)
	walk = func(r *ir.Region) {
		for _, v := range r.Params {
			defined[v] = true
		}
		for _, pi := range r.Pis {
			defined[pi.V] = true
			used[pi.Of] = true
			if pi.Other >= 0 {
				used[pi.Other] = true
			}
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				defined[v] = true
			}
			for _, a := range s.Args {
				used[a] = true
			}
			for _, a := range s.Store {
				used[a] = true
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		for _, a := range r.Args {
			used[a] = true
		}
		if r.T == ir.TBranch {
			used[r.Cond] = true
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	seen := map[ir.V]bool{}
	var out []ir.V
	for v := range used {
		res := p.pl.Res(v)
		if defined[v] || defined[res] || seen[res] || p.pl.Const[res] != nil || p.pl.Zero[res] {
			continue
		}
		seen[res] = true
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PlantCtxSwapped makes the lifted function read E from its first parameter
// instead of the context word: the λ then reads its captures from wherever the
// host's handle points (TestACallbackCapturesItsEnvironmentThroughTheContextWord).
var PlantCtxSwapped bool
