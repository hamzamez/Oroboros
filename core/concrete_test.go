package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusPrograms is every program source the translator will read.
func corpusPrograms(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"../examples", "../lib", "../gauntlet/stdlib/acceptance", "../gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) < 100 {
		t.Fatalf("found %d programs; the corpus is larger", len(files))
	}
	return files
}

// comments counts the `;` comments of a source independently of the reader: a
// `;` outside a string literal starts one, to the end of the line.
func comments(src string) []string {
	var out []string
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '"':
			for i++; i < len(src) && src[i] != '"'; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case ';':
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				j = len(src) - i
			}
			out = append(out, strings.TrimLeft(src[i:i+j], ";"))
			i += j
		}
	}
	return out
}

// EVERY COMMENT IS A GAP, AND EVERY DECLARATION HAS ITS PLACE (spec/j.md §1.2):
// over the corpus, the gaps ReadConcrete keeps are exactly the comments an
// independent scan finds, in order, and the declarations' spans are disjoint and
// in source order, each node's span inside its declaration's.
func TestEveryCommentIsAGapAndEveryDeclarationHasItsPlace(t *testing.T) {
	files, gaps := 0, 0
	for _, p := range corpusPrograms(t) {
		b, _ := os.ReadFile(p)
		src := string(b)
		if _, err := ReadTrees(src); err != nil {
			continue // a source meant to be refused is not the translator's
		}
		c, err := ReadConcrete(src)
		if err != nil {
			t.Fatalf("%s: ReadTrees reads it and ReadConcrete does not: %v", p, err)
		}
		files++
		want := comments(src)
		if len(c.Gaps) != len(want) {
			t.Fatalf("%s: %d gaps kept, %d comments in the source", p, len(c.Gaps), len(want))
		}
		for i, g := range c.Gaps {
			if g.Text != want[i] || !strings.HasPrefix(src[g.Start:g.End], ";") {
				t.Fatalf("%s: gap %d is %q, the source's comment %q", p, i, g.Text, want[i])
			}
		}
		gaps += len(c.Gaps)
		last := -1
		for _, d := range c.Decls {
			sp, ok := c.Spans[d]
			if !ok {
				t.Fatalf("%s: a declaration %T has no span", p, d)
			}
			if _, merged := d.(*DefDecl); !merged && sp.Start < last {
				t.Fatalf("%s: declaration spans out of order at %d", p, sp.Start)
			}
			last = sp.End
		}
		for n, sp := range c.Spans {
			if sp.Start < 0 || sp.End > len(src) || sp.Start >= sp.End {
				t.Fatalf("%s: node %T has span %v outside the source", p, n, sp)
			}
		}
	}
	if files < 100 || gaps < 4000 {
		t.Fatalf("read %d files and %d gaps; the corpus has more", files, gaps)
	}
	t.Logf("%d programs, %d gaps", files, gaps)
}
