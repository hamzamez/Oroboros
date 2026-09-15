//go:build ignore

// HOW MUCH OF THE GO STANDARD LIBRARY CAN THIS LANGUAGE DECLARE?
//
// The parasite thesis (ADR 0001) says a target is an ECOSYSTEM and that reaching
// it costs a line of data per name. This measures that claim against the largest
// ecosystem we target, symbol by symbol, instead of asserting it.
//
// The universe is `$GOROOT/api/go1*.txt` — the manifest the Go team maintains as
// the definition of the compatibility promise, so it is the API's own account of
// itself rather than ours. Platform-qualified lines (`pkg syscall (windows-386)`)
// are counted separately, because they are a different question: they are one
// platform's surface, not the portable library.
//
// TWO LEVELS, and keeping them apart is the whole point.
//
//	DECLARABLE  a `(prim …)` line can be written for it: every argument and
//	            result type has a spelling, the arity is fixed, and there is
//	            exactly one result. This is a question about the target FORMAT.
//
//	USABLE      a program can then do something with it: construct what it
//	            takes, and take apart what it returns. This is a question about
//	            the LANGUAGE.
//
// A name can be declarable and unusable — `os.Open` returns `(*File, error)`,
// and a program that cannot inspect either has been handed a token it can only
// pass back. Reporting one number would hide exactly that.
//
//	go run gauntlet/stdlib/survey.go            # the table
//	go run gauntlet/stdlib/survey.go -emit DIR  # generate the declarable subset
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- the reasons
//
// Each is a property of the LANGUAGE or of the target FORMAT, named so that the
// count says what would have to change. Ordered by where the blame sits.

type reason string

const (
	ok reason = ""

	// The target FORMAT cannot say it.
	rMultiResult reason = "several results"       // (T, error) — Prim.Result is one string
	rVariadic    reason = "variadic"              // ...T — arity is fixed
	rGeneric     reason = "type parameters"       // [T any] — the residual is monomorphic
	rGenericInst reason = "generic instantiation" // iter.Seq[T] — no type application
	rNoResultTyp reason = "unspellable result"    // the result type has no spelling
	rNoArgTyp    reason = "unspellable argument"  // an argument type has no spelling

	// The LANGUAGE cannot say it.
	rStruct    reason = "struct value"      // no product with named fields
	rInterface reason = "interface value"   // no dynamic dispatch
	rFunc      reason = "function argument" // callbacks.md tiers 2 and 3
	rChan      reason = "channel"           // no concurrency
	rUnsafe    reason = "unsafe / uintptr"
	rComplex   reason = "complex number"
	rMapKey    reason = "map key is not int" // maps.md: `=` is integer equality
	rElem      reason = "table of opaque"    // (array V) needs a V we can name
)

// blame says which side a reason falls on, for the summary.
func blame(r reason) string {
	switch r {
	case rMultiResult, rVariadic, rGeneric, rGenericInst, rNoResultTyp, rNoArgTyp:
		return "format"
	case ok:
		return "-"
	}
	return "language"
}

// ------------------------------------------------------------ type classifying
//
// A type is OPAQUE when the target can give it a spelling and a program can do
// nothing with it but pass it along — `*os.File`, `io.Writer`. That is a real
// and useful category: the parasite model is built on handing host values back
// to the host. It is recorded so that "declarable" is never mistaken for
// "usable".

type verdict struct {
	why    reason
	opaque bool // declarable, but the program cannot look inside
}

// A GO INTEGER TYPE IS A RANGE, and collapsing twelve of them into `int` is
// what gostdlib-2026-09-06 recorded as a GENERATOR limit rather than a format
// one: *"the format can already say the right thing"*. It says it here.
//
// The difference is not cosmetic. `[]uint8` as `(array int)` is `[]int` on Go
// and `os.File.Read` takes `[]byte`, so every generated line touching a byte
// slice named a type the host does not accept; declared `(array (int 0 255))`
// it emits `[]byte` on Go and `short[]` on the JVM, which is ADR 0003's ladder
// doing the work (elemwidth-2026-08-27).
//
// `int64` and `uint64` stay `int`, and that is ADR 0012 rather than laziness: a
// range past the portable window makes its own operations unprovable, which is
// a compile error at the call site — the honest place for it — where a declared
// `(int 0 (pow 2 64))` would promote the value to arbitrary precision and
// silently stop being the host's word.
var scalar = map[string]string{
	"bool": "bool", "string": "string",
	"int": "int", "int64": "int", "uint": "int", "uint64": "int",
	"int8":  "(int -128 127)",
	"int16": "(int -32768 32767)",
	"int32": "(int -2147483648 2147483647)",
	"uint8": "(int 0 255)", "byte": "(int 0 255)",
	"uint16":  "(int 0 65535)",
	"uint32":  "(int 0 4294967295)",
	"rune":    "(int -2147483648 2147483647)",
	"float64": "f64", "float32": "f64",
	"error": "error",
}

func classify(t string) verdict {
	t = strings.TrimSpace(t)
	switch {
	case t == "":
		return verdict{ok, false}
	case strings.HasPrefix(t, "..."):
		return verdict{rVariadic, false}
	case strings.HasPrefix(t, "$"):
		return verdict{rGeneric, false}
	case strings.HasPrefix(t, "chan ") || strings.HasPrefix(t, "<-chan ") ||
		strings.HasPrefix(t, "chan<- ") || t == "chan":
		return verdict{rChan, false}
	case strings.HasPrefix(t, "func("):
		return verdict{rFunc, false}
	case strings.HasPrefix(t, "struct{"):
		return verdict{rStruct, false}
	case strings.HasPrefix(t, "interface{") || t == "any":
		return verdict{rInterface, false}
	case t == "unsafe.Pointer" || t == "uintptr":
		return verdict{rUnsafe, false}
	case t == "complex64" || t == "complex128":
		return verdict{rComplex, false}
	case t == "error":
		// An `error` is an interface, so a program can hold one and not ask it
		// anything. Declarable as opaque; sums.md is what would make it usable.
		return verdict{ok, true}
	}
	if _, isScalar := scalar[t]; isScalar {
		return verdict{ok, false}
	}
	// A slice is our table, and only if the element is a type we can name.
	if strings.HasPrefix(t, "[]") {
		e := classify(t[2:])
		if e.why != ok {
			return verdict{rElem, false}
		}
		if e.opaque {
			// `[]io.Writer` — a table whose element the program cannot use. The
			// table itself is fine; the element is the problem.
			return verdict{ok, true}
		}
		return verdict{ok, false}
	}
	// A fixed-size array is not a slice and has no length in our sense.
	if strings.HasPrefix(t, "[") {
		return verdict{rElem, false}
	}
	if strings.HasPrefix(t, "map[") {
		d, i := 1, 4
		for ; i < len(t) && d > 0; i++ {
			switch t[i] {
			case '[':
				d++
			case ']':
				d--
			}
		}
		k, v := t[4:i-1], t[i:]
		if kv := classify(k); kv.why != ok || scalar[k] != "int" {
			// maps.md: `(map K V)` is well-formed exactly where `=` is defined,
			// and `=` is integer equality. A string key is the common Go case
			// and is exactly what the language refuses.
			return verdict{rMapKey, false}
		}
		if vv := classify(v); vv.why != ok {
			return verdict{rElem, false}
		}
		return verdict{ok, false}
	}
	if strings.HasPrefix(t, "*") {
		inner := classify(t[1:])
		if inner.why == rGeneric || inner.why == rStruct {
			return inner
		}
		return verdict{ok, true} // a pointer is a token we hand back
	}
	// A GENERIC INSTANTIATION — `iter.Seq[[]uint8]`, `atomic.Pointer[T]`. It is
	// a named type in Go and has no name here: our type language has one
	// constructor per concept (`(array V)`, `(map K V)`) and no way to APPLY a
	// host type constructor. It reaches this survey as the result of an
	// ordinary non-generic function, so it is a separate count from a function
	// with type parameters.
	if strings.ContainsAny(t, "[],{} ") {
		return verdict{rGenericInst, false}
	}
	// Anything left is a NAMED type: `time.Duration`, `Reader`, `os.FileMode`.
	// The target can spell it. Whether the program can do anything with it
	// depends on what it is, which the manifest tells us elsewhere — so it is
	// opaque here and the type table below sharpens it.
	return verdict{ok, true}
}

// splitTop splits a comma-separated list at depth zero.
func splitTop(s string) []string {
	var out []string
	d, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		case ',':
			if d == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" {
		out = append(out, last)
	}
	return out
}

