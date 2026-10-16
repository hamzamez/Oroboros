package core

import (
	"strings"
	"testing"
)

// THE ROLE MARKERS (spec/errors.md §2, ADR 0040). A variant may mark the
// constructor that is the value of the exception monad, (success c), and deny
// weakening for its values, (relevant). The compiler knows these two roles and
// no error type.

func TestAVariantDeclaresItsRoles(t *testing.T) {
	p, err := loadSrc(t, `set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant`)
	if err != nil {
		t.Fatal(err)
	}
	var s *Sum
	for _, x := range p.Sums {
		if x.Name == "result" {
			s = x
		}
	}
	if s == nil {
		t.Fatal("the variant was not loaded")
	}
	if s.Success != "ok" || !s.Relevant || len(s.Variants) != 2 {
		t.Errorf("roles not read: success %q, relevant %v, %d constructors", s.Success, s.Relevant, len(s.Variants))
	}
	if OptionSum().Success != "some" || OptionSum().Relevant {
		t.Errorf("option's success is some, and it is not relevant")
	}
}

func TestARoleMarkerIsChecked(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`set R = 'ok(Int) ∪ 'err(Int) success 'yes`, "names no constructor"},
		{`set R = 'ok(Int) ∪ 'err(Int) success 'ok success 'err`, "one success constructor"},
		{`set R = 'ok(Int) ∪ 'err(Int) relevant relevant`, "declared twice"},
		{`set R = 'ok(Int) ∪ 'err(Int) relevant 'err`, "takes nothing"},
		{`set R = 'ok(Int) ∪ 'success`, "reserved"},
		{`set R = 'ok(Int) ∪ 'err(Int) relevant Int`, "takes nothing"},
	} {
		_, err := loadSrc(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error saying %q, got %v", c.src, c.want, err)
		}
	}
}
