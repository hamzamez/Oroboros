//go:build ignore

// THE SIZE OF INSTALLMENT 2'S PROGRAMS, PER NOTATION (docs/surface-design.md §2.5, §2.9).
//
//	go run gauntlet/surface/tokens.go              installment 2: §2.4's P1–P8, and §2.9's K and L
//	go run gauntlet/surface/tokens.go -section 3   installment 3: §3.5's C1–C8 in T, B′, D, K and J
//	go run gauntlet/surface/tokens.go -section 4   installment 4: §4.5's six programs in T, D and J
//	go run gauntlet/surface/tokens.go -section 5   installment 5: §5.5's five programs in D and J
//
// Reads surface-design.md itself, so a table and the programs it counts cannot
// drift apart. Counted: §2.4's P1–P8 in T, B′, D and J (each fenced block whose
// first line ends with its column marker; P9 has no T and is left out), and
// §2.9's K and L blocks, which hold P1–P8 each.
//
// THE RULE, fixed here because the first tables' script was not kept:
//   - column markers (`; T`, `; B′`, `\\ D`, `// J`, `\\ Pn`, `// Pn`) and `;;`
//     lines are stripped, and a comment: `;` in the prefix notations (T, B′),
//     `\*` in D and `//` in J;
//   - a string literal is one token;
//   - a word is one token: letters, digits and `_`, joined by `.` before a
//     letter (`os.Args`), and in the prefix notations by `-` before a letter
//     (`print-line`), since there `-` cannot be an operator inside a word;
//   - every other non-space character is one token: `:=` is two, `…` one;
//     a second count, LEXEMES, takes a run of operator characters as one
//     token (`:=`, `==`, `->`, `|->`, `>=`, `..`), as a lexer would;
//   - depth and the longest closing run count ( [ { ⟨ and their partners; a run
//     is consecutive closers with only spaces between them;
//   - lines are the non-empty lines.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

type measure struct{ tokens, lexemes, depth, run, lines int }

var marker = regexp.MustCompile(`\s+(;\s*(T|B′)|\\\\\s*(D|K|P\d)|//\s*(J|P\d))\s*$`)

func count(text string, prefix bool, m *measure) {
	for _, line := range strings.Split(text, "\n") {
		line = marker.ReplaceAllString(line, "")
		if strings.HasPrefix(strings.TrimSpace(line), ";;") {
			continue
		}
		if prefix {
			line = stripComment(line)
		} else {
			line = stripInfixComment(line)
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		m.lines++
		toks := tokenize(line, prefix)
		m.tokens += len(toks)
		m.lexemes += len(lexemes(toks))
	}
	depth, run := 0, 0
	for _, line := range strings.Split(text, "\n") {
		line = marker.ReplaceAllString(line, "")
		if strings.HasPrefix(strings.TrimSpace(line), ";;") {
			continue
		}
		if prefix {
			line = stripComment(line)
		} else {
			line = stripInfixComment(line)
		}
		inStr := false
		for _, r := range line {
			if r == '"' {
				inStr = !inStr
				run = 0
				continue
			}
			if inStr {
				continue
			}
			switch {
			case strings.ContainsRune("([{⟨", r):
				depth++
				if depth > m.depth {
					m.depth = depth
				}
				run = 0
			case strings.ContainsRune(")]}⟩", r):
				depth--
				run++
				if run > m.run {
					m.run = run
				}
			case unicode.IsSpace(r):
			default:
				run = 0
			}
		}
	}
}

func stripComment(line string) string {
	inStr := false
	for i, r := range line {
		if r == '"' {
			inStr = !inStr
		}
		if r == ';' && !inStr {
			return line[:i]
		}
	}
	return line
}

// stripInfixComment drops a comment in D (`\*`) or J (`//`), outside a string.
func stripInfixComment(line string) string {
	inStr := false
	for i := 0; i+1 < len(line); i++ {
		if line[i] == '"' {
			inStr = !inStr
		}
		if !inStr && ((line[i] == '/' && line[i+1] == '/') || (line[i] == '\\' && line[i+1] == '*')) {
			return line[:i]
		}
	}
	return line
}

func word(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func tokenize(line string, prefix bool) []string {
	rs := []rune(line)
	var toks []string
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				if rs[j] == '\\' {
					j++
				}
				j++
			}
			toks = append(toks, string(rs[i:min(j+1, len(rs))]))
			i = j + 1
		case word(r):
			j := i
			for j < len(rs) {
				if word(rs[j]) {
					j++
					continue
				}
				join := rs[j] == '.' || (prefix && rs[j] == '-')
				if join && j+1 < len(rs) && unicode.IsLetter(rs[j+1]) {
					j++
					continue
				}
				break
			}
			toks = append(toks, string(rs[i:j]))
			i = j
		default:
			toks = append(toks, string(r))
			i++
		}
	}
	return toks
}

