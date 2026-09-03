//go:build ignore

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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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
	"go":      {"(use go)\n(use go/fmt)", "fmt.Println"},
	"js":      {"(use js)\n(use js/console as console)", "console.log"},
	"java":    {"(use java)\n(use java/System as sys)", "sys.out-println"},
	"windows": {"(use x64)\n(use win/fmt)", "fmt.print-int"},
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
	"java": {"classes", func(o string) *exec.Cmd {
		return exec.Command("java", "-cp", o, "Main")
	}},
}

// The default inputs. A case may override them with a `; inputs: …` line, and
// a heavy one has to: reduction inlines `run` at every call site, so nine calls
// that each build a table is nine tables in one procedure — which on x86 is
// ninety integer operations and more spilled operands than there are scratch
// registers. That ceiling is real and belongs to the backend; provoking it from
// a test harness measures the harness.
var defaultInputs = []int{0, 1, 2, 3, 5, 8, 13, 21, 34}

func inputsFor(src string) []int {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "; inputs:") {
			continue
		}
		var out []int
		for _, f := range strings.Fields(strings.TrimPrefix(l, "; inputs:")) {
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
// in the READER or the REDUCER, which all four share. `; expect:` is what makes
// each case a test of the compiler rather than only of its consistency.
func expectFor(src string) string {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "; expect:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "; expect:"))
		}
	}
	return ""
}

func render(src, target string) string {
	inputs := inputsFor(src)
	d := driver[target]
	var b strings.Builder
	b.WriteString(d.uses)
	b.WriteString("\n(export main)\n")
	body := src
	for k, v := range ops[target] {
		body = strings.ReplaceAll(body, k, v)
	}
	b.WriteString(body)
	b.WriteString("\n(def main (fn ()\n")
	// A right-nested `seq` chain: print each result in order.
	for i, n := range inputs {
		b.WriteString(strings.Repeat("  ", i+1))
		if i < len(inputs)-1 {
			fmt.Fprintf(&b, "(seq (%s (run %d))\n", d.print, n)
		} else {
			fmt.Fprintf(&b, "(%s (run %d))", d.print, n)
		}
	}
	// n inputs open n-1 `seq`s; the last print is balanced on its own.
	b.WriteString(strings.Repeat(")", len(inputs)-1))
	b.WriteString("))\n")
	return b.String()
}

func build(caseName, src, target, bigRepr, work string, keep bool) (string, error) {
	dir := filepath.Join(work, caseName+"."+target+bigRepr)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if !keep {
		defer func() { _ = os.RemoveAll(dir) }()
	}
	oro := filepath.Join(dir, "case.oro")
	if err := os.WriteFile(oro, []byte(render(src, target)), 0o644); err != nil {
		return "", err
	}
	a, ok := artifact[target]
	if !ok {
		return "", fmt.Errorf("no artifact convention for %s", target)
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
	args := []string{"run", "./cmd/build", "-checked", "-target=" + target}
	if bigRepr != "" {
		args = append(args, "-big-repr="+bigRepr)
	}
	cmd := exec.Command("go", append(args, "-o", out, oro)...)
	cmd.Dir = repoRoot()
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build: %v\n%s", err, indent(string(b)))
	}
	rc := a.run(out)
	rc.Dir = dir
	b, err := rc.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run: %v\n%s", err, indent(string(b)))
	}
	return normalise(string(b)), nil
}

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

func main() {
	filter, keep := "", false
	for _, a := range os.Args[1:] {
		if a == "-keep" {
			keep = true
		} else {
			filter = a
		}
	}
	cases, err := filepath.Glob("cases/*.oro")
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

	targets := []string{"go", "js", "java", "windows"}
	fails := 0
	for _, c := range cases {
		name := strings.TrimSuffix(filepath.Base(c), ".oro")
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}
		raw, err := os.ReadFile(c)
		if err != nil {
			panic(err)
		}
		src := string(raw)

		// A case may exclude a target, with the reason in the file. That is a
		// real answer — a program using a host's own name is not portable and
		// this suite is not the place to pretend otherwise — but it has to be
		// SAID, not achieved by the case quietly not being run anywhere.
		skip := map[string]bool{}
		for _, t := range targets {
			if strings.Contains(src, "; skip: "+t) {
				skip[t] = true
			}
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
		if strings.Contains(src, "; big-repr: both") {
			for _, t := range []string{"go", "js", "java"} {
				variants[t] = []string{"host", "limbs"}
			}
		}

		outs := map[string]string{}
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
				o, err := build(name, src, t, rep, work, keep)
				if err != nil {
					errs = append(errs, fmt.Sprintf("  %s: %v", label, err))
					continue
				}
				outs[label] = o
			}
		}
		if len(errs) > 0 {
			fails++
			fmt.Printf("FAIL %s\n%s\n", name, strings.Join(errs, "\n"))
			continue
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
			fails++
			fmt.Printf("FAIL %s — targets disagree\n", name)
			for _, t := range targets {
				if o, ok := outs[t]; ok {
					fmt.Printf("  %-8s %s\n", t, strings.ReplaceAll(o, "\n", " "))
				}
			}
			continue
		}
		if want := expectFor(src); want != "" {
			got := strings.ReplaceAll(ref, "\n", " ")
			if got != want {
				fails++
				fmt.Printf("FAIL %s — every target agrees, and all of them are wrong\n", name)
				fmt.Printf("  expected  %s\n  got       %s\n", want, got)
				continue
			}
		} else {
			fails++
			fmt.Printf("FAIL %s — no `; expect:` line. Agreement is not correctness;\n"+
				"  a bug in the reader or the reducer is shared by every backend.\n", name)
			continue
		}
		var ran []string
		for _, t := range order {
			if _, ok := outs[t]; ok {
				ran = append(ran, t)
			}
		}
		fmt.Printf("ok   %-22s %s  [%s]\n", name,
			strings.ReplaceAll(ref, "\n", " "), strings.Join(ran, " "))
	}
	if fails > 0 {
		fmt.Printf("\n%d case(s) failed\n", fails)
		os.Exit(1)
	}
	fmt.Println("\nall cases agree on every target")
}
