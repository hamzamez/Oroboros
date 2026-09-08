//go:build ignore

// HOW MUCH OF THE WINDOWS API CAN THIS LANGUAGE DECLARE?
//
// The companion to survey.go, and it is deliberately the same three questions
// asked of a different ecosystem — because the interesting result is where the
// two DISAGREE. Go's standard library is values and objects; Win32 is words and
// handles, and this language has no objects and nothing but words.
//
// The universe is the installed Windows SDK headers, `um/` and `shared/`. That
// is the API's own account of itself, the way `$GOROOT/api` is Go's.
//
// TWO THINGS WIN32 HAS THAT GO'S MANIFEST DOES NOT.
//
//	SAL       `_In_opt_` says a pointer may be NULL. So a parameter we cannot
//	          BUILD may still be one we can PASS — which has no analogue in the
//	          Go survey and changes the answer materially.
//
//	one word  every handle, pointer and integer is one register on x86-64
//	          (windows-target.md), so `opaque` costs less here than it does on a
//	          host with a type system.
//
//	go run gauntlet/stdlib/win32.go
//	go run gauntlet/stdlib/win32.go -emit DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ------------------------------------------------------------------ the types
//
// Every one of these is ONE REGISTER on x86-64, which is the whole reason this
// survey reads differently from the Go one. `windows-target.md`: a table is one
// register because a fat pointer needs two and this convention passes one value
// per register. A HANDLE is a word; so is every pointer.

var word = map[string]bool{}

func init() {
	for _, t := range strings.Fields(`
		BOOL BOOLEAN BYTE CHAR UCHAR WCHAR TCHAR SHORT USHORT WORD ATOM
		INT UINT LONG ULONG DWORD LONGLONG ULONGLONG DWORDLONG
		INT8 INT16 INT32 INT64 UINT8 UINT16 UINT32 UINT64
		LONG32 LONG64 ULONG32 ULONG64 DWORD32 DWORD64 SHORT64
		INT_PTR UINT_PTR LONG_PTR ULONG_PTR DWORD_PTR SIZE_T SSIZE_T
		HRESULT NTSTATUS LRESULT LPARAM WPARAM LSTATUS
		COLORREF LCID LANGID LGRPID SCODE HFILE
		int unsigned char short long float double size_t wchar_t __int64 __int32
		MMRESULT CONFIGRET DNS_STATUS SECURITY_INFORMATION REGSAM ACCESS_MASK
		FLOAT DOUBLE`) {
		word[t] = true
	}
}

// A NUL-terminated string is a pointer, and it is the one pointer this language
// can already PRODUCE on this target: a string literal emits as `db …,0` and its
// label is a word. That makes `LPCSTR` categorically different from every other
// pointer here (string-literals.md, and the differential case `string-escapes`).
var strPtr = map[string]bool{
	"LPCSTR": true, "LPSTR": true, "PCSTR": true, "PSTR": true,
	"LPCWSTR": true, "LPWSTR": true, "PCWSTR": true, "PWSTR": true,
	"LPCTSTR": true, "LPTSTR": true, "LPCCH": true, "LPCH": true,
}

// A COM interface identifier is passed by reference — `REFIID` is `const IID&`
// in C++ and `const IID*` in C. It is a pointer to a constant the program
// cannot build, which is exactly what it is counted as.
var refPtr = map[string]bool{
	"REFIID": true, "REFGUID": true, "REFCLSID": true, "REFFMTID": true,
	"REFPROPERTYKEY": true, "REFKNOWNFOLDERID": true,
}

type reason string

