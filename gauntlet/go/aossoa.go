package gauntlet

// AoS AGAINST SoA ON THE NODE TABLE — products.md §11 item 3, and the one
// measurement the 2026-09-09 assessment said could show part of that week's
// work to be worth nothing.
//
// products.md §6 proves the three layouts are the SAME OBJECT:
//
//	Π_{i∈I} (A × B)  ≅  (Π_{i∈I} A) × (Π_{i∈I} B)      array-of-structs ≅ struct-of-arrays
//	Π_{(i,j)∈I×J} V  ≅  Π_{i∈I} Π_{j∈J} V              the flat stride
//
// and concludes that which one is EMITTED is a target decision. The product
// build shipped the flat stride because it is what the hand-written programs
// already chose. If every host prefers that layout for every access pattern,
// the compiler's freedom to choose buys nothing and the record is legibility
// alone — which is a result worth having.
//
// THE VARIABLE IS THE LAYOUT AND NOTHING ELSE. Same algorithm, same element
// width on each host, no clamps — TreeFlat's algorithm written three ways:
//
//	flat   nodes[4*n+f], one table            what the product pass emits
//	SoA    tag[n], val[n], kid[n], sib[n]     four parallel tables
//	AoS    nodes[n].tag, a slice of records   Go's value struct
//
// AND BOTH ACCESS PATTERNS ARE CARRIED, because the rule that has refuted six
// beliefs here is to carry the form expected to win AND the one expected to
// lose (gauntlet.md):
//
//	W1  the tree's build-and-walk. Touches all four fields of ONE node at a
//	    time, in an order the data decides. Expected to favour flat/AoS.
//	W2  a column scan over a large table — Σ val where tag == 2. Reads TWO of
//	    four fields of EVERY node, in order. Expected to favour SoA, because
//	    it streams half the bytes.
//
// On Go, AoS and flat are the same bytes: a []struct{tag, val, kid, sib int}
// IS a stride-4 []int. So Go predicts flat ≡ AoS, and a difference would be
// the compiler, not the memory — which is worth knowing too.

type tnode struct{ tag, val, kid, sib int }

// ------------------------------------------------------------- W1, SoA

