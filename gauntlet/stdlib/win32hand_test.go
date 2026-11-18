package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A WIN32 DECLARATION WRITTEN BY HAND IS ITS PROTOTYPE (ADR 0094, 0126).
//
// Win32 is by header: `windows/H` in targets/win32 is header H.h, written by
// hand, and the survey emits the same header's declarable prototypes as `win/H`
// (ADR 0022: the generator is the checker of the mechanical half). Both are
// loaded into ONE target, the generated files a layer over targets/, so every
// type name resolves by the same definitions, the data model's among them, and
// a hand declaration agrees with the SDK's when:
//
//   - its callee, convention, library and kind are the generated one's;
//   - its C arguments, its parameters or its argument terms (ADR 0118), are as
//     many as the prototype's, and each is passed as the prototype's is: the
//     same LLVM type, and for an integer the same set, since a C integer's set
//     is its type's whole value set and so determines its width and its sign;
//   - a pointer it passes is a pointer the prototype takes: `ptr`, a cell for
//     an out-parameter (ADR 0048), a byte table's elements, or null, where the
//     SDK has a pointer or a C string; a C string only where the SDK has one;
//   - its result is the prototype's, an integer result a subset of it (a claim
//     a person justifies, ADR 0022), and ∅ where the SDK's is a void the
//     header marks DECLSPEC_NORETURN (ADR 0090).
//
// A name the generator cannot declare is EXEMPT, with its reason, and an
// exemption the generator could meet is refused, so none goes stale. A checker
// that cannot fail proves nothing: each of win32Mistakes is planted and must be
// caught.
func TestWin32DeclarationsAreTheirPrototypes(t *testing.T) {
	s := surveyOf(t, "win32")
	hand, err := emit.LoadTarget(filepath.Join(root, "targets", "windows"))
	if err != nil {
		t.Fatal(err)
	}
	modules := map[string]bool{}
	for n, p := range hand.Prims {
		if p.CSym != "" && strings.HasPrefix(n, "windows/") {
			modules[n[:strings.LastIndex(n, ".")]] = true
		}
	}
	if len(modules) == 0 {
		t.Fatal("targets/windows declares no Win32 header module, so nothing is checked")
	}
	// THE GENERATED HEADERS AS A LAYER over the built-in targets, the way an
	// acceptance program loads them.
	layer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(layer, "windows"), 0o755); err != nil {
		t.Fatal(err)
	}
	for m := range modules {
		h := strings.TrimPrefix(m, "windows/")
		b, err := os.ReadFile(filepath.Join(s.runs[0].emit, h+".oro"))
		if err != nil {
			t.Fatalf("%s: the survey emits no %s.oro: %v", m, h, err)
		}
		if err := os.WriteFile(filepath.Join(layer, "windows", h+".oro"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tg, err := emit.LoadTargetLayers("windows", []string{layer, filepath.Join(root, "targets")})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for n, p := range tg.Prims {
		if p.CSym != "" && modules[moduleOfName(n)] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	checked := 0
	for _, n := range names {
		g, inSDK := tg.Prims[sdkName(n)]
		why, exempt := win32Exempt[n]
		switch {
		case exempt && inSDK:
			t.Errorf("%s is exempt (%s), and the survey declares it: the exemption is stale", n, why)
		case exempt:
		case lacksPrototype(tg, n):
			t.Errorf("%s is declared by hand and the SDK's %s declares no such function", n, moduleOfName(n))
		default:
			if err := agreeC(tg, n, tg.Prims[n], g); err != nil {
				t.Error(err)
			}
			checked++
		}
	}
	for n, why := range win32Exempt {
		if _, ok := tg.Prims[n]; !ok {
			t.Errorf("%s is exempt (%s), and no hand file declares it: the exemption is dead", n, why)
		}
	}
	if checked < len(names)-len(win32Exempt) || checked == 0 {
		t.Fatalf("%d of %d hand declarations checked", checked, len(names))
	}
	t.Logf("%d hand declarations in %d headers agree with their prototypes; %d exempt", checked, len(modules), len(win32Exempt))

	for what, plant := range win32Mistakes {
		n, p := plant(tg)
		if err := agreeC(tg, n, p, tg.Prims[sdkName(n)]); err == nil {
			t.Errorf("planted: %s (%s) was not caught", what, n)
		}
	}
	// A NAME THE SDK LACKS, the coverage half: planted in a hand module, the
	// loop above must have refused it.
	tg.Prims["windows/fileapi.Bogus"] = tg.Prims["windows/fileapi.CreateFileA"]
	if _, ok := tg.Prims[sdkName("windows/fileapi.Bogus")]; ok || !lacksPrototype(tg, "windows/fileapi.Bogus") {
		t.Error("planted: a name the SDK lacks was not caught")
	}
	delete(tg.Prims, "windows/fileapi.Bogus")
}

// lacksPrototype is the loop's refusal of a hand name: not exempt, and no
// generated declaration of the name.
func lacksPrototype(tg *emit.Target, n string) bool {
	_, inSDK := tg.Prims[sdkName(n)]
	_, exempt := win32Exempt[n]
	return !inSDK && !exempt
}

// win32Exempt: the hand declarations the generator cannot produce, each with why.
var win32Exempt = map[string]string{
	"windows/winnt.RtlZeroMemory": "winnt.h writes it as a macro over memset; kernel32 exports the function",
	"windows/fileapi.ReadFile":    "the survey cannot resolve LPOVERLAPPED's typedef",
	"windows/heapapi.HeapFree":    "the survey cannot resolve the `__drv_freesMem(Mem) LPVOID` parameter",
	"windows/fileapi.SetFilePointerEx": "LARGE_INTEGER by value, a union the survey counts as a struct; " +
		"its eight bytes cross as an i64 does on both triples",
	"windows/winuser.PtInRect": "POINT by value, which the survey refuses; its passing is the ABI's, measured " +
		"against clang on both triples (ADR 0129, TestAStructIsPassedByValue)",
}

// win32Mistakes are the errors a person could make writing a header by hand,
// each planted on a copy of one declaration.
var win32Mistakes = map[string]func(tg *emit.Target) (string, emit.Prim){
	"a library not the SDK's": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/fileapi.CreateFileA", func(p *emit.Prim) { p.Lib = "user32" })
	},
	"a callee not the SDK's": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/fileapi.CreateFileA", func(p *emit.Prim) { p.CSym = "CreateFileW" })
	},
	"a convention not the SDK's": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/winuser.ShowWindow", func(p *emit.Prim) { p.CConv = "cdecl" })
	},
	"an argument of another width": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/fileapi.CreateFileA", func(p *emit.Prim) { p.Args[1] = "int 0 65535" })
	},
	"an argument of the other sign": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/winuser.ShowWindow", func(p *emit.Prim) { p.Args[1] = "int 0 4294967295" })
	},
	"a 64-bit unsigned argument read as the word": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/heapapi.HeapAlloc", func(p *emit.Prim) { p.Args[2] = "int" })
	},
	"a 64-bit unsigned argument narrowed to the word's naturals": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/heapapi.HeapAlloc", func(p *emit.Prim) { p.Args[2] = "int 0 9223372036854775807" })
	},
	"a C argument dropped": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/winuser.SetWindowPos", func(p *emit.Prim) { p.Args = p.Args[:len(p.Args)-1] })
	},
	"a C string where the SDK passes a pointer": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/heapapi.HeapAlloc", func(p *emit.Prim) { p.Args[0] = "string" })
	},
	"a result of another width": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/handleapi.CloseHandle", func(p *emit.Prim) { p.Result = "int 0 65535" })
	},
	"a result wider than the SDK's": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/winuser.GetKeyState", func(p *emit.Prim) { p.Result = "int -2147483648 2147483647" })
	},
	"a statement where the SDK answers": func(tg *emit.Target) (string, emit.Prim) {
		return mutateC(tg, "windows/errhandlingapi.GetLastError", func(p *emit.Prim) { p.Kind = "stmt" })
	},
}

