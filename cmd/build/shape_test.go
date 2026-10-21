package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shapeReference is examples/lines/shape.oro's definition, written from the
// factorization and not from bufio: x split at every LF is c₁…c_k u with the
// delimiters removed, the last part u a record when it is not empty, and a
// record's fields its SEP count plus one.
func shapeReference(x []byte, sep byte) string {
	parts := bytes.Split(x, []byte{'\n'})
	k := len(parts) - 1
	recs := parts[:k]
	open := len(parts[k]) > 0
	if open {
		recs = parts
	}
	f, rag, ri, rf, long := 0, 0, 0, 0, 0
	for i, r := range recs {
		fc := bytes.Count(r, []byte{sep}) + 1
		if i == 0 {
			f = fc
		} else if fc != f {
			if rag == 0 {
				ri, rf = i+1, fc
			}
			rag++
		}
		long = max(long, len(r))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "records %d\nfields %d\n", len(recs), f)
	if rag > 0 {
		fmt.Fprintf(&b, "ragged %d, the first record %d with %d fields\n", rag, ri, rf)
	} else {
		b.WriteString("ragged 0\n")
	}
	fmt.Fprintf(&b, "longest %d\n", long)
	if open {
		b.WriteString("the last record has no final LF\n")
	}
	return b.String()
}

// THE PROGRAM IS ITS DEFINITION on the factorization's edges: no record, an
// empty one, an unterminated last one, CR kept as a byte, SEP at a record's ends,
// and records at and around the 4,096 and 65,536 bytes a read may stop at: the
// counts are homomorphisms, so where the reads cut the input is invisible.
func TestShapeIsTheFoldOverTheFactorization(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on the path")
	}
	exe := filepath.Join(t.TempDir(), "shape.exe")
	if err := run("../../targets", "../../examples/lines/shape.oro", "go", exe, "../../lib", false, false, ""); err != nil {
		t.Fatalf("build: %v", err)
	}
	row := func(n int) string { return strings.Repeat("a", n-2) + "\tb" }
	for _, c := range []struct {
		name, in string
		sep      byte
	}{
		{"the empty input has no record", "", '\t'},
		{"one LF is one empty record, of one field", "\n", '\t'},
		{"a final LF ends a record and begins none", "a\tb\nc\td\n", '\t'},
		{"an unterminated last record", "a\tb\nc\td", '\t'},
		{"ragged records, the first one named", "a\tb\tc\nd\te\tf\ng\th\n\nlong\tx\ty\tz", '\t'},
		{"CR is a byte of the record", "a\tb\r\nc\td\r\n", '\t'},
		{"SEP at both ends makes empty fields", "\ta\t\n\t\t\t\n", '\t'},
		{"another SEP", "a,b,c\nd,e\n", ','},
		{"bytes that are not UTF-8", "\xff\t\xfe\n\xc3\n", '\t'},
		{"a record of 4,095 bytes and its LF fill the buffer exactly", row(4095) + "\nx\ty\n", '\t'},
		{"a record of 4,096 bytes, the LF past the buffer", row(4096) + "\nx\ty\n", '\t'},
		{"a record of 4,097 bytes", row(4097) + "\nx\n", '\t'},
		{"a record of 10,000 bytes across three buffers, unterminated", "x\ty\n" + row(10000), '\t'},
		{"a record of 65,535 bytes and its LF fill the program's buffer exactly", row(65535) + "\nx\ty\n", '\t'},
		{"a record of 65,536 bytes, the LF past the program's buffer", row(65536) + "\nx\ty\n", '\t'},
		{"a record of 65,537 bytes", row(65537) + "\nx\n", '\t'},
		{"a record of 200,000 bytes across four reads, between short ones", "a\tb\n" + row(200000) + "\nc\n", '\t'},
	} {
		want := shapeReference([]byte(c.in), c.sep)
		args := []string{}
		if c.sep != '\t' {
			args = append(args, string(c.sep))
		}
		cmd := exec.Command(exe, args...)
		cmd.Stdin = strings.NewReader(c.in)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: %v (stderr %q)", c.name, err, stderr.String())
		}
		if got := stdout.String(); got != want {
			t.Errorf("%s:\ngot  %q\nwant %q", c.name, got, want)
		}
	}
	for _, args := range [][]string{{"ab"}, {"\n"}, {""}, {",", ","}} {
		cmd := exec.Command(exe, args...)
		cmd.Stdin = strings.NewReader("a\n")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		err := cmd.Run()
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 || stdout.Len() != 0 {
			t.Errorf("shape %q: %v, stdout %q; want status 2 and nothing on standard output", args, err, stdout.String())
		}
	}
}
