package emit

import (
	"fmt"
	"math/big"

	"oroboros/core"
)

// THE HULL OF A TELESCOPE'S SET (book finding 9). `i ∈ 0..len(a)-1` reaches
// the target in separation normal form, i ∈ 0.. where i <= len(a) - 1 (core's
// openRange), and an entry's parameter takes its representation from its set
// (ADR 0079), so the open end would hold i above the word. The set's hull is
// target data: sup {x : x <= e(ā)} = sup e over the other parameters' sets,
// len(p) ∈ [0, max_len]. hullSig narrows each open end a `where` conjunct
// bounds, by e's interval; the conjunct itself stays, decided at each call.
// The target applies it to every header when it makes a program's
// environment (Env), so every stage reads the narrowed set.
func hullSig(tg *Target, sig *core.Sig) *core.Sig {
	if sig == nil || sig.Where == nil || PlantNoTelescopeHull {
		return sig
	}
	ranges := map[string][2]*big.Int{}
	tables := map[string]bool{}
	for _, p := range sig.Params {
		if lo, hi, ok := core.RangeEnds(p.Type); ok {
			ranges[p.Name] = [2]*big.Int{lo, hi}
		}
		if core.ArrayElem(p.Type) != "" || core.IsBuffer(p.Type) {
			tables[p.Name] = true
		}
	}
	maxLen := tg.Word.Hi
	if tg.MaxLen > 0 && tg.MaxLen < maxLen {
		maxLen = tg.MaxLen
	}
	// eval is e's interval over the parameters' sets; ok false where unknown
	var eval func(e *core.Term) (lo, hi *big.Int, ok bool)
	eval = func(e *core.Term) (*big.Int, *big.Int, bool) {
		switch e.Kind {
		case core.KInt:
			return big.NewInt(e.Int), big.NewInt(e.Int), true
		case core.KName:
			if r, ok := ranges[e.Name]; ok && r[0] != nil && r[1] != nil {
				return r[0], r[1], true
			}
			return nil, nil, false
		case core.KApp:
			op, args := e.Op(), e.Args()
			if op.Kind != core.KName {
				return nil, nil, false
			}
			if op.Name == "len" && len(args) == 1 && args[0].Kind == core.KName && tables[args[0].Name] {
				return big.NewInt(0), big.NewInt(maxLen), true
			}
			if len(args) != 2 {
				return nil, nil, false
			}
			al, ah, ok1 := eval(args[0])
			bl, bh, ok2 := eval(args[1])
			if !ok1 || !ok2 {
				return nil, nil, false
			}
			switch op.Name {
			case "+":
				return new(big.Int).Add(al, bl), new(big.Int).Add(ah, bh), true
			case "-":
				return new(big.Int).Sub(al, bh), new(big.Int).Sub(ah, bl), true
			}
		}
		return nil, nil, false
	}
	var conj []*core.Term
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "if" && len(t.Args()) == 3 &&
			t.Args()[2].Kind == core.KBool && !t.Args()[2].IsTrue() {
			walk(t.Args()[0])
			walk(t.Args()[1])
			return
		}
		conj = append(conj, t)
	}
	walk(sig.Where)
	var out *core.Sig
	for i, p := range sig.Params {
		lo, hi, ok := core.RangeEnds(p.Type)
		if !ok || (lo != nil && hi != nil) {
			continue
		}
		for _, c := range conj {
			if c.Kind != core.KApp || c.Op().Kind != core.KName || c.Op().Name != "<=" || len(c.Args()) != 2 {
				continue
			}
			a, b := c.Args()[0], c.Args()[1]
			switch {
			case hi == nil && a.Kind == core.KName && a.Name == p.Name:
				if _, h, ok := eval(b); ok {
					hi = h
				}
			case lo == nil && b.Kind == core.KName && b.Name == p.Name:
				if l, _, ok := eval(a); ok {
					lo = l
				}
			}
		}
		if lo == nil || hi == nil || lo.Cmp(hi) > 0 {
			continue
		}
		if out == nil {
			c := *sig
			c.Params = append([]core.SigParam(nil), sig.Params...)
			out = &c
		}
		out.Params[i].Type = fmt.Sprintf("int %s %s", lo, hi)
	}
	if out == nil {
		return sig
	}
	return out
}

// PlantNoTelescopeHull leaves a telescope's open end open: the export's
// parameter is then held above the word (TestATelescopeIsASeparation).
var PlantNoTelescopeHull bool
