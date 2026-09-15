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
// AND TWO QUESTIONS THIS TOOL ASKS THE HOST RATHER THAN ANSWERING ITSELF, both
// because it has been wrong about them (structval-2026-09-12): what an
// aggregate's Win64 layout is, and whether a name it read as an enum really is
// an integer type. MSVC knows both, the way `go build` knows which subsumption
// edges are real (coercion-2026-09-09). The sizes are committed in
// `win32-sizes.txt` so the report stays a function of the headers alone.
//
//	go run gauntlet/stdlib/win32.go
//	go run gauntlet/stdlib/win32.go -emit DIR
//	go run gauntlet/stdlib/win32.go -sig CreateFileA          one signature
//	go run gauntlet/stdlib/win32.go -check-enums              needs MSVC
//	go run gauntlet/stdlib/win32.go -sizes gauntlet/stdlib/win32-sizes.txt
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ------------------------------------------------------------------ the types
//
// Every one of these is ONE REGISTER on x86-64, which is the whole reason this
// survey reads differently from the Go one. `windows-target.md`: a table is one
// register because a fat pointer needs two and this convention passes one value
// per register. A HANDLE is a word; so is every pointer.

var word = map[string]bool{}

// floating is NOT a word: Win64 passes it in XMM0-3 and returns it in XMM0.
var floating = map[string]bool{"FLOAT": true, "DOUBLE": true, "float": true, "double": true}

func init() {
	for _, t := range strings.Fields(`
		BOOL BOOLEAN BYTE CHAR UCHAR WCHAR TCHAR SHORT USHORT WORD ATOM
		INT UINT LONG ULONG DWORD LONGLONG ULONGLONG DWORDLONG
		INT8 INT16 INT32 INT64 UINT8 UINT16 UINT32 UINT64
		LONG32 LONG64 ULONG32 ULONG64 DWORD32 DWORD64 SHORT64
		INT_PTR UINT_PTR LONG_PTR ULONG_PTR DWORD_PTR SIZE_T SSIZE_T
		HRESULT NTSTATUS LRESULT LPARAM WPARAM LSTATUS
		COLORREF LCID LANGID LGRPID SCODE HFILE
		int unsigned char short long size_t wchar_t __int64 __int32
		MMRESULT CONFIGRET DNS_STATUS SECURITY_INFORMATION REGSAM ACCESS_MASK
		`) {
		word[t] = true
	}
}

// enumBase maps every enum name the headers declare -- the typedef name, its tag
// and any comma-listed alias -- to the C type it is STORED as: `int`, unless the
// declaration names another (`typedef enum _X : BYTE {...} X;`, five of them in
// the SDK, plus 29 C++ `enum class`).
var enumBase = map[string]string{}

// enumWasStruct records the enum names the struct table had captured. It is the
// number assessment-2026-09-09 asserted as "675" without ever measuring it.
var enumWasStruct = map[string]bool{}

// structHdr records the header each aggregate name was first declared in, so a
// sizeof probe can include the file that defines it.
//
// THE HOST IS THE ORACLE FOR A LAYOUT. The ABI class of a struct is decided by
// its SIZE — Win64 passes an aggregate of exactly 1, 2, 4 or 8 bytes in a
// register and everything else by reference — and a size computed here would
// have to get bitfields, `#pragma pack`, anonymous unions and nested alignment
// right on 900 declarations. MSVC already knows. So `-sizes` asks it, the way
// coercion-2026-09-09 asked `go build` which subsumption edges are real.
var structHdr = map[string]string{}

func baseOr(b string) string {
	if b = strings.TrimSpace(b); b == "" {
		return "int"
	}
	return b
}

// resultMove says how a result reaches our register, and it is a CORRECTNESS
// question rather than a cosmetic one. Under Win64 an integer result narrower
// than 64 bits is in EAX, AX or AL and the rest of RAX is not part of it -- and
// writing EAX ZERO-extends, so `mov %r, rax` turned every negative `int` into a
// large positive one. `MulDiv(-7, 6, 2)` read 4294967275, and every failing
// `HRESULT`, whose failure IS the negative value, read as a success. Measured,
// not inferred: gauntlet/stdlib/acceptance/signed-result.oro.
//
// The C type decides, after the alias chain and after an enum becomes its base.
// A type this cannot place keeps the full register, which is right for every
// pointer, handle and 64-bit word.
func resultMove(t string, alias map[string]string) string {
	for i := 0; i < 8; i++ {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "_"))
		for _, q := range []string{"CONST ", "const ", "volatile ", "enum ", "struct ", "union "} {
			b = strings.TrimPrefix(b, q)
		}
		b = strings.TrimSpace(b)
		if strings.HasSuffix(b, "*") {
			return "mov %r, rax"
		}
		// `unsigned` is a qualifier classify1 strips because the SHAPE does not
		// depend on it, and here it is the whole answer: `unsigned int`
		// sign-extended would turn 3,000,000,000 negative.
		unsigned := false
		if b == "unsigned" || strings.HasPrefix(b, "unsigned ") {
			unsigned, b = true, strings.TrimSpace(strings.TrimPrefix(b, "unsigned"))
			if b == "" {
				b = "int"
			}
		}
		b = strings.TrimSpace(strings.TrimPrefix(b, "signed "))
		if eb, isEnum := enumBase[b]; isEnum {
			b = eb
		}
		if w, signed, known := intWidth(b); known {
			if unsigned {
				signed = false
			}
			switch {
			case w == 64:
				return "mov %r, rax"
			case w == 32 && signed:
				return "movsxd %r, eax"
			case w == 32:
				return "mov %er, eax"
			case w == 16 && signed:
				return "movsx %r, ax"
			case w == 16:
				return "movzx %er, ax"
			case w == 8 && signed:
				return "movsx %r, al"
			default:
				return "movzx %er, al"
			}
		}
		nxt, have := alias[b]
		if !have || nxt == b {
			return "mov %r, rax"
		}
		t = nxt
	}
	return "mov %r, rax"
}

