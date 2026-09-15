//go:build ignore

// jvm.go — how much of the JVM's exported API this language can declare, and why
// not the rest. One of the two targets that had never been priced.
//
//	java gauntlet/stdlib/jdk/Dump.java > /tmp/jdk-api.txt
//	go run gauntlet/stdlib/jvm.go -api /tmp/jdk-api.txt [-emit DIR]
//
// THE RUNTIME IS THE MANIFEST. Go ships `api/go1*.txt` and the Windows SDK ships
// headers; the JDK ships neither and needs neither, because `jrt:/` plus
// reflection IS the JDK. So unlike the other two surveys this one cannot be
// wrong about what EXISTS — only about what we can do with it.
//
// WHAT MAKES THIS HOST DIFFERENT, stated before any number so it can be checked
// rather than discovered:
//
//	OUR `int` IS JAVA'S `long`, so Java's `int` is a RANGE — `(int -2147483648
//	2147483647)` — and ADR 0003's ladder renders it back. That is not a
//	concession: targets/java/java.oro says declaring our `int` as Java's would be
//	a silent miscompilation, because Java's wraps at 2^31, inside the range our
//	literals already cover.
//
//	A BOXED TYPE IS ITS PRIMITIVE. `java.lang.Long` where a `long` is wanted and
//	back is the HOST's own coercion (JLS 5.1.7, 5.1.8), inserted by javac exactly
//	as Go inserts an interface coercion — zero emitted characters. The hazard is
//	named rather than hidden: unboxing a null throws, and a Java programmer has
//	that hazard too.
//
//	A TYPE VARIABLE IS NOT A TYPE. `E`, `T`, `? extends E` — there is nothing to
//	name. This is the JVM's defining refusal and it is not a version of Go's:
//	Go's standard library is barely generic and Java's is generic to the bone.
//
//	AND A PARAMETERISED TYPE IS SPELLABLE AT A GROUND INSTANTIATION.
//	`java.util.List<java.lang.String>` gets a name the way `*os.File` does — we
//	never APPLY a type constructor, we NAME the applied thing. That is a
//	correction the Go survey owes itself, where `iter.Seq[[]uint8]` is refused as
//	a `generic instantiation` on an argument that does not survive here.
//
//	A FUNCTIONAL INTERFACE IS AN ORDINARY OBJECT. On Go a `func(…)` value is
//	refused in both directions; on the JVM a callback is an object with one
//	method, so HOLDING one costs nothing and CALLING one is an ordinary method
//	call. Only MANUFACTURING one is callbacks.md tier 3 — and the obtainable
//	fixed point decides that already, with no special case.
//
// THE DISCIPLINE, because four of this repository's survey numbers turned out to
// be the tool talking about itself: a name counted declarable and never EMITTED
// is a claim, so `-emit` writes every one of them and the two counts must agree;
// and a measurement of one's own language must never round in its own favour.
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

// ------------------------------------------------------------------ the reasons

type reason string

const (
	ok          reason = "ok"
	rTypeVar    reason = "type variable"
	rObject     reason = "java.lang.Object"
	rVariadic   reason = "variadic"
	rArrayElem  reason = "array element"
	rAbstract   reason = "no `new` on abstract"
	rInner      reason = "inner class"
	rGenericFn  reason = "generic method"
	rUnexported reason = "type not exported"
	uArgOpaque  reason = "cannot build the argument"
	uResOpaque  reason = "cannot read the result"
)

// blame says WHERE a refusal lives, which is the whole point: the three answers
// cost completely different amounts.
func blame(r reason) string {
	switch r {
	case rTypeVar, rGenericFn:
		return "language: our type language has no type variables (type-algebra.md)"
	case rObject:
		return "language: a value with no operations — Go's interface{} again"
	case rVariadic:
		return "format: one declaration per arity, as Println/Println2/Println3 are"
	case rArrayElem:
		return "language: a table whose element we cannot name"
	case rAbstract:
		return "host: `new` on an abstract class or an interface is not legal Java"
	case rInner:
		return "format: a non-static inner class needs an enclosing instance"
	case rUnexported:
		return "host: the signature names a type no module exports"
	case uArgOpaque, uResOpaque:
		return "reachability: nothing declarable produces the type"
	}
	return ""
}

// ------------------------------------------------------------------- the types

type verdict struct {
	why    reason
	opaque bool
}

