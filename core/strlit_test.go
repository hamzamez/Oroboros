package core

import (
	"strings"
	"testing"
)

// STRING LITERALS — docs/spec/string-literals.md.
//
// A literal denotes an element of Scalar*, and the escape set is DERIVED rather
// than borrowed: two escapes because the delimiter and the escape introducer
// cannot denote themselves, three because the source grammar owns tab and the
// line terminators, and one general escape because totality demands that every
// scalar be writable.

func TestStringLiteralEscapes(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`"AB"`, "AB"},
		{`"a\tb"`, "a\tb"},
		{`"a\nb"`, "a\nb"},
		{`"a\rb"`, "a\rb"},
		{`"a\"b"`, "a\"b"},
		{`"a\\b"`, "a" + string(rune(92)) + "b"},
		{`""`, ""},
		// The general escape reaches every scalar, in one to six digits, in
		// either case.
		{`"\u{41}"`, "A"},
		{`"\u{e9}"`, "\u00e9"},
		{`"\u{E9}"`, "\u00e9"},
		{`"\u{1F642}"`, "\U0001F642"},
		{`"\u{10FFFF}"`, "\U0010FFFF"},
		// U+0000 IS AN ORDINARY SCALAR VALUE. The first draft of the spec
		// refused it because a windows string is NUL-terminated — a property of
		// the C calling convention and not of strings. That refusal was a target
		// dictating the language, and it was removed.
		{`"a\u{0}b"`, "a\x00b"},
		// NFC constrains the program TEXT, not the data: this is two scalars.
		{`"e\u{301}"`, "e\u0301"},
	} {
		got, err := Read("(def x " + c.src + ")")
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if v := got[0].Term.Str; v != c.want {
			t.Errorf("%s gave %q, want %q", c.src, v, c.want)
		}
	}
}

// AND WHAT IS REFUSED, each for a reason that is about the notation or about
// Unicode — never about a host.
func TestStringLiteralRefusals(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		// Not escapes. An unknown escape is an ERROR rather than the character
		// itself, which follows from unambiguity: if \a meant `a`, two different
		// literals would denote one string. JavaScript does not have this rule,
		// and string-literals.md §6.2 measured what that costs there.
		{`"\a"`, "is not an escape"},
		{`"\v"`, "is not an escape"},
		{`"\b"`, "is not an escape"},
		{`"\f"`, "is not an escape"},
		{`"\'"`, "is not an escape"},
		// A byte is not a scalar value, so a notation that can build one is a
		// notation for a different object.
		{`"\xff"`, "is not an escape"},
		{`"\377"`, "is not an escape"},
		// A SURROGATE is not a scalar value — it exists only inside the UTF-16
		// encoding form, and a sequence containing one has no UTF-8 at all.
		{`"\u{D800}"`, "SURROGATE"},
		{`"\u{DFFF}"`, "SURROGATE"},
		// Above the largest scalar value.
		{`"\u{110000}"`, "10FFFF"},
		{`"\u{1234567}"`, "six hexadecimal"},
		{`"\u{}"`, "no digits"},
		{`"\u{zz}"`, "not hexadecimal"},
		{`"\u41"`, "must be written"},
	} {
		if _, err := Read("(def x " + c.src + ")"); err == nil {
			t.Errorf("%s was accepted", c.src)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error does not mention %q: %v", c.src, c.want, err)
		}
	}
}

// A RAW TAB OR LINE TERMINATOR IS REFUSED, and the reason is the source grammar
// rather than legibility alone: a token that may contain a raw line terminator
// makes an unterminated literal indistinguishable from a long one, so the error
// would be reported at end of file instead of where it happened.
func TestStringLiteralRejectsRawWhitespace(t *testing.T) {
	for _, raw := range []string{"a\tb", "a\nb", "a\rb"} {
		if _, err := Read(`(def x "` + raw + `")`); err == nil {
			t.Errorf("a literal containing a raw %q was accepted", raw)
		} else if !strings.Contains(err.Error(), "raw") {
			t.Errorf("%q: %v", raw, err)
		}
	}
}
