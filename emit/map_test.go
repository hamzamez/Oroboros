package emit

import (
	"testing"
)

// A MAP TYPE RESOLVES THROUGH ONE DECLARATION, which is `array`'s argument one
// constructor over. The alternative is an entry per (K, V) pair —
// `targets/java/util.oro` calls that "the same limitation squared", having had
// to declare Map<String,Long> and nothing else.
func TestMapTypeResolvesThroughOneDeclaration(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.ty("map int int"); got != "map[int]int" {
		t.Errorf("map int int spelled %q, want map[int]int", got)
	}
	// A value type that is itself compound must survive the split. K may not be
	// compound — it is restricted to what `=` decides — so splitting on the
	// first space is unambiguous.
	if got := tg.ty("map int array f64"); got != "map[int][]float64" {
		t.Errorf("map int array f64 spelled %q, want map[int][]float64", got)
	}
}
