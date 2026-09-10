//go:build ignore

// js.go — how much of the JavaScript runtime this language can declare, and why
// the question the other two surveys ask does not survive contact with this
// host.
//
//	node gauntlet/stdlib/jsdump.mjs > /tmp/js-api.txt
//	go run gauntlet/stdlib/js.go -api /tmp/js-api.txt [-emit DIR]
//
// THE HEADLINE IS THAT THE HEADLINE IS MEANINGLESS, and saying so is the point.
//
// Go's survey and the JVM's both compute DECLARABLE by asking whether every type
// in a signature can be named, and USABLE by a least fixed point over the types
// a program can obtain. Neither question has content here. `targets/js` declares
// every type `any` — deliberately, and target-native.md says why — so:
//
//	DECLARABLE is ~100% and it measures nothing. Any host function can be given
//	`(prim f ((a0 any) (a1 any)) any expr "f(%s, %s)")`, because `any` demands
//	nothing of its argument. Reporting that as a capability would be the FIFTH
//	time a survey's first number described the measurer, after methods-at-0%,
//	Win32's typedef residue, Win32's voids, and the Go survey's collapsed type
//	names — and this one would be the largest.
//
//	USABLE has no domain. The fixed point asks which host types a program can
//	obtain; `any` is one type and everything is it, so the answer is "all of it"
//	and means "we know nothing". That is the same fact that made ADR 0019's
//	bounded-by-default VACUOUS on JavaScript (bigrep-2026-09-02): the analysis
//	counted zero integer operations because a language operator inherited `any`
//	from the host, and a check with nothing to prove reports success.
//
// SO THIS SURVEY MEASURES WHAT IS ACTUALLY THERE: the SHAPE. What a JavaScript
// declaration needs is a name, a call form and an ARITY, and exactly one of the
// three is in doubt.
//
//	A NAME — a Symbol-keyed member has none, and a member whose key is not an
//	identifier cannot be written. Both are countable and both are small.
//
//	A CALL FORM — `new X(…)` against `f(…)` against `%s.m(…)` against a getter's
//	`%s.p`. Getting it wrong throws at RUN time, because nothing here fails to
//	compile. Countable, and the distinction is in the descriptor.
//
//	AN ARITY — and THIS is the finding. `Function.length` is the count of
//	parameters before the first default or rest, and a NATIVE function's
//	parameters are not introspectable at all. `Math.max.length` is 2 and
//	`Math.max(1,2,3,4)` is 4; `console.log.length` is 0. So the host does not
//	merely decline to tell us the types — IT MISREPORTS THE SHAPE, which is the
//	one thing we needed from it.
//
// The comparison that makes this worth writing: win32-2026-09-06 found that the
// host with no type system priced BETTER than Go, because "on x86-64 every
// handle, pointer and integer is ONE REGISTER, so there is nothing to be opaque
// about". JavaScript is that observation taken to its limit and inverted:
// everything is one dynamic value, so nothing is opaque AND nothing is checked.
// Opacity is a property of the pair; so is checkability.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type member struct {
	root  string // globalThis, or node:fs
	path  string // globalThis.Math.max
	kind  string // function | class | getter | value | namespace
	arity int
	named bool // the key is a writable identifier
	ours  bool // the host names it and OUR reader cannot
}

// recv splits a member path into the thing a template writes and the receiver,
// if it has one. A `.prototype.` in the path is what marks an INSTANCE method:
// `globalThis.Array.prototype.map` is `%s.map(…)` with the receiver as argument
// 0, which is the convention Win32's `HANDLE` used and gomethods-2026-09-09
// generalised to every Go method.
func (m member) instance() bool { return strings.Contains(m.path, ".prototype.") }

// owner is the class or namespace a member hangs off.
func (m member) owner() string {
	p := strings.TrimSuffix(m.path, "."+m.leaf())
	return strings.TrimSuffix(p, ".prototype")
}

func (m member) leaf() string {
	i := strings.LastIndex(m.path, ".")
	return m.path[i+1:]
}