// matchParen returns the index just past the ")" closing the "(" at i.
func matchParen(s string, i int) int {
	d := 0
	for ; i < len(s); i++ {
		switch s[i] {
		case '(':
			d++
		case ')':
			d--
			if d == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// ------------------------------------------------------- interfaces, measured
//
// AN INTERFACE IS AN EXISTENTIAL TYPE — `∃X. X × (X → …)`, a hidden
// representation packed with the operations that consume it (Mitchell & Plotkin
// 1988). Packing one is manufacturing a closure, which callbacks.md tier 3
// refuses. But `io.Copy(dst, src)` does not ask us to PACK one: it asks us to
// hand it a value that already is one, and a `*os.File` obtained from `os.Open`
// already is.
//
// So the question splits, and this measures the split rather than arguing it:
//
//	an interface RESULT      a token we hold and call methods on — an opaque
//	                         host type like any other, and nothing new
//	an interface ARGUMENT,   Go inserts the coercion at the call site; the only
//	  we hold an impl        thing missing is a DECLARED edge saying so
//	an interface ARGUMENT,   genuinely tier 3
//	  we hold nothing
//
// A method's identity for this purpose is its NAME and its qualified signature,
// which is Go's own rule: implementation is structural.

type msig struct {
	params, results string
}

// ifaceMethods is the method set of every interface type in the manifest,
// keyed `pkg.Name`. The manifest gives both halves — a brace line listing the
// names (already flattened through embedding) and one line per method with its
// signature — and only the second is needed.
func ifaceMethods(lines []string) map[string]map[string]msig {
	out := map[string]map[string]msig{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "pkg ") {
			continue
		}
		c := strings.Index(line, ", ")
		if c < 0 {
			continue
		}
		pkg, rest := line[4:c], line[c+2:]
		if i := strings.Index(pkg, " "); i >= 0 {
			continue // a platform-qualified line; a different question
		}
		if !strings.HasPrefix(rest, "type ") {
			continue
		}
		rest = rest[5:]
		sp := strings.Index(rest, " ")
		if sp < 0 {
			continue
		}
		name, tail := rest[:sp], rest[sp+1:]
		if !strings.HasPrefix(tail, "interface, ") {
			continue // the brace line, or not an interface at all
		}
		m, ok := parseSig(pkg, tail[len("interface, "):])
		if !ok {
			continue
		}
		k := qual(pkg, name)
		if out[k] == nil {
			out[k] = map[string]msig{}
		}
		out[k][m.name] = msig{params: strings.Join(m.params, ","), results: strings.Join(m.results, ",")}
	}
	return out
}

// parseSig reads `Name(params) results` with every type name qualified against
// the package it was written in, so an interface's `Reader` and a concrete
// method's `io.Reader` compare equal.
func parseSig(pkg, s string) (sym, bool) {
	p := strings.IndexAny(s, "([")
	if p < 0 || s[p] == '[' {
		return sym{}, false // generic; the residual is monomorphic
	}
	var m sym
	m.pkg, m.name = pkg, s[:p]
	rest := s[p:]
	e := matchParen(rest, 0)
	if e < 0 {
		return sym{}, false
	}
	for _, t := range splitTop(rest[1 : e-1]) {
		m.params = append(m.params, qual(pkg, t))
	}
	res := strings.TrimSpace(rest[e:])
	if strings.HasPrefix(res, "(") {
		res = strings.TrimSuffix(strings.TrimPrefix(res, "("), ")")
		for _, t := range splitTop(res) {
			m.results = append(m.results, qual(pkg, t))
		}
	} else if res != "" {
		m.results = append(m.results, qual(pkg, res))
	}
	return m, true
}

// concreteMethods is the method set of every non-interface type, keyed the same
// way. A pointer receiver and a value receiver are DIFFERENT keys, because Go's
// method sets differ: `*T` has both, `T` has only the value methods.
func concreteMethods(syms []sym) map[string]map[string]msig {
	out := map[string]map[string]msig{}
	for _, s := range syms {
		if s.kind != "method" || s.generic {
			continue
		}
		k := qual(s.pkg, s.recv)
		if out[k] == nil {
			out[k] = map[string]msig{}
		}
		var ps, rs []string
		for _, t := range s.params {
			ps = append(ps, qual(s.pkg, t))
		}
		for _, t := range s.results {
			rs = append(rs, qual(s.pkg, t))
		}
		out[k][s.name] = msig{params: strings.Join(ps, ","), results: strings.Join(rs, ",")}
		// A VALUE method belongs to the pointer's set as well. Go's rule, and
		// leaving it out would refuse `*os.File` for an interface satisfied by
		// a method declared on `os.File`.
		if !strings.HasPrefix(s.recv, "*") {
			pk := qual(s.pkg, "*"+s.recv)
			if out[pk] == nil {
				out[pk] = map[string]msig{}
			}
			out[pk][s.name] = out[k][s.name]
		}
	}
	return out
}

// implements is Go's own rule: structural, by name and signature. It is a
// LOOKUP and not an inference — no variance, no quantifiers, no fixed point —
// which is why the subtyping type-algebra.md refuses does not arise. Pierce's
// undecidable F<: is about BOUNDED QUANTIFICATION, and after staging there is
// nothing quantified left.
func implements(have map[string]msig, want map[string]msig) bool {
	if len(want) == 0 {
		return true // `interface{}` — everything is below the top
	}
	for n, w := range want {
		h, ok := have[n]
		if !ok || h != w {
			return false
		}
	}
	return true
}

// candidateImplements is the relation computed from the manifest, before the
// host has been asked. See verifyImplements for why that is only half of it.
func candidateImplements(raw []string, syms []sym) map[string][]string {
	ifaces := ifaceMethods(raw)
	concrete := concreteMethods(syms)
	sat := map[string][]string{}
	for t, ms := range concrete {
		for i, want := range ifaces {
			if len(want) > 0 && implements(ms, want) {
				sat[t] = append(sat[t], i)
			}
		}
	}
	for t := range sat {
		sort.Strings(sat[t])
	}
	return sat
}

// verifiedSat is the host-accepted relation, computed once in main so that the
// usable number and the emitted edges rest on the same verified facts.
var verifiedSat = map[string][]string{}

// ------------------------------------------------------------- struct literals
//
// A STRUCT IS A PRODUCT WITH LABELS — `Π_{f ∈ fields} T_f` — and products.md §7
// defers the heterogeneous product for want of a LAYOUT. On a host that has
// structs there is no layout to invent: `&http.Client{Timeout: d}` is built by
// the HOST, and the format can already say it,
//
//	(prim Client ((timeout time-Duration)) ptr-http-Client expr
//	  "&http.Client{Timeout: %s}")
//
// so this is a GENERATOR question, exactly as methods and several results turned
// out to be. What it needs from the manifest is the field list, which is there.
//
// AND CONSTRUCTIBLE IS NOT USEFUL, which is why this is counted separately from
// `obtainable` rather than folded into it. `&bytes.Buffer{}` is what a Go
// programmer writes; `&os.File{}` is a broken file. A survey that merged the two
// would report a capability the language does not have, which is the failure
// mode four of this tool's five corrections have had.

type structDef struct {
	fields   []sym // name + one result type, as the manifest writes a field
	known    bool
	pkg, tag string // the full import path and the type's own name
	amb      bool   // two packages share a base name and both define this type
}

// structTypes reads `pkg P, type T struct` and its field lines. A type with no
// field line still gets an entry: `bytes.Buffer` has no exported field and
// `&bytes.Buffer{}` is exactly the idiom.
func structTypes(lines []string) map[string]*structDef {
	out := map[string]*structDef{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "pkg ") {
			continue
		}
		c := strings.Index(line, ", ")
		if c < 0 {
			continue
		}
		pkg, rest := line[4:c], line[c+2:]
		if strings.Contains(pkg, " ") || !strings.HasPrefix(rest, "type ") {
			continue
		}
		// THE ISSUE NUMBER IS NOT PART OF THE TYPE. `parseLine` strips ` #` and
		// this did not, so `OmitHost bool #46059` was a field of type
		// "bool #46059" -- unspellable, which demoted `net/url.URL` from
		// every-field-spellable to partial. Found by generating the file the
		// measurement was a projection of.
		if h := strings.Index(rest, " #"); h >= 0 {
			rest = rest[:h]
		}
		rest = rest[5:]
		sp := strings.Index(rest, " ")
		if sp < 0 {
			continue
		}
		name, tail := rest[:sp], rest[sp+1:]
		if strings.ContainsAny(name, "[]") {
			continue // generic
		}
		k := qual(pkg, name)
		switch {
		case tail == "struct":
			if out[k] == nil {
				out[k] = &structDef{pkg: pkg, tag: name}
			}
			out[k].known = true
			out[k].amb = out[k].amb || out[k].pkg != pkg
		case strings.HasPrefix(tail, "struct, "):
			f := tail[len("struct, "):]
			i := strings.Index(f, " ")
			if i < 0 {
				continue
			}
			fn, ft := f[:i], f[i+1:]
			// `CompressedSize //deprecated` IS NOT A FIELD OF TYPE
			// `//deprecated`. The manifest records HISTORY, and a later file
			// marks an existing field deprecated by re-listing it with an
			// ANNOTATION where the type goes. Reading that as a type gave
			// `archive/zip.FileHeader` two `CompressedSize` fields -- a
			// composite literal with a duplicate key, which Go refuses -- and,
			// because a repeated name makes sort-by-name a PARTIAL order, two
			// runs of the generator produced two different files. Found by
			// diffing two emits, which is the check backend-2026-09-06 exists
			// to make routine.
			if strings.HasPrefix(ft, "//") {
				continue
			}
			if fn == "" || fn[0] < 'A' || fn[0] > 'Z' {
				// unexported: not writable in a composite literal. An EMBEDDED
				// field is written `embedded T` and lands here too -- its key
				// in a literal is the type name, and leaving it zero is the
				// conservative reading rather than a limitation.
				continue
			}
			if out[k] == nil {
				out[k] = &structDef{pkg: pkg, tag: name}
			}
			out[k].known = true
			out[k].amb = out[k].amb || out[k].pkg != pkg
			out[k].fields = append(out[k].fields, sym{pkg: pkg, name: fn, results: []string{ft}})
		}
	}
	return out
}

// structCtors turns the manifest's struct definitions into CONSTRUCTORS, as
// ORDINARY SYMS, so the fixed point, the report and the emitter all see one
// shape and cannot disagree about which types a program can build. That is the
// lesson of the subsumption relation two commits ago: the number and the
// emitted declaration must rest on the same facts.
//
//	mk_T : Π_{f ∈ writable(T)} T_f  →  *T          ⟦mk_T⟧ = &T{f: …}
//
// A CONSTRUCTOR IS GENERATED ONLY FOR A STRUCT WITH AT LEAST ONE SPELLABLE
// EXPORTED FIELD, and that restriction is the `pure` rule again: a generator
// does not make a claim it cannot justify. `&bytes.Buffer{}` is the documented
// idiom and `&os.File{}` is a broken file, and the manifest -- the exported API
// -- cannot tell them apart, because the difference is a sentence in a doc
// comment. A zero literal is therefore a HAND declaration, where somebody can
// be answerable for it. That costs 220 struct types and is the difference
// between +1,012 names and +486.
//
// A FIELD WE CANNOT SPELL IS LEFT ZERO, which is not a limitation we impose: a
// composite literal names the fields it sets and a Go program outside the
// package writes exactly the same thing.
//
// FIELDS ARE SORTED BY NAME. A Go map has no order and the emitter must be a
// FUNCTION OF ITS INPUT — backend-2026-09-06, where six identical runs of
// `cmd/gen` produced two different programs, and this is where that would have
// been easiest to miss.
//
// THE CONSTRUCTOR TAKES THE TYPE'S OWN NAME, and it cannot collide with a
// function's: a type and a func are both package-scope identifiers in Go, so
// `url.URL` names exactly one thing and the module `go/net-url` gains `URL`
// with nothing displaced.
// ctorBoth counts the types whose value methods a pointer constructor puts out
// of reach — the honest residue of the rule below.
var ctorBoth int

func structCtors(lines []string, syms []sym) []sym {
	// THE FORM IS DECIDED BY THE TYPE'S OWN METHOD SET, and this is the one
	// place the two languages disagree about a composite literal.
	//
	// In Go, `&T{…}` is a `*T` whose method set holds BOTH the value and the
	// pointer methods, and `T{…}` is a `T` whose method set holds only the
	// value ones — so `&T{…}` is strictly the more capable value there. Our
	// checker compares TYPE NAMES, so it gets no auto-dereference: whichever
	// form is generated is the only method set the program can reach.
	//
	// So generate `&T{…}` exactly when the manifest gives T a pointer-receiver
	// method, and `T{…}` otherwise. `image.Rectangle` and `color.RGBA` are all
	// value methods and become usable; `http.Client` and `os.PathError` need
	// the pointer. A type with BOTH loses its value methods, which is counted
	// rather than hidden: the general fix is an auto-dereference rule in the
	// checker for a receiver position, which is a compiler question and not a
	// generator one.
	ptrRecv, valRecv := map[string]bool{}, map[string]bool{}
	for _, s := range syms {
		if s.kind != "method" {
			continue
		}
		if strings.HasPrefix(s.recv, "*") {
			ptrRecv[s.pkg+"."+s.recv[1:]] = true
		} else {
			valRecv[s.pkg+"."+s.recv] = true
		}
	}
	defs := structTypes(lines)
	var keys []string
	for k := range defs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []sym
	for _, k := range keys {
		d := defs[k]
		if !d.known || d.amb {
			// `math/rand` and `crypto/rand` share a base name, which is what
			// the manifest itself writes; guessing which one a field belongs
			// to would be the tool inventing a fact. Skipped and counted.
			continue
		}
		// DEDUPED BY NAME, LAST WINS -- the rule `parseLine` already uses, for
		// the same reason: the manifest records history, so one field can be
		// listed twice. Deduping is what makes sort-by-name a TOTAL order, and
		// a total order is what makes the generator a function of its input.
		seen := map[string]int{}
		var fs []sym
		for _, f := range d.fields {
			if at, dup := seen[f.name]; dup {
				fs[at] = f
				continue
			}
			seen[f.name] = len(fs)
			fs = append(fs, f)
		}
		sort.Slice(fs, func(i, j int) bool { return fs[i].name < fs[j].name })
		res := d.tag
		if ptrRecv[d.pkg+"."+d.tag] {
			res = "*" + d.tag
			if valRecv[d.pkg+"."+d.tag] {
				ctorBoth++
			}
		}
		c := sym{pkg: d.pkg, kind: "func", name: d.tag, results: []string{res}}
		for _, f := range fs {
			if classify(f.results[0]).why != ok {
				continue
			}
			c.fields = append(c.fields, f.name)
			c.params = append(c.params, f.results[0])
		}
		if len(c.fields) == 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// structReport says what the constructors bought, measured the way everything
// else here is: against the same survey with them removed.
//
// It used to be a PROJECTION -- struct-literals.md's +486 -- and generating the
// files is what turns it into a measurement. Two things moved on contact, both
// recorded in that document's own §5 as risks: a field type carrying the
// manifest's issue number was unspellable, and a struct constructor whose
// argument nothing can build is declarable and NOT usable, which seeding the
// fixed point blindly would have counted anyway.
func structReport(lines []string, syms, ctors []sym, have map[string]bool) {
	defs := structTypes(lines)
	zeroOnly, ambiguous, full, partial := 0, 0, 0, 0
	byName := map[string]*sym{}
	for i := range ctors {
		byName[ctors[i].pkg+"."+ctors[i].name] = &ctors[i]
	}
	for _, d := range defs {
		if !d.known {
			continue
		}
		switch {
		case d.amb:
			ambiguous++
		case byName[d.pkg+"."+d.tag] == nil:
			zeroOnly++
		default:
			spellable := true
			for _, f := range d.fields {
				if classify(f.results[0]).why != ok {
					spellable = false
				}
			}
			if spellable {
				full++
			} else {
				partial++
			}
		}
	}

	// WITHOUT THE CONSTRUCTORS, on the same survey. A constructed
	// `*http.Request` is an argument to functions returning things nothing else
	// reaches, so the fixed point has to be re-run rather than topped up.
	bare := obtainable(syms)
	grown := supplied
	supplied = suppliedBy(bare)
	before := 0
	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" {
			continue
		}
		if d, u, _ := judge(s, bare); d && u {
			before++
		}
	}
	supplied = grown
	after, ctorUsable := 0, 0
	var unlocked []string
	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" {
			continue
		}
		_, was, _ := judge(s, bare)
		d, u, _ := judge(s, have)
		if d && u {
			after++
			if !was && len(unlocked) < 6 {
				n := s.pkg + "." + s.name
				if s.recv != "" {
					n = s.pkg + "." + strings.TrimPrefix(s.recv, "*") + "." + s.name
				}
				unlocked = append(unlocked, n)
			}
		}
	}
	built := 0
	for t := range have {
		if !bare[t] {
			built++
		}
	}
	for _, c := range ctors {
		if d, u, _ := judge(c, have); d && u {
			ctorUsable++
		}
	}

	fmt.Printf("\nSTRUCT LITERALS: %d struct types; %d get a generated constructor\n",
		len(defs), full+partial)
	fmt.Printf("  %d have every exported field spellable; %d have one we cannot spell,\n",
		full, partial)
	fmt.Printf("  which a composite literal LEAVES ZERO -- what a Go program does.\n")
	fmt.Printf("  %d have no exported field and get NOTHING: a zero literal is useful for\n", zeroOnly)
	fmt.Printf("  `bytes.Buffer` and broken for `os.File`, and the manifest cannot tell\n")
	fmt.Printf("  them apart, so that one is a hand declaration -- the `pure` rule again.\n")
	fmt.Printf("  %d are skipped: two packages share a base name and both define the type.\n", ambiguous)
	fmt.Printf("  %d of the %d constructors are themselves USABLE -- the rest want a field\n",
		ctorUsable, len(ctors))
	fmt.Printf("  whose type nothing can build, which is declarable and not callable.\n")
	fmt.Printf("  USABLE GOES %d -> %d (+%d), %.1f%% -> %.1f%% of the callable surface.\n",
		before, after, after-before, pct(before, 4932), pct(after, 4932))
	fmt.Printf("  %d types have BOTH a value and a pointer method and get the pointer form,\n", ctorBoth)
	fmt.Printf("  so their value methods are out of reach: the checker compares type names\n")
	fmt.Printf("  and gets no auto-dereference, which is a compiler question, not this one.\n")
	fmt.Printf("  %d host types are buildable that nothing RETURNS, and they are what the\n", built)
	fmt.Printf("  names above hang off, for example: %s\n", strings.Join(unlocked, ", "))
	fmt.Printf("  Counted apart from the constructors themselves, which are not part of\n")
	fmt.Printf("  the host's callable surface: they are ours, and the surface is Go's.\n")
}

