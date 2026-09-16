package emit

import (
	"math/rand"
	"testing"
)

// FARKAS IS SOUND, CHECKED AGAINST THE GROUND TRUTH.
//
// For random systems over three integer variables, each boxed into [-4, 4], every
// entailment farkas CLAIMS is checked by enumerating all 729 integer points: a
// point satisfying every fact must satisfy the goal. That is γ-soundness for the
// decision procedure (containment-2026-08-27's property, one level down), and a
// claim it gets wrong is a silently discharged obligation.
//
// The anti-vacuity guard: the test fails unless farkas proves a real share of the
// goals that ARE entailed, so a procedure that never answers yes cannot pass.
func TestFarkasIsSoundAgainstEnumeration(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	vars := []string{"x", "y", "z"}
	randRow := func() *linear {
		r := constant(int64(rng.Intn(13) - 6))
		for _, v := range vars {
			if c := int64(rng.Intn(7) - 3); c != 0 {
				r.coef[v] = c
			}
		}
		return r
	}
	eval := func(r *linear, pt map[string]int64) int64 {
		s := r.konst
		for v, c := range r.coef {
			s += c * pt[v]
		}
		return s
	}
	proved, entailed := 0, 0
	for trial := 0; trial < 4000; trial++ {
		var le []*linear
		for _, v := range vars { // the box: -4 <= v <= 4
			le = append(le, variable(v).addScaled(constant(-4), 1))
			le = append(le, constant(-4).addScaled(variable(v), -1))
		}
		for k := rng.Intn(4); k >= 0; k-- {
			le = append(le, randRow())
		}
		g := randRow()
		truth := true
		for x := int64(-4); x <= 4 && truth; x++ {
			for y := int64(-4); y <= 4 && truth; y++ {
				for z := int64(-4); z <= 4 && truth; z++ {
					pt := map[string]int64{"x": x, "y": y, "z": z}
					ok := true
					for _, f := range le {
						if eval(f, pt) > 0 {
							ok = false
							break
						}
					}
					if ok && eval(g, pt) > 0 {
						truth = false
					}
				}
			}
		}
		claim := farkas(le, g)
		if claim && !truth {
			t.Fatalf("trial %d: farkas claims %s <= 0 from %v, and a point in the box refutes it", trial, g, le)
		}
		if truth {
			entailed++
			if claim {
				proved++
			}
		}
	}
	if entailed == 0 || proved*2 < entailed {
		t.Errorf("farkas proved %d of %d entailed goals — too few for the test to mean anything", proved, entailed)
	}
	t.Logf("proved %d of %d entailed goals", proved, entailed)
}