const (
	ok reason = ""

	// The target FORMAT cannot say it.
	rVariadic  reason = "variadic"
	// A `void` result is refused only when there is no argument either. With
	// one, the format already says it: `stmt`'s value IS argument 0, which is
	// how `Sleep` and `WriteFile` are declared in targets/windows/kernel32.oro
	// today. Refusing all of them attributed 3.5% of the API to the format when
	// the format could say it — the survey talking about itself for the third
	// time, after methods-at-0% in the Go survey and the typedef residue here.
	rVoidRes   reason = "void result and no argument"
	rOutParam  reason = "out-parameter"   // the result comes back through a pointer
	rStructVal reason = "struct by value" // does not fit one register

	// The LANGUAGE cannot say it.
	rCallback reason = "function pointer"
	rUnknown  reason = "unresolved typedef"
)

func blame(r reason) string {
	switch r {
	case rVariadic, rVoidRes, rOutParam, rStructVal:
		return "format"
	case ok:
		return "-"
	}
	return "language"
}

// ------------------------------------------------------------------ the symbol

type param struct {
	sal  string // _In_, _In_opt_, _Out_, _Out_writes_(n), …
	typ  string
	name string
}

type fn struct {
	hdr      string
	name     string
	result   string
	params   []param
	variadic bool
}

var (
	// A flat C entry point. The SDK writes the decoration, the result, the
	// calling convention and the name on separate lines, which is what makes
	// this tractable without a C parser.
	reFn = regexp.MustCompile(
		`(?s)([A-Za-z_][A-Za-z0-9_ *]*?)\s+(?:WINAPI|APIENTRY|NTAPI|STDAPICALLTYPE|WINAPIV)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^;{}]*)\)\s*;`)
	// STDAPI expands to EXTERN_C HRESULT STDAPICALLTYPE, so the result is
	// implicit. STDAPI_(t) names it.
	reStdapi = regexp.MustCompile(
		`(?s)\bSTDAPI(_\(([^)]*)\))?\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^;{}]*)\)\s*;`)
	reComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reLine     = regexp.MustCompile(`//[^\n]*`)
	reCallback = regexp.MustCompile(`\b(?:CALLBACK|STDMETHODCALLTYPE)\b`)
	reSAL      = regexp.MustCompile(`^_[A-Za-z][A-Za-z0-9_]*(?:\([^)]*\))?`)
)

// parseParams splits a parameter list and peels the SAL annotation off each one.
func parseParams(s string) ([]param, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "void" || s == "VOID" {
		return nil, false
	}
	var out []param
	variadic := false
	depth, start := 0, 0
	parts := []string{}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	for _, raw := range parts {
		p := strings.Join(strings.Fields(raw), " ")
		if p == "" {
			continue
		}
		if p == "..." {
			variadic = true
			continue
		}
		var sal []string
		for {
			m := reSAL.FindString(p)
			if m == "" {
				break
			}
			sal = append(sal, m)
			p = strings.TrimSpace(p[len(m):])
		}
		// Drop qualifiers the shape does not depend on.
		for _, q := range []string{"CONST ", "const ", "struct ", "unsigned ", "volatile ",
			"union ", "enum "} {
			p = strings.ReplaceAll(p, q, "")
		}
		// THE LEGACY ANNOTATIONS. `IN`, `OUT`, `OPTIONAL` are SAL's predecessors
		// and are still everywhere in the SDK — they carry the same information
		// in an older spelling, so they are folded into `sal` rather than
		// dropped, and leaving them in the TYPE was worth 300 false "unresolved
		// typedef" verdicts.
		for {
			f := strings.Fields(p)
			if len(f) < 2 {
				break
			}
			switch f[0] {
			case "IN":
				sal = append(sal, "_In_")
			case "OUT":
				sal = append(sal, "_Out_")
			case "OPTIONAL":
				sal = append(sal, "_opt_")
			default:
				goto doneLegacy
			}
			p = strings.Join(f[1:], " ")
		}
	doneLegacy:
		// The last identifier is the parameter's name, unless the whole thing
		// is a bare type.
		f := strings.Fields(p)
		pp := param{sal: strings.Join(sal, " ")}
		if len(f) >= 2 && !strings.HasSuffix(f[len(f)-1], "*") {
			pp.name = strings.TrimSuffix(f[len(f)-1], "[]")
			pp.typ = strings.Join(f[:len(f)-1], " ")
		} else {
			pp.typ = p
		}
		pp.typ = strings.TrimSpace(pp.typ)
		if strings.HasSuffix(pp.name, "[]") || strings.Contains(pp.name, "[") {
			pp.typ += "*"
		}
		out = append(out, pp)
	}
	return out, variadic
}

