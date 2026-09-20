package core

import (
	"reflect"
	"strings"
	"testing"
)

// COMMENTS (docs/spec/comments.md, ADR 0024).
//
// A comment is not a token: it is gap between tokens, erased by the lexer, and
// `parse` is therefore not injective. Nothing below the reader can see one,
// which is what makes a comment free — and what makes a source-rewriting tool
// the only thing that can lose one.
//
// None of this was specified until 2026-09-20, so these tests are what the
// reader already did, written down.

// WHERE A GAP MAY GO: between any two tokens, because the reader calls
// skipSpace before every term it reads. There is no position in a form that is
// special, and a file of only comments is not an error.
func TestACommentMayGoWhereverAGapMay(t *testing.T) {
	// what each source must read as: the form's kind, its name, and its term
	for _, c := range []struct{ src, want string }{
		{"(+ 1 ; inside a form\n 2)", "term  (+ 1 2)"},
		{"(def ; between the head and the name\n f 1)", "def f 1"},
		{"(cond p 1 ; between clauses\n else 2)", "term  (if p 1 2)"},
		{"(let x ; between a binder and its value\n 1 x)", "term  ((fn (x) x) 1)"},
		{"(sig f ((n ; inside a signature\n int)) int)", "sig f "},
		{"(loop ((i ; inside the variable list\n 0)) (< i 3) i else (again (+ i 1)))",
			"term  (loop (fn (i) (if (< i 3) i (again (+ i 1)))) 0)"},
		// an empty parameter list is the one place `()` is legal, and a comment
		// may sit inside it
		{"(def f ( ; inside an empty parameter list\n ) 1)", "def f (fn () 1)"},
	} {
		forms, err := Read(c.src)
		if err != nil {
			t.Fatalf("%q: %v", c.src, err)
		}
		if len(forms) != 1 {
			t.Fatalf("%q: expected one form, got %d", c.src, len(forms))
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
	forms, err := Read("; nothing but this\n; and this\n")
	if err != nil {
		t.Errorf("a file of only comments must read: %v", err)
	} else if len(forms) != 0 {
		t.Errorf("a file of only comments has no forms, got %d", len(forms))
	}
}

// A COMMENT ENDS AT A NEWLINE **OR AT END OF INPUT**. core-0.md §1.2 said
// `";" any* newline`, which makes a file whose last line is a comment with no
// trailing newline unterminated. The reader has always accepted it; the grammar
// was what was wrong.
func TestACommentEndsAtANewlineOrAtEndOfInput(t *testing.T) {
	for _, src := range []string{
		"(+ 1 2) ; no trailing newline",
		"(+ 1 2) ; with one\n",
		"(+ 1 2);no space before the semicolon",
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

// `;` IS NOT MAGIC IN THE TWO PLACES IT IS NOT A COMMENT. A string is scanned
// by its own rule, which the gap never enters; and `;` is a delimiter, so it
// ends a name rather than being part of one.
func TestASemicolonInAStringOrAfterANameIsNotAComment(t *testing.T) {
	got, err := ReadTerm(`"a;b"`)
	if err != nil {
		t.Fatalf("a string containing a semicolon must read: %v", err)
	}
	if got.Kind != KStr || got.Str != "a;b" {
		t.Errorf(`"a;b" must be the three-character string, got %s`, got)
	}
	forms, err := Read("foo;bar\n")
	if err != nil {
		t.Fatalf("foo;bar: %v", err)
	}
	if len(forms) != 1 || forms[0].Term == nil || forms[0].Term.String() != "foo" {
		t.Errorf("foo;bar is the name `foo` and a comment, got %v", forms)
	}
}

// NO DATUM COMMENT AND NO BLOCK COMMENT, which is a decision (ADR 0024) rather
// than a gap. `#;` would let a comment change WHICH GRAMMAR a form takes, since
// `def`'s shorthand, `let` and every clause chain are discriminated by arity or
// parity — so the test is that both are refused, not merely absent.
func TestNeitherDatumNorBlockCommentsAreRead(t *testing.T) {
	for _, src := range []string{
		"(let x 1 #;y #;2 x)",
		"(+ 1 #;2 3)",
		"(+ 1 #| a block |# 2)",
	} {
		if got, err := ReadTerm(src); err == nil {
			t.Errorf("%q must be refused, read as %s", src, got)
		} else if !strings.Contains(err.Error(), "#") {
			t.Errorf("%q: the refusal should name the character, got %v", src, err)
		}
	}
}

// AND THE WHOLE POINT: a comment changes nothing. The same program with and
// without its comments reads to an IDENTICAL term, which is the statement that
// `parse` factors through the quotient by gaps.
func TestCommentsAreErased(t *testing.T) {
	commented := `
		; the sum of two numbers
		(def add2 (a b)        ; a and b
		  (+ a b))             ; nothing else
	`
	bare := `(def add2 (a b) (+ a b))`
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
