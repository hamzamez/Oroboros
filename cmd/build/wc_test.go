package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// wcReference is examples/os/wc.oro's definition over the WHOLE input, not by
// reads: L the LFs, C the bytes, M the runes of d(x), Go's DecodeRune decoding,
// and W the runs of runes outside White_Space, each counted where it begins.
func wcReference(x []byte) (l, w, m, c int) {
	c = len(x)
	in := false
	for k := 0; k < len(x); {
		r, sz := utf8.DecodeRune(x[k:])
		if r == '\n' {
			l++
		}
		sp := unicode.IsSpace(r)
		if !sp && !in {
			w++
		}
		in = !sp
		m++
		k += sz
	}
	return
}

func wcLine(l, w, m, c int, name string) string {
	return fmt.Sprintf("%7d %7d %7d %7d %s\n", l, w, m, c, name)
}

// THE PROGRAM IS ITS DEFINITION, whatever the reads were: the counts of d(x)
// are a function of x, the transducer's state across a read is the tail of
// fewer than 4 bytes, and so every input below, read as a file and through a
// pipe, gives the whole input's counts. The inputs sit on the edges: White_Space
// and its near misses, bytes that begin no codeword, a codeword cut by the end of
// the input, and a codeword of each length straddling the program's 65,536-byte
// read at each offset.
func TestWcIsItsDefinitionWhateverTheReads(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on the path")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "wc.exe")
	if err := run("../../targets", "../../examples/os/wc.oro", "go", exe, "../../lib", false, false, ""); err != nil {
		t.Fatalf("build: %v", err)
	}
	cases := []struct{ name, in string }{
		{"empty", ""},
		{"one word", "abc"},
		{"words and lines", "a b\tc\n  d  e\n"},
		{"spaces at both ends", "  \t a  \n"},
		{"NBSP, NEL and the ideographic space are White_Space", "a\u00a0b\u0085c\u3000d"},
		{"U+200B and U+FEFF are not", "a\u200bb\ufeffc"},
		{"vertical tab, form feed, CR", "a\vb\fc\rd"},
		{"bytes that begin no codeword", "\xff\xfe a \x80\xbf\n"},
		{"an invalid second byte", "\xe0\x80\x80 x"},
		{"a codeword cut by the end", "ab \xe2\x82"},
		{"a lone lead byte at the end", "ab\xf0"},
		{"multibyte runes", "héllo wörld 日本語 🙂\n"},
	}
	for _, lead := range []int{65533, 65534, 65535, 65536, 65537} {
		for _, r := range []string{"é", "€", "🙂", "\u3000"} {
			cases = append(cases, struct{ name, in string }{
				fmt.Sprintf("%q at byte %d", r, lead), strings.Repeat("a", lead) + r + "b c\n"})
		}
	}
	cases = append(cases, struct{ name, in string }{"300,000 bytes of 3-byte runes and spaces",
		strings.Repeat("日本 語", 30000)})
	for _, c := range cases {
		want := wcLine(wcFour(c.in, ""))
		// As a file.
		p := filepath.Join(dir, "in.txt")
		if err := os.WriteFile(p, []byte(c.in), 0o644); err != nil {
			t.Fatal(err)
		}
		got, status := wcRun(t, exe, "", p)
		if status != 0 || got != wcLine(wcFour(c.in, p)) {
			t.Errorf("%s, a file: status %d\ngot  %q\nwant %q", c.name, status, got, wcLine(wcFour(c.in, p)))
		}
		// Through a pipe, which reads in other pieces.
		if got, status := wcRun(t, exe, c.in); status != 0 || got != want {
			t.Errorf("%s, standard input: status %d\ngot  %q\nwant %q", c.name, status, got, want)
		}
	}
	// Several files and their total; one that will not open is named on standard
	// error, skipped, and the status is 1.
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("one two\n"), 0o644)
	os.WriteFile(b, []byte("three\u00a0four five\n"), 0o644)
	missing := filepath.Join(dir, "none.txt")
	la, wa, ma, ca := wcReference([]byte("one two\n"))
	lb, wb, mb, cb := wcReference([]byte("three\u00a0four five\n"))
	want := wcLine(la, wa, ma, ca, a) + wcLine(lb, wb, mb, cb, b) + wcLine(la+lb, wa+wb, ma+mb, ca+cb, "total")
	cmd := exec.Command(exe, a, missing, b)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	ee, isExit := err.(*exec.ExitError)
	if !isExit || ee.ExitCode() != 1 || stdout.String() != want || !strings.Contains(stderr.String(), "none.txt: cannot open") {
		t.Errorf("a missing file among two: %v, stderr %q\ngot  %q\nwant %q", err, stderr.String(), stdout.String(), want)
	}
}

func wcFour(in, name string) (int, int, int, int, string) {
	l, w, m, c := wcReference([]byte(in))
	return l, w, m, c, name
}

// wcRun runs exe with stdin and args, and returns its standard output and status.
func wcRun(t *testing.T, exe, stdin string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return stdout.String(), ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run: %v (stderr %q)", err, stderr.String())
	}
	return stdout.String(), 0
}