// ------------------------------------------------------------- classification

type shape int

const (
	shWord   shape = iota // one register, and the program can compute it
	shString              // a NUL-terminated literal: one register, and we can MAKE one
	shPtr                 // one register, and the program cannot make one
	shStruct              // does not fit a register
	shFunc                // a code address
	shVoid
	shUnknown
)

func classify(t string, structs, handles, callbacks map[string]bool, alias map[string]string) shape {
	// FOLLOW THE ALIAS CHAIN, bounded so a cycle in the headers cannot hang the
	// tool. `MSIHANDLE` is `ULONG_PTR` is a word; without this every status and
	// handle typedef in the SDK counted as unresolved, and that was 23.5% of
	// the surface attributed to the LANGUAGE when it belonged to this parser.
	for i := 0; i < 8; i++ {
		sh := classify1(t, structs, handles, callbacks)
		if sh != shUnknown {
			return sh
		}
		nxt, have := alias[strings.TrimSpace(t)]
		if !have || nxt == t {
			return shUnknown
		}
		t = nxt
	}
	return shUnknown
}

func classify1(t string, structs, handles, callbacks map[string]bool) shape {
	t = strings.TrimSpace(strings.TrimPrefix(t, "_"))
	for _, q := range []string{"CONST ", "const ", "unsigned ", "signed ", "struct ",
		"union ", "enum ", "volatile "} {
		t = strings.TrimPrefix(t, q)
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return shVoid
	}
	if t == "void" || t == "VOID" {
		return shVoid
	}
	if strings.HasSuffix(t, "*") {
		return shPtr
	}
	if word[t] {
		return shWord
	}
	if strPtr[t] {
		return shString
	}
	if refPtr[t] {
		return shPtr
	}
	if handles[t] || strings.HasPrefix(t, "H") && len(t) > 1 && strings.ToUpper(t) == t {
		return shWord // a handle IS a word — this is the parasite model working
	}
	if callbacks[t] {
		return shFunc
	}
	if structs[t] {
		return shStruct
	}
	// LP…/P… naming is the SDK's own convention for a pointer.
	if strings.HasPrefix(t, "LP") || (strings.HasPrefix(t, "P") && strings.ToUpper(t) == t && len(t) > 2) {
		return shPtr
	}
	return shUnknown
}

// canPass says whether a program could supply this argument TODAY.
//
// This is where SAL earns its place. A pointer to a structure we cannot build
// is unusable — unless the annotation says NULL is accepted, in which case the
// call is reachable with a literal 0. `_Reserved_` means it must be 0, which is
// even better: the parameter is decided.
func canPass(p param, sh shape) bool {
	switch sh {
	case shWord, shString:
		return true
	case shPtr:
		return strings.Contains(p.sal, "_opt_") || strings.Contains(p.sal, "_Reserved_")
	}
	return false
}

