package gauntlet

// Hand-written JSON tokenisers — the bar for examples/json/tokenize.oro.
//
// TWO FORMS, per the rule that has refuted five beliefs so far: carry the one
// expected to win and the one expected to lose.
//
//	TokBytes  []byte — what a person actually writes
//	TokInts   []int  — the same memory shape the emitter produces
//
// Our `int` is 64-bit (ADR 0012), so an emitted table of integers is []int and
// the input is eight times larger than a person's. Carrying both separates
// "our code generation is slower" from "our element type is wider", which one
// number could not.
//
// Both compute the same packed answer as the .oro, in the same clause order,
// with the same meaning at every edge: ntokens*1000 + maxdepth*10 + ok.

const tokCap = 32

func tokSpace(c int) bool { return c == 32 || c == 9 || c == 10 || c == 13 }
func tokDigit(c int) bool { return c >= 48 && c <= 57 }
func tokAlpha(c int) bool { return c >= 97 && c <= 122 }
func tokNumeric(c int) bool {
	return tokDigit(c) || c == 45 || c == 43 || c == 46 || c == 101 || c == 69
}
func tokOpener(c int) bool { return c == 123 || c == 91 }
func tokCloser(c int) bool { return c == 125 || c == 93 }
func tokPunct(c int) bool  { return c == 58 || c == 44 }

// ---------------------------------------------------------------- []byte

func TokBytes(src []byte) int {
	var stk [tokCap]byte
	i, nt, sp, mx, ok := 0, 0, 0, 0, 1
	for {
		if i >= len(src) {
			if sp != 0 {
				ok = 0
			}
			return nt*1000 + mx*10 + ok
		}
		if sp >= tokCap {
			return nt * 1000
		}
		c := src[i]
		switch {
		case c == 32 || c == 9 || c == 10 || c == 13:
			i++
		case c == '{' || c == '[':
			if c == '{' {
				stk[sp] = '}'
			} else {
				stk[sp] = ']'
			}
			i++
			nt++
			sp++
			if sp > mx {
				mx = sp
			}
		case c == '}' || c == ']':
			i++
			nt++
			if sp < 1 {
				ok = 0
			} else {
				sp--
				if stk[sp] != c {
					ok = 0
				}
			}
		case c == ':' || c == ',':
			i++
			nt++
		case c == '"':
			i = tokStringB(src, i)
			nt++
		case tokNumeric(int(c)):
			j := i
			for j < len(src) && tokNumeric(int(src[j])) {
				j++
			}
			i = j
			nt++
		case c >= 'a' && c <= 'z':
			j := i
			for j < len(src) && src[j] >= 'a' && src[j] <= 'z' {
				j++
			}
			i = j
			nt++
		default:
			i++
			ok = 0
		}
	}
}

func tokStringB(src []byte, i int) int {
	for j := i + 1; ; {
		if j >= len(src) {
			return j
		}
		if src[j] == '\\' {
			j += 2
			continue
		}
		if src[j] == '"' {
			return j + 1
		}
		j++
	}
}

// ---------------------------------------------------------------- []int

func TokInts(src []int) int {
	var stk [tokCap]int
	i, nt, sp, mx, ok := 0, 0, 0, 0, 1
	for {
		if i >= len(src) {
			if sp != 0 {
				ok = 0
			}
			return nt*1000 + mx*10 + ok
		}
		if sp >= tokCap {
			return nt * 1000
		}
		c := src[i]
		switch {
		case tokSpace(c):
			i++
		case tokOpener(c):
			if c == 123 {
				stk[sp] = 125
			} else {
				stk[sp] = 93
			}
			i++
			nt++
			sp++
			if sp > mx {
				mx = sp
			}
		case tokCloser(c):
			i++
			nt++
			if sp < 1 {
				ok = 0
			} else {
				sp--
				if stk[sp] != c {
					ok = 0
				}
			}
		case tokPunct(c):
			i++
			nt++
		case c == 34:
			i = tokStringI(src, i)
			nt++
		case tokNumeric(c):
			j := i
			for j < len(src) && tokNumeric(src[j]) {
				j++
			}
			i = j
			nt++
		case tokAlpha(c):
			j := i
			for j < len(src) && tokAlpha(src[j]) {
				j++
			}
			i = j
			nt++
		default:
			i++
			ok = 0
		}
	}
}

func tokStringI(src []int, i int) int {
	for j := i + 1; ; {
		if j >= len(src) {
			return j
		}
		if src[j] == 92 {
			j += 2
			continue
		}
		if src[j] == 34 {
			return j + 1
		}
		j++
	}
}

// ---------------------------------------------------------------- the input
//
// A document with the shape a tokeniser actually meets: nested objects and
// arrays, strings with escapes, numbers, and the three literals. Built rather
// than embedded so the size is a parameter of the benchmark.

func TokDoc(records int) []byte {
	out := []byte("{\"items\":[")
	for r := 0; r < records; r++ {
		if r > 0 {
			out = append(out, ',')
		}
		out = append(out, []byte(
			"{\"id\":1234,\"name\":\"a b\\\"c\",\"tags\":[\"x\",\"y\",\"z\"],"+
				"\"score\":-12.5e3,\"ok\":true,\"prev\":null,"+
				"\"meta\":{\"depth\":2,\"flag\":false}}")...)
	}
	out = append(out, ']', '}')
	return out
}

func TokDocInts(src []byte) []int {
	out := make([]int, len(src))
	for i, b := range src {
		out[i] = int(b)
	}
	return out
}
