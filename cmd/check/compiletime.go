package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// COMPILE TIME, GATED AGAINST THE BASELINE — gauntlet/check/README.md, "The rule
// for compile time", and compiletime-2026-09-21.
//
// It exists because the tokeniser went from 117 ms to 1,361 ms at 7e36002 and
// nothing noticed: two results each said "no slower", measured against the
// commit before, and the sweep's TOTAL did not move because one slow program
// among 456 parallel compilations is invisible in a sum.
//
// WHAT IS MEASURED is the CPU time — user plus system — of each `gen` process in
// the emission sweep. That is not the serial time a person waits for: the sweep
// runs 16 compilations at once on a hybrid P/E-core machine, and the tokeniser
// costs about 2.5x its serial time there. But it is REPEATABLE, because the job
// order is fixed and so is the contention it meets: across three sweeps of one
// compiler the heavy compiles varied by at most 1.21x (max/min), and the sweep's
// total by 0.6%. A gate needs repeatability against a baseline taken the same
// way, not an absolute truth.
//
// THE NOISE MODEL. An observation is t·m·(1+ε), where m is the machine's speed
// that day (the same binary measured 7.0 s on 2026-09-17 and 8.8 s on
// 2026-09-21: m moved 26%) and ε ≥ 0, because interference only ever ADDS time —
// an E-core, a cache miss, a neighbour's GC. Under positive noise the MINIMUM over
// repeated observations is the consistent estimator of t·m (Chen & Revels,
// "Robust benchmarking in noisy environments", 2016), which is why a baseline is
// the minimum of two sweeps and why a suspect is confirmed by a second one.

const (
	// A compile is SLOWER when both hold. Measured, not chosen:
	//   - 1.5 sits above the largest max/min seen between identical sweeps for a
	//     compile over 300 ms (1.21), and below the smallest regression this gate
	//     exists for (freq, 1.67x at 7e36002);
	//   - 250 ms sits above what Windows' 15.6 ms CPU-time tick does to a small
	//     program, where 16 ms against 47 ms is one tick against three and a ratio
	//     means nothing.
	// Against the sweeps either side of 7e36002 the rule flags six compiles; against
	// every ordering of three identical sweeps it flags none.
	slowRatio = 1.5
	slowDelta = 250 * time.Millisecond

	// gatedFloor is where a ratio starts to mean something, for the drift figure:
	// below it the 100-300 ms compiles varied by up to 1.74x between identical
	// sweeps. The rule above needs no floor of its own, because slowDelta is one —
	// which is what lets it catch a SMALL program that became slow (the tokeniser's
	// baseline was 234 ms, under this floor).
	gatedFloor = 300 * time.Millisecond

	// driftNote is where the median ratio says the machine, rather than one
	// program, has changed speed since the baseline.
	driftNote = 1.3
)

// timeKey orders compiles as the outcomes are ordered: by source, then target.
type timeKey struct{ source, target string }

func (k timeKey) String() string { return k.source + " " + k.target }

func timesOf(list []outcome) map[timeKey]time.Duration {
	m := make(map[timeKey]time.Duration, len(list))
	for _, o := range list {
		m[timeKey{o.Source, o.Target}] = o.CPU
	}
	return m
}

// minTimes is the per-compile minimum of two sweeps — the estimator under
// positive noise.
func minTimes(a, b map[timeKey]time.Duration) map[timeKey]time.Duration {
	out := make(map[timeKey]time.Duration, len(a))
	for k, v := range a {
		out[k] = v
		if w, ok := b[k]; ok && w < v {
			out[k] = w
		}
	}
	for k, w := range b {
		if _, ok := a[k]; !ok {
			out[k] = w
		}
	}
	return out
}

func sortedKeys(m map[timeKey]time.Duration) []timeKey {
	order := map[string]int{}
	for i, t := range targets {
		order[t.name] = i
	}
	keys := make([]timeKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		return order[keys[i].target] < order[keys[j].target]
	})
	return keys
}

const timesHeader = "# Compile time per source and target: the CPU time of the gen process in the\n" +
	"# emission sweep, in milliseconds, the minimum of two sweeps. Written by -accept;\n" +
	"# the rule is in README.md, \"The rule for compile time\".\n"

func formatTimes(m map[timeKey]time.Duration) string {
	var b strings.Builder
	b.WriteString(timesHeader)
	for _, k := range sortedKeys(m) {
		fmt.Fprintf(&b, "%s %s %d\n", k.source, k.target, m[k]/time.Millisecond)
	}
	return b.String()
}

// formatRun is this run's times with the wall clock beside them, for .check/.
func formatRun(list []outcome) string {
	var b strings.Builder
	b.WriteString("# source target cpu_ms wall_ms\n")
	for _, o := range sorted(list) {
		fmt.Fprintf(&b, "%s %s %d %d\n", o.Source, o.Target, o.CPU/time.Millisecond, o.Wall/time.Millisecond)
	}
	return b.String()
}

