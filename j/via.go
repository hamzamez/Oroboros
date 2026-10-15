package j

import (
	"os"
	"path/filepath"
	"strings"

	"oroboros/core"
)

// ViaJ reads a source of today's notation through J: translated, then read by
// J's reader, then desugared. It is the drivers' path when ORO_VIA_J is set, so
// that cmd/check's whole emission sweep can be run on J's reading of every
// program and compared with the baseline (jread-2026-10-15). A target fragment
// is no program and is read as it always was.
func ViaJ(text, src string) ([]core.Form, error) {
	dir := filepath.Dir(src)
	root := repoRoot(dir)
	r, err := Translate(text, dir, root)
	if err != nil {
		if strings.Contains(err.Error(), "step 2d") {
			return core.Read(text)
		}
		return nil, err
	}
	decls, err := ReadProgram(r.J, ModuleOf(text), dir, root)
	if err != nil {
		return nil, err
	}
	return core.FormsOf(decls)
}

// ViaJLibraries makes the loader read every imported module through J too,
// found on the driver's search path as the driver's resolver finds it, so that
// the gate covers a program's libraries and not its entry file alone.
func ViaJLibraries(dirs []string) {
	core.ReadLibrary = func(src, path string) ([]core.Form, error) {
		for _, d := range dirs {
			file := filepath.Join(d, filepath.FromSlash(path)+".oro")
			if _, err := os.Stat(file); err == nil {
				return ViaJ(src, file)
			}
		}
		return core.Read(src)
	}
}

// ModuleOf is a library file's module path, its one `(module P)` standing first,
// which J does not write (ADR 0064 item 1); "" for an entry file.
func ModuleOf(src string) string {
	ds, err := core.ReadTrees(src)
	if err != nil {
		return ""
	}
	modules, first := 0, ""
	for _, d := range ds {
		if m, ok := d.(*core.ModuleDecl); ok {
			modules++
			if first == "" {
				first = m.Path
			}
		}
	}
	if modules == 1 && isModuleFirst(ds) {
		return first
	}
	return ""
}

// repoRoot is the directory holding go.mod above dir, whose lib/ a program's
// imports are read from.
func repoRoot(dir string) string {
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