// interfaceReport classifies what an interface-typed position actually costs.
//
// maxlen-2026-08-28's discipline: before building anything, every blocked name
// is classified by the fact that would settle it. There it killed octagons.
func interfaceReport(lines []string, syms []sym, have map[string]bool) {
	ifaces := ifaceMethods(lines)
	concrete := concreteMethods(syms)

	// Which interfaces can we SUPPLY? One pass over the obtainable set.
	supply := map[string][]string{}
	for t := range have {
		ms, ok := concrete[t]
		if !ok {
			continue
		}
		for i, want := range ifaces {
			if implements(ms, want) {
				supply[i] = append(supply[i], t)
			}
		}
	}

	isIface := func(t string) (string, bool) {
		b := strings.TrimPrefix(strings.TrimPrefix(t, "[]"), "*")
		if _, ok := ifaces[b]; ok {
			return b, true
		}
		return "", false
	}

	var byIface = map[string]int{}
	held, unheld, other, resultOnly, resultReadable := 0, 0, 0, 0, 0
	var examples []string
	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" {
			continue
		}
		decl, usable, _ := judge(s, have)
		if !decl || usable {
			continue
		}
		var blockers []string
		pos := append([]string{}, s.params...)
		if s.recv != "" {
			pos = append(pos, s.recv)
		}
		for _, p := range pos {
			v := classify(p)
			if v.opaque && !have[qual(s.pkg, p)] {
				blockers = append(blockers, qual(s.pkg, p))
			}
			if _, yes := isIface(qual(s.pkg, p)); yes {
				blockers = append(blockers, qual(s.pkg, p))
			}
		}
		if len(blockers) == 0 {
			resultOnly++
			// AND CAN THE RESULT BE READ AFTER ALL? `judge` calls an opaque
			// result unreadable whatever it is -- so `os.Open` scores unusable
			// while gauntlet/stdlib/acceptance/os-methods.oro opens a file with
			// it, reads through the methods of what it returns and prints 64.
			// A result whose type is in the OBTAINABLE set has methods, and
			// having methods is the whole of what reading it means here.
			readable := true
			for _, r := range s.results {
				v := classify(r)
				if v.opaque && !have[qual(s.pkg, r)] {
					readable = false
				}
			}
			if readable {
				resultReadable++
			}
			continue
		}
		allIface, allHeld := true, true
		for _, b := range blockers {
			name, yes := isIface(b)
			if !yes {
				allIface = false
				continue
			}
			byIface[name]++
			if len(supply[name]) == 0 {
				allHeld = false
			}
		}
		switch {
		case allIface && allHeld:
			held++
			if len(examples) < 8 {
				examples = append(examples, s.pkg+"."+s.name)
			}
		case allIface:
			unheld++
		default:
			other++
		}
	}

	fmt.Printf("\nINTERFACES: %d declared in the manifest, %d of them satisfied by a type\n",
		len(ifaces), len(supply))
	fmt.Printf("            a program can already obtain\n")
	fmt.Printf("\nWHAT AN INTERFACE-TYPED ARGUMENT COSTS (declarable and not usable)\n")
	fmt.Printf("  every blocker is an interface WE HOLD an implementation of   %5d\n", held)
	fmt.Printf("  every blocker is an interface we hold NOTHING for            %5d\n", unheld)
	fmt.Printf("  at least one blocker is not an interface                     %5d\n", other)
	fmt.Printf("  nothing blocks an argument; the RESULT is what is unread     %5d\n", resultOnly)
	fmt.Printf("    of those, the result type IS obtainable and has methods    %5d\n", resultReadable)
	if len(examples) > 0 {
		fmt.Printf("  reachable by a declared coercion, for example: %s\n",
			strings.Join(examples, ", "))
	}

	type dm struct {
		name string
		n    int
	}
	var ds []dm
	for k, v := range byIface {
		ds = append(ds, dm{k, v})
	}
	sort.Slice(ds, func(i, j int) bool { return byCount(ds[i].n, ds[j].n, ds[i].name, ds[j].name) })
	fmt.Printf("\nTHE INTERFACES MOST ASKED FOR, and what we could hand them\n")
	for i, d := range ds {
		if i >= 12 {
			break
		}
		s := "nothing"
		if len(supply[d.name]) > 0 {
			sort.Strings(supply[d.name])
			s = fmt.Sprintf("%d types, e.g. %s", len(supply[d.name]), supply[d.name][0])
		}
		fmt.Printf("  %-28s %4d positions   %s\n", d.name, d.n, s)
	}
}

