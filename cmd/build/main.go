// Command build turns an Oroboros program into an artifact.
//
//	build -target=go -o hello examples/hello.oro
//
// It follows the entry file's imports, reduces the export named `main`, emits
// one complete source file, and hands it to the host's own toolchain — which is
// declared in the target file as data, like everything else (docs/spec/build.md).
package main

import (
	"errors"
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
	"oroboros/ir/llvm"
)

func main() {
	target := flag.String("target", "go", "target to build for")
	dir := flag.String("targets", "targets", "search path for target declarations; the source's own directory is always the nearest layer")
	path := flag.String("path", "lib", "search path for imported modules")
	out := flag.String("o", "", "artifact to write (default: the source's stem)")
	checkedFlag := flag.Bool("checked", false,
		"rewrite integer operations the compiler cannot bound to the target's checked form")
	keep := flag.Bool("keep", false, "keep the emitted source and print where it is")
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
	emit.ResetImports()
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
		return fmt.Errorf("target %q declares neither `build \"cmd\"` nor `artifact \"file\"`, so it can "+
			"emit source but not produce a deliverable; use cmd/gen instead", target)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// A TARGET FRAGMENT IS NOT A PROGRAM. `provides TARGET PATH { … }` is read by
	// the target loader — a fragment of the model written where the library
	// lives — so a file that says nothing else has nothing to emit, and saying so
	// beats reporting its first word as unexpected.
	if core.IsFragment(string(text)) {
		return fmt.Errorf("%s is a target fragment — it declares provides … and nothing else, "+
			"so there is no program here to build", src)
	}
	// THE PROGRAM, READ IN J (spec/j.md, ADR 0068).
	forms, err := core.ReadModule(string(text), src, core.ModulePath(src, filepath.SplitList(path)))
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	// THE LLVM LAYER'S PARAMETER IS IN SCOPE OF THE RUN (ADR 0093): the entry is
	// composed with Σ_OS's exit below, so its module is loaded with the program.
	if b, _ := tg.ResolveBackend(); b == "llvm" {
		forms = append([]core.Form{{Kind: "use", Name: "llvm/os", Alias: "#llvm-os"}}, forms...)
	}
	prog, _, err := core.LoadWithDefs(forms, core.SearchPath(libDirs(src, path)), tg.Defs)
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
	if err := emit.CheckSignatures(tg, prog, env, ir.CheckClaim(tg, allSigs(prog))); err != nil {
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
		return fmt.Errorf("%s has no entry point: a program needs `export fn main() { … }`", src)
	}
	// A DEFINITION'S CONTRACT IS CHECKED AT ITS CALLS (ADR 0028): decided
	// here, immediately after reduction, and erased before anything else runs.
	reqs := emit.InstallRequires(env, prog)
	// A RUN IS exit(0) ∘ main on a target printed as LLVM IR (ADR 0093): ending
	// the process is the operating system's, Σ_OS's `exit`, which the OS layer
	// defines. So the entry is the composition λ(). (λ_. exit(0)) (main()), its
	// argument evaluated first (β's effect rule), and it never returns.
	root := prog.Defs[entry]
	if b, _ := tg.ResolveBackend(); b == "llvm" {
		run := core.App(core.Fn([]string{"_"}, core.App(core.Name("llvm/os.exit"), core.Int(0))), core.App(root))
		root = core.Fn(nil, run)
	}
	// THE DEFINITIONS KEPT AS FUNCTIONS (ADR 0113): chosen by the compiler,
	// each built once under its contract; one that cannot be kept, or whose
	// call is outside its contract, is dropped and the program built again
	// with it inlined (P4), so no verdict depends on the choice.
	drop := map[string]bool{}
	var nf *core.Term
	var fA *ir.Func
	var leg *ir.Legality
	for {
		K, err := ir.Keep(tg, prog, env, reqs, []string{entry}, allSigs(prog), drop)
		var kd *ir.KeepDrop
		if errors.As(err, &kd) {
			drop[kd.Name] = true
			fmt.Fprintf(os.Stderr, "note: %v\n", firstLine(kd.Error()))
			continue
		}
		if err != nil {
			return err
		}
		nf, fA, leg, err = reduceAndLower(tg, prog, env, reqs, entry, root, target, checked)
		if n, ok := ir.KeptCallee(err, K); ok {
			drop[n] = true
			continue
		}
		// P4'S NET (ADR 0102, 0113): a contract proves less than its sites
		// (Theorem 3), so a refusal with K ≠ ∅ may be K's. K = ∅ is then the
		// verdict, and no definition is kept.
		if err != nil && len(K) > 0 {
			for n := range K {
				drop[n] = true
			}
			fmt.Fprintf(os.Stderr, "note: every definition inlined: kept, %s\n", firstLine(err.Error()))
			continue
		}
		if err != nil {
			return err
		}
		if len(K) > 0 {
			fmt.Fprintf(os.Stderr, "note: %d definition(s) kept as functions, %d inlined by their contracts\n", len(K), len(drop))
			// each kept definition's proofs, once, as an entry's are reported
			for _, kc := range ir.KeptCounts(tg) {
				fmt.Fprintf(os.Stderr, "note: %s (kept): %d of %d integer operations bounded; "+
					"%d of %d loop(s) proven terminating\n", kc.Name, kc.Proven, kc.Ops, kc.Halts, kc.Loops)
			}
		}
		break
	}
	_ = nf
	if leg.Ops > 0 || leg.Loops > 0 {
		fmt.Fprintf(os.Stderr, "note: %d of %d integer operations bounded; "+
			"%d of %d loop(s) proven terminating\n",
			leg.Proven, leg.Ops, leg.Halts, leg.Loops)
	}
	// BOUNDED BY DEFAULT (ADR 0019). `-checked` is the second escape: the IR
	// has written `trap` where the proof failed.
	if checked {
		// SAID, NOT ONLY DONE: an operation `-checked` turned into a trap is one
		// the compiler did not prove, and a caller that asked for the trap may
		// still need to know it was taken — the differential harness holds every
		// case to being proven unless it declares otherwise. The line is stable.
		if leg.Proven < leg.Ops {
			fmt.Fprintf(os.Stderr, "note: -checked: %d of %d integer operation(s) are traps, not proofs\n",
				leg.Ops-leg.Proven, leg.Ops)
		}
	} else if err := leg.Refusal(entry, tg); err != nil {
		return err
	}
	if err := leg.SizeRefusal(entry, tg); err != nil {
		return err
	}
	// DIVISION BY A POWER OF TWO IS A SHIFT (ir/shift.go, cmd/gen).
	if shifts := ir.SelectShifts(tg, fA); shifts > 0 {
		fmt.Fprintf(os.Stderr, "note: %d division(s) became a shift or a mask\n", shifts)
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
	var calleeIR []*ir.Func
	switch backend {
	case "js":
		code, err = js.FromFunc(tg, fA)
	case "java":
		code, err = java.FromFunc(tg, fA)
	case "llvm":
		code, err = llvm.FromFunc(tg, fA)
		if err == nil {
			// AND EACH PROGRAM FUNCTION IT CALLS (ADR 0101), defined once
			funcs := map[string]string{"oro-main": code}
			var cs map[string]string
			cs, calleeIR, err = llvm.Callees(tg, fA)
			for n, c := range cs {
				funcs[n] = c
			}
			code = llvm.File(tg, funcs, "oro-main")
		}
	case "go":
		code, err = golang.FromFunc(tg, fA)
	default:
		return fmt.Errorf("no code generator for backend %q", backend)
	}
	if err != nil {
		return err
	}
	// THE IR (ADR 0032, ADR 0038), the IR_P the printer read, as `gen -ir`
	// writes it. The differential runner reads it: its cases' entry points are
	// compiled here and never by the emission sweep.
	if irOut != "" {
		p := &ir.Program{Target: target, Stage: ir.StageP, Funcs: append([]*ir.Func{fA.Clone()}, calleeIR...)}
		_ = ir.WriteFile(irOut, tg, p, nil)
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

// reduceAndLower is the entry's reduction and the pipeline after it, with the
// kept definitions left as calls.
func reduceAndLower(tg *emit.Target, prog *core.Program, env *core.Env, reqs *emit.RequireSet,
	entry string, root *core.Term, target string, checked bool) (*core.Term, *ir.Func, *ir.Legality, error) {
	nf, err := core.Normalize(root, env, core.DefaultFuel)
	if err != nil {
		return nil, nil, nil, err
	}
	// A JOIN POINT IS NOT JUMPED OUT OF (tables.md §2.5): refused before any
	// pass walks a clause chain.
	// η FOR PRODUCTS (tables.md §2.5), as cmd/gen.
	nf = emit.EtaTails(tg, nf)
	nf = emit.MaterializeResult(nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", entry, err)
	}
	if nf, err = emit.DischargeRequires(reqs, tg, entry, prog.Sigs[entry], nf,
		func(x *core.Term) (*core.Term, error) { return ir.DischargeRanges(tg, prog.Sigs[entry], x) }); err != nil {
		return nil, nil, nil, err
	}
	if nf.Kind != core.KFn || len(nf.Params) != 0 {
		return nil, nil, nil, fmt.Errorf("main must take no arguments, got %s", nf)
	}
	if left := core.Residual(nf, env); len(left) > 0 {
		return nil, nil, nil, fmt.Errorf("not in normal form for target %q: %s", target, strings.Join(left, ", "))
	}

	// THE PIPELINE (ir.Entry) — see cmd/gen.
	fA, leg, err := ir.Entry(tg, entry, "oro-main", prog.Sigs[entry], nf, allSigs(prog), checked,
		func(n ir.Note) {
			switch n.Kind {
			case ir.NoteFlattened:
				fmt.Fprintf(os.Stderr, "note: %d product access(es) flattened\n", n.N)
			case ir.NoteRefine:
				fmt.Fprintln(os.Stderr, "note:", n.Text)
			case ir.NoteBig:
				fmt.Fprintf(os.Stderr, "note: %d operation(s) in arbitrary precision\n", n.N)
			case ir.NoteWord:
				fmt.Fprintf(os.Stderr, "note: %d operation(s) or conversion(s) in the unsigned word\n", n.N)
			}
		})
	if err != nil {
		return nil, nil, nil, err
	}
	// BOUNDED BY DEFAULT (ADR 0019), decided here so a refusal K caused is
	// retried with K = ∅ (P4).
	if !checked {
		if err := leg.Refusal(entry, tg); err != nil {
			return nil, nil, nil, err
		}
	}
	if err := leg.SizeRefusal(entry, tg); err != nil {
		return nil, nil, nil, err
	}
	return nf, fA, leg, nil
}

// firstLine is a message's first line: a definition's fallback reason, said once.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
