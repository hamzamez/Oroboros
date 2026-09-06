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

var scalar = map[string]string{
	"bool": "bool", "string": "string",
	"int": "int", "int8": "int", "int16": "int", "int32": "int", "int64": "int",
	"uint": "int", "uint8": "int", "uint16": "int", "uint32": "int", "uint64": "int",
	"byte": "int", "rune": "int",
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

// ------------------------------------------------------------------ the symbol

type sym struct {
	pkg, kind, name string
	params, results []string
	recv            string
	generic         bool
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

// judge decides DECLARABLE and USABLE for one symbol.
func judge(s sym, have map[string]bool) (declarable bool, usable bool, why reason) {
	if s.generic {
		return false, false, rGeneric
	}
	if len(s.results) > 1 {
		// The FORMAT's limit, not the language's: `Prim.Result` is one string,
		// while values.md gives the language several results and measures them
		// at parity. Reported as its own reason for exactly that reason.
		return false, false, rMultiResult
	}
	argOpaque, resOpaque, isErr := false, false, false
	// The receiver is argument 0.
	if s.recv != "" {
		v := classify(s.recv)
		if v.why != ok {
			return false, false, v.why
		}
		argOpaque = argOpaque || (v.opaque && !have[s.recv])
	}
	for _, p := range s.params {
		v := classify(p)
		if v.why != ok {
			return false, false, v.why
		}
		// An argument the program cannot CONSTRUCT makes the call unreachable
		// even though the line can be written.
		argOpaque = argOpaque || (v.opaque && !have[p])
	}
	for _, r := range s.results {
		v := classify(r)
		if v.why != ok {
			return false, false, v.why
		}
		if r == "error" {
			isErr = true
		} else {
			resOpaque = resOpaque || v.opaque
		}
	}
	switch {
	case argOpaque:
		return true, false, uArgOpaque
	case isErr:
		return true, false, uError
	case resOpaque:
		return true, false, uResOpaque
	}
	return true, true, ok
}

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
func obtainable(syms []sym) map[string]bool {
	have := map[string]bool{}
	for {
		grew := false
		for _, s := range syms {
			if s.kind != "func" || s.generic || len(s.results) != 1 || s.recv != "" {
				continue
			}
			r := s.results[0]
			v := classify(r)
			if v.why != ok || !v.opaque || have[r] {
				continue
			}
			reachable := true
			for _, p := range s.params {
				pv := classify(p)
				if pv.why != ok || (pv.opaque && !have[p]) {
					reachable = false
					break
				}
			}
			if reachable {
				have[r] = true
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
					syms[i] = s
					continue
				}
				index[k] = len(syms)
				syms = append(syms, s)
			}
		}
		fh.Close()
	}

	have := obtainable(syms)
	type tally struct{ decl, usable, total int }
	byKind := map[string]*tally{"func": {}, "method": {}}
	byReason := map[reason]int{}
	byGap := map[reason]int{}
	byPkg := map[string]*tally{}
	all := tally{}
	callable := 0
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
	sort.Slice(rs, func(i, j int) bool { return rs[i].n > rs[j].n })
	for _, x := range rs {
		fmt.Printf("  %-24s %6d  %5.1f%%   [%s]\n", x.r, x.n, pct(x.n, all.total), blame(x.r))
	}

	fmt.Printf("\nHOST TYPES A PROGRAM CAN OBTAIN: %d (least fixed point over declarable constructors)\n", len(have))
	fmt.Println("\nAND WHY A DECLARABLE NAME IS STILL NOT USABLE")
	var gs []rc
	for r, n := range byGap {
		gs = append(gs, rc{r, n})
	}
	sort.Slice(gs, func(i, j int) bool { return gs[i].n > gs[j].n })
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

	if *emitDir != "" {
		if err := emit(*emitDir, declarables); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

// emit writes the declarable subset as target files, one per Go package. This is
// the half that turns "expressible in principle" into a thing that either loads
// or does not.
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
		sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
		var b strings.Builder
		fmt.Fprintf(&b, "; GENERATED by gauntlet/stdlib/survey.go — do not edit.\n")
		fmt.Fprintf(&b, "; The declarable subset of Go's %s.\n\n", p)
		fmt.Fprintf(&b, "(target go\n  (module go/%s\n", strings.ReplaceAll(p, "/", "-"))
		base := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			base = p[i+1:]
		}
		for _, s := range list {
			if s.recv != "" {
				continue // a method needs a receiver expression; §, below
			}
			var args []string
			var holes []string
			for range s.params {
				holes = append(holes, "%s")
			}
			for _, a := range s.params {
				args = append(args, spell(a))
			}
			res := "none"
			if len(s.results) == 1 {
				res = spell(s.results[0])
			}
			argList := "(none)"
			if len(args) > 0 {
				argList = "(" + strings.Join(args, " ") + ")"
			}
			if res == "none" {
				continue // a void function has no value; stmt would need arg 0
			}
			fmt.Fprintf(&b, "    (prim %s %s %s expr \"%s.%s(%s)\" pure (import %q))\n",
				s.name, argList, res, base, s.name, strings.Join(holes, ", "), p)
			n++
		}
		fmt.Fprintf(&b, "  ))\n")
		out := filepath.Join(dir, strings.ReplaceAll(p, "/", "-")+".oro")
		if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("\nemitted %d primitives across %d packages into %s\n", n, len(pkgs), dir)
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
