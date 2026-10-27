package core

import "testing"

// THE BOOLEAN OPERATIONS, THE SHIFTS AND MOD ARE READ AT GO'S LEVELS (ADR 0082
// item 6): each source below reads to the core term beside it, and printing the
// term writes J that reads back to it.
func TestTheBitOperatorsReadAtGosLevels(t *testing.T) {
	for src, want := range map[string]string{
		"a & b":             "(& a b)",
		"a | b":             "(| a b)",
		"a ⊕ b":             "(⊕ a b)",
		`a \oplus b`:        "(⊕ a b)",
		"a << 3":            "(<< a 3)",
		"a >> k":            "(>> a k)",
		"h mod 2^64":        "(mod h (+ (* 18446 1000000000000000) 744073709551616))", // a literal past int64, as the core holds one
		"~x":                "(~ x)",
		"~5":                "-6",
		"0xEDB88320":        "3988292384",
		"0Xff":              "255",
		"x & 1 == 0":        "(= (& x 1) 0)",
		"a | b & c":         "(| a (& b c))",
		"a ⊕ b * c":         "(⊕ a (* b c))",
		"a + b << 2":        "(+ a (<< b 2))",
		"(h ⊕ b) * p mod m": "(mod (* (⊕ h b) p) m)",
		"-x & y":            "(& (- x) y)",
	} {
		term, err := ReadTerm(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if term.String() != want {
			t.Errorf("%s reads as %s, want %s", src, term, want)
			continue
		}
		back, err := ReadTerm(ShowTerm(term))
		if err != nil || back.String() != want {
			t.Errorf("%s prints as %s, which reads as %v (%v)", src, ShowTerm(term), back, err)
		}
	}
	// A HEXADECIMAL LITERAL WITH A LETTER AFTER IT is not two tokens.
	if _, err := ReadTerm("0x1g"); err == nil {
		t.Error("0x1g read; it is no literal")
	}
	// `mod` IS A NAME WHERE NO OPERATOR STANDS (j.md §1.4's reservation).
	if term, err := ReadTerm("{ mod = 3; mod + 1 }"); err != nil {
		t.Errorf("`mod` as a binder: %v", err)
	} else if ShowTerm(term) == "" {
		t.Error("nothing printed")
	}
}
