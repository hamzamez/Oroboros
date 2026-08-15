// Command gen compiles an Oroboros program for a target and writes the result.
//
// The target is loaded from targets/NAME.oro — a data file, not Go source. A
// program declares no targets of its own; which names are primitive comes
// entirely from the target file, which is the whole of ADR 0002's parameter.
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
	dir := flag.String("targets", "targets", "directory holding target declarations")
	name := flag.String("name", "", "name for the emitted function (defaults to the source's stem)")
	path := flag.String("path", "lib", "search path for imported modules")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gen [-targets DIR] [-name N] SRC.oro TARGET OUT\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), flag.Arg(1), flag.Arg(2), *name, *path); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, name, path string) error {
	tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
	if err != nil {
		return err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	prog, terms, err := core.LoadWith(forms, fileResolver(libDirs(src, path)))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// A signature is checked against the TARGET's native implementation as
	// well as against the definition — the one job no host compiler can do,
	// since the two live on different targets (docs/spec/types.md).
	if err := emit.CheckSignatures(tg, prog, env); err != nil {
		return err
	}

	// A program's entry points are its EXPORTS, and an emitted function is named
	// after the export it came from. Naming by position — GenGeneric0,
	// GenGeneric1 — was the last thing modules left unfinished (modules.md §1).
	//
	// A file with no `(export …)` still works: its anonymous top-level terms are
	// named from the source stem, as before. That is the whole of the fallback.
	type unit struct {
		name string
		qual string // the fully qualified export, so its signature can be found
		term *core.Term
	}
	var units []unit
	prefix := name
	if prefix == "" {
		prefix = "gen"
	}
	for _, q := range prog.Exports {
		local := q
		if i := strings.LastIndex(local, "."); i >= 0 {
			local = local[i+1:]
		}
		units = append(units, unit{name: prefix + "-" + local, qual: q, term: prog.Defs[q]})
	}
	if len(units) == 0 {
		stem := name
		if stem == "" {
			stem = "gen-" + strings.TrimSuffix(filepath.Base(src), ".oro")
		}
		for i, t := range terms {
			n := stem
			if len(terms) > 1 {
				n = fmt.Sprintf("%s-%d", stem, i)
			}
			units = append(units, unit{name: n, term: t})
		}
	}

	funcs := map[string]string{}
	for _, u := range units {
		nf, err := core.Normalize(u.term, env, core.DefaultFuel)
		if err != nil {
			return err
		}
		fname := u.name
		// Check the residual before emitting it (docs/spec/types.md). On Go and
		// Java the host would catch most of this; on JavaScript nothing would.
		if err := emit.Check(tg, fname, nf); err != nil {
			return err
		}
		// Refinements: the bounds obligation primitives.md §2 recorded and
		// nothing checked (docs/spec/refinements.md).
		if notes, err := emit.Refine(tg, fname, prog.Sigs[u.qual], nf); err != nil {
			return err
		} else {
			for _, n := range notes {
				fmt.Fprintln(os.Stderr, "note:", n)
			}
		}
		var code string
		switch target {
		case "js":
			code, err = emit.JSFunc(tg, fname, nf)
		case "java":
			code, err = emit.JavaMethod(tg, fname, nf)
		default:
			code, err = emit.Func(tg, fname, nf)
		}
		if err != nil {
			return err
		}
		funcs[fname] = code
	}

	var text2 string
	switch target {
	case "js":
		text2 = emit.JSFile(funcs)
	case "java":
		base := filepath.Base(out)
		text2 = emit.JavaFile(strings.TrimSuffix(base, ".java"), funcs)
	default:
		text2 = emit.File("gauntlet", funcs)
	}
	if err := os.WriteFile(out, []byte(text2), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", out)
	return nil
}

// fileResolver finds a module on a search path: `(use num/vec)` looks for
// num/vec.oro under each directory in turn. A path with no file is not an
// error — it is a module the TARGET provides, like go/strings.
func fileResolver(dirs []string) core.Resolver {
	return func(path string) (string, bool, error) {
		for _, d := range dirs {
			p := filepath.Join(d, filepath.FromSlash(path)+".oro")
			b, err := os.ReadFile(p)
			if err == nil {
				return string(b), true, nil
			}
			if !os.IsNotExist(err) {
				return "", false, err
			}
		}
		return "", false, nil
	}
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
