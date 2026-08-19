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