// hostPath is what a template writes for a STATIC member: the path with the
// `globalThis.` prefix removed, because `globalThis.Math.max` is spelled
// `Math.max` in any JavaScript that is not deliberately obscure, and a `node:`
// module is reached through the import the target already declares.
func (m member) hostPath() string {
	p := strings.TrimPrefix(m.path, "globalThis.")
	if strings.HasPrefix(m.root, "node:") {
		// THE MODULE KEEPS ITS ALIAS. `(import "node:fs")` binds the module to
		// `fs`, which is what lib/os/js.oro's templates already write — so
		// `node:path.join` is `path.join(…)` and NOT `join(…)`. Stripping the
		// module name was the first version and it emitted a call to a name
		// nothing binds.
		alias := strings.TrimPrefix(m.root, "node:")
		if i := strings.LastIndex(alias, "/"); i >= 0 {
			alias = alias[i+1:]
		}
		p = alias + strings.TrimPrefix(p, m.root)
	}
	return p
}

// readable is OUR rule, not JavaScript's, and the distinction matters enough to
// count separately. `core/read.go` allows letters, digits, marks, `_` and the
// symbol characters `-+*/<>=!?_%&|^~` — deliberately NOT `$`, because a target
// file renaming a host operation would be "teaching the reader's limitations
// rather than the host's", which that file says is the opposite of what a
// parasite target is for.
//
// So `RegExp.$1` and `RegExp.$&` are names the host HAS and we cannot write.
// This is the only refusal in this whole survey that is ours, and it is nine
// names.
func readable(n string) bool {
	const sym = "-+*/<>=!?_%&|^~"
	for i, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		case strings.ContainsRune(sym, r):
		default:
			return false
		}
	}
	return n != ""
}

func main() {
	api := flag.String("api", "", "the manifest written by gauntlet/stdlib/jsdump.mjs")
	emitDir := flag.String("emit", "", "write the declarable subset as target files into DIR")
	flag.Parse()
	if *api == "" {
		fmt.Fprintln(os.Stderr, "usage: node gauntlet/stdlib/jsdump.mjs > /tmp/js-api.txt")
		fmt.Fprintln(os.Stderr, "       go run gauntlet/stdlib/js.go -api /tmp/js-api.txt [-emit DIR]")
		os.Exit(2)
	}
	fh, err := os.Open(*api)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer fh.Close()

	var ms []member
	var roots []string
	symbols, symbolHolders := 0, 0
	root := ""
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "root":
			root = f[1]
			roots = append(roots, root)
		case "sym":
			if len(f) == 3 {
				symbolHolders++
				var n int
				fmt.Sscanf(f[2], "%d", &n)
				symbols += n
			}
		case "mem":
			if len(f) < 4 {
				continue
			}
			var a int
			fmt.Sscanf(f[3], "%d", &a)
			m := member{
				root: root, path: f[1], kind: f[2], arity: a,
				named: !(len(f) > 4 && f[4] == "unnameable"),
			}
			if m.named && !readable(m.leaf()) {
				m.named, m.ours = false, true
			}
			ms = append(ms, m)
		}
	}

	byKind := map[string]int{}
	callable, unnameable, instance, callableAll, unreadable := 0, 0, 0, 0, 0
	zeroArity, byArity := 0, map[int]int{}
	for _, m := range ms {
		byKind[m.kind]++
		if m.kind == "function" || m.kind == "class" || m.kind == "getter" {
			callableAll++
		}
		if !m.named {
			// COUNTED AGAINST THE CALLABLE SURFACE, not against every member.
			// A `value` nobody can name is a constant, and reporting it as a
			// refusal would put data in the denominator — which is the shape of
			// four earlier survey errors here.
			if m.kind == "function" || m.kind == "class" || m.kind == "getter" {
				unnameable++
				if m.ours {
					unreadable++
				}
			}
			continue
		}
		switch m.kind {
		case "function", "class":
			callable++
			byArity[m.arity]++
			if m.arity == 0 {
				zeroArity++
			}
			if m.instance() {
				instance++
			}
		case "getter":
			callable++
			if m.instance() {
				instance++
			}
		}
	}

	fmt.Printf("THE JAVASCRIPT RUNTIME, %d roots (globalThis + %d node: modules), %d members\n",
		len(roots), len(roots)-1, len(ms))
	var kinds []string
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	fmt.Print("  ")
	for _, k := range kinds {
		fmt.Printf("%s %d   ", k, byKind[k])
	}
	fmt.Println()
	fmt.Printf("\nCALLABLE SURFACE (function + class + getter): %d\n", callable)
	fmt.Printf("  of those, %d are INSTANCE members, reached through `.prototype.`\n", instance)
	fmt.Printf("  and declared with the receiver as argument 0.\n")

	fmt.Printf("\nDECLARABLE: %d of %d callable, %.1f%% — AND THIS NUMBER MEASURES NOTHING\n",
		callable, callableAll, 100*float64(callable)/float64(callableAll))
	fmt.Printf("  `targets/js` declares every type `any`, and `any` demands nothing, so\n")
	fmt.Printf("  every host function is declarable BY CONSTRUCTION. The only refusals\n")
	fmt.Printf("  are names that cannot be written at all:\n")
	fmt.Printf("    %d callable members have a key that is not an identifier\n", unnameable-unreadable)
	fmt.Printf("    %d have a key the HOST accepts and OUR READER does not -- `RegExp.$1`,\n", unreadable)
	fmt.Printf("      because core/read.go omits `$` on purpose. The only refusal here\n")
	fmt.Printf("      that is ours rather than the host's.\n")
	fmt.Printf("    %d Symbol-keyed members across %d objects have no name to declare\n",
		symbols, symbolHolders)
	fmt.Printf("  Reporting ~100%% as a capability would be the fifth time a survey's\n")
	fmt.Printf("  first number described the measurer, and the largest.\n")

	fmt.Printf("\nUSABLE: NOT ANSWERABLE FROM THIS HOST, and that is the result.\n")
	fmt.Printf("  Go's survey and the JVM's compute it as a least fixed point over the\n")
	fmt.Printf("  types a program can OBTAIN. Here there is one type, `any`, and every\n")
	fmt.Printf("  value has it — so the fixed point is total and says nothing. It is the\n")
	fmt.Printf("  same fact that made ADR 0019 vacuous on JavaScript: a check with\n")
	fmt.Printf("  nothing to prove reports success (bigrep-2026-09-02).\n")

	arityReport(ms, byArity, zeroArity, callable)

	if *emitDir != "" {
		if err := emit(*emitDir, ms); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
	}
}

