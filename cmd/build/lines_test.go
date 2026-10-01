package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// examples/lines/lines.oro IS THE FOLD OF ITS STEP OVER λ(x), and this checks
// it against λ written from bufio.oro's definition, not from bufio.
//
// x = u₀ LF u₁ LF … LF u_k; the tokens are c(u₀), …, c(u_{k−1}), and c(u_k)
// when u_k is not empty, c dropping one trailing CR. A u of 65,536 bytes or
// more ends the scan with an error (bufio.oro, THE BOUND), and what was
// numbered before it has been written.
func linesReference(x []byte, col int) (out string, status int) {
	const capValue = 100000000000
	parts := bytes.Split(x, []byte{'\n'})
	k := len(parts) - 1
	var b strings.Builder
	n, used, sum := 0, 0, 0
	for i, u := range parts {
		if i == k && len(u) == 0 {
			break
		}
		if len(u) >= 65536 {
			return b.String(), 1
		}
		if len(u) > 0 && u[len(u)-1] == '\r' {
			u = u[:len(u)-1]
		}
		n++
		fmt.Fprintf(&b, "%6d\t%s\n", n, u)
		fs := strings.Fields(string(u))
		if len(fs) < col {
			continue
		}
		v, err := strconv.ParseInt(fs[col-1], 10, 64)
		if err == nil && v >= -capValue && v <= capValue {
			sum += int(v)
			used++
		}
	}
	fmt.Fprintf(&b, "%d lines; column %d: %d values, sum %d\n", n, col, used, sum)
	return b.String(), 0
}

func TestLinesFoldsTheLineSplit(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on the path")
	}
	exe := filepath.Join(t.TempDir(), "lines.exe")
	if err := run("../../targets", "../../examples/lines/lines.oro", "go", exe, "../../lib", false, false, ""); err != nil {
		t.Fatalf("build: %v", err)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	for _, c := range []struct {
		name, in string
		col      int
	}{
		{"a column, a blank line, a non-number, an unterminated last line", "a 10 x\nb 20\n\nc notnum 5\nd 7 1\ne -3", 2},
		{"the empty input has no lines", "", 1},
		{"one LF is one empty line", "\n", 1},
		{"a final LF ends a line and begins none", "1\n2\n", 1},
		{"CR LF, and a CR before the end of input", "3\r\n4\r", 1},
		{"a CR inside a line stays", "5\rx\n", 1},
		{"bytes that are not UTF-8 pass through", "\xff\xfe 5\n\xc3 6\n", 2},
		{"the value limit, at it and one past it", "100000000000\n-100000000000\n100000000001\n-100000000001\n", 1},
		{"a numeral past the word is not a value", "9223372036854775808\n1\n", 1},
		{"a token of 65,535 bytes is read", long(65535) + "\n7\n", 1},
		{"a token of 65,536 bytes ends the scan", "1\n" + long(65536) + "\n7\n", 1},
		{"an unterminated token of 65,536 bytes too", "1\n" + long(65536), 1},
	} {
		want, status := linesReference([]byte(c.in), c.col)
		cmd := exec.Command(exe, strconv.Itoa(c.col))
		cmd.Stdin = strings.NewReader(c.in)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		got := 0
		if ee, ok := err.(*exec.ExitError); ok {
			got = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != status {
			t.Errorf("%s: status %d, want %d (stderr %q)", c.name, got, status, stderr.String())
		}
		if stdout.String() != want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, clip(stdout.String()), clip(want))
		}
		if (status != 0) != (stderr.Len() > 0) {
			t.Errorf("%s: status %d with stderr %q", c.name, status, stderr.String())
		}
	}
	// The column is the argument: absent is 1, and anything but 1..65536 is a
	// usage error that reads nothing.
	for _, c := range []struct {
		args   []string
		status int
	}{
		{nil, 0}, {[]string{"65536"}, 0},
		{[]string{"0"}, 1}, {[]string{"65537"}, 1}, {[]string{"two"}, 1}, {[]string{"-1"}, 1},
	} {
		cmd := exec.Command(exe, c.args...)
		cmd.Stdin = strings.NewReader("4 5\n")
		out, err := cmd.Output()
		got := 0
		if ee, ok := err.(*exec.ExitError); ok {
			got = ee.ExitCode()
		}
		if got != c.status {
			t.Errorf("lines %v: status %d, want %d", c.args, got, c.status)
		}
		if c.status != 0 && len(out) != 0 {
			t.Errorf("lines %v: a usage error wrote %q", c.args, out)
		}
		if c.args == nil && !strings.HasSuffix(string(out), "1 lines; column 1: 1 values, sum 4\n") {
			t.Errorf("lines with no argument sums column 1, got %q", out)
		}
	}
}

func clip(s string) string {
	if len(s) > 300 {
		return s[:150] + " … " + s[len(s)-150:]
	}
	return s
}