func parseTimes(text string) (map[timeKey]time.Duration, error) {
	m := map[timeKey]time.Duration{}
	for n, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		f := strings.Fields(raw)
		if len(f) < 3 {
			return nil, fmt.Errorf("compiletime.txt line %d: malformed %q", n+1, raw)
		}
		ms, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil || ms < 0 {
			return nil, fmt.Errorf("compiletime.txt line %d: bad milliseconds %q", n+1, f[2])
		}
		m[timeKey{f[0], f[1]}] = time.Duration(ms) * time.Millisecond
	}
	return m, nil
}

type slowCompile struct {
	key       timeKey
	base, now time.Duration
}

func (s slowCompile) ratio() float64 {
	if s.base <= 0 {
		return 0
	}
	return float64(s.now) / float64(s.base)
}

// slower is every compile the rule calls slower, in the outcomes' order. A
// compile with no baseline time is new and has nothing to be slower than.
func slower(base, now map[timeKey]time.Duration) []slowCompile {
	var out []slowCompile
	for _, k := range sortedKeys(now) {
		b, ok := base[k]
		if !ok {
			continue
		}
		n := now[k]
		if float64(n) >= slowRatio*float64(b) && n-b >= slowDelta {
			out = append(out, slowCompile{k, b, n})
		}
	}
	return out
}

// drift is the median of now/base over the compiles heavy enough for a ratio to
// mean something. A regression moves a few of them; the machine moves all of
// them, so a median well above 1 says "the machine", and a list of slower
// compiles under a median near 1 says "the compiler".
func drift(base, now map[timeKey]time.Duration) (float64, int) {
	var rs []float64
	for k, n := range now {
		b, ok := base[k]
		if !ok || b <= 0 || (b < gatedFloor && n < gatedFloor) {
			continue
		}
		rs = append(rs, float64(n)/float64(b))
	}
	if len(rs) == 0 {
		return 1, 0
	}
	sort.Float64s(rs)
	if len(rs)%2 == 1 {
		return rs[len(rs)/2], len(rs)
	}
	return (rs[len(rs)/2-1] + rs[len(rs)/2]) / 2, len(rs)
}

// compileTime screens this run against the baseline, confirms any suspect with a
// second sweep, and leaves the estimate -accept would record in c.times.
func (c *checker) compileTime(baseDir string, outs []outcome) (note string, slow []slowCompile, err error) {
	if err := os.WriteFile(filepath.Join(c.work, "compiletime.txt"), []byte(formatRun(outs)), 0o644); err != nil {
		return "", nil, err
	}
	now := timesOf(outs)
	c.times = now
	text, err := os.ReadFile(filepath.Join(baseDir, "compiletime.txt"))
	switch {
	case os.IsNotExist(err):
		c.noTimeBaseline = true
		return "no compile-time baseline yet; -accept records one", nil, nil
	case err != nil:
		return "", nil, err
	}
	base, err := parseTimes(string(text))
	if err != nil {
		return "", nil, err
	}
	confirmed := ""
	if len(slower(base, now)) > 0 {
		// A SUSPECT EXPLAINS ITSELF before it is recorded: the same sweep again, the
		// per-compile minimum, and the rule applied to that. A real regression
		// survives; a neighbour's GC does not.
		if err := c.secondSweep(outs); err != nil {
			return "", nil, err
		}
		now = c.times
		confirmed = "; a second sweep confirmed it"
	}
	slow = slower(base, now)
	med, n := drift(base, now)
	note = fmt.Sprintf("compile time %.2fx the baseline (median of %d compiles over %d ms)%s",
		med, n, gatedFloor/time.Millisecond, confirmed)
	if med >= driftNote {
		note += fmt.Sprintf(" — the median itself is %.2fx, so the MACHINE is slower than when the baseline was "+
			"taken; re-run idle and on mains power before reading the list as regressions", med)
	}
	return note, slow, nil
}

// secondSweep runs the sweep again, takes the per-compile minimum into c.times,
// and checks that emission did not change between the two — the emitter must be
// a function of its input (CLAUDE.md), and this is running it twice.
func (c *checker) secondSweep(first []outcome) error {
	if c.swept2 {
		return nil
	}
	again, err := c.sweep(filepath.Join(c.work, "emitted-2"))
	if err != nil {
		return err
	}
	if diff := compare(first, again); len(diff) > 0 {
		return fmt.Errorf("emission is not deterministic: two sweeps of one tree differ on %s (%s)",
			diff[0].key, diff[0].kind)
	}
	c.times = minTimes(c.times, timesOf(again))
	c.swept2 = true
	return nil
}