// scalar maps a Java type to what the target format calls it.
//
// OUR `int` IS JAVA'S `long`. Everything narrower is a RANGE and ADR 0003's
// ladder picks the storage back. A BOXED TYPE IS ITS PRIMITIVE, because javac
// inserts the conversion in both directions.
var scalar = map[string]string{
	"long": "int", "java.lang.Long": "int",
	"int": "(int -2147483648 2147483647)", "java.lang.Integer": "(int -2147483648 2147483647)",
	"short": "(int -32768 32767)", "java.lang.Short": "(int -32768 32767)",
	"byte": "(int -128 127)", "java.lang.Byte": "(int -128 127)",
	"char": "(int 0 65535)", "java.lang.Character": "(int 0 65535)",
	"boolean": "bool", "java.lang.Boolean": "bool",
	"double": "f64", "java.lang.Double": "f64",
	"float": "f64", "java.lang.Float": "f64",
	"java.lang.String": "string",
}

var (
	genericCls = map[string]bool{} // has type parameters: used RAW
	abstract   = map[string]bool{} // abstract class or interface: no `new`
	funcIface  = map[string]bool{} // one abstract method
	inner      = map[string]bool{} // needs an enclosing instance
	known      = map[string]bool{} // every type some module EXPORTS
	supplied   = map[string]bool{}
)

func classify(t string) verdict {
	t = strings.TrimSpace(t)
	switch {
	case t == "":
		return verdict{ok, false}
	case strings.HasPrefix(t, "..."):
		return verdict{rVariadic, false}
	case t == "java.lang.Object":
		// A VALUE WITH NO OPERATIONS. Go's survey refuses `interface{}` for the
		// same reason, and the comparison is only meaningful if both do:
		// treating it as `any` would make the obtainable fixed point vacuous,
		// since a large part of the JDK returns one.
		return verdict{rObject, false}
	}
	if _, isScalar := scalar[t]; isScalar {
		return verdict{ok, false}
	}
	if strings.HasSuffix(t, "[]") {
		e := classify(t[:len(t)-2])
		if e.why != ok {
			return verdict{rArrayElem, false}
		}
		return verdict{ok, e.opaque}
	}
	if strings.HasPrefix(t, "?") {
		return verdict{rTypeVar, false}
	}
	if i := strings.IndexByte(t, '<'); i >= 0 {
		// A GROUND INSTANTIATION IS A NAME. We never apply a type constructor;
		// we name the applied thing, exactly as `*os.File` is named.
		head, args := t[:i], splitTop(t[i+1:len(t)-1])
		if !strings.Contains(head, ".") {
			return verdict{rTypeVar, false}
		}
		for _, a := range args {
			if v := classify(a); v.why != ok {
				return verdict{rTypeVar, false}
			}
		}
		return verdict{ok, true}
	}
	// EVERY REAL JDK TYPE IS PACKAGE-QUALIFIED, so a name with no dot is a type
	// VARIABLE — `E`, `T`, `K`, `V`.
	if !strings.Contains(t, ".") {
		return verdict{rTypeVar, false}
	}
	// AND A TYPE NO MODULE EXPORTS IS NOT SPELLABLE AT ALL, however public the
	// method that mentions it. `jdk.internal.*` and the `sun.*` packages are
	// public classes in packages exported to NOBODY, so a program cannot write
	// the name — and a survey that let them through would count a signature it
	// could not emit. Checked against the manifest's own type lines rather than
	// against a list of prefixes, because the module graph is the fact and a
	// prefix is a guess.
	if !known[t] {
		return verdict{rUnexported, false}
	}
	// A NAMED CLASS OR INTERFACE: an opaque token with methods, which is what a
	// Win32 `HANDLE` has been since that target existed.
	return verdict{ok, true}
}

