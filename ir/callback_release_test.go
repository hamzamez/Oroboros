package ir

import "testing"

// A TABLE A CALLBACK READS LIVES TO THE CALL THAT APPLIES IT (ADR 0143). The
// λ's region runs at the call, 0.. times, so the release analysis places it
// there: a table the λ reads is released after EnumWindows returns. The fault
// places the region at its `fn` statement, as an ordinary statement's region
// is, and the release falls between the two, where the λ, run by the host,
// reads freed memory. Freed memory often still holds its bytes, so the
// witness is the order itself, not a run.
func TestATableACallbackReadsLivesToItsCall(t *testing.T) {
	const src = `use win/print
use windows/winuser
use windows/errhandlingapi

export fn main() {
    t = array { errhandlingapi.GetLastError() % 2, 5, 7 }
    print.print_int(winuser.EnumWindows(fn(h) {
        s = loop j = 0, s = 0 { j < len(t) -> again(j + 1, s + t[j]); else -> s }
        if s > 11 { 1 } else { 0 }
    }))
}
`
	order := func() (call, release int) {
		_, f, _ := doubleEntry(t, src)
		call, release = -1, -1
		for i, s := range f.Body.Stmts {
			if s.Op == OCall && s.Name == "windows/winuser.EnumWindows" && call < 0 {
				call = i
			}
			// the release, realized by the allocator's term: a statement
			// whose regions free the table's storage
			if release < 0 && frees(&f.Body.Stmts[i]) {
				release = i
			}
		}
		return call, release
	}
	call, release := order()
	if call < 0 || release < 0 {
		t.Fatalf("main should call EnumWindows and release the table: call %d, release %d", call, release)
	}
	if release < call {
		t.Errorf("the table is released at statement %d, before EnumWindows at %d applies the λ that reads it", release, call)
	}
	PlantFnReleasedEarly = true
	defer func() { PlantFnReleasedEarly = false }()
	if call, release := order(); release > call {
		t.Errorf("with the region placed at its fn statement the release should come first, so the witness can fail: call %d, release %d", call, release)
	}
}

// frees says s, or a region inside it, calls the allocator's free.
func frees(s *Stmt) bool {
	if s.Op == OCall && (s.Name == "windows/heapapi.HeapFree" || s.Name == "windows/memoryapi.VirtualFree") {
		return true
	}
	for _, r := range s.Sub {
		for i := range r.Stmts {
			if frees(&r.Stmts[i]) {
				return true
			}
		}
	}
	return false
}
