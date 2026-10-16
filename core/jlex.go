package core

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// THE LEXER, spec/j.md §1: maximal munch over the token classes, comments and
// whitespace a gap. A token remembers whether a line break came before it, which
// is what the parser's line rule reads (§1.7).

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tQuoted
	tInt
	tFloat
	tString
	tTag
	tPrime
	tSym
)

type token struct {
	kind tokKind
	text string // an identifier's or a symbol's spelling, a literal's source, a string's value
	pos  int
	line int
	nl   bool // a line break precedes it
}

func (t token) String() string {
	if t.kind == tEOF {
		return "the end of the file"
	}
	return fmt.Sprintf("%q", t.text)
}

// symbols, longest first where one is a prefix of another.
var symbols = []string{"...", "<=>", "..", ":=", "==", "!=", "<=", ">=", "=>", "->", "&&", "||", "++",
	"+", "-", "*", "/", "%", "^", "<", ">", "=", "!", ":", ",", ";", ".", "(", ")", "[", "]", "{", "}",
	"∈", "∉", "∪", "∩", "∖", "⊆", "⊂", "×", "∀", "∃", "⇒", "⇔"}

// ascii are TLA+'s ASCII spellings of the symbols (ADR 0065, 0067).
var ascii = map[string]string{`\in`: "∈", `\notin`: "∉", `\cup`: "∪", `\cap`: "∩", `\subseteq`: "⊆",
	`\subset`: "⊂", `\X`: "×", `\A`: "∀", `\E`: "∃"}

