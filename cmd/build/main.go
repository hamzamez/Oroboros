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
	"oroboros/ir"
	"oroboros/ir/golang"
	"oroboros/ir/java"
	"oroboros/ir/js"
)

func main() {
	target := flag.String("target", "go", "target to build for")
	dir := flag.String("targets", "targets", "search path for target declarations; the source's own directory is always the nearest layer")
	path := flag.String("path", "lib", "search path for imported modules")
	out := flag.String("o", "", "artifact to write (default: the source's stem)")
	checkedFlag := flag.Bool("checked", false,
		"rewrite integer operations the compiler cannot bound to the target's checked form")
	keep := flag.Bool("keep", false, "keep the emitted source and print where it is")
	flag.StringVar(&printer, "printer", "go,js,java", "the backends printed from the IR (ADR 0032), comma-separated: `go`, `js`, `java`; `terms` prints every backend from terms, as before the IR")
	flag.StringVar(&irOut, "ir", "", "also lower what the backend receives to the IR (docs/spec/ir.md), verify it, and write its canonical text to `FILE`, or the reason to FILE.err; it changes nothing that is built")
	bigRepr := flag.String("big-repr", "", "storage for a value above the target's word: `limbs` or `host`, overriding what the target declares. The BOUND is the declaration's either way, so this changes how a program is stored and not what it computes")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: build [-target=NAME] [-o ARTIFACT] SRC.oro\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*dir, flag.Arg(0), *target, *out, *path, *keep, *checkedFlag, *bigRepr); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(targetDir, src, target, out, path string, keep, checked bool, bigRepr string) error {
	layers, err := emit.SearchPath(src, targetDir)
	if err != nil {
		return err
	}
	tg, err := emit.LoadTargetLayers(target, layers, libDirs(src, path))
	if err != nil {
		return err
	}
	// WHICH CODE GENERATOR COMPILES THIS TARGET, asked of the TARGET and not of
	// the flag, and asked HERE so a target that cannot answer is refused before
	// anything is reduced (target-system.md §1.1).
	backend, err := tg.ResolveBackend()
	if err != nil {
		return err
	}
	// AN OVERRIDE, NOT A DECISION. The target declares which representation it
	// prefers, because that declaration is a measurement somebody took. This
	// exists so the alternative can be measured on the same program, and so the
	// two can be checked against each other — a change of storage that changed
	// an answer would be ADR 0009's rule broken at the representation boundary.
	if bigRepr != "" {
		if bigRepr != "limbs" && bigRepr != "host" {
			return fmt.Errorf("-big-repr is `limbs` or `host`, got %q", bigRepr)
		}
		tg.BigRepr = bigRepr
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
	// A TARGET FRAGMENT IS NOT A PROGRAM. `(provides TARGET PATH …)` is read by
	// the target loader — it is `(target T (module PATH …))` written where the
	// library lives — so a file that says nothing else has nothing to emit, and
	// saying so beats reporting the first name inside it as unbound.
	if core.OnlyFragments(forms) {
		return fmt.Errorf("%s is a target fragment — it declares (provides …) and nothing else, "+
			"so there is no program here to build", src)
	}
	prog, _, err := core.LoadWithDefs(forms, fileResolver(libDirs(src, path)), tg.Defs)
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
	// `▷` SAID OUT LOUD, one rung further in: a definition the target also
	// defines is the target's (target-system.md §6.2's `P_T ▷ D_T ▷ D`).
	for _, n := range prog.TargetDefined {
		fmt.Fprintf(os.Stderr, "note: %s is defined here and also by target %q; "+
			"the target's definition is used\n", n, target)
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
	// A DEFINITION'S CONTRACT IS CHECKED AT ITS CALLS (ADR 0028): decided
	// here, immediately after reduction, and erased before anything else runs.
	reqs := emit.InstallRequires(env, prog)
	nf, err := core.Normalize(prog.Defs[entry], env, core.DefaultFuel)
	if err != nil {
		return err
	}
	// A JOIN POINT IS NOT JUMPED OUT OF (tables.md §2.5): refused before any
	// pass walks a clause chain.
	if err := emit.CheckJoins(tg, nf); err != nil {
		return fmt.Errorf("%s: %w", entry, err)
	}
	if nf, err = emit.DischargeRequires(reqs, tg, entry, prog.Sigs[entry], nf); err != nil {
		return err
	}
	if nf.Kind != core.KFn || len(nf.Params) != 0 {
		return fmt.Errorf("main must take no arguments, got %s", nf)
	}
	if left := core.Residual(nf, env); len(left) > 0 {
		return fmt.Errorf("not in normal form for target %q: %s", target, strings.Join(left, ", "))
	}

	// THE PRODUCT, FLATTENED (emit/product.go). FIRST, because after it the term
	// is exactly what a hand-strided program is — ordinary tables and ordinary
	// index arithmetic — so nothing below this line, and no backend, learns that
	// products exist.
	esig := prog.Sigs[entry]
	if nfl, fsig, k, err := emit.FlattenProducts(tg, esig, nf); err != nil {
		return fmt.Errorf("%s: %w", entry, err)
	} else if k > 0 {
		nf, esig = nfl, fsig
		fmt.Fprintf(os.Stderr, "note: %d product access(es) flattened\n", k)
	}
	// ARBITRARY PRECISION, ADR 0019's THIRD ESCAPE (emit/bigrep.go). It runs
	// BEFORE the checker because the promotion is part of what the program
	// MEANS: the checker types `(* acc i)` as `int` and would refuse it against
	// a value the program has said is bigger than a word.
	nb, n, err := emit.PromoteBig(tg, esig, nf, allSigs(prog)...)
	if err != nil {
		return fmt.Errorf("%s: %w", entry, err)
	}
	// THE ERASED TERM IS KEPT EVEN WHEN NOTHING WAS PROMOTED. PromoteBig is
	// also where every ascription is removed, and a body that folded to a
	// literal under a declared wide range promotes nothing and still carries
	// one: `(the "int 0 …" 1000000000000000000)`, which no backend emits. It was
	// unreachable while folding stopped at 2^53 (ADR 0026 made it reachable).
	nf = nb
	if n > 0 {
		fmt.Fprintf(os.Stderr, "note: %d operation(s) in arbitrary precision\n", n)
	}
	// THE UNSIGNED WORD (ADR 0026 (10), emit/wordsel.go): selected before the
	// checker when U is declared, and after a refusal that found an operation U
	// would hold — see cmd/gen for the note.
	worded := false
	selectWords := func() {
		worded = true
		if nw, k := emit.SelectWords(tg, esig, nf); k > 0 {
			nf = nw
			fmt.Fprintf(os.Stderr, "note: %d operation(s) or conversion(s) in the unsigned word\n", k)
		}
	}
	if emit.DeclaresWord(tg, esig, nf) {
		selectWords()
	}
checks:
	// Check the residual before emitting it (docs/spec/types.md). On Go and
	// Java the host would catch most of this; on JavaScript nothing would.
	if err := emit.Check(tg, entry, nf); err != nil {
		return err
	}
	// Refinements: the bounds obligation primitives.md §2 recorded and
	// nothing checked (docs/spec/refinements.md).
	// ADR 0018's linearity, checked on the residual rather than by a type.
	if err := emit.CheckLinear(nf, tg, esig); err != nil {
		return fmt.Errorf("%s: %w", entry, err)
	}
	if notes, err := emit.Refine(tg, entry, esig, nf); err != nil {
		return err
	} else {
		for _, n := range notes {
			fmt.Fprintln(os.Stderr, "note:", n)
		}
	}
	// REPRESENTATION SELECTION — see cmd/gen for the note.
	// A POSTCONDITION on an exported definition is an OBLIGATION, not an
	// assumption: the caller is outside the program (postconditions.md §2).
	if ok, note := emit.CheckEnsures(tg, esig, nf); !ok {
		return fmt.Errorf("%s: %s", entry, note)
	} else if note != "" {
		fmt.Fprintln(os.Stderr, "note:", entry+": "+note)
	}
	rep, sel := emit.Intervals(tg, esig, nf, 0)
	if rep.InU && !worded {
		selectWords()
		goto checks
	}
	if rep.Ops > 0 || rep.Loops > 0 {
		fmt.Fprintf(os.Stderr, "note: %d of %d integer operations bounded; "+
			"%d of %d loop(s) proven terminating\n",
			rep.Proven, rep.Ops, rep.Terminates, rep.Loops)
	}
	// BOUNDED BY DEFAULT (ADR 0019). `-checked` is the second escape: it takes
	// the trap instead of the refusal.
	if checked {
		nf = sel
		// SAID, NOT ONLY DONE: an operation `-checked` turned into a trap is one
		// the compiler did not prove, and a caller that asked for the trap may
		// still need to know it was taken — the differential harness holds every
		// case to being proven unless it declares otherwise. The line is stable.
		if rep.Proven < rep.Ops {
			fmt.Fprintf(os.Stderr, "note: -checked: %d of %d integer operation(s) are traps, not proofs\n",
				rep.Ops-rep.Proven, rep.Ops)
		}
	} else if err := emit.Unbounded(entry, rep); err != nil {
		return err
	}
	// DIVISION BY A POWER OF TWO IS A SHIFT where the analysis can prove the
	// dividend non-negative and inside the target's declared shift width
	// (shiftdiv-2026-09-03). LAST, because the fixed-limb library's own carry
	// splits are spliced in by the promotion above and are what this is most
	// for; and its own pass, because `Intervals` above has the checked
	// selection ON and using its rebuilt term by default would reverse ADR 0012
	// without an ADR.
	if sh, k := emit.SelectShifts(tg, esig, nf); k > 0 {
		nf = sh
		fmt.Fprintf(os.Stderr, "note: %d division(s) became a shift or a mask\n", k)
	}
	// THE IR (ADR 0032), lowered from exactly what the backend receives, as
	// `gen -ir` does. The differential runner reads it: its cases' entry points
	// are compiled here and never by the emission sweep.
	if irOut != "" {
		p := &ir.Program{Target: target, Stage: ir.StageA}
		var errs []string
		if f, err := ir.Lower(tg, "oro-main", esig, nf, ir.Options{Decided: true}); err != nil {
			errs = append(errs, err.Error())
		} else {
			p.Funcs = append(p.Funcs, f)
		}
		_ = ir.WriteFile(irOut, tg, p, errs)
	}
	// THE BACKEND IS THE TARGET'S, NOT THE FLAG'S (target-system.md §1.1).
	//
	// This switch read the -target FLAG STRING and fell through to the Go
	// backend for anything it did not recognise, so a target directory named
	// anything of its own was compiled by the wrong generator — silently, with
	// the first sign of it being MASM refusing a file full of Go control flow
	// (win32-2026-09-06 §6). There is no default now: a target that cannot say
	// which backend compiles it is refused before any work is done.
	var code string
	switch backend {
	case "js":
		if printsIR("js") {
			code, err = js.FromResidual(tg, "oro-main", esig, nf)
		} else {
			code, err = emit.JSFunc(tg, "oro-main", esig, nf)
		}
	case "java":
		if printsIR("java") {
			code, err = java.FromResidual(tg, "oro-main", esig, nf)
		} else {
			code, err = emit.JavaMethod(tg, "oro-main", esig, nf)
		}
	case "x86-64":
		code, err = emit.AsmProc(tg, "oro-main", esig, nf)
		if err == nil {
			code = emit.AsmFile(tg, map[string]string{"oro-main": code}, "oro-main")
		}
	case "go":
		if printsIR("go") {
			code, err = golang.FromResidual(tg, "oro-main", esig, nf)
		} else {
			code, err = emit.Func(tg, "oro-main", esig, nf)
		}
	default:
		return fmt.Errorf("no code generator for backend %q", backend)
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

// allSigs is every signature the program declares. The fixed-limb rung's width
// comes from the whole program rather than from one function, because reduction
// inlines every non-exported call and `main` has no signature at all — see
// emit/biglimb.go's LimbWidth.
func allSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}

// irOut is -ir's file.
var irOut string

// printer is -printer, as gen's.
var printer string

// printsIR reports whether -printer names a backend.
func printsIR(backend string) bool {
	for _, b := range strings.Split(printer, ",") {
		if strings.TrimSpace(b) == backend || b == "ir" {
			return true
		}
	}
	return false
}
