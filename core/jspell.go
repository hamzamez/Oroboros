package core

import (
	"math/big"
	"strings"
	"unicode"
)

// σ_J, J's SPELLING OF A NAME (spec/j.md §1.5, ADR 0067). A name is an atom of
// abstract.md's carrier 𝒩, and its spelling belongs to the rendering: `-` becomes
// `_`, a final `?` stays, and a name outside the identifier alphabet is quoted,
// the host's own name unchanged. σ_J is injective on a program or the program is
// refused (Collisions).

// reserved are the words J reserves (spec/j.md §1.4): never an identifier.
var reserved = map[string]bool{"fn": true, "spec": true, "lemma": true, "set": true, "array": true,
	"map": true, "var": true, "loop": true, "again": true, "case": true, "if": true, "else": true,
	"try": true, "expect": true, "assert": true, "use": true, "module": true, "provides": true,
	"private": true, "export": true, "axiom": true, "true": true, "false": true}

// isIdent reports whether s is an identifier: a letter or `_`, then letters,
// digits and `_`, optionally ending in one `?`.
func isIdent(s string) bool {
	s = strings.TrimSuffix(s, "?")
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case i > 0 && (unicode.IsDigit(r) || unicode.IsMark(r)):
		default:
			return false
		}
	}
	return true
}

// word spells one segment of a name: kebab-case becomes snake_case, and what is
// still no identifier, or is a reserved word, is quoted.
//
// A name that already holds `_` is quoted too: an unquoted `_` is always the
// abstract name's `-`, so σ_J is invertible (Unspell) as well as injective.
// No program name holds one; four host names in target files do
// (ERROR_FILE_NOT_FOUND and its kin).
func word(s string, member bool) string {
	if s == "_" {
		return s
	}
	t := strings.ReplaceAll(s, "-", "_")
	if isIdent(t) && (member || !reserved[t]) && !strings.Contains(s, "_") && !strings.HasPrefix(s, "-") {
		return t
	}
	return "`" + s + "`"
}

// Unspell is σ_J⁻¹ on one identifier as J writes it: `_` is `-`.
// The wildcard `_` is itself.
func Unspell(s string) string {
	if s == "_" {
		return s
	}
	return strings.ReplaceAll(s, "_", "-")
}

// Spell is σ_J on a name as today's notation writes it: a bare name, or a
// qualified one, `alias.member`, whose member may be any word (§1.4).
func Spell(n string) string {
	i := strings.Index(n, ".")
	if i <= 0 || i == len(n)-1 {
		return word(n, false)
	}
	return word(n[:i], false) + "." + word(n[i+1:], true)
}

// Collisions is the pairs of distinct names σ_J spells alike (spec/j.md §5, J8).
func Collisions(names []string) [][2]string {
	seen := map[string]string{}
	var out [][2]string
	for _, n := range names {
		s := Spell(n)
		if m, ok := seen[s]; ok && m != n {
			out = append(out, [2]string{m, n})
			continue
		}
		seen[s] = n
	}
	return out
}

// langSets are the language's sets, spelled by the convention (ADR 0067 item 3).
var langSets = map[string]string{"int": "Int", "bool": "Bool", "string": "String", "f64": "F64",
	"any": "Any", "option": "Option", "array": "Array", "map": "Map"}

// SetName is a set's name by the convention: UpperCamelCase.
func SetName(n string) string {
	if s, ok := langSets[n]; ok {
		return s
	}
	var b strings.Builder
	for _, part := range strings.FieldsFunc(n, func(r rune) bool { return r == '-' || r == '_' }) {
		rs := []rune(part)
		b.WriteRune(unicode.ToUpper(rs[0]))
		b.WriteString(string(rs[1:]))
	}
	return b.String()
}

// Integer spells an integer literal: a power of two, or one less, or its
// negation, as `2^k` from 2¹⁶ up, so that a literal says which bound it is
// (ADR 0067 item 4); every other integer as written.
func Integer(text string) string {
	v, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return text
	}
	a := new(big.Int).Abs(v)
	k := a.BitLen()
	if k < 17 {
		return text
	}
	one := big.NewInt(1)
	pow := new(big.Int).Lsh(one, uint(k-1))
	sign := ""
	if v.Sign() < 0 {
		sign = "-"
	}
	switch {
	case a.Cmp(pow) == 0:
		return sign + "2^" + itoa(k-1)
	case a.Cmp(new(big.Int).Sub(new(big.Int).Lsh(one, uint(k)), one)) == 0 && v.Sign() > 0:
		return "2^" + itoa(k) + "-1"
	}
	return text
}

func itoa(n int) string { return big.NewInt(int64(n)).String() }