func lex(src string) ([]token, error) {
	// UTF-8, NFC and no bidirectional control (core-0 §1.1): two identifiers
	// that display alike are one identifier.
	if err := validSource(src); err != nil {
		return nil, err
	}
	var out []token
	line, nl := 1, true
	for i := 0; i < len(src); {
		r, w := utf8.DecodeRuneInString(src[i:])
		switch {
		case r == '\n':
			line++
			nl = true
			i++
			continue
		case r == ' ' || r == '\t' || r == '\r':
			i++
			continue
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		t := token{pos: i, line: line, nl: nl}
		nl = false
		switch {
		case r == '_' || unicode.IsLetter(r):
			j := i
			for j < len(src) {
				c, cw := utf8.DecodeRuneInString(src[j:])
				if c != '_' && !unicode.IsLetter(c) && !unicode.IsDigit(c) && !unicode.IsMark(c) {
					break
				}
				j += cw
			}
			if j < len(src) && src[j] == '?' {
				j++
			}
			t.kind, t.text = tIdent, src[i:j]
			i = j
		case r == '`':
			j := strings.IndexByte(src[i+1:], '`')
			if j < 0 || strings.ContainsRune(src[i+1:i+1+j], '\n') {
				return nil, fmt.Errorf("line %d: a quoted name has no closing backquote", line)
			}
			t.kind, t.text = tQuoted, src[i+1:i+1+j]
			i += j + 2
		case r >= '0' && r <= '9':
			j := i
			for j < len(src) && src[j] >= '0' && src[j] <= '9' {
				j++
			}
			t.kind = tInt
			if j+1 < len(src) && src[j] == '.' && src[j+1] >= '0' && src[j+1] <= '9' {
				t.kind = tFloat
				j++
				for j < len(src) && src[j] >= '0' && src[j] <= '9' {
					j++
				}
			}
			if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
				k := j + 1
				if k < len(src) && (src[k] == '+' || src[k] == '-') {
					k++
				}
				if k < len(src) && src[k] >= '0' && src[k] <= '9' {
					t.kind = tFloat
					for j = k; j < len(src) && src[j] >= '0' && src[j] <= '9'; j++ {
					}
				}
			}
			t.text = src[i:j]
			i = j
		case r == '"':
			s, n, err := unquote(src[i:], line)
			if err != nil {
				return nil, err
			}
			t.kind, t.text = tString, s
			i += n
		case r == '\'':
			after, _ := utf8.DecodeRuneInString(src[i+1:])
			prev := rune(0)
			if i > 0 {
				prev, _ = utf8.DecodeLastRuneInString(src[:i])
			}
			if prev == '_' || unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '?' {
				t.kind, t.text = tPrime, "'"
				i++
				break
			}
			if after != '_' && !unicode.IsLetter(after) {
				return nil, fmt.Errorf("line %d: a tag is `'` followed by a name", line)
			}
			j := i + 1
			for j < len(src) {
				c, cw := utf8.DecodeRuneInString(src[j:])
				if c != '_' && !unicode.IsLetter(c) && !unicode.IsDigit(c) {
					break
				}
				j += cw
			}
			if j < len(src) && src[j] == '?' {
				j++
			}
			t.kind, t.text = tTag, src[i+1:j]
			i = j
		case r == '\\':
			matched := false
			for spell, sym := range ascii {
				if strings.HasPrefix(src[i:], spell) && !isWordAt(src, i+len(spell)) {
					t.kind, t.text = tSym, sym
					i += len(spell)
					matched = true
					break
				}
			}
			if !matched {
				t.kind, t.text = tSym, "∖" // `\` alone is set difference
				i++
			}
		default:
			found := false
			for _, s := range symbols {
				if strings.HasPrefix(src[i:], s) {
					t.kind, t.text = tSym, s
					i += len(s)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("line %d: %q starts no token", line, string(r))
			}
			_ = w
		}
		out = append(out, t)
	}
	out = append(out, token{kind: tEOF, pos: len(src), line: line, nl: true})
	return out, nil
}

func isWordAt(src string, i int) bool {
	if i >= len(src) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(src[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// unquote reads a string literal at the start of s: string-literals.md's six
// escapes, an unknown one an error. It returns the value and the bytes read.
func unquote(s string, line int) (string, int, error) {
	var b strings.Builder
	for i := 1; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			return b.String(), i + 1, nil
		case c == '\t' || c == '\n' || c == '\r':
			// A token that may contain a raw line terminator makes an unterminated
			// literal indistinguishable from a long one (string-literals.md §4).
			return "", 0, fmt.Errorf("line %d: a string literal may not contain a raw "+
				"tab, newline or carriage return; write \\t, \\n or \\r", line)
		case c == '\\':
			if i+1 >= len(s) {
				return "", 0, fmt.Errorf("line %d: unterminated escape in string literal", line)
			}
			switch s[i+1] {
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case 'u':
				v, n, err := scalarEscape(s[i+2:], line)
				if err != nil {
					return "", 0, err
				}
				b.WriteRune(v)
				i += 2 + n
				continue
			default:
				return "", 0, fmt.Errorf("line %d: \\%c is not an escape. The escapes are "+
					"\\\" \\\\ \\t \\n \\r and \\u{H…} for any scalar value "+
					"(docs/spec/string-literals.md §3)", line, s[i+1])
			}
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", 0, fmt.Errorf("line %d: a string has no closing quote", line)
}

// scalarEscape reads `{H…}` after `\u`, returning the scalar and the bytes read.
// One to six hexadecimal digits, and the value must be a SCALAR VALUE: Unicode
// excludes the surrogate range D800–DFFF, an artefact of the UTF-16 encoding
// form rather than a character, which is what makes a string's value
// encoding-independent (string-literals.md §1).
func scalarEscape(s string, line int) (rune, int, error) {
	if len(s) == 0 || s[0] != '{' {
		return 0, 0, fmt.Errorf("line %d: \\u must be written \\u{H…} with one to six "+
			"hexadecimal digits", line)
	}
	end := strings.IndexByte(s, '}')
	if end < 0 {
		return 0, 0, fmt.Errorf("line %d: unterminated \\u{…} escape", line)
	}
	digits := s[1:end]
	switch {
	case len(digits) == 0:
		return 0, 0, fmt.Errorf("line %d: \\u{} has no digits", line)
	case len(digits) > 6:
		return 0, 0, fmt.Errorf("line %d: \\u{…} takes at most six hexadecimal "+
			"digits; the largest scalar value is 10FFFF", line)
	}
	v, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("line %d: \\u{%s} is not hexadecimal", line, digits)
	}
	if v > 0x10FFFF {
		return 0, 0, fmt.Errorf("line %d: \\u{%s} is above the largest scalar value, 10FFFF", line, digits)
	}
	if v >= 0xD800 && v <= 0xDFFF {
		return 0, 0, fmt.Errorf("line %d: \\u{%s} is a SURROGATE, which is not a scalar "+
			"value — it exists only inside the UTF-16 encoding form, and a sequence "+
			"containing one has no representation in UTF-8 at all "+
			"(docs/spec/string-literals.md §1)", line, digits)
	}
	return rune(v), end + 1, nil
}