// splitTop splits a comma-separated type-argument list at depth zero.
func splitTop(s string) []string {
	var out []string
	d, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			d++
		case '>':
			d--
		case ',':
			if d == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

// jspell names a Java type for the target file. It is verbose on purpose:
// `List` is declared in `java.util` AND in `java.awt`, and a survey that
// collapsed them would repeat gomethods-2026-09-09's bug, where `*File` in `os`
// and in `archive/zip` were one map entry and the number inflated.
func jspell(t string) string {
	if s, isScalar := scalar[t]; isScalar {
		return s
	}
	if strings.HasSuffix(t, "[]") {
		return "(array " + jspell(t[:len(t)-2]) + ")"
	}
	// `[]` FIRST, and inside a type argument it is not a table but part of a
	// NAME: `Map<Thread, StackTraceElement[]>` is one opaque token, and `[` is
	// not an identifier character. Found by the reader refusing a generated
	// file, which is the loud direction.
	r := strings.NewReplacer("[]", "-array", ".", "-", "<", "-of-", ">", "",
		", ", "-and-", ",", "-and-", " ", "", "$", "-")
	return r.Replace(t)
}

// rawOf strips the type arguments a generic class's members are written
// against. A GENERIC CLASS IS USED RAW, which is what the JVM erases it to, and
// every member mentioning a type variable is refused — so nothing survives that
// would have needed the argument.
func rawOf(t string) string {
	if i := strings.IndexByte(t, '<'); i >= 0 {
		return t[:i]
	}
	return t
}

// ------------------------------------------------------------------ the symbol

type sym struct {
	pkg, kind, name string
	owner           string // the declaring class, fully qualified
	params, results []string
	static          bool
	generic         bool
}

func parseLine(line string) (sym, bool) {
	if !strings.HasPrefix(line, "pkg ") {
		return sym{}, false
	}
	c := strings.Index(line, ", ")
	if c < 0 {
		return sym{}, false
	}
	s := sym{pkg: line[4:c]}
	rest := line[c+2:]
	fn := strings.HasSuffix(rest, " functional")
	if fn {
		rest = strings.TrimSuffix(rest, " functional")
	}
	if strings.HasSuffix(rest, " generic") {
		s.generic = true
		rest = strings.TrimSuffix(rest, " generic")
	}
	sp := strings.Index(rest, " ")
	if sp < 0 {
		return sym{}, false
	}
	s.kind, rest = rest[:sp], rest[sp+1:]

	switch s.kind {
	case "extends":
		i := strings.Index(rest, " ")
		if i < 0 {
			return sym{}, false
		}
		s.owner, s.name = rest[:i], rest[:i]
		s.params = strings.Fields(rest[i+1:])
		return s, true
	case "type":
		s.name = rest
		if i := strings.Index(rest, " "); i >= 0 {
			s.name, s.results = rest[:i], []string{rest[i+1:]}
		}
		s.owner = s.name
		if fn {
			// STRIPPED BEFORE THE SPLIT, so the attribute has to be carried
			// rather than re-read off the tail. The first version looked for it
			// in `s.results` AFTER trimming it away and reported zero functional
			// interfaces on a host that has 286 of them -- a tool talking about
			// itself, caught by the count being exactly 0.
			funcIface[s.name] = true
		}
		return s, true
	case "ctor":
		i := strings.IndexByte(rest, '(')
		if i < 0 || !strings.HasSuffix(rest, ")") {
			return sym{}, false
		}
		s.owner, s.name = rest[:i], rest[:i]
		s.params = argList(rest[i+1 : len(rest)-1])
		s.results = []string{s.owner}
		s.static = true
		return s, true
	case "func", "method":
		if s.kind == "method" {
			if !strings.HasPrefix(rest, "(") {
				return sym{}, false
			}
			e := strings.IndexByte(rest, ')')
			if e < 0 {
				return sym{}, false
			}
			s.owner, rest = rest[1:e], strings.TrimSpace(rest[e+1:])
		} else {
			s.static = true
		}
		i := strings.IndexByte(rest, '(')
		j := strings.LastIndex(rest, ")")
		if i < 0 || j < i {
			return sym{}, false
		}
		nm := rest[:i]
		if s.kind == "func" {
			d := strings.LastIndex(nm, ".")
			if d < 0 {
				return sym{}, false
			}
			s.owner, nm = nm[:d], nm[d+1:]
		}
		s.name = nm
		s.params = argList(rest[i+1 : j])
		if r := strings.TrimSpace(rest[j+1:]); r != "" && r != "void" {
			s.results = []string{r}
		}
		return s, true
	case "static", "field":
		i := strings.LastIndex(rest, " ")
		if i < 0 {
			return sym{}, false
		}
		nm, ty := rest[:i], rest[i+1:]
		d := strings.LastIndex(nm, ".")
		if d < 0 {
			return sym{}, false
		}
		s.owner, s.name = nm[:d], nm[d+1:]
		s.results = []string{ty}
		s.static = s.kind == "static"
		return s, true
	}
	return sym{}, false
}

func argList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return splitTop(s)
}

// ------------------------------------------------------------------ the verdict

// judge decides DECLARABLE and USABLE for one member.
func judge(s sym, have map[string]bool) (declarable bool, usable bool, why reason) {
	if s.generic {
		return false, false, rGenericFn
	}
	// `new` ON AN ABSTRACT CLASS OR AN INTERFACE IS NOT LEGAL JAVA, and
	// `getConstructors` lists them anyway. The host decides this one, the way
	// `go build` decided the subsumption edges.
	if s.kind == "ctor" {
		if abstract[s.owner] {
			return false, false, rAbstract
		}
		if inner[s.owner] {
			return false, false, rInner
		}
	}
	argOpaque, resOpaque := false, false
	if !s.static {
		r := rawOf(s.owner)
		v := classify(r)
		if v.why != ok {
			return false, false, v.why
		}
		argOpaque = argOpaque || (v.opaque && !have[r])
	}
	for _, p := range s.params {
		v := classify(p)
		if v.why != ok {
			return false, false, v.why
		}
		argOpaque = argOpaque || (v.opaque && !have[p] && !supplied[p])
	}
	for _, r := range s.results {
		v := classify(r)
		if v.why != ok {
			return false, false, v.why
		}
		// A RESULT WHOSE TYPE IS OBTAINABLE CAN BE READ — interfaces.md §5's
		// correction, applied here from the start rather than found later.
		// Having methods is the whole of what reading an opaque value means.
		resOpaque = resOpaque || (v.opaque && !have[r])
	}
	switch {
	case argOpaque:
		return true, false, uArgOpaque
	case resOpaque:
		return true, false, uResOpaque
	}
	return true, true, ok
}

