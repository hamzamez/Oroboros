//go:build ignore

// THE SUITE ON A SECOND TRIPLE (assessment-2026-11-16 §5 item 4, "program first:
// the suite run on the triple, every failure classified, before anything is
// built"). A copy of gauntlet/differential/run.go with two changes: windows is
// the only target, and every build reads the target declarations from
// $I686_TARGETS, a copy of targets/ whose windows layer names the triple
// i686-pc-windows-msvc (experiments/i686/README.md). Run from
// gauntlet/differential: go run ../../experiments/i686/run32.go

// Differential conformance: one program, four targets, the same answer.
//
// The existing suite next door tests our LOWERINGS of one primitive by
// hand-writing the same test in three languages. This one tests the COMPILER:
// it takes an `.oro` program, builds it on every target, RUNS each artifact,
// and requires the outputs to be byte-identical.
//
// It exists because emitting is not the same as being right, and two silent
// wrong-answer bugs in one day proved it:
//
//   - the JavaScript post-hoist emitted the loop increment twice, so the sieve
//     advanced by two per iteration and got 1984 of 2000 answers wrong. It
//     compiled and returned a number
//     (gauntlet/results/loopshape-2026-08-25.md §3).
//   - the x86 element-width pass read a byte table with a qword load, taking
//     seven bytes of the following elements per access
//     (gauntlet/results/wintables-2026-08-25.md §4a).
//
// Both were caught only because someone hand-wrote a reference and compared.
// `run.sh` next door and the example sweep both check that a program EMITS,
// and both of these emitted cleanly.
//
//	go run run.go            # every case, every target
//	go run run.go match      # only cases whose name contains "match"
//	go run run.go -keep      # leave the work directories for inspection
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"oroboros/ir"
)

// A case is written ONCE, with `@op` where a target-native name goes.
//
// This is not a portable layer sneaking back in. A test has to say "the same
// program on four hosts", and the only thing that may differ is the host's own
// spelling — so the substitution is exactly the set of names where the four
// agree on meaning and disagree on spelling. Anything the hosts disagree about
// (integer division, float formatting) is deliberately absent, because a
// differential test of a genuine divergence would only ever fail.
// ops WAS the macro table, and its deletion is the point.
//
// Every case used to write `@add`, `@lt` and so on, and this map expanded them
// into `go.+`, `x64.setl` and the other two spellings — because the LANGUAGE
// had no arithmetic. `=` was the only integer operator it owned, so a case
// could compare for equality portably and could not add two numbers.
//
// The operators are the language's now, found per target the way `=` always
// was, so a case writes `+`, `-`, `*`, `/`, `%`, `<`, `<=`, `>`, `>=` and this
// table has nothing left to do. That the cases compile unchanged on four
// targets with the substitution gone is the proof the promotion worked.
//
// Kept as an empty map rather than deleted outright so that `render` still has
// one place to grow if a construct ever needs per-target spelling again.
var ops = map[string]map[string]string{
	"go": {}, "js": {}, "java": {}, "windows": {},
}

// Printing is target-native on every host — it is the one thing a portable
// program cannot do — so the harness supplies the `main` that prints, and a
// case only ever defines `run`.
var driver = map[string]struct{ uses, print string }{
	"go":   {"use go\nuse go/fmt", "fmt.Println"},
	"js":   {"use js\nuse js/console as console", "console.log"},
	"java": {"use java\nuse java/System as sys", "sys.out_println"},
	// WINDOWS (ADR 0080, docs/spec/llvm.md) prints through lib/win/print.oro,
	// written in the language's operators over WriteFile.
	"windows": {"use win/print", "print.print_int"},
}

// AND A DRIVER THAT PRINTS A STRING, selected by `// prints: string` on a case.
//
// Three of the four print an integer and a string through the same name; on
// windows lib/win/print has one of each, and each ends its line, as the other
// three drivers do: a case printing SEVERAL strings must come out one per line
// everywhere, or the harness disagrees with itself rather than the compiler.
//
// It exists because otherwise the suite CANNOT CHECK A STRING LITERAL AT ALL:
// the harness prints an integer, and there is no portable operation from a
// string to one — which is strings.md §2's finding arriving in the test harness
// rather than in a program.
var stringDriver = map[string]struct{ uses, print string }{
	"go":      {"use go\nuse go/fmt", "fmt.Println"},
	"js":      {"use js\nuse js/console as console", "console.log"},
	"java":    {"use java\nuse java/System as sys", "sys.out_println"},
	"windows": {"use win/print", "print.print_str"},
}

