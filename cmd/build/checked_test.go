package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// runChecked builds src with -checked for target and runs it, answering what
// it printed and whether it stopped (a trap: a panic, an exception, an illegal
// instruction). A build that fails is the test's failure, unless refuse.
func runChecked(t *testing.T, src, target string) (out string, trapped bool, buildErr error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	art := map[string]string{"go": "p.exe", "windows": "p.exe", "js": "p.mjs", "java": "classes"}[target]
	art = filepath.Join(dir, art)
	if err := run("../../targets", path, target, art, "../../lib", false, true, ""); err != nil {
		return "", false, err
	}
	var cmd *exec.Cmd
	switch target {
	case "js":
		cmd = program("node", art)
	case "java":
		cmd = program("java", "-cp", art, "Main")
	default:
		cmd = program(art)
	}
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err != nil, nil
}

// checkedProgram is a loop applying step k times to z, then printing the
// result through last.
func checkedProgram(z, step string, k int, last string) string {
	return "use io\n\nexport fn main() {\n" +
		"    p = loop i = 0, p = " + z + " {\n" +
		"        i < " + itoa(k) + " -> again(i + 1, " + step + ")\n" +
		"        else -> p\n    }\n" +
		"    io.print_int(" + last + ")\n}\n"
}

// -CHECKED STOPS WHERE THE PROOF FAILED AND THE VALUE LEFT THE WORD, on every
// target (ADR 0019; the book's finding 93). Each operation that can leave a
// word is run one step inside it and one step past it, on Go, the JVM and
// windows, whose word is S₆₄, and JavaScript's ±(2⁵³−1): inside, the answer is
// ℤ's; past, the program stops and prints nothing. Before, JavaScript declared
// no checked form and Go and the JVM none for `<<` and negation, so the
// decision left such an operation `exact` and the build called it a trap:
// 3^128 printed 1.18 · 10⁶¹ on JavaScript and 3 << 69 printed 0 on Go.
func TestCheckedStopsOnEveryTarget(t *testing.T) {
	for _, tool := range []string{"node", "javac", "java"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on the path", tool)
		}
	}
	const trap = "trap"
	cases := []struct {
		name string
		src  string
		want map[string]string // target → its answer, or trap
	}{
		{"3^32, the squares inside both words", checkedProgram("3", "p * p", 5, "p"),
			map[string]string{"go": "1853020188851841", "js": "1853020188851841", "java": "1853020188851841", "windows": "1853020188851841"}},
		{"3^64, past both", checkedProgram("3", "p * p", 6, "p"),
			map[string]string{"go": trap, "js": trap, "java": trap, "windows": trap}},
		{"3 · 2⁵¹, a shift inside both", checkedProgram("3", "p << 1", 51, "p"),
			map[string]string{"go": "6755399441055744", "js": "6755399441055744", "java": "6755399441055744", "windows": "6755399441055744"}},
		{"3 · 2⁶¹, inside S₆₄ and past JavaScript's word", checkedProgram("3", "p << 1", 61, "p"),
			map[string]string{"go": "6917529027641081856", "js": trap, "java": "6917529027641081856", "windows": "6917529027641081856"}},
		{"3 · 2⁶², past both", checkedProgram("3", "p << 1", 62, "p"),
			map[string]string{"go": trap, "js": trap, "java": trap, "windows": trap}},
		{"~(2⁵²−1), inside both", checkedProgram("0", "p * 2 + 1", 52, "~p"),
			map[string]string{"go": "-4503599627370496", "js": "-4503599627370496", "java": "-4503599627370496", "windows": "-4503599627370496"}},
		{"~(2⁵³−1) = −2⁵³, past JavaScript's word only", checkedProgram("0", "p * 2 + 1", 53, "~p"),
			map[string]string{"go": "-9007199254740992", "js": trap, "java": "-9007199254740992", "windows": "-9007199254740992"}},
		{"−(−2⁶²)", checkedProgram("-1", "p * 2", 62, "-p"),
			map[string]string{"go": "4611686018427387904", "js": trap, "java": "4611686018427387904", "windows": "4611686018427387904"}},
		{"−(−2⁶³), the one negation past S₆₄", checkedProgram("-1", "p * 2", 63, "-p"),
			map[string]string{"go": trap, "js": trap, "java": trap, "windows": trap}},
	}
	for _, c := range cases {
		for _, target := range []string{"go", "js", "java", "windows"} {
			out, trapped, err := runChecked(t, c.src, target)
			if err != nil {
				t.Errorf("%s on %s: refused: %v", c.name, target, err)
				continue
			}
			got := out
			if trapped {
				got = trap
				if out != "" {
					got = "trap after printing " + out
				}
			}
			if got != c.want[target] {
				t.Errorf("%s on %s: %s, want %s", c.name, target, got, c.want[target])
			}
		}
	}
	// A HOST'S OWN NEGATION: the language's −p is 0 − p, so a negation is a
	// program naming one, `java.neg`, whose checked form is Math.negateExact.
	jneg := func(k int) string {
		return strings.Replace(checkedProgram("-1", "p * 2", k, "java.neg(p)"), "use io\n", "use io\nuse java\n", 1)
	}
	for k, want := range map[int]string{62: "4611686018427387904", 63: trap} {
		out, trapped, err := runChecked(t, jneg(k), "java")
		if got := map[bool]string{true: trap, false: out}[trapped]; err != nil || got != want {
			t.Errorf("java.neg after %d doublings of −1: %q, trapped %v, %v; want %s", k, out, trapped, err, want)
		}
	}
	// LEMMA C'S FAULT: negation claimed closed on S₆₄. The JVM then prints
	// −(−2⁶³) as −2⁶³, its wrapped answer, where the case wants the stop.
	ir.PlantNegClosed = true
	out, trapped, err := runChecked(t, jneg(63), "java")
	ir.PlantNegClosed = false
	if err != nil || trapped {
		t.Errorf("with −a claimed closed the JVM's −(−2⁶³) is %q, trapped %v, %v: Lemma C's case cannot fail", out, trapped, err)
	}
}

// AN OPERATION NEITHER PROVEN NOR TRAPPED IS REFUSED under -checked, naming
// it (ir.CheckedRefusal). The faults: no target with a checked form, the state
// JavaScript shipped in, must be refused; and with the refusal planted away
// too, the bug as it shipped, the program must build and answer wrongly, or
// this case could not fail.
func TestAnUntrappedOperationIsRefused(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not on the path")
	}
	src := checkedProgram("3", "p * p", 7, "p")
	ir.PlantNoCheckedForm = true
	_, _, err := runChecked(t, src, "js")
	if err == nil || !strings.Contains(err.Error(), "neither proven nor trapped") || !strings.Contains(err.Error(), "p * p") {
		t.Errorf("with no checked form the untrapped p * p is %v, want a refusal naming it", err)
	}
	ir.PlantUntrappedAccepted = true
	out, trapped, err := runChecked(t, src, "js")
	ir.PlantUntrappedAccepted, ir.PlantNoCheckedForm = false, false
	if err != nil || trapped || out == "" {
		t.Errorf("with the refusal planted away the program is %q, trapped %v, %v: the case cannot fail", out, trapped, err)
	}
}