// obtainableFrom is the least set of host types a program can GET HOLD OF: a
// type is obtainable when some declarable member produces it and everything that
// member needs is a scalar or already obtainable.
//
// TWO SOURCES, not one, and the second is what this host is made of. Go's survey
// only has to consider package functions, because Go's constructor idiom is
// `NewT`. Here `new` is the idiom (4,138 public constructors) AND a method on an
// obtainable receiver is a source too — `list.iterator()` is how a large part of
// the JDK is reached, and leaving it out would understate this host the way
// scoring methods at 0% understated Go.
func obtainableFrom(syms []sym, seed map[string]bool) map[string]bool {
	have := map[string]bool{}
	for k := range seed {
		have[k] = true
	}
	reachable := func(s sym) bool {
		for _, p := range s.params {
			pv := classify(p)
			if pv.why != ok || (pv.opaque && !have[p]) {
				return false
			}
		}
		return true
	}
	produce := func(s sym) bool {
		grew := false
		for _, r := range s.results {
			v := classify(r)
			if v.why != ok || !v.opaque || have[r] {
				continue
			}
			have[r] = true
			grew = true
		}
		return grew
	}
	for {
		grew := false
		for _, s := range syms {
			if s.generic || len(s.results) == 0 {
				continue
			}
			switch s.kind {
			case "ctor":
				if abstract[s.owner] || inner[s.owner] || !reachable(s) {
					continue
				}
			case "func", "static":
				if !reachable(s) {
					continue
				}
			case "method", "field":
				if !have[rawOf(s.owner)] || !reachable(s) {
					continue
				}
			default:
				continue
			}
			if produce(s) {
				grew = true
			}
		}
		if !grew {
			return have
		}
	}
}

// sub maps a type to the supertypes it may stand in for.
var sub = map[string][]string{}

// closeSub takes the transitive closure, deduped and sorted, so the emitted
// edges are a function of the input and a program never depends on a
// consequence nobody wrote down.
func closeSub() {
	var walk func(t string, seen map[string]bool) []string
	walk = func(t string, seen map[string]bool) []string {
		if seen[t] {
			return nil
		}
		seen[t] = true
		var out []string
		for _, up := range sub[t] {
			out = append(out, up)
			out = append(out, walk(up, seen)...)
		}
		return out
	}
	closed := map[string][]string{}
	for t := range sub {
		u := map[string]bool{}
		for _, x := range walk(t, map[string]bool{}) {
			if x != t {
				u[x] = true
			}
		}
		var l []string
		for x := range u {
			l = append(l, x)
		}
		sort.Strings(l)
		closed[t] = l
	}
	sub = closed
}