// What each target's artifact is CALLED, and how to run it.
//
// Three hosts, three notions of a deliverable (build.md §3): Go and windows
// produce an executable, JavaScript a module a runtime is pointed at, and
// javac a class DIRECTORY. The extensions are not cosmetic — Windows will not
// exec a file without `.exe`, and node will not treat a file without `.mjs` as
// a module.
//
// This is test infrastructure and deliberately not a language concept. A target
// declares how to BUILD because a program needs that; nothing in the language
// needs to know how to run one.
var artifact = map[string]struct {
	name string
	run  func(out string) *exec.Cmd
}{
	"go":      {"out.exe", func(o string) *exec.Cmd { return exec.Command(o) }},
	"windows": {"out.exe", func(o string) *exec.Cmd { return exec.Command(o) }},
	"js":      {"out.mjs", func(o string) *exec.Cmd { return exec.Command("node", o) }},
	// THE JVM PRINTS IN THE PLATFORM CHARSET unless told otherwise, so a U+FFFD
	// came out as `?` on Windows (strings.md §5, a property of PRINTING, not of
	// the program). The output is compared as bytes, so it is asked for in
	// UTF-8, as Go and V8 already write it: `file.encoding` is what System.out
	// uses on a pipe up to JDK 18, and `stdout.encoding` from JDK 19.
	"java": {"classes", func(o string) *exec.Cmd {
		return exec.Command("java", "-Dfile.encoding=UTF-8", "-Dstdout.encoding=UTF-8",
			"-Dsun.stdout.encoding=UTF-8", "-cp", o, "Main")
	}},
}

// The default inputs. A case may override them with a `// inputs: …` line, and
// a heavy one has to: reduction inlines `run` at every call site, so nine calls
// that each build a table is nine tables in one procedure — which on x86 is
// ninety integer operations and more spilled operands than there are scratch
// registers. That ceiling is real and belongs to the backend; provoking it from
// a test harness measures the harness.
var defaultInputs = []int{0, 1, 2, 3, 5, 8, 13, 21, 34}

func inputsFor(src string) []int {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "// inputs:") {
			continue
		}
		var out []int
		for _, f := range strings.Fields(strings.TrimPrefix(l, "// inputs:")) {
			var n int
			if _, err := fmt.Sscanf(f, "%d", &n); err == nil {
				out = append(out, n)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultInputs
}

// A case states the answer it expects, and the harness checks it.
//
// Agreement alone is not correctness: four backends can be wrong in the same
// way, and the one bug that would hide from a purely differential test is a bug
// in the READER or the REDUCER, which all four share. `// expect:` is what makes
// each case a test of the compiler rather than only of its consistency.
func expectFor(src string) string {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "// expect:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "// expect:"))
		}
	}
	return ""
}

func render(src, target string) string {
	inputs := inputsFor(src)
	d := driver[target]
	// A case that returns a STRING needs the host's string print, and on windows
	// that is a different primitive. Everything else about the driver is the same.
	if strings.Contains(src, "// prints: string") {
		d = stringDriver[target]
	}
	// THE DRIVER, IN J (spec/j.md): the target's imports, the case, and an
	// entry point printing each result in order, the last print the block's
	// value.
	var b strings.Builder
	b.WriteString(d.uses)
	b.WriteString("\n")
	body := src
	for k, v := range ops[target] {
		body = strings.ReplaceAll(body, k, v)
	}
	b.WriteString(body)
	b.WriteString("\nexport fn main() {\n")
	for _, n := range inputs {
		fmt.Fprintf(&b, "    %s(run(%d))\n", d.print, n)
	}
	b.WriteString("}\n")
	return b.String()
}

