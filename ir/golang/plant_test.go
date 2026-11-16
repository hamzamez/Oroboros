package golang

import (
	"strings"
	"testing"
)

// plantOnce is s with its one occurrence of old replaced by new. A planted
// fault whose fragment occurs no times, or twice, fails here, so a rewrite
// that no longer matches its source cannot leave the test passing on the
// unplanted program (assessment-2026-11-16 §3.3: a planted fault asserts that
// it reached its rule).
func plantOnce(t testing.TB, s, old, new string) string {
	t.Helper()
	if n := strings.Count(s, old); n != 1 {
		t.Fatalf("the planted rewrite's fragment %q occurs %d times, not once: restate the fault", old, n)
	}
	return strings.Replace(s, old, new, 1)
}