// mutateC copies a declaration, its slices included, and plants a mistake.
func mutateC(tg *emit.Target, n string, f func(*emit.Prim)) (string, emit.Prim) {
	p := tg.Prims[n]
	p.Args = append([]string(nil), p.Args...)
	p.CArgs = append([]emit.CArg(nil), p.CArgs...)
	f(&p)
	return n, p
}

// sdkName is a hand name's generated counterpart: windows/H.F is win/H.F.
func sdkName(n string) string { return "win/" + strings.TrimPrefix(n, "windows/") }

func moduleOfName(n string) string {
	if i := strings.LastIndex(n, "."); i > 0 {
		return n[:i]
	}
	return ""
}

// agreeC is the C half of a hand declaration's agreement with its prototype.
func agreeC(tg *emit.Target, n string, h, g emit.Prim) error {
	switch {
	case h.CSym != g.CSym:
		return fmt.Errorf("%s calls %q, and the SDK's is %q", n, h.CSym, g.CSym)
	case h.CConv != g.CConv:
		return fmt.Errorf("%s is %q, and the SDK's prototype is %q", n, h.CConv, g.CConv)
	case h.Lib != g.Lib:
		return fmt.Errorf("%s links %q, and the SDK's import libraries resolve it to %q", n, h.Lib, g.Lib)
	}
	// ∅ IS A VOID THAT NEVER RETURNS (ADR 0090): the generator writes the void
	// as a statement, the hand file as a function into ∅.
	noreturn := h.Kind == "expr" && h.Result == "empty" && g.Kind == "stmt"
	if h.Kind != g.Kind && !noreturn {
		return fmt.Errorf("%s is a %s, and the SDK's is a %s", n, h.Kind, g.Kind)
	}
	// THE C ARGUMENTS: the argument terms when the declaration writes them,
	// otherwise the parameters in order.
	var cs []string
	if len(h.CArgs) > 0 {
		for _, c := range h.CArgs {
			switch c.Kind {
			case emit.CParam:
				cs = append(cs, h.Args[c.I])
			case emit.CElem:
				cs = append(cs, "elem")
			case emit.CNull:
				cs = append(cs, "null")
			}
		}
	} else {
		cs = h.Args
	}
	if len(cs) != len(g.Args) {
		return fmt.Errorf("%s passes %d C argument(s), and its prototype takes %d", n, len(cs), len(g.Args))
	}
	for i, a := range cs {
		if err := agreeCArg(tg, a, g.Args[i]); err != nil {
			return fmt.Errorf("%s: argument %d: %v", n, i+1, err)
		}
	}
	if h.Kind == "stmt" || noreturn {
		return nil
	}
	// THE HOST'S OWN RESULT (ADR 0070): a write-borrow's table is the argument
	// handed back, not the C function's answer.
	hr := emit.HostResults(h)
	if len(hr) != 1 {
		return fmt.Errorf("%s answers %d value(s), and its prototype one", n, len(hr))
	}
	hll, _, _, _ := tg.CType(hr[0])
	gll, _, _, _ := tg.CType(g.Result)
	if hll != gll {
		return fmt.Errorf("%s answers %s (%s), and its prototype %s (%s)", n, hr[0], hll, g.Result, gll)
	}
	if strings.HasPrefix(hll, "i") && !rangeWithin(tg.Word, hr[0], g.Result) {
		return fmt.Errorf("%s answers %s, which is not the prototype's %s or a subset of it", n, hr[0], g.Result)
	}
	return nil
}