func build(caseName, src, target, bigRepr, work string, keep bool) (string, bool, error) {
	dir := filepath.Join(work, caseName+"."+target+bigRepr)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	if !keep {
		defer func() { _ = os.RemoveAll(dir) }()
	}
	oro := filepath.Join(dir, "case.oro")
	if err := os.WriteFile(oro, []byte(render(src, target)), 0o644); err != nil {
		return "", false, err
	}
	a, ok := artifact[target]
	if !ok {
		return "", false, fmt.Errorf("no artifact convention for %s", target)
	}
	out := filepath.Join(dir, a.name)
	// BUILT WITH `-checked`, and there are two reasons rather than one.
	//
	// ADR 0019 makes an integer operation the compiler cannot bound a compile
	// error. These cases carry no signatures at all — the harness writes their
	// `main`, and each is the smallest program that exercises one construct —
	// so ten of sixteen cannot prove: `(+ acc (t i))` sums values read out of a
	// table and nothing bounds a table's elements unless someone declares it.
	// Annotating each would answer a question this suite does not ask.
	//
	// AND IT EXERCISES THE REBUILD PATH, which is the better reason. The
	// selected term is discarded unless `-checked` is on, and that path has now
	// hidden two bugs: a lambda rewrapped with `FnClosed`, which does not close,
	// and variable capture from `openFresh` being handed a fresh empty set at
	// every call site. The second silently made a five-entry map answer `len` of
	// 1 on windows. A path nothing runs is a path nothing checks.
	//
	// It changes no ANSWER: the values are in range at run time, which is
	// exactly what the suite then verifies on four targets.
	//
	// BUT IT MUST NOT CHANGE WHAT IS PROVEN WITHOUT SAYING SO. `-checked` turns
	// an operation the compiler cannot bound into a trap, so a case written to
	// be proven — u128's arithmetic, every refinement case — would pass here
	// with its proof gone and nothing would notice; the emission sweep compiles
	// a case without this `main` and saw an empty package. So the build reports
	// each trap it took (cmd/build), and the case must declare it: see
	// checkedFor.
	// AND LOWERED TO THE IR (docs/spec/ir.md §11): a case's entry point is
	// compiled only here, never by the emission sweep, so this is where the
	// IR's totality on these programs is checked. A case whose IR is refused
	// fails, naming the rule.
	// $I686_TARGET names the target built (windows32, ADR 0122, once it was
	// one; the copy's windows layer before), and $I686_TARGETS its declarations
	tname := target
	if t := os.Getenv("I686_TARGET"); t != "" {
		tname = t
	}
	args := []string{"-checked", "-target=" + tname, "-ir", out + ".ir", "-targets", os.Getenv("I686_TARGETS")}
	if bigRepr != "" {
		args = append(args, "-big-repr="+bigRepr)
	}
	cmd := exec.Command(buildBin, append(args, "-o", out, oro)...)
	cmd.Dir = repoRoot()
	bout, err := cmd.CombinedOutput()
	if err != nil {
		return "", false, fmt.Errorf("build: %v\n%s", err, indent(string(bout)))
	}
	if msg, ok := ir.CheckFile(out + ".ir"); !ok {
		return "", false, fmt.Errorf("ir: %s", msg)
	}
	traps := strings.Contains(string(bout), trapNote)
	rc := a.run(out)
	rc.Dir = dir
	var stdout, stderr bytes.Buffer
	rc.Stdout, rc.Stderr = &stdout, &stderr
	if err := rc.Run(); err != nil {
		// A DECLARED BOUND ENFORCED IS AN OUTCOME, not a failure of the harness:
		// above the word a range is checked at run time (ADR 0029), and the
		// property is that every target traps on the SAME value. So a run that
		// stops with the bound's own message answers what it printed, then
		// `trap`, and `// expect: … trap` states it. Any other failure is one.
		if strings.Contains(stderr.String(), boundMessage) {
			return strings.TrimSpace(normalise(stdout.String()) + "\ntrap"), traps, nil
		}
		// AND SO IS AN ABANDONMENT (spec/errors.md §8): `expect` crashes with its
		// reason on any constructor but the success, through `abandon`, and the
		// property is that every target crashes at the SAME point with the same
		// reason. A case names the reason, `// abandons: REASON`; a run that stops
		// with it on standard error answers what it printed, then `abandon`.
		if why := abandonsFor(src); why != "" && strings.Contains(stderr.String(), why) {
			return strings.TrimSpace(normalise(stdout.String()) + "\nabandon"), traps, nil
		}
		return "", false, fmt.Errorf("run: %v\n%s", err, indent(stdout.String()+stderr.String()))
	}
	return normalise(stdout.String() + stderr.String()), traps, nil
}

// abandonsFor reads a case's `// abandons: REASON`, the reason its `expect`
// crashes with; empty when it declares none.
func abandonsFor(src string) string {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "// abandons:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "// abandons:"))
		}
	}
	return ""
}

