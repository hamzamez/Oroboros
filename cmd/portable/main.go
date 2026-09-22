// Command portable reports a program's portability — ADR 0026 (8).
//
// Portability is a property the compiler COMPUTES, not one the language
// guarantees (ADR 0001, and hamza's decision of 2026-09-22 for integers). For
// a program P and the targets T it is compiled for:
//
//	A(P) = { T : T accepts P }          P is portable to S  ⟺  S ⊆ A(P)
//
// and the integer window survives only as a DERIVED number, the meet of the
// accepting targets' words:
//
//	W(A(P)) = ⋂_{T ∈ A(P)} word_T
//
// which is exactly ADR 0012's window when A(P) = {go, java, js}, and is now
// reported rather than assumed. A refused target is listed with the first
// line of its refusal, which names the target's word when the reason is an
// integer (emit/bounded.go).
//
//	go run ./cmd/portable examples/dot.oro
//	go run ./cmd/portable -require go,js examples/dot.oro   # exit 1 unless portable to both
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	targetList := flag.String("targets", "go,js,java,windows", "the targets to compile for")
	require := flag.String("require", "", "exit 1 unless the program is portable to these targets (comma-separated)")
	path := flag.String("path", "lib", "search path for imported modules")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: portable [-targets go,js,java,windows] [-require T,…] SRC.oro\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	ok, err := run(flag.Arg(0), split(*targetList), split(*require), *path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "portable:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func split(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

type verdict struct {
	target   string
	accepted bool
	detail   string // the integer note when accepted, the refusal's first line when not
}

func run(src string, targets, require []string, path string) (bool, error) {
	work, err := os.MkdirTemp("", "oro-portable-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(work)
	gen := filepath.Join(work, "gen.exe")
	if out, err := exec.Command("go", "build", "-o", gen, "./cmd/gen").CombinedOutput(); err != nil {
		return false, fmt.Errorf("building cmd/gen: %v\n%s", err, out)
	}

	vs := make([]verdict, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t string) {
			defer wg.Done()
			out := filepath.Join(work, "out."+t)
			b, err := exec.Command(gen, "-path", path, src, t, out).CombinedOutput()
			vs[i] = verdict{target: t, accepted: err == nil, detail: summarize(string(b), err == nil)}
		}(i, t)
	}
	wg.Wait()

	fmt.Println(src)
	var accepted, refused []string
	for _, v := range vs {
		state := "refused "
		if v.accepted {
			state = "accepted"
			accepted = append(accepted, v.target)
		} else {
			refused = append(refused, v.target)
		}
		fmt.Printf("  %-8s %s  %s\n", v.target, state, v.detail)
	}

	switch {
	case len(accepted) == 0:
		fmt.Println("portable to no target")
	case len(refused) == 0:
		fmt.Printf("portable to %s\n", strings.Join(accepted, ", "))
	default:
		fmt.Printf("portable to %s — not %s\n", strings.Join(accepted, ", "), strings.Join(refused, ", "))
	}
	if len(accepted) > 0 {
		w, err := meet(accepted)
		if err != nil {
			return false, err
		}
		fmt.Printf("W(%s) = %s   (the meet of their words: derived, not assumed — ADR 0026)\n",
			strings.Join(accepted, ", "), w)
	}

	ok := true
	for _, r := range require {
		if !contains(accepted, r) {
			ok = false
		}
	}
	if len(require) > 0 && !ok {
		fmt.Printf("NOT portable to the required %s\n", strings.Join(require, ", "))
	}
	return ok, nil
}

// meet is ⋂ word_T, the interval every accepting target computes natively.
func meet(targets []string) (core.Word, error) {
	var w core.Word
	for i, t := range targets {
		tg, err := emit.LoadTargetLayers(t, []string{"targets"})
		if err != nil {
			return core.Word{}, err
		}
		if i == 0 || tg.Word.Lo > w.Lo {
			w.Lo = tg.Word.Lo
		}
		if i == 0 || tg.Word.Hi < w.Hi {
			w.Hi = tg.Word.Hi
		}
	}
	return w, nil
}

// summarize keeps the line a person needs: gen's integer note for an accepted
// program, and the refusal's first line for a refused one.
func summarize(out string, accepted bool) string {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	if accepted {
		// THE SAME INTEGER, NOT THE SAME COST: a declared range above one
		// target's word is arbitrary precision there and a machine word
		// elsewhere, so the representation a target chose is part of the report.
		var keep []string
		for _, l := range lines {
			if strings.Contains(l, "integer operations bounded") || strings.Contains(l, "in arbitrary precision") {
				keep = append(keep, strings.TrimPrefix(l, "note: "))
			}
		}
		return strings.Join(keep, "; ")
	}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "note:") && l != "exit status 1" {
			return strings.TrimPrefix(l, "gen: ")
		}
	}
	return "refused"
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
