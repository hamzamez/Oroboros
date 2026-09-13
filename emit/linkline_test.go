package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE LINK LINE MOVED FROM A CONSTANT TO DATA, and the claim that nothing moved
// with it is the one worth pinning: a program that calls no primitive naming a
// `lib` must get exactly the build script it always got. The literal below is
// the line emit/target.go carried until linkline-2026-09-13, copied rather than
// derived, because a test that computes its expectation from the code under
// test cannot fail.
func TestALinkLineWithNoLibsIsTheOldConstant(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	const old = "link -nologo -subsystem:console -entry:main main.obj kernel32.lib msvcrt.lib ucrt.lib vcruntime.lib legacy_stdio_definitions.lib -out:main.exe || exit /b 1"
	got := asmBuildScript(tg.Link, map[string]bool{})
	if !strings.Contains(got, old+"\n") {
		t.Fatalf("a program using no lib no longer gets the old link line:\n%s", got)
	}
	if strings.Contains(got, "{{LIBS}}") {
		t.Fatalf("the placeholder survived:\n%s", got)
	}
}

// A used library joins the line after the target's own, sorted, once — and a
// library the target already links is not repeated, in any spelling, because
// Windows library names are case-insensitive.
func TestUsedLibrariesJoinTheLineSortedAndOnce(t *testing.T) {
	link := []string{"kernel32", "msvcrt"}
	used := map[string]bool{"user32": true, "advapi32": true, "KERNEL32": true}
	got := asmBuildScript(link, used)
	want := "main.obj kernel32.lib msvcrt.lib advapi32.lib user32.lib -out:main.exe"
	if !strings.Contains(got, want) {
		t.Fatalf("want %q in:\n%s", want, got)
	}
	// And it is a function of its input: map order must not reach the output.
	for i := 0; i < 20; i++ {
		if asmBuildScript(link, used) != got {
			t.Fatal("two calls with the same input built different scripts")
		}
	}
}

// `(lib "…")` parses onto the primitive and `(link …)` onto the target, and a
// `lib` with no string is refused rather than silently dropped.
func TestLibAndLinkParse(t *testing.T) {
	parse := func(src string) (*Target, error) {
		term, err := core.ReadTerm(src)
		if err != nil {
			t.Fatal(err)
		}
		return parseTarget(term, "t.oro")
	}
	tg, err := parse(`(target t
  (backend x86-64)
  (link "kernel32" "ucrt")
  (link "vcruntime")
  (prim f (int) int expr "call F" (import "F") (lib "user32")))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tg.Link, " "); got != "kernel32 ucrt vcruntime" {
		t.Fatalf("link = %q", got)
	}
	if p := tg.Prims["f"]; p.Lib != "user32" || p.Import != "F" {
		t.Fatalf("prim f: lib %q import %q", p.Lib, p.Import)
	}
	if _, err := parse(`(target t (prim f (int) int expr "call F" (lib user32)))`); err == nil {
		t.Fatal("(lib user32) with a bare name was accepted")
	}
	if _, err := parse(`(target t (link))`); err == nil {
		t.Fatal("(link) naming nothing was accepted")
	}
}
