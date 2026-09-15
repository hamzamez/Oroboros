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
func expandCase(t *Term, look ctorLookup) (*Term, error) {
	if t == nil {
		return t, nil
	}
	if t.Kind == KApp && len(t.Kids) > 0 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "case" {
		return caseForm(t, look)
	}
	if len(t.Kids) == 0 {
		return t, nil
	}
	out := *t
	out.Kids = make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := expandCase(k, look)
		if err != nil {
			return nil, err
		}
		out.Kids[i] = x
	}
	return &out, nil
}

// A VARIANT TYPE IS ITS DECLARATION, NOT ITS SPELLING (spec/data.md §5.5.1).
//
// Resolution is a renaming ρ from what a module can write to the declarations
// it means, and the one law it must obey is INJECTIVITY on declarations that
// differ: two `result`s in modules a and b are two types because ρ sends them to
// `a.result` and `b.result`. The old table was keyed by the spelling, which is ρ
// composed with forgetting the module — not injective — so the later declaration
// overwrote the earlier.
//
// A `case` pattern is a constructor, and a constructor is a NAME, so it resolves
// by exactly the rule every other name in the module does (`Module.resolve`):
// unqualified in the module's own declarations, `alias.c` through a `use`. A
// global table had made patterns the one kind of name that ignored scope, with
// three measured consequences — a root module's variant captured by a module
// that never imported it, an imported one written unqualified accepted with its
// tag left free, and the qualified spelling refused.
type sumRef struct {
	key string // the declaration's qualified name; the language's `option` is "option"
	sum *Sum
}

// ctorRef is a constructor after resolution: the variant type it belongs to,
// its local name in that declaration, and the spelling of its tag in the
// module that wrote the pattern — which `Module.resolve` then qualifies exactly
// as it qualifies the constructor itself.
type ctorRef struct {
	ref   sumRef
	local string
	tag   string
}

type ctorLookup func(spelling string) (ctorRef, error)

// sumKey is ρ on a declaration: its module path and its name. The language's
// `option` is the declaration with no path (langSums).
func sumKey(path string, s *Sum) string { return qualify(path, s.Name) }

func caseForm(t *Term, look ctorLookup) (*Term, error) {
	if len(t.Kids) < 4 {
		return nil, fmt.Errorf("case takes a scrutinee and at least two clauses: %s", t)
	}
	scrut, err := expandCase(t.Kids[1], look)
	if err != nil {
		return nil, err
	}
	rest := t.Kids[2:]
	if len(rest)%2 != 0 {
		return nil, fmt.Errorf("case: every clause is a pattern and a body, so the "+
			"clause list has an even length; got %d terms", len(rest))
	}

	type clause struct {
		variant string // the pattern's spelling; "" for else
		bind    string // "" when the variant carries nothing, or it is else
		tag     string // the tag's spelling in this module
		body    *Term
	}
	var cls []clause
	var on *sumRef
	seen := map[string]bool{}
	for i := 0; i < len(rest); i += 2 {
		pat, raw := rest[i], rest[i+1]
		body, err := expandCase(raw, look)
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
			return nil, fmt.Errorf("case: a clause pattern is a constructor, a constructor "+
				"and one binding — `(ok v)` — or `else`; got %s", pat)
		}
		if c.variant != "" {
			ctor, err := look(c.variant)
			if err != nil {
				return nil, err
			}
			// By IDENTITY of the declaration, which is its key — not by pointer,
			// since every module holds its own copy of `option`, and not by
			// spelling, which is the non-injective map this replaced.
			if on != nil && ctor.ref.key != on.key {
				return nil, fmt.Errorf("case: %s is a constructor of %s, but this case is "+
					"on %s — one case eliminates one variant type", c.variant, ctor.ref.key, on.key)
			}
			on = &ctor.ref
			if seen[ctor.local] {
				return nil, fmt.Errorf("case: %s is matched twice", c.variant)
			}
			seen[ctor.local] = true
			if c.bind != "" && ctor.ref.sum.payloadOf(ctor.local) == "" {
				return nil, fmt.Errorf("case: %s carries no payload, so `(%s %s)` has "+
					"nothing to bind — write `%s`", c.variant, c.variant, c.bind, c.variant)
			}
			c.tag = ctor.tag
		}
		cls = append(cls, c)
	}
	if on == nil {
		return nil, fmt.Errorf("case: no clause names a constructor, so there is nothing to " +
			"eliminate — use `if` or `match`")
	}
	if err := checkCase(on.sum, seen, cls[len(cls)-1].variant == ""); err != nil {
		return nil, err
	}

	// Innermost first: the LAST clause is unconditional, which is what
	// exhaustiveness earns — see the comment on expandCase.
	body := renameFree(cls[len(cls)-1].body, bindOf(cls[len(cls)-1].bind))
	for i := len(cls) - 2; i >= 0; i-- {
		c := cls[i]
		test := &Term{Kind: KApp, Kids: []*Term{
			Name("="), Name("#t"), Name(c.tag)}}
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

func (s *Sum) has(variant string) bool {
	for _, v := range s.Variants {
		if v.Name == variant {
			return true
		}
	}
	return false
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
