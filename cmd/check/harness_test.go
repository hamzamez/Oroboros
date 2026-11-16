package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AN EMITTED FILE CLANG REFUSES FAILS THE ASSEMBLY STEP. THE PLANTED FAULT is
// the text that once went through on comparison alone: `icmp sgt` on doubles
// (llvmf64-2026-11-06), which clang refuses; its correct form, `fcmp ogt`, is
// the control and assembles.
func TestAnEmittedFileIsAssembled(t *testing.T) {
	clang := clangPath()
	if clang == "" {
		t.Skip("no clang")
	}
	const mod = "target triple = \"x86_64-pc-windows-msvc\"\n\n" +
		"define i1 @gt(double %a, double %b) {\nentry:\n  %r = CMP double %a, %b\n  ret i1 %r\n}\n"
	dir := t.TempDir()
	for _, c := range []struct {
		cmp  string
		good bool
	}{{"fcmp ogt", true}, {"icmp sgt", false}} {
		ll := filepath.Join(dir, strings.ReplaceAll(c.cmp, " ", "-")+".ll")
		if err := os.WriteFile(ll, []byte(strings.Replace(mod, "CMP", c.cmp, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		err := assembleOne(clang, ll, ll+".obj")
		if c.good && err != nil {
			t.Errorf("%s: refused: %v", c.cmp, err)
		}
		if !c.good && err == nil {
			t.Errorf("%s on doubles assembled; want clang's refusal", c.cmp)
		}
	}
}

// A CHAPTER WHOSE BLOCK IS NOT WHAT IT QUOTES FAILS THE BOOK STEP. THE PLANTED
// FAULT is a block that says it is examples/hello.oro, with its first line
// changed; the same block quoted right is the control. The step reads the
// checker's exit, so the checker must fail on the fault.
func TestABookBlockIsChecked(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the book's tools")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	hello, err := os.ReadFile(filepath.Join(root, "examples", "hello.oro"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	chapter := func(name, body string) string {
		p := filepath.Join(dir, name)
		text := "# A chapter\n\n<!-- file examples/hello.oro -->\n```\n" + body + "```\n"
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	run := func(p string) error {
		cmd := exec.Command("go", "run", filepath.FromSlash("docs/book/check.go"), p)
		cmd.Dir = root
		_, err := cmd.CombinedOutput()
		return err
	}
	if err := run(chapter("right.md", string(hello))); err != nil {
		t.Errorf("the block quoted right: %v", err)
	}
	wrong := strings.Replace(string(hello), "The first Oroboros program", "The second Oroboros program", 1)
	if err := run(chapter("wrong.md", wrong)); err == nil {
		t.Errorf("a block that misquotes its file passed the book's checker")
	}
}
