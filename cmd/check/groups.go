package main

// THE COMPILER STEP, SPLIT (CLAUDE.md plan item 0, hamza 2026-11-20).
//
// One `go test` over the compiler's packages runs each package's tests in one
// process, one after another, so the step's wall time was its longest
// package's, cmd/build's: 1,930 s of a 62-minute check. The step is now a
// schedule.
//
// THE PROBLEM is makespan scheduling, P||Cmax: jobs, the top-level tests, each
// of a measured weight wᵢ; machines, the processes run at once. No schedule on
// m machines finishes before
//
//	τ = max(maxᵢ wᵢ, Σᵢ wᵢ / m),
//
// a test being indivisible and the work conserved. P||Cmax is NP-hard, and
// Graham's LPT rule (1969), the heaviest job first, each to the machine then
// lightest, is within (4/3 − 1/(3m)) of the optimum. Here a machine is a
// `go test -run '^(A|B|…)$' PKG` process, so a group holds tests of one package:
// a package of weight Wₚ is split into kₚ = ⌈Wₚ/τ⌉ groups by LPT, and the
// packages light enough for one group share one `go test`, which runs them in
// parallel itself. Every group is its own process, so a planted fault one test
// sets is never another group's (ADR: no switch leaks).
//
// THE PARTITION IS CHECKED, since a schedule that drops a test is a check that
// runs less while reporting a pass (CLAUDE.md: a path nothing runs is a path
// nothing checks). Every test `go test -list` names is in exactly one group,
// and every planned test reports pass, fail or skip in its group's -json
// stream; a test that does not report fails the step.
//
// THE WEIGHTS are gauntlet/check/testtimes.txt, measured by this step's own
// runs and re-recorded with -accept -retime; a test the table does not know
// weighs the table's median. The weights choose the groups and nothing else:
// a verdict is the same under every partition.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// compilerPackages are the step's packages, as `go test` patterns.
var compilerPackages = []string{"./core/", "./emit/", "./ir/...", "./cmd/..."}

// testKey names a top-level test of a package.
type testKey struct{ pkg, test string }

// group is one `go test` process: tests of one package, or whole packages.
type group struct {
	pkgs  []string // whole packages, run together
	pkg   string   // or one package's tests,
	tests []string // these
	load  float64  // its planned seconds
}

func (g group) name() string {
	if g.pkg == "" {
		return fmt.Sprintf("%d packages", len(g.pkgs))
	}
	return fmt.Sprintf("%s (%d tests)", strings.TrimPrefix(g.pkg, "oroboros/"), len(g.tests))
}

// PlantGroupDropsATest leaves one planned test out of its group: the step must
// then fail, naming it (TestTheSplitCompilerStepRunsEveryTest).
var PlantGroupDropsATest bool

// lpt is Graham's LPT schedule of the weighted jobs onto k machines: the
// heaviest first, each to the machine then lightest (ties to the lowest index,
// so the schedule is a function of its input).
func lpt(jobs []string, w map[string]float64, k int) ([][]string, []float64) {
	order := append([]string(nil), jobs...)
	sort.SliceStable(order, func(i, j int) bool {
		if w[order[i]] != w[order[j]] {
			return w[order[i]] > w[order[j]]
		}
		return order[i] < order[j]
	})
	out := make([][]string, k)
	load := make([]float64, k)
	for _, j := range order {
		best := 0
		for m := 1; m < k; m++ {
			if load[m] < load[best] {
				best = m
			}
		}
		out[best] = append(out[best], j)
		load[best] += w[j]
	}
	for m := range out {
		sort.Strings(out[m])
	}
	return out, load
}