// trapNote is what cmd/build says under `-checked` when it took a trap for an
// operation it did not prove.
const trapNote = "note: -checked: "

// checkedFor reads a case's `// checked: TARGET… — reason`: the targets on
// which the case is ALLOWED to be unproven, and why. Every other target must
// prove every integer operation, and a declared target must still need it, so
// the declaration is held in both directions and cannot go stale. A case with
// no such line is proven everywhere it runs.
func checkedFor(src string) (map[string]bool, error) {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "// checked:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(l, "// checked:"))
		ts, why, ok := strings.Cut(rest, "—")
		if !ok || strings.TrimSpace(why) == "" {
			return nil, fmt.Errorf("`// checked:` names its targets, then ` — ` and the reason: %q", l)
		}
		out := map[string]bool{}
		for _, t := range strings.Fields(ts) {
			if _, known := driver[t]; !known {
				return nil, fmt.Errorf("`// checked:` names %q, which is not a target", t)
			}
			out[t] = true
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("`// checked:` names no target: %q", l)
		}
		return out, nil
	}
	return map[string]bool{}, nil
}

// boundMessage is what every host's `big-fit` and the limb rung's `trap-if`
// say when a value leaves its declared range (targets/*/bigint.oro).
const boundMessage = "bignum overflow: the declared range is too small"

// Trailing whitespace and line endings are the host's, not the program's.
func normalise(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		out = append(out, strings.TrimSpace(l))
	}
	return strings.Join(out, "\n")
}

func indent(s string) string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		out = append(out, "    "+l)
	}
	return strings.Join(out, "\n")
}

func repoRoot() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..")
}

// targets are the four every case runs on.
var targets = []string{"windows"}

// buildBin is cmd/build, built once into the work directory; caseDir holds
// the cases (`-cases DIR`, for the harness's own test).
var buildBin, caseDir = "", "cases"

