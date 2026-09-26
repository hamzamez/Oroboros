package x86

import (
	"strings"
	"testing"
)

// TestThreadingKeepsEveryPath: jump threading and unreachable-code removal
// change which labels a jump names, never where control goes.
func TestThreadingKeepsEveryPath(t *testing.T) {
	src := strings.Join([]string{
		"        cmp rbx, rdi",
		"        jge La",     // → La → Lb → Lc: a chain of trampolines
		"        mov rax, 1", // reachable: falls through
		"        jmp Lret1",
		"        mov rax, 9", // unreachable: after a jmp, before a referenced label
		"La:",
		"        jmp Lb",
		"Lb:",
		"        jmp Lc",
		"Lc:",
		"        mov rax, 0",
		"        mov rax, r10",
		"        Lstr7:", // a template's label and loop: opaque, untouched
		"        cmp byte ptr [rax], 0",
		"        je Lstrend7",
		"        inc rax",
		"        jmp Lstr7",
		"        Lstrend7:",
		"Lret1:",
	}, "\n")
	out := threadJumps(src)
	if !strings.Contains(out, "jge Lc") {
		t.Errorf("the trampoline chain was not threaded to its end:\n%s", out)
	}
	if strings.Contains(out, "mov rax, 9") {
		t.Errorf("code after an unconditional jmp survived:\n%s", out)
	}
	for _, keep := range []string{"mov rax, 1", "mov rax, 0", "Lstr7:", "je Lstrend7", "jmp Lstr7", "Lstrend7:", "Lret1:", "Lc:"} {
		if !strings.Contains(out, keep) {
			t.Errorf("%q was lost:\n%s", keep, out)
		}
	}
	// The trampolines themselves are gone: nothing jumps to La or Lb.
	if strings.Contains(out, "La:") || strings.Contains(out, "Lb:") {
		t.Errorf("an unreferenced trampoline survived:\n%s", out)
	}
}
