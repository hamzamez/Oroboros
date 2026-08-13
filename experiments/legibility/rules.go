package legibility

// SROA written as rewrite rules, plus the engine that runs them.
//
// The engine is counted separately from the rules, because it is amortised
// across every layer in the language; the rules are what a layer author writes.

// ---------------------------------------------------------------- the engine

type Pat struct {
	Meta string // if set, matches anything and binds
	Kind Kind
	I    int
	UseI bool
	Kids []Pat
	Rest string // matches all remaining children as a sequence, and binds them
}

type Bind struct {
	Self int            // index of the matched node, in the input
	Node map[string]int   // metavariable -> node index in the input
	Seq  map[string][]int // sequence variable -> all remaining children
}

type Ctx struct{ Split map[string]int } // local name -> field count

// Sub rewrites a child of the matched node.
type Sub func(int) int

type Rule struct {
	Name string
	LHS  Pat
	RHS  func(in, out *Fn, b Bind, c *Ctx, sub Sub) int
}

func m(name string) Pat         { return Pat{Meta: name} }
func p(k Kind, kids ...Pat) Pat { return Pat{Kind: k, Kids: kids} }
func pi(k Kind, i int, kids ...Pat) Pat {
	return Pat{Kind: k, I: i, UseI: true, Kids: kids}
}

func match(f *Fn, pat Pat, i int, b Bind) bool {
	if pat.Meta != "" {
		if prev, ok := b.Node[pat.Meta]; ok {
			return prev == i
		}
		b.Node[pat.Meta] = i
		return true
	}
	n := f.Nodes[i]
	if n.Kind != pat.Kind {
		return false
	}
	if pat.Rest != "" {
		if len(n.Kids) < len(pat.Kids) {
			return false
		}
		b.Seq[pat.Rest] = n.Kids[len(pat.Kids):]
	} else if len(n.Kids) != len(pat.Kids) {
		return false
	}
	if pat.UseI && n.I != pat.I {
		return false
	}
	for j, kp := range pat.Kids {
		if !match(f, kp, n.Kids[j], b) {
			return false
		}
	}
	return true
}

// Rewrite walks top-down, tries each rule at each node, and copies anything no
// rule matched. Rules rewrite their own children via sub, so a rule sees the
// unlowered form of what it matched — which is what makes the raise-before-lower
// discipline of g4 §8 possible.
func Rewrite(in *Fn, rules []Rule) *Fn {
	out := &Fn{}
	ctx := &Ctx{Split: map[string]int{}}
	memo := map[int]int{}

	var walk func(i int) int
	walk = func(i int) int {
		if r, ok := memo[i]; ok {
			return r
		}
		for _, rule := range rules {
			b := Bind{Self: i, Node: map[string]int{}, Seq: map[string][]int{}}
			if match(in, rule.LHS, i, b) {
				r := rule.RHS(in, out, b, ctx, walk)
				memo[i] = r
				return r
			}
		}
		n := in.Nodes[i]
		kids := make([]int, len(n.Kids))
		for j, k := range n.Kids {
			kids[j] = walk(k)
		}
		r := out.add(Node{Kind: n.Kind, Str: n.Str, F64: n.F64, I: n.I, Kids: kids})
		memo[i] = r
		return r
	}
	out.Root = walk(in.Root)
	return out
}

// ---------------------------------------------------------------- the rules

// SROARules is what a layer author writes. Four rules, any struct arity.
//
// `fields` is a sequence variable: it matches all remaining children of the
// struct, whatever their number. Adding that to the engine took twelve lines and
// is what makes these rules arity-general rather than fixed at two.
func SROARules() []Rule {
	return []Rule{
		{
			Name: "split-declaration",
			LHS:  p(KVar, Pat{Kind: KStruct, Rest: "fields"}),
			RHS: func(in, out *Fn, b Bind, c *Ctx, sub Sub) int {
				v := in.Nodes[b.Self].Str
				c.Split[v] = len(b.Seq["fields"])
				return out.Seq(mapFields(b.Seq["fields"], func(j, kid int) int {
					return out.Var(fieldName(v, j), sub(kid))
				})...)
			},
		},
		{
			Name: "split-assignment",
			LHS:  p(KSet, Pat{Kind: KStruct, Rest: "fields"}),
			RHS: func(in, out *Fn, b Bind, c *Ctx, sub Sub) int {
				v := in.Nodes[b.Self].Str
				// Always Par. Whether the fields actually conflict is the
				// backend's problem — this layer does not need to know.
				return out.Par(mapFields(b.Seq["fields"], func(j, kid int) int {
					return out.Set(fieldName(v, j), sub(kid))
				})...)
			},
		},
		{
			Name: "field-of-constructor",
			LHS:  p(KField, Pat{Kind: KStruct, Rest: "fields"}),
			RHS: func(in, out *Fn, b Bind, c *Ctx, sub Sub) int {
				return sub(b.Seq["fields"][in.Nodes[b.Self].I])
			},
		},
		{
			Name: "field-of-split-local",
			LHS:  p(KField, p(KRef)),
			RHS: func(in, out *Fn, b Bind, c *Ctx, sub Sub) int {
				self := in.Nodes[b.Self]
				name := in.Nodes[self.Kids[0]].Str
				if _, ok := c.Split[name]; !ok {
					return out.Field(self.I, sub(self.Kids[0]))
				}
				return out.Ref(fieldName(name, self.I))
			},
		},
	}
}

func mapFields(kids []int, f func(j, kid int) int) []int {
	out := make([]int, len(kids))
	for j, k := range kids {
		out[j] = f(j, k)
	}
	return out
}