// lexemes joins adjacent one-character operator tokens into one.
func lexemes(toks []string) []string {
	op := func(t string) bool { return len(t) == 1 && strings.Contains(":=<>|-.!&", t) }
	var out []string
	for i, t := range toks {
		if i > 0 && op(t) && op(toks[i-1]) {
			out[len(out)-1] += t
			continue
		}
		out = append(out, t)
	}
	return out
}

func main() {
	section := flag.String("section", "2", "the installment whose programs to count: 2, 3, 4 or 5")
	flag.Parse()
	b, err := os.ReadFile("docs/surface-design.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	doc := string(b)
	block := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	sec := func(from, to string) string {
		i := strings.Index(doc, from)
		j := strings.Index(doc[i:], to)
		return doc[i : i+j]
	}
	res := map[string]*measure{}
	order := []string{"T", "B′", "D", "J", "K", "L"}
	from, to := "### 2.4 Nine programs", "**P9"
	switch *section {
	case "3":
		order = []string{"T", "B′", "D", "K", "J"}
		from, to = "### 3.5 Eight programs", "### 3.6 Measured"
	case "4":
		order = []string{"T", "D", "J"}
		from, to = "### 4.5 Six programs", "### 4.6 Measured"
	case "5":
		// Most of installment 5's claims have no spelling in today's notation,
		// so D and J are compared with each other.
		order = []string{"D", "J"}
		from, to = "### 5.5 Five programs", "### 5.6 Measured"
	}
	for _, n := range order {
		res[n] = &measure{}
	}
	// A block belongs to the notation its first line's marker names.
	firstMarker := regexp.MustCompile(`(;\s*(T|B′)|\\\\\s*(D|K)|//\s*(J))\s*$`)
	for _, m := range block.FindAllStringSubmatch(sec(from, to), -1) {
		first := strings.SplitN(m[1], "\n", 2)[0]
		f := firstMarker.FindStringSubmatch(first)
		if f == nil {
			continue
		}
		n := f[2] + f[3] + f[4]
		if res[n] == nil {
			continue // a notation this section does not count
		}
		count(m[1], n == "T" || n == "B′", res[n])
	}
	if *section == "2" {
		k := block.FindStringSubmatch(sec("#### K — statements", "#### L — a binder"))
		count(k[1], false, res["K"])
		l := block.FindStringSubmatch(sec("#### L — a binder", "#### Measured"))
		count(l[1], false, res["L"])
	}
	baseName := order[0]
	t, x := float64(res[baseName].tokens), float64(res[baseName].lexemes)
	fmt.Printf("| notation | tokens | × %s | lexemes | × %s | depth | closing run | lines |\n", baseName, baseName)
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|---:|")
	for _, n := range order {
		m := res[n]
		fmt.Printf("| %s | %d | %.2f | %d | %.2f | %d | %d | %d |\n", n, m.tokens, float64(m.tokens)/t,
			m.lexemes, float64(m.lexemes)/x, m.depth, m.run, m.lines)
	}
}