// intWidth gives a Win64 integer type's width in bits and whether it is signed.
// Windows is LLP64, not LP64: `long` is 32 bits here, and so are `LONG`,
// `HRESULT` and `clock_t`, which is the whole reason this table exists. TCHAR
// is WCHAR in a UNICODE build, which is how these headers are read.
func intWidth(b string) (int, bool, bool) {
	switch b {
	case "INT", "LONG", "BOOL", "HRESULT", "NTSTATUS", "LSTATUS", "INT32", "LONG32",
		"SCODE", "HFILE", "DNS_STATUS", "int", "long", "__int32":
		return 32, true, true
	case "UINT", "ULONG", "DWORD", "UINT32", "ULONG32", "DWORD32", "COLORREF", "LCID",
		"LGRPID", "SECURITY_INFORMATION", "REGSAM", "ACCESS_MASK", "MMRESULT", "CONFIGRET":
		return 32, false, true
	case "SHORT", "short", "INT16":
		return 16, true, true
	case "USHORT", "WORD", "ATOM", "WCHAR", "wchar_t", "LANGID", "UINT16", "TCHAR":
		return 16, false, true
	case "CHAR", "char", "INT8":
		return 8, true, true
	case "BYTE", "UCHAR", "BOOLEAN", "UINT8", "bool":
		return 8, false, true
	case "LONGLONG", "ULONGLONG", "DWORDLONG", "INT64", "UINT64", "LONG64", "ULONG64",
		"DWORD64", "SHORT64", "INT_PTR", "UINT_PTR", "LONG_PTR", "ULONG_PTR", "DWORD_PTR",
		"SIZE_T", "SSIZE_T", "LRESULT", "LPARAM", "WPARAM", "size_t", "__int64", "long long":
		return 64, true, true
	}
	return 0, false, false
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
	rVariadic reason = "variadic"
	// A `void` result is refused only when there is no argument either. With
	// one, the format already says it: `stmt`'s value IS argument 0, which is
	// how `Sleep` and `WriteFile` are declared in targets/windows/kernel32.oro
	// today. Refusing all of them attributed 3.5% of the API to the format when
	// the format could say it — the survey talking about itself for the third
	// time, after methods-at-0% in the Go survey and the typedef residue here.
	rVoidRes   reason = "void result and no argument"
	rOutParam  reason = "out-parameter"   // the result comes back through a pointer
	rStructVal reason = "struct by value" // does not fit one register
	// FLOATING POINT is passed and returned in XMM registers under Win64,
	// and the generated template moves arguments into RCX/RDX/R8/R9 and
	// reads RAX. `FLOAT` and `DOUBLE` sat in the word table, so every one of
	// these was declared and would have passed and read the wrong register
	// -- a claim this generator cannot justify, so it is refused until a
	// template says which XMM register. targets/windows/msvcrt.oro writes
	// printf's double by hand, `movsd xmm1`, which is the shape it wants.
	rFloat reason = "floating point"

	// The LANGUAGE cannot say it.
	rCallback reason = "function pointer"
	rUnknown  reason = "unresolved typedef"
)

