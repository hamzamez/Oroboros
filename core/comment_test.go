package core

import (
	"reflect"
	"testing"
)

// COMMENTS (docs/spec/comments.md, ADR 0024; J's lexical level, spec/j.md
// §1.4, ADR 0067).
//
// A comment is not a token: it is gap between tokens, erased by the lexer, and
// `parse` is therefore not injective. Nothing below the reader can see one,
// which is what makes a comment free — and what makes a source-rewriting tool
// the only thing that can lose one. A comment is `//` to the end of the line,
// and there is no other kind.

// WHERE A GAP MAY GO: between any two tokens. There is no position in a form
// that is special, and a file of only comments is not an error.
func TestACommentMayGoWhereverAGapMay(t *testing.T) {
	// what each source must read as: the form's kind, its name, and its term
	for _, c := range []struct{ src, want string }{
		{"fn f() { (1 + // inside an expression\n 2) }", "def f (fn () (+ 1 2))"},
		{"// before the head\nfn f() { 1 }", "def f (fn () 1)"},
		{"fn f(p) { if p { 1 } else { // inside an arm\n 2 } }", "def f (fn (p) (if p 1 2))"},
		{"fn f() {\n    x = 1 // after a binding\n    x\n}", "def f (fn () ((fn (x) x) 1))"},
		{"fn f(n ∈ // inside a header\n Int) (Int) { n }", "sig f "},
		{"fn f() { loop i = 0 { // inside a loop\n i < 3 -> i; else -> again(i + 1) } }",
			"def f (fn () (loop (fn (i) (if (< i 3) i (again (+ i 1)))) 0))"},
		// an empty parameter list, and a comment inside it
		{"fn f(\n    // inside an empty parameter list\n) { 1 }", "def f (fn () 1)"},
	} {
		forms, err := Read(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if len(forms) == 0 {
			t.Fatalf("%q: expected a form", c.src)
		}
		got := forms[0].Kind + " " + forms[0].Name + " "
		if forms[0].Term != nil {
			got += forms[0].Term.String()
		}
		if got != c.want {
			t.Errorf("%q\n got %q\nwant %q", c.src, got, c.want)
		}
	}
	// A FILE OF ONLY COMMENTS reads as no forms, and that is not an error.
	forms, err := Read("// nothing but this\n// and this\n")
	if err != nil {
		t.Errorf("a file of only comments must read: %v", err)
	} else if len(forms) != 0 {
		t.Errorf("a file of only comments has no forms, got %d", len(forms))
	}
}

// A COMMENT ENDS AT A NEWLINE OR AT END OF INPUT: a file whose last line is a
// comment with no trailing newline is not unterminated.
func TestACommentEndsAtANewlineOrAtEndOfInput(t *testing.T) {
	for _, src := range []string{
		"1 + 2 // no trailing newline",
		"1 + 2 // with one\n",
		"1 + 2//no space before the slashes",
	} {
		got, err := ReadTerm(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if got.String() != "(+ 1 2)" {
			t.Errorf("%q: got %s", src, got)
		}
	}
}

// `//` IS NOT A COMMENT INSIDE A STRING, which is scanned by its own rule the
// gap never enters; and `//` ends a name rather than being part of one.
func TestSlashesInAStringOrAfterANameAreNotAComment(t *testing.T) {
	got, err := ReadTerm(`"a//b"`)
	if err != nil {
		t.Fatalf("a string containing slashes must read: %v", err)
	}
	if got.Kind != KStr || got.Str != "a//b" {
		t.Errorf(`"a//b" must be the four-character string, got %s`, got)
	}
	got, err = ReadTerm("foo//bar\n")
	if err != nil {
		t.Fatalf("foo//bar: %v", err)
	}
	if got.String() != "foo" {
		t.Errorf("foo//bar is the name `foo` and a comment, got %s", got)
	}
}

// NO BLOCK COMMENT, which is a decision (ADR 0024, 0067) rather than a gap: a
// comment that could close mid-line could change which form a line reads as.
// `/*` is the division sign and a product's, so a would-be block comment does
// not read.
func TestThereIsNoBlockComment(t *testing.T) {
	for _, src := range []string{
		"1 + /* a block */ 2",
		"f(/* a block */)",
	} {
		if got, err := ReadTerm(src); err == nil {
			t.Errorf("%q must be refused, read as %s", src, got)
		}
	}
}

// AND THE WHOLE POINT: a comment changes nothing. The same program with and
// without its comments reads to an IDENTICAL term, which is the statement that
// `parse` factors through the quotient by gaps.
func TestCommentsAreErased(t *testing.T) {
	commented := `// the sum of two numbers
fn add2(a, b) {
    // a and b
    a + b  // nothing else
}`
	bare := `fn add2(a, b) { a + b }`
	with, err := Read(commented)
	if err != nil {
		t.Fatal(err)
	}
	without, err := Read(bare)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(with, without) {
		t.Errorf("comments must be erased:\n with %v\n without %v", with, without)
	}
}
