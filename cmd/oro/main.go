// Command oro reduces a program to normal form against a target.
//
// The whole thesis is visible from the command line: the same file, two
// targets, two normal forms.
//
//	oro -target=blas examples/dot.oro
//	oro -target=go   examples/dot.oro
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
	dir := flag.String("targets", "targets", "directory holding target declarations")
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
	if err := run(*dir, flag.Arg(0), *target, *fuel, *steps); err != nil {
		fmt.Fprintf(os.Stderr, "oro: %v\n", err)
		os.Exit(1)
	}
}

func run(targetDir, path, target string, fuel int, steps bool) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	forms, err := core.Read(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
	if err != nil {
		return err
	}
	env := tg.Env(prog)

	for _, t := range terms {
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
