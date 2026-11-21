package emit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AND, OR AND XOR ON JAVASCRIPT'S WHOLE WORD ARE THE SPLIT'S (integers.md §0b,
// pairsel-2026-11-21). The bit operations act bit by bit, so they commute with
// x = h·2³² + l, l = ToUint32(x), h = (x − l)/2³²: x ∘ y = (hₓ ∘ h_y)·2³² +
// (lₓ ∘ l_y). The target's own templates, `js.and-s53`, `js.or-s53` and
// `js.xor-s53`, are run in node over W's edges and a sweep of every bit length
// against BigInt, whose operators are ℤ's: every result is compared, −2⁵³ (one
// past W, which the result set leaves to the facts) included. THE PLANTED
// FAULT: the and with its high digits or'ed must disagree, so the comparison
// can fail.
func TestTheBitOperationsOnTheWholeWordAreTheSplits(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	fn := func(form string) string {
		return "(x, y) => " + strings.Replace(strings.Replace(form, "%s", "x", 1), "%s", "y", 1)
	}
	forms := map[string]string{}
	for _, op := range []string{"and", "or", "xor"} {
		p, ok := tg.Prims["js."+op+"-s53"]
		if !ok || strings.Count(p.Form, "%s") != 2 {
			t.Fatalf("js.%s-s53 is not a two-argument template: %q", op, p.Form)
		}
		forms[op] = fn(p.Form)
	}
	high := "return (((x - p) / 4294967296) & ((y - q) / 4294967296))"
	if !strings.Contains(forms["and"], high) {
		t.Fatal("js.and-s53 no longer ands its high digits as written here: restate the planted fault")
	}
	planted := strings.Replace(forms["and"], high, "return (((x - p) / 4294967296) | ((y - q) / 4294967296))", 1)
	script := func(and string) string {
		return fmt.Sprintf(`const ops = [['and', %s, (a, b) => a & b], ['or', %s, (a, b) => a | b], ['xor', %s, (a, b) => a ^ b]];
const W = 2 ** 53 - 1;
const vals = [0, 1, -1, 2, -2, W, -W, W - 1, -W + 1, 2 ** 32, -(2 ** 32), 2 ** 32 - 1, -(2 ** 32) + 1,
  2 ** 31, -(2 ** 31), 2 ** 31 - 1, 2 ** 52, -(2 ** 52), 2 ** 40 - 1, 2 ** 52 + 7, -(2 ** 52) + 3];
for (let k = 1; k <= 53; k++) { const m = 2 ** k - 1; vals.push(m, -m, Math.floor(m / 3), -Math.floor(m / 5)); }
let n = 0, bad = 0;
for (const x of vals) for (const y of vals) for (const [name, f, g] of ops) {
  n++;
  const got = f(x, y);
  if (!Number.isInteger(got) || BigInt(got) !== g(BigInt(x), BigInt(y))) { bad++; }
}
console.log(n + ' ' + bad);
`, and, forms["or"], forms["xor"])
	}
	run := func(src string) (int, int) {
		path := filepath.Join(t.TempDir(), "split.mjs")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(node, path).CombinedOutput()
		if err != nil {
			t.Fatalf("node: %v\n%s", err, out)
		}
		var n, bad int
		if _, err := fmt.Sscanf(string(out), "%d %d", &n, &bad); err != nil {
			t.Fatalf("node printed %q", out)
		}
		return n, bad
	}
	if n, bad := run(script(forms["and"])); bad != 0 || n < 3*200*200 {
		t.Errorf("the split disagrees with ℤ on %d of %d", bad, n)
	} else {
		t.Logf("%d results, each ℤ's", n)
	}
	if _, bad := run(script(planted)); bad == 0 {
		t.Error("with its high digits or'ed, the and still agrees with ℤ: the comparison cannot fail")
	}
}