func main() {
	sdk := flag.String("sdk", "", "SDK include directory (default: the newest installed)")
	emitDir := flag.String("emit", "", "write the declarable subset as target files into DIR")
	flag.Parse()

	dir := *sdk
	if dir == "" {
		var err error
		dir, err = newestSDK()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	structs := map[string]bool{}
	handles := map[string]bool{}
	callbacks := map[string]bool{}
	alias := map[string]string{}
	var fns []fn
	comMethods := 0
	files := 0

	for _, sub := range []string{"um", "shared"} {
		hs, _ := filepath.Glob(filepath.Join(dir, sub, "*.h"))
		for _, h := range hs {
			b, err := os.ReadFile(h)
			if err != nil {
				continue
			}
			files++
			src := reLine.ReplaceAllString(reComment.ReplaceAllString(string(b), " "), " ")
			collectTypes(src, structs, handles, callbacks, alias)
			comMethods += len(reCallback.FindAllString(src, -1))
			base := filepath.Base(h)
			for _, m := range reFn.FindAllStringSubmatch(src, -1) {
				res := strings.Fields(m[1])
				r := ""
				if len(res) > 0 {
					r = res[len(res)-1]
					if strings.HasPrefix(r, "WIN") || strings.HasPrefix(r, "EXTERN") {
						r = ""
					}
				}
				ps, va := parseParams(m[3])
				fns = append(fns, fn{base, m[2], r, ps, va})
			}
			for _, m := range reStdapi.FindAllStringSubmatch(src, -1) {
				r := "HRESULT"
				if m[2] != "" {
					r = strings.TrimSpace(m[2])
				}
				ps, va := parseParams(m[4])
				fns = append(fns, fn{base, m[3], r, ps, va})
			}
		}
	}

	// One name, one entry — the A/W pairs are two real entry points and stay.
	seen := map[string]bool{}
	var uniq []fn
	for _, f := range fns {
		if seen[f.name] {
			continue
		}
		seen[f.name] = true
		uniq = append(uniq, f)
	}
	fns = uniq

	var declarable, callable, usable int
	byReason := map[reason]int{}
	byGap := map[reason]int{}
	salCount := map[string]int{}
	unres := map[string]int{}
	nullPassed := 0
	arity := map[int]int{}
	var decls []fn

	for _, f := range fns {
		why := ok
		anyOutParam := false
		for _, p := range f.params {
			for _, s := range strings.Fields(p.sal) {
				salCount[salKind(s)]++
			}
		}
		if f.variadic {
			why = rVariadic
		}
		if why == ok {
			for _, p := range f.params {
				sh := classify(p.typ, structs, handles, callbacks, alias)
				switch sh {
				case shStruct:
					why = rStructVal
				case shFunc:
					why = rCallback
				case shUnknown:
					why = rUnknown
					unres[p.typ]++
				case shVoid:
				}
				if sh == shPtr && strings.HasPrefix(p.sal, "_Out") {
					anyOutParam = true
				}
				if why != ok {
					break
				}
			}
		}
		if why == ok {
			switch classify(f.result, structs, handles, callbacks, alias) {
			case shStruct:
				why = rStructVal
			case shUnknown:
				why = rUnknown
				unres[f.result]++
			case shVoid:
				// A stmt hands back its first argument, so a void result needs
				// one and nothing more.
				if len(f.params) == 0 {
					why = rVoidRes
				}
			}
		}
		if why != ok {
			byReason[why]++
			continue
		}
		declarable++
		arity[len(f.params)]++
		decls = append(decls, f)

		reach := true
		usedNull := false
		for _, p := range f.params {
			sh := classify(p.typ, structs, handles, callbacks, alias)
			if !canPass(p, sh) {
				reach = false
				break
			}
			if sh == shPtr {
				usedNull = true
			}
		}
		if !reach {
			byGap[rOutParam]++
			continue
		}
		callable++
		if usedNull {
			nullPassed++
		}
		// The result is a word, so it can be tested, compared and stored — the
		// only question left is whether the CALL says everything. An out-param
		// means the answer came back through memory we cannot read.
		if anyOutParam {
			byGap[rOutParam]++
			continue
		}
		usable++
	}

	fmt.Printf("WINDOWS SDK %s\n", filepath.Base(dir))
	fmt.Printf("  %d headers, %d distinct flat entry points\n", files, len(fns))
	fmt.Printf("  (%d COM/callback method slots counted separately)\n\n", comMethods)

	fmt.Printf("FLAT C API: %d\n", len(fns))
	fmt.Printf("  declarable as a (prim …):  %5d  %5.1f%%\n", declarable, pct(declarable, len(fns)))
	fmt.Printf("  callable by a program:     %5d  %5.1f%%\n", callable, pct(callable, len(fns)))
	fmt.Printf("  and the call says it all:  %5d  %5.1f%%\n\n", usable, pct(usable, len(fns)))
	// AND NULL IS NOW WRITABLE, which this count assumed and nothing provided:
	// the language has no null, an integer literal is an `int` and the checker
	// refuses it where `ptr` is required, and the target declared nothing. So
	// 17% of the callable number was a claim about a capability nobody had.
	// `(x64.null)` is one line of target data, added 2026-09-08.
	fmt.Printf("  of the callable, %d reach it only by passing NULL where SAL says _opt_/_Reserved_\n", nullPassed)
	fmt.Printf("  — writable since 2026-09-08 as (x64.null); before that the count was aspirational\n\n")

	// ARITY WAS A REAL CEILING ON A HOST WITH NO EXPRESSIONS, and it is not one
	// any more. The three limits this reported — four for the Win64 register
	// quota, six for the emitter's reserved home space, nine for the `%1`…`%9`
	// template holes — were a convention, a constant and a syntax, and the last
	// two were raised on 2026-09-08: asmShadow reserves 112 bytes, enough for
	// the widest entry point in the SDK, and `%{10}` names a tenth operand.
	//
	// The breakdown stays because the SHAPES are still worth seeing — what
	// fraction of an OS API does not fit four registers is a fact about the API
	// — but none of these rows is a refusal now. `-emit` writes a template for
	// every declarable name.
	var over4, over6, over9, maxA int
	for n, c := range arity {
		if n > 4 {
			over4 += c
		}
		if n > 6 {
			over6 += c
		}
		if n > 9 {
			over9 += c
		}
		if n > maxA {
			maxA = n
		}
	}
	fmt.Printf("ARITY, of the declarable (widest is %d arguments)\n", maxA)
	fmt.Printf("  more than 4 — some arguments go on the stack:  %5d  %5.1f%%\n", over4, pct(over4, declarable))
	fmt.Printf("  more than 6 — into the 112-byte reserved home: %5d  %5.1f%%\n", over6, pct(over6, declarable))
	fmt.Printf("  more than 9 — named by the %%{10} braced hole:  %5d  %5.1f%%\n\n", over9, pct(over9, declarable))

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
		fmt.Printf("  %-24s %6d  %5.1f%%   [%s]\n", x.r, x.n, pct(x.n, len(fns)), blame(x.r))
	}

	if len(unres) > 0 {
		type uc struct {
			t string
			n int
		}
		var us []uc
		for t, n := range unres {
			us = append(us, uc{t, n})
		}
		sort.Slice(us, func(i, j int) bool { return us[i].n > us[j].n })
		fmt.Println("\nTOP UNRESOLVED TYPEDEFS (this tool's residue, not a language limit)")
		for i, x := range us {
			if i >= 25 {
				break
			}
			fmt.Printf("  %-32s %5d\n", x.t, x.n)
		}
		fmt.Printf("  … %d distinct in total\n", len(us))
	}

	fmt.Println("\nSAL, BY WHAT IT SAYS")
	var ks []string
	for k := range salCount {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return salCount[ks[i]] > salCount[ks[j]] })
	total := 0
	for _, k := range ks {
		total += salCount[k]
	}
	for _, k := range ks {
		fmt.Printf("  %-22s %7d  %5.1f%%\n", k, salCount[k], pct(salCount[k], total))
	}

	if *emitDir != "" {
		if err := emit(*emitDir, decls, structs, handles, callbacks, alias); err != nil {
			fmt.Fprintln(os.Stderr, "emit:", err)
			os.Exit(1)
		}
	}
}