// plan is the schedule of every listed test onto m machines.
func plan(listed map[string][]string, times map[testKey]float64, m int) []group {
	median := medianTime(times)
	weight := func(k testKey) float64 {
		if t, ok := times[k]; ok {
			return t
		}
		return median
	}
	var pkgs []string
	total, heaviest := 0.0, 0.0
	wp := map[string]float64{}
	for p, ts := range listed {
		pkgs = append(pkgs, p)
		for _, t := range ts {
			x := weight(testKey{p, t})
			wp[p] += x
			total += x
			heaviest = math.Max(heaviest, x)
		}
	}
	sort.Strings(pkgs)
	tau := math.Max(heaviest, total/float64(max(m, 1)))
	var out []group
	small := group{}
	for _, p := range pkgs {
		k := int(math.Ceil(wp[p] / tau))
		if k <= 1 {
			small.pkgs = append(small.pkgs, p)
			small.load = math.Max(small.load, wp[p])
			continue
		}
		w := map[string]float64{}
		for _, t := range listed[p] {
			w[t] = weight(testKey{p, t})
		}
		parts, loads := lpt(listed[p], w, k)
		for i, ts := range parts {
			if len(ts) > 0 {
				out = append(out, group{pkg: p, tests: ts, load: loads[i]})
			}
		}
	}
	if len(small.pkgs) > 0 {
		out = append([]group{small}, out...)
	}
	if PlantGroupDropsATest {
		for i := range out {
			if len(out[i].tests) > 0 {
				out[i].tests = out[i].tests[1:]
				if len(out[i].tests) == 0 {
					out = append(out[:i], out[i+1:]...)
				}
				break
			}
		}
	}
	return out
}

func medianTime(times map[testKey]float64) float64 {
	if len(times) == 0 {
		return 1
	}
	var xs []float64
	for _, t := range times {
		xs = append(xs, t)
	}
	sort.Float64s(xs)
	return xs[len(xs)/2]
}

// readTestTimes reads "package test seconds" lines.
func readTestTimes(path string) map[testKey]float64 {
	out := map[testKey]float64{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || strings.HasPrefix(l, "#") {
			continue
		}
		if s, err := strconv.ParseFloat(f[2], 64); err == nil {
			out[testKey{f[0], f[1]}] = s
		}
	}
	return out
}

func writeTestTimes(path string, times map[testKey]float64) error {
	var keys []testKey
	for k := range times {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].pkg != keys[j].pkg {
			return keys[i].pkg < keys[j].pkg
		}
		return keys[i].test < keys[j].test
	})
	var b strings.Builder
	b.WriteString("# gauntlet/check/testtimes.txt — each compiler test's seconds, the weights the compiler\n" +
		"# step schedules by (cmd/check/groups.go). Written by -accept -retime. Do not edit by hand.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %s %.2f\n", k.pkg, k.test, times[k])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// listTests is every top-level test, example and fuzz target of the step's
// packages, by `go test -list` (benchmarks do not run in a test run).
func (c *checker) listTests() (map[string][]string, error) {
	args := append([]string{"list", "-f", "{{.ImportPath}} {{len .TestGoFiles}} {{len .XTestGoFiles}}"}, compilerPackages...)
	cmd := exec.Command("go", args...)
	cmd.Dir = c.root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v", err)
	}
	listed := map[string][]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || f[1] == "0" && f[2] == "0" {
			continue // no tests
		}
		p := f[0]
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command("go", "test", "-list", ".", p)
			cmd.Dir = c.root
			b, err := cmd.Output()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("go test -list %s: %v\n%s", p, err, b)
				}
				return
			}
			for _, name := range strings.Split(string(b), "\n") {
				name = strings.TrimSpace(name)
				if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Example") || strings.HasPrefix(name, "Fuzz") {
					listed[p] = append(listed[p], name)
				}
			}
		}()
	}
	wg.Wait()
	return listed, firstErr
}

