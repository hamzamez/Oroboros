// Command check runs every check this repository has, in one command, with
// nothing excluded.
//
//	go run ./cmd/check                   every step
//	go run ./cmd/check -skip tooling     every step but the host surveys
//	go run ./cmd/check -only emission    one step, or a comma-separated list
//	go run ./cmd/check -accept "why"     keep this run's emission as the baseline, and log why
//
// THE STEPS, in order. Every step runs, whatever the ones before it did, so one
// run reports everything:
//
//	vet          go vet ./...
//	compiler     go test ./core/ ./emit/ ./ir/ ./cmd/...
//	emission     every .oro under examples/, lib/ and the differential cases, on
//	             every target, compared with the baseline committed in gauntlet/check/
//	ir           every program that emitted, lowered to the IR and verified; the
//	             canonical printing round-trips (docs/spec/ir.md §11)
//	differential gauntlet/differential: build, RUN and agree on four targets
//	tooling      gauntlet/stdlib: the surveys twice, the pins, the acceptance programs
//
// THE RULE FOR EMISSION (gauntlet/check/README.md). Emitted code is compared byte
// for byte with the baseline. A difference is neither a pass nor a failure: it is
// a CHANGE, reported for review. A change is KEPT when the new result is correct
// and no slower — the suites pass, and a benchmark covering the program does not
// regress — and `-accept` then makes it the baseline and logs the reason. A change
// that is not an improvement is a regression, and the code is fixed instead.
//
// `-accept` is refused unless the compiler and differential steps ran and passed
// IN THE SAME RUN: the reason is written by a person, the correctness evidence is
// not.
//
// It exists because this repository's checks were a list run by hand, and the
// list had a hole in it: the emission sweep skipped examples/tally, which is how
// tally's refusal reached the tooling suite unseen (hex-2026-09-14 §4).
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type status string

const (
	pass   status = "pass"
	fail   status = "FAIL"
	review status = "REVIEW"
	skip   status = "skip"
)

type result struct {
	status status
	detail string
	took   time.Duration
}

type step struct {
	name string
	run  func(c *checker) result
}

type checker struct {
	root    string // the repository root
	work    string // .check/, git-ignored: this run's emission and every log
	jobs    int
	results map[string]result

	// What the emission step found, for -accept to act on once every step has run.
	emitted     []outcome
	changes     []change
	summary     string // the sweep's counts, without any verdict
	noBaseline  bool
	emissionRan bool

	// What the compile-time gate found (compiletime.go).
	times          map[timeKey]time.Duration // the estimate -accept records
	slow           []slowCompile
	fast           []slowCompile // compiles faster than the baseline, which -accept records
	retime         bool          // -retime: re-record the time baseline though nothing crossed the rule
	timeNote       string
	noTimeBaseline bool
	swept2         bool // a second sweep has run, so c.times is a minimum of two
}

var stepNames = []string{"vet", "compiler", "emission", "ir", "differential", "tooling"}

func main() {
	skipList := flag.String("skip", "", "comma-separated steps to skip")
	onlyList := flag.String("only", "", "comma-separated steps to run, and no others")
	accept := flag.String("accept", "", "keep this run's emission as the baseline; the argument is the reason, logged in gauntlet/check/ACCEPTED.md")
	jobs := flag.Int("jobs", runtime.NumCPU(), "parallel compilations in the emission sweep")
	// A GAIN BELOW THE GATE'S RESOLUTION CAN STILL BE LOCKED IN, deliberately.
	// The rule is 1.5x either way, and in the sweep a real 1.78x serial speed-up
	// measured 1.48x (tokenize-compile-2026-09-23). Left alone, the baseline would
	// keep the old cost, and a regression all the way back would pass unflagged.
	retime := flag.Bool("retime", false, "with -accept: re-record the compile-time baseline even when no compile crossed the rule")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: check [-skip S,…] [-only S,…] [-accept REASON]\n"+
			"steps: %s\n", strings.Join(stepNames, ", "))
		flag.PrintDefaults()
	}
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(1)
	}
	c := &checker{
		root:    root,
		work:    filepath.Join(root, ".check"),
		jobs:    *jobs,
		results: map[string]result{},
		retime:  *retime,
	}
	if err := os.MkdirAll(filepath.Join(c.work, "logs"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(1)
	}

	steps := []step{
		{"vet", func(c *checker) result { return c.command("vet", c.root, "go", "vet", "./...") }},
		{"compiler", func(c *checker) result {
			return c.command("compiler", c.root, "go", "test", "-count=1", "./core/", "./emit/", "./ir/", "./cmd/...")
		}},
		{"emission", (*checker).emission},
		{"ir", (*checker).ir},
		{"differential", func(c *checker) result {
			return c.command("differential", filepath.Join(c.root, "gauntlet", "differential"), "go", "run", "run.go")
		}},
		{"tooling", func(c *checker) result {
			return c.command("tooling", c.root, "go", "test", "-count=1", "-timeout", "60m", "./gauntlet/stdlib/")
		}},
	}
	only, err := stepSet(*onlyList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	skipped, err := stepSet(*skipList)
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}

	for _, s := range steps {
		if (len(only) > 0 && !only[s.name]) || skipped[s.name] {
			c.results[s.name] = result{status: skip}
			continue
		}
		fmt.Printf("── %s\n", s.name)
		start := time.Now()
		r := s.run(c)
		r.took = time.Since(start).Round(time.Second)
		c.results[s.name] = r
		fmt.Printf("   %s  %s  %s\n", r.status, r.took, r.detail)
	}

	if *accept != "" {
		c.acceptRun(*accept)
	}

	fmt.Println("\n── summary")
	code := 0
	for _, name := range stepNames {
		r := c.results[name]
		fmt.Printf("   %-13s %-7s %s\n", name, r.status, r.detail)
		switch r.status {
		case fail:
			code = 1
		case review:
			if code == 0 {
				code = 2
			}
		}
	}
	if code == 2 {
		fmt.Println("\n   Emission changed. Review it (gauntlet/check/README.md). Keep it with -accept \"reason\"\n" +
			"   only if it is correct and nothing a benchmark covers got slower; otherwise fix the code.")
	}
	os.Exit(code)
}

