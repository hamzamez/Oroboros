package stdlib

import (
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"oroboros/emit"
)

// A RESULT IS OURS ONLY WHERE THE HOST MAPS EVERY INPUT INTO V (ADR 0030).
//
// A Go string is B*, any bytes; ours is Σ*, held as V = enc(Σ*) ⊆ B*. A hand
// declaration may give a result as `string` only when the host provably lands
// in V on every input, and the checker lets that through by design, since it is
// the reader's claim (agreeOne). So the claim gets a witness: each
// go/strings declaration whose result is ours is run, as Go's own function, on
// every string of up to three bytes over an alphabet of invalid, surrogate and
// case-mapped bytes, and 200,000 random ones, and must never leave V
// (gotarget-2026-09-30). The set is read from the file, so a declaration made
// ours without a witness here fails; and a control that CAN leave V must.
func TestStringsResultsDeclaredOursLandInUTF8(t *testing.T) {
	tg, err := emit.LoadTarget(filepath.Join(root, "targets", "go"))
	if err != nil {
		t.Fatal(err)
	}
	var ours []string
	for name, p := range tg.Prims {
		if short, ok := strings.CutPrefix(name, "go/strings."); ok && p.Result == "string" {
			ours = append(ours, short)
		}
	}
	sort.Strings(ours)

	witnessed := map[string]func(string) string{
		"ToLower":     strings.ToLower,
		"ToUpper":     strings.ToUpper,
		"ToTitle":     strings.ToTitle,
		"Title":       strings.Title, //nolint:staticcheck // deprecated, and still declared
		"ToValidUTF8": func(s string) string { return strings.ToValidUTF8(s, "�") },
	}
	for _, n := range ours {
		if witnessed[n] == nil {
			t.Errorf("go/strings.%s is declared with OUR string as its result and has no witness here", n)
		}
	}
	for n := range witnessed {
		found := false
		for _, o := range ours {
			found = found || o == n
		}
		if !found {
			t.Errorf("%s is witnessed here and no longer declared ours: drop the witness", n)
		}
	}

	inputs := sigmaInputs()
	for n, f := range witnessed {
		for _, s := range inputs {
			if out := f(s); !utf8.ValidString(out) {
				t.Errorf("strings.%s(%q) = %q, which is not valid UTF-8", n, s, out)
				break
			}
		}
	}
	// THE CONTROL: a function that keeps invalid bytes must be caught leaving V,
	// or the inputs test nothing.
	left := 0
	for _, s := range inputs {
		if !utf8.ValidString(strings.TrimSpace(s)) {
			left++
		}
	}
	if left == 0 {
		t.Error("TrimSpace never left V on these inputs: they cannot tell a map into V from any other")
	}
}

func sigmaInputs() []string {
	alpha := []byte{0x00, 'a', 'A', 'z', ' ', 0x7F, 0x80, 0xBF, 0xC2, 0xC3, 0x84, 0xE0, 0xED, 0xA0,
		0xF0, 0x90, 0xF4, 0xFF, 0xCE, 0xA3, 0xC4, 0xB0}
	var out []string
	var rec func(p []byte, n int)
	rec = func(p []byte, n int) {
		out = append(out, string(p))
		if n == 0 {
			return
		}
		for _, b := range alpha {
			rec(append(append([]byte{}, p...), b), n-1)
		}
	}
	rec(nil, 3)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 200000; i++ {
		b := make([]byte, r.Intn(12))
		for j := range b {
			b[j] = byte(r.Intn(256))
		}
		out = append(out, string(b))
	}
	return out
}