func blame(r reason) string {
	switch r {
	case rVariadic, rVoidRes, rOutParam, rStructVal, rFloat:
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
		//
		// THE DECLARATOR CARRIES THE POINTER, AND IT IS THE SDK'S OWN STYLE.
		// C binds `*` to the DECLARATOR, so `SURFOBJ *pso` is a pointer whose
		// name is `pso` — and taking the last field as the name threw the star
		// away, making the parameter a `SURFOBJ` BY VALUE. That is not a
		// cosmetic misread: it put 78 entry points under the largest refusal in
		// the table, and in the other direction it made `DWORD *pcb` a WORD, so
		// a pointer the program cannot build was counted as passable and
		// `-emit` generated a prim taking an integer where the host wants an
		// address. `TYPE name[]` had the same hole — `TrimSuffix` removed the
		// brackets before the test that looks for them could see them.
		f := strings.Fields(p)
		pp := param{sal: strings.Join(sal, " ")}
		if len(f) >= 2 && !strings.HasSuffix(f[len(f)-1], "*") {
			d := f[len(f)-1]
			stars := 0
			for strings.HasPrefix(d, "*") {
				stars, d = stars+1, d[1:]
			}
			if i := strings.Index(d, "["); i >= 0 {
				stars, d = stars+1, d[:i]
			}
			pp.name = d
			pp.typ = strings.Join(f[:len(f)-1], " ") + strings.Repeat("*", stars)
			// A declarator this does NOT read is an inline function pointer,
			// `int (*cb)(void)`, where the name is inside parentheses and the
			// type is spread around it. Counted rather than assumed absent:
			// the SDK writes callbacks as typedefs, and if that stops being
			// true the row says so.
			if strings.ContainsAny(d, "()") {
				oddDeclarators++
			}
		} else {
			pp.typ = p
		}
		pp.typ = strings.TrimSpace(pp.typ)
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
	shFloat               // passed and returned in an XMM register
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

// cleanType strips the qualifiers a SHAPE does not depend on. Factored out of
// classify1 so that `structName` can report the same spelling classify1
// matched — a sizeof probe has to ask the host about the name the headers use.
func cleanType(t string) string {
	t = strings.TrimSpace(strings.TrimPrefix(t, "_"))
	for _, q := range []string{"CONST ", "const ", "unsigned ", "signed ", "struct ",
		"union ", "enum ", "volatile "} {
		t = strings.TrimPrefix(t, q)
	}
	return strings.TrimSpace(t)
}

func classify1(t string, structs, handles, callbacks map[string]bool) shape {
	t = cleanType(t)
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
	if floating[t] {
		return shFloat
	}
	// AN ENUM IS A C `int` (or its declared underlying type), decided HERE,
	// before the naming heuristics below: `PROCESS_DPI_AWARENESS` is all
	// capitals and starts with P, so the LP.../P... rule would call it a
	// pointer, and the struct table would call it a struct.
	if _, isEnum := enumBase[t]; isEnum {
		// Recorded so `-check-enums` can ask the host to confirm it. The
		// enum-or-struct decision has been wrong twice, and getting it wrong
		// this way is the DANGEROUS direction: an aggregate over 8 bytes is
		// passed as a POINTER to a copy, so declaring it a word makes the
		// callee dereference whatever integer the program passed.
		enumUsed[t] = true
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

// structName follows the same alias chain `classify` follows and returns the
// name that made the type an aggregate — the spelling a sizeof probe has to ask
// the host about. `PRECTL` is a pointer and `RECTL` is not, and which of the two
// a parameter says decides whether this is asked at all.
func structName(t string, structs, handles, callbacks map[string]bool, alias map[string]string) string {
	for i := 0; i < 8; i++ {
		if classify1(t, structs, handles, callbacks) == shStruct {
			return cleanType(t)
		}
		nxt, have := alias[strings.TrimSpace(t)]
		if !have || nxt == t {
			return cleanType(t)
		}
		t = nxt
	}
	return cleanType(t)
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
	sig := flag.String("sig", "", "print the parsed signature and verdict for one entry point")
	sizes := flag.String("sizes", "", "ask MSVC for each blocking aggregate's size and write it to FILE")
	checkEnums := flag.Bool("check-enums", false, "ask MSVC to confirm every enum a declaration relies on is an integer type")
	sizeTab := flag.String("sizetable", sizeTableFile, "read aggregate sizes from FILE")
	flag.Parse()
	readSizes(*sizeTab)

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
			base := filepath.Base(h)
			collectTypes(base, src, structs, handles, callbacks, alias)
			comMethods += len(reCallback.FindAllString(src, -1))
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

	// -sig prints ONE entry point as this tool sees it. It exists because every
	// number here is a sum over 11,575 signatures, and a sum cannot say that a
	// signature was misread: `SURFOBJ *pso` was parsed as a `SURFOBJ` by value
	// and counted under the largest refusal in the table.
	if *sig != "" {
		for _, f := range fns {
			if f.name != *sig {
				continue
			}
			fmt.Printf("%s  [%s]\n", f.name, f.hdr)
			fmt.Printf("  result %-28q shape %v\n", f.result,
				shapeName(classify(f.result, structs, handles, callbacks, alias)))
			for i, p := range f.params {
				fmt.Printf("  arg %-2d type %-24q name %-16q sal %-22q shape %v\n",
					i, p.typ, p.name, p.sal,
					shapeName(classify(p.typ, structs, handles, callbacks, alias)))
			}
			return
		}
		fmt.Fprintf(os.Stderr, "%s: not among the %d entry points\n", *sig, len(fns))
		os.Exit(1)
	}

	var declarable, callable, usable int
	byReason := map[reason]int{}
	byGap := map[reason]int{}
	// Struct by value is the largest remaining refusal, and the roadmap
	// question is not how many but WHICH: an aggregate of 1, 2, 4 or 8 bytes
	// travels in a register and everything else travels by reference, so the
	// bucket splits on a size the host knows. `argOf`/`resOf` keep the position
	// because the ABI treats them differently and so does our emitter: an
	// argument by reference is a caller-built copy, a result over 8 bytes is a
	// hidden pointer in RCX that nothing here emits.
	argOf := map[string]int{}
	resOf := map[string]int{}
	structFns := map[string][]string{}
	var callableNames []string
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
					n := structName(p.typ, structs, handles, callbacks, alias)
					argOf[n]++
					structFns[n] = append(structFns[n], f.name)
				case shFloat:
					why = rFloat
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
				n := structName(f.result, structs, handles, callbacks, alias)
				resOf[n]++
				structFns[n] = append(structFns[n], f.name)
			case shFloat:
				why = rFloat
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
		callableNames = append(callableNames, f.name)
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

	if *sizes != "" {
		var ns []string
		seenN := map[string]bool{}
		for _, m := range []map[string]int{argOf, resOf} {
			for n := range m {
				if !seenN[n] {
					seenN[n], ns = true, append(ns, n)
				}
			}
		}
		sort.Strings(ns)
		hdrs := map[string]string{}
		for _, n := range ns {
			hdrs[n] = structHdr[n]
		}
		if err := writeSizes(*sizes, filepath.Base(dir), ns, hdrs); err != nil {
			fmt.Fprintln(os.Stderr, "sizes:", err)
			os.Exit(1)
		}
		return
	}

	if *checkEnums {
		var ns []string
		for n := range enumUsed {
			ns = append(ns, n)
		}
		sort.Strings(ns)
		hdrs := map[string]string{}
		for _, n := range ns {
			hdrs[n] = enumHdr[n]
		}
		if err := verifyEnums(ns, hdrs); err != nil {
			fmt.Fprintln(os.Stderr, "check-enums:", err)
			os.Exit(1)
		}
		return
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

	if oddDeclarators > 0 {
		fmt.Println("PARSER RESIDUE: parameter declarators not fully read:", oddDeclarators)
		fmt.Println("  (an inline function pointer, int (*cb)(void), whose name is in parentheses)")
		fmt.Println()
	}
	fmt.Printf("ENUMS: %d enum type names; %d of them the struct table had been reading as a struct\n",
		len(enumBase), len(enumWasStruct))
	fmt.Printf("  -- an enum is a C `int` under Win64, one register: refusing it was this tool, not the format.\n\n")
	// EVERY ROW IS A FIRST REFUSAL, and saying so is not pedantry: the walk
	// stops at the first thing it cannot declare, so clearing one refusal
	// makes another RISE. Fixing the pointer declarator took struct by value
	// from 945 to 256 and pushed function pointer from 329 to 337
	// (structval-2026-09-12) — nothing got worse.
	fmt.Println("WHY THE REST CANNOT BE DECLARED (each name counted at its FIRST refusal)")
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
		fmt.Printf("  %-24s %6d  %5.1f%%   [%s]\n", x.r, x.n, pct(x.n, len(fns)), blame(x.r))
	}

	reportStructs(argOf, resOf, structFns)
	lk, lkErr := loadLinkage(dir)
	reportLinking(lk, lkErr, callableNames, len(fns))

	if len(unres) > 0 {
		type uc struct {
			t string
			n int
		}
		var us []uc
		for t, n := range unres {
			us = append(us, uc{t, n})
		}
		sort.Slice(us, func(i, j int) bool { return byCount(us[i].n, us[j].n, us[i].t, us[j].t) })
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
	sort.Slice(ks, func(i, j int) bool { return byCount(salCount[ks[i]], salCount[ks[j]], ks[i], ks[j]) })
	total := 0
	for _, k := range ks {
		total += salCount[k]
	}
	for _, k := range ks {
		fmt.Printf("  %-22s %7d  %5.1f%%\n", k, salCount[k], pct(salCount[k], total))
	}

	if *emitDir != "" {
		if err := emit(*emitDir, decls, structs, handles, callbacks, alias, lk); err != nil {
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
	reAlias   = regexp.MustCompile(`typedef\s+([A-Za-z_][A-Za-z0-9_ ]*?)\s+(\**[A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*\**[A-Za-z_][A-Za-z0-9_]*)*)\s*;`)
	reStruct  = regexp.MustCompile(`\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:,[^;]*)?;`)
	reCbType  = regexp.MustCompile(`typedef[^;]*\(\s*(?:CALLBACK|WINAPI|APIENTRY|NTAPI|__stdcall|\*)\s*\*?\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	reEnumTD  = regexp.MustCompile(`typedef\s+enum\s+(?:class\s+)?([A-Za-z_]\w*)?\s*(?::\s*([A-Za-z_]\w*)\s*)?\{[^}]*\}\s*([A-Za-z_]\w*)((?:\s*,\s*\**\s*[A-Za-z_]\w*)*)\s*;`)
	reEnumTag = regexp.MustCompile(`\benum\s+(?:class\s+)?([A-Za-z_]\w*)\s*(?::\s*([A-Za-z_]\w*)\s*)?\{`)
)

// collectTypes builds the tables `classify` consults. Approximate on purpose:
// what it cannot resolve is counted as `unresolved typedef` rather than guessed,
// so the residue is visible instead of being folded into a better-looking number.
func collectTypes(hdr, src string, structs, handles, callbacks map[string]bool, alias map[string]string) {
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
	// AN ENUM IS A C `int` -- or its declared underlying type -- and one
	// register. `reStruct` takes the name after ANY closing brace, so
	// `typedef enum _X { A, B } X;` put X in the struct table, and every
	// function taking or returning one was refused as "struct by value".
	// The SDK declares 5,178 `typedef enum`s.
	isEnum := map[string]bool{}
	for _, m := range reEnumTag.FindAllStringSubmatch(src, -1) {
		enumBase[m[1]] = baseOr(m[2])
	}
	for _, m := range reEnumTD.FindAllStringSubmatch(src, -1) {
		b := baseOr(m[2])
		if m[1] != "" {
			enumBase[m[1]] = b
		}
		enumBase[m[3]], isEnum[m[3]] = b, true
		for _, nm := range strings.Split(m[4], ",") {
			nm = strings.TrimSpace(nm)
			ptr := strings.HasPrefix(nm, "*")
			nm = strings.TrimSpace(strings.TrimLeft(nm, "* "))
			if nm == "" {
				continue
			}
			if ptr {
				if alias[nm] == "" {
					alias[nm] = m[3] + "*"
				}
				continue
			}
			enumBase[nm], isEnum[nm] = b, true
		}
	}
	// ENUM OR STRUCT IS DECIDED BY THE BRACE, NOT BY A REGEX OVER THE TYPEDEF.
	//
	// `reStruct` finds `} NAME;` and says nothing about what that brace OPENED.
	// win32enum-2026-09-11 answered it with `reEnumTD`, a pattern over
	// `typedef enum TAG [: BASE] { … } NAME;` — and the SDK writes three
	// spellings that pattern cannot see:
	//
	//	typedef _Return_type_success_(return == 0) enum _JsErrorCode : unsigned int
	//	                       junk between `typedef` and `enum`, AND a base of
	//	                       two tokens where the pattern allows one
	//	                       (`JsErrorCode`, 99 entry points)
	//	enum tagX { … } X;     no `typedef` at all — legal C++, and the SDK
	//	                       uses it (`EapHostPeerMethodResultReason`)
	//
	// So the decision moves to where the fact is: match the brace backwards and
	// read the keyword in front of it. That answers every spelling at once
	// rather than one more of them, which is the difference between fixing a
	// bug and fixing its instance.
	for _, ix := range reStruct.FindAllStringSubmatchIndex(src, -1) {
		nm := src[ix[2]:ix[3]]
		kind, base := kindOfBody(src, ix[0])
		if kind == "enum" {
			if _, had := enumBase[nm]; !had {
				enumBase[nm] = baseOr(base)
			}
			if enumHdr[nm] == "" {
				enumHdr[nm] = hdr
			}
			if structs[nm] {
				// A header seen earlier read it as a struct; the brace is the
				// authority, so take the name back.
				delete(structs, nm)
			}
			enumWasStruct[nm] = true
			continue
		}
		if _, isEnum := enumBase[nm]; isEnum {
			// Declared an enum somewhere and re-listed here; enum wins, since
			// an enum name and a struct name cannot both be right.
			enumWasStruct[nm] = true
			continue
		}
		structs[nm] = true
		if structHdr[nm] == "" {
			structHdr[nm] = hdr
		}
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
	// NO ARITY CEILING. There was one at fourteen, "the widest entry point in
	// the SDK" -- true until enums stopped being refused as structs, when the
	// widest became SEVENTEEN and six names were counted declarable and never
	// emitted. The limit was already stale: emit/asm.go's asmShadowFor sizes
	// each procedure's reserved home from the widest prim that procedure
	// calls, with no cap, and the %{10} hole names any operand. A name
	// counted declarable and never emitted is a claim, and the two counts
	// agree again.
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

func emit(dir string, fns []fn, structs, handles, callbacks map[string]bool, alias map[string]string, lk *linkage) error {
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
			// A RESULT IS READ AT ITS C WIDTH AND SIGNEDNESS -- see resultMove.
			if kind == "expr" && res == "int" {
				tmpl = strings.TrimSuffix(tmpl, "mov %r, rax") + resultMove(f.result, alias)
			}
			if kind == "stmt" {
				// A statement's template must not write `%r`: the emitter takes
				// the value from argument 0 and allocates no result register,
				// so `%r` would have nothing to expand to.
				tmpl = strings.TrimSuffix(tmpl, "\nmov %r, rax")
			}
			// THE LIBRARY THAT RESOLVES IT, when exactly one can be named
			// honestly (linkage.resolve). A declaration with none still loads
			// and still type-checks; what it cannot do is link, and that is a
			// refusal at the linker rather than a binding to the wrong DLL.
			libAttr := ""
			if lib, _ := lk.resolve(f.name); lib != "" {
				libAttr = fmt.Sprintf(" (lib %q)", lib)
			}
			fmt.Fprintf(&b, "    (sig %s %s %s (host %s %q (import %q)%s))\n",
				f.name, strings.Replace(al, "(none)", "()", 1), res, kind, tmpl, f.name, libAttr)
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

// ---------------------------------------------------------------------------
// STRUCT BY VALUE, BY ABI CLASS
//
// 945 names, 8.2%, the largest remaining refusal once enums stopped being read
// as structs (win32enum-2026-09-11). The number was the whole of what was
// known about it; this says which aggregates they are and what the calling
// convention does with each, because those are two different pieces of work:
//
//	1, 2, 4 or 8 bytes   passed in a register, returned in RAX. One word — which
//	                     is what this target's convention already carries. What
//	                     is missing is a way to BUILD the word.
//
//	anything else        passed as a POINTER to a caller-allocated copy, and
//	                     returned through a hidden pointer the caller passes in
//	                     RCX and the callee hands back in RAX. The argument case
//	                     needs the heterogeneous product's layout (products.md
//	                     §7); the result case needs a convention no backend here
//	                     emits.
//
// The sizes come from MSVC (`-sizes`), never from a layout computed here.

// sizeTable is the measured size of each aggregate, read from the file `-sizes`
// writes. Absent means unmeasured, and unmeasured is reported as its own row
// rather than folded into either class — the residue stays visible, which is
// this tool's rule for its own vocabulary.
var sizeTable = map[string]int{}

// regClass is the Win64 rule, and it is exact rather than a threshold: a
// 3-byte aggregate is NOT register-passed even though it fits in a register.
func regClass(sz int) bool { return sz == 1 || sz == 2 || sz == 4 || sz == 8 }

func reportStructs(argOf, resOf map[string]int, fns map[string][]string) {
	all := map[string]bool{}
	for n := range argOf {
		all[n] = true
	}
	for n := range resOf {
		all[n] = true
	}
	type row struct {
		name     string
		arg, res int
		size     int
		known    bool
	}
	var rows []row
	for n := range all {
		sz, known := sizeTable[n]
		rows = append(rows, row{n, argOf[n], resOf[n], sz, known})
	}
	sort.Slice(rows, func(i, j int) bool {
		return byCount(rows[i].arg+rows[i].res, rows[j].arg+rows[j].res, rows[i].name, rows[j].name)
	})

	var regA, regR, memA, memR, unkA, unkR int
	var regT, memT, unkT int
	for _, r := range rows {
		switch {
		case !r.known:
			unkA, unkR, unkT = unkA+r.arg, unkR+r.res, unkT+1
		case regClass(r.size):
			regA, regR, regT = regA+r.arg, regR+r.res, regT+1
		default:
			memA, memR, memT = memA+r.arg, memR+r.res, memT+1
		}
	}

	fmt.Printf("\nSTRUCT BY VALUE: %d aggregate types across %d refusals\n",
		len(rows), regA+regR+memA+memR+unkA+unkR)
	fmt.Println("  (a refusal is counted once per position; a function refused for two")
	fmt.Println("   struct arguments stops at the first, so these are first refusals)")
	fmt.Printf("  %-34s %6s %6s %6s\n", "ABI class", "types", "args", "results")
	fmt.Printf("  %-34s %6d %6d %6d\n", "1/2/4/8 bytes — one register", regT, regA, regR)
	fmt.Printf("  %-34s %6d %6d %6d\n", "other — by reference / RCX", memT, memA, memR)
	if unkT > 0 {
		fmt.Printf("  %-34s %6d %6d %6d\n", "size not measured", unkT, unkA, unkR)
	}

	fmt.Println("\n  THE AGGREGATES, by how many entry points they block")
	for i, r := range rows {
		if i >= 30 {
			break
		}
		sz := "     ?"
		cls := "unmeasured"
		if r.known {
			sz = fmt.Sprintf("%6d", r.size)
			cls = "by reference"
			if regClass(r.size) {
				cls = "one register"
			}
		}
		fmt.Printf("    %-34s %s  %-13s arg %4d  res %4d  [%s]\n",
			r.name, sz, cls, r.arg, r.res, structHdr[r.name])
	}
	fmt.Printf("    … %d distinct aggregate types in total\n", len(rows))
}

func shapeName(s shape) string {
	switch s {
	case shWord:
		return "word"
	case shString:
		return "string"
	case shPtr:
		return "pointer"
	case shStruct:
		return "STRUCT BY VALUE"
	case shFunc:
		return "function pointer"
	case shFloat:
		return "float"
	case shVoid:
		return "void"
	}
	return "unresolved"
}

// ---------------------------------------------------------------------------
// WHICH LIBRARY WOULD THE LINK LINE NEED?
//
// win32enum-2026-09-11 named this and did not measure it: `build.bat` links
// kernel32, msvcrt, ucrt and vcruntime and nothing else, so a generated
// declaration for user32 or gdi32 is counted CALLABLE and cannot link. That
// result said the survey "cannot map a header to its DLL without reading the
// SDK's import libraries". It can — and it needs no toolchain, because a COFF
// archive carries its own symbol table:
//
//	8 bytes "!<arch>\n", then members with a 60-byte header, and the FIRST
//	member is named "/" and holds a big-endian count, that many offsets, then
//	that many NUL-terminated symbol names.
//
// So the mapping is read off the same SDK the headers come from, which makes it
// a measurement rather than a list somebody maintains.

// baseLink is the libraries the windows target links into EVERY program, read
// from targets/windows/windows.oro rather than repeated here. Until
// linkline-2026-09-13 the list was a constant in emit/target.go and a copy of it
// in this file: a host fact written twice is two facts that can disagree.
func baseLink() (map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join("targets", "windows", "windows.oro"))
	if err != nil {
		return nil, err
	}
	var code []string
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(ln), ";") {
			code = append(code, ln)
		}
	}
	m := reLink.FindStringSubmatch(strings.Join(code, "\n"))
	if m == nil {
		return nil, fmt.Errorf("targets/windows/windows.oro declares no (link …)")
	}
	base := map[string]bool{}
	for _, q := range reQuoted.FindAllStringSubmatch(m[1], -1) {
		base[strings.ToLower(q[1])] = true
	}
	return base, nil
}

var (
	reLink   = regexp.MustCompile(`\(link((?:\s+"[^"]*")+)\s*\)`)
	reQuoted = regexp.MustCompile(`"([^"]*)"`)
)

// archSymbols returns every symbol name an import library exports.
func archSymbols(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 68 || string(b[:8]) != "!<arch>\n" {
		return nil, fmt.Errorf("%s: not a COFF archive", filepath.Base(path))
	}
	if nm := strings.TrimSpace(string(b[8:24])); nm != "/" {
		return nil, fmt.Errorf("%s: first member is %q, not the linker member", filepath.Base(path), nm)
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(b[56:66])))
	if err != nil || 68+size > len(b) {
		return nil, fmt.Errorf("%s: bad linker member size", filepath.Base(path))
	}
	c := b[68 : 68+size]
	if len(c) < 4 {
		return nil, fmt.Errorf("%s: empty linker member", filepath.Base(path))
	}
	n := int(binary.BigEndian.Uint32(c[:4]))
	if 4+4*n > len(c) {
		return nil, fmt.Errorf("%s: symbol count %d does not fit", filepath.Base(path), n)
	}
	names := c[4+4*n:]
	out := make([]string, 0, n)
	for s := 0; s < len(names) && len(out) < n; {
		e := s
		for e < len(names) && names[e] != 0 {
			e++
		}
		out = append(out, string(names[s:e]))
		s = e + 1
	}
	return out, nil
}

// libIndex reads every import library in the SDK and returns three things the
// host states about itself:
//
//	idx    symbol -> the libraries whose linker member lists it
//	bind   symbol -> for each library, the DLL that library binds it to
//	dlls   library -> how many DLLs it binds to in all
//
// `bind` is the one that decides anything. A Windows import library is an
// archive of SHORT IMPORT objects, each an IMPORT_OBJECT_HEADER (Sig1 0x0000,
// Sig2 0xFFFF) followed by the symbol name and the DLL name, NUL-terminated —
// so which DLL a name resolves to through which library is not inferred, it is
// read. linkline-2026-09-13 first decided by counting a library's
// `__IMPORT_DESCRIPTOR_`s, called the split bimodal, and was wrong twice: 20
// libraries sit between 2 and 428 DLLs, and an "umbrella" like `onecore`
// binds `MulDiv` to kernel32.dll exactly as kernel32.lib does.
func libIndex(sdkInclude string) (map[string][]string, map[string][]binding, map[string]int, int, error) {
	root := filepath.Dir(filepath.Dir(sdkInclude)) // …/10/Include/<ver> -> …/10
	ver := filepath.Base(sdkInclude)
	idx := map[string][]string{}
	bind := map[string][]binding{}
	dlls := map[string]int{}
	nlib := 0
	for _, sub := range []string{"um", "ucrt"} {
		dir := filepath.Join(root, "Lib", ver, sub, "x64")
		libs, _ := filepath.Glob(filepath.Join(dir, "*"))
		sort.Strings(libs)
		for _, l := range libs {
			if e := strings.ToLower(filepath.Ext(l)); e != ".lib" {
				continue
			}
			syms, err := archSymbols(l)
			if err != nil {
				continue
			}
			nlib++
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(l), filepath.Ext(l)))
			seen := map[string]bool{}
			for _, s := range syms {
				if strings.HasPrefix(s, "__IMPORT_DESCRIPTOR_") {
					dlls[base]++
					continue
				}
				// __imp_X is the same entry point; the plain name is enough.
				if strings.HasPrefix(s, "__imp_") || strings.HasPrefix(s, "__NULL_IMPORT") ||
					strings.HasSuffix(s, "_NULL_THUNK_DATA") || seen[s] {
					continue
				}
				seen[s] = true
				idx[s] = append(idx[s], base)
			}
			shortImports(l, func(sym, dll string) {
				bind[sym] = append(bind[sym], binding{base, strings.ToLower(dll)})
			})
		}
	}
	if nlib == 0 {
		return nil, nil, nil, 0, fmt.Errorf("no import libraries under %s", filepath.Join(root, "Lib", ver))
	}
	return idx, bind, dlls, nlib, nil
}

// binding is one library's statement of which DLL a symbol resolves to.
type binding struct{ lib, dll string }

// shortImports walks every member of an archive and reports each short import
// object's symbol and DLL. A member is 60 bytes of header, then its size in
// bytes, padded to an even offset.
func shortImports(path string, fn func(sym, dll string)) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 8 || string(b[:8]) != "!<arch>\n" {
		return
	}
	for p := 8; p+60 <= len(b); {
		size, err := strconv.Atoi(strings.TrimSpace(string(b[p+48 : p+58])))
		if err != nil || p+60+size > len(b) {
			return
		}
		d := b[p+60 : p+60+size]
		if len(d) >= 20 && binary.LittleEndian.Uint16(d[0:]) == 0 && binary.LittleEndian.Uint16(d[2:]) == 0xFFFF {
			if n := int(binary.LittleEndian.Uint32(d[12:])); 20+n <= len(d) {
				if parts := strings.SplitN(string(d[20:20+n]), "\x00", 3); len(parts) >= 2 && parts[0] != "" {
					fn(parts[0], parts[1])
				}
			}
		}
		p += 60 + size
		if p%2 == 1 {
			p++
		}
	}
}

// apiSet says whether a DLL name is an API SET contract rather than a DLL —
// `api-ms-win-core-…` and `ext-ms-win-…`, the host's own documented prefixes. A
// binding to one is resolved by the loader to whatever implements the contract
// on the machine it runs on.
func apiSet(dll string) bool {
	return strings.HasPrefix(dll, "api-ms-win-") || strings.HasPrefix(dll, "ext-ms-win-")
}

// linkage answers, for a generated declaration, which import library it may
// name. It is nil when the SDK's libraries could not be read, and then no
// declaration names one.
type linkage struct {
	idx  map[string][]string
	bind map[string][]binding
	dlls map[string]int
	base map[string]bool
	nlib int
}

func loadLinkage(sdkInclude string) (*linkage, error) {
	base, err := baseLink()
	if err != nil {
		return nil, err
	}
	idx, bind, dlls, nlib, err := libIndex(sdkInclude)
	if err != nil {
		return nil, err
	}
	return &linkage{idx, bind, dlls, base, nlib}, nil
}

// resolve says which library a declaration of `name` may carry as its `lib`,
// and why when it may carry none. The question is which DLL the name MEANS, and
// the libraries answer it:
//
//	base      a library the target links into every program exports it: the
//	          program already imports from that library, so naming it binds
//	          nothing new — which is also how `kernel32` settles against
//	          `vertdll` without anyone choosing.
//	lib       every library that binds it names ONE real DLL. The library
//	          named is the one binding the fewest DLLs, so kernel32.lib over
//	          onecore.lib and avifil32.lib over vfw32.lib — the same DLL at run
//	          time either way, and the narrower library on the line.
//	tied      libraries bind it to DIFFERENT DLLs — `AbortPrinter` to
//	          winspool.drv and to spoolss.dll, which is the client API and the
//	          spooler's own side. The headers do not say which is meant, so this
//	          names neither and the linker refuses the call.
//	apiset    it is reachable only through an API set contract.
//	unbound   a library lists it and no short import object binds it — a
//	          static library, code rather than a DLL call.
//	none      no library in the SDK lists it at all.
func (lk *linkage) resolve(name string) (lib, why string) {
	if lk == nil {
		return "", "unmeasured"
	}
	libs := append([]string(nil), lk.idx[name]...)
	sort.Strings(libs)
	for _, l := range libs {
		if lk.base[l] {
			return l, "base"
		}
	}
	real := map[string][]string{}
	viaAPI := false
	for _, x := range lk.bind[name] {
		if apiSet(x.dll) {
			viaAPI = true
			continue
		}
		real[x.dll] = append(real[x.dll], x.lib)
	}
	switch {
	case len(real) == 1:
		for _, cands := range real {
			sort.Slice(cands, func(i, j int) bool {
				if lk.dlls[cands[i]] != lk.dlls[cands[j]] {
					return lk.dlls[cands[i]] < lk.dlls[cands[j]]
				}
				return cands[i] < cands[j]
			})
			return cands[0], "lib"
		}
	case len(real) > 1:
		var ds []string
		for d := range real {
			ds = append(ds, d)
		}
		sort.Strings(ds)
		return "", "tied:" + strings.Join(ds, " + ")
	case viaAPI:
		return "", "apiset"
	case len(libs) > 0:
		return "", "unbound"
	}
	return "", "none"
}

// reportLinking says how many callable names the emitted link line can actually
// reach. A name this survey counts and a program cannot link is a claim, which
// is the same sentence win32-2026-09-08 wrote about a name counted declarable
// and never emitted.
func reportLinking(lk *linkage, lkErr error, callable []string, total int) {
	if lkErr != nil {
		fmt.Printf("\nLINKING: not measured — %v\n", lkErr)
		return
	}
	var base, withLib, apiset, unbound, none, tiedN int
	reach := map[string]int{}
	ties := map[string]int{}
	for _, n := range callable {
		lib, why := lk.resolve(n)
		switch {
		case why == "base":
			base++
		case why == "lib":
			withLib++
			reach[lib]++
		case strings.HasPrefix(why, "tied:"):
			tiedN++
			ties[strings.TrimPrefix(why, "tied:")]++
		case why == "apiset":
			apiset++
		case why == "unbound":
			unbound++
		default:
			none++
		}
	}
	links := base + withLib
	fmt.Printf("\nLINKING, of the %d callable (%d import libraries read; the target links %d into every program)\n",
		len(callable), lk.nlib, len(lk.base))
	fmt.Printf("  by a library the target always links:   %5d  %5.1f%%\n", base, pct(base, len(callable)))
	fmt.Printf("  bound to ONE DLL by every library:      %5d  %5.1f%%   [declared (lib …); joins the line when used]\n",
		withLib, pct(withLib, len(callable)))
	fmt.Printf("  refused: bound to two or more DLLs:     %5d  %5.1f%%   [the headers do not say which is meant]\n",
		tiedN, pct(tiedN, len(callable)))
	fmt.Printf("  refused: only through an API set:       %5d  %5.1f%%\n", apiset, pct(apiset, len(callable)))
	fmt.Printf("  refused: listed and bound to no DLL:    %5d  %5.1f%%   [a static library]\n",
		unbound, pct(unbound, len(callable)))
	fmt.Printf("  in no import library at all:    %5d  %5.1f%%   [a header with no static import]\n",
		none, pct(none, len(callable)))
	fmt.Printf("  CALLABLE AND IT LINKS:                  %5d  %5.1f%% of the flat API\n", links, pct(links, total))

	type lc struct {
		lib string
		n   int
	}
	ranked := func(m map[string]int) []lc {
		var ls []lc
		for l, n := range m {
			ls = append(ls, lc{l, n})
		}
		sort.Slice(ls, func(i, j int) bool { return byCount(ls[i].n, ls[j].n, ls[i].lib, ls[j].lib) })
		return ls
	}
	fmt.Println("\n  THE LIBRARIES A DECLARATION NOW NAMES, by callable names in each")
	ls := ranked(reach)
	for i, x := range ls {
		if i >= 15 {
			break
		}
		fmt.Printf("    %-28s %5d\n", x.lib, x.n)
	}
	fmt.Printf("    … %d distinct libraries in total\n", len(ls))
	if len(ties) > 0 {
		fmt.Println("\n  THE TIES, by callable names: one name, several DLLs")
		ts := ranked(ties)
		for i, x := range ts {
			if i >= 12 {
				break
			}
			fmt.Printf("    %-44s %5d\n", x.lib, x.n)
		}
		fmt.Printf("    … %d distinct tie sets in total\n", len(ts))
	}
}

// kindOfBody says what the brace closing at `end` opened, and with what
// underlying type. It scans back to the matching `{`, then back again to the
// nearest `;`, `{` or `}` — which cannot be crossed, so nothing from a
// neighbouring declaration or a nested member leaks in — and reads the last
// aggregate keyword in that window.
//
// `end` is the index of the closing `}`.
func kindOfBody(src string, end int) (kind, base string) {
	depth := 0
	open := -1
	for j := end; j >= 0; j-- {
		switch src[j] {
		case '}':
			depth++
		case '{':
			depth--
			if depth == 0 {
				open = j
			}
		}
		if open >= 0 {
			break
		}
	}
	if open < 0 {
		return "", ""
	}
	start := 0
	for j := open - 1; j >= 0; j-- {
		if c := src[j]; c == ';' || c == '{' || c == '}' {
			start = j + 1
			break
		}
	}
	head := src[start:open]
	// The LAST keyword decides: `typedef struct { … }` inside nothing, but also
	// `union … enum` can never both appear in one window without a brace.
	at, kw := -1, ""
	for _, k := range []string{"enum", "struct", "union"} {
		if i := lastWord(head, k); i > at {
			at, kw = i, k
		}
	}
	if kw == "" {
		return "", ""
	}
	// What follows is `TAG` and optionally `: BASE`, and BASE may be several
	// tokens (`unsigned int`).
	rest := head[at+len(kw):]
	if i := strings.Index(rest, ":"); i >= 0 {
		base = strings.TrimSpace(rest[i+1:])
	}
	return kw, base
}

// lastWord finds the last occurrence of w in s as a whole word, or -1.
func lastWord(s, w string) int {
	for i := len(s) - len(w); i >= 0; i-- {
		if s[i:i+len(w)] != w {
			continue
		}
		if i > 0 && isIdentByte(s[i-1]) {
			continue
		}
		if j := i + len(w); j < len(s) && isIdentByte(s[j]) {
			continue
		}
		return i
	}
	return -1
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// ---------------------------------------------------------------------------
// ASKING THE HOST FOR A LAYOUT
//
// `-sizes FILE` writes the size of every aggregate that blocks an entry point,
// as MSVC reports it, and the survey reads that file to classify. Two reasons
// it is a file rather than a computation:
//
//	IT IS NOT OURS TO COMPUTE. Win64 layout is natural alignment plus
//	`#pragma pack` plus bitfield packing plus anonymous unions, and a size
//	wrong by one byte moves an aggregate between ABI classes and so moves the
//	ROADMAP. MSVC already knows. This is coercion-2026-09-09's rule — the host
//	decides, we candidate-generate and host-filter.
//
//	THE SURVEY MUST STAY A FUNCTION OF ITS INPUT. tooling-2026-09-11 runs it
//	twice and compares; invoking a C compiler from the default path would make
//	the report depend on a toolchain as well as on the headers.

const sizeTableFile = "gauntlet/stdlib/win32-sizes.txt"

// readSizes fills sizeTable. A missing file is not an error: the class column
// then reads "size not measured", which is the honest answer and is visible.
func readSizes(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		f := strings.Fields(ln)
		if len(f) != 2 {
			continue
		}
		if n, err := strconv.Atoi(f[1]); err == nil {
			sizeTable[f[0]] = n
		}
	}
}

// writeSizes generates a C probe for these aggregates, compiles and runs it
// under MSVC, and writes what the host said.
//
// CANDIDATE-GENERATE AND HOST-FILTER. Not every SDK header compiles when
// included on its own, and not every aggregate name is spellable in a C
// translation unit (some are C++-only, some are behind a version macro). So the
// probe is compiled, whatever cl refuses is dropped, and the rest is asked
// again — bounded, and what never resolves stays unmeasured rather than being
// guessed at.
func writeSizes(path, sdkVer string, names []string, hdrs map[string]string) error {
	tmp, err := os.MkdirTemp("", "win32sizes")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	got := map[string]int{}

	for round := 0; round < 8 && len(want) > 0; round++ {
		var ns []string
		incs := map[string]bool{}
		for n := range want {
			ns = append(ns, n)
			if h := hdrs[n]; h != "" {
				incs[h] = true
			}
		}
		sort.Strings(ns)
		var ih []string
		for h := range incs {
			ih = append(ih, h)
		}
		sort.Strings(ih)

		var src strings.Builder
		src.WriteString("#include <windows.h>\n#include <stdio.h>\n")
		for _, h := range ih {
			fmt.Fprintf(&src, "#include <%s>\n", h)
		}
		src.WriteString("int main(void){\n")
		for _, n := range ns {
			fmt.Fprintf(&src, "  printf(%q, (size_t)sizeof(%s));\n", n+" %zu\n", n)
		}
		src.WriteString("  return 0;\n}\n")
		if err := os.WriteFile(filepath.Join(tmp, "probe.c"), []byte(src.String()), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmp, "run.bat"), []byte(probeBat), 0o644); err != nil {
			return err
		}

		out, runErr := runBat(tmp)
		before := len(got)
		for _, ln := range strings.Split(out, "\n") {
			f := strings.Fields(strings.TrimSpace(ln))
			if len(f) == 2 && want[f[0]] {
				if v, e := strconv.Atoi(f[1]); e == nil {
					got[f[0]] = v
					delete(want, f[0])
				}
			}
		}
		// A round that measured nothing is worth showing: the host refused the
		// whole probe, and its reason is the only thing that says why.
		if len(got) == before {
			fmt.Fprintf(os.Stderr, "sizes: round %d measured nothing (err %v); cl said:\n%s\n",
				round, runErr, out)
		}
		if runErr == nil && len(want) == 0 {
			break
		}
		// Drop whatever cl named. An error line is
		// `probe.c(12): error C2065: 'X': undeclared identifier` for a type,
		// or names a header that will not compile.
		dropped := 0
		for _, ln := range strings.Split(out, "\n") {
			if !strings.Contains(ln, "error") && !strings.Contains(ln, "fatal error") {
				continue
			}
			for n := range want {
				if strings.Contains(ln, "'"+n+"'") || strings.Contains(ln, "sizeof("+n+")") {
					delete(want, n)
					dropped++
				}
			}
			for _, h := range ih {
				if !strings.Contains(ln, h) {
					continue
				}
				// A header that will not compile takes its own includes out,
				// and its types are retried through windows.h alone.
				for n, hh := range hdrs {
					if hh == h && want[n] {
						hdrs[n] = ""
						dropped++
					}
				}
			}
		}
		if dropped == 0 {
			break
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Win64 sizeof, as MSVC reports it, for every aggregate that blocks an\n")
	fmt.Fprintf(&b, "# entry point. Regenerate with:\n")
	fmt.Fprintf(&b, "#   go run gauntlet/stdlib/win32.go -sizes %s\n", sizeTableFile)
	fmt.Fprintf(&b, "# Windows SDK %s. A size is the HOST's answer, never one computed here.\n", sdkVer)
	var ks []string
	for k := range got {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(&b, "%s %d\n", k, got[k])
	}
	if len(want) > 0 {
		var un []string
		for n := range want {
			un = append(un, n)
		}
		sort.Strings(un)
		fmt.Fprintf(&b, "# not spellable in a C translation unit, so unmeasured: %s\n", strings.Join(un, " "))
	}
	fmt.Printf("sizes: %d measured, %d refused by the host\n", len(got), len(want))
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// probeBat finds MSVC the way emit/target.go's asmBuildBat does, because that
// is the discovery this project already relies on.
const probeBat = `@echo off
setlocal enabledelayedexpansion
set "VCV="
for %%p in ("%ProgramFiles%\Microsoft Visual Studio" "%ProgramFiles(x86)%\Microsoft Visual Studio") do (
  for /d %%v in ("%%~p\*") do (
    for /d %%e in ("%%~v\*") do (
      if exist "%%~e\VC\Auxiliary\Build\vcvars64.bat" set "VCV=%%~e\VC\Auxiliary\Build\vcvars64.bat"
    )
  )
)
if not defined VCV (echo probe: no MSVC toolchain was found & exit /b 1)
call "!VCV!" >nul || exit /b 1
cl -nologo -W0 probe.c -Feprobe.exe || exit /b 1
.\probe.exe
`

func runBat(dir string) (string, error) {
	// `.\run.bat`, spelled relative on purpose: this machine sets
	// NoDefaultCurrentDirectoryInExePath, so cmd does not search the working
	// directory for a command and a bare `run.bat` is not found.
	cmd := exec.Command("cmd", "/c", `call .\run.bat`)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// enumUsed is every name a declaration was written against BECAUSE this tool
// called it an enum. Only these matter: a misread name that no signature
// mentions costs nothing.
var enumUsed = map[string]bool{}

// enumHdr is where each enum name was declared, so the probe can include it.
var enumHdr = map[string]string{}

// oddDeclarators counts parameter declarators this parser does not fully read.
var oddDeclarators = 0

// verifyEnums asks MSVC whether each of those names really is an integer type.
// `T v = (T)0; return (int)v;` compiles for an enum and for an integer typedef,
// and fails for a struct or a union — so the host decides, and what it refuses
// is printed rather than counted.
func verifyEnums(names []string, hdrs map[string]string) error {
	tmp, err := os.MkdirTemp("", "win32enums")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// A CONTROL, because a check that cannot fail proves nothing: `RECT` is an
	// aggregate, so `RECT v = (RECT)0` must be REFUSED. If the host accepts it
	// the probe is not testing what it claims and says so.
	const control = "RECT"
	names = append(append([]string{}, names...), control)
	hdrs[control] = ""
	controlRefused := false

	// One translation unit per batch, and a batch is small enough that a
	// refusal names the type rather than sinking the run. cl reports every
	// error in a file, so one compile settles a whole batch.
	const batch = 400
	bad, tested := []string{}, 0
	for i := 0; i < len(names); i += batch {
		j := min(i+batch, len(names))
		part := names[i:j]
		incs := map[string]bool{}
		for _, n := range part {
			if h := hdrs[n]; h != "" {
				incs[h] = true
			}
		}
		var ih []string
		for h := range incs {
			ih = append(ih, h)
		}
		sort.Strings(ih)

		var src strings.Builder
		src.WriteString("#include <windows.h>\n")
		for _, h := range ih {
			fmt.Fprintf(&src, "#include <%s>\n", h)
		}
		for k, n := range part {
			fmt.Fprintf(&src, "int probe%d(void){ %s v = (%s)0; return (int)v; }\n", k, n, n)
		}
		if err := os.WriteFile(filepath.Join(tmp, "probe.c"), []byte(src.String()), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tmp, "run.bat"), []byte(enumBat), 0o644); err != nil {
			return err
		}
		out, _ := runBat(tmp)
		// A type the host does not know at all is not evidence either way —
		// it is behind a version macro or is C++-only — so `undeclared` is
		// separated from a real refusal.
		unknown := 0
		for _, ln := range strings.Split(out, "\n") {
			if !strings.Contains(ln, ": error") && !strings.Contains(ln, ": fatal error") {
				continue
			}
			named := ""
			for _, n := range part {
				if strings.Contains(ln, "'"+n+"'") {
					named = n
					break
				}
			}
			if named == "" {
				continue
			}
			if strings.Contains(ln, "C2065") || strings.Contains(ln, "undeclared") ||
				strings.Contains(ln, "C2061") {
				unknown++
				continue
			}
			if named == control {
				controlRefused = true
				continue
			}
			bad = append(bad, named+": "+strings.TrimSpace(ln))
		}
		tested += len(part) - unknown
	}
	fmt.Printf("ENUM CHECK: %d names a declaration relies on, %d spellable in C\n", len(names)-1, tested-1)
	if !controlRefused {
		fmt.Printf("  BROKEN: the control %s was not refused, so this check is testing nothing\n", control)
		return fmt.Errorf("control %s accepted as an integer type", control)
	}
	fmt.Printf("  the control %s was refused, so a struct read as an enum would be caught\n", control)
	if len(bad) == 0 {
		fmt.Println("  and the host accepts every one of the rest as an integer type")
		return nil
	}
	fmt.Printf("  THE HOST REFUSED %d:\n", len(bad))
	sort.Strings(bad)
	for _, b := range bad {
		fmt.Println("   ", b)
	}
	return nil
}

const enumBat = `@echo off
setlocal enabledelayedexpansion
set "VCV="
for %%p in ("%ProgramFiles%\Microsoft Visual Studio" "%ProgramFiles(x86)%\Microsoft Visual Studio") do (
  for /d %%v in ("%%~p\*") do (
    for /d %%e in ("%%~v\*") do (
      if exist "%%~e\VC\Auxiliary\Build\vcvars64.bat" set "VCV=%%~e\VC\Auxiliary\Build\vcvars64.bat"
    )
  )
)
if not defined VCV (echo probe: no MSVC toolchain was found & exit /b 1)
call "!VCV!" >nul || exit /b 1
cl -nologo -W0 -c probe.c
`
