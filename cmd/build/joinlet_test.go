package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
)

// AN `again` AFTER A JOIN POINT IS A BACK EDGE OF THE ENCLOSING LOOP (ADR 0139):
// `(a, b) = loop … { … }; again(…)` is a `let` of two names, and every walker of
// a clause chain reads its body as a tail. The program is accepted and right
// on Go and windows. Its back edge counts in every proof: an index kept at 0 by
// one `again` and driven to −4 by the join body's is refused, where it panicked
// at run time with the pattern unread. The fault PlantJoinLetUnread leaves the
// pattern unread by the walkers, and the monotonicity proof then certifies
// 0 <= k past the edge it did not see: the program builds, as it did with
// ADR 0134's refusal merely switched off (joinlet-2026-11-19).
func TestAnAgainAfterAJoinPointIsABackEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("builds Go and windows programs")
	}
	nested := `fn run(n ∈ 0..100) (Int) {
    loop j = 0, s = 0 {
        j >= n -> s
        else -> {
            (a, b) = loop i = 0, t = 0 { i >= 3 -> (i, t); else -> again(i + 1, t + j) }
            again(j + 1, s + a + b)
        }
    }
}
`
	drivers := map[string][2]string{
		"windows": {"use win/print\n", "export fn main() { print.print_int(run(10)); print.print_int(run(0)); print.print_int(run(100)) }\n"},
		"go":      {"use go/fmt\n", "export fn main() { fmt.Println(run(10)); fmt.Println(run(0)); fmt.Println(run(100)) }\n"},
	}
	for target, d := range drivers {
		got, err := runOnMayFail(t, d[0]+nested+d[1], target)
		if want := "165 0 15150"; err != nil || got != want {
			t.Errorf("%s: the nested loops gave %q, %v; want %q", target, got, err, want)
		}
	}
	// the join body drives k from 1 to −4; the other back edge keeps it ≥ 0
	negative := `use go/fmt

fn run(n ∈ 0..100) (Int) {
    var t := array { 7 : _ ∈ 0..9 }
    loop j = 0, k = 0, s = 0 {
        j >= n -> s
        k >= 9 -> s
        j % 2 == 0 -> again(j + 1, k + 1, s + t[k])
        else -> {
            (a, b) = loop i = 0 { i >= 2 -> (i, i); else -> again(i + 1) }
            again(j + 1 + a - b, k - 5, s)
        }
    }
}

export fn main() { fmt.Println(run(5)) }
`
	if _, err := runOnMayFail(t, negative, "go"); err == nil || !strings.Contains(err.Error(), "0 <= k") {
		t.Errorf("an index the join body drives below 0 should be refused naming 0 <= k, got %v", err)
	}
	emit.PlantJoinLetUnread = true
	_, err := buildOnly(t, negative, "go")
	emit.PlantJoinLetUnread = false
	if err != nil {
		t.Errorf("with the pattern unread the walkers should mis-prove 0 <= k and build it, which is the fault; got %v", err)
	}
}

// buildOnly builds src for target and does not run it.
func buildOnly(t *testing.T, src, target string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	return exe, run("../../targets", path, target, exe, "../../lib", false, false, "")
}
