package gauntlet

// Nesting: a heap structure stored inside another structure.
//
// This is the last known gap in the uniqueness story, and it collides with a
// decision already made. g2 decided structs are VALUES — always copied, never
// referenced, no interior pointers — which is what makes SROA unconditional and
// makes aliasing a struct local impossible. That reasoning holds for `point`,
// which is two f64s.
//
// A struct holding a dict is different: the field is a heap reference, so
// "copy the struct" has two possible meanings, and they differ by O(n).

// Boxed models a heap structure whose sharing can be tracked at runtime.
type Boxed struct {
	m  map[string]int
	rc int32
}

func NewBoxed(m map[string]int) *Boxed { return &Boxed{m: m, rc: 1} }

// Cache is a struct with one heap field and one scalar field.
type Cache struct {
	Entries *Boxed
	Hits    int
}

// ---------------------------------------------------------------- copying

// CopyShallow copies the struct; the dict is now shared.
func CopyShallow(c Cache) Cache {
	c.Entries.rc++
	return c
}

// CopyDeep copies the struct and the dict, preserving value semantics.
func CopyDeep(c Cache) Cache {
	return Cache{Entries: NewBoxed(DictCopy(c.Entries.m)), Hits: c.Hits}
}

// ---------------------------------------------------------------- mutation

// MutateDirect is what the compiler emits when the field is provably unique.
func MutateDirect(c Cache, k string, v int) Cache {
	c.Entries.m[k] = v
	return c
}

// MutateCOW checks sharing first — copy-on-write, the RC fallback applied one
// level down.
func MutateCOW(c Cache, k string, v int) Cache {
	if c.Entries.rc > 1 {
		c.Entries.rc--
		c.Entries = NewBoxed(DictCopy(c.Entries.m))
	}
	c.Entries.m[k] = v
	return c
}

// ---------------------------------------------------------------- dynamic index
//
// The genuinely hard case: the struct lives in a slice at a runtime index, so
// no static analysis can distinguish cs[i] from cs[j]. Liveness works over
// variables; cs[i] is not one.

//go:noinline
func MutateIndexedDirect(cs []Cache, i int, k string, v int) {
	cs[i].Entries.m[k] = v
}

//go:noinline
func MutateIndexedCOW(cs []Cache, i int, k string, v int) {
	if cs[i].Entries.rc > 1 {
		cs[i].Entries.rc--
		cs[i].Entries = NewBoxed(DictCopy(cs[i].Entries.m))
	}
	cs[i].Entries.m[k] = v
}

// ---------------------------------------------------------------- two levels

// Grid is a slice of slices: mutating one row, where the row itself is shared.
type Grid struct {
	Rows [][]float64
}

//go:noinline
func GridSetDirect(g Grid, r, c int, v float64) {
	g.Rows[r][c] = v
}

//go:noinline
func GridSetCopyRow(g Grid, r, c int, v float64) Grid {
	row := make([]float64, len(g.Rows[r]))
	copy(row, g.Rows[r])
	row[c] = v
	rows := make([][]float64, len(g.Rows))
	copy(rows, g.Rows)
	rows[r] = row
	return Grid{Rows: rows}
}