// salKind buckets an annotation by WHAT IT ASSERTS, which is the question
// general-purpose.md asks: how much of SAL is something our refinement layer
// already decides?
func salKind(s string) string {
	base := s
	if i := strings.Index(base, "("); i >= 0 {
		base = base[:i]
	}
	switch {
	case strings.Contains(base, "reads") || strings.Contains(base, "writes") ||
		strings.Contains(base, "bytes") || strings.Contains(base, "part"):
		return "buffer + size"
	case strings.Contains(base, "range"):
		return "value range"
	case strings.HasPrefix(base, "_Success"):
		return "success condition"
	case strings.HasPrefix(base, "_Ret"):
		return "postcondition"
	case strings.HasPrefix(base, "_Reserved"):
		return "must be zero"
	case strings.HasSuffix(base, "_opt_"):
		return "nullable"
	case strings.HasPrefix(base, "_In"):
		return "direction: in"
	case strings.HasPrefix(base, "_Out"):
		return "direction: out"
	case strings.HasPrefix(base, "_Inout"):
		return "direction: inout"
	}
	return "other"
}

var (
	reHandle = regexp.MustCompile(`DECLARE_HANDLE\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	// `typedef ULONG_PTR MSIHANDLE;` — a plain alias. Win32 has hundreds, and
	// following them is the difference between a measurement and this tool's
	// own vocabulary.
	reAlias  = regexp.MustCompile(`typedef\s+([A-Za-z_][A-Za-z0-9_ ]*?)\s+(\**[A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*\**[A-Za-z_][A-Za-z0-9_]*)*)\s*;`)
	reStruct = regexp.MustCompile(`\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:,[^;]*)?;`)
	reCbType = regexp.MustCompile(`typedef[^;]*\(\s*(?:CALLBACK|WINAPI|APIENTRY|NTAPI|__stdcall|\*)\s*\*?\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
)

// collectTypes builds the tables `classify` consults. Approximate on purpose:
// what it cannot resolve is counted as `unresolved typedef` rather than guessed,
// so the residue is visible instead of being folded into a better-looking number.
func collectTypes(src string, structs, handles, callbacks map[string]bool, alias map[string]string) {
	for _, m := range reAlias.FindAllStringSubmatch(src, -1) {
		t := strings.TrimSpace(m[1])
		// `typedef DWORD DEVNODE, DEVINST;` names two aliases, and
		// `typedef DEVNODE *PDEVNODE, *PDEVINST;` names two POINTERS to one.
		// Both spellings are common in the SDK, and taking only the first name
		// left 500 typedefs unresolved.
		for _, nm := range strings.Split(m[2], ",") {
			nm = strings.TrimSpace(nm)
			to := t
			for strings.HasPrefix(nm, "*") {
				nm, to = strings.TrimSpace(nm[1:]), to+"*"
			}
			if nm != "" && nm != to && alias[nm] == "" {
				alias[nm] = to
			}
		}
	}
	for _, m := range reHandle.FindAllStringSubmatch(src, -1) {
		handles[m[1]] = true
	}
	for _, m := range reStruct.FindAllStringSubmatch(src, -1) {
		structs[m[1]] = true
	}
	for _, m := range reCbType.FindAllStringSubmatch(src, -1) {
		callbacks[m[1]] = true
	}
}

func newestSDK() (string, error) {
	for _, root := range []string{
		`C:\Program Files (x86)\Windows Kits\10\Include`,
		`C:\Program Files\Windows Kits\10\Include`,
	} {
		ents, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var vs []string
		for _, e := range ents {
			if e.IsDir() {
				vs = append(vs, e.Name())
			}
		}
		if len(vs) == 0 {
			continue
		}
		sort.Strings(vs)
		return filepath.Join(root, vs[len(vs)-1]), nil
	}
	return "", fmt.Errorf("no Windows SDK found; pass -sdk")
}

// win64 writes the call the way the windows target actually declares one:
// arguments into rcx/rdx/r8/r9 and then the reserved home space, the call, and
// rax into the destination the emitter allocated. Entirely mechanical, which is
// the point — this is what "a line of data per name" costs on a host with no
// expressions.
func win64(name string, n int) (string, bool) {
	// FOURTEEN is the widest entry point in the SDK, and the emitter now
	// reserves exactly that much (emit/asm.go's asmShadow). It was six, and
	// both ceilings under it were a constant and a hole syntax rather than
	// anything about the host.
	if n > 14 {
		return "", false
	}
	regs := []string{"rcx", "rdx", "r8", "r9"}
	// A tenth operand needs the BRACED hole: `%12` cannot mean operand 12,
	// because it would have to mean operand 1 followed by the character `2` in
	// a template with fewer operands, and a template's meaning may not depend
	// on its arity.
	hole := func(i int) string {
		if i < 9 {
			return fmt.Sprintf("%%%d", i+1)
		}
		return fmt.Sprintf("%%{%d}", i+1)
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i < 4 {
			fmt.Fprintf(&b, "mov %s, %s\n", regs[i], hole(i))
		} else {
			fmt.Fprintf(&b, "mov rax, %s\nmov [rsp+%d], rax\n", hole(i), 32+8*(i-4))
		}
	}
	fmt.Fprintf(&b, "call %s\nmov %%r, rax", name)
	return b.String(), true
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func emit(dir string, fns []fn, structs, handles, callbacks map[string]bool, alias map[string]string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	byHdr := map[string][]fn{}
	for _, f := range fns {
		byHdr[f.hdr] = append(byHdr[f.hdr], f)
	}
	var hs []string
	for h := range byHdr {
		hs = append(hs, h)
	}
	sort.Strings(hs)
	n := 0
	for _, h := range hs {
		list := byHdr[h]
		sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
		mod := strings.TrimSuffix(h, ".h")
		var b strings.Builder
		fmt.Fprintf(&b, "; GENERATED by gauntlet/stdlib/win32.go — do not edit.\n")
		fmt.Fprintf(&b, "; The declarable subset of the Windows SDK's %s.\n\n", h)
		fmt.Fprintf(&b, "(target windows\n  (module win/%s\n", mod)
		for _, f := range list {
			var args []string
			for _, p := range f.params {
				switch classify(p.typ, structs, handles, callbacks, alias) {
				case shString:
					args = append(args, "string")
				case shPtr:
					args = append(args, "ptr")
				default:
					args = append(args, "int")
				}
			}
			al := "(none)"
			if len(args) > 0 {
				al = "(" + strings.Join(args, " ") + ")"
			}
			res, kind := "int", "expr"
			switch classify(f.result, structs, handles, callbacks, alias) {
			case shPtr:
				res = "ptr"
			case shVoid:
				// THE RESULT OF A STATEMENT IS ITS FIRST ARGUMENT, on every
				// target, so a void entry point is declared `stmt` and typed by
				// the argument it hands back. `Sleep` and `WriteFile` in
				// targets/windows/kernel32.oro are hand-written this way and
				// always were; this tool refused 409 of them.
				res, kind = args[0], "stmt"
			}
			tmpl, okArity := win64(f.name, len(f.params))
			if !okArity {
				continue
			}
			if kind == "stmt" {
				// A statement's template must not write `%r`: the emitter takes
				// the value from argument 0 and allocates no result register,
				// so `%r` would have nothing to expand to.
				tmpl = strings.TrimSuffix(tmpl, "\nmov %r, rax")
			}
			fmt.Fprintf(&b, "    (prim %s %s %s %s %q (import %q))\n",
				f.name, al, res, kind, tmpl, f.name)
			n++
		}
		fmt.Fprintf(&b, "  ))\n")
		if err := os.WriteFile(filepath.Join(dir, mod+".oro"), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("\nemitted %d primitives across %d headers into %s\n", n, len(hs), dir)
	return nil
}
