// Command oro reduces a program to normal form against a target.
//
// The whole thesis is visible from the command line: a target decides where
// reduction stops.
//
//	oro -target=blas examples/dot.oro           # stops at blas's own dot
//	oro -target=go   examples/native/dot-go.oro # reduces to one loop
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	target := flag.String("target", "go", "target whose primitive set defines the normal form")
	dir := flag.String("targets", "targets", "search path for target declarations; the source's own directory is always the nearest layer")
	path := flag.String("path", "lib", "search path for imported modules")
	steps := flag.Bool("steps", false, "print each top-level term before and after reduction")
	fuel := flag.Int("fuel", core.DefaultFuel, "maximum reduction steps")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: oro [-target=NAME] [-steps] FILE\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), *target, *fuel, *steps, *path); err != nil {
		fmt.Fprintf(os.Stderr, "oro: %v\n", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target string, fuel int, steps bool, path string) error {
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.ReadModule(string(text), src, core.ModulePath(src, filepath.SplitList(path)))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// THE TARGET IS LOADED FIRST, because it may carry definitions of its own —
	// `D_T`, target-system.md §6.2 — and those are part of the program.
	layers, err := emit.SearchPath(src, targetDir)
	if err != nil {
		return err
	}
	tg, err := emit.LoadTargetLayers(target, layers, libDirs(src, path))
	if err != nil {
		return err
	}
	prog, terms, err := core.LoadWithDefs(forms, core.SearchPath(libDirs(src, path)), tg.Defs)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		// `src`, not `path` — this said "lib:" on every purity error, naming the
		// module search path instead of the file the mistake is in.
		return fmt.Errorf("%s: %w", src, err)
	}
	// Name resolution, over EVERY definition rather than only what reduction
	// reaches — a typo in unused code was previously invisible.
	if err := env.CheckProgram(terms); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}

	for _, n := range env.Shadowed() {
		fmt.Fprintf(os.Stderr, "note: %s is defined here and provided natively by target %q; "+
			"the target's is used\n", n, target)
	}

	// A program's entry points are its exports; anonymous top-level terms are
	// the unnamed alternative, kept because they are convenient to experiment
	// with. Reduce whichever the file used.
	labels := make([]string, len(terms))
	for _, q := range prog.Exports {
		labels = append(labels, q)
		terms = append(terms, prog.Defs[q])
	}

	for i, t := range terms {
		if labels[i] != "" {
			fmt.Printf("%s =\n", labels[i])
		}
		if steps {
			fmt.Printf("     %s\n", t)
		}
		out, err := core.Normalize(t, env, fuel)
		if err != nil {
			return err
		}
		if steps {
			fmt.Printf("  ⟶  ")
		}
		fmt.Println(out)

		if left := core.Residual(out, env); len(left) > 0 {
			fmt.Fprintf(os.Stderr,
				"\n  not in normal form for target %q: %s\n"+
					"  %s is neither primitive on this target nor a definition in scope.\n"+
					"  Either declare it primitive, or provide a definition that lowers it.\n",
				target, strings.Join(left, ", "), plural(left))
		}
	}
	return nil
}

func plural(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return "each of these"
}

// libDirs is the search path: the entry file's own directory first, so a
// program can keep its modules beside it, then whatever -path adds.
func libDirs(entry, extra string) []string {
	dirs := []string{filepath.Dir(entry)}
	for _, d := range filepath.SplitList(extra) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}
