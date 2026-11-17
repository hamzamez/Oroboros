package llvm

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A TABLE'S POINTER IS A REGION'S (φ's third kind; tableAttr). A table or
// string at a function's boundary is printed `nonnull dereferenceable(8)
// align 8`: every such value is a region's address, which holds at least its
// 8-byte length, or a string literal's global. The witness reads the printed
// text:
//   - a table parameter takes the attribute, an integer none;
//   - every `dereferenceable(n)` in it claims n ≤ 8, the least region's
//     length word (a table of no elements holds 8 bytes and its zero byte).
//
// THE PLANTED FAULT claims 4096 bytes, more than a short table's region, and
// the second assertion must catch it.
func TestATablesPointerIsARegions(t *testing.T) {
	const src = "export fn f(t ∈ Array(0..255), k ∈ 0..9) (Int) { len(t) + k }\n"
	deref := regexp.MustCompile(`dereferenceable\((\d+)\)`)
	claims := func() []int {
		t.Helper()
		got, err := compile(t, src)
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`ptr nonnull dereferenceable\(\d+\) align 8 %v`).MatchString(got) {
			t.Errorf("the table parameter takes no attribute:\n%s", got)
		}
		if regexp.MustCompile(`i64 (range\([^)]*\) )?nonnull`).MatchString(got) {
			t.Errorf("an integer took a pointer's attribute:\n%s", got)
		}
		var ns []int
		for _, m := range deref.FindAllStringSubmatch(got, -1) {
			n, _ := strconv.Atoi(m[1])
			ns = append(ns, n)
		}
		return ns
	}
	for _, n := range claims() {
		if n > 8 {
			t.Errorf("dereferenceable(%d) claims more than a region's length word", n)
		}
	}
	PlantTableAttrWide = true
	wide := claims()
	PlantTableAttrWide = false
	over := false
	for _, n := range wide {
		over = over || n > 8
	}
	if !over {
		t.Errorf("with the fault planted some claim should pass 8 bytes, so the test reaches the rule: %v", wide)
	}
}

// NOALIAS IS A PARAMETER'S, NEVER A RESULT'S (φ's fourth kind). On a result,
// noalias says the pointer is fresh, aliasing nothing the caller holds; an
// export that writes its lent buffer and hands it back returns the caller's
// own pointer. So the buffer parameter takes noalias and the result does not.
// THE PLANTED FAULT is the bug the gauntlet's smooth_into showed first:
// noalias on the result as well.
func TestNoaliasIsAParametersNeverAResults(t *testing.T) {
	// the gauntlet's stencil, whose export smooth_into writes its lent dst and
	// returns it (gauntlet/regen.txt's windows line)
	text, err := os.ReadFile("../../examples/native/smooth-go.oro")
	if err != nil {
		t.Fatal(err)
	}
	// smooth_into the one export, so it is the function compile prints
	src := strings.Replace(string(text), "export fn smooth(", "fn smooth(", 1)
	if src == string(text) {
		t.Fatal("smooth-go.oro no longer exports smooth: restate the witness")
	}
	header := func() string {
		t.Helper()
		got, err := compile(t, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(got, "\n") {
			if strings.HasPrefix(l, "define") {
				return l
			}
		}
		t.Fatalf("no definition printed:\n%s", got)
		return ""
	}
	resultNoalias := regexp.MustCompile(`^define (internal )?noalias`)
	h := header()
	if !strings.Contains(h, "ptr noalias nonnull") || resultNoalias.MatchString(h) {
		t.Errorf("the lent buffer should be noalias and the result not: %s", h)
	}
	PlantNoaliasResult = true
	h = header()
	PlantNoaliasResult = false
	if !resultNoalias.MatchString(h) {
		t.Errorf("with the fault planted the result should take noalias, so the test reaches the rule: %s", h)
	}
}
