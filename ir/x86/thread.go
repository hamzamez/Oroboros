package x86

import "strings"

// threadJumps is jump threading and unreachable-code removal over the
// printer's own output, both classical and both preserving every path:
//
//   - a jump to a label whose first instruction is `jmp X` goes to X directly
//     (the label is a trampoline, and the path through it is the same path);
//   - after an unconditional `jmp`, instructions up to the next label anything
//     still jumps to are reached by no path, and are dropped.
//
// Jumping code produces trampolines by construction: a boolean `if` whose arm
// yields a constant jumps to that arm's label, and the arm is one `jmp`
// (booleans.md §2.7 wants both failures of `and` to leave for ONE label). A
// template's own labels (indented, `Lstr13:`) are left alone: a template is
// opaque data, and its jumps stay inside it.
func threadJumps(src string) string {
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	for pass := 0; pass < 8; pass++ {
		// A trampoline: an unindented label whose next instruction is `jmp X`.
		to := map[string]string{}
		for i, l := range lines {
			if !isOwnLabel(l) {
				continue
			}
			for j := i + 1; j < len(lines); j++ {
				t := strings.TrimSpace(lines[j])
				if isOwnLabel(lines[j]) {
					continue
				}
				if x, ok := strings.CutPrefix(t, "jmp "); ok && x != labelOf(l) {
					to[labelOf(l)] = x
				}
				break
			}
		}
		final := func(l string) string {
			for k := 0; k < 16; k++ {
				x, ok := to[l]
				if !ok {
					break
				}
				l = x
			}
			return l
		}
		changed := false
		for i, l := range lines {
			op, target, ok := jumpOf(l)
			if !ok {
				continue
			}
			if f := final(target); f != target {
				lines[i] = "        " + op + " " + f
				changed = true
			}
		}
		// Unreachable: after an unconditional jmp, up to a referenced label.
		referenced := map[string]bool{}
		for _, l := range lines {
			if _, target, ok := jumpOf(l); ok {
				referenced[target] = true
			}
		}
		var out []string
		dead := false
		for _, l := range lines {
			if isOwnLabel(l) || isTemplateLabel(l) {
				if referenced[labelOf(l)] || isTemplateLabel(l) {
					dead = false
				}
				if !referenced[labelOf(l)] && !isTemplateLabel(l) && isOwnLabel(l) && !keepLabel(l) {
					changed = true
					continue // nothing jumps here any more
				}
				out = append(out, l)
				continue
			}
			if dead {
				changed = true
				continue
			}
			out = append(out, l)
			if t := strings.TrimSpace(l); strings.HasPrefix(t, "jmp ") {
				dead = true
			}
		}
		lines = out
		if !changed {
			break
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// isOwnLabel: a label the printer wrote, at column 0.
func isOwnLabel(l string) bool {
	return strings.HasSuffix(l, ":") && l != "" && l[0] != ' ' && !strings.ContainsAny(l, " \t")
}

// isTemplateLabel: a label a template carries, indented like an instruction.
func isTemplateLabel(l string) bool {
	t := strings.TrimSpace(l)
	return l != t && strings.HasSuffix(t, ":") && !strings.ContainsAny(t, " \t")
}

func labelOf(l string) string { return strings.TrimSuffix(strings.TrimSpace(l), ":") }

// keepLabel: the procedure's return label is the fall-through target of its
// last yield, so it stays even when no jump names it.
func keepLabel(l string) bool { return strings.HasPrefix(labelOf(l), "Lret") }

// jumpOf reads `jmp X` or `jcc X`.
func jumpOf(l string) (op, target string, ok bool) {
	t := strings.TrimSpace(l)
	if !strings.HasPrefix(t, "j") {
		return "", "", false
	}
	op, target, ok = strings.Cut(t, " ")
	if !ok || strings.ContainsAny(target, " ,[") {
		return "", "", false
	}
	return op, strings.TrimSpace(target), true
}