// ------------------------------------------------------------------ the symbol

type sym struct {
	pkg, kind, name string
	params, results []string
	recv            string
	generic         bool
	// fields is non-empty only for a STRUCT CONSTRUCTOR (structCtors): the
	// label each parameter fills in, parallel to `params`. It is what makes
	// the template a composite literal rather than a call, and it is the only
	// thing that distinguishes the two shapes downstream.
	fields []string
	// value is a CONSTANT's value as the manifest writes it. The manifest gives
	// a constant two lines — `const MaxRune = 1114111` and `const MaxRune
	// ideal-char` — and dedup by identity kept only the second, so every value
	// was dropped before anything could read it (mergeConst).
	value string
}

func parseLine(line string) (sym, bool) {
	// pkg PATH, KIND ...
	if !strings.HasPrefix(line, "pkg ") {
		return sym{}, false
	}
	c := strings.Index(line, ", ")
	if c < 0 {
		return sym{}, false
	}
	s := sym{pkg: line[4:c]}
	rest := line[c+2:]
	if h := strings.Index(rest, " #"); h >= 0 { // the issue number
		rest = rest[:h]
	}
	sp := strings.Index(rest, " ")
	if sp < 0 {
		return sym{}, false
	}
	s.kind, rest = rest[:sp], rest[sp+1:]

	switch s.kind {
	case "const", "var", "type":
		s.name = rest
		if i := strings.Index(rest, " "); i >= 0 {
			s.name, s.results = rest[:i], []string{rest[i+1:]}
		}
		return s, true
	case "method":
		// method (*Reader) Read([]uint8) (int, error)
		if !strings.HasPrefix(rest, "(") {
			return sym{}, false
		}
		e := matchParen(rest, 0)
		if e < 0 {
			return sym{}, false
		}
		s.recv = rest[1 : e-1]
		rest = strings.TrimSpace(rest[e:])
	case "func":
	default:
		return sym{}, false
	}

	// NAME[typeparams](params) results
	p := strings.IndexAny(rest, "([")
	if p < 0 {
		return sym{}, false
	}
	s.name = rest[:p]
	if rest[p] == '[' {
		s.generic = true
		d := 0
		i := p
		for ; i < len(rest); i++ {
			if rest[i] == '[' {
				d++
			} else if rest[i] == ']' {
				d--
				if d == 0 {
					break
				}
			}
		}
		rest = rest[i+1:]
	} else {
		rest = rest[p:]
	}
	if !strings.HasPrefix(rest, "(") {
		return sym{}, false
	}
	e := matchParen(rest, 0)
	if e < 0 {
		return sym{}, false
	}
	s.params = splitTop(rest[1 : e-1])
	res := strings.TrimSpace(rest[e:])
	if strings.HasPrefix(res, "(") {
		res = strings.TrimSuffix(strings.TrimPrefix(res, "("), ")")
		s.results = splitTop(res)
	} else if res != "" {
		s.results = []string{res}
	}
	return s, true
}

// Why a symbol is DECLARABLE and still not usable. These are not failures of the
// FORMAT — the line can be written — they are the LANGUAGE having no way to make
// or to read the value at one end of it.
const (
	uArgOpaque reason = "cannot build the argument"
	uResOpaque reason = "cannot read the result"
	uError     reason = "result is an error"
)

