// Command build turns an Oroboros program into an artifact.
//
//	build -target=go -o hello examples/hello.oro
//
// It follows the entry file's imports, reduces the export named `main`, emits
// one complete source file, and hands it to the host's own toolchain — which is
// declared in the target file as data, like everything else (docs/spec/build.md).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

func main() {
	target := flag.String("target", "go", "target to build for")
	dir := flag.String("targets", "targets", "directory holding target declarations")
	path := flag.String("path", "lib", "search path for imported modules")
	out := flag.String("o", "", "artifact to write (default: the source's stem)")
	checkedFlag := flag.Bool("checked", false,
		"rewrite integer operations the compiler cannot bound to the target's checked form")
	keep := flag.Bool("keep", false, "keep the emitted source and print where it is")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: build [-target=NAME] [-o ARTIFACT] SRC.oro\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), *target, *out, *path, *keep, *checkedFlag); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, path string, keep, checked bool) error {
	tg, err := emit.LoadTarget(filepath.Join(targetDir, target+".oro"))
	if err != nil {
		return err
	}
	if tg.Build == "" && tg.Artifact == "" {
		return fmt.Errorf("target %q declares neither (build …) nor (artifact …), so it can "+
			"emit source but not produce a deliverable; use cmd/gen instead", target)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	prog, _, err := core.LoadWith(forms, fileResolver(libDirs(src, path)))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// Name resolution, over EVERY definition rather than only what reduction
	// reaches — a typo in unused code was previously invisible.
	if err := env.CheckProgram(nil); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	for _, n := range env.Shadowed() {
		fmt.Fprintf(os.Stderr, "note: %s is defined here and provided natively by target %q; "+
			"the target's is used\n", n, target)
	}

	// A signature is checked against the TARGET's native implementation as
	// well as against the definition — the one job no host compiler can do,
	// since the two live on different targets (docs/spec/types.md).
	if err := emit.CheckSignatures(tg, prog, env); err != nil {
		return err
	}

	// The entry point is an export named `main` taking no arguments — build.md
	// §2. Distinguished by name and arity, never by module.
	entry := ""
	for _, q := range prog.Exports {
		if q == "main" || strings.HasSuffix(q, ".main") {
			entry = q
			break
		}
	}
	if entry == "" {
		return fmt.Errorf("%s has no entry point: a program needs `(export main)` where main "+
			"is `(fn () …)`", src)
	}
	nf, err := core.Normalize(prog.Defs[entry], env, core.DefaultFuel)
	if err != nil {
		return err
	}
	if nf.Kind != core.KFn || len(nf.Params) != 0 {
		return fmt.Errorf("main must take no arguments, got %s", nf)
	}
	if left := core.Residual(nf, env); len(left) > 0 {
		return fmt.Errorf("not in normal form for target %q: %s", target, strings.Join(left, ", "))
	}

	// Check the residual before emitting it (docs/spec/types.md). On Go and
	// Java the host would catch most of this; on JavaScript nothing would.
	if err := emit.Check(tg, entry, nf); err != nil {
		return err
	}
	// Refinements: the bounds obligation primitives.md §2 recorded and
	// nothing checked (docs/spec/refinements.md).
	// ADR 0018's linearity, checked on the residual rather than by a type.
	if err := emit.CheckLinear(nf, tg); err != nil {
		return fmt.Errorf("%s: %w", entry, err)
	}
	if notes, err := emit.Refine(tg, entry, prog.Sigs[entry], nf); err != nil {
		return err
	} else {
		for _, n := range notes {
			fmt.Fprintln(os.Stderr, "note:", n)
		}
	}
	// REPRESENTATION SELECTION — see cmd/gen for the note.
	// A POSTCONDITION on an exported definition is an OBLIGATION, not an
	// assumption: the caller is outside the program (postconditions.md §2).
	if ok, note := emit.CheckEnsures(tg, prog.Sigs[entry], nf); !ok {
		return fmt.Errorf("%s: %s", entry, note)
	} else if note != "" {
		fmt.Fprintln(os.Stderr, "note:", entry+": "+note)
	}
	rep, sel := emit.Intervals(tg, prog.Sigs[entry], nf, 0)
	if rep.Ops > 0 || rep.Loops > 0 {
		fmt.Fprintf(os.Stderr, "note: %d of %d integer operations bounded; "+
			"%d of %d loop(s) proven terminating\n",
			rep.Proven, rep.Ops, rep.Terminates, rep.Loops)
	}
	if checked {
		nf = sel
	}
	var code string
	switch target {
	case "js":
		code, err = emit.JSFunc(tg, "oro-main", prog.Sigs[entry], nf)
	case "java":
		code, err = emit.JavaMethod(tg, "oro-main", prog.Sigs[entry], nf)
	case "windows":
		code, err = emit.AsmProc(tg, "oro-main", prog.Sigs[entry], nf)
		if err == nil {
			code = emit.AsmFile(tg, map[string]string{"oro-main": code}, "oro-main")
		}
	default:
		code, err = emit.Func(tg, "oro-main", prog.Sigs[entry], nf)
	}
	if err != nil {
		return err
	}

	if out == "" {
		out = strings.TrimSuffix(filepath.Base(src), ".oro")
	}
	if out, err = filepath.Abs(out); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "oro-build-")
	if err != nil {
		return err
	}
	if !keep {
		defer os.RemoveAll(work)
	}
	if err := tg.WriteProgram(work, code, "oro-main"); err != nil {
		return err
	}

	// A host with no compile step delivers the emitted source itself. Copy it
	// first, so a `build` command that only checks has something to check —
	// and again afterwards, for a toolchain that PRODUCES the artifact rather
	// than being handed a destination. `go build -o` takes one; ml64 and link
	// do not, and neither does any toolchain driven through a script.
	copyArtifact := func(must bool) error {
		if tg.Artifact == "" {
			return nil
		}
		b, err := os.ReadFile(filepath.Join(work, tg.Artifact))
		if err != nil {
			if !must && os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return os.WriteFile(out, b, 0o644)
	}
	if err := copyArtifact(false); err != nil {
		return err
	}
	// Print this BEFORE running the toolchain: a failed build is exactly when
	// you need to see the source, and printing it afterwards hid it.
	if keep {
		fmt.Println("source kept in", work)
	}
	if tg.Build != "" {
		argv := strings.Fields(emit.Fill(tg.Build, out, work))
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = work
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
		}
	}
	if keep {
		fmt.Printf("source kept in %s\n", work)
	}
	if err := copyArtifact(true); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", out)
	return nil
}

// fileResolver finds a module on a search path; a path with no file is one the
// target provides rather than a library.
func fileResolver(dirs []string) core.Resolver {
	return func(p string) (string, bool, error) {
		for _, d := range dirs {
			f := filepath.Join(d, filepath.FromSlash(p)+".oro")
			b, err := os.ReadFile(f)
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

func libDirs(entry, extra string) []string {
	dirs := []string{filepath.Dir(entry)}
	for _, d := range filepath.SplitList(extra) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}
