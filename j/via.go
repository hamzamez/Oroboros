package j

import "oroboros/core"

// ModuleOf is a library file's module path in today's notation, its one
// `(module P)` standing first; "" for an entry file. The translator's.
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