// runCase builds case c on every target it does not skip, runs each, and
// writes its report to w: true when the targets agree, the answer is the one
// the case expects, and what is proven is what it says.
func runCase(w *strings.Builder, c, work string, keep bool) bool {
	name := strings.TrimSuffix(filepath.Base(c), ".oro")
	raw, err := os.ReadFile(c)
	if err != nil {
		panic(err)
	}
	src := string(raw)

	// A case may exclude a target, with the reason in the file. That is a
	// real answer — a program using a host's own name is not portable and
	// this suite is not the place to pretend otherwise — but it has to be
	// SAID, not achieved by the case quietly not being run anywhere.
	//
	// AND A SKIP IS A CLAIM, CHECKED (the harness, gauntlet's plan item 7): the
	// proposition that the case FAILS on the target, `// skip: T because
	// "TEXT"`, where TEXT is a substring of the failure. Every skipped target is
	// still built and run below, and the claim is evaluated. A target that
	// builds and agrees is a STALE skip, and fails the case (three skipped the
	// JVM for a month after its unsigned word was built, llvm64-2026-11-06); a
	// target that fails otherwise than its reason says fails it too, so a skip
	// cannot stay true for a reason no longer the one written.
	skips, err := skipsOf(src)
	if err != nil {
		fmt.Fprintf(w, "FAIL %s — %v\n", name, err)
		return false
	}
	skip := map[string]bool{}
	for t := range skips {
		skip[t] = true
	}

	// AND A CASE MAY ASK FOR BOTH REPRESENTATIONS, which turns the suite's
	// question from "four hosts agree" into "four hosts and two storage
	// choices agree".
	//
	// That is the property the representation policy claims: a range is
	// SEMANTICS, the target picks the storage, and picking differently
	// changes what a program COSTS and not what it computes. Without this
	// the limb rung lost its cross-target coverage the moment three of the
	// four targets declared `(big-repr host)` — the code would still be
	// emitted and nothing would run it.
	//
	// Fixed limbs need a host bignum only to RENDER the result, so windows
	// takes that variant alone; the set is hardcoded here the way the
	// drivers and artifacts above are, because this is test infrastructure.
	variants := map[string][]string{}
	for _, t := range targets {
		variants[t] = []string{""}
	}
	if strings.Contains(src, "// big-repr: both") {
		for _, t := range []string{"go", "js", "java"} {
			variants[t] = []string{"host", "limbs"}
		}
	}

	outs := map[string]string{}
	trapped := map[string]bool{} // label -> -checked took a trap
	var order []string
	var errs []string
	for _, t := range targets {
		if skip[t] {
			continue
		}
		for _, rep := range variants[t] {
			label := t
			if rep != "" {
				label = t + "/" + rep
			}
			order = append(order, label)
			o, traps, err := build(name, src, t, rep, work, keep)
			if err != nil {
				errs = append(errs, fmt.Sprintf("  %s: %v", label, err))
				continue
			}
			outs[label] = o
			trapped[label] = traps
		}
	}
	if len(errs) > 0 {
		fmt.Fprintf(w, "FAIL %s\n%s\n", name, strings.Join(errs, "\n"))
		return false
	}
	// Every target must agree, and the first one alphabetically is only the
	// one the differences are reported against — no target is the oracle.
	var ref string
	var refT string
	agree := true
	for _, t := range order {
		o, ok := outs[t]
		if !ok {
			continue
		}
		if ref == "" && refT == "" {
			ref, refT = o, t
			continue
		}
		if o != ref {
			agree = false
		}
	}
	if !agree {
		fmt.Fprintf(w, "FAIL %s — targets disagree\n", name)
		for _, t := range targets {
			if o, ok := outs[t]; ok {
				fmt.Fprintf(w, "  %-8s %s\n", t, strings.ReplaceAll(o, "\n", " "))
			}
		}
		return false
	}
	if want := expectFor(src); want != "" {
		got := strings.ReplaceAll(ref, "\n", " ")
		if got != want {
			fmt.Fprintf(w, "FAIL %s — every target agrees, and all of them are wrong\n", name)
			fmt.Fprintf(w, "  expected  %s\n  got       %s\n", want, got)
			return false
		}
	} else {
		fmt.Fprintf(w, "FAIL %s — no `// expect:` line. Agreement is not correctness;\n"+
			"  a bug in the reader or the reducer is shared by every backend.\n", name)
		return false
	}
	// PROVEN, UNLESS SAID OTHERWISE, AND SAID ONLY WHERE TRUE.
	allowed, err := checkedFor(src)
	if err != nil {
		fmt.Fprintf(w, "FAIL %s — %v\n", name, err)
		return false
	}
	var proof []string
	for _, label := range order {
		t, _, _ := strings.Cut(label, "/")
		switch {
		case trapped[label] && !allowed[t]:
			proof = append(proof, fmt.Sprintf("  %s: an integer operation is a trap, not a proof (built with -checked).\n"+
				"    Prove it (a signature on run, a narrower range), or declare `// checked: %s — why`.", label, t))
		case !trapped[label] && allowed[t]:
			proof = append(proof, fmt.Sprintf("  %s: `// checked:` names %s, and every operation there is proven. Remove it.", label, t))
		}
	}
	if len(proof) > 0 {
		fmt.Fprintf(w, "FAIL %s — what is proven is not what the case says\n%s\n", name, strings.Join(proof, "\n"))
		return false
	}
	// EACH SKIP, EVALUATED: built and run as the others are, it must fail, and
	// for the reason it states.
	var stale []string
	for _, t := range targets {
		why, ok := skips[t]
		if !ok {
			continue
		}
		o, traps, err := build(name, src, t, "", work, keep)
		var failure string
		switch {
		case err != nil:
			failure = err.Error()
		case o != ref:
			failure = "output: " + strings.ReplaceAll(o, "\n", " ")
		case traps && !allowed[t]:
			failure = "an integer operation is a trap, not a proof (built with -checked)"
		}
		switch {
		case failure == "":
			stale = append(stale, fmt.Sprintf("  %s: skipped because %q, and it builds, runs and agrees: a stale skip. Remove it.", t, why))
		case !strings.Contains(failure, why):
			var lines []string
			for _, l := range strings.Split(strings.TrimSpace(failure), "\n") {
				if l = strings.TrimSpace(l); l != "" && len(lines) < 4 {
					lines = append(lines, "      "+l[:min(len(l), 240)])
				}
			}
			stale = append(stale, fmt.Sprintf("  %s: skipped because %q, and it fails otherwise:\n%s", t, why, strings.Join(lines, "\n")))
		}
	}
	if len(stale) > 0 {
		fmt.Fprintf(w, "FAIL %s — a skip is not what the target does\n%s\n", name, strings.Join(stale, "\n"))
		return false
	}
	var ran []string
	for _, t := range order {
		if _, ok := outs[t]; ok {
			ran = append(ran, t)
		}
	}
	for _, t := range targets {
		if _, ok := skips[t]; ok {
			ran = append(ran, t+"(skipped, checked)")
		}
	}
	fmt.Fprintf(w, "ok   %-22s %s  [%s]\n", name,
		strings.ReplaceAll(ref, "\n", " "), strings.Join(ran, " "))
	return true
}