func TreeSoA(src []int) int {
	tag := make([]int, treeNMax)
	val := make([]int, treeNMax)
	kid := make([]int, treeNMax)
	sib := make([]int, treeNMax)
	stk := make([]int, 2*treeDMax)
	link := func(sp, k int) {
		if sp < 1 {
			return
		}
		if lc := stk[2*(sp-1)+1]; lc == 0 {
			kid[stk[2*(sp-1)]] = k
		} else {
			sib[lc] = k
		}
	}
	i, nn, sp := 0, 1, 0
	for {
		if i >= len(src) || nn >= treeNMax || sp >= treeDMax {
			break
		}
		c := src[i]
		switch {
		case c == 32 || c == 9 || c == 10 || c == 13 || c == 58 || c == 44:
			i++
		case c == 123 || c == 91:
			tg := 5
			if c == 91 {
				tg = 4
			}
			tag[nn], val[nn] = tg, 0
			link(sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			stk[2*sp], stk[2*sp+1] = nn, 0
			i++
			sp++
			nn++
		case c == 125 || c == 93:
			i++
			if sp >= 1 {
				sp--
			}
		case c == 34 || tokNumeric(c) || tokAlpha(c):
			tg, ni := 2, 0
			switch {
			case c == 34:
				ni = tokStringI(src, i)
			case tokNumeric(c):
				tg = 1
				j := i
				for j < len(src) && tokNumeric(src[j]) {
					j++
				}
				ni = j
			default:
				tg = 3
				j := i
				for j < len(src) && tokAlpha(src[j]) {
					j++
				}
				ni = j
			}
			tag[nn], val[nn] = tg, ni-i
			link(sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			i = ni
			nn++
		default:
			i++
		}
	}
	wl := make([]int, 2*treeNMax)
	wl[0], wl[1] = 1, 1
	wp, seen, acc, steps := 1, 0, 0, 0
	for wp >= 1 && steps < 2*treeNMax {
		n, d := wl[2*(wp-1)], wl[2*(wp-1)+1]
		sb, kd := sib[n], kid[n]
		wp--
		if sb != 0 {
			wl[2*wp], wl[2*wp+1] = sb, d
			wp++
		}
		if kd != 0 {
			wl[2*wp], wl[2*wp+1] = kd, d+1
			wp++
		}
		seen++
		acc += tag[n] * d
		steps++
	}
	return seen*1000 + acc
}

// ------------------------------------------------------------- W1, AoS

func TreeAoS(src []int) int {
	nodes := make([]tnode, treeNMax)
	stk := make([]int, 2*treeDMax)
	link := func(sp, k int) {
		if sp < 1 {
			return
		}
		if lc := stk[2*(sp-1)+1]; lc == 0 {
			nodes[stk[2*(sp-1)]].kid = k
		} else {
			nodes[lc].sib = k
		}
	}
	i, nn, sp := 0, 1, 0
	for {
		if i >= len(src) || nn >= treeNMax || sp >= treeDMax {
			break
		}
		c := src[i]
		switch {
		case c == 32 || c == 9 || c == 10 || c == 13 || c == 58 || c == 44:
			i++
		case c == 123 || c == 91:
			tg := 5
			if c == 91 {
				tg = 4
			}
			nodes[nn].tag, nodes[nn].val = tg, 0
			link(sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			stk[2*sp], stk[2*sp+1] = nn, 0
			i++
			sp++
			nn++
		case c == 125 || c == 93:
			i++
			if sp >= 1 {
				sp--
			}
		case c == 34 || tokNumeric(c) || tokAlpha(c):
			tg, ni := 2, 0
			switch {
			case c == 34:
				ni = tokStringI(src, i)
			case tokNumeric(c):
				tg = 1
				j := i
				for j < len(src) && tokNumeric(src[j]) {
					j++
				}
				ni = j
			default:
				tg = 3
				j := i
				for j < len(src) && tokAlpha(src[j]) {
					j++
				}
				ni = j
			}
			nodes[nn].tag, nodes[nn].val = tg, ni-i
			link(sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			i = ni
			nn++
		default:
			i++
		}
	}
	wl := make([]int, 2*treeNMax)
	wl[0], wl[1] = 1, 1
	wp, seen, acc, steps := 1, 0, 0, 0
	for wp >= 1 && steps < 2*treeNMax {
		n, d := wl[2*(wp-1)], wl[2*(wp-1)+1]
		sb, kd := nodes[n].sib, nodes[n].kid
		wp--
		if sb != 0 {
			wl[2*wp], wl[2*wp+1] = sb, d
			wp++
		}
		if kd != 0 {
			wl[2*wp], wl[2*wp+1] = kd, d+1
			wp++
		}
		seen++
		acc += nodes[n].tag * d
		steps++
	}
	return seen*1000 + acc
}

// ------------------------------------------------------------------ W2
//
// A COLUMN SCAN, which is the access pattern the tree does NOT have and the one
// SoA exists for. The table is large — 65,536 nodes, 2 MB flat on this host —
// because a scan over 443 nodes lives in L1 and would measure nothing but the
// loop. The fill is deterministic and outside the timed region.

const scanN = 65536

func scanFill(i int) tnode {
	return tnode{tag: (i*7)%5 + 1, val: i % 97, kid: (i + 1) % scanN, sib: (i * 3) % scanN}
}

func ScanTables() (flat []int, tag, val, kid, sib []int, aos []tnode) {
	flat = make([]int, 4*scanN)
	tag, val = make([]int, scanN), make([]int, scanN)
	kid, sib = make([]int, scanN), make([]int, scanN)
	aos = make([]tnode, scanN)
	for i := 0; i < scanN; i++ {
		n := scanFill(i)
		flat[4*i], flat[4*i+1], flat[4*i+2], flat[4*i+3] = n.tag, n.val, n.kid, n.sib
		tag[i], val[i], kid[i], sib[i] = n.tag, n.val, n.kid, n.sib
		aos[i] = n
	}
	return
}

func ScanFlat(nodes []int) int {
	s := 0
	for i := 0; i+3 < len(nodes); i += 4 {
		if nodes[i] == 2 {
			s += nodes[i+1]
		}
	}
	return s
}

func ScanSoA(tag, val []int) int {
	s := 0
	val = val[:len(tag)]
	for i, t := range tag {
		if t == 2 {
			s += val[i]
		}
	}
	return s
}

func ScanAoS(nodes []tnode) int {
	s := 0
	for i := range nodes {
		if nodes[i].tag == 2 {
			s += nodes[i].val
		}
	}
	return s
}

// ------------------------------------------------------------------ W3
//
// THE CASE AoS EXISTS FOR, and the first version of this file did not carry it.
// W1 has AoS's access pattern — all four fields of one node — but the node table
// is 443 nodes, 16 KB, resident in L1, so where a node's fields live cannot
// matter and W1 measures address arithmetic instead. SoA won it, and a result
// saying "SoA never loses" on that evidence would be the losing form missing
// from the comparison, which is exactly what gauntlet.md's rule is there to
// prevent.
//
// W3 is AoS's access pattern at a size where locality IS the cost: random nodes
// of a 1,048,576-node table (32 MB flat on this host, far past L3), all four
// fields read. One cache miss per node for AoS; four for SoA.
//
// The index sequence is Park–Miller, fixed and precomputed, so every layout
// visits exactly the same nodes in the same order and the generator is outside
// the timed region.

const gatherN = 1 << 20
const gatherM = 1 << 16

func GatherIdx() []int {
	idx := make([]int, gatherM)
	x := 1
	for i := range idx {
		x = x * 48271 % 2147483647
		idx[i] = x & (gatherN - 1)
	}
	return idx
}

func gatherFill(i int) tnode {
	return tnode{tag: (i*7)%5 + 1, val: i % 97, kid: (i + 1) % gatherN, sib: (i * 3) % gatherN}
}

func GatherTables() (flat, tag, val, kid, sib []int, aos []tnode) {
	flat = make([]int, 4*gatherN)
	tag, val = make([]int, gatherN), make([]int, gatherN)
	kid, sib = make([]int, gatherN), make([]int, gatherN)
	aos = make([]tnode, gatherN)
	for i := 0; i < gatherN; i++ {
		n := gatherFill(i)
		flat[4*i], flat[4*i+1], flat[4*i+2], flat[4*i+3] = n.tag, n.val, n.kid, n.sib
		tag[i], val[i], kid[i], sib[i] = n.tag, n.val, n.kid, n.sib
		aos[i] = n
	}
	return
}

func GatherFlat(t, idx []int) int {
	s := 0
	for _, k := range idx {
		s += t[4*k] + t[4*k+1] + t[4*k+2] + t[4*k+3]
	}
	return s
}

func GatherSoA(tag, val, kid, sib, idx []int) int {
	s := 0
	for _, k := range idx {
		s += tag[k] + val[k] + kid[k] + sib[k]
	}
	return s
}

func GatherAoS(ns []tnode, idx []int) int {
	s := 0
	for _, k := range idx {
		n := &ns[k]
		s += n.tag + n.val + n.kid + n.sib
	}
	return s
}
