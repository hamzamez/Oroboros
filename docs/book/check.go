//go:build ignore

// check re-runs a chapter's examples against the compiler and compares them
// with what the chapter prints.
//
//	go run docs/book/check.go docs/book/00-tour.md      (from the repository's root)
//
// A fenced block is checked when the line before its fence is a directive, an
// HTML comment, which a rendered page does not show:
//
//	<!-- file PATH -->                 the block is the file, byte for byte
//	<!-- part PATH -->                 the block is a run of the file's lines
//	<!-- gen TARGET PATH [-checked] -->  the block is a run of lines of what `gen`
//	                                   printed, or of the file it emitted
//	<!-- run TARGET PATH -->           the block is what the built program printed
//	<!-- portable PATH -->             the block is a run of lines of `portable`
//
// In a `part`, `gen` or `portable` block a line holding only `…` stands for
// lines left out: the pieces around it must appear in that order. Tabs compare
// as four spaces and trailing blanks are dropped; nothing else is forgiven.
//
// A block with no directive is not checked, and the last line says how many
// were and were not, so an unchecked block is a count and not a silence.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var directive = regexp.MustCompile(`^<!--\s+(file|part|gen|run|portable)\s+(.*?)\s*-->$`)

func norm(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(strings.ReplaceAll(l, "\t", "    "), " ")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

// pieces splits a block at its elision lines.
func pieces(block string) []string {
	var out []string
	var cur []string
	for _, l := range strings.Split(block, "\n") {
		if strings.TrimSpace(l) == "…" {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, l)
	}
	return append(out, strings.Join(cur, "\n"))
}

// within reports whether the block's pieces are runs of whole lines of text,
// in order.
func within(block, text string) bool {
	text = "\n" + text + "\n"
	at := 0
	for _, p := range pieces(block) {
		if strings.Trim(p, "\n") == "" {
			continue
		}
		k := strings.Index(text[at:], "\n"+strings.Trim(p, "\n")+"\n")
		if k < 0 {
			return false
		}
		at += k + len(p) + 1
	}
	return true
}

func run(name string, args ...string) string {
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	cmd.Run() // a refusal is an exit code, and its text is what is compared
	return buf.String()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run docs/book/check.go CHAPTER.md   (from the repository's root)")
		os.Exit(2)
	}
	text, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	tmp, err := os.MkdirTemp("", "book")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer os.RemoveAll(tmp)
	tool := map[string]string{}
	for _, t := range []string{"gen", "build", "portable"} {
		tool[t] = filepath.Join(tmp, t+".exe")
		if out := run("go", "build", "-o", tool[t], "./cmd/"+t); out != "" {
			fmt.Fprint(os.Stderr, out)
			os.Exit(2)
		}
	}

	lines := strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n")
	checked, unchecked, failed := 0, 0, 0
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "```") {
			continue
		}
		open := i
		var body []string
		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
			body = append(body, lines[i])
		}
		block := norm(strings.Join(body, "\n"))
		m := []string(nil)
		if open > 0 {
			m = directive.FindStringSubmatch(strings.TrimSpace(lines[open-1]))
		}
		if m == nil {
			unchecked++
			continue
		}
		checked++
		args := strings.Fields(m[2])
		var got string
		ok := false
		switch m[1] {
		case "file", "part":
			src, err := os.ReadFile(args[0])
			got = norm(string(src))
			if err != nil {
				got = err.Error()
			} else if m[1] == "file" {
				ok = block == got
			} else {
				ok = within(block, got)
			}
		case "gen":
			out := filepath.Join(tmp, "out.txt")
			os.Remove(out)
			a := []string{"-name", "tour"}
			if len(args) > 2 {
				a = append(a, args[2:]...)
			}
			said := run(tool["gen"], append(a, args[1], args[0], out)...)
			said = strings.ReplaceAll(said, filepath.ToSlash(out), "OUT")
			emitted, _ := os.ReadFile(out)
			got = norm(said) + "\n" + norm(string(emitted))
			ok = within(block, got)
		case "run":
			var exe string
			switch args[0] {
			case "go", "windows":
				exe = filepath.Join(tmp, "prog.exe")
			case "js":
				exe = filepath.Join(tmp, "prog.mjs")
			default:
				fmt.Fprintf(os.Stderr, "line %d: run on %s is not something this checker does\n", open, args[0])
				os.Exit(2)
			}
			os.Remove(exe)
			said := run(tool["build"], "-target="+args[0], "-o", exe, args[1])
			if _, err := os.Stat(exe); err != nil {
				got = norm(said)
			} else if args[0] == "js" {
				got = norm(run("node", exe))
				ok = block == got
			} else {
				got = norm(run(exe))
				ok = block == got
			}
		case "portable":
			got = norm(run(tool["portable"], args[0]))
			ok = within(block, got)
		}
		if !ok {
			failed++
			fmt.Printf("%s:%d: <!-- %s %s --> does not hold.\n--- the chapter prints\n%s\n--- the compiler gives\n%s\n\n",
				os.Args[1], open+1, m[1], m[2], block, got)
		}
	}
	fmt.Printf("%s: %d block(s) checked, %d failed; %d block(s) carry no directive and were not checked\n",
		os.Args[1], checked, failed, unchecked)
	if failed > 0 {
		os.Exit(1)
	}
}