// builtin is Go's predeclared type names, which belong to no package.
var builtin = map[string]bool{
	"bool": true, "string": true, "error": true, "any": true, "rune": true, "byte": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
}

// supplied is the set of interfaces some OBTAINABLE concrete type satisfies —
// the subsumption relation, read as a question about reachability.
//
// An interface-typed argument is not a blocker when we hold something that goes
// there: the host inserts the coercion and `(implements T I)` is the one line
// that tells our checker so. Empty until `interfaceReport` fills it, so a run
// that does not compute the relation scores exactly as before.
var supplied = map[string]bool{}

// suppliedBy reads that relation off a given obtainable set. It is a function
// rather than a loop in `main` because the struct report has to ask the same
// question of the survey WITHOUT the constructors, and an interface supplied by
// a type only a constructor reaches would otherwise leak into the before.
func suppliedBy(have map[string]bool) map[string]bool {
	out := map[string]bool{}
	for t, ifs := range verifiedSat {
		if !have[t] {
			continue
		}
		for _, i := range ifs {
			out[i] = true
		}
	}
	return out
}

// qual gives a host type the key the obtainable set is indexed by, and it is a
// CORRECTION rather than tidiness.
//
// The api manifest writes a type local to its own package bare — `*File` inside
// `os` — and a type from elsewhere qualified — `io.Reader`. Keyed by the raw
// string, `*File` in `os` and `*File` in `archive/zip` are ONE ENTRY, so
// obtaining an `*os.File` made every `*zip.File` method look reachable. That can
// only inflate the usable number, which is the direction a measurement of one's
// own language must never round.
//
// Base name rather than import path, because the manifest itself writes
// `io.Reader` and not `io.Reader`'s path; two packages with the same base name
// and the same type name still collide, and that residue is named rather than
// hidden.
func qual(pkg, t string) string {
	// A PREDECLARED TYPE BELONGS TO NO PACKAGE. `[]uint8` qualified against `os`
	// is `[]os.uint8`, which matches nothing — and that mattered the moment this
	// was used to compare an interface method against a concrete one, where the
	// two are written in different packages and every builtin would differ.
	if base := strings.TrimLeft(t, "*[]"); builtin[base] {
		return t
	}
	pre := ""
	for {
		switch {
		case strings.HasPrefix(t, "*"):
			pre, t = pre+"*", t[1:]
		case strings.HasPrefix(t, "[]"):
			pre, t = pre+"[]", t[2:]
		default:
			if strings.Contains(t, ".") {
				return pre + t
			}
			base := pkg
			if i := strings.LastIndex(pkg, "/"); i >= 0 {
				base = pkg[i+1:]
			}
			return pre + base + "." + t
		}
	}
}

// judge decides DECLARABLE and USABLE for one symbol.
func judge(s sym, have map[string]bool) (declarable bool, usable bool, why reason) {
	if s.generic {
		return false, false, rGeneric
	}
	// SEVERAL RESULTS ARE DECLARABLE SINCE 2026-09-06 (multiresult-2026-09-06).
	// `Prim.Results` exists, the elimination form is `((f x) (fn (a b) …))`
	// which values.md already had, and a program opens a file. This branch is
	// kept, empty, so the count of what it USED to refuse stays visible.
	_ = rMultiResult
	argOpaque, resOpaque, isErr := false, false, false
	// The receiver is argument 0.
	if s.recv != "" {
		v := classify(s.recv)
		if v.why != ok {
			return false, false, v.why
		}
		argOpaque = argOpaque || (v.opaque && !have[qual(s.pkg, s.recv)])
	}
	for _, p := range s.params {
		v := classify(p)
		if v.why != ok {
			return false, false, v.why
		}
		// An argument the program cannot CONSTRUCT makes the call unreachable
		// even though the line can be written.
		// AND A DECLARED SUBSUMPTION EDGE MAKES AN INTERFACE ARGUMENT
		// REACHABLE. `io.ReadAll(r io.Reader)` is callable because we can
		// obtain an `*os.File` and the host coerces it; that is the whole of
		// what the coercion buys, measured rather than predicted.
		k := qual(s.pkg, p)
		argOpaque = argOpaque || (v.opaque && !have[k] && !supplied[k])
	}
	for _, r := range s.results {
		v := classify(r)
		if v.why != ok {
			return false, false, v.why
		}
		if r == "error" {
			isErr = true
		} else {
			// A RESULT WHOSE TYPE IS OBTAINABLE CAN BE READ, and calling it
			// unreadable was this survey deciding against itself. `os.Open`
			// returns `*os.File`, which is in the obtainable set BY THIS CALL,
			// and gauntlet/stdlib/acceptance/os-methods.oro opens a file with it,
			// reads through that type's methods and prints 64. Having methods is
			// the whole of what reading an opaque value means here.
			//
			// Circular and well-founded: the fixed point decides obtainability
			// without consulting usability, and this consults the fixed point.
			resOpaque = resOpaque || (v.opaque && !have[qual(s.pkg, r)])
		}
	}
	// AN `error` BESIDE A USABLE RESULT IS NOT A BLOCKER, since 2026-09-06.
	// A target declares `(prim err-nil ((e error)) bool expr "%s == nil")` and
	// the program branches on it — which is what `examples/io/wc.oro` does, and
	// what most Go code does at the point of the call. What a program still
	// cannot do is READ an error, which needs an interface (callbacks.md tier
	// 3), so these are counted separately rather than folded in silently: the
	// count is the size of what sums.md would buy.
	switch {
	case argOpaque:
		return true, false, uArgOpaque
	case resOpaque:
		return true, false, uResOpaque
	case isErr:
		errOnly++
		return true, true, ok
	}
	return true, true, ok
}

// errOnly counts calls whose only imperfection is an error the program can test
// and cannot read.
var errOnly int

// obtainable is the least set of host types a program can actually GET HOLD OF.
//
// A host type is obtainable when some declarable function RETURNS it and every
// argument that function needs is itself a scalar or already obtainable. That is
// a least fixed point, computed by iterating to stability — the same shape as
// covering, and for the same reason: reachability from what the program can
// write down.
//
// It exists because "a method takes a receiver the program cannot build" is a
// CLAIM, and the claim is false whenever a constructor exists. `strings.NewReader`
// returns a `*Reader` with a string argument, so every `*Reader` method is
// reachable; `os.Open` returns `(*File, error)`, which the format cannot declare
// at all, so no `*File` method is. Measuring the difference is the point.
// obtainable is the least fixed point with nothing seeded: a type is obtainable
// only when some declarable CONSTRUCTOR returns it.
func obtainable(syms []sym) map[string]bool { return obtainableFrom(syms, nil) }

func obtainableFrom(syms []sym, seed map[string]bool) map[string]bool {
	have := map[string]bool{}
	for k := range seed {
		have[k] = true
	}
	for {
		grew := false
		for _, s := range syms {
			if s.generic || len(s.results) == 0 {
				continue
			}
			// A METHOD ON AN OBTAINABLE RECEIVER IS ALSO A SOURCE, and leaving
			// it out was this survey under-counting itself. `(*os.File).Stat`
			// gives an `fs.FileInfo` from a file we can open, and a program that
			// can open a file can plainly obtain one.
			//
			// FOUND BY WRITING THE JVM SURVEY, where `new` and a method are the
			// two idioms and leaving either out would have been obvious. Go's
			// constructor idiom is a package function, so the omission looked
			// like the whole story — and the two numbers are only comparable
			// once both ask the same question.
			//
			// Well-founded for the same reason the rest is: this is a LEAST
			// fixed point, and a receiver is consulted only once it is already
			// in `have`.
			if s.kind != "func" && s.kind != "method" {
				continue
			}
			if s.recv != "" && !have[qual(s.pkg, s.recv)] {
				continue
			}
			// EVERY RESULT POSITION, not just a lone one. Since 2026-09-06 a
			// target may declare a call that gives back several values, so
			// `os.Open` returning `(*File, error)` is a CONSTRUCTOR — which is
			// the whole reason that change compounds: `(T, error)` is Go's
			// constructor idiom, so it unlocks the type AND every method on it.
			reachable := true
			for _, p := range s.params {
				pv := classify(p)
				if pv.why != ok || (pv.opaque && !have[qual(s.pkg, p)]) {
					reachable = false
					break
				}
			}
			if !reachable {
				continue
			}
			for _, r := range s.results {
				v := classify(r)
				k := qual(s.pkg, r)
				if v.why != ok || !v.opaque || have[k] {
					continue
				}
				have[k] = true
				grew = true
			}
		}
		if !grew {
			return have
		}
	}
}

// ------------------------------------------------------------------------ main