// byCount orders by count, largest first, and then by NAME. A count alone is a
// partial order, ties are common, every ranked list here is filled from a map,
// and sort.Slice is not stable — so the report changed between two identical
// runs, and nothing noticed until a test ran the tool twice (tooling-2026-09-11).
func byCount[K ~string](a, b int, ka, kb K) bool {
	if a != b {
		return a > b
	}
	return ka < kb
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

// ---------------------------------------------------------------------- main

func main() {
	api := flag.String("api", "", "the manifest written by gauntlet/stdlib/jdk/Dump.java")
	emitDir := flag.String("emit", "", "write the declarable subset as target files into DIR")
	flag.Parse()
	if *api == "" {
		fmt.Fprintln(os.Stderr, "usage: java gauntlet/stdlib/jdk/Dump.java > /tmp/jdk-api.txt")
		fmt.Fprintln(os.Stderr, "       go run gauntlet/stdlib/jvm.go -api /tmp/jdk-api.txt [-emit DIR]")
		os.Exit(2)
	}
	fh, err := os.Open(*api)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer fh.Close()

	var syms []sym
	var typesN int
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var raw []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		raw = append(raw, line)
	}
	// THE TYPE LINES FIRST, because every later judgement reads them: whether a
	// class is abstract decides whether its constructors exist, and whether it
	// is generic decides whether it is used raw.
	for _, line := range raw {
		s, okp := parseLine(line)
		if !okp || s.kind != "type" {
			continue
		}
		typesN++
		attrs := ""
		if len(s.results) > 0 {
			attrs = s.results[0]
		}
		if strings.HasPrefix(attrs, "interface") || strings.HasPrefix(attrs, "abstract") {
			abstract[s.name] = true
		}
		known[s.name] = true
		if s.generic {
			genericCls[s.name] = true
		}
	}
	// THE SUBTYPING RELATION, READ RATHER THAN DERIVED. On Go this had to be
	// computed structurally and then host-filtered, because 182 of 1,651
	// candidates were false; here `extends` and `implements` are written down,
	// so there is nothing to guess and nothing for a filter to catch.
	//
	// Closed TRANSITIVELY, the way `closeImplements` does at load: a target file
	// stating a closure by hand would get it wrong.
	for _, line := range raw {
		s, okp := parseLine(line)
		if !okp || s.kind != "extends" {
			continue
		}
		sub[s.owner] = append(sub[s.owner], s.params...)
	}
	closeSub()

	// A NON-STATIC INNER CLASS is not something the manifest says directly, and
	// guessing from the name would be the tool inventing a fact — so it is left
	// to the host: `-emit` writes the declaration and javac refuses it. Nothing
	// is marked here, and the count is honestly zero rather than approximate.
	for _, line := range raw {
		s, okp := parseLine(line)
		if !okp || s.kind == "type" || s.kind == "extends" {
			continue
		}
		syms = append(syms, s)
	}

	have := obtainableFrom(syms, nil)
	// AN INTERFACE OR SUPERCLASS WE CAN SUPPLY IS NOT A BLOCKER: the host
	// inserts the widening, so `⟦coerce⟧ = id` and the emitted text is
	// unchanged — coercion-2026-09-09's finding arriving on a host that
	// declares its own relation. Only an OBTAINABLE subtype counts.
	//
	// `string` is Java's `String`, so a program holding one can call anything
	// that takes a `CharSequence`, which is how half of `java.util` is written.
	for t := range have {
		for _, up := range sub[t] {
			supplied[up] = true
		}
	}
	for _, up := range sub["java.lang.String"] {
		supplied[up] = true
	}

	type tally struct{ decl, usable, total int }
	byKind := map[string]*tally{"func": {}, "method": {}, "ctor": {}}
	byReason := map[reason]int{}
	byGap := map[reason]int{}
	byPkg := map[string]*tally{}
	all := tally{}
	var declarables []sym

	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" && s.kind != "ctor" {
			continue
		}
		t := byPkg[s.pkg]
		if t == nil {
			t = &tally{}
			byPkg[s.pkg] = t
		}
		t.total++
		all.total++
		k := byKind[s.kind]
		k.total++
		d, u, why := judge(s, have)
		if d {
			t.decl++
			all.decl++
			k.decl++
			declarables = append(declarables, s)
			if !u {
				byGap[why]++
			}
		} else {
			byReason[why]++
		}
		if u {
			t.usable++
			all.usable++
			k.usable++
		}
	}

	fmt.Printf("THE JVM, %d exported packages, %d public types, %d members\n", len(byPkg), typesN, len(syms))
	fmt.Printf("  (%d generic classes, used RAW; %d abstract or interface; %d functional interfaces)\n\n",
		len(genericCls), len(abstract), len(funcIface))
	fmt.Printf("CALLABLE SURFACE (func + method + ctor): %d\n", all.total)
	fmt.Printf("  declarable as a (prim …):  %6d  %5.1f%%\n", all.decl, pct(all.decl, all.total))
	fmt.Printf("  usable by a program:       %6d  %5.1f%%\n", all.usable, pct(all.usable, all.total))
	for _, k := range []string{"func", "method", "ctor"} {
		t := byKind[k]
		fmt.Printf("    %-7s %6d total  %6d declarable (%4.1f%%)  %6d usable (%4.1f%%)\n",
			k, t.total, t.decl, pct(t.decl, t.total), t.usable, pct(t.usable, t.total))
	}
	fmt.Println()

	fmt.Println("WHY THE REST CANNOT BE DECLARED")
	type rc struct {
		r reason
		n int
	}
	var rs []rc
	for r, n := range byReason {
		rs = append(rs, rc{r, n})
	}
	sort.Slice(rs, func(i, j int) bool { return byCount(rs[i].n, rs[j].n, rs[i].r, rs[j].r) })
	for _, x := range rs {
		fmt.Printf("  %-26s %6d  %5.1f%%   [%s]\n", x.r, x.n, pct(x.n, all.total), blame(x.r))
	}

	fmt.Printf("\nHOST TYPES A PROGRAM CAN OBTAIN: %d (least fixed point over `new` AND methods)\n", len(have))
	fmt.Println("\nAND WHY A DECLARABLE NAME IS STILL NOT USABLE")
	var gs []rc
	for r, n := range byGap {
		gs = append(gs, rc{r, n})
	}
	sort.Slice(gs, func(i, j int) bool { return byCount(gs[i].n, gs[j].n, gs[i].r, gs[j].r) })
	for _, x := range gs {
		fmt.Printf("  %-26s %6d  %5.1f%% of declarable\n", x.r, x.n, pct(x.n, all.decl))
	}

	fmt.Println("\nBY PACKAGE (the twenty a program is most likely to want)")
	want := []string{"java.lang", "java.util", "java.io", "java.nio.file", "java.math",
		"java.text", "java.time", "java.util.regex", "java.net", "java.security",
		"java.util.stream", "java.util.function", "java.util.concurrent", "javax.crypto",
		"java.sql", "java.awt", "javax.swing", "java.lang.reflect", "java.nio.charset", "java.util.zip"}
	fmt.Printf("  %-24s %7s %7s %7s\n", "package", "total", "decl", "usable")
	for _, p := range want {
		if t := byPkg[p]; t != nil {
			fmt.Printf("  %-24s %7d %7d %7d\n", p, t.total, t.decl, t.usable)
		} else {
			fmt.Printf("  %-24s %7s\n", p, "-")
		}
	}

	genericReport(syms, all.total)

	if *emitDir != "" {
		if err := emit(*emitDir, declarables); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
	}
}

