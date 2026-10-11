package emit

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"oroboros/presburger"
)

// THE PRESBURGER MEASUREMENT — settypes-research.md §8, ADR 0049 decision 6.
//
// Before the refinement layer's entailment is replaced, every question it is
// asked is put to a complete procedure as well: the Omega test over ℤ
// (package presburger). entails answers as it always has; when the
// environment names a directory in OROBOROS_ENTAIL_LOG, each question, today's
// answer and the Omega test's are written to a file there, one per process, so
// `cmd/check -only emission` measures the whole corpus without changing what
// it compiles. The aggregation is gauntlet/results/presburger-2026-10-11.md's.

// entails is the refinement layer's entailment: facts ⊢ goal ≤ 0.
func (f *facts) entails(goal *linear) bool {
	today := f.entailsFM(goal)
	if entailLog != nil {
		entailLog.record(f, goal, today)
	}
	return today
}

type entailRecorder struct {
	mu  sync.Mutex
	out io.Writer
}

var entailLog = openEntailLog()

func openEntailLog() *entailRecorder {
	dir := os.Getenv("OROBOROS_ENTAIL_LOG")
	if dir == "" {
		return nil
	}
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("entail-%d.log", os.Getpid())))
	if err != nil {
		return nil
	}
	fmt.Fprintf(f, "# %s\n", strings.Join(os.Args[1:], " "))
	return &entailRecorder{out: f}
}

// record puts the question to the Omega test and writes both answers: facts ⊢
// g ≤ 0 holds over ℤ exactly when the facts with g ≥ 1 have no integer
// solution.
func (r *entailRecorder) record(f *facts, goal *linear, today bool) {
	rows := f.substituted()
	g := f.substitute(goal)
	cs := make([]presburger.Constraint, 0, len(rows)+1)
	var key []string
	for _, e := range rows {
		cs = append(cs, constraintOf(e, -1, 0)) // e ≤ 0 is −e ≥ 0
		key = append(key, e.String()+"<=0")
	}
	cs = append(cs, constraintOf(g, 1, -1)) // the goal negated: g − 1 ≥ 0
	sort.Strings(key)
	start := time.Now()
	res := presburger.Satisfiable(cs)
	took := time.Since(start)
	omega := "unknown"
	switch res {
	case presburger.Unsat:
		omega = "proved"
	case presburger.Sat:
		omega = "refuted"
	}
	_, file, line, _ := runtime.Caller(2)
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "%v\t%s\t%d\t%s:%d\t%s |- %s<=0\n", today, omega, took.Microseconds(),
		filepath.Base(file), line, strings.Join(key, " & "), g)
}

// constraintOf is s·e + k ≥ 0 as a constraint.
func constraintOf(e *linear, s, k int64) presburger.Constraint {
	c := presburger.Constraint{Coef: make(map[string]int64, len(e.coef)), K: s*e.konst + k}
	for x, a := range e.coef {
		c.Coef[x] = s * a
	}
	return c
}
