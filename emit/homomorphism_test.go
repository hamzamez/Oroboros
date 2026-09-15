package emit

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// LOADING IS A MONOID HOMOMORPHISM, so where a file boundary falls cannot matter.
//
// A layer is the glue of its fragments, ⊔ (target-system.md §2). A file is a
// sequence of forms, and sequences form the free monoid under concatenation. So
// the only reading of a file that agrees with the reading of a directory is the
// homomorphism
//
//	load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂)
//
// and the property that follows is the one a target author relies on without
// knowing it: SPLITTING A FILE IN TWO, OR MERGING TWO, CHANGES NOTHING. Both
// load to the same target, or both are refused.
//
// Parsing a file used to assign each field as it went, so a second
// `(array-type …)` in one file silently REPLACED the first, while the same two
// lines in two files of one directory were refused as a disagreement.
func TestSplittingAFileChangesNothing(t *testing.T) {
	pairs := []struct{ a, b string }{
		// disagreements: glue must refuse them, in one file as in two
		{`(type (array A) (host "[]%s"))`, `(type (array A) (host "%s[]"))`},
		{`(type (map K V) (host "map[%s]%s"))`, `(type (map K V) (host "M<%s,%s>"))`},
		{`(type t (host "a"))`, `(type t (host "b"))`},
		{`(repr (ref int) (host "Long"))`, `(repr (ref int) (host "Integer"))`},
		{`(repr narrow (host "x"))`, `(repr narrow (host "y"))`},
		{`(fact max-len ((a (array A))) (<= (len a) 10))`, `(fact max-len ((a (array A))) (<= (len a) 20))`},
		{`(repr shift 31)`, `(repr shift 63)`},
		{`(repr big host)`, `(repr big limbs)`},
		{`(repr map host)`, `(repr map library)`},
		{`(build "a %s %s")`, `(build "b %s %s")`},
		{`(sig f (int) int pure (host expr "A(%s)"))`, `(sig f (int) int pure (host expr "B(%s)"))`},
		// agreements and ordered data: both must load, to one target
		{`(type t (host "a"))`, `(type t (host "a"))`},
		{`(repr (int 0 255) (host "byte"))`, `(repr (int 0 65535) (host "uint16"))`},
		{`(link "a.lib")`, `(link "b.lib")`},
		{`(implements T I)`, `(implements T J)`},
		{`(sig f (int) int pure (host expr "A(%s)"))`, `(sig g (int) int pure (host expr "B(%s)"))`},
	}
	for i, p := range pairs {
		root := t.TempDir()
		one, two := filepath.Join(root, "one"), filepath.Join(root, "two")
		writeTarget(t, filepath.Join(one, "x"), "a", "(target x "+p.a+" "+p.b+")")
		writeTarget(t, filepath.Join(two, "x"), "a", "(target x "+p.a+")")
		writeTarget(t, filepath.Join(two, "x"), "b", "(target x "+p.b+")")

		whole, errWhole := LoadTargetLayers("x", []string{one})
		split, errSplit := LoadTargetLayers("x", []string{two})
		switch {
		case (errWhole == nil) != (errSplit == nil):
			t.Errorf("%d: %s then %s — one file gives %v, two files give %v; "+
				"a file boundary changed the meaning", i, p.a, p.b, errWhole, errSplit)
		case errWhole == nil && !reflect.DeepEqual(whole, split):
			t.Errorf("%d: %s then %s load to different targets in one file and in two", i, p.a, p.b)
		case errWhole != nil && !strings.Contains(errWhole.Error(), "declared"):
			t.Errorf("%d: refused, but not as a disagreement: %v", i, errWhole)
		}
	}
}
