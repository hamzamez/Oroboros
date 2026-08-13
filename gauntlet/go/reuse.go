package gauntlet

import "sync/atomic"

// Cross-boundary reuse: what it costs to decide uniqueness at a function
// boundary, by each available strategy.
//
// g7 priced a uniqueness false negative at 40x-1540x. Everything derived so far
// is intraprocedural. These measure the three ways to decide it when the value
// crosses a call:
//
//	Static    — the caller's grade crosses in the signature. Nothing at runtime.
//	RC        — check a reference count at the update site (Perceus, Koka/Lean).
//	RC atomic — same, but sound under concurrency.
//	Copy      — give up and copy (the g7 penalty).
//
// All are //go:noinline so the boundary is real. Go inlines aggressively enough
// that without this the measurement would be of no boundary at all.

type RCDict struct {
	m  map[string]int
	rc int32
}

func NewRCDict(m map[string]int) *RCDict { return &RCDict{m: m, rc: 1} }

//go:noinline
func InsertStatic(d *RCDict, k string, v int) *RCDict {
	d.m[k] = v
	return d
}

//go:noinline
func InsertRC(d *RCDict, k string, v int) *RCDict {
	if d.rc == 1 {
		d.m[k] = v
		return d
	}
	n := &RCDict{m: DictCopy(d.m), rc: 1}
	n.m[k] = v
	return n
}

//go:noinline
func InsertRCAtomic(d *RCDict, k string, v int) *RCDict {
	if atomic.LoadInt32(&d.rc) == 1 {
		d.m[k] = v
		return d
	}
	n := &RCDict{m: DictCopy(d.m), rc: 1}
	n.m[k] = v
	return n
}

//go:noinline
func InsertCopy(d *RCDict, k string, v int) *RCDict {
	n := &RCDict{m: DictCopy(d.m), rc: 1}
	n.m[k] = v
	return n
}

// Threading the same value through several real boundaries, which is the case
// intraprocedural liveness cannot see. Thread3RC models Perceus-style *borrowed*
// parameters: the callee does not retain, so no reference count is bumped and
// the uniqueness check succeeds.

//go:noinline
func Thread3Static(d *RCDict) *RCDict {
	d = InsertStatic(d, "p", 1)
	d = InsertStatic(d, "q", 2)
	d = InsertStatic(d, "r", 3)
	return d
}

//go:noinline
func Thread3RC(d *RCDict) *RCDict {
	d = InsertRC(d, "p", 1)
	d = InsertRC(d, "q", 2)
	d = InsertRC(d, "r", 3)
	return d
}

// Naive retain/release around every call — what you get from reference counting
// done without a borrowing analysis.
//
// The retain makes rc==2 for the duration of the call, so the uniqueness check
// inside InsertRC always FAILS and copies. That is not a flaw in the benchmark;
// it is the classic naive-RC failure mode, and avoiding it is precisely what
// Perceus contributes. Compare Thread3RC above, which models borrowed
// (non-consuming) parameters and emits no retain at all.

//go:noinline
func Thread3RCFull(d *RCDict) *RCDict {
	d.rc++
	d = InsertRC(d, "p", 1)
	d.rc--
	d.rc++
	d = InsertRC(d, "q", 2)
	d.rc--
	d.rc++
	d = InsertRC(d, "r", 3)
	d.rc--
	return d
}

//go:noinline
func Thread3RCFullAtomic(d *RCDict) *RCDict {
	atomic.AddInt32(&d.rc, 1)
	d = InsertRCAtomic(d, "p", 1)
	atomic.AddInt32(&d.rc, -1)
	atomic.AddInt32(&d.rc, 1)
	d = InsertRCAtomic(d, "q", 2)
	atomic.AddInt32(&d.rc, -1)
	atomic.AddInt32(&d.rc, 1)
	d = InsertRCAtomic(d, "r", 3)
	atomic.AddInt32(&d.rc, -1)
	return d
}
