package gauntlet

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

// Gauntlet program 5, generated against the hand-written baseline in
// gauntlet.go. This is the first gauntlet program with an effect, so the pass
// condition is behavioural before it is numerical: three lines, in order, once
// each, byte-identical to what a person would write.
//
// The generated form calls fmt.Println rather than taking a writer, because the
// language has no notion of a writer and `print-line` is a Tier 2 binding to the
// host's own output (docs/spec/effects.md §6). So the comparison redirects
// os.Stdout.

func capture(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		io.Copy(&b, r)
		done <- b.String()
	}()
	f()
	w.Close()
	os.Stdout = old
	return <-done
}

func TestGeneratedReportMatchesHandWritten(t *testing.T) {
	var want bytes.Buffer
	Report(&want, "totals", vecA)

	got := capture(t, func() { GenReport("totals", vecA) })

	if got != want.String() {
		t.Errorf("output differs\n got: %q\nwant: %q", got, want.String())
	}
}

// Each effect must happen exactly once per source occurrence. A reducer that
// duplicated, dropped, or reordered would show up here as a line count or an
// ordering difference — which is what effects.md §4 is defending, and what six
// pure programs could not have detected.
func TestGeneratedReportPrintsEachLineOnce(t *testing.T) {
	out := capture(t, func() { GenReport("totals", vecA) })

	var lines []string
	for _, l := range bytes.Split([]byte(out), []byte("\n")) {
		if len(l) > 0 {
			lines = append(lines, string(l))
		}
	}
	if len(lines) != 3 {
		t.Fatalf("expected exactly 3 lines, got %d: %q", len(lines), lines)
	}
	var acc float64
	for _, x := range vecA {
		acc += x * x
	}
	for i, want := range []string{"totals", fmt.Sprint(len(vecA)), fmt.Sprint(acc)} {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i+1, lines[i], want)
		}
	}
}

// Parity against the hand-written form. The interesting quantity is not the
// clock — fmt dominates both — but whether the *pure* part still fused. If an
// effect had blocked fusion the generated form would build an intermediate
// vector and allocate, which ReportAllocs would show.
func BenchmarkG5GenReport(b *testing.B) {
	b.ReportAllocs()
	old := os.Stdout
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Skip(err)
	}
	os.Stdout = null
	defer func() { os.Stdout = old; null.Close() }()
	for i := 0; i < b.N; i++ {
		GenReport("label", vecA)
	}
}
