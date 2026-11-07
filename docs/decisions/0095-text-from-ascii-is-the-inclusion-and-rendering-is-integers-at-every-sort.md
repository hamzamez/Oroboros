# 0095 — Text from ASCII is the inclusion ι, and rendering is ℤ's at every sort

Date: 2026-11-07
Status: Accepted (hamza, 2026-11-07: "go ahead with step 3. always mathematical and algebraic", 6b′
step 3's `big_str` cases). Built with
[llvm-render-2026-11-07](../../gauntlet/results/llvm-render-2026-11-07.md).

## Context

render : ℤ → Σ* writes an integer's decimal numeral. Positional notation in base 10 is a bijection
between ℕ and the digit strings with no leading zero, so render is the sign and the remainders of
repeated Euclidean division, a fold into the free monoid (printing-research §2.2). Go, the JVM and
JavaScript realize it on a value above the word by their bignum's own numeral (`big_str`). Windows
has no bignum: it holds a value above the word as fixed limbs (ADR 0019, 0035), whose library
computes the quotient and remainder by a word but had no way to make text. The limb rung refused
`big_str` there "until a string surface exists", and seven differential cases skipped windows.

Two things were missing:
- **A string from computed characters.** The language builds text by η (`string_of`) and `++`. A
  renderer computes its digits into a table, and Σ_L had no map from a table to the string it spells.
  The loader's own comment had recorded the gap ("a whole table of scalars at once … is not built,
  because nothing has needed it").
- **Rendering where the value is in the word.** `big_str`'s parameter is `big`, so on windows a word
  value was asked for arbitrary precision the target cannot give.

## Decision

1. **ι : (B₇)* ↪ Σ\* is in Σ_L**, `ascii(b, lo, n)`, the word b[lo, lo+n) of scalars below 128 as
   text. ASCII* is the free monoid on the 128 scalars below 128, a submonoid of Σ*, and on it UTF-8
   is the identity. So ι is the inclusion: total, a monoid homomorphism (ι(uv) = ι(u) ++ ι(v)), and
   needing no validation. Its domain is a set the decision already proves, a table whose elements are
   in 0..127 (one interval per table), with lo + n ≤ len(b).
   - Each target realizes it with one copy: Go `string(b[lo:lo+n])`, the JVM `new String(b, lo, n,
     US_ASCII)`, JavaScript `TextDecoder` on the subarray, windows a helper copying the bytes behind
     a length and before the NUL (strings.md §10).
   - The segment is passed as (b, lo, n), as `WriteFile` takes one: a segment reaches a host only as
     a borrow, and a rule materialized for it would be a second copy.
2. **ι observes a live buffer by its meaning.** ι(b[lo, lo+n)) is the element of Σ* those bytes spell
   at the call, a value, so its result holds no part of the buffer whatever realizes it: Theorem L's
   condition (ADR 0069) holds of the name, not of a host declaration. A renderer calls it on its digit
   buffer while the buffer is live. No other string-valued host call becomes an observer.
3. **Rendering is ℤ's at every sort, chosen per representation.** The value's representation decides
   render's realization, as `<` at F64 is `fcmp` (llvm.md §3b):
   - on a host bignum, its own numeral (unchanged on Go, the JVM and JavaScript);
   - on fixed limbs, the limb library's `render`: one pass of `rem_small` and `div_small` by 10⁷ per
     seven digits, into a static field, then ι (emit/bignum.oro);
   - in the word, a term over the word's operations: `print_int`'s digit loop over LLVM's `sdiv` and
     `srem`, then ι (`realize big_str = lang.render`, targets/llvm/lang.oro).

   On a target with no bignum, `big_str`'s argument keeps the representation its facts give it, so
   a word value is not widened into limbs it does not need.

## Why not

- **The general map Scalar^[0,n) → Σ\*.** It is the free monoid's universal map, of which ι is the
  restriction. But its domain is "every element is a scalar", Scalar = [0, D7FF] ∪ [E000, 10FFFF], a
  union, which a table's one element interval cannot state, and its realization encodes. ι's domain
  is one interval and its realization a copy. The general map waits for a program that builds
  non-ASCII text from a table.
- **render as η and `++`, a digit at a time.** That is the fold itself and needs no new name, but it
  allocates once per digit and copies quadratically. One buffer and one ι is the fold's efficient
  realization, the same answer.
- **The decode d (`os.text_of`).** It is total and is ι on ASCII*. But it validates every byte, it
  belongs to `os`, which a numeric library should not depend on, and windows has no `os` cell yet.
- **A string surface inside the limb library.** The library would need a representation of strings,
  which differs on every host. ι is the one boundary, and the library is written once over it.
- **Rendering every value through limbs on windows.** A word value would be widened to limbs only to
  be divided back, and the program's one set would have to bound a value that never left the word.

## Consequences

- `big-mul`, `big-negmid`, `big-select`, `big-subdiv` and `word-wide` run on windows. The first four
  also run on Go, JavaScript and the JVM with both representations, seven configurations agreeing.
- `ascii` is a word of Σ_L, in the inventory, with a realization on four targets and a differential
  case (`ascii.oro`).
- The limb rung's trap now crashes with the bound's own reason on windows, as every host's does
  (ADR 0029: one set, one outcome).
- Not built:
  - ν on ι, `nulfree(ascii(b, lo, n))` decided by b's elements being at least 1;
  - `big_str` of a U value past 2⁶³ on windows, which the word-sort term does not take (`print_int`
    renders one);
  - the general map Scalar^[0,n) → Σ*;
  - rendering on Go, the JVM and JavaScript by the same per-representation rule, where a word value
    is still widened to the host's bignum to be printed (printing's P3).
