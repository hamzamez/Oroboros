package emit

import (
	"strings"
	"testing"
	"unicode/utf16"
)

// RENDERING A LITERAL — docs/spec/string-literals.md §6.
//
// The obligation is that EVERY element of Scalar* is rendered faithfully on
// every target. A target says how it spells a scalar; it does not get to say
// which scalars exist.

func TestStringLiteralRendering(t *testing.T) {
	for _, c := range []struct{ in, goWant, u16Want string }{
		{"AB", `"AB"`, `"AB"`},
		{"a\tb", `"a\tb"`, `"a\tb"`},
		{"q\"z", `"q\"z"`, `"q\"z"`},
		{"a" + string(rune(92)) + "b", `"a\\b"`, `"a\\b"`},
		// Non-ASCII is escaped rather than passed through, which is what makes
		// javac's platform-charset hazard unreachable (strings.md §5).
		{"caf\u00e9", `"caf\u00E9"`, `"caf\u00E9"`},
		{"\u200b", `"\u200B"`, `"\u200B"`},
		// ABOVE THE BMP the two hosts genuinely differ, and this is the case
		// that was broken: Go has an eight-digit escape and the UTF-16 hosts do
		// not, so they get the SURROGATE PAIR that encoding form uses for the
		// same scalar.
		{"\U0001F642", `"\U0001F642"`, `"\uD83D\uDE42"`},
		{"\U000E0001", `"\U000E0001"`, `"\uDB40\uDC01"`},
		// U+0000 is an ordinary scalar and every host holds it. Only a C API
		// cannot read past one, which is a precondition on that primitive.
		{"a\x00b", `"a\u0000b"`, `"a\u0000b"`},
	} {
		if got := GoStringLit(c.in); got != c.goWant {
			t.Errorf("GoStringLit(%q) = %s, want %s", c.in, got, c.goWant)
		}
		if got := UTF16StringLit(c.in); got != c.u16Want {
			t.Errorf("UTF16StringLit(%q) = %s, want %s", c.in, got, c.u16Want)
		}
	}
}

// THE OBLIGATION IS TOTAL, checked over every scalar value rather than over a
// list somebody thought of: the output is ASCII only, and the UTF-16 rendering
// decodes back to the scalar it came from.
//
// Totality is the requirement string-literals.md §2 puts first, and it is the
// one a table of examples cannot establish.
func TestEveryScalarRendersAndRoundTrips(t *testing.T) {
	checked := 0
	for r := rune(0); r <= 0x10FFFF; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue // not a scalar value (§1)
		}
		checked++
		for name, got := range map[string]string{
			"Go":    GoStringLit(string(r)),
			"UTF16": UTF16StringLit(string(r)),
		} {
			for i := 0; i < len(got); i++ {
				if got[i] > 0x7E || got[i] < 0x20 {
					t.Fatalf("%s rendering of U+%04X is not ASCII: %q", name, r, got)
				}
			}
		}
		// The UTF-16 rendering must denote the same scalar. Decode the escapes
		// back and compare — a surrogate PAIR is that encoding form's spelling
		// of one scalar, so this is the property that matters rather than the
		// shape of the text.
		if u := UTF16StringLit(string(r)); strings.Contains(u, "\\u") {
			var units []uint16
			for i := 0; i+6 <= len(u); {
				if u[i] == 92 && u[i+1] == 'u' {
					var v uint16
					for k := i + 2; k < i+6; k++ {
						v = v*16 + uint16(hexVal(u[k]))
					}
					units = append(units, v)
					i += 6
					continue
				}
				i++
			}
			if len(units) > 0 {
				if dec := utf16.Decode(units); len(dec) != 1 || dec[0] != r {
					t.Fatalf("UTF16 rendering of U+%04X decodes to %v", r, dec)
				}
			}
		}
	}
	if checked < 1112064 {
		t.Fatalf("only %d scalar values were checked; there are 1,112,064", checked)
	}
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return 0
}

// AND NO TARGET IS ASKED FOR AN ESCAPE IT DOES NOT HAVE. The five shared ones
// are the only letters that ever follow a backslash in a UTF-16 rendering
// besides `u` — `\a` and `\v` do not exist in Java, and `\U` exists only in Go,
// which is what string-literals.md §6.2 measured breaking.
func TestUTF16RenderingUsesOnlyEscapesBothHostsHave(t *testing.T) {
	ok := map[byte]bool{92: true, 0x22: true, 'n': true, 'r': true, 't': true, 'u': true}
	for r := rune(0); r <= 0x10FFFF; r += 7 { // a stride, since this is a shape check
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		s := UTF16StringLit(string(r))
		for i := 0; i+1 < len(s); i++ {
			if s[i] == 92 {
				if !ok[s[i+1]] {
					t.Fatalf("U+%04X rendered with escape %q, which Java or JavaScript lacks: %s",
						r, s[i:i+2], s)
				}
				i++
			}
		}
	}
}