// arityReport prices the one thing a JavaScript declaration genuinely needs and
// the host genuinely will not say.
func arityReport(ms []member, byArity map[int]int, zero, callable int) {
	fmt.Printf("\nTHE ARITY, WHICH IS THE ONE THING WE NEEDED AND THE ONE THING IT MISREPORTS\n")
	var ks []int
	for k := range byArity {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	fmt.Print("  declared arity: ")
	for _, k := range ks {
		if k <= 4 {
			fmt.Printf("%d→%d  ", k, byArity[k])
		}
	}
	fmt.Printf("(and %d at 5 or more)\n", func() int {
		n := 0
		for k, v := range byArity {
			if k >= 5 {
				n += v
			}
		}
		return n
	}())
	fmt.Printf("  %d of %d callable members report arity ZERO (%.1f%%), which for a\n",
		zero, callable, 100*float64(zero)/float64(callable))
	fmt.Printf("  native function means either that it takes nothing or that nothing can\n")
	fmt.Printf("  be known: `console.log.length` is 0 and it is variadic.\n")

	// THE WITNESSES, NAMED, because a claim about a host should be checkable in
	// one line at a REPL rather than believed.
	witness := map[string]int{
		"Math.max":                -1,
		"Math.min":                -1,
		"Array.prototype.push":    -1,
		"Array.prototype.map":     -1,
		"String.prototype.concat": -1,
		"console.log":             -1,
	}
	for _, m := range ms {
		p := m.hostPath()
		if _, want := witness[p]; want && m.kind == "function" {
			witness[p] = m.arity
		}
	}
	var wn []string
	for k := range witness {
		wn = append(wn, k)
	}
	sort.Strings(wn)
	fmt.Printf("  Witnesses, every one variadic or under-reported:\n")
	for _, k := range wn {
		if witness[k] >= 0 {
			fmt.Printf("    %-24s reports %d\n", k, witness[k])
		}
	}
	fmt.Printf("  A DECLARATION MUST THEREFORE NAME ITS ARITY AND THE HOST CANNOT CHECK IT,\n")
	fmt.Printf("  which is overloading.md's Println/Println2/Println3 arriving as the only\n")
	fmt.Printf("  honest shape: one declaration per arity a program actually uses.\n")
}

// ----------------------------------------------------------------------- emit
//
// A name counted declarable and never emitted is a claim — win32-2026-09-08's
// sentence, which gomethods-2026-09-09 then had to apply to the Go survey. So
// every nameable callable member is written.
//
// FOUR CALL FORMS, and choosing wrong throws at RUN time rather than failing to
// compile, which is this host's whole hazard:
//
//	function   `path(%s, …)`
//	class      `new path(%s, …)`
//	instance   `%s.name(…)`, the receiver as argument 0
//	getter     `%s.name` with a receiver, or `path` without
func emit(dir string, ms []member) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	byRoot := map[string][]member{}
	for _, m := range ms {
		if !m.named || (m.kind != "function" && m.kind != "class" && m.kind != "getter") {
			continue
		}
		byRoot[m.root] = append(byRoot[m.root], m)
	}
	var roots []string
	for r := range byRoot {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	n, overloaded := 0, 0
	for _, r := range roots {
		list := byRoot[r]
		sort.Slice(list, func(i, j int) bool { return list[i].path < list[j].path })
		mods := map[string]*strings.Builder{}
		modBuf := func(name string) *strings.Builder {
			if b, okb := mods[name]; okb {
				return b
			}
			b := &strings.Builder{}
			mods[name] = b
			return b
		}
		base := strings.TrimPrefix(r, "node:")
		if r == "globalThis" {
			base = "global"
		}
		taken := map[string]int{}
		for _, m := range list {
			// THE MODULE IS THE OWNER, and the owner has to be computed after
			// the root prefix is gone: `node:path`'s own members belong to the
			// module `js/path`, and taking the last dotted segment of the raw
			// path gave `js/path/node:path`, which the reader refused for the
			// colon. A root has no dot in it, so there was nothing to strip.
			mod := "js/" + base
			if owner := m.owner(); owner != m.root {
				leaf := owner
				if i := strings.LastIndex(leaf, "."); i >= 0 {
					leaf = leaf[i+1:]
				}
				if leaf != "" && leaf != base {
					mod += "/" + leaf
				}
			}
			var args, holes []string
			if m.instance() || (m.kind == "getter" && m.instance()) {
				args = append(args, "(self any)")
				holes = append(holes, "%s")
			}
			for k := 0; k < m.arity; k++ {
				args = append(args, fmt.Sprintf("(a%d any)", k))
				holes = append(holes, "%s")
			}
			argList := "(none)"
			if len(args) > 0 {
				argList = "(" + strings.Join(args, " ") + ")"
			}
			callArgs := holes
			if m.instance() {
				callArgs = holes[1:]
			}
			var call string
			switch {
			case m.kind == "getter" && m.instance():
				call = "%s." + m.leaf()
			case m.kind == "getter":
				call = m.hostPath()
			case m.instance():
				call = "%s." + m.leaf() + "(" + strings.Join(callArgs, ", ") + ")"
			case m.kind == "class":
				// PARENTHESISED, for the reason a Go composite literal and a
				// Java `new` are: the template is a VALUE wherever it lands.
				call = "(new " + m.hostPath() + "(" + strings.Join(callArgs, ", ") + "))"
			default:
				call = m.hostPath() + "(" + strings.Join(callArgs, ", ") + ")"
			}
			pname := m.leaf()
			key := mod + "." + pname
			taken[key]++
			if k := taken[key]; k > 1 {
				pname = fmt.Sprintf("%s%d", pname, k)
				overloaded++
			}
			imp := ""
			if strings.HasPrefix(r, "node:") {
				imp = fmt.Sprintf(" (import %q)", r)
			}
			fmt.Fprintf(modBuf(mod), "    (prim %s %s any expr %q%s)\n", pname, argList, call, imp)
			n++
		}
		var b strings.Builder
		fmt.Fprintf(&b, "; GENERATED by gauntlet/stdlib/js.go — do not edit.\n")
		fmt.Fprintf(&b, "; The nameable surface of %s.\n", r)
		fmt.Fprintf(&b, ";\n; EVERY TYPE IS `any`, which is not this generator being lazy: it is what\n")
		fmt.Fprintf(&b, "; `targets/js` declares and what the host knows. The consequence is that\n")
		fmt.Fprintf(&b, "; nothing in this file can be checked, by us or by the host.\n")
		b.WriteString("(target js\n")
		var mn []string
		for k := range mods {
			mn = append(mn, k)
		}
		sort.Strings(mn)
		for _, k := range mn {
			fmt.Fprintf(&b, "  (module %s\n%s  )\n", k, mods[k].String())
		}
		b.WriteString(")\n")
		name := strings.NewReplacer(":", "-", "/", "-").Replace(r)
		if err := os.WriteFile(filepath.Join(dir, name+".oro"), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("\nemitted %d primitives across %d roots into %s\n", n, len(roots), dir)
	fmt.Printf("  %d renamed with a numeral, because `tg.Prims` is keyed by name alone\n", overloaded)
	fmt.Printf("  and one object's member can shadow another's in the same module.\n")
	return nil
}
