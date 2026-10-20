package core

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

// READING A SOURCE IN J (spec/j.md; ADR 0068). Sources are written in J, and
// the compiler reads their declarations as the forms core loads: J's reader
// builds abstract.md's trees, and D makes the forms (FormsOf). core reads
// no source itself; a driver hands it what J read.

// ReadEntry reads a program's entry file: its module is the root, "".
func ReadEntry(text, file string) ([]Form, error) { return ReadModule(text, file, "") }

// ModulePath is the module a file compiled directly is (ADR 0064 item 1: a
// file is a theory named by its path): its path relative to a library
// directory that holds it, `lib/num/u128.oro` being num/u128 on the path lib,
// or "", the root, for a program's entry file. libraries are the search path's
// named directories, not the file's own, which every program is found in. A
// library compiled directly has its public names as its entry points, as
// today's header's exports were.
func ModulePath(file string, libraries []string) string {
	abs, _ := filepath.Abs(file)
	for _, d := range libraries {
		ad, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(ad, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		return strings.TrimSuffix(filepath.ToSlash(rel), ".oro")
	}
	return ""
}

// ReadModule reads a module's source, the module named by the path it was
// found under (ADR 0064 item 1: a file is a theory named by its path, and J
// writes no header).
func ReadModule(text, file, module string) ([]Form, error) {
	dir := filepath.Dir(file)
	decls, err := ReadProgram(text, module, dir, Root(dir))
	if err != nil {
		return nil, err
	}
	return FormsOf(decls)
}

// ReadTargetSource reads a target file or a fragment: the model's forms, in a
// `(target T …)` wrapper when target names the model whose directory or file
// it is (ADR 0064 item 9: the target is its directory), as the target loader
// reads them.
func ReadTargetSource(text, file, target string) ([]*Term, error) {
	return ReadTarget(text, target, filepath.Dir(file), Root(filepath.Dir(file)))
}

// IsFragment reports a source holding a target's fragment, `provides T M { … }`:
// the target loader's to read, no program. It looks for a line that begins
// with the word, not for a parse, so that a fragment that does not parse is
// still the loader's and refused naming its file (hoststring-2026-09-24), never
// skipped.
func IsFragment(text string) bool {
	for _, line := range strings.Split(text, string(rune(10))) { // each line
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "provides ") {
			return true
		}
	}
	return false
}

// SearchPath finds a module on a search path, `use num/vec` in DIR/num/vec.oro,
// and reads it in J. A path with no file is one the target provides.
func SearchPath(dirs []string) Resolver {
	return func(path string) ([]Form, bool, error) {
		for _, d := range dirs {
			file := filepath.Join(d, filepath.FromSlash(path)+".oro")
			b, err := os.ReadFile(file)
			if err == nil {
				forms, err := ReadModule(string(b), file, path)
				if err != nil {
					return nil, true, err
				}
				return forms, true, nil
			}
			if !os.IsNotExist(err) {
				return nil, false, err
			}
		}
		return nil, false, nil
	}
}

// Root is the directory holding go.mod above dir, whose lib/ a source's imports
// are read from.
func Root(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "."
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return "."
		}
	}
}

// Read reads a program's entry source in J: its module the root.
func Read(text string) ([]Form, error) { return ReadEntry(text, "") }

// ReadAll reads a model's forms in J, a target file or a fragment, with no
// target named.
func ReadAll(text string) ([]*Term, error) { return ReadTargetSource(text, "", "") }

// ReadTerm reads one expression in J and gives D of it, the core term it means.
func ReadTerm(text string) (*Term, error) {
	toks, err := lex(text)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, sets: map[string]string{"Option": "option"}, folded: map[Expr]*big.Int{},
		indexed: map[Expr]bool{}}
	t, err := p.tail()
	if err != nil {
		return nil, err
	}
	p.endOfStatement()
	if p.peek().kind != tEOF {
		return nil, p.errorf("%s after the expression", p.peek())
	}
	return Desugar(t), nil
}