// genericReport prices the JVM's defining refusal, because a number that large
// deserves to be decomposed rather than quoted.
//
// maxlen-2026-08-28's discipline: classify every blocked name by the fact that
// would settle it BEFORE proposing to build anything. There it killed octagons.
func genericReport(syms []sym, total int) {
	var tvOnly, tvOwner, tvSig int
	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" && s.kind != "ctor" {
			continue
		}
		if s.generic {
			continue
		}
		hit := false
		mentions := append(append([]string{}, s.params...), s.results...)
		if !s.static {
			mentions = append(mentions, s.owner)
		}
		for _, t := range mentions {
			if classify(t).why == rTypeVar {
				hit = true
			}
		}
		if !hit {
			continue
		}
		tvOnly++
		if genericCls[rawOf(s.owner)] {
			tvOwner++
		} else {
			tvSig++
		}
	}
	fmt.Printf("\nTHE TYPE VARIABLE, DECOMPOSED: %d names mention one (%.1f%% of the surface)\n",
		tvOnly, pct(tvOnly, total))
	fmt.Printf("  %d are a member of a GENERIC CLASS -- `ArrayList.add(E)` -- where the\n", tvOwner)
	fmt.Printf("  variable comes from the receiver, so instantiating the class at a ground\n")
	fmt.Printf("  type would substitute it. That is one declaration per (class, argument),\n")
	fmt.Printf("  which targets/java/util.oro calls \"the same limitation squared\" and is\n")
	fmt.Printf("  why it declares Map<String,Long> and nothing else.\n")
	fmt.Printf("  %d are a variable in the member's OWN signature with a non-generic owner,\n", tvSig)
	fmt.Printf("  and nothing can substitute those: they are the honest refusal.\n")
}

// ----------------------------------------------------------------------- emit
//
// A NAME COUNTED DECLARABLE AND NEVER EMITTED IS A CLAIM — win32-2026-09-08's
// sentence, which gomethods-2026-09-09 then had to apply to the Go survey when
// it turned out to be writing a third of what it counted. So this writes every
// declarable member and prints the count, and the two must agree.
//
// NOTHING IS IMPORTED. Java resolves a fully-qualified name everywhere, so a
// template can spell `java.util.Objects.toString(%s)` and needs no `(import …)`
// at all — which also means two classes with the same simple name can never
// collide in a generated file.
var overloaded, skipped int

// edgePairs counts what the report calls an EDGE: one (subtype, supertype) pair,
// however many package files repeat it. The first version counted `implements`
// LINES, one per subject per file, and printed that as edges — 4,626 against
// 10,095 pairs — while the published figure, 5,186, matched neither and was never
// reproduced (tooling-2026-09-11).
var edgePairs = map[string]bool{}
var edgeSubjects = map[string]bool{}

