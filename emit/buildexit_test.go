package emit_test

import "testing"

// A BUILD'S VALUE CARRIES THE ELEMENT ITS STORES SOLVED (types.md §3.1,
// sumofsums-2026-10-06). A clause chain lists its exits first, so the loop
// filling a buffer returned it before the store that solved its open element
// was typed, and the build froze to `array ?`. The table was then typed by
// whichever read the checker met first: here strings.Compare, whose parameter
// is `bytestring`, before `concat`, which needs `string`. The loop's exits are
// now typed with the variables' solved types, so the table is `array string`
// whatever order its reads come in.
func TestABuildCarriesTheElementItsStoresSolved(t *testing.T) {
	_, err := entryGo(t, `use go
use go/strings
export fn f(n ∈ 1..10) (String) {
    cs = {
        var b := array { 0 : _ ∈ 0..n-1 }
        loop i = 0 { i >= n -> b; else -> { b[i] := "x"; again(i + 1) } }
    }
    if strings.Compare(cs[0], cs[0]) == 0 { cs[0] ++ "!" } else { "" }
}`)
	if err != nil {
		t.Errorf("a buffer filled with strings froze to a table of strings: %v", err)
	}
}
