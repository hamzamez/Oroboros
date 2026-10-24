# 0074 — A segment is its function restricted, wherever its table is, and a writer's too

Date: 2026-10-24
Status: Accepted (hamza, 2026-10-24: "go ahead with item 3, always mathematical and algebraic", the 10-21
plan's item 3; this is the declaration model `examples/os/wc.oro` needed). Extends
[ADR 0069](0069-an-observer-at-a-host-parameter.md) item 3, which defined a segment for an observer whose
table is its first parameter.

## Context

ADR 0069 made `segment.F(b, lo, hi, x̄) = F(b[lo:hi], x̄)` the way a program reads part of a buffer it owns
with the host's own function, and checked each against F (`TestASegmentIsItsFunctionRestricted`). The
check knew one shape: `bytes`' observers, table first, restricted on both sides, the law exactly F's.

A word count over files (wc-2026-10-24) needs two more:

- **`utf8.DecodeRune` on a window**, b[k:n], and its law on it, 1 ≤ size ≤ n − k. The size's lower half
  holds only on a non-empty input, a disjunction on all of B* (DecodeRune's header could state neither
  half until a header could name a component, ADR 0069 item 5).
- **`File.Read` into a suffix**, b[t:], after the program moves a cut codeword's bytes to the front.
  This is the C idiom `copy(b, b[k:n]); f.Read(b[t:])`, and in Go a hand-written `wc` writes the same.
  It is a WRITER (a write-borrow, ADR 0070), its table is its second parameter, and only the start is
  restricted.

## Decision

1. **A segment is F composed with a restriction of F's table parameter, wherever it stands.** Two
   restrictions: b[lo:hi], adding lo, hi ∈ Int after the table, and b[lo:], adding lo. The template is
   F's with the slice in the table's hole, and the result and purity are F's.
2. **Its precondition holds the restriction's domain**, 0 ≤ lo ≤ hi ≤ len b or 0 ≤ lo ≤ len b, among
   its conjuncts. A further conjunct narrows the domain, which is sound for a precondition.
3. **Its law holds F's law restricted.** The host sees q = b[lo:hi] or b[lo:]. So F's law holds with
   len b replaced by hi − lo or len b − lo, and for a writer with the same replacement on the buffer it
   hands back in b's place, b′, since |q′| = len b′ − lo. Cancellation in (ℤ, +), x − c = y − c ⟺
   x = y, turns len b′ − lo = len b − lo back into len b′ = len b: the whole buffer keeps its length.
4. **A further conjunct of the law is a claim about F on the narrower domain**, read off the host's
   source as every claim is (ADR 0022): `segment.DecodeRune`'s 1 ≤ size, on lo < hi. The checker holds
   the restricted part. The rest is the declaration's.
5. **That a writer leaves b[0, lo) as it was is the host's**: io.Reader writes only into the slice it
   is handed. The language does not state a frame law. A program that relies on it relies on Read's
   documentation, as the declaration's comment says.
6. **The check is one rule for every segment**, found by the path segment `segment` in the name and
   compared with the function it names (`segmentBase`). Each clause has a planted mistake on each kind:
   the slice dropped, the law unrestricted, the domain missing, a parameter's sort, purity, a decode over
   the whole table, a suffix written two-sided, a read's count bounded by the whole buffer, a suffix
   past the buffer.

## Why not

- **A language-level segment `b[lo..hi]`**: ADR 0069's "Why not" stands, a view of a live buffer is the
  lent algebra, and on Java a segment is (array, offset, length).
- **Carrying the cut codeword in a scratch buffer and decoding the junction there**: it needs a
  transducer composition in the program for each junction (a codeword of 3 bytes cut after 1 may decode
  as an error and leave 2 to carry again). The suffix read is what the host's programmers write, and it
  keeps the program's state the tail alone.
- **Stating DecodeRune's whole law on B\***: size = 0 iff len p = 0 is a disjunction, outside the linear
  fragment. On the restriction to B⁺ it is linear, and that is where a program calls it.

## Consequences

- `targets/go`: `go/unicode/utf8/segment.DecodeRune`, `go/os/File/segment.Read`, and DecodeRune's own
  law, size ≤ len p.
- The check covers four segments (bytes' two, utf8's, File's) and ten planted mistakes.
