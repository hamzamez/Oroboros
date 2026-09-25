package ir

import (
	"os"
	"strings"

	"oroboros/emit"
)

// WriteFile verifies a lowered program and writes its canonical text to path,
// or, when lowering failed (errs) or verification refuses it, the reasons to
// path.err. It is what `gen -ir` and `build -ir` share: the IR is written
// beside the code and never changes what is emitted, so nothing here reaches
// stderr (docs/spec/ir.md §11).
func WriteFile(path string, tg *emit.Target, p *Program, errs []string) error {
	_ = os.Remove(path)
	_ = os.Remove(path + ".err")
	if len(errs) == 0 {
		if err := Verify(tg, p); err != nil {
			// The refused program follows its reasons, so a refusal can be read
			// against the text it refers to.
			errs = append(errs, "verify:\n"+err.Error(), "\n; the program refused:\n"+Print(p))
		}
	}
	if len(errs) > 0 {
		return os.WriteFile(path+".err", []byte(strings.Join(errs, "\n")+"\n"), 0o644)
	}
	return os.WriteFile(path, []byte(Print(p)), 0o644)
}

// CheckFile reads what WriteFile wrote and reports whether it is a program
// whose canonical printing round-trips; the reason when it is not.
func CheckFile(path string) (string, bool) {
	if b, err := os.ReadFile(path + ".err"); err == nil {
		return strings.TrimSpace(string(b)), false
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return "no IR was written", false
	}
	p, err := Read(string(text))
	if err != nil {
		return "the reader refuses the printer's text: " + err.Error(), false
	}
	if Print(p) != string(text) {
		return "print ∘ read ∘ print ≠ print", false
	}
	return "", true
}