// testEvent is one line of `go test -json`.
type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// compilerStep runs the schedule, every group at once.
func (c *checker) compilerStep() result {
	listed, err := c.listTests()
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	times := readTestTimes(filepath.Join(c.root, "gauntlet", "check", "testtimes.txt"))
	groups := plan(listed, times, c.testProcs)
	type ran struct {
		g      group
		text   string
		err    error
		report map[testKey]testEvent
		took   time.Duration
	}
	runs := make([]ran, len(groups))
	var wg sync.WaitGroup
	for i, g := range groups {
		wg.Add(1)
		go func() {
			defer wg.Done()
			args := []string{"test", "-count=1", "-json", "-timeout", "40m"}
			if g.pkg == "" {
				args = append(args, g.pkgs...)
			} else {
				quoted := make([]string, len(g.tests))
				for j, t := range g.tests {
					quoted[j] = regexp.QuoteMeta(t)
				}
				args = append(args, "-run", "^("+strings.Join(quoted, "|")+")$", g.pkg)
			}
			cmd := exec.Command("go", args...)
			cmd.Dir = c.root
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			t0 := time.Now()
			err := cmd.Run()
			took := time.Since(t0).Round(time.Second)
			text, report := readTestJSON(out.Bytes())
			runs[i] = ran{g, text, err, report, took}
		}()
	}
	wg.Wait()

	// every listed test reports, in exactly the group it was planned in
	var log strings.Builder
	var failed []string
	seen := map[testKey]bool{}
	observed := map[testKey]float64{}
	for i, r := range runs {
		fmt.Fprintf(&log, "══ group %d: %s, planned %.0f s, took %s\n", i+1, r.g.name(), r.g.load, r.took)
		log.WriteString(r.text)
		for k, e := range r.report {
			seen[k] = true
			if e.Action == "pass" || e.Action == "fail" {
				observed[k] = e.Elapsed
			}
		}
		if r.err != nil {
			failed = append(failed, fmt.Sprintf("group %d (%s): %v", i+1, r.g.name(), r.err))
		}
	}
	var missing []string
	for p, ts := range listed {
		for _, t := range ts {
			if !seen[testKey{p, t}] {
				missing = append(missing, strings.TrimPrefix(p, "oroboros/")+"."+t)
			}
		}
	}
	sort.Strings(missing)
	logPath := filepath.Join(c.work, "logs", "compiler.log")
	_ = os.WriteFile(logPath, []byte(log.String()), 0o644)
	_ = writeTestTimes(filepath.Join(c.work, "testtimes.txt"), observed)
	rel, _ := filepath.Rel(c.root, logPath)
	n := 0
	for _, ts := range listed {
		n += len(ts)
	}
	longest := 0
	for i, r := range runs {
		if r.took > runs[longest].took {
			longest = i
		}
	}
	detail := fmt.Sprintf("%d tests in %d processes, the longest %s (%s) — %s", n, len(groups),
		runs[longest].took, runs[longest].g.name(), filepath.ToSlash(rel))
	if len(missing) > 0 {
		return result{status: fail, detail: fmt.Sprintf("%d test(s) did not run: %s — %s",
			len(missing), strings.Join(first(missing, 5), ", "), filepath.ToSlash(rel))}
	}
	if len(failed) > 0 {
		var tails []string
		for _, r := range runs {
			if r.err != nil {
				tails = append(tails, tail(failedLines(r.text), 30))
			}
		}
		return result{status: fail, detail: strings.Join(failed, "; ") + " — " + filepath.ToSlash(rel),
			log: strings.Join(tails, "\n")}
	}
	return result{status: pass, detail: detail}
}

func first(xs []string, n int) []string {
	if len(xs) > n {
		return append(xs[:n:n], "…")
	}
	return xs
}

// readTestJSON is a -json stream as the text `go test -v` prints, and each
// top-level test's last action.
func readTestJSON(b []byte) (string, map[testKey]testEvent) {
	var text strings.Builder
	report := map[testKey]testEvent{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var e testEvent
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			text.Write(sc.Bytes()) // the build's own output, a compile error
			text.WriteString("\n")
			continue
		}
		text.WriteString(e.Output)
		if e.Test != "" && !strings.Contains(e.Test, "/") &&
			(e.Action == "pass" || e.Action == "fail" || e.Action == "skip") {
			report[testKey{e.Package, e.Test}] = e
		}
	}
	return text.String(), report
}

// failedLines keeps a failing group's text from its first failure on.
func failedLines(s string) string {
	if i := strings.Index(s, "--- FAIL"); i >= 0 {
		return s[i:]
	}
	return s
}
