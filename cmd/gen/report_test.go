package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"oroboros/ir"
)

// THE REPORT IS D_A ITSELF (ADR 0150's report, hamza: "report is enough"):
// every operation held exactly, each by its source as J writes it, not the
// first four, and an operation a rewrite derived by its own kind in the
// application it came from. Two programs:
//
//  1. six products and sums of S₆₄'s words past the double word, on Go: six
//     sources named, no "…"; PlantReportFirstFour keeps four and the "…";
//  2. chapter 8's ring on JavaScript, its decimals rounded through the pair:
//     the rounding's subtraction and division are named in the rounding,
//     `sub in round(…)`, not `sub`; PlantSrcNotInherited names them bare.
func TestTheReportNamesEveryOperation(t *testing.T) {
	const six = "export fn f(a ∈ -2^63..2^63-1, b ∈ -2^63..2^63-1, c ∈ -2^63..2^63-1) (-6..6) {\n" +
		"    (a * b * c + a * b * a + c * c * c) % 7\n}\n"
	note := func(target, src string) string {
		return reportOf(t, func() { _, _ = surfaceGen(t, target, map[string]string{"p.oro": src}) })
	}
	got := note("go", six)
	line := lineWith(got, "held exactly:")
	if line == "" || strings.Contains(line, "…") {
		t.Fatalf("six operations past E on Go: want each named, got %q", line)
	}
	n := strings.Count(line[strings.Index(line, "held exactly:"):], ";") + 1
	if !strings.Contains(line, ": "+itoa(n)+" operation(s)") {
		t.Errorf("the report counts one number and names another: %q", line)
	}
	if n < 5 {
		t.Errorf("want more than four operations named, so the first four's cut can show: %q", line)
	}
	ir.PlantReportFirstFour = true
	planted := lineWith(note("go", six), "held exactly:")
	ir.PlantReportFirstFour = false
	if !strings.Contains(planted, "…") {
		t.Errorf("with the first four kept, the report still names every operation: %q", planted)
	}

	ring, err := os.ReadFile("../../docs/book/code/numbers/ring.oro")
	if err != nil {
		t.Fatal(err)
	}
	got = lineWith(note("js", string(ring)), "held exactly:")
	if !strings.Contains(got, "sub in round(") || !strings.Contains(got, "div in round(") {
		t.Errorf("ring on JavaScript: want the rounding's operations named in it, got %q", got)
	}
	ir.PlantSrcNotInherited = true
	planted = lineWith(note("js", string(ring)), "held exactly:")
	ir.PlantSrcNotInherited = false
	if !strings.Contains(planted, "held exactly: sub; div") {
		t.Errorf("with the provenance not inherited, ring's operations are still named in their source: %q", planted)
	}
}

// reportOf is what gen printed to standard error while f ran.
func reportOf(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	f()
	os.Stderr = saved
	w.Close()
	return <-done
}

func lineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
