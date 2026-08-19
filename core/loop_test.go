package core

import "testing"

func TestLoopDesugars(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(loop ((acc 0.0) (i 0)) (lt i 10) (again (add acc 1.0) (add i 1)) else acc)`,
			`(loop (fn (acc i) (if (lt i 10) (again (add acc 1.0) (add i 1)) acc)) 0.0 0)`},
		{`(loop ((i 0)) (ge i n) -1 (hit i) i else (again (add i 1)))`,
			`(loop (fn (i) (if (ge i n) -1 (if (hit i) i (again (add i 1))))) 0)`},
	} {
		got, err := ReadTerm(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got.String() != c.want {
			t.Errorf("\n got %s\nwant %s", got, c.want)
		}
	}
	for _, bad := range []string{
		`(loop ((i 0)) (lt i 3) (again (add i 1)))`,
		`(loop ((i 0)) else i (lt i 3) (again (add i 1)))`,
		`(loop ((i 0)) (lt i 3) (if p (again (add i 1)) (again i)) else i)`,
		`(loop ((i 0)) (lt i 3) (f (again (add i 1))) else i)`,
		`(loop ((i 0) (i 1)) else i)`,
		`(loop ((again 0)) else again)`,
		`(loop ((i 0) (j 0)) (lt i 3) (again (add i 1)) else i)`,
		`(loop () else 1)`,
	} {
		if _, err := ReadTerm(bad); err == nil {
			t.Errorf("should be rejected: %s", bad)
		}
	}
	// `again` under a let IS allowed — let binds, if branches.
	if _, err := ReadTerm(`(loop ((i 0)) (lt i 3) (let (f i) (fn (x) (again x))) else i)`); err != nil {
		t.Errorf("again under a let must be allowed: %v", err)
	}
}

// A loop's `again` is BOUND, so it is not residual.
//
// This is the bug that made `build` refuse every program containing a loop
// while `gen` accepted them: Residual read free names and `again` is not a
// primitive, so `oro` and `gen` were happy and the one command that produces a
// binary reported `not in normal form for target "go": again`. ADR 0015 was
// validated entirely through `gen`, and nothing built a binary until the
// windows target needed one.
func TestAgainIsNotResidual(t *testing.T) {
	body, err := ReadTerm(`(fn (n) (loop ((i 0)) (lt i n) (again (add i 1)) else i))`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	env := &Env{Prim: map[string]bool{"lt": true, "add": true, "loop": true, "if": true}}
	if left := Residual(body, env); len(left) != 0 {
		t.Errorf("residual should be empty, got %v", left)
	}
	// An `again` outside any loop is still free, and still reported.
	loose, err := ReadTerm(`(f (again 1))`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	found := false
	for _, n := range Residual(loose, env) {
		if n == "again" {
			found = true
		}
	}
	if !found {
		t.Error("an `again` with no enclosing loop should be residual")
	}
}
