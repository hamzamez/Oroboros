package main

import (
	"testing"
	"time"
)

// THE PAIRED CONFIRMATION (paired.go). A fake machine whose speed the test
// controls: each binary has a cost in work, and the machine multiplies it by
// its slowness at the moment of the call. What the pair must do is cancel the
// machine and keep the compiler.

type fakeMachine struct {
	cost     map[string]time.Duration // per binary
	slowness func(call int) float64
	calls    int
}

func (m *fakeMachine) run(gen, root string, key timeKey) (time.Duration, error) {
	s := m.slowness(m.calls)
	m.calls++
	return time.Duration(float64(m.cost[gen]) * s), nil
}

var freqGo = timeKey{"examples/io/freq.oro", "go"}

func suspect(base, now time.Duration) []slowCompile {
	return []slowCompile{{key: freqGo, base: base, now: now}}
}

func verdict(t *testing.T, m *fakeMachine, sweepBase, sweepNow time.Duration) (kept, cleared []slowCompile) {
	t.Helper()
	pairs, err := pairedTimes(m.run, "old", "base-tree", "new", "tree", []timeKey{freqGo}, pairRounds)
	if err != nil {
		t.Fatal(err)
	}
	return pairedVerdict(suspect(sweepBase, sweepNow), pairs)
}

// THE FALSE ALARM IS CLEARED: the sweep read freq at 1.63x (names-2026-09-25),
// the compiler is unchanged, and the machine is 1.6x slower all afternoon. The
// pair sees 1.00x.
func TestAPairClearsAMachineThatIsSlowerToday(t *testing.T) {
	m := &fakeMachine{
		cost:     map[string]time.Duration{"old": 12 * time.Second, "new": 12 * time.Second},
		slowness: func(int) float64 { return 1.6 },
	}
	kept, cleared := verdict(t, m, 16687*time.Millisecond, 27234*time.Millisecond)
	if len(kept) != 0 || len(cleared) != 1 {
		t.Fatalf("kept %v, cleared %v: an unchanged compiler on a slow machine is not a regression", keys(kept), keys(cleared))
	}
	if r := cleared[0].pairRatio(); r != 1 {
		t.Errorf("paired ratio %.2f, want 1.00", r)
	}
	// And one interference in one sample — the first of this binary's, doubled —
	// is what the minimum is for.
	spike := &fakeMachine{
		cost:     map[string]time.Duration{"old": 12 * time.Second, "new": 12 * time.Second},
		slowness: func(i int) float64 { return map[bool]float64{true: 2, false: 1}[i == 1] },
	}
	if kept, _ := verdict(t, spike, 16687*time.Millisecond, 27234*time.Millisecond); len(kept) != 0 {
		t.Errorf("one doubled sample convicted an unchanged compiler: the estimate is not the minimum")
	}
}

// A REAL REGRESSION SURVIVES, however the machine behaves: 1.6x the work is
// 1.6x in the pair on a quiet machine, a slow one, and one that alternates
// between P- and E-cores call by call.
func TestAPairKeepsARealRegression(t *testing.T) {
	for name, slow := range map[string]func(int) float64{
		"quiet":       func(int) float64 { return 1 },
		"slow":        func(int) float64 { return 1.6 },
		"alternating": func(i int) float64 { return []float64{1, 1.6}[i%2] },
	} {
		m := &fakeMachine{
			cost:     map[string]time.Duration{"old": 10 * time.Second, "new": 16 * time.Second},
			slowness: slow,
		}
		kept, _ := verdict(t, m, 16*time.Second, 26*time.Second)
		if len(kept) != 1 {
			t.Errorf("%s machine: a 1.6x regression was cleared", name)
			continue
		}
		if name != "alternating" && kept[0].pairRatio() < slowRatio {
			t.Errorf("%s machine: paired ratio %.2f", name, kept[0].pairRatio())
		}
	}
}

// THE PAIR IS INTERLEAVED. A machine that slows down steadily through the
// measurement — a neighbour's job starting — must not be read as the second
// binary being slower. Timing all of the baseline first and then all of this
// binary would put every baseline sample in the fast part: the minimum of each
// side would then be 1.0 against 1.9, a false regression.
func TestAPairIsInterleavedSoADriftCancels(t *testing.T) {
	m := &fakeMachine{
		cost:     map[string]time.Duration{"old": 10 * time.Second, "new": 10 * time.Second},
		slowness: func(i int) float64 { return 1 + 0.18*float64(i) },
	}
	kept, cleared := verdict(t, m, 10*time.Second, 16*time.Second)
	if len(kept) != 0 || len(cleared) != 1 {
		t.Fatalf("a steady drift was read as a regression: kept %v", keys(kept))
	}
}

// A SUSPECT NOTHING PAIRED STANDS: the pair clears, it never convicts by absence.
func TestAnUnpairedSuspectStands(t *testing.T) {
	kept, cleared := pairedVerdict(suspect(10*time.Second, 16*time.Second), map[timeKey]pairedSide{})
	if len(kept) != 1 || len(cleared) != 0 {
		t.Errorf("kept %v, cleared %v", keys(kept), keys(cleared))
	}
}

// THE PAIR IS HELD TO THE SAME RULE, delta included: a small compile at 2x but
// 100 ms more in the pair is not slower, exactly as it would not be in a sweep.
func TestThePairUsesTheSameRule(t *testing.T) {
	m := &fakeMachine{
		cost:     map[string]time.Duration{"old": 100 * time.Millisecond, "new": 200 * time.Millisecond},
		slowness: func(int) float64 { return 1 },
	}
	kept, _ := verdict(t, m, 200*time.Millisecond, 600*time.Millisecond)
	if len(kept) != 0 {
		t.Errorf("a 100 ms pair difference was kept: the delta half of the rule is missing")
	}
}