// acceptRun applies -accept after every step has run, because the evidence it
// requires — the differential suite — runs after the emission step.
func (c *checker) acceptRun(reason string) {
	r := c.results["emission"]
	switch {
	case !c.emissionRan:
		fmt.Println("\n── accept: refused — the emission step did not run")
		c.results["emission"] = result{status: fail, detail: "-accept needs the emission step"}
		return
	case r.status == fail:
		fmt.Println("\n── accept: refused — the emission step failed")
		return
	case len(c.changes) == 0 && !c.noBaseline && len(c.slow) == 0 && len(c.fast) == 0 && !c.noTimeBaseline && !c.retime:
		fmt.Println("\n── accept: nothing to accept — emission is byte-identical to the baseline and no compile is slower or faster")
		return
	}
	for _, need := range []string{"compiler", "differential"} {
		if got := c.results[need]; got.status != pass {
			fmt.Printf("\n── accept: refused — %s must pass in the same run, and it is %s\n", need, got.status)
			c.results["emission"] = result{status: review,
				detail: r.detail + fmt.Sprintf(" — NOT accepted: %s is %s", need, got.status)}
			return
		}
	}
	tooling := c.results["tooling"].status
	if tooling == fail {
		fmt.Println("\n── accept: refused — the tooling step failed")
		c.results["emission"] = result{status: review, detail: r.detail + " — NOT accepted: tooling failed"}
		return
	}
	if err := c.acceptBaseline(reason, tooling); err != nil {
		c.results["emission"] = result{status: fail, detail: "accepting: " + err.Error()}
		return
	}
	fmt.Println("\n── accept: the baseline is this run's emission; logged in gauntlet/check/ACCEPTED.md")
	what := fmt.Sprintf("ACCEPTED %d change(s)", len(c.changes))
	if len(c.fast) > 0 {
		what += fmt.Sprintf(" and %d faster compile(s)", len(c.fast))
	}
	if len(c.slow) > 0 {
		what += fmt.Sprintf(" and %d slower compile(s)", len(c.slow))
	}
	if c.noBaseline {
		what = "ACCEPTED as the initial baseline"
	} else if c.noTimeBaseline && len(c.changes) == 0 {
		what = "ACCEPTED the initial compile-time baseline"
	}
	c.results["emission"] = result{status: pass, detail: what + " — " + c.summary}
}

// command runs one external step, logs everything it prints to .check/logs, and
// shows the tail of the log when it fails.
func (c *checker) command(name, dir string, argv ...string) result {
	logPath := filepath.Join(c.work, "logs", name+".log")
	var out bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	_ = os.WriteFile(logPath, out.Bytes(), 0o644)
	rel, _ := filepath.Rel(c.root, logPath)
	if err != nil {
		fmt.Println(tail(out.String(), 30))
		return result{status: fail, detail: fmt.Sprintf("%v — %s", err, filepath.ToSlash(rel))}
	}
	return result{status: pass, detail: filepath.ToSlash(rel)}
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, l := range lines {
		lines[i] = "   │ " + l
	}
	return strings.Join(lines, "\n")
}

func stepSet(list string) (map[string]bool, error) {
	known := map[string]bool{}
	for _, s := range stepNames {
		known[s] = true
	}
	set := map[string]bool{}
	for _, s := range strings.Split(list, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !known[s] {
			return nil, fmt.Errorf("no step named %q (steps: %s)", s, strings.Join(stepNames, ", "))
		}
		set[s] = true
	}
	return set, nil
}

// repoRoot walks up from the working directory to the go.mod that declares
// module oroboros.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		f, err := os.Open(filepath.Join(dir, "go.mod"))
		if err == nil {
			sc := bufio.NewScanner(f)
			isRoot := sc.Scan() && strings.TrimSpace(sc.Text()) == "module oroboros"
			f.Close()
			if isRoot {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside the oroboros repository (no go.mod declaring module oroboros)")
		}
		dir = parent
	}
}
