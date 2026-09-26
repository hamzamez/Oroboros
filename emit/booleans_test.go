package emit

import (
	"testing"
)

// The conditional belongs to the language now, so a target declaring one is an
// error rather than a redundancy.
func TestTargetCannotDeclareTheConditional(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := tg.Prims["if"]; !ok || p.Kind != "cond" {
		t.Errorf("every target should be given `if`, got %+v", p)
	}
	for _, n := range []string{"and", "or", "not", "cond", "if"} {
		if !coreNames[n] {
			t.Errorf("%s should belong to the language", n)
		}
	}
	// And no target file declares booleans any more.
	for _, dir := range []string{"go", "js", "java", "windows"} {
		tg, err := LoadTarget("../targets/" + dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		for _, gone := range []string{"true", "false", "logic.and"} {
			if _, ok := tg.Prims[gone]; ok {
				t.Errorf("%s still declares %s", dir, gone)
			}
		}
	}
}
