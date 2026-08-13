package legibility

import "testing"

func TestBothAgreeOnCentroid(t *testing.T) {
	a := SROAPass(Centroid()).String()
	b := Rewrite(Centroid(), SROARules()).String()
	t.Logf("pass:\n%s", a)
	t.Logf("rules:\n%s", b)
	if a != b {
		t.Errorf("pass and rules disagree")
	}
}

func TestBothPreserveSimultaneity(t *testing.T) {
	a := SROAPass(Swap()).String()
	b := Rewrite(Swap(), SROARules()).String()
	t.Logf("pass:\n%s", a)
	t.Logf("rules:\n%s", b)
	if a != b {
		t.Errorf("pass and rules disagree on the swap")
	}
}

func TestBothHandleArityThree(t *testing.T) {
	a := SROAPass(Triple()).String()
	b := Rewrite(Triple(), SROARules()).String()
	if a != b {
		t.Errorf("disagree on arity 3:\npass:\n%s\nrules:\n%s", a, b)
	}
	t.Logf("both:\n%s", b)
}
