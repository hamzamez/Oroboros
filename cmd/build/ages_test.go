package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// examples/lines/ages.oro IS ITS FOLD, checked against the fold written in Go
// with fmt.Sscan itself, so the scanning rule is Go's own and not a model of
// it: a line counts when it scans as a name and an integer in 0 … 150, the
// first of the oldest is kept, and the mean is ⌊sum / n⌋.
func agesReference(in string) string {
	n, sum, skipped, bestAge := 0, 0, 0, -1
	bestName := ""
	sc := bufio.NewScanner(strings.NewReader(in))
	for sc.Scan() {
		var name string
		var age int
		if _, err := fmt.Sscan(sc.Text(), &name, &age); err == nil && age >= 0 && age <= 150 {
			n++
			sum += age
			if age > bestAge {
				bestName, bestAge = name, age
			}
		} else {
			skipped++
		}
	}
	var b strings.Builder
	if n == 0 {
		b.WriteString("no people\n")
	} else {
		fmt.Fprintf(&b, "%d people, mean age %d, oldest %s (%d)\n", n, sum/n, bestName, bestAge)
	}
	fmt.Fprintf(&b, "%d lines skipped\n", skipped)
	return b.String()
}

func TestAgesFoldsItsLines(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on the path")
	}
	exe := filepath.Join(t.TempDir(), "ages.exe")
	if err := run("../../targets", "../../examples/lines/ages.oro", "go", exe, "../../lib", false, false, ""); err != nil {
		t.Fatalf("build: %v", err)
	}
	cases := []struct{ name, in string }{
		{"the empty input", ""},
		{"one person", "ana 42\n"},
		{"skipped lines: blank, one field, a bad number, out of range", "ana 42\n\nsolo\ndan x\neve 151\nfay -3\n"},
		{"Go reads 1x as 1, 007 as octal, and ignores a third field", "a 1x\nb 007\nc 88 extra\n"},
		{"the first of the oldest is kept, and UTF-8 names pass", "zoë 30\nbob 88\n李 88\n"},
		{"white space around the fields, and an unterminated last line", "   hal \t 61  \nivy 5"},
	}
	r := rand.New(rand.NewSource(7))
	names := []string{"ana", "zoë", "李", "o'neil", "x"}
	for k := 0; k < 3; k++ {
		var b strings.Builder
		for i := 0; i < 2000; i++ {
			switch p := r.Intn(10); {
			case p < 7:
				fmt.Fprintf(&b, "%s %d\n", names[r.Intn(len(names))], r.Intn(190)-20)
			case p < 9:
				b.WriteString([]string{"", "solo", "a b", "a 1x", "a +12", "a 007", "a 0x1f"}[r.Intn(7)] + "\n")
			default:
				fmt.Fprintf(&b, "\t%s  %d tail\n", names[r.Intn(len(names))], r.Intn(151))
			}
		}
		cases = append(cases, struct{ name, in string }{fmt.Sprintf("random input %d", k), b.String()})
	}
	for _, c := range cases {
		cmd := exec.Command(exe)
		cmd.Stdin = strings.NewReader(c.in)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got, want := stdout.String(), agesReference(c.in); got != want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, want)
		}
	}
}