func emit(dir string, syms []sym) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	byPkg := map[string][]sym{}
	for _, s := range syms {
		byPkg[s.pkg] = append(byPkg[s.pkg], s)
	}
	var pkgs []string
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	n := 0
	for _, p := range pkgs {
		list := byPkg[p]
		sort.Slice(list, func(i, j int) bool {
			if list[i].owner != list[j].owner {
				return list[i].owner < list[j].owner
			}
			if list[i].kind != list[j].kind {
				return list[i].kind < list[j].kind
			}
			if list[i].name != list[j].name {
				return list[i].name < list[j].name
			}
			return strings.Join(list[i].params, ",") < strings.Join(list[j].params, ",")
		})
		types := map[string]string{}
		spellQ := func(t string) string {
			if _, isScalar := scalar[t]; isScalar {
				return jspell(t)
			}
			if strings.HasSuffix(t, "[]") {
				e := t[:len(t)-2]
				if _, isScalar := scalar[e]; !isScalar {
					types[jspell(e)] = e
				}
				return "(array " + jspell(e) + ")"
			}
			types[jspell(t)] = t
			return jspell(t)
		}
		// OVERLOADING IS THE FORMAT PROBLEM THIS HOST HAS AND THE OTHER TWO DO
		// NOT. `tg.Prims` is keyed by NAME ALONE, so `nextInt()` and
		// `nextInt(int)` are one entry — overloading.md §3 already records this
		// as "a target-file wart: Println, Println2, Println3 exist because
		// tg.Prims is keyed by name alone", with ONE live instance. Java has
		// tens of thousands, so the wart is a first-class refusal here.
		//
		// The existing precedent is the answer: the first spelling keeps the
		// name and each later one gets a numeral. It is a function of the input
		// because `list` was sorted by (owner, kind, name, parameter list)
		// first — an unsorted map would make the SAME declaration land on a
		// different overload between runs, which is backend-2026-09-06 with a
		// wrong answer instead of a different file.
		taken := map[string]int{}
		mods := map[string]*strings.Builder{}
		modBuf := func(name string) *strings.Builder {
			if b, okb := mods[name]; okb {
				return b
			}
			b := &strings.Builder{}
			mods[name] = b
			return b
		}
		pkgMod := "java/" + strings.ReplaceAll(p, ".", "-")
		for _, s := range list {
			simple := s.owner
			if i := strings.LastIndex(simple, "."); i >= 0 {
				simple = simple[i+1:]
			}
			// A CONSTRUCTOR IS NAMED FOR ITS CLASS, not for its fully qualified
			// spelling: a dot in a prim name would be read as a module
			// qualification and resolve to nothing.
			pname := s.name
			if s.kind == "ctor" {
				pname = simple
			}
			var args, holes []string
			// EVERY MEMBER LIVES IN ITS CLASS'S MODULE, STATIC OR NOT — which
			// is what targets/java/lang.oro already does by hand: `(use
			// java/Math)` then `(Math.abs x)`, reading exactly as Java reads.
			//
			// The package module was the first attempt and it collides:
			// `java.lang.Math.atan2` and `java.lang.StrictMath.atan2` are two
			// classes, one package and one name. A CLASS is the unit here
			// because a class is what Java qualifies a static call by.
			//
			// A CONSTRUCTOR STAYS IN THE PACKAGE MODULE, under the class's own
			// name, so `(ju.StringJoiner ", ")` reads as the type being built
			// rather than as `StringJoiner.StringJoiner`.
			mod := pkgMod + "/" + simple
			if s.kind == "ctor" {
				mod = pkgMod
			}
			if !s.static {
				// A METHOD'S RECEIVER IS ARGUMENT 0 — the convention Win32's
				// `HANDLE` used and gomethods-2026-09-09 generalised. A generic
				// class is named RAW here, which is what the JVM erases it to.
				args = append(args, "(self "+spellQ(rawOf(s.owner))+")")
				holes = append(holes, "%s")
			}
			for k, a := range s.params {
				args = append(args, fmt.Sprintf("(a%d %s)", k, spellQ(a)))
				holes = append(holes, "%s")
			}
			argList := "(none)"
			if len(args) > 0 {
				argList = "(" + strings.Join(args, " ") + ")"
			}
			callArgs := holes
			if !s.static {
				callArgs = holes[1:]
			}
			var call string
			switch s.kind {
			case "ctor":
				// PARENTHESISED, for the reason a Go composite literal is:
				// `new X().m()` is legal but `%s` may land anywhere, and one
				// pair of parentheses makes the template a VALUE wherever it is
				// spliced.
				call = "(new " + s.owner + "(" + strings.Join(callArgs, ", ") + "))"
			case "static":
				call = s.owner + "." + s.name
			case "field":
				call = holes[0] + "." + s.name
			case "func":
				call = s.owner + "." + s.name + "(" + strings.Join(callArgs, ", ") + ")"
			default:
				call = holes[0] + "." + s.name + "(" + strings.Join(callArgs, ", ") + ")"
			}
			kind, res := "expr", ""
			switch {
			case len(s.results) == 0:
				// A statement's value IS its first argument, which is what
				// `stmt` means. A void with no argument has nothing to be, and
				// that is the honest refusal.
				if len(args) == 0 {
					skipped++
					continue
				}
				kind = "stmt"
				res = strings.TrimSuffix(strings.TrimPrefix(args[0],
					"("+strings.Fields(args[0][1:])[0]+" "), ")")
			default:
				res = spellQ(s.results[0])
			}
			key := mod + "." + pname
			taken[key]++
			if k := taken[key]; k > 1 {
				pname = fmt.Sprintf("%s%d", pname, k)
				overloaded++
			}
			fmt.Fprintf(modBuf(mod), "    (sig %s %s %s (host %s %q))\n",
				pname, strings.Replace(argList, "(none)", "()", 1), res, kind, call)
			n++
		}
		// THE EDGES FOR THE TYPES THIS FILE DECLARES, put with the SUBTYPE,
		// because that is where the file already spells it — and the supertype
		// is spelled here too, so a program loading one file is never left
		// naming a type the target does not have.
		//
		// `(implements string java-lang-CharSequence …)` is the one that pays:
		// our `string` IS Java's `String`, and `java.util` is written against
		// `CharSequence` throughout.
		edges := map[string][]string{}
		// A SCALAR HAS SUPERTYPES TOO, and `string` ≤ `CharSequence` is the one
		// edge that pays for the rest: our `string` IS Java's `String`, and
		// `java.util` is written against `CharSequence` throughout. A scalar is
		// never in `types`, because `spellQ` returns its spelling without
		// recording it — so its edges have to be sought rather than found.
		//
		// The edge is emitted and the `(type …)` is NOT: `string`, `int` and the
		// rest are the target's own names, declared in targets/java/java.oro,
		// and re-declaring one from a generated layer would be a collision
		// rather than a contribution.
		mine := map[string]string{}
		for t := range types {
			mine[t] = types[t]
		}
		for t := range sub {
			if _, isScalar := scalar[t]; !isScalar {
				continue
			}
			if i := strings.LastIndex(t, "."); i < 0 || t[:i] != p {
				continue
			}
			mine[jspell(t)] = t
		}
		for k, q := range mine {
			// A RANGE IS NOT A NAME, so it cannot carry an edge. `java.lang.Byte`
			// spells `(int -128 127)`, which is a type EXPRESSION — and nothing
			// is lost, because `ValueType` normalises a range to `int` and `int`
			// carries the edges. `string` and `int` are names; the other boxes
			// are ranges and go through `int`.
			if strings.ContainsAny(k, "() ") {
				continue
			}
			ups := sub[rawOf(q)]
			if len(ups) == 0 {
				continue
			}
			for _, up := range ups {
				if classify(up).why != ok {
					continue
				}
				edges[k] = append(edges[k], jspell(up))
				if _, isScalar := scalar[up]; !isScalar {
					types[jspell(up)] = up
				}
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "; GENERATED by gauntlet/stdlib/jvm.go — do not edit.\n")
		fmt.Fprintf(&b, "; The declarable subset of the JVM's %s.\n", p)
		b.WriteString("(target java\n")
		var tn []string
		for k := range types {
			tn = append(tn, k)
		}
		sort.Strings(tn)
		for _, k := range tn {
			fmt.Fprintf(&b, "  (type %s (host %q))\n", k, jsrc(types[k]))
		}
		if len(tn) > 0 {
			b.WriteString("\n")
		}
		var en []string
		for k := range edges {
			en = append(en, k)
		}
		sort.Strings(en)
		for _, k := range en {
			fmt.Fprintf(&b, "  (implements %s %s)\n", k, strings.Join(edges[k], " "))
			edgeSubjects[k] = true
			for _, up := range edges[k] {
				edgePairs[k+" "+up] = true
			}
		}
		if len(en) > 0 {
			b.WriteString("\n")
		}
		var mn []string
		for k := range mods {
			mn = append(mn, k)
		}
		sort.Strings(mn)
		for _, k := range mn {
			fmt.Fprintf(&b, "  (module %s\n%s  )\n", k, mods[k].String())
		}
		b.WriteString(")\n")
		out := filepath.Join(dir, strings.ReplaceAll(p, ".", "-")+".oro")
		if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("\nemitted %d primitives across %d packages into %s\n", n, len(pkgs), dir)
	fmt.Printf("  %d of them are OVERLOADS renamed with a numeral, which is\n", overloaded)
	fmt.Printf("  overloading.md's Println/Println2/Println3 wart at scale: `tg.Prims`\n")
	fmt.Printf("  is keyed by name alone, and Java's surface is overloaded throughout.\n")
	fmt.Printf("  %d declarable names have no template: a void with no argument, which\n", skipped)
	fmt.Printf("  has nothing to be, since a statement's value IS its first argument.\n")
	fmt.Printf("  %d subsumption edges over %d subtypes, READ from `extends`/`implements`\n",
		len(edgePairs), len(edgeSubjects))
	fmt.Printf("  rather than derived: this host declares its own relation, so unlike Go\n")
	fmt.Printf("  there is nothing to guess and nothing for a host filter to catch. They\n")
	fmt.Printf("  are the CLOSURE, which the loader would compute anyway, so emitting it\n")
	fmt.Printf("  changes no program.\n")
	return nil
}

// jsrc is the type AS JAVA SOURCE SPELLS IT. A generic class is written RAW,
// because every member mentioning its type variable is refused, so nothing
// survives that would need the argument — and javac's unchecked warning is about
// exactly those members.
func jsrc(t string) string { return rawOf(t) }
