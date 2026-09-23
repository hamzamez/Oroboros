package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// THE COMPILE-TIME GATE (compiletime.go). A gate that cannot fail proves
// nothing, and neither does one that always fails, so both halves are pinned on
// REAL sweeps recorded in testdata/ (compiletime-2026-09-21):
//
//   - sweep-80386e1 and sweep-7e36002 are the new sweep run at the commits
//     either side of the regression the assessment found unnoticed;
//   - sweep-identical-{1,2,3} are three sweeps of one compiler.

func loadSweep(t *testing.T, name string) map[timeKey]time.Duration {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseTimes(string(b))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func keys(s []slowCompile) []string {
	var out []string
	for _, c := range s {
		out = append(out, c.key.String())
	}
	return out
}

// THE REGRESSION THAT WENT UNNOTICED IS CAUGHT. At 7e36002 the tokeniser went
// 234 → 2,109 ms in the sweep, and freq and jsonfmt with it; the rule flags
// exactly those compiles and not the ones that moved within noise (tree 1.16x,
// render 1.08x, big/render 1.25x, and freq on the JVM at 1.30x).
func TestTheGateCatchesTheRegressionThatWentUnnoticed(t *testing.T) {
	got := keys(slower(loadSweep(t, "sweep-80386e1.txt"), loadSweep(t, "sweep-7e36002.txt")))
	want := []string{
		"examples/io/freq.oro go",
		"examples/io/freq.oro js",
		"examples/io/jsonfmt.oro go",
		"examples/io/jsonfmt.oro js",
		"examples/io/jsonfmt.oro java",
		"examples/json/tokenize.oro go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the gate at 7e36002:\n got %v\nwant %v", got, want)
	}
}

// AND IT FLAGS NOTHING WHEN NOTHING CHANGED: every ordering of three sweeps of
// one compiler, including the tick-quantised small programs that vary 4x.
func TestIdenticalSweepsAreNeverSlower(t *testing.T) {
	names := []string{"sweep-identical-1.txt", "sweep-identical-2.txt", "sweep-identical-3.txt"}
	for _, a := range names {
		for _, b := range names {
			if a == b {
				continue
			}
			if s := slower(loadSweep(t, a), loadSweep(t, b)); len(s) > 0 {
				t.Errorf("baseline %s, now %s: flagged %v", a, b, keys(s))
			}
		}
	}
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

// THE RULE'S TWO HALVES, at their edges, so neither can be dropped silently.
func TestTheRuleNeedsBothTheRatioAndTheDelta(t *testing.T) {
	k := timeKey{"p.oro", "go"}
	for _, c := range []struct {
		base, now int64
		slow      bool
		why       string
	}{
		{1000, 2000, true, "2x on a heavy compile"},
		{234, 2109, true, "a SMALL program that became slow — the tokeniser's shape"},
		{16, 47, false, "one tick against three: a ratio of 2.9 that means nothing"},
		{1000, 1490, false, "under the ratio, whatever the delta"},
		{498, 747, false, "at the ratio, one millisecond under the delta"},
		{500, 750, true, "at the ratio and at the delta"},
		{10000, 12000, false, "1.2x: machine drift, not a regression"},
	} {
		got := len(slower(map[timeKey]time.Duration{k: ms(c.base)}, map[timeKey]time.Duration{k: ms(c.now)})) > 0
		if got != c.slow {
			t.Errorf("%d → %d ms (%s): slower = %v, want %v", c.base, c.now, c.why, got, c.slow)
		}
	}
}

// A NEW COMPILE HAS NOTHING TO BE SLOWER THAN, and a removed one is not a
// regression.
func TestOnlyCompilesInBothTablesAreCompared(t *testing.T) {
	base := map[timeKey]time.Duration{{"gone.oro", "go"}: ms(100)}
	now := map[timeKey]time.Duration{{"new.oro", "go"}: ms(90000)}
	if s := slower(base, now); len(s) > 0 {
		t.Errorf("a compile in one table only was compared: %v", keys(s))
	}
}

// DRIFT IS THE MEDIAN, which a uniform slowdown moves and one regression does
// not — so the note can say "the machine" rather than list every program.
func TestDriftSeparatesTheMachineFromOneProgram(t *testing.T) {
	base := map[timeKey]time.Duration{}
	machine := map[timeKey]time.Duration{}
	oneSlow := map[timeKey]time.Duration{}
	for i, n := range []int64{400, 800, 1600, 3200, 6400} {
		k := timeKey{string(rune('a'+i)) + ".oro", "go"}
		base[k] = ms(n)
		machine[k] = ms(n * 13 / 10)
		oneSlow[k] = ms(n)
	}
	oneSlow[timeKey{"c.oro", "go"}] = ms(1600 * 3)
	if med, n := drift(base, machine); n != 5 || med < 1.29 || med > 1.31 {
		t.Errorf("a uniform 1.3x: median %.3f over %d, want 1.30 over 5", med, n)
	}
	if med, _ := drift(base, oneSlow); med != 1 {
		t.Errorf("one program 3x slower must not move the median: got %.3f", med)
	}
	if s := slower(base, machine); len(s) > 0 {
		t.Errorf("a uniform 1.3x drift is under the rule, flagged %v", keys(s))
	}
}

// THE BASELINE IS A MINIMUM, per compile, over two sweeps.
func TestMinTimesIsPerCompile(t *testing.T) {
	a := map[timeKey]time.Duration{{"x", "go"}: ms(100), {"y", "go"}: ms(50), {"only-a", "go"}: ms(7)}
	b := map[timeKey]time.Duration{{"x", "go"}: ms(80), {"y", "go"}: ms(60), {"only-b", "go"}: ms(9)}
	want := map[timeKey]time.Duration{{"x", "go"}: ms(80), {"y", "go"}: ms(50), {"only-a", "go"}: ms(7), {"only-b", "go"}: ms(9)}
	if got := minTimes(a, b); !reflect.DeepEqual(got, want) {
		t.Errorf("minTimes:\n got %v\nwant %v", got, want)
	}
}

func TestTimesRoundTrip(t *testing.T) {
	m := loadSweep(t, "sweep-identical-1.txt")
	back, err := parseTimes(formatTimes(m))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, back) {
		t.Error("formatTimes then parseTimes lost something")
	}
	if !strings.HasPrefix(formatTimes(m), "#") {
		t.Error("the baseline must say what it is in a header")
	}
}

