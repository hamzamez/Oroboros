package gauntlet

// Hand-written JSON tree builders — the bar for examples/json/tree.oro.
//
// THREE FORMS, and the first two are the whole question:
//
//	TreeRec    recursive descent into a linked node struct — what a person
//	           writes, and what ADR 0014 says we may not
//	TreeFlat   a flat node table plus indices with an explicit stack — our
//	           shape, hand-written, with no clamping
//	GenMeasure emitted from the .oro
//
// TreeRec against TreeFlat prices the REPRESENTATION: pointers and per-node
// allocation against one flat array and an explicit stack. TreeFlat against
// GenMeasure prices our CODE GENERATION plus the clamped addressing the
// refinement layer asks for, with the representation held fixed.
//
// All three produce the same tree and the same answer: seen*1000 + Σ tag×depth
// over a depth-first walk. Tags are 1 number, 2 string, 3 literal, 4 array,
// 5 object; an object's children are its keys and values in document order,
// because `:` and `,` are skipped rather than structured — which is what the
// .oro does and what these must therefore do too.

const (
	treeNMax = 512
	treeDMax = 32
)

// ---------------------------------------------------------- recursive, boxed

type jnode struct {
	tag, val int
	kid, sib *jnode
}

func TreeRec(src []int) int {
	root, _ := jparseValue(src, jskip(src, 0))
	seen, acc := jwalkRec(root, 1)
	return seen*1000 + acc
}

func jskip(src []int, i int) int {
	for i < len(src) && (src[i] == 32 || src[i] == 9 || src[i] == 10 || src[i] == 13 ||
		src[i] == 58 || src[i] == 44) {
		i++
	}
	return i
}

func jparseValue(src []int, i int) (*jnode, int) {
	if i >= len(src) {
		return nil, i
	}
	c := src[i]
	if c == 123 || c == 91 {
		n := &jnode{tag: 5}
		if c == 91 {
			n.tag = 4
		}
		i++
		var last *jnode
		for {
			i = jskip(src, i)
			if i >= len(src) || src[i] == 125 || src[i] == 93 {
				if i < len(src) {
					i++
				}
				break
			}
			child, ni := jparseValue(src, i)
			if child == nil {
				break
			}
			i = ni
			if last == nil {
				n.kid = child
			} else {
				last.sib = child
			}
			last = child
		}
		return n, i
	}
	if c == 34 {
		ni := tokStringI(src, i)
		return &jnode{tag: 2, val: ni - i}, ni
	}
	if tokNumeric(c) {
		j := i
		for j < len(src) && tokNumeric(src[j]) {
			j++
		}
		return &jnode{tag: 1, val: j - i}, j
	}
	if tokAlpha(c) {
		j := i
		for j < len(src) && tokAlpha(src[j]) {
			j++
		}
		return &jnode{tag: 3, val: j - i}, j
	}
	return nil, i + 1
}

func jwalkRec(n *jnode, d int) (int, int) {
	if n == nil {
		return 0, 0
	}
	seen, acc := 1, n.tag*d
	for c := n.kid; c != nil; c = c.sib {
		s, a := jwalkRec(c, d+1)
		seen += s
		acc += a
	}
	return seen, acc
}

// ------------------------------------------------------------- flat, indexed
//
// The same algorithm the .oro runs, written the way a person would write it:
// no clamped addressing, because Go's own bounds check is already there and a
// person would rely on it.

func TreeFlat(src []int) int {
	nodes := make([]int, 4*treeNMax)
	stk := make([]int, 2*treeDMax)
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
			nodes[4*nn] = tg
			nodes[4*nn+1] = 0
			jlink(nodes, stk, sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			stk[2*sp] = nn
			stk[2*sp+1] = 0
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
			nodes[4*nn] = tg
			nodes[4*nn+1] = ni - i
			jlink(nodes, stk, sp, nn)
			if sp >= 1 {
				stk[2*(sp-1)+1] = nn
			}
			i = ni
			nn++
		default:
			i++
		}
	}
	return jwalkFlat(nodes)
}

