package core

import (
	"math"
	"math/big"
	"testing"
)

// THE CANONICAL FORM (num.go): one (m, k) per element, k least, read back
// exactly by every reader.
func TestNumeralCanonical(t *testing.T) {
	cases := []struct {
		in, out string
		m       int64
		k       int
	}{
		{"0.1", "0.1", 1, 1},
		{"2.50", "2.5", 25, 1},
		{"2.0", "2.0", 2, 0},
		{"0.0", "0.0", 0, 0},
		{"-0.0", "0.0", 0, 0}, // N has one zero
		{"1e-3", "0.001", 1, 3},
		{"1e3", "1000.0", 1000, 0},
		{"1.0825", "1.0825", 10825, 4},
		{"-12.340", "-12.34", -1234, 2},
		{"6.02e23", "602000000000000000000000.0", 0, 0},
	}
	for _, c := range cases {
		d, err := ParseNum(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if d.String() != c.out {
			t.Errorf("%s printed %s, want %s", c.in, d.String(), c.out)
		}
		if c.m != 0 || c.out == "0.0" {
			if d.M.Cmp(big.NewInt(c.m)) != 0 || d.K != c.k {
				t.Errorf("%s is (%s, %d), want (%d, %d)", c.in, d.M, d.K, c.m, c.k)
			}
		}
		back, err := ParseNum(d.String())
		if err != nil || !back.Equal(d) {
			t.Errorf("%s did not read back: %v %v", d, back, err)
		}
	}
	for _, bad := range []string{"", ".", "1.2.3", "e5", "1e", "--1"} {
		if _, err := ParseNum(bad); err == nil {
			t.Errorf("%q read as a numeral", bad)
		}
	}
}

// A LITERAL IS EXACT: 0.1 is one tenth, not the double nearest it, and lies
// in F64 only when it is a double (Theorem R's two directions).
func TestNumeralExact(t *testing.T) {
	d, _ := ParseNum("0.1")
	if d.Rat().Cmp(big.NewRat(1, 10)) != 0 {
		t.Errorf("0.1 is %s", d.Rat())
	}
	if _, exact := d.Float64(); exact {
		t.Errorf("0.1 claimed to be a double")
	}
	h, _ := ParseNum("0.5")
	if f, exact := h.Float64(); !exact || f != 0.5 {
		t.Errorf("0.5 is a double: %v %v", f, exact)
	}
	// Every finite double is an element, exactly, and comes back as itself.
	for _, f := range []float64{0.1, 1.0 / 3, math.SmallestNonzeroFloat64, math.MaxFloat64, -2.5, 1e21, 0} {
		e := DecOfFloat64(f)
		g, exact := e.Float64()
		if !exact || g != f {
			t.Errorf("%v → %s → %v (exact %v)", f, e, g, exact)
		}
		if e.Rat().Cmp(new(big.Rat).SetFloat64(f)) != 0 {
			t.Errorf("%v is not %s", f, e)
		}
	}
	// A planted fault: the element past F64's range is not a double.
	big1e400, _ := ParseNum("1e400")
	if _, exact := big1e400.Float64(); exact {
		t.Errorf("1e400 claimed to be a double")
	}
}
