package emit_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A HOST STRUCT'S LAYOUT IS CHECKED, NEVER TRUSTED (llvm.md §6b, ADR 0128 item
// 10): every struct a Windows target declares, on both triples, its derived
// size, alignment and each field's offset stated to clang as _Static_asserts
// against the Windows SDK's own definition. rust-bindgen's layout tests and
// GHC's hsc2hs ask the C compiler the same question. The fault that ignores a
// packing must fail it: richedit.h's pack(4) makes FORMATRANGE 4-aligned on
// x86_64, where its natural alignment is 8.
func TestAStructsLayoutIsTheCCompilers(t *testing.T) {
	clang := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "LLVM", "bin", "clang.exe")
	if _, err := os.Stat(clang); err != nil {
		t.Skipf("no clang at %s: the layout is checked where LLVM is installed (CLAUDE.md)", clang)
	}
	check := func(target, triple string) (int, error) {
		tg, err := emit.LoadTarget(filepath.Join("..", "targets", target))
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for n := range tg.Layouts {
			names = append(names, n)
		}
		sort.Strings(names)
		var b strings.Builder
		b.WriteString("#define WIN32_LEAN_AND_MEAN\n#include <windows.h>\n#include <commdlg.h>\n#include <richedit.h>\n#include <stddef.h>\n")
		for _, n := range names {
			// the struct's C name, as the header spells it: INIT_ONCE, whose
			// core name is INIT-ONCE (σ_J's inverse, as each label is)
			c := core.Spell(n[strings.LastIndex(n, ".")+1:])
			labels, offs, size, align, err := tg.FieldLayout(n)
			if err != nil {
				t.Fatalf("%s on %s: %v", n, target, err)
			}
			fmt.Fprintf(&b, "_Static_assert(sizeof(%s) == %d, \"%s's size on %s\");\n", c, size, c, target)
			fmt.Fprintf(&b, "_Static_assert(_Alignof(%s) == %d, \"%s's alignment on %s\");\n", c, align, c, target)
			for i, l := range labels {
				fmt.Fprintf(&b, "_Static_assert(offsetof(%s, %s) == %d, \"%s.%s's offset on %s\");\n",
					c, core.Spell(l), offs[i], c, core.Spell(l), target)
			}
		}
		src := filepath.Join(t.TempDir(), "layout.c")
		if err := os.WriteFile(src, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(clang, "--target="+triple, "-fsyntax-only", src).CombinedOutput()
		if err != nil {
			return len(names), fmt.Errorf("%v\n%s", err, out)
		}
		return len(names), nil
	}
	for _, c := range []struct{ target, triple string }{
		{"windows", "x86_64-pc-windows-msvc"}, {"windows32", "i686-pc-windows-msvc"},
	} {
		n, err := check(c.target, c.triple)
		if err != nil {
			t.Errorf("%s: a derived layout is not clang's: %v", c.target, err)
		}
		if n < 5 {
			t.Errorf("%s declares %d structs; the check checks nothing it does not read", c.target, n)
		}
	}
	emit.PlantNaturalLayout = true
	defer func() { emit.PlantNaturalLayout = false }()
	if _, err := check("windows", "x86_64-pc-windows-msvc"); err == nil || !strings.Contains(err.Error(), "FORMATRANGE's alignment") {
		t.Errorf("a layout that ignores pack(4) still agrees with clang: the check cannot fail (%v)", err)
	}
	// and the data model's pack(1): PRINTDLGA laid out naturally on i686 is
	// 68 bytes, where commdlg.h's `#if !defined(_WIN64)` makes it 66
	if _, err := check("windows32", "i686-pc-windows-msvc"); err == nil || !strings.Contains(err.Error(), "PRINTDLGA's size") {
		t.Errorf("a layout that ignores the model's pack(1) still agrees with clang (%v)", err)
	}
}