func jlink(nodes, stk []int, sp, k int) {
	if sp < 1 {
		return
	}
	if lc := stk[2*(sp-1)+1]; lc == 0 {
		nodes[4*stk[2*(sp-1)]+2] = k
	} else {
		nodes[4*lc+3] = k
	}
}

func jwalkFlat(nodes []int) int {
	wl := make([]int, 2*treeNMax)
	wl[0], wl[1] = 1, 1
	sp, seen, acc, steps := 1, 0, 0, 0
	for sp >= 1 && steps < 2*treeNMax {
		n, d := wl[2*(sp-1)], wl[2*(sp-1)+1]
		sb, kd := nodes[4*n+3], nodes[4*n+2]
		sp--
		if sb != 0 {
			wl[2*sp], wl[2*sp+1] = sb, d
			sp++
		}
		if kd != 0 {
			wl[2*sp], wl[2*sp+1] = kd, d+1
			sp++
		}
		seen++
		acc += nodes[4*n] * d
		steps++
	}
	return seen*1000 + acc
}

// THE SAME PROGRAM WITH THE CLAMPS, which is what the .oro carries and the
// hand-written version above does not.
//
// The refinement layer discharges every index obligation by CLAMPING rather
// than by proving: `(go.* 4 (cn k))` is in range by construction, at the price
// of two compares per access. A person writing this by hand relies on Go's own
// bounds check instead. If this measures like the generated code then the
// clamps are the whole gap; if it measures like TreeFlat they are not.
//
// json-tokeniser found the same layer's three extra compares cost nothing —
// but that program had three of them and this one has one per table access.

func jcn(k int) int {
	if k < 0 || k >= treeNMax {
		return 0
	}
	return k
}

func jcd(d int) int {
	if d < 0 || d >= treeDMax {
		return 0
	}
	return d
}

func jcw(k int) int {
	if k < 0 || k >= treeNMax {
		return 0
	}
	return k
}

func TreeFlatClamped(src []int) int {
	nodes := make([]int, 4*treeNMax)
	stk := make([]int, 2*treeDMax)
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
			nodes[4*jcn(nn)] = tg
			nodes[4*jcn(nn)+1] = 0
			jlinkClamped(nodes, stk, sp, nn)
			if sp >= 1 {
				stk[2*jcd(sp-1)+1] = nn
			}
			stk[2*jcd(sp)] = nn
			stk[2*jcd(sp)+1] = 0
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
			nodes[4*jcn(nn)] = tg
			nodes[4*jcn(nn)+1] = ni - i
			jlinkClamped(nodes, stk, sp, nn)
			if sp >= 1 {
				stk[2*jcd(sp-1)+1] = nn
			}
			i = ni
			nn++
		default:
			i++
		}
	}
	return jwalkFlatClamped(nodes)
}

func jlinkClamped(nodes, stk []int, sp, k int) {
	if sp < 1 {
		return
	}
	if lc := stk[2*jcd(sp-1)+1]; lc == 0 {
		nodes[4*jcn(stk[2*jcd(sp-1)])+2] = k
	} else {
		nodes[4*jcn(lc)+3] = k
	}
}

func jwalkFlatClamped(nodes []int) int {
	wl := make([]int, 2*treeNMax)
	wl[0], wl[1] = 1, 1
	sp, seen, acc, steps := 1, 0, 0, 0
	for sp >= 1 && steps < 2*treeNMax {
		n, d := wl[2*jcw(sp-1)], wl[2*jcw(sp-1)+1]
		sb, kd := nodes[4*jcn(n)+3], nodes[4*jcn(n)+2]
		sp--
		if sb != 0 {
			wl[2*jcw(sp)], wl[2*jcw(sp)+1] = sb, d
			sp++
		}
		if kd != 0 {
			wl[2*jcw(sp)], wl[2*jcw(sp)+1] = kd, d+1
			sp++
		}
		seen++
		acc += nodes[4*jcn(n)] * d
		steps++
	}
	return seen*1000 + acc
}
