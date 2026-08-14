// Command gen emits Oroboros source into the gauntlet's Go package, so that
// generated and hand-written code are benchmarked side by side in one binary
// against one baseline.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: gen SRC.oro TARGET OUT.go FUNCNAME")
		os.Exit(2)
	}
	src, err := os.ReadFile(os.Args[1])
	must(err)
	forms, err := core.Read(string(src))
	must(err)
	prog, terms, err := core.Load(forms)
	must(err)
	env, err := prog.Env(os.Args[2])
	must(err)

	funcs := map[string]string{}
	for i, t := range terms {
		nf, err := core.Normalize(t, env, core.DefaultFuel)
		must(err)
		name := os.Args[4]
		if len(terms) > 1 {
			name = fmt.Sprintf("%s-%d", name, i)
		}
		var code string
		switch os.Args[2] {
		case "js":
			code, err = emit.JSFunc(name, nf)
		case "java":
			code, err = emit.JavaMethod(name, nf)
		default:
			code, err = emit.Func(name, nf)
		}
		must(err)
		funcs[name] = code
	}
	var text string
	switch os.Args[2] {
	case "js":
		text = emit.JSFile(funcs)
	case "java":
		base := filepath.Base(os.Args[3])
		text = emit.JavaFile(strings.TrimSuffix(base, ".java"), funcs)
	default:
		text = emit.File("gauntlet", funcs)
	}
	must(os.WriteFile(os.Args[3], []byte(text), 0o644))
	fmt.Printf("wrote %s\n", os.Args[3])
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}
