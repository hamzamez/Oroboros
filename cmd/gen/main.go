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
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gen [-targets DIR] [-name N] SRC.oro TARGET OUT\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), flag.Arg(1), flag.Arg(2), *name); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, name string) error {
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
	prog, terms, err := core.Load(forms)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	env := tg.Env(prog)

	if name == "" {
		name = "gen-" + strings.TrimSuffix(filepath.Base(src), ".oro")
	}
	funcs := map[string]string{}
	for i, t := range terms {
		nf, err := core.Normalize(t, env, core.DefaultFuel)
		if err != nil {
			return err
		}
		fname := name
		if len(terms) > 1 {
			fname = fmt.Sprintf("%s-%d", name, i)
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
