package emit

import (
	"fmt"
	"sort"
	"strings"
)

// InPlace is one component of a threaded result that is an argument's own
// storage handed back (Prim.InPlace, ADR 0070).
type InPlace struct{ Result, Arg int }

// THE ALGEBRA (host-buffers.md §1, ADR 0070). A host call that writes a buffer
// is aspect 1, and aspect 1 is two arrows:
//
//	a write-borrow       B ⊸ B ⊗ R,  the B handed back the SAME storage as the B given
//	a consume-and-return B ⊸ B ⊗ R,  the B handed back storage the host returns,
//	                                  possibly new (an append that grows)
//
// The core needs only the threaded arrow, B ⊗ R, the state-passing translation's
// (core/place.go). The host's own call returns R for a write-borrow and B ⊗ R
// for a consume. So a write-borrow's handed-back B is the identity on its
// argument, and a host primitive whose results are R alone is the call a Go
// programmer writes, `n := hex.Encode(dst, src)`, with no closure to manufacture
// the pair.

// HostResults are the results the host's own call gives: the threaded results
// without the in-place components.
func HostResults(p Prim) []string {
	all := p.Results
	if len(all) == 0 && p.Result != "" {
		all = []string{p.Result}
	}
	if len(p.InPlace) == 0 {
		return all
	}
	skip := map[int]bool{}
	for _, ip := range p.InPlace {
		skip[ip.Result] = true
	}
	var out []string
	for j, r := range all {
		if !skip[j] {
			out = append(out, r)
		}
	}
	return out
}

// HostName is the name of the primitive the IR calls for a primitive with
// in-place components. The `#` keeps a program from writing it.
func HostName(q string) string { return "#host:" + strings.ReplaceAll(q, ".", ":") }

// hostPrims derives, for each primitive with in-place components, the host
// primitive the IR calls: its arguments, template, purity and imports, and the
// host's own results. It runs after the retractions, so a fallible write-borrow's
// raw call is the one derived from, its components the host's values.
func (tg *Target) hostPrims() error {
	var names []string
	for n, p := range tg.Prims {
		if len(p.InPlace) > 0 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		p := tg.Prims[n]
		for _, ip := range p.InPlace {
			if ip.Arg >= len(p.Args) || !strings.HasPrefix(p.Args[ip.Arg], "buffer ") {
				return fmt.Errorf("%s: an in-place result is argument %d, which is no buffer", n, ip.Arg+1)
			}
		}
		res := HostResults(p)
		if len(res) == 0 {
			return fmt.Errorf("%s writes its buffer in place and its host returns nothing: "+
				"a statement host call over a buffer is not built (ADR 0070)", n)
		}
		h := p
		h.Name, h.InPlace, h.Host, h.Ensures, h.Retract = HostName(n), nil, "", nil, nil
		h.Result, h.Results = "", nil
		if len(res) == 1 {
			h.Result = res[0]
		} else {
			h.Results = res
		}
		tg.Prims[h.Name] = h
		p.Host = h.Name
		tg.Prims[n] = p
	}
	return nil
}
