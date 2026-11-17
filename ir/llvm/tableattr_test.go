package llvm

import (
	"regexp"
	"strconv"
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
