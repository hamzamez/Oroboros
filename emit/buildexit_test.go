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
	_, err := entryGo(t, `(use go)
(use go/strings as strings)
(export f)
(sig f ((n (int 1 10))) string)
(def f (n)
  (let cs (local b (table n 0) (loop ((b b) (i 0))
                       (>= i n) b
                       else     (again (set b i "x") (+ i 1))))
    (if (= (strings.Compare (cs 0) (cs 0)) 0) (concat (cs 0) "!") "")))`)
	if err != nil {
		t.Errorf("a buffer filled with strings froze to a table of strings: %v", err)
	}
}