// agreeCArg: a hand argument is passed as the prototype's parameter is.
func agreeCArg(tg *emit.Target, hand, sdk string) error {
	gll, gcross, _, ok := tg.CType(sdk)
	if !ok {
		return fmt.Errorf("the SDK's %s is no C type", sdk)
	}
	if hand == "elem" || hand == "null" {
		if gll != "ptr" {
			return fmt.Errorf("%s is a pointer, and the prototype takes %s", hand, sdk)
		}
		return nil
	}
	hll, hcross, _, ok := tg.CType(hand)
	if !ok {
		return fmt.Errorf("%s is no C type", hand)
	}
	if hll != gll {
		return fmt.Errorf("%s is passed as %s, and the prototype's %s as %s", hand, hll, sdk, gll)
	}
	if hll == "ptr" {
		// A C STRING ONLY WHERE THE SDK HAS ONE: its bytes and its NUL are a
		// claim about what the callee reads (ADR 0091).
		if hcross == "cstr" && gcross != "cstr" {
			return fmt.Errorf("a C string where the prototype takes a pointer that is not one")
		}
		return nil
	}
	if hand != sdk && !sameRange(hand, sdk) {
		return fmt.Errorf("%s, and the prototype's C type is %s", hand, sdk)
	}
	return nil
}

func sameRange(a, b string) bool {
	al, ah, ok1 := core.IntRangeBig(a)
	bl, bh, ok2 := core.IntRangeBig(b)
	return ok1 && ok2 && al.Cmp(bl) == 0 && ah.Cmp(bh) == 0
}

// trpSurface is what acceptance/trp-surface.oro's calls print from C on both
// triples (tinyretropad-2026-11-17).
var trpSurface = []string{"1", "5", "1", "3", "1", "1", "1", "0", "8", "4294967295", "1", "1", "1", "5", "1", "1", "1"}
