package emit

import (
	"fmt"
	"strings"
)

// RENDERING A STRING LITERAL — docs/spec/string-literals.md §6.
//
// The obligation is one sentence: the compiler must render every element of
// Scalar* faithfully on every target, and where a host's notation cannot express
// a scalar directly, the compiler finds one that can. A target does not get to
// narrow what a string IS; it gets to say how it spells one.
//
// Two renderers because two hosts have different objects: a Go string is bytes
// holding UTF-8, and a Java or JavaScript string is a sequence of UTF-16 code
// units. The same scalar above U+FFFF is one \U escape on Go and a SURROGATE
// PAIR on the other two — the pair being that encoding form's representation of
// the very same scalar, which is why §1 chose scalar values in the first place.
//
// ═══ OUTPUT IS ASCII ONLY, AND THAT IS THE POINT
//
// `javac` defaults to the PLATFORM CHARSET, so a non-ASCII byte in an emitted
// `.java` file means whatever the build's locale says — the hazard strings.md §5
// recorded, where `café` became five characters because Windows-1252 was
// assumed. Emitting \u00E9 removes it by construction rather than by
// remembering to pass `-encoding UTF-8`.
//
// ═══ WHAT THIS REPLACED, AND WHY IT WAS WRONG
//
// All three backends called `strconv.Quote` under the comment "strconv.Quote
// escapes non-ASCII to \u, which is valid in all three hosts and sidesteps
// javac's platform-charset hazard entirely". It does neither:
//
//   - it passes PRINTABLE non-ASCII through RAW, so the emitted Java file
//     contained UTF-8 bytes and worked only because the build happened to pass
//     `-encoding UTF-8`;
//   - it renders an UNPRINTABLE scalar above the BMP as \UHHHHHHHH, which only
//     Go has — javac refuses it, and JavaScript drops the backslash and yields
//     ten literal characters. Measured, string-literals.md §6.2.
//
// The five shared escapes are emitted as escapes rather than as \u00XX because
// they are the ones every host spells the same way and a human reads at a glance.

// escShared writes the five escapes Go, Java and JavaScript agree on, and
// reports whether it handled the rune.
func escShared(b *strings.Builder, r rune) bool {
	switch r {
	case 0x5C: // reverse solidus
		b.WriteString("\\\\")
	case 0x22: // quotation mark
		b.WriteString("\\" + string(rune(0x22)))
	case 0x0A:
		b.WriteString("\\n")
	case 0x0D:
		b.WriteString("\\r")
	case 0x09:
		b.WriteString("\\t")
	default:
		return false
	}
	return true
}

// asciiPrintable is the range that may be written as itself: space through `~`,
// minus the two the notation needs back.
func asciiPrintable(r rune) bool {
	return r >= 0x20 && r <= 0x7E && r != 0x5C && r != 0x22
}

// GoStringLit renders a string as a Go literal.
//
// Go's string is UTF-8 and its escapes reach every scalar directly: \uHHHH in
// the BMP and \UHHHHHHHH above it.
func GoStringLit(s string) string {
	var b strings.Builder
	b.WriteByte(0x22)
	for _, r := range s {
		switch {
		case escShared(&b, r):
		case asciiPrintable(r):
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, "\\u%04X", r)
		default:
			fmt.Fprintf(&b, "\\U%08X", r)
		}
	}
	b.WriteByte(0x22)
	return b.String()
}

// UTF16StringLit renders a string as a Java or JavaScript literal.
//
// Both store UTF-16 code units and neither has an eight-digit escape, so a
// scalar above the BMP is written as the SURROGATE PAIR that encoding form uses
// for it. Nothing is lost and nothing is a workaround: the pair IS how UTF-16
// represents that scalar, and the host's own string type is what is being
// written.
func UTF16StringLit(s string) string {
	var b strings.Builder
	b.WriteByte(0x22)
	for _, r := range s {
		switch {
		case escShared(&b, r):
		case asciiPrintable(r):
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, "\\u%04X", r)
		default:
			// UTF-16: subtract the BMP, then split the remaining twenty bits.
			v := r - 0x10000
			fmt.Fprintf(&b, "\\u%04X\\u%04X", 0xD800+(v>>10), 0xDC00+(v&0x3FF))
		}
	}
	b.WriteByte(0x22)
	return b.String()
}
