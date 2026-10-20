package core

import "strconv"

// FormsOf is D of a module's declarations as the forms the compiler reads.
func FormsOf(decls []Decl) ([]Form, error) { return newDesugarer().forms(carryClaims(decls)) }

// IntLiteral is the term a decimal integer is: an int64,
// or past it Horner's form over 10¹⁵ (bigLiteral).
func IntLiteral(text string) *Term {
	if v, err := strconv.ParseInt(text, 10, 64); err == nil {
		return Int(v)
	}
	return bigLiteral(text)
}

// Desugar is D of one tree: the core term it means, for a rendering's messages
// and checks.
func Desugar(a Tail) *Term { return newDesugarer().term(a) }

// DeclTerms is D of a module's declarations: the declaration terms, in the
// order D gives them (decl.go, placed), as a target file's D_T reads them.
func DeclTerms(decls []Decl) []*Term {
	ts, _ := newDesugarer().placed(carryClaims(decls))
	return ts
}
