package emit

import (
	"strings"
	"testing"
)

// A PURE CALL'S GUARANTEE HOLDS WHERE ITS TERM IS READ (postconditions.md §5).
// Go's documentation sizes hex.Encode's buffer as `make([]byte,
// hex.EncodedLen(len(src)))`. That buffer's length is an atom — the call — and
// only EncodedLen's `ensures`, result = 2n, relates it to Encode's
// 2·len src ≤ len dst. It has to be assumed at the `build` (or the `let`)
// where the length is recorded, which is not a call whose `where` is checked.

const hexHead = `(use go/encoding/hex as hex)
(export f)
(sig f ((src (array (int 0 255)))) (array (int 0 255)) (where (<= (len src) 1000000)))
`

func TestABufferSizedByEncodedLenHoldsTheEncoding(t *testing.T) {
	if err := refineGo(t, hexHead+`(def f (src)
  (build (hex.EncodedLen (len src)) (fn (b) ((hex.Encode b src) (fn (b2 n) b2)))))`); err != nil {
		t.Errorf("len b = EncodedLen(len src) = 2·len src: %v", err)
	}
}

func TestABufferSizedByDecodedLenHoldsTheDecoding(t *testing.T) {
	if err := refineGo(t, hexHead+`(def f (src)
  (build (hex.DecodedLen (len src)) (fn (b) ((hex.Decode b src) (fn (b2 n e) b2)))))`); err != nil {
		t.Errorf("len src ≤ 2·⌊len src / 2⌋ + 1: %v", err)
	}
}

// THE GUARANTEE PROVES ONLY WHAT IT SAYS. Half the length is not enough to
// encode into, and a guarantee about DecodedLen must not be read as more.
func TestAShortBufferIsStillRefused(t *testing.T) {
	err := refineGo(t, hexHead+`(def f (src)
  (build (hex.DecodedLen (len src)) (fn (b) ((hex.Encode b src) (fn (b2 n) b2)))))`)
	if err == nil || !strings.Contains(err.Error(), "Encode") {
		t.Errorf("⌊len src / 2⌋ < 2·len src when src is not empty; got %v", err)
	}
}

// LEMMA 1: a guarantee holds only where its precondition is proven. With src
// unbounded, EncodedLen's own `where` (|n| ≤ 2^62 − 1) does not follow, so the
// program is refused there, and nothing is concluded from the result it would
// have had.
func TestAGuaranteeNeedsItsPrecondition(t *testing.T) {
	err := refineGo(t, `(use go/encoding/hex as hex)
(export f)
(sig f ((src (array (int 0 255)))) (array (int 0 255)))
(def f (src)
  (build (hex.EncodedLen (len src)) (fn (b) ((hex.Encode b src) (fn (b2 n) b2)))))`)
	if err == nil || !strings.Contains(err.Error(), "EncodedLen") {
		t.Errorf("len src may pass 2^62 − 1, where Go's EncodedLen wraps; got %v", err)
	}
}

// AND THROUGH A `let`: an impure allocation is bound, never substituted (ADR
// 0010), so `len a` is recorded against its length contract, whose argument is
// the atom. The atom's guarantee was assumed as that allocation's argument
// (postconditions.md §5, rule 1), and it alone makes i < 2·len src an index.
func TestALetBoundAllocationSizedByEncodedLen(t *testing.T) {
	const src = `(use go)
(use go/encoding/hex as hex)
(export f)
(sig f ((src (array (int 0 255))) (i int)) int
  (where (and (<= (len src) 1000) (and (<= 0 i) (< i (* K (len src)))))))
(def f (src i) (let a (go.make-int (hex.EncodedLen (len src))) (go.at-int a i)))`
	if err := refineGo(t, strings.Replace(src, "K", "2", 1)); err != nil {
		t.Errorf("i < 2·len src = len a: %v", err)
	}
	// EXACTLY THE LAW, NOT MORE: the buffer has 2·len src cells, not 3.
	if err := refineGo(t, strings.Replace(src, "K", "3", 1)); err == nil {
		t.Error("i may reach 3·len src − 1, past the 2·len src cells EncodedLen gives")
	}
}

// DecodedLen gives ⌊len src / 2⌋ cells, and an index below len src may pass them.
func TestDecodedLenIsHalfNotMore(t *testing.T) {
	err := refineGo(t, `(use go)
(use go/encoding/hex as hex)
(export f)
(sig f ((src (array (int 0 255))) (i int)) int
  (where (and (<= (len src) 1000) (and (<= 0 i) (< i (len src))))))
(def f (src i) (let a (go.make-int (hex.DecodedLen (len src))) (go.at-int a i)))`)
	if err == nil {
		t.Error("i may reach len src − 1, past ⌊len src / 2⌋")
	}
}