func main() {
	emitDir := flag.String("emit", "", "write the declarable subset as target files into DIR")
	flag.Parse()

	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go env GOROOT:", err)
		os.Exit(1)
	}
	apiDir := filepath.Join(strings.TrimSpace(string(root)), "api")
	files, _ := filepath.Glob(filepath.Join(apiDir, "go1*.txt"))
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no api manifest at", apiDir)
		os.Exit(1)
	}

	seen := map[string]bool{}
	var syms []sym
	index := map[string]int{}
	platform := 0
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			// `pkg syscall (windows-386), …` is one platform's surface.
			if i := strings.Index(line, ", "); i > 0 && strings.Contains(line[:i], " (") {
				platform++
				continue
			}
			if s, ok := parseLine(line); ok {
				// THE MANIFEST RECORDS HISTORY, so one name can appear with
				// two spellings of the same type — `os.Chmod` takes a
				// `FileMode` in go1 and an `fs.FileMode` after go1.16, which
				// is one alias and two lines. Keyed by identity, last wins.
				k := s.pkg + "|" + s.kind + "|" + s.recv + "|" + s.name
				if i, dup := index[k]; dup {
					if s.kind == "const" {
						syms[i] = mergeConst(syms[i], s)
						continue
					}
					syms[i] = s
					continue
				}
				if s.kind == "const" {
					s = mergeConst(sym{pkg: s.pkg, kind: s.kind, name: s.name}, s)
				}
				index[k] = len(syms)
				syms = append(syms, s)
			}
		}
		fh.Close()
	}

	// THE SUBSUMPTION RELATION, COMPUTED AND VERIFIED ONCE, so the usable number
	// and the emitted `(implements ...)` edges rest on the same host-accepted
	// facts. A candidate the Go compiler refuses must not appear in either.
	var raw []string
	for l := range seen {
		raw = append(raw, l)
	}
	cand := candidateImplements(raw, syms)
	nCand := 0
	for _, ifs := range cand {
		nCand += len(ifs)
	}
	if v, passes, err := verifyImplements(syms, cand); err != nil {
		fmt.Fprintf(os.Stderr, "\nNO SUBSUMPTION EDGE IS USED: %v\n"+
			"An unverified edge is a claim, and a claim that makes the type checker\n"+
			"accept a program the host refuses is the failure this avoids.\n", err)
	} else {
		verifiedSat = v
		nKept := 0
		for _, ifs := range v {
			nKept += len(ifs)
		}
		fmt.Printf("SUBSUMPTION: %d candidate edges, %d accepted by the host "+
			"in %d refining pass(es)\n", nCand, nKept, passes)
	}
	// A STRUCT LITERAL IS A CONSTRUCTOR, so the constructors go into the fixed
	// point rather than being seeded into its answer. Seeding would claim every
	// struct with a spellable field is buildable; running the fixed point over
	// them asks whether the FIELDS can be built, which is the same question the
	// fixed point already answers for `os.Open`'s arguments.
	ctors := structCtors(raw, syms)
	have := obtainableFrom(append(append([]sym{}, syms...), ctors...), nil)
	// AN INTERFACE WE CAN SUPPLY IS NOT A BLOCKER. Only an OBTAINABLE concrete
	// type counts: holding nothing that implements `io.Reader` leaves an
	// `io.Reader` argument exactly as unreachable as it was.
	supplied = suppliedBy(have)
	type tally struct{ decl, usable, total int }
	byKind := map[string]*tally{"func": {}, "method": {}}
	byReason := map[reason]int{}
	byGap := map[reason]int{}
	byPkg := map[string]*tally{}
	all := tally{}
	callable := 0
	// CONSTANTS ARE PART OF A PACKAGE, and until unicode/utf8 was supported
	// in full nothing here counted one. A Go constant is evaluated at compile
	// time and has no effect, so its declaration may say `pure` — the one claim
	// this generator can justify for every name of a kind.
	var consts []sym
	constTotal := 0
	constRefused := map[reason]int{}
	for _, s := range syms {
		if s.kind != "const" {
			continue
		}
		constTotal++
		if _, why := constResult(s); why != "" {
			constRefused[reason(why)]++
			continue
		}
		consts = append(consts, s)
	}

	var declarables []sym

	for _, s := range syms {
		if s.kind != "func" && s.kind != "method" {
			continue
		}
		callable++
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

	fmt.Printf("GO STANDARD LIBRARY, %d packages, %d portable symbols\n", len(byPkg), len(syms))
	fmt.Printf("  (%d platform-qualified lines counted separately)\n\n", platform)
	fmt.Printf("CALLABLE SURFACE (func + method): %d\n", all.total)
	fmt.Printf("  declarable as a (prim …):  %5d  %5.1f%%\n", all.decl, pct(all.decl, all.total))
	fmt.Printf("  usable by a program:       %5d  %5.1f%%\n", all.usable, pct(all.usable, all.total))
	fmt.Printf("    of those, %d return an error the program can TEST and cannot READ\n", errOnly)
	// SPLIT BY KIND, because the two are different questions. A `func` is an
	// operation on values; a `method` needs a RECEIVER, which is a host object
	// the program has no way to build. Reporting them together hides which of
	// the two the parasite model actually reaches.
	for _, k := range []string{"func", "method"} {
		t := byKind[k]
		fmt.Printf("    %-7s %5d total  %5d declarable (%4.1f%%)  %5d usable (%4.1f%%)\n",
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
		fmt.Printf("  %-24s %6d  %5.1f%%   [%s]\n", x.r, x.n, pct(x.n, all.total), blame(x.r))
	}

	fmt.Printf("\nHOST TYPES A PROGRAM CAN OBTAIN: %d (least fixed point over declarable constructors)\n", len(have))
	fmt.Println("\nAND WHY A DECLARABLE NAME IS STILL NOT USABLE")
	var gs []rc
	for r, n := range byGap {
		gs = append(gs, rc{r, n})
	}
	sort.Slice(gs, func(i, j int) bool { return byCount(gs[i].n, gs[j].n, gs[i].r, gs[j].r) })
	for _, x := range gs {
		fmt.Printf("  %-24s %6d  %5.1f%% of declarable\n", x.r, x.n, pct(x.n, all.decl))
	}

	fmt.Println("\nBY PACKAGE (the twenty a program is most likely to want)")
	want := []string{"strings", "strconv", "math", "math/bits", "sort", "slices", "unicode",
		"unicode/utf8", "bytes", "os", "io", "fmt", "time", "errors", "encoding/json",
		"net/http", "regexp", "path/filepath", "sync", "context"}
	fmt.Printf("  %-20s %6s %6s %6s\n", "package", "total", "decl", "usable")
	for _, p := range want {
		if t := byPkg[p]; t != nil {
			fmt.Printf("  %-20s %6d %6d %6d\n", p, t.total, t.decl, t.usable)
		} else {
			fmt.Printf("  %-20s %6s\n", p, "-")
		}
	}

	// THE INTERFACE QUESTION, MEASURED RATHER THAN ARGUED. See interfaceReport.
	interfaceReport(raw, syms, have)
	structReport(raw, syms, ctors, have)

	fmt.Printf("\nCONSTANTS: %d exported, %d declarable\n", constTotal, len(consts))
	var crs []string
	for r := range constRefused {
		crs = append(crs, string(r))
	}
	sort.Slice(crs, func(i, j int) bool {
		return byCount(constRefused[reason(crs[i])], constRefused[reason(crs[j])], crs[i], crs[j])
	})
	for _, r := range crs {
		fmt.Printf("  %-44s %5d\n", r, constRefused[reason(r)])
	}

	if *emitDir != "" {
		if err := emit(*emitDir, append(append(declarables, ctors...), consts...), raw); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
	}
}

// writeImplementsCheck writes the subsumption relation as GO SOURCE, so the
// HOST decides whether we got it right.
//
// This is a conformance story no `prim` template has. A template is a claim
// nobody can check until some program happens to call it; an edge is
//
//	var _ io.Reader = *new(*os.File)
//
// one line, and `go build` refuses the file if the claim is false. `*new(T)`
// rather than `(T)(nil)` because it is well typed for EVERY T — a pointer, an
// interface, and `os.FileMode`, which is an integer.
//
// A base name that two packages share is SKIPPED and counted rather than
// guessed at: the manifest writes `rand.Rand` and does not say which `rand`.
func writeImplementsCheck(path string, syms []sym, sat map[string][]string) (map[string]int, error) {
	lineOf := map[string]int{}
	pkgOf, ambiguous := map[string]string{}, map[string]bool{}
	for _, s := range syms {
		base := s.pkg
		if i := strings.LastIndex(base, "/"); i >= 0 {
			base = base[i+1:]
		}
		if p, seen := pkgOf[base]; seen && p != s.pkg {
			ambiguous[base] = true
		}
		pkgOf[base] = s.pkg
	}
	pkgOfType := func(t string) (string, bool) {
		t = strings.TrimLeft(t, "*[]")
		i := strings.Index(t, ".")
		if i < 0 {
			return "", false
		}
		base := t[:i]
		if ambiguous[base] {
			return "", false
		}
		p, ok := pkgOf[base]
		return p, ok
	}
	type edge struct{ t, i string }
	var edges []edge
	need := map[string]bool{}
	skipped := 0
	var subs []string
	for t := range sat {
		subs = append(subs, t)
	}
	sort.Strings(subs)
	for _, t := range subs {
		pt, ok := pkgOfType(t)
		if !ok || strings.Contains(pt, "internal") {
			skipped += len(sat[t])
			continue
		}
		for _, i := range sat[t] {
			pi, ok := pkgOfType(i)
			if !ok || strings.Contains(pi, "internal") {
				skipped++
				continue
			}
			edges = append(edges, edge{t, i})
			need[pt], need[pi] = true, true
		}
	}
	var b strings.Builder
	b.WriteString("// GENERATED by gauntlet/stdlib/survey.go — do not edit.\n")
	b.WriteString("//\n")
	b.WriteString("// Every (implements T I) edge the generated target files declare, as a\n")
	b.WriteString("// Go assignment. `go build` on this file is the host checking our claim.\n\n")
	b.WriteString("package check\n\nimport (\n")
	var imps []string
	for p := range need {
		imps = append(imps, p)
	}
	sort.Strings(imps)
	for _, p := range imps {
		fmt.Fprintf(&b, "\t%q\n", p)
	}
	b.WriteString(")\n\n")
	// The line NUMBER is the key: a compiler error names a line, and that is how
	// a refused edge is identified. Counted rather than assumed, so the header
	// above may change without breaking the mapping.
	line := strings.Count(b.String(), "\n") + 1
	for n, e := range edges {
		fmt.Fprintf(&b, "var _%d %s = *new(%s)\n", n, e.i, e.t)
		lineOf[e.t+" "+e.i] = line
		line++
	}
	fmt.Fprintf(&b, "\n// %d edges; %d skipped for an ambiguous or internal package.\n",
		len(edges), skipped)
	return lineOf, os.WriteFile(path, []byte(b.String()), 0o644)
}

// verifyImplements asks the HOST which candidate edges are true, and keeps only
// those. It is the difference between a measurement and a claim.
//
// THE MANIFEST CANNOT SEE A SEALED INTERFACE. `go1.txt` lists the EXPORTED API,
// and `ast.Decl` is `{ Pos, End, declNode }` with the third unexported — so the
// structural computation sees two methods, finds them on every AST node, and
// concludes that `*ast.ArrayType` is an `ast.Decl`. It is not, and `go build`
// says so in one line. That is not a bug in the rule; it is the manifest not
// containing the fact, and no amount of care with the manifest recovers it.
//
// So the relation is CANDIDATE-GENERATE and HOST-FILTER. Every surviving edge
// has been accepted by the Go compiler, which is a stronger guarantee than any
// `prim` template in this repository carries — a template is checked only when
// some program happens to call it.
//
// AND IF THE TOOLCHAIN CANNOT RUN, NO EDGE IS EMITTED. An unverified edge is a
// claim, and a claim that makes the type checker accept a program the host will
// refuse is exactly the failure this whole file exists to avoid.
func verifyImplements(syms []sym, sat map[string][]string) (map[string][]string, int, error) {
	dir, err := os.MkdirTemp("", "oroimpl")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module check\n\ngo 1.21\n"), 0o644); err != nil {
		return nil, 0, err
	}
	src := filepath.Join(dir, "implements_check.go")
	for pass := 0; pass < 20; pass++ {
		lineOf, err := writeImplementsCheck(src, syms, sat)
		if err != nil {
			return nil, 0, err
		}
		cmd := exec.Command("go", "build", "-gcflags=-e", "./...")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err == nil {
			return sat, pass, nil
		}
		// `implements_check.go:93:8: …` — the line names the edge that failed.
		bad := map[int]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			i := strings.Index(line, "implements_check.go:")
			if i < 0 {
				continue
			}
			rest := line[i+len("implements_check.go:"):]
			j := strings.Index(rest, ":")
			if j < 0 {
				continue
			}
			var n int
			if _, err := fmt.Sscanf(rest[:j], "%d", &n); err == nil {
				bad[n] = true
			}
		}
		if len(bad) == 0 {
			return nil, 0, fmt.Errorf("the host refused the check and named no edge:\n%s", out)
		}
		next := map[string][]string{}
		for t, ifs := range sat {
			for _, i := range ifs {
				if !bad[lineOf[t+" "+i]] {
					next[t] = append(next[t], i)
				}
			}
		}
		sat = next
	}
	return nil, 0, fmt.Errorf("the candidate relation did not settle in twenty passes")
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