// skipRe is a skip with its reason: `// skip: T because "TEXT"`.
var skipRe = regexp.MustCompile(`(?m)^// skip: (\w+) because "([^"]+)"`)

// skipsOf is a case's skips, each target's reason; a skip that states none,
// or a target no case runs on, is refused.
func skipsOf(src string) (map[string]string, error) {
	out := map[string]string{}
	for _, m := range skipRe.FindAllStringSubmatch(src, -1) {
		if !slices.Contains(targets, m[1]) {
			continue // a target this run does not take (only windows here)
		}
		out[m[1]] = m[2]
	}
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(line, "// skip: ")
		if !ok {
			continue
		}
		t, _, _ := strings.Cut(rest, " ")
		if _, ok := out[t]; !ok && slices.Contains(targets, t) {
			return nil, fmt.Errorf("`%s` states no reason: write `// skip: %s because \"TEXT\"`, "+
				"TEXT a part of the failure, which the harness checks", strings.TrimSpace(line), t)
		}
	}
	return out, nil
}

func main() {
	filter, keep, jobs := "", false, max(runtime.NumCPU()/2, 1)
	for i := 1; i < len(os.Args); i++ {
		switch a := os.Args[i]; {
		case a == "-keep":
			keep = true
		case a == "-j" && i+1 < len(os.Args):
			n, err := strconv.Atoi(os.Args[i+1])
			if err != nil || n < 1 {
				fmt.Fprintln(os.Stderr, "-j takes a positive count")
				os.Exit(2)
			}
			jobs = n
			i++
		case a == "-cases" && i+1 < len(os.Args):
			caseDir = os.Args[i+1]
			i++
		default:
			filter = a
		}
	}
	cases, err := filepath.Glob(filepath.Join(caseDir, "*.oro"))
	if err != nil || len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "no cases found in cases/")
		os.Exit(1)
	}
	sort.Strings(cases)
	work, err := os.MkdirTemp("", "differential")
	if err != nil {
		panic(err)
	}
	if !keep {
		defer func() { _ = os.RemoveAll(work) }()
	} else {
		fmt.Println("work:", work)
	}

	// THE COMPILER IS BUILT ONCE, not by `go run` at every case × target: some
	// 300 builds, each paying a link (differential-2026-11-13).
	buildBin = filepath.Join(work, "oro-build.exe")
	gb := exec.Command("go", "build", "-o", buildBin, "./cmd/build")
	gb.Dir = repoRoot()
	if out, err := gb.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building cmd/build: %v\n%s", err, out)
		os.Exit(1)
	}

	// THE CASES RUN IN PARALLEL, `jobs` at a time. Each builds into its own
	// directories under work and its verdict is a function of its source alone,
	// so the order they run in changes none; each report is printed in the
	// cases' order, as soon as every case before it is done.
	var todo []string
	for _, c := range cases {
		if name := strings.TrimSuffix(filepath.Base(c), ".oro"); filter == "" || strings.Contains(name, filter) {
			todo = append(todo, c)
		}
	}
	reports := make([]string, len(todo))
	oks := make([]bool, len(todo))
	done := make([]chan struct{}, len(todo))
	for i := range done {
		done[i] = make(chan struct{})
	}
	next := make(chan int)
	for k := 0; k < jobs; k++ {
		go func() {
			for i := range next {
				var b strings.Builder
				oks[i] = runCase(&b, todo[i], work, keep)
				reports[i] = b.String()
				close(done[i])
			}
		}()
	}
	go func() {
		for i := range todo {
			next <- i
		}
		close(next)
	}()
	fails := 0
	for i := range todo {
		<-done[i]
		fmt.Print(reports[i])
		if !oks[i] {
			fails++
		}
	}
	if fails > 0 {
		fmt.Printf("\n%d case(s) failed\n", fails)
		os.Exit(1)
	}
	fmt.Println("\nall cases agree on every target")
}
