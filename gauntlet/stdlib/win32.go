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
//	go run gauntlet/stdlib/win32.go -org                      the partitions by header and by DLL
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

	"oroboros/gauntlet/stdlib/jdecl"
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

// cInt is a C integer type's width in bits and whether it is signed, after the
// alias chain and after an enum becomes its base; known is false for a type it
// cannot place, which is held as the full word, right for every pointer, handle
// and 64-bit word. The width is a CORRECTNESS question: under Win64 an integer
// narrower than 64 bits is the low bits of its register, so MulDiv(-7, 6, 2)
// read as the whole of RAX was 4294967275, and every failing HRESULT, whose
// failure IS the negative value, read as a success (signed-result.oro). LLVM
// says it with a type: the call returns i32 and `sext` makes it the integer.
func cInt(t string, alias map[string]string) (w int, signed, known bool) {
	for i := 0; i < 8; i++ {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "_"))
		for _, q := range []string{"CONST ", "const ", "volatile ", "enum ", "struct ", "union "} {
			b = strings.TrimPrefix(b, q)
		}
		b = strings.TrimSpace(b)
		if strings.HasSuffix(b, "*") {
			return 64, true, false
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
			return w, signed, true
		}
		nxt, have := alias[b]
		if !have || nxt == b {
			return 64, true, false
		}
		t = nxt
	}
	return 64, true, false
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
	case "LONGLONG", "INT64", "LONG64", "SHORT64", "INT_PTR", "LONG_PTR", "SSIZE_T", "LRESULT",
		"LPARAM", "__int64", "long long":
		return 64, true, true
	case "ULONGLONG", "DWORDLONG", "UINT64", "ULONG64", "DWORD64", "UINT_PTR", "ULONG_PTR",
		"DWORD_PTR", "SIZE_T", "WPARAM", "size_t":
		return 64, false, true
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
	// how `ExitProcess` is declared in targets/win32/kernel32.oro. Refusing
	// all of them attributed 3.5% of the API to the format when
	// the format could say it — the survey talking about itself for the third
	// time, after methods-at-0% in the Go survey and the typedef residue here.
	rVoidRes   reason = "void result and no argument"
	rOutParam  reason = "out-parameter"   // the result comes back through a pointer
	rStructVal reason = "struct by value" // does not fit one register
	// FLOATING POINT. On MASM the generated template moved arguments into
	// RCX/RDX/R8/R9 and read RAX, and a double travels in XMM, so a float
	// was refused until a template said which register. On LLVM a `double`
	// is a type and its register is LLVM's to choose, so the reason now is
	// the language's: a host's double is not yet a type it holds (numbers.md
	// §6.3, waiting on a program that reads one), and a FLOAT is a rounding
	// onto F(24, …) that no target realizes (ADR 0086).
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
	// conv is the calling convention its macro names (ADR 0094): WINAPIV is
	// __cdecl, and WINAPI, APIENTRY, NTAPI, STDAPICALLTYPE and CALLBACK are
	// __stdcall.
	conv string
}