// FASTER IS THE MIRROR, so a buy-back can be locked in. Run backwards, the
// historical sweeps are a compiler that got faster: the same six compiles
// must be called faster, and identical sweeps nothing.
func TestTheGateSeesAFixAsWellAsARegression(t *testing.T) {
	got := keys(faster(loadSweep(t, "sweep-7e36002.txt"), loadSweep(t, "sweep-80386e1.txt")))
	want := keys(slower(loadSweep(t, "sweep-80386e1.txt"), loadSweep(t, "sweep-7e36002.txt")))
	if !reflect.DeepEqual(got, want) || len(got) != 6 {
		t.Errorf("the regression undone:\n got %v\nwant %v", got, want)
	}
	names := []string{"sweep-identical-1.txt", "sweep-identical-2.txt", "sweep-identical-3.txt"}
	for _, a := range names {
		for _, b := range names {
			if a != b {
				if f := faster(loadSweep(t, a), loadSweep(t, b)); len(f) > 0 {
					t.Errorf("baseline %s, now %s: called faster %v", a, b, keys(f))
				}
			}
		}
	}
	k := timeKey{"p.oro", "go"}
	for _, c := range []struct {
		base, now int64
		fast      bool
	}{
		{2000, 1000, true},
		{2109, 234, true},
		{47, 16, false},   // one tick against three
		{750, 500, true},  // at the ratio and at the delta
		{747, 498, false}, // one millisecond under the delta
		{1490, 1000, false},
	} {
		got := len(faster(map[timeKey]time.Duration{k: ms(c.base)}, map[timeKey]time.Duration{k: ms(c.now)})) > 0
		if got != c.fast {
			t.Errorf("%d → %d ms: faster = %v, want %v", c.base, c.now, got, c.fast)
		}
	}
}
