package core

import (
	"fmt"
	"sort"
	"strings"
)

// `case` is the sum's eliminator, and it expands HERE rather than in the reader
// because the reader sees one file and a sum may be declared in another.
//
//	(case r
//	  (ok v)  body1
//	  (err e) body2)
//
// becomes
//
//	(r (fn (#t #p)
//	     (if (= #t ok#tag) body1[v := #p]
//	                       body2[e := #p])))
//
// `r` is the tag/payload PRODUCT that a constructor builds — `(values 0 x)`,
// which is `(fn (#x) (#x 0 x))` — so applying it to a two-parameter function is
// just the product's own elimination, already built on all four targets
// (values.md). Nothing new reaches the reducer: no term kind, no rule.
//
// Two consequences worth stating.
//
// The STATIC case is free and always was (sums-research.md §0): `(case (ok 3) …)`
// beta-reduces to `body1[v := 3]` with no tag anywhere in the residual, which is
// the Church-encoded sum working exactly as the research said it did.
//
// The LAST clause carries no test. That is not an optimisation, it is what
// exhaustiveness buys: the clauses cover the declared variants, so once the
// others are excluded the last one is the only thing left. `checkCase` is what
// earns it, and an `else` clause is how a deliberately partial match says so.
func expandCase(t *Term, sums map[string]*Sum, byVariant map[string]*Sum) (*Term, error) {
	if t == nil {
		return t, nil
	}
	if t.Kind == KApp && len(t.Kids) > 0 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "case" {
		return caseForm(t, sums, byVariant)
	}
	if len(t.Kids) == 0 {
		return t, nil
	}
	out := *t
	out.Kids = make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := expandCase(k, sums, byVariant)
		if err != nil {
			return nil, err
		}
		out.Kids[i] = x
	}
	return &out, nil
}

func caseForm(t *Term, sums map[string]*Sum, byVariant map[string]*Sum) (*Term, error) {
	if len(t.Kids) < 4 {
		return nil, fmt.Errorf("case takes a scrutinee and at least two clauses: %s", t)
	}
	scrut, err := expandCase(t.Kids[1], sums, byVariant)
	if err != nil {
		return nil, err
	}
	rest := t.Kids[2:]
	if len(rest)%2 != 0 {
		return nil, fmt.Errorf("case: every clause is a pattern and a body, so the "+
			"clause list has an even length; got %d terms", len(rest))
	}

	type clause struct {
		variant string // "" for else
		bind    string // "" when the variant carries nothing, or it is else
		body    *Term
	}
	var cls []clause
	var sum *Sum
	seen := map[string]bool{}
	for i := 0; i < len(rest); i += 2 {
		pat, raw := rest[i], rest[i+1]
		body, err := expandCase(raw, sums, byVariant)
		if err != nil {
			return nil, err
		}
		var c clause
		switch {
		case pat.Kind == KName && pat.Name == "else":
			if i+2 != len(rest) {
				return nil, fmt.Errorf("case: `else` must be the last clause; the ones " +
					"after it could never be reached")
			}
			c = clause{body: body}
		case pat.Kind == KName:
			c = clause{variant: pat.Name, body: body}
		case pat.Kind == KApp && len(pat.Kids) == 2 &&
			pat.Kids[0].Kind == KName && pat.Kids[1].Kind == KName:
			c = clause{variant: pat.Kids[0].Name, bind: pat.Kids[1].Name, body: body}
		default:
			return nil, fmt.Errorf("case: a clause pattern is a variant name, a variant "+
				"and one binding — `(ok v)` — or `else`; got %s", pat)
		}
		if c.variant != "" {
			owner, ok := byVariant[c.variant]
			if !ok {
				return nil, fmt.Errorf("case: %s is not a variant of any sum in scope. "+
					"A sum is declared with `(sum name (variant type) …)`", c.variant)
			}
			if sum != nil && owner != sum {
				return nil, fmt.Errorf("case: %s is a variant of %s, but this case is "+
					"on %s — one case eliminates one sum", c.variant, owner.Name, sum.Name)
			}
			sum = owner
			if seen[c.variant] {
				return nil, fmt.Errorf("case: %s is matched twice", c.variant)
			}
			seen[c.variant] = true
			if c.bind != "" && owner.payloadOf(c.variant) == "" {
				return nil, fmt.Errorf("case: %s carries no payload, so `(%s %s)` has "+
					"nothing to bind — write `%s`", c.variant, c.variant, c.bind, c.variant)
			}
		}
		cls = append(cls, c)
	}
	if sum == nil {
		return nil, fmt.Errorf("case: no clause names a variant, so there is nothing to " +
			"eliminate — use `if` or `match`")
	}
	if err := checkCase(sum, seen, cls[len(cls)-1].variant == ""); err != nil {
		return nil, err
	}

	// Innermost first: the LAST clause is unconditional, which is what
	// exhaustiveness earns — see the comment on expandCase.
	body := renameFree(cls[len(cls)-1].body, bindOf(cls[len(cls)-1].bind))
	for i := len(cls) - 2; i >= 0; i-- {
		c := cls[i]
		test := &Term{Kind: KApp, Kids: []*Term{
			Name("="), Name("#t"), Name(c.variant + "#tag")}}
		body = &Term{Kind: KApp, Kids: []*Term{
			Name("if"), test, renameFree(c.body, bindOf(c.bind)), body}}
	}
	return &Term{Kind: KApp, Kids: []*Term{scrut, Fn([]string{"#t", "#p"}, body)}}, nil
}

func bindOf(name string) map[string]string {
	if name == "" {
		return nil
	}
	return map[string]string{name: "#p"}
}

func (s *Sum) payloadOf(variant string) string {
	for _, v := range s.Variants {
		if v.Name == variant {
			return v.Payload
		}
	}
	return ""
}

// checkCase is exhaustiveness, and it is the reason the last clause needs no
// test. A sum is CLOSED and FINITE by construction, so "did the clauses cover
// it" is decidable by counting — which is the cheap half of what
// sums-research.md §5.2 says pattern-matching costs, and the expensive half
// (nested patterns, ML's usefulness algorithm) is not owed because our patterns
// are flat.
func checkCase(sum *Sum, seen map[string]bool, hasElse bool) error {
	if hasElse {
		if len(seen) == len(sum.Variants) {
			return fmt.Errorf("case on %s: every variant is matched, so `else` is dead "+
				"code — remove it", sum.Name)
		}
		return nil
	}
	var missing []string
	for _, v := range sum.Variants {
		if !seen[v.Name] {
			missing = append(missing, v.Name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("case on %s is not exhaustive: %s %s unmatched. Add %s a clause, "+
			"or an `else`", sum.Name, strings.Join(missing, ", "),
			plural(len(missing), "is", "are"), plural(len(missing), "it", "them"))
	}
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// uniformPayload is the condition for a sum to CROSS A BOUNDARY: its variants
// must agree on a payload type, because the value transmitted is a tag and a
// payload and the payload gets one slot.
//
// A variant carrying nothing agrees with anything — it uses the slot and
// ignores it, which is what a niche encoding does with the space it does not
// need. Inside a program a mixed sum is fine, because reduction removes it.
func (s *Sum) uniformPayload() (string, bool) {
	ty := ""
	for _, v := range s.Variants {
		if v.Payload == "" {
			continue
		}
		if ty == "" {
			ty = v.Payload
			continue
		}
		if ty != v.Payload {
			return "", false
		}
	}
	if ty == "" {
		ty = "int" // an enum: the tag alone, with an unused payload slot
	}
	return ty, true
}
