package gauntlet

// Cycles: the standard failure of reference counting, and the reason Koka and
// Lean restrict what can be cyclic. s4 left this as the last untested thing that
// could genuinely hurt, because the design now leans on an RC fallback exactly
// where static analysis cannot reach.
//
// Two questions, and only the second is measurable:
//
//	1. Can a cycle be constructed at all, given that the only mutation is
//	   compiler-emitted in-place update gated on liveness?  (analytic — see
//	   docs/derivations/s5-cycles.md)
//	2. If cycles are forbidden, what does the workaround cost?  Graphs then have
//	   to be expressed as an arena plus integer indices, which is what Rust
//	   programs do for the same reason.
//
// These measure (2): the same graph, same traversal, as pointers and as indices.

// ---------------------------------------------------------------- pointer graph

type PNode struct {
	Val int32
	Adj []*PNode
}

// BuildPointerGraph makes a graph that is genuinely cyclic: node i points to
// i+1, i+7, i+53 and i-1, all modulo n. Every node is reachable from itself.
func BuildPointerGraph(n, deg int) []*PNode {
	nodes := make([]*PNode, n)
	for i := range nodes {
		nodes[i] = &PNode{Val: int32(i)}
	}
	offs := []int{1, 7, 53, -1}
	for i, nd := range nodes {
		nd.Adj = make([]*PNode, 0, deg)
		for d := 0; d < deg; d++ {
			j := ((i+offs[d%len(offs)])%n + n) % n
			nd.Adj = append(nd.Adj, nodes[j])
		}
	}
	return nodes
}

func SumPointerGraph(nodes []*PNode) int64 {
	var acc int64
	for _, nd := range nodes {
		for _, m := range nd.Adj {
			acc += int64(m.Val)
		}
	}
	return acc
}

// ---------------------------------------------------------------- index graph

// IndexGraph is the same graph as an arena plus integer indices. A cycle in the
// data is fine; there is no cycle in the *references*, so nothing here defeats a
// reference count, and the GC has no pointers to trace.
type IndexGraph struct {
	Vals []int32
	Adj  []int32 // flattened, deg entries per node
	Deg  int
}

func BuildIndexGraph(n, deg int) IndexGraph {
	g := IndexGraph{
		Vals: make([]int32, n),
		Adj:  make([]int32, n*deg),
		Deg:  deg,
	}
	offs := []int{1, 7, 53, -1}
	for i := 0; i < n; i++ {
		g.Vals[i] = int32(i)
		for d := 0; d < deg; d++ {
			j := ((i+offs[d%len(offs)])%n + n) % n
			g.Adj[i*deg+d] = int32(j)
		}
	}
	return g
}

func SumIndexGraph(g IndexGraph) int64 {
	var acc int64
	for i := 0; i < len(g.Vals); i++ {
		base := i * g.Deg
		for d := 0; d < g.Deg; d++ {
			acc += int64(g.Vals[g.Adj[base+d]])
		}
	}
	return acc
}

// Scattered variants. The graphs above use near-local offsets (1, 7, 53, -1),
// which flatters the pointer version because Go allocates the nodes in order and
// the neighbours land in cache. A graph with random edges is the honest test of
// whether pointer chasing or index arithmetic wins.

func BuildPointerGraphRandom(n, deg int, seed int64) []*PNode {
	r := newRng(seed)
	nodes := make([]*PNode, n)
	for i := range nodes {
		nodes[i] = &PNode{Val: int32(i)}
	}
	for _, nd := range nodes {
		nd.Adj = make([]*PNode, deg)
		for d := 0; d < deg; d++ {
			nd.Adj[d] = nodes[r()%uint64(n)]
		}
	}
	return nodes
}

func BuildIndexGraphRandom(n, deg int, seed int64) IndexGraph {
	r := newRng(seed)
	g := IndexGraph{Vals: make([]int32, n), Adj: make([]int32, n*deg), Deg: deg}
	for i := 0; i < n; i++ {
		g.Vals[i] = int32(i)
	}
	for i := 0; i < n; i++ {
		for d := 0; d < deg; d++ {
			g.Adj[i*deg+d] = int32(r() % uint64(n))
		}
	}
	return g
}

// xorshift64*, so both builders see the same edge sequence from the same seed.
func newRng(seed int64) func() uint64 {
	s := uint64(seed) | 1
	return func() uint64 {
		s ^= s >> 12
		s ^= s << 25
		s ^= s >> 27
		return s * 2685821657736338717
	}
}