// emit writes the declarable subset as target files.
//
// IT MUST WRITE WHAT `judge` COUNTS, or the percentage is a claim. This is the
// same defect win32-2026-09-08 fixed on the other survey and it was worse here:
// `judge` had learned about several results, methods and voids and the generator
// had not, so 130 declarable names in `os` produced 36 lines. A name counted
// declarable and never emitted is a claim; the generated count is the one that
// has to be true.
//
// THREE SHAPES THE FORMAT ALWAYS HAD.
//
//	several results   `(T, error)` is Go's constructor idiom, declarable since
//	                  multiresult-2026-09-06 and the reason the obtainable-type
//	                  fixed point has anything in it.
//	a METHOD          the receiver is argument 0 — the same convention Win32's
//	                  `HANDLE` already uses — and the module is `go/PKG/TYPE`,
//	                  so `f.Read(b)` is `(File.Read f b)`.
//	a VOID with an    a statement's value IS its first argument, which is what
//	argument          `stmt` means. Win32's survey refused 447 of these and 409
//	                  were declarable all along.
//
// AND IT NO LONGER CLAIMS `pure`. Every generated line said `pure`, including
// `os.Chdir` — an operation whose whole purpose is to change global state, which
// a pure declaration lets the reducer substitute into two places or drop
// entirely (ADR 0010). Purity is one declared bit whose default is IMPURE
// precisely so that an author's omission costs speed rather than correctness
// (effects.md); a generator cannot justify the claim for 1,007 functions, so it
// does not make it. A target author adding `pure` by hand is how it comes back.
func emit(dir string, syms []sym, raw []string) error {
	// The relation was computed and HOST-VERIFIED once, in main: the usable
	// number and the emitted edges must rest on the same accepted facts.
	sat := verifiedSat
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
	n, skipped := 0, 0
	for _, p := range pkgs {
		list := byPkg[p]
		sort.Slice(list, func(i, j int) bool {
			if list[i].recv != list[j].recv {
				return list[i].recv < list[j].recv
			}
			return list[i].name < list[j].name
		})
		// An opaque type must be SPELLED for the host, or the emitted file names
		// a type the backend cannot write. Collected as we go and declared at
		// target level, because a type is the target's and not a module's.
		types := map[string]string{}
		// A TYPE IS RECORDED AS IT IS SPELLED, POINTER AND ALL. `os.Open` gives
		// back `*os.File` and `(*File).Read`'s receiver is the same type, so the
		// declaration has to be `(type ptr-os-File "*os.File")` — recording the
		// pointee would name a type no prim mentions and leave every prim naming
		// one the target does not have.
		var spellQ func(string) string
		spellQ = func(t string) string {
			if _, isScalar := scalar[t]; isScalar {
				return spell(t)
			}
			if strings.HasPrefix(t, "[]") {
				return "(array " + spellQ(t[2:]) + ")"
			}
			if strings.HasPrefix(t, "map[") {
				return "(map int " + spellQ(t[strings.Index(t, "]")+1:]) + ")"
			}
			q := qual(p, t)
			types[spell(q)] = q
			return spell(q)
		}
		base := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			base = p[i+1:]
		}
		// One buffer per MODULE: package-level functions in `go/PKG`, and each
		// type's methods in `go/PKG/TYPE`.
		mods := map[string]*strings.Builder{}
		modBuf := func(name string) *strings.Builder {
			if b, ok := mods[name]; ok {
				return b
			}
			b := &strings.Builder{}
			mods[name] = b
			return b
		}
		for _, s := range list {
			if s.kind == "const" {
				// `utf8.MaxRune` is a primitive of no arguments whose result
				// is the EXACT range [v, v], so the interval analysis knows the
				// value and `(+ (u.MaxRune) 1)` is provable. The emitter
				// receives a range result as the language's integer.
				res, why := constResult(s)
				if why != "" {
					continue
				}
				fmt.Fprintf(modBuf("go/"+strings.ReplaceAll(p, "/", "-")),
					"    (sig %s () %s pure (host expr \"%s.%s\" (import %q)))\n",
					s.name, res, base, s.name, p)
				n++
				continue
			}
			var args, holes []string
			mod := "go/" + strings.ReplaceAll(p, "/", "-")
			tmplRecv := ""
			if s.recv != "" {
				rt := strings.TrimPrefix(s.recv, "*")
				mod = "go/" + strings.ReplaceAll(p, "/", "-") + "/" + spell(rt)
				args = append(args, "(self "+spellQ(s.recv)+")")
				holes = append(holes, "%s")
				tmplRecv = "%s."
			}
			for k, a := range s.params {
				// A CONSTRUCTOR'S PARAMETER IS NAMED FOR ITS FIELD, because
				// that is the only thing at a call site that says which slot a
				// value fills: `(u.URL "https" "example.com" …)` has eleven of
				// them and the declaration is where a reader finds out which.
				nm := fmt.Sprintf("a%d", k)
				if len(s.fields) > 0 {
					nm = strings.ToLower(s.fields[k][:1]) + s.fields[k][1:]
				}
				args = append(args, fmt.Sprintf("(%s %s)", nm, spellQ(a)))
				holes = append(holes, hostArg(a))
			}
			argList := "(none)"
			if len(args) > 0 {
				argList = "(" + strings.Join(args, " ") + ")"
			}
			// The call text. A method is `%s.Name(…)`; a function is
			// `pkg.Name(…)`, and the receiver hole is already in `holes`.
			callArgs := holes
			if s.recv != "" {
				callArgs = holes[1:]
			}
			call := tmplRecv + s.name + "(" + strings.Join(callArgs, ", ") + ")"
			if s.recv == "" {
				call = base + "." + call
			}
			// A STRUCT CONSTRUCTOR IS A COMPOSITE LITERAL, and that is the only
			// place the template is not a call. PARENTHESISED, because a
			// literal is a value here: `&T{…}.M()` parses as `&(T{…}.M())`,
			// which is not addressable, and a bare literal in a `for` header or
			// an `if` condition is ambiguous in Go's own grammar.
			if len(s.fields) > 0 {
				var kv []string
				for k, f := range s.fields {
					kv = append(kv, f+": "+holes[k])
				}
				amp := ""
				if strings.HasPrefix(s.results[0], "*") {
					amp = "&"
				}
				call = "(" + amp + base + "." + s.name + "{" + strings.Join(kv, ", ") + "})"
			}
			kind, res := "expr", ""
			switch len(s.results) {
			case 0:
				// A statement's value is its first argument, so a void with no
				// argument has nothing to be. That is the honest refusal and it
				// is small: `runtime.Gosched` and its kind.
				if len(args) == 0 {
					skipped++
					continue
				}
				kind, res = "stmt", strings.TrimSuffix(strings.TrimPrefix(args[0],
					"("+strings.Fields(args[0][1:])[0]+" "), ")")
			case 1:
				res = spellQ(s.results[0])
			default:
				var rs []string
				for _, r := range s.results {
					rs = append(rs, spellQ(r))
				}
				res = "(" + strings.Join(rs, " ") + ")"
			}
			fmt.Fprintf(modBuf(mod), "    (sig %s %s %s (host %s \"%s\" (import %q)))\n",
				s.name, strings.Replace(argList, "(none)", "()", 1), res, kind, call, p)
			n++
		}
		// THE EDGES FOR THE TYPES THIS FILE DECLARES. Put with the CONCRETE type,
		// because that is where the file already spells it -- and the interface it
		// satisfies is spelled here too, so a program loading one file is never left
		// naming a type the target does not have.
		edges := map[string][]string{}
		for k, q := range types {
			ifs := sat[q]
			if len(ifs) == 0 {
				continue
			}
			for _, iq := range ifs {
				edges[k] = append(edges[k], spell(iq))
			}
		}
		for _, ifs := range sat {
			_ = ifs
		}
		for k := range edges {
			for _, q := range sat[types[k]] {
				types[spell(q)] = q
			}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "; GENERATED by gauntlet/stdlib/survey.go — do not edit.\n")
		fmt.Fprintf(&b, "; The declarable subset of Go's %s.\n", p)
		b.WriteString("(target go\n")
		var tn []string
		for k := range types {
			tn = append(tn, k)
		}
		sort.Strings(tn)
		for _, k := range tn {
			fmt.Fprintf(&b, "  (type %s (host %q))\n", k, types[k])
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
		out := filepath.Join(dir, strings.ReplaceAll(p, "/", "-")+".oro")
		if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	if _, err := writeImplementsCheck(filepath.Join(dir, "implements_check.go"), syms, sat); err != nil {
		return err
	}
	fmt.Printf("\nemitted %d primitives across %d packages into %s (%d void with no argument)\n",
		n, len(pkgs), dir, skipped)
	return nil
}

// spell maps a Go type to the name a target file uses for it. An opaque type
// gets its own name, which the generated file would also have to `(type …)`.
func spell(t string) string {
	if s, isScalar := scalar[t]; isScalar {
		if t == "error" {
			return "error"
		}
		return s
	}
	if strings.HasPrefix(t, "[]") {
		return "(array " + spell(t[2:]) + ")"
	}
	if strings.HasPrefix(t, "map[") {
		return "(map int " + spell(t[strings.Index(t, "]")+1:]) + ")"
	}
	// An opaque host type keeps the host's own name, with `.` and `*` made
	// legal in an identifier position.
	t = strings.NewReplacer("*", "ptr-", ".", "-", "/", "-").Replace(t)
	return t
}

// hostArg is the template hole for one argument, CONVERTED to the host's own
// parameter type when that type is not the target's integer.
//
// A range says what a value IS, and our integer is Go's `int` whatever the
// range; so `(u.RuneLen r)` hands `utf8.RuneLen` an `int`, which Go refuses —
// `cannot use r (variable of type int) as rune value`. Every earlier acceptance
// program passed a LITERAL, which Go treats as an untyped constant and converts
// silently, so the refusal was invisible to every test this tool had. The host
// type is known here and nowhere else — a hand-written `(int 0 64)` is a range,
// not a Go type — which is why the conversion is written into the template, as
// `bits.OnesCount64(uint64(%s))` was verified in gostdlib-2026-09-06, rather
// than inferred by the emitter.
//
// Only the narrow integer types are converted. `int64`, `uint` and `uint64` are
// spelled `int` here and are the next package's question, because converting a
// negative value to `uint64` wraps silently.
func hostArg(goType string) string {
	switch goType {
	case "int8", "int16", "int32", "rune", "uint8", "byte", "uint16", "uint32":
		return goType + "(%s)"
	}
	return "%s"
}

// mergeConst folds a constant's two manifest lines into one symbol: the
// `= VALUE` line gives the value, the other gives the type. Either may come
// first, and a later spelling of either replaces an earlier one, which is the
// manifest's own history rule.
func mergeConst(old, s sym) sym {
	if len(s.results) > 0 && strings.HasPrefix(s.results[0], "= ") {
		old.value = strings.TrimPrefix(s.results[0], "= ")
		return old
	}
	s.value = old.value
	return s
}

// portableWindow is ADR 0012's: an `int` is exact within ±(2^53−1).
var portableWindow = new(big.Int).SetUint64(1<<53 - 1)

// constResult spells a constant's result, or says why it cannot be declared.
//
// An INTEGER constant is the exact range [v, v] — which is both its type and
// the one fact the interval analysis needs — and one outside ADR 0012's window
// is refused rather than promoted: `math.MaxUint64` is a value the language's
// integer cannot hold, and declaring it an `int` would make the Go compiler
// refuse `int(math.MaxUint64)` at best. A constant of a NAMED type
// (`fs.ModeDir FileMode`) is refused for now: its value is a `FileMode`, and
// a range would drop the name every method on it needs.
func constResult(s sym) (string, string) {
	t := ""
	if len(s.results) > 0 {
		t = s.results[0]
	}
	switch t {
	case "ideal-int", "ideal-char", "int", "int8", "int16", "int32", "int64", "rune",
		"uint", "uint8", "byte", "uint16", "uint32", "uint64", "uintptr":
		// A PORTABLE TYPE AND A PER-PLATFORM VALUE. `math.MaxInt`,
		// `math/bits.UintSize` and `os.O_APPEND` have their type in the
		// portable section and their value only on lines like `pkg math
		// (darwin-amd64), const MaxInt = …`, because the value depends on the
		// platform. A declaration fixing one value would be a claim about a
		// platform, so there is none.
		if s.value == "" {
			return "", "integer constant whose value is per-platform"
		}
		v, ok := new(big.Int).SetString(s.value, 0)
		if !ok {
			return "", "integer constant with no readable value"
		}
		if new(big.Int).Abs(v).Cmp(portableWindow) > 0 {
			return "", "integer constant outside the portable window"
		}
		return fmt.Sprintf("(int %s %s)", v, v), ""
	case "ideal-float", "float64", "float32":
		return "f64", ""
	case "ideal-string", "string":
		return "string", ""
	case "ideal-bool", "bool":
		return "bool", ""
	case "ideal-complex", "complex64", "complex128":
		return "", "complex constant"
	}
	return "", "constant of a named type"
}