var (
	// A flat C entry point. The SDK writes the decoration, the result, the
	// calling convention and the name on separate lines, which is what makes
	// this tractable without a C parser.
	// CALLBACK is __stdcall too, and declares exported functions as well as
	// callback types: `LRESULT CALLBACK DefWindowProcA(…);` (tinyretropad-
	// 2026-11-17). A typedef's `(CALLBACK *T)` has no name before its `(`.
	reFn = regexp.MustCompile(
		`(?s)([A-Za-z_][A-Za-z0-9_ *]*?)\s+(WINAPI|APIENTRY|NTAPI|STDAPICALLTYPE|WINAPIV|CALLBACK)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^;{}]*)\)\s*;`)
	// STDAPI expands to EXTERN_C HRESULT STDAPICALLTYPE, so the result is
	// implicit. STDAPI_(t) names it. The shell's, the light-weight utility
	// library's and the property system's macros are the same expansion under
	// their own names (shellapi.h's SHSTDAPI_(HINSTANCE) ShellExecuteA, shlwapi.h's
	// LWSTDAPI, propsys.h's PSSTDAPI): 773 entry points the survey did not see
	// until tinyretropad-2026-11-17.
	reStdapi = regexp.MustCompile(
		`(?s)\b(?:STDAPI|SHSTDAPI|LWSTDAPI|PSSTDAPI|SHFOLDERAPI|SHDOCAPI)(_\(([^)]*)\))?\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^;{}]*)\)\s*;`)
	// a convention the preprocessor chooses before a name, read as the native
	// Windows build's branch: outside /clr:pure, CALLBACK; and with _MAC
	// undefined, the first branch, `#ifndef _MAC LRESULT WINAPI #else LRESULT
	// CALLBACK #endif DefWindowProcA(…)` (WinUser.h)
	reCeePure  = regexp.MustCompile(`#ifdef _M_CEE_PURE\s+WINAPI\s+#else\s+CALLBACK\s+#endif`)
	reMac      = regexp.MustCompile(`#ifndef _MAC\s+([^#]*?)\bWINAPI\s+#else\s+[^#]*?\bCALLBACK\s+#endif`)
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

// THE ARGUMENTS A PROGRAM PASSES SINCE 2026-11-18 (survey-2026-11-19), each the
// ADR that built it. canPass was the model of 2026-09-08: a word, a C string, or
// NULL for a pointer SAL marks optional. A pointer is now passed:
//   - `_Out_ T*`: a result of the declaration, its address the call's `&r`
//     (ADR 0131), T a word or a struct;
//   - `_Inout_ T*`: a cell the callee writes, `x ∈ T inout` (ADR 0048, 0135);
//   - `_In_ T*`: `&x`, a cell's address or a struct's fresh copy (ADR 0130);
//   - a byte region and its count, `_In_reads_bytes_(n)`, `_Out_writes_(n)` over
//     bytes, n a parameter: one segment, `b` or `b[lo:hi]` (ADR 0132).
//
// A struct passes only where its layout is derived and checked against clang
// (ADR 0128), which this survey does not parse: so a struct counts in the lower
// bound when the windows target already declares it as a record, and in the
// upper bound whenever the SDK defines it. A wide string, a typed array, and a
// pointer whose direction SAL does not say, still do not pass.
// model2 is canPass2 over an entry point's arguments and its result: the lower
// bound, structs the target declares, and the upper, every struct.
func model2(f fn, structs, handles, callbacks map[string]bool, alias map[string]string) (bool, bool) {
	if f.variadic {
		return false, false
	}
	switch classify(f.result, structs, handles, callbacks, alias) {
	case shStruct, shFloat, shUnknown, shFunc:
		return false, false // a struct result by value is a hidden pointer nothing emits
	case shVoid:
		if len(f.params) == 0 {
			return false, false // the format's: a stmt hands back its first argument
		}
	}
	lo, up := true, true
	for _, p := range f.params {
		p = unSAL(p)
		sh := classify(p.typ, structs, handles, callbacks, alias)
		// A FUNCTION POINTER IS A λ WRITTEN AT THE CALL (ADR 0143), passable
		// where the callback's own parameters and result are scalars a claim's
		// sets state. Whether the API keeps it past the call, which tier 1
		// excludes, is its documentation's and not the header's, so this
		// counts every function pointer of that shape: an upper bound.
		if sh == shFunc {
			if !callbackPassable(cleanType(p.typ), structs, handles, callbacks, alias) {
				lo, up = false, false
			}
			continue
		}
		if !canPass2(p, sh, f, structs, alias, false) {
			lo = false
		}
		if !canPass2(p, sh, f, structs, alias, true) {
			up = false
		}
	}
	return lo, up
}

// unSAL moves annotations the parser left in a parameter's type into its SAL:
// `__drv_freesMem(Mem) _Frees_ptr_opt_ LPVOID`, `__out_data_source(FILE) LPVOID`.
func unSAL(p param) param {
	fs := strings.Fields(p.typ)
	var keep []string
	for _, w := range fs {
		// `__callback`, an annotation the SDK defines empty, marks a function
		// pointer parameter (PINIT_ONCE_FN's)
		if w == "__callback" || strings.HasPrefix(w, "_") && (strings.HasSuffix(w, "_") || strings.Contains(w, "(")) &&
			!word[strings.TrimLeft(w, "_")] {
			p.sal = strings.TrimSpace(p.sal + " " + w)
			continue
		}
		keep = append(keep, w)
	}
	if len(keep) > 0 {
		p.typ = strings.Join(keep, " ")
	}
	return p
}

func canPass2(p param, sh shape, f fn, structs map[string]bool, alias map[string]string, upper bool) bool {
	// A STRUCT BY VALUE (ADR 0129): the bare set, passed by the triple's ABI,
	// where its layout is derived
	if sh == shStruct {
		n := cleanType(p.typ)
		return upper || handRecords[n] || handRecords[resolveName(n, alias)] || handWords[n]
	}
	// A WIDE STRING IS UTF-16, a codec on the kernel not built (ADR 0091's
	// rest): canPass counted it a C string. LPCTSTR is wide in the UNICODE
	// build this survey reads.
	if sh == shString && wideString[cleanType(p.typ)] {
		return false
	}
	// A C STRING IS AN INPUT: an LPSTR the callee writes, `_Out_writes_(n)`,
	// is a byte region, passed as a segment only with its count a parameter
	// (ADR 0132); canPass counted it a string, which a program cannot write into
	if sh == shString && (strings.HasPrefix(strings.TrimSpace(p.sal), "_Out") || strings.HasPrefix(strings.TrimSpace(p.sal), "_Inout")) &&
		!strings.Contains(p.sal, "_opt_") {
		base := p.sal
		if i := strings.Index(base, "("); i >= 0 {
			base = base[:i]
		}
		if n := salArg(p.sal); segmentSAL(strings.TrimSpace(base)) && n != "" {
			for _, q := range f.params {
				if q.name == n {
					return true
				}
			}
		}
		return false
	}
	if canPass(p, sh) {
		return true
	}
	if sh != shPtr {
		return false
	}
	if strings.Contains(p.sal, "_opt_") || strings.Contains(p.sal, "_Reserved_") {
		return true // NULL, as canPass passes it, its annotation having been in the type
	}
	pt := pointee(p.typ, structs, alias)
	if pt == "" {
		return false
	}
	pw := word[pt] || isWordAlias(pt, alias) || handWords[pt]
	declared := handRecords[pt] || handRecords[resolveName(pt, alias)] || handRecordNaming(pt, alias)
	structOK := (structs[pt] || declared) && (upper || declared)
	base := p.sal
	if i := strings.Index(base, "("); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(strings.Fields(base + " _")[0])
	switch base {
	case "_Out_", "_Inout_", "_In_":
		if pw || structOK {
			return true
		}
	}
	// A POINTER THE HOST GAVE, handed back as the value `ptr` (HeapFree,
	// HeapSize, VirtualFree): a void* the callee reads or frees, not writes
	if byteType(pt) && (pt == "void" || pt == "VOID") {
		for _, w := range strings.Fields(p.sal) {
			switch {
			case w == "_In_", strings.HasPrefix(w, "_Frees_ptr_"), w == "_Pre_notnull_":
				return true
			}
		}
	}
	if byteType(pt) && segmentSAL(base) {
		if n := salArg(p.sal); n != "" {
			for _, q := range f.params {
				if q.name == n {
					return true
				}
			}
		}
	}
	return false
}

var wideString = map[string]bool{"LPCWSTR": true, "LPWSTR": true, "PCWSTR": true, "PWSTR": true,
	"LPCTSTR": true, "LPTSTR": true}

// pointee is a pointer type's pointee, by the typedef chain, or by the SDK's
// LP/P naming of a struct's or a word's pointer typedef; "" when unknown.
func pointee(t string, structs map[string]bool, alias map[string]string) string {
	for i := 0; i < 8; i++ {
		c := cleanType(t)
		if strings.HasSuffix(c, "*") {
			return cleanType(strings.TrimSpace(strings.TrimSuffix(c, "*")))
		}
		nxt, ok := alias[strings.TrimSpace(t)]
		if !ok || nxt == t {
			break
		}
		t = nxt
	}
	c := cleanType(t)
	for _, pre := range []string{"LPC", "LP", "PC", "NP", "P"} {
		if x := strings.TrimPrefix(c, pre); x != c && (structs[x] || word[x]) {
			return x
		}
	}
	return ""
}

func isWordAlias(t string, alias map[string]string) bool {
	for i := 0; i < 8; i++ {
		if word[t] {
			return true
		}
		nxt, ok := alias[t]
		if !ok || nxt == t {
			return false
		}
		t = cleanType(nxt)
	}
	return false
}

func byteType(t string) bool {
	switch t {
	case "void", "VOID", "BYTE", "UCHAR", "CHAR", "char", "uint8_t", "INT8", "UINT8":
		return true
	}
	return false
}

func segmentSAL(base string) bool {
	switch base {
	case "_In_reads_bytes_", "_Out_writes_bytes_", "_Inout_updates_bytes_", "_In_reads_", "_Out_writes_",
		"_Inout_updates_", "_Out_writes_bytes_to_", "_Out_writes_to_":
		return true
	}
	return false
}

// salArg is a SAL annotation's first argument, when it is a name.
func salArg(sal string) string {
	i, j := strings.Index(sal, "("), strings.Index(sal, ")")
	if i < 0 || j < i {
		return ""
	}
	a := strings.TrimSpace(sal[i+1 : j])
	if k := strings.Index(a, ","); k >= 0 {
		a = strings.TrimSpace(a[:k])
	}
	for _, r := range a {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return a
}

// handRecords are the structs the windows target declares as records, each
// checked against clang (TestWin32DeclarationsAreTheirPrototypes), read from
// targets/win32: `set NAME = record {`.
var handRecords = map[string]bool{}

// handWords are the SDK's types the target declares as an integer set, not a
// record: LARGE_INTEGER, a union read as its QuadPart (winnt.oro).
var handWords = map[string]bool{}

// handRecordNaming reports whether a record the target declares is pt under
// another typedef name: INIT_ONCE, whose chain ends at the union RTL_RUN_ONCE
// a PINIT_ONCE points to. Its layout is checked against clang either way
// (TestAStructsLayoutIsTheCCompilers).
func handRecordNaming(pt string, alias map[string]string) bool {
	r := resolveName(pt, alias)
	for n := range handRecords {
		if m := resolveName(n, alias); m == pt || m == r || strings.TrimPrefix(m, "_") == strings.TrimPrefix(pt, "_") {
			return true
		}
	}
	return false
}

// resolveName follows a name's typedef chain to its last name.
func resolveName(n string, alias map[string]string) string {
	for i := 0; i < 8; i++ {
		nxt, ok := alias[n]
		if !ok || nxt == n || strings.Contains(nxt, "*") {
			return n
		}
		n = cleanType(nxt)
	}
	return n
}

func readHandRecords() {
	fs, _ := filepath.Glob(filepath.Join("targets", "win32", "*.oro"))
	re := regexp.MustCompile(`(?m)^\s*(?:pub\s+)?set\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*record\b`)
	reWord := regexp.MustCompile(`(?m)^\s*(?:pub\s+)?set\s+([A-Z_][A-Z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_.]*)`)
	for _, f := range fs {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			handRecords[m[1]] = true
		}
		for _, m := range reWord.FindAllStringSubmatch(string(b), -1) {
			if !handRecords[m[1]] && m[2] != "record" && m[2] != "host" && m[2] != "fn" {
				handWords[m[1]] = true
			}
		}
	}
}

func main() {
	sdk := flag.String("sdk", "", "SDK include directory (default: the newest installed)")
	emitDir := flag.String("emit", "", "write the declarable subset as target files into DIR")
	sig := flag.String("sig", "", "print the parsed signature and verdict for one entry point")
	sizes := flag.String("sizes", "", "ask MSVC for each blocking aggregate's size and write it to FILE")
	checkEnums := flag.Bool("check-enums", false, "ask MSVC to confirm every enum a declaration relies on is an integer type")
	sizeTab := flag.String("sizetable", sizeTableFile, "read aggregate sizes from FILE")
	cbs := flag.Bool("callbacks", false, "list each entry point taking a function pointer, whether it passes a context word and whether the callback receives one, and stop")
	org := flag.Bool("org", false, "report how the SDK's functions partition by header and by DLL, and stop")
	namesFile := flag.String("names", "", "for each entry point FILE names (one a line, `_imp__` and `@n` stripped), print its header, verdict, arity and DLL by this survey's own rules, and stop")
	flag.Parse()
	readSizes(*sizeTab)
	readHandRecords()

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
			src = reCeePure.ReplaceAllString(src, "CALLBACK")
			src = reMac.ReplaceAllString(src, "${1}WINAPI")
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
				ps, va := parseParams(m[4])
				conv := "stdcall"
				if m[2] == "WINAPIV" {
					conv = "cdecl"
				}
				fns = append(fns, fn{base, m[3], r, ps, va, conv})
			}
			for _, m := range reStdapi.FindAllStringSubmatch(src, -1) {
				r := "HRESULT"
				if m[2] != "" {
					r = strings.TrimSpace(m[2])
				}
				ps, va := parseParams(m[4])
				fns = append(fns, fn{base, m[3], r, ps, va, "stdcall"})
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

	if *cbs {
		reportCallbacks(fns, structs, handles, callbacks, alias)
		return
	}

	if *org {
		lk, err := loadLinkage(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		reportOrganization(fns, lk)
		return
	}

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
	var callable2, callable2Upper, callable0Narrow int
	var callable2Names []string
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
	// each entry point's verdict by the rules below, for -names
	verdictOf := map[string]string{}
	// and by the model of 2026-11-19, the lower bound (canPass2)
	verdict2Of := map[string]string{}

	for _, f := range fns {
		why := ok
		anyOutParam := false
		for _, p := range f.params {
			for _, s := range strings.Fields(p.sal) {
				salCount[salKind(s)]++
			}
		}
		// THE MODEL OF 2026-11-19 (canPass2), on every entry point: a struct
		// by value is passed since ADR 0129, so a name the old classifier
		// refused for one is judged too
		all2, all2u := model2(f, structs, handles, callbacks, alias)
		verdict2Of[f.name] = "not callable"
		if all2 {
			verdict2Of[f.name] = "callable"
			callable2++
			callable2Names = append(callable2Names, f.name)
		}
		if all2u {
			callable2Upper++
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
			verdictOf[f.name] = "not declarable: " + string(why)
			continue
		}
		declarable++
		arity[len(f.params)]++
		decls = append(decls, f)

		reach := true
		usedNull := false
		blocked := ""
		for _, p := range f.params {
			sh := classify(p.typ, structs, handles, callbacks, alias)
			if !canPass(p, sh) {
				reach = false
				blocked = p.typ + " " + p.sal
				break
			}
			if sh == shPtr {
				usedNull = true
			}
		}
		// the model of 2026-09-08 with UTF-16 left out, for the comparison
		all0 := true
		for _, p := range f.params {
			sh := classify(p.typ, structs, handles, callbacks, alias)
			if !canPass(p, sh) || sh == shString && wideString[cleanType(p.typ)] {
				all0 = false
			}
		}
		if all0 {
			callable0Narrow++
		}
		if !reach {
			byGap[rOutParam]++
			verdictOf[f.name] = "declarable, not callable: an argument it cannot pass, " + strings.TrimSpace(blocked)
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
			verdictOf[f.name] = "callable, its answer through an out-parameter"
			continue
		}
		verdictOf[f.name] = "usable"
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
	// `llvm.null`, Σ_LLVM's own constant, is one line of target data (MASM's
	// `x64.null`, added 2026-09-08, until it retired).
	fmt.Printf("  of the callable, %d reach it only by passing NULL where SAL says _opt_/_Reserved_\n", nullPassed)
	fmt.Printf("  — writable since 2026-09-08, now as llvm.null; before that the count was aspirational\n\n")

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

	if *namesFile != "" {
		lk, lkErr := loadLinkage(dir)
		if err := reportNames(*namesFile, fns, verdictOf, verdict2Of, lk, lkErr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	reportStructs(argOf, resOf, structFns)
	lk, lkErr := loadLinkage(dir)
	reportLinking(lk, lkErr, callableNames, len(fns), "the model of 2026-09-08")
	fmt.Printf("\nTHE MODEL OF 2026-11-19 (canPass2: _Out_, _Inout_ and _In_ pointers, byte segments, ADR 0130-0132):\n")
	fmt.Printf("  the model of 2026-09-08, wide strings out: %5d  %5.1f%%   [what canPass passes, less UTF-16]\n", callable0Narrow, pct(callable0Narrow, len(fns)))
	fmt.Printf("  callable, structs the target declares:  %5d  %5.1f%%\n", callable2, pct(callable2, len(fns)))
	fmt.Printf("  callable, every struct the SDK defines: %5d  %5.1f%%\n", callable2Upper, pct(callable2Upper, len(fns)))
	fmt.Printf("  (%d structs declared by the target)\n", len(handRecords))
	reportLinking(lk, lkErr, callable2Names, len(fns), "today's model")

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
	reCbType  = regexp.MustCompile(`typedef[^;]*\(\s*(?:CALLBACK|WINAPI|APIENTRY|NTAPI|__stdcall|\*)\s*\*?\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)(?:\s*\(([^;]*)\))?`)
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
		// `} RECT, *PRECT, NEAR *NPRECT, FAR *LPRECT;`: each starred name after
		// the comma is a pointer to the struct (survey-2026-11-19; reStruct
		// read the first name only, so LPRECT had no pointee)
		for _, tok := range strings.Split(src[ix[3]:ix[1]], ",") {
			if !strings.Contains(tok, "*") {
				continue
			}
			fs := strings.Fields(strings.NewReplacer("*", " ", ";", " ").Replace(tok))
			if len(fs) > 0 {
				if pn := fs[len(fs)-1]; alias[pn] == "" {
					alias[pn] = nm + "*"
				}
			}
		}
	}
	for _, m := range reCbType.FindAllStringSubmatch(src, -1) {
		callbacks[m[1]] = true
		if _, have := cbParams[m[1]]; !have {
			cbParams[m[1]] = m[2]
		}
	}
}

// cbParams is each callback typedef's parameter list, as written.
var cbParams = map[string]string{}

// ctxWord names the types C uses for a closure's environment passed by hand:
// a pointer to anything, or a pointer-width integer (callbacks-research §2.1).
var ctxWord = map[string]bool{"LPARAM": true, "LPVOID": true, "PVOID": true, "LPCVOID": true,
	"PCVOID": true, "void*": true, "VOID*": true, "DWORD_PTR": true, "ULONG_PTR": true,
	"UINT_PTR": true, "LONG_PTR": true, "INT_PTR": true}

func isCtx(t string, alias map[string]string) bool {
	for i := 0; i < 8; i++ {
		c := strings.ReplaceAll(cleanType(t), " ", "")
		if ctxWord[c] {
			return true
		}
		nxt, have := alias[strings.TrimSpace(t)]
		if !have || nxt == t {
			return false
		}
		t = nxt
	}
	return false
}

// reportCallbacks answers tier-1-research §2.2: where a C API takes a
// function pointer, does it also take a context word, the environment of a
// closure converted by hand, and does the callback's own type receive one? A
// callback in a struct's field (WNDCLASSA's lpfnWndProc) is not a parameter
// and is not counted here.
func reportCallbacks(fns []fn, structs, handles, callbacks map[string]bool, alias map[string]string) {
	var both, apiOnly, cbOnly, neither int
	for _, f := range fns {
		var cb []string
		apiCtx := false
		for _, p := range f.params {
			if classify(p.typ, structs, handles, callbacks, alias) == shFunc {
				cb = append(cb, cleanType(p.typ))
			} else if isCtx(p.typ, alias) {
				apiCtx = true
			}
		}
		if len(cb) == 0 {
			continue
		}
		cbCtx := false
		for _, c := range cb {
			for i, t := 0, c; i < 8 && t != ""; i++ {
				if ps, have := cbParams[t]; have {
					params, _ := parseParams(ps)
					for _, q := range params {
						if isCtx(q.typ, alias) {
							cbCtx = true
						}
					}
					break
				}
				t = alias[t]
			}
		}
		kind := "neither"
		switch {
		case apiCtx && cbCtx:
			kind, both = "both", both+1
		case apiCtx:
			kind, apiOnly = "api-only", apiOnly+1
		case cbCtx:
			kind, cbOnly = "callback-only", cbOnly+1
		default:
			neither++
		}
		fmt.Printf("%-14s %-48s %-18s %s\n", kind, f.name, f.hdr, strings.Join(cb, ","))
	}
	fmt.Printf("\ntotal %d: context passed and received %d, passed only %d, received only %d, neither %d\n",
		both+apiOnly+cbOnly+neither, both, apiOnly, cbOnly, neither)
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

// llvmCall is a Win32 function's prototype as the windows target declares one
// (ADR 0094): its sets, which ARE its C types, and its C strings' obligations.
// Its call is the C ABI's, derived by the LLVM printer on the triple, so nothing
// here writes an instruction. Each value's set:
//
//	an integer                  its C type's whole value set: the width is the
//	                            ABI, and a subset does not determine it; a 64-bit
//	                            unsigned type is 0..2^64-1, the unsigned word, at
//	                            an argument as at a result (ADR 0126)
//	a pointer or a handle       `ptr`
//	a C string                  `String` where nulfree (ADR 0091)
//
// Until ADR 0126 a 64-bit unsigned argument was 0..2^63-1 and a result the
// signed word: two sets for one type, one of them a claim no C type makes.
type llvmDecl struct {
	sets   []string // each parameter's set
	result string   // the result's set, or "" for void
	where  []string // each C string parameter's obligation, nulfree (ADR 0091)
}

func llvmCall(f fn, structs, handles, callbacks map[string]bool, alias map[string]string) llvmDecl {
	var d llvmDecl
	for k, p := range f.params {
		nm := paramName(k)
		switch classify(p.typ, structs, handles, callbacks, alias) {
		case shString:
			d.sets = append(d.sets, "String")
			d.where = append(d.where, "nulfree("+nm+")")
		case shPtr:
			d.sets = append(d.sets, "ptr")
		default:
			if isHandle(p.typ, handles, alias) {
				d.sets = append(d.sets, "ptr")
				continue
			}
			w, signed, known := cInt(p.typ, alias)
			if known {
				d.sets = append(d.sets, cRange(w, signed))
			} else {
				d.sets = append(d.sets, wordSet)
			}
		}
	}
	switch classify(f.result, structs, handles, callbacks, alias) {
	case shVoid:
	case shPtr, shString:
		d.result = "ptr"
	default:
		if isHandle(f.result, handles, alias) {
			d.result = "ptr"
		} else if w, signed, known := cInt(f.result, alias); known {
			d.result = cRange(w, signed)
		} else {
			d.result = wordSet
		}
	}
	return d
}

// isHandle follows the alias chain classify follows to a handle. A handle is a
// word to the verdicts, which ask whether a program can pass one (it can: it
// holds the one a call answered), and a pointer to the declaration, which says
// what C type crosses: DECLARE_HANDLE(HWND) is `struct HWND__ *`, and HANDLE is
// PVOID, so `ptr`, four bytes on i686 where the word is eight (ADR 0126).
func isHandle(t string, handles map[string]bool, alias map[string]string) bool {
	for i := 0; i < 8; i++ {
		c := cleanType(t)
		if handles[c] || strings.HasPrefix(c, "H") && len(c) > 1 && strings.ToUpper(c) == c {
			return true
		}
		nxt, have := alias[strings.TrimSpace(t)]
		if !have || nxt == t {
			return false
		}
		t = nxt
	}
	return false
}

// wordSet is the windows target's word, S₆₄ (ADR 0083), the set of a 64-bit value.
const wordSet = "-2^63..2^63-1"

// cRange is the set of a C integer type of w bits.
func cRange(w int, signed bool) string {
	if signed {
		return fmt.Sprintf("-2^%d..2^%d-1", w-1, w-1)
	}
	return fmt.Sprintf("0..2^%d-1", w)
}

// paramName is the k-th parameter's name: a, b, c, … past r, the result's.
func paramName(k int) string {
	const letters = "abcdefghijklmnopqstuvwxyz"
	if k < len(letters) {
		return letters[k : k+1]
	}
	return fmt.Sprintf("a%d", k)
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
		// THE MODULE IS THE HEADER, case-folded: Windows' file names are equal up
		// to case, the documentation writes them in lower case, and so do the
		// hand modules (windows/winuser for WinUser.h, ADR 0126).
		mod := strings.ToLower(strings.TrimSuffix(h, ".h"))
		var b, body strings.Builder
		b.WriteString(jdecl.Comment("GENERATED by gauntlet/stdlib/win32.go — do not edit.",
			"The declarable subset of the Windows SDK's "+h+"."))
		b.WriteString("\n")
		for _, f := range list {
			// The C prototype's parameters are unnamed: they are a, b, c, …,
			// as the hand files name them, and `r` is the result.
			d := llvmCall(f, structs, handles, callbacks, alias)
			var args []jdecl.Param
			for k := range f.params {
				args = append(args, jdecl.Param{Name: paramName(k), Set: d.sets[k]})
			}
			res, kind := "r ∈ "+d.result, "expr"
			if d.result == "" {
				// THE RESULT OF A STATEMENT IS ITS FIRST ARGUMENT, on every
				// target, so a void entry point is declared `stmt` and typed by
				// the argument it hands back; its template writes no `${r}`,
				// since the emitter allocates none.
				res, kind = args[0].Set, "stmt"
			}
			// THE LIBRARY THAT RESOLVES IT, when exactly one can be named
			// honestly (linkage.resolve). A declaration with none still loads
			// and still type-checks; what it cannot do is link, and that is a
			// refusal at the linker rather than a binding to the wrong DLL.
			lib, _ := lk.resolve(f.name)
			body.WriteString(jdecl.Fn{Name: f.name, Params: args, Result: res, Kind: kind,
				Call: f.name, Conv: f.conv, Lib: lib, Where: strings.Join(d.where, " && ")}.String())
			n++
		}
		b.WriteString(jdecl.Module("win/"+mod, body.String()))
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
// from the family's base, targets/win32/win32.oro (ADR 0122), rather than repeated here. Until
// linkline-2026-09-13 the list was a constant in emit/target.go and a copy of it
// in this file: a host fact written twice is two facts that can disagree.
func baseLink() (map[string]bool, error) {
	b, err := os.ReadFile(filepath.Join("targets", "win32", "win32.oro"))
	if err != nil {
		return nil, err
	}
	var code []string
	for _, ln := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(ln), "//") {
			code = append(code, ln)
		}
	}
	m := reLink.FindStringSubmatch(strings.Join(code, "\n"))
	if m == nil {
		return nil, fmt.Errorf("targets/win32/win32.oro declares no link …")
	}
	base := map[string]bool{}
	for _, q := range reQuoted.FindAllStringSubmatch(m[1], -1) {
		base[strings.ToLower(q[1])] = true
	}
	return base, nil
}

var (
	// The model's form in J: `link "kernel32" "msvcrt" …`, a line of its own
	// (spec/j.md §2.12).
	reLink   = regexp.MustCompile(`(?m)^\s*link((?:[ 	]+"[^"]*")+)[ 	]*$`)
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
func reportLinking(lk *linkage, lkErr error, callable []string, total int, model string) {
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
	fmt.Printf("  CALLABLE AND IT LINKS, %s: %5d  %5.1f%% of the flat API\n", model, links, pct(links, total))

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

// probeBat finds MSVC the way emit/target.go's buildBat does, because that
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

// reportOrganization measures two partitions of the SDK's entry points, BY
// HEADER (what a C program includes, and what the documentation's URLs are
// keyed by: learn.microsoft.com/…/api/fileapi/nf-fileapi-readfile) and BY DLL
// (what the loader binds, through the import libraries), and how far each is a
// refinement of the other (win32org-2026-10-05). A name whose libraries bind it
// to one real DLL has that DLL; one reachable only through an API set, or bound
// to several, or to none, is counted apart, since it has no DLL to file it under.
func reportOrganization(fns []fn, lk *linkage) {
	dllOf := map[string]string{}
	why := map[string]int{}
	for _, f := range fns {
		real := map[string]bool{}
		viaAPI := false
		for _, x := range lk.bind[f.name] {
			if apiSet(x.dll) {
				viaAPI = true
				continue
			}
			real[strings.ToLower(x.dll)] = true
		}
		switch {
		case len(real) == 1:
			for d := range real {
				dllOf[f.name] = d
			}
			why["one DLL"]++
		case len(real) > 1:
			why["several DLLs"]++
		case viaAPI:
			why["an API set only"]++
		default:
			why["no import"]++
		}
	}
	byHdr := map[string]map[string]int{}
	byDLL := map[string]map[string]int{}
	hdrN := map[string]int{}
	for _, f := range fns {
		hdrN[f.hdr]++
		d, ok := dllOf[f.name]
		if !ok {
			continue
		}
		if byHdr[f.hdr] == nil {
			byHdr[f.hdr] = map[string]int{}
		}
		byHdr[f.hdr][d]++
		if byDLL[d] == nil {
			byDLL[d] = map[string]int{}
		}
		byDLL[d][f.hdr]++
	}
	fmt.Printf("ENTRY POINTS: %d, in %d headers\n", len(fns), len(hdrN))
	for _, k := range []string{"one DLL", "several DLLs", "an API set only", "no import"} {
		fmt.Printf("  %-18s %6d  %5.1f%%\n", k, why[k], pct(why[k], len(fns)))
	}
	pure := func(m map[string]map[string]int) (parts, pureParts, fnsIn, fnsPure int) {
		for _, inner := range m {
			parts++
			n := 0
			for _, c := range inner {
				n += c
			}
			fnsIn += n
			if len(inner) == 1 {
				pureParts++
				fnsPure += n
			}
		}
		return
	}
	hp, hpp, hf, hfp := pure(byHdr)
	dp, dpp, df, dfp := pure(byDLL)
	fmt.Printf("\nOF THE %d NAMES WITH ONE DLL:\n", hf)
	fmt.Printf("  headers: %4d, of which %4d bind ONE DLL (%5.1f%% of the names)\n", hp, hpp, pct(hfp, hf))
	fmt.Printf("  DLLs:    %4d, of which %4d come from ONE header (%5.1f%% of the names)\n", dp, dpp, pct(dfp, df))
	type kv struct {
		k string
		n int
	}
	top := func(m map[string]map[string]int, title string) {
		var xs []kv
		for k, inner := range m {
			n := 0
			for _, c := range inner {
				n += c
			}
			xs = append(xs, kv{k, n})
		}
		sort.Slice(xs, func(i, j int) bool { return byCount(xs[i].n, xs[j].n, xs[i].k, xs[j].k) })
		fmt.Printf("\nTHE LARGEST %s:\n", title)
		for i, x := range xs {
			if i == 12 {
				break
			}
			var parts []kv
			for k, c := range m[x.k] {
				parts = append(parts, kv{k, c})
			}
			sort.Slice(parts, func(i, j int) bool { return byCount(parts[i].n, parts[j].n, parts[i].k, parts[j].k) })
			var ps []string
			for j, p := range parts {
				if j == 4 {
					ps = append(ps, fmt.Sprintf("… %d more", len(parts)-4))
					break
				}
				ps = append(ps, fmt.Sprintf("%s %d", p.k, p.n))
			}
			fmt.Printf("  %-28s %5d  %s\n", x.k, x.n, strings.Join(ps, ", "))
		}
	}
	top(byDLL, "DLLs, AND THE HEADERS THEIR NAMES COME FROM")
	top(byHdr, "HEADERS, AND THE DLLs THEIR NAMES BIND")
	fmt.Printf("\nTHE CONSOLE'S NAMES:\n")
	for _, n := range []string{"GetStdHandle", "WriteFile", "ReadFile", "WriteConsoleW", "ReadConsoleW",
		"GetConsoleMode", "SetConsoleMode", "SetConsoleOutputCP", "GetConsoleOutputCP", "SetConsoleCP",
		"OutputDebugStringW", "MultiByteToWideChar", "WideCharToMultiByte", "ExitProcess"} {
		for _, f := range fns {
			if f.name == n {
				fmt.Printf("  %-22s %-16s %s\n", n, f.hdr, dllOf[n])
			}
		}
	}
}

// reportNames is -names: each listed entry point's header, verdict, arity and
// DLL, one a line, tab-separated, by the survey's own rules, so a program's
// surface is measured as the SDK's was (tinyretropad-2026-11-17).
func reportNames(file string, fns []fn, verdictOf, verdict2Of map[string]string, lk *linkage, lkErr error) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	byName := map[string]fn{}
	for _, f := range fns {
		if _, dup := byName[f.name]; !dup {
			byName[f.name] = f
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		n := strings.TrimSpace(line)
		n = strings.TrimPrefix(n, "_imp__")
		if i := strings.IndexByte(n, '@'); i >= 0 {
			n = n[:i]
		}
		if n == "" {
			continue
		}
		f, ok := byName[n]
		if !ok {
			fmt.Printf("%s\t-\tnot among the SDK's entry points\t-\t-\n", n)
			continue
		}
		dll := "-"
		if lkErr == nil {
			lib, why := lk.resolve(n)
			dll = lib
			if lib == "" {
				dll = why
			}
		}
		v2 := verdict2Of[n]
		if v2 == "" {
			v2 = "not callable"
		}
		fmt.Printf("%s\t%s\t%s\t%d\t%s\t%s\n", n, f.hdr, verdictOf[n], len(f.params), dll, v2)
	}
	return nil
}

// callbackPassable says the callback typedef t's parameters are words,
// handles or pointers (a handle classifies as a pointer): what a claim's sets can
// state, so a λ can be written for it (ADR 0143). A context word is a pointer
// or a pointer-width integer, so both the API's context and its absence pass.
func callbackPassable(t string, structs, handles, callbacks map[string]bool, alias map[string]string) bool {
	for i := 0; i < 8 && t != ""; i++ {
		ps, have := cbParams[t]
		if !have {
			t = alias[t]
			continue
		}
		params, _ := parseParams(ps)
		for _, q := range params {
			switch classify(q.typ, structs, handles, callbacks, alias) {
			case shWord, shPtr: // a handle classifies as a pointer
			default:
				return false
			}
		}
		return true
	}
	return false
}
